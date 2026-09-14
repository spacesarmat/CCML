package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/spacesarmat/CCML/internal/audio"
	jobqueue "github.com/spacesarmat/CCML/internal/jobs"
	"github.com/spacesarmat/CCML/internal/library"
	"github.com/spacesarmat/CCML/internal/metadata"
	"github.com/spacesarmat/CCML/internal/model"
	"github.com/spacesarmat/CCML/internal/organize"
	"github.com/spacesarmat/CCML/internal/settings"
	"github.com/spacesarmat/CCML/internal/store"
	"github.com/spacesarmat/CCML/internal/tagging"
)

// App is the Wails binding exposed to the React frontend.
type App struct {
	ctx            context.Context
	store          *store.Store
	scanMu         sync.Mutex
	scanCancel     context.CancelFunc
	tools          *audio.Toolchain
	scanner        *library.Scanner
	processor      *audio.Processor
	bpmKey         *audio.EssentiaAnalyzer
	metadataMu     sync.RWMutex
	metadata       *metadata.Service
	settings       *settings.Service
	metadataConfig model.MetadataSettings
	organizer      *organize.Service
	tagEditor      *tagging.Service
	media          *mediaService
	jobs           *jobqueue.Manager
	toolUpdater    *audio.ToolUpdater
	toolUpdateMu   sync.Mutex
	toolUpdating   bool
	toolUpdateErr  string
}

// NewApp creates all backend services and opens the media-library database.
func NewApp() (*App, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config directory: %w", err)
	}

	appDir := filepath.Join(configDir, "CCML")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, fmt.Errorf("create application directory: %w", err)
	}

	db, err := store.Open(filepath.Join(appDir, "library.db"))
	if err != nil {
		return nil, err
	}

	tools := audio.DiscoverToolchain()
	probe := audio.NewProbe(tools)
	processor := audio.NewProcessor(tools)
	settingsService := settings.New(appDir)
	metadataConfig, err := settingsService.Load()
	if err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close database after settings failure: %w", closeErr))
		}
		return nil, err
	}
	metaService := buildMetadataService(metadataConfig)
	tagEditor, err := tagging.NewService(db, appDir)
	if err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close database after tag-editor failure: %w", closeErr))
		}
		return nil, err
	}

	media, err := newMediaService(db, tools, appDir)
	if err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close database after media-service failure: %w", closeErr))
		}
		return nil, err
	}

	app := &App{
		store:          db,
		tools:          tools,
		scanner:        library.NewScanner(db, probe),
		processor:      processor,
		bpmKey:         audio.NewEssentiaAnalyzer(),
		metadata:       metaService,
		settings:       settingsService,
		metadataConfig: metadataConfig,
		organizer:      organize.NewService(db),
		tagEditor:      tagEditor,
		media:          media,
	}
	app.toolUpdater = audio.NewToolUpdater(appDir, tools)
	app.jobs = jobqueue.New(db)
	app.jobs.RegisterConcurrent("metadata_enrichment", metadataConfig.MetadataEnrichmentConcurrency, app.runMetadataJobItem)
	app.jobs.SetEmitter(func(name string, payload any) {
		if app.ctx != nil {
			runtime.EventsEmit(app.ctx, name, payload)
		}
	})
	return app, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.jobs != nil {
		if err := a.jobs.Start(ctx); err != nil {
			runtime.LogErrorf(ctx, "start background jobs: %v", err)
		}
	}
	if a.toolUpdater == nil {
		return
	}
	due, err := a.toolUpdater.ShouldAutoCheck()
	if err != nil {
		a.toolUpdateMu.Lock()
		a.toolUpdateErr = err.Error()
		a.toolUpdateMu.Unlock()
		runtime.LogWarningf(ctx, "prepare automatic FFmpeg update check: %v", err)
		return
	}
	if due {
		a.toolUpdateMu.Lock()
		a.toolUpdating = true
		a.toolUpdateMu.Unlock()
		go func() {
			// Let the frontend subscribe to Wails events before emitting status.
			timer := time.NewTimer(300 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				a.toolUpdateMu.Lock()
				a.toolUpdating = false
				a.toolUpdateMu.Unlock()
				return
			case <-timer.C:
			}
			if _, err := a.runFFmpegUpdate(ctx, false, true); err != nil && !errors.Is(err, context.Canceled) {
				runtime.LogWarningf(ctx, "automatic FFmpeg update failed: %v", err)
			}
		}()
	}
}

func (a *App) shutdown(_ context.Context) {
	a.CancelScan()
	if a.jobs != nil {
		a.jobs.Stop()
	}
}

// Close releases application resources.
func (a *App) Close() error {
	var errs []error
	if a.media != nil {
		if err := a.media.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if a.store != nil {
		if err := a.store.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// SystemStatus reports availability of optional external audio tools.
func (a *App) SystemStatus() model.SystemStatus {
	snapshot := a.tools.Snapshot()
	a.toolUpdateMu.Lock()
	updating := a.toolUpdating
	updateErr := a.toolUpdateErr
	a.toolUpdateMu.Unlock()
	autoUpdateSupported := a.toolUpdater != nil && a.toolUpdater.AutoUpdateSupported()
	return model.SystemStatus{
		FFmpegPath:                snapshot.FFmpeg,
		FFprobePath:               snapshot.FFprobe,
		FFmpegVersion:             snapshot.Version,
		FFmpegSource:              snapshot.Source,
		FFmpegReady:               a.tools.Ready(),
		FFmpegUpdating:            updating,
		FFmpegUpdateError:         updateErr,
		FFmpegAutoUpdateSupported: autoUpdateSupported,
		EssentiaPath:              a.bpmKey.Path(),
		EssentiaReady:             a.bpmKey.Available(),
		MetadataProviders:         a.metadataService().ProviderNames(),
	}
}

// GetMetadataSettings returns the locally stored metadata provider configuration.
func (a *App) GetMetadataSettings() model.MetadataSettings {
	a.metadataMu.RLock()
	defer a.metadataMu.RUnlock()
	return a.metadataConfig
}

// SaveMetadataSettings stores provider configuration and activates it immediately.
func (a *App) SaveMetadataSettings(config model.MetadataSettings) (model.MetadataSettings, error) {
	config.Normalize()
	if err := validateMetadataSettings(config); err != nil {
		return model.MetadataSettings{}, err
	}
	if a.settings == nil {
		return model.MetadataSettings{}, errors.New("settings service is not available")
	}
	if err := a.settings.Save(config); err != nil {
		return model.MetadataSettings{}, err
	}

	service := buildMetadataService(config)
	a.metadataMu.Lock()
	a.metadataConfig = config
	a.metadata = service
	a.metadataMu.Unlock()

	// New metadata-enrichment jobs use the updated worker count immediately.
	// A job that is already running keeps the worker pool it started with.
	if a.jobs != nil {
		a.jobs.RegisterConcurrent("metadata_enrichment", config.MetadataEnrichmentConcurrency, a.runMetadataJobItem)
	}

	if a.store != nil {
		if err := a.store.ClearMetadataLookupCache(a.context()); err != nil && a.ctx != nil {
			runtime.LogWarningf(a.ctx, "clear metadata cache after settings change: %v", err)
		}
	}

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "metadata:settings-updated", service.ProviderNames())
	}
	return config, nil
}

// OpenMetadataLink opens a trusted provider documentation or credential page in the system browser.
// The caller supplies a symbolic key rather than an arbitrary URL so the Wails binding cannot be
// used as a generic URL launcher.
func (a *App) OpenMetadataLink(key string) error {
	if a.ctx == nil {
		return errors.New("application is not ready")
	}
	links := map[string]string{
		"musicbrainz":      "https://musicbrainz.org/doc/MusicBrainz_API",
		"deezer":           "https://developers.deezer.com/api",
		"itunes":           "https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/index.html",
		"theaudiodb":       "https://www.theaudiodb.com/free_music_api",
		"discogs":          "https://www.discogs.com/settings/developers",
		"spotify":          "https://developer.spotify.com/dashboard",
		"applemusic":       "https://developer.apple.com/account/resources/authkeys/list",
		"youtube":          "https://console.cloud.google.com/apis/credentials",
		"soundcloud":       "https://developers.soundcloud.com/docs/api/register-app",
		"yandexmusic":      "https://yandex.ru/dev/id/doc/ru/register-api",
		"traxsource":       "https://www.traxsource.com/terms-of-service",
		"traxsourcebridge": "https://parse.bot/marketplace/0123da1a-5d2d-4970-bf6f-398f792855a1/traxsource-com-api",
	}
	url, ok := links[strings.ToLower(strings.TrimSpace(key))]
	if !ok {
		return fmt.Errorf("unknown metadata help link: %s", key)
	}
	runtime.BrowserOpenURL(a.ctx, url)
	return nil
}

// TestMetadataProviders checks the settings currently entered in the Settings
// dialog without persisting them first. This lets a user verify new credentials
// before pressing Save.
func (a *App) TestMetadataProviders(config model.MetadataSettings) ([]model.MetadataProviderReport, error) {
	config.Normalize()
	if err := validateMetadataSettings(config); err != nil {
		return nil, err
	}
	service := buildMetadataService(config)
	if len(service.ProviderNames()) == 0 {
		return nil, errors.New("no metadata providers enabled")
	}
	return service.ValidateProviders(a.context()), nil
}

func (a *App) metadataService() *metadata.Service {
	a.metadataMu.RLock()
	defer a.metadataMu.RUnlock()
	return a.metadata
}

func validateMetadataSettings(config model.MetadataSettings) error {
	if config.DiscogsEnabled && config.DiscogsToken == "" {
		return errors.New("Discogs is enabled but its token is empty")
	}
	if config.SpotifyEnabled && config.SpotifyAccessToken == "" && (config.SpotifyClientID == "" || config.SpotifyClientSecret == "") {
		return errors.New("Spotify is enabled but neither an access token nor client ID + client secret are configured")
	}
	if config.AppleMusicEnabled && config.AppleMusicDeveloperToken == "" {
		return errors.New("Apple Music is enabled but its developer token is empty")
	}
	if config.YouTubeEnabled && config.YouTubeAPIKey == "" {
		return errors.New("YouTube is enabled but its API key is empty")
	}
	if config.SoundCloudEnabled && config.SoundCloudAccessToken == "" {
		return errors.New("SoundCloud is enabled but its access token is empty")
	}
	return nil
}

func buildMetadataService(config model.MetadataSettings) *metadata.Service {
	const metadataUserAgent = "CCML/0.6 (https://github.com/spacesarmat/CCML)"
	providers := make([]metadata.Provider, 0, 11)
	if config.MusicBrainzEnabled {
		providers = append(providers, metadata.NewMusicBrainzProvider(metadataUserAgent))
	}
	if config.TheAudioDBEnabled {
		providers = append(providers, metadata.NewTheAudioDBProvider(config.TheAudioDBAPIKey))
	}
	if config.DeezerEnabled {
		providers = append(providers, metadata.NewDeezerProvider())
	}
	if config.ITunesEnabled {
		providers = append(providers, metadata.NewITunesProvider(config.ITunesCountry))
	}
	if config.DiscogsEnabled && config.DiscogsToken != "" {
		providers = append(providers, metadata.NewDiscogsProvider(config.DiscogsToken, metadataUserAgent))
	}
	if config.SpotifyEnabled && (config.SpotifyAccessToken != "" || (config.SpotifyClientID != "" && config.SpotifyClientSecret != "")) {
		providers = append(providers, metadata.NewSpotifyProvider(config.SpotifyAccessToken, config.SpotifyClientID, config.SpotifyClientSecret, config.SpotifyMarket))
	}
	if config.AppleMusicEnabled && config.AppleMusicDeveloperToken != "" {
		providers = append(providers, metadata.NewAppleMusicProvider(config.AppleMusicDeveloperToken, config.AppleMusicStorefront))
	}
	if config.YouTubeEnabled && config.YouTubeAPIKey != "" {
		providers = append(providers, metadata.NewYouTubeProvider(config.YouTubeAPIKey))
	}
	if config.SoundCloudEnabled && config.SoundCloudAccessToken != "" {
		providers = append(providers, metadata.NewSoundCloudProvider(config.SoundCloudAccessToken))
	}
	if config.YandexMusicEnabled {
		providers = append(providers, metadata.NewYandexMusicProvider(config.YandexMusicToken, config.YandexMusicLanguage, metadataUserAgent))
	}
	if config.TraxsourceEnabled {
		providers = append(providers, metadata.NewTraxsourceProviderWithFallback(metadataUserAgent, config.TraxsourceAPIKey))
	}
	return metadata.NewService(providers...)
}

// UpdateFFmpeg checks for and installs the latest supported managed FFmpeg build.
func (a *App) UpdateFFmpeg() (model.FFmpegUpdateResult, error) {
	return a.runFFmpegUpdate(a.context(), true, false)
}

func (a *App) runFFmpegUpdate(ctx context.Context, force, alreadyMarked bool) (model.FFmpegUpdateResult, error) {
	if a.toolUpdater == nil {
		return model.FFmpegUpdateResult{}, errors.New("FFmpeg updater is not available")
	}
	if !alreadyMarked {
		a.toolUpdateMu.Lock()
		if a.toolUpdating {
			a.toolUpdateMu.Unlock()
			return model.FFmpegUpdateResult{}, errors.New("FFmpeg update is already running")
		}
		a.toolUpdating = true
		a.toolUpdateErr = ""
		a.toolUpdateMu.Unlock()
	}

	runtime.EventsEmit(ctx, "tools:ffmpeg:update-started")
	result, err := a.toolUpdater.EnsureLatest(ctx, force)

	a.toolUpdateMu.Lock()
	a.toolUpdating = false
	if err != nil {
		a.toolUpdateErr = err.Error()
	} else {
		a.toolUpdateErr = ""
	}
	a.toolUpdateMu.Unlock()

	if err != nil {
		runtime.EventsEmit(ctx, "tools:ffmpeg:update-error", err.Error())
		return model.FFmpegUpdateResult{}, err
	}
	out := model.FFmpegUpdateResult{
		Version: result.Version, Changed: result.Changed, FFmpegPath: result.FFmpegPath,
		FFprobePath: result.FFprobePath, Source: result.Source,
	}
	runtime.EventsEmit(ctx, "tools:ffmpeg:update-finished", out)
	return out, nil
}

// SelectMusicFolder opens a native directory picker.
func (a *App) SelectMusicFolder() (string, error) {
	if a.ctx == nil {
		return "", errors.New("application is not ready")
	}
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select music folder",
	})
	if err != nil {
		return "", fmt.Errorf("select music folder: %w", err)
	}
	return path, nil
}

// ScanFolder incrementally scans supported audio files and updates the SQLite media library.
func (a *App) ScanFolder(root string) (model.ScanResult, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return model.ScanResult{}, errors.New("music folder is required")
	}
	if !a.tools.Ready() {
		return model.ScanResult{}, errors.New("ffmpeg/ffprobe not found; install FFmpeg or configure CCML_FFMPEG and CCML_FFPROBE")
	}

	a.scanMu.Lock()
	if a.scanCancel != nil {
		a.scanMu.Unlock()
		return model.ScanResult{}, errors.New("a library scan is already running")
	}
	scanCtx, cancel := context.WithCancel(a.context())
	a.scanCancel = cancel
	a.scanMu.Unlock()

	defer func() {
		cancel()
		a.scanMu.Lock()
		a.scanCancel = nil
		a.scanMu.Unlock()
	}()

	runtime.EventsEmit(a.context(), "library:scan:started", model.ScanProgress{Root: root})
	result, err := a.scanner.Scan(scanCtx, root, func(progress model.ScanProgress) {
		runtime.EventsEmit(a.context(), "library:scan:progress", progress)
	})
	if err != nil {
		runtime.EventsEmit(a.context(), "library:scan:error", err.Error())
		return result, err
	}
	runtime.EventsEmit(a.context(), "library:scan:finished", result)
	return result, nil
}

// CancelScan cancels the active library scan, if any.
func (a *App) CancelScan() bool {
	a.scanMu.Lock()
	cancel := a.scanCancel
	a.scanMu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

// ListLibraryRoots returns all managed music folders.
func (a *App) ListLibraryRoots() ([]model.LibraryRoot, error) {
	return a.store.ListLibraryRoots(a.context())
}

// RemoveLibraryRoot forgets a managed folder. Files on disk are never deleted.
func (a *App) RemoveLibraryRoot(root string, deleteTracks bool) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return errors.New("library root is required")
	}
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return fmt.Errorf("resolve library root: %w", err)
	}
	return a.store.RemoveLibraryRoot(a.context(), absRoot, deleteTracks)
}

// LibraryStatistics returns aggregate counts for the media library.
func (a *App) LibraryStatistics() (model.LibraryStats, error) {
	stats, err := a.store.LibraryStatistics(a.context())
	if err != nil {
		return model.LibraryStats{}, err
	}
	tracks, err := a.store.AllTracks(a.context())
	if err != nil {
		return model.LibraryStats{}, err
	}
	stats.DuplicateGroups = len(library.FindDuplicates(tracks, 2_000))
	return stats, nil
}

// ListTracks returns a page of tracks from the media library.
func (a *App) ListTracks(search string, limit, offset int) ([]model.Track, error) {
	return a.store.ListTracks(a.context(), search, limit, offset)
}

// FindDuplicates finds probable duplicates by normalized metadata and duration.
func (a *App) FindDuplicates() ([]model.DuplicateGroup, error) {
	tracks, err := a.store.AllTracks(a.context())
	if err != nil {
		return nil, err
	}
	return library.FindDuplicates(tracks, 2_000), nil
}

// ReadTrackTags reads editable tags directly from the selected audio file.
func (a *App) ReadTrackTags(trackID int64) (model.TagSnapshot, error) {
	return a.tagEditor.Read(a.context(), trackID)
}

// PreviewTagEdits previews a partial tag edit for one or more tracks.
func (a *App) PreviewTagEdits(trackIDs []int64, patch model.TagPatch) ([]model.TagPreview, error) {
	return a.tagEditor.Preview(a.context(), trackIDs, patch)
}

// ApplyTagEdits writes a partial tag edit to files and SQLite, recording Undo history.
func (a *App) ApplyTagEdits(trackIDs []int64, patch model.TagPatch) (model.TagApplyResult, error) {
	return a.tagEditor.Apply(a.context(), trackIDs, patch)
}

// SelectCoverArt opens a native JPEG/PNG picker.
func (a *App) SelectCoverArt() (string, error) {
	if a.ctx == nil {
		return "", errors.New("application is not ready")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select cover artwork",
		Filters: []runtime.FileFilter{
			{DisplayName: "Images (*.jpg;*.jpeg;*.png)", Pattern: "*.jpg;*.jpeg;*.png"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("select cover artwork: %w", err)
	}
	return path, nil
}

// SetCoverArt embeds one JPEG/PNG cover into all selected tracks.
func (a *App) SetCoverArt(trackIDs []int64, imagePath string) (model.TagApplyResult, error) {
	return a.tagEditor.SetCoverArt(a.context(), trackIDs, imagePath)
}

// RemoveCoverArt removes the front-cover image from all selected tracks.
func (a *App) RemoveCoverArt(trackIDs []int64) (model.TagApplyResult, error) {
	return a.tagEditor.RemoveCoverArt(a.context(), trackIDs)
}

// ApplyMetadataCandidate writes a provider candidate to the file and optionally embeds its artwork.
func (a *App) ApplyMetadataCandidate(trackID int64, candidate model.MetadataCandidate, includeArtwork bool) (model.TagApplyResult, error) {
	return a.tagEditor.ApplyMetadataCandidate(a.context(), trackID, candidate, includeArtwork)
}

// ListTagHistory returns recent reversible metadata edits.
func (a *App) ListTagHistory(limit int) ([]model.TagHistory, error) {
	return a.tagEditor.History(a.context(), limit)
}

// UndoTagChange restores the tags and changed front covers from one change set.
func (a *App) UndoTagChange(changeSetID int64) (model.TagApplyResult, error) {
	return a.tagEditor.Undo(a.context(), changeSetID)
}

// PrepareTrackMedia prepares a browser-compatible audio preview and embedded cover art.
func (a *App) PrepareTrackMedia(trackID int64) (model.TrackMedia, error) {
	if a.media == nil {
		return model.TrackMedia{}, errors.New("media service is not available")
	}
	return a.media.PrepareTrack(a.context(), trackID)
}

// PrepareTrackAudioPreview creates a compatibility MP3 only when native WebView playback fails.
func (a *App) PrepareTrackAudioPreview(trackID int64) (string, error) {
	if a.media == nil {
		return "", errors.New("media service is not available")
	}
	return a.media.PrepareAudioPreview(a.context(), trackID)
}

// GenerateSpectrograms creates cached before/after full-track spectrogram images.
// processedPath may be empty before the first render.
func (a *App) GenerateSpectrograms(trackID int64, processedPath string) (model.SpectrogramComparison, error) {
	if a.media == nil {
		return model.SpectrogramComparison{}, errors.New("media service is not available")
	}
	return a.media.Spectrograms(a.context(), trackID, processedPath)
}

// AnalyzeLoudness runs EBU R128 loudness analysis and stores the result.
func (a *App) AnalyzeLoudness(trackID int64) (model.Loudness, error) {
	track, err := a.store.TrackByID(a.context(), trackID)
	if err != nil {
		return model.Loudness{}, err
	}
	measurement, err := a.processor.Analyze(a.context(), track.Path, audio.AnalysisOptions{})
	if err != nil {
		return model.Loudness{}, err
	}
	if err := a.store.UpdateLoudness(a.context(), trackID, measurement); err != nil {
		return model.Loudness{}, err
	}
	return measurement, nil
}

// NormalizeTrack creates a processed copy using the configured mastering chain.
func (a *App) NormalizeTrack(trackID int64, opts model.ProcessingOptions) (model.ProcessingResult, error) {
	track, err := a.store.TrackByID(a.context(), trackID)
	if err != nil {
		return model.ProcessingResult{}, err
	}
	result, err := a.processor.Process(a.context(), track.Path, opts)
	if err != nil {
		return model.ProcessingResult{}, err
	}
	return result, nil
}

// WriteReplayGain writes ReplayGain-style metadata without re-encoding audio.
func (a *App) WriteReplayGain(trackID int64, targetLUFS float64) (model.Loudness, error) {
	track, err := a.store.TrackByID(a.context(), trackID)
	if err != nil {
		return model.Loudness{}, err
	}
	measurement, err := a.processor.WriteReplayGain(a.context(), track.Path, targetLUFS)
	if err != nil {
		return model.Loudness{}, err
	}
	if err := a.store.UpdateLoudness(a.context(), trackID, measurement); err != nil {
		return model.Loudness{}, err
	}
	return measurement, nil
}

// AnalyzeBPMKey optionally invokes Essentia for BPM and musical-key analysis.
func (a *App) AnalyzeBPMKey(trackID int64) (model.BPMKey, error) {
	track, err := a.store.TrackByID(a.context(), trackID)
	if err != nil {
		return model.BPMKey{}, err
	}
	result, err := a.bpmKey.Analyze(a.context(), track.Path)
	if err != nil {
		return model.BPMKey{}, err
	}
	if err := a.store.UpdateBPMKey(a.context(), trackID, result); err != nil {
		return model.BPMKey{}, err
	}
	return result, nil
}

// LookupMetadata searches configured metadata providers for a track, reusing a
// short-lived SQLite cache when the query and provider configuration are unchanged.
func (a *App) LookupMetadata(trackID int64) (model.MetadataLookupResult, error) {
	return a.lookupMetadataTrack(trackID, false)
}

// RefreshMetadata bypasses the lookup cache and asks the providers again.
func (a *App) RefreshMetadata(trackID int64) (model.MetadataLookupResult, error) {
	return a.lookupMetadataTrack(trackID, true)
}

func (a *App) lookupMetadataTrack(trackID int64, force bool) (model.MetadataLookupResult, error) {
	track, err := a.store.TrackByID(a.context(), trackID)
	if err != nil {
		return model.MetadataLookupResult{}, err
	}
	tags, tagErr := a.tagEditor.Read(a.context(), trackID)
	isrc := track.ISRC
	if tagErr == nil && strings.TrimSpace(tags.ISRC) != "" {
		isrc = tags.ISRC
	}
	return a.lookupMetadataQuery(a.context(), metadata.QueryFromTrack(track, isrc), force)
}

func (a *App) lookupMetadataQuery(ctx context.Context, query model.MetadataQuery, force bool) (model.MetadataLookupResult, error) {
	key, err := a.metadataCacheKey(query)
	if err != nil {
		return model.MetadataLookupResult{}, err
	}
	if !force && a.store != nil {
		if cached, age, ok, cacheErr := a.store.MetadataLookupCache(ctx, key); cacheErr != nil {
			if a.ctx != nil {
				runtime.LogWarningf(a.ctx, "read metadata lookup cache: %v", cacheErr)
			}
		} else if ok {
			cached.Cached = true
			cached.CacheAgeSeconds = int64(age.Seconds())
			return cached, nil
		}
	}

	result, err := a.metadataService().Search(ctx, query)
	if err != nil {
		return result, err
	}
	if a.store != nil && metadataLookupCacheable(result) {
		if cacheErr := a.store.PutMetadataLookupCache(ctx, key, result, 12*time.Hour); cacheErr != nil && a.ctx != nil {
			runtime.LogWarningf(a.ctx, "write metadata lookup cache: %v", cacheErr)
		}
	}
	return result, nil
}

func metadataLookupCacheable(result model.MetadataLookupResult) bool {
	if len(result.Candidates) == 0 {
		return false
	}
	for _, report := range result.ProviderReports {
		if report.Status == "error" {
			return false
		}
	}
	return true
}

func (a *App) metadataCacheKey(query model.MetadataQuery) (string, error) {
	a.metadataMu.RLock()
	config := a.metadataConfig
	a.metadataMu.RUnlock()
	raw, err := json.Marshal(struct {
		Query  model.MetadataQuery    `json:"query"`
		Config model.MetadataSettings `json:"config"`
	}{Query: query, Config: config})
	if err != nil {
		return "", fmt.Errorf("encode metadata cache key: %w", err)
	}
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum[:]), nil
}

// EnrichMetadata automatically looks up and applies the best high-confidence
// metadata match for each selected track. This synchronous API remains useful for
// small selections; large library operations should use CreateMetadataEnrichmentJob.
func (a *App) EnrichMetadata(trackIDs []int64, opts model.MetadataEnrichmentOptions) (model.MetadataEnrichmentResult, error) {
	if len(trackIDs) == 0 {
		return model.MetadataEnrichmentResult{}, errors.New("no tracks selected")
	}
	if len(trackIDs) > 100 {
		return model.MetadataEnrichmentResult{}, fmt.Errorf("metadata enrichment is limited to 100 tracks per synchronous batch; use the background queue for larger selections")
	}
	if err := normalizeEnrichmentOptions(&opts); err != nil {
		return model.MetadataEnrichmentResult{}, err
	}

	result := model.MetadataEnrichmentResult{Items: make([]model.MetadataEnrichmentItem, 0, len(trackIDs))}
	for _, trackID := range trackIDs {
		if err := a.context().Err(); err != nil {
			return result, err
		}
		item, err := a.enrichMetadataTrack(a.context(), trackID, opts)
		if err != nil {
			item.Error = err.Error()
			result.Failed++
		} else if item.Applied {
			result.Applied++
		} else {
			item.Skipped = true
			result.Skipped++
		}
		result.Processed++
		result.Items = append(result.Items, item)
	}
	return result, nil
}

// CreateMetadataEnrichmentJob queues metadata enrichment without blocking the UI.
// Jobs and item progress are persisted in SQLite and survive application restarts.
func (a *App) CreateMetadataEnrichmentJob(trackIDs []int64, opts model.MetadataEnrichmentOptions) (model.BackgroundJob, error) {
	if a.jobs == nil {
		return model.BackgroundJob{}, errors.New("background job manager is not available")
	}
	if len(trackIDs) == 0 {
		return model.BackgroundJob{}, errors.New("no tracks selected")
	}
	if err := normalizeEnrichmentOptions(&opts); err != nil {
		return model.BackgroundJob{}, err
	}
	raw, err := json.Marshal(opts)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("encode metadata enrichment job options: %w", err)
	}
	job, err := a.store.CreateBackgroundJob(a.context(), "metadata_enrichment", "Metadata enrichment", string(raw), trackIDs)
	if err != nil {
		return model.BackgroundJob{}, err
	}
	a.jobs.Wake()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "jobs:created", job)
	}
	return job, nil
}

// CreateLibraryMetadataEnrichmentJob queues metadata enrichment for the whole library.
func (a *App) CreateLibraryMetadataEnrichmentJob(opts model.MetadataEnrichmentOptions) (model.BackgroundJob, error) {
	if a.jobs == nil {
		return model.BackgroundJob{}, errors.New("background job manager is not available")
	}
	if err := normalizeEnrichmentOptions(&opts); err != nil {
		return model.BackgroundJob{}, err
	}
	raw, err := json.Marshal(opts)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("encode metadata enrichment job options: %w", err)
	}
	job, err := a.store.CreateBackgroundJobForLibrary(a.context(), "metadata_enrichment", "Metadata enrichment — entire library", string(raw), 100000)
	if err != nil {
		return model.BackgroundJob{}, err
	}
	a.jobs.Wake()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "jobs:created", job)
	}
	return job, nil
}

// ListBackgroundJobs returns persistent job summaries, newest first.
func (a *App) ListBackgroundJobs(limit int) ([]model.BackgroundJob, error) {
	return a.store.ListBackgroundJobs(a.context(), limit)
}

// ListBackgroundJobItems returns track-level details for a job.
func (a *App) ListBackgroundJobItems(jobID int64, limit, offset int) ([]model.BackgroundJobItem, error) {
	return a.store.ListBackgroundJobItems(a.context(), jobID, limit, offset)
}

// ListRunningBackgroundJobItems returns the exact set of tracks currently being
// executed by a parallel background job.
func (a *App) ListRunningBackgroundJobItems(jobID int64, limit int) ([]model.BackgroundJobItem, error) {
	return a.store.ListRunningBackgroundJobItems(a.context(), jobID, limit)
}

func (a *App) PauseBackgroundJob(jobID int64) (model.BackgroundJob, error) {
	if a.jobs == nil {
		return model.BackgroundJob{}, errors.New("background job manager is not available")
	}
	return a.jobs.Pause(a.context(), jobID)
}

func (a *App) ResumeBackgroundJob(jobID int64) (model.BackgroundJob, error) {
	if a.jobs == nil {
		return model.BackgroundJob{}, errors.New("background job manager is not available")
	}
	return a.jobs.Resume(a.context(), jobID)
}

func (a *App) CancelBackgroundJob(jobID int64) (model.BackgroundJob, error) {
	if a.jobs == nil {
		return model.BackgroundJob{}, errors.New("background job manager is not available")
	}
	return a.jobs.Cancel(a.context(), jobID)
}

func (a *App) RetryFailedBackgroundJob(jobID int64) (model.BackgroundJob, error) {
	if a.jobs == nil {
		return model.BackgroundJob{}, errors.New("background job manager is not available")
	}
	return a.jobs.RetryFailed(a.context(), jobID)
}

func normalizeEnrichmentOptions(opts *model.MetadataEnrichmentOptions) error {
	if opts.MinimumConfidence <= 0 {
		opts.MinimumConfidence = 0.86
	}
	if opts.MinimumConfidence < 0.5 || opts.MinimumConfidence > 1 {
		return fmt.Errorf("minimum confidence must be between 0.5 and 1.0")
	}
	opts.SearchMode = metadata.NormalizeEnrichmentSearchMode(opts.SearchMode)
	return nil
}

func (a *App) enrichMetadataTrack(ctx context.Context, trackID int64, opts model.MetadataEnrichmentOptions) (model.MetadataEnrichmentItem, error) {
	item := model.MetadataEnrichmentItem{TrackID: trackID}
	track, err := a.store.TrackByID(ctx, trackID)
	if err != nil {
		return item, err
	}
	item.Path = track.Path
	tags, tagErr := a.tagEditor.Read(ctx, trackID)
	isrc := track.ISRC
	if tagErr == nil && strings.TrimSpace(tags.ISRC) != "" {
		isrc = tags.ISRC
	}
	lookup, searchDiagnostics, err := a.metadataService().SearchEnrichment(
		ctx,
		metadata.QueryFromTrack(track, isrc),
		opts.SearchMode,
		opts.MinimumConfidence,
	)
	item.SearchMode = searchDiagnostics.Mode
	item.SearchDurationMS = searchDiagnostics.DurationMS
	item.ProvidersResponded = searchDiagnostics.ProvidersResponded
	item.ProvidersSkipped = searchDiagnostics.ProvidersSkipped
	item.EarlyStopped = searchDiagnostics.EarlyStopped
	if err != nil {
		return item, err
	}
	if len(lookup.Candidates) == 0 {
		item.Skipped = true
		return item, nil
	}
	candidate := lookup.Suggested
	if candidate.Confidence == 0 {
		candidate = lookup.Candidates[0]
	}
	item.Source, item.Confidence = candidate.Source, candidate.Confidence
	if candidate.MatchClass == "rejected" || candidate.Confidence < opts.MinimumConfidence {
		item.Skipped = true
		return item, nil
	}
	applyResult, err := a.tagEditor.ApplyMetadataCandidateWithPolicy(ctx, trackID, candidate, opts.IncludeArtwork, opts.OnlyMissing)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no applicable fields") {
			item.Skipped = true
			return item, nil
		}
		return item, err
	}
	if applyResult.Changed > 0 {
		item.Applied = true
	} else {
		item.Skipped = true
	}
	return item, nil
}

func (a *App) runMetadataJobItem(ctx context.Context, job model.BackgroundJob, work model.BackgroundJobItem) (jobqueue.ItemResult, error) {
	var opts model.MetadataEnrichmentOptions
	if err := json.Unmarshal([]byte(job.OptionsJSON), &opts); err != nil {
		return jobqueue.ItemResult{}, fmt.Errorf("decode metadata enrichment job options: %w", err)
	}
	if err := normalizeEnrichmentOptions(&opts); err != nil {
		return jobqueue.ItemResult{}, err
	}
	item, err := a.enrichMetadataTrack(ctx, work.TrackID, opts)
	raw, marshalErr := json.Marshal(item)
	if marshalErr != nil {
		return jobqueue.ItemResult{}, fmt.Errorf("encode metadata enrichment item result: %w", marshalErr)
	}
	if err != nil {
		return jobqueue.ItemResult{ResultJSON: string(raw)}, err
	}
	status := "completed"
	if item.Skipped {
		status = "skipped"
	}
	return jobqueue.ItemResult{Status: status, ResultJSON: string(raw)}, nil
}

// PreviewRename renders a safe target path from track metadata and a template.
func (a *App) PreviewRename(trackID int64, req model.OrganizeRequest) (string, error) {
	track, err := a.store.TrackByID(a.context(), trackID)
	if err != nil {
		return "", err
	}
	return a.organizer.Preview(track, req)
}

// OrganizeTrack moves or renames a track and updates its database path.
func (a *App) OrganizeTrack(trackID int64, req model.OrganizeRequest) (string, error) {
	track, err := a.store.TrackByID(a.context(), trackID)
	if err != nil {
		return "", err
	}
	return a.organizer.Apply(a.context(), track, req)
}

func (a *App) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
