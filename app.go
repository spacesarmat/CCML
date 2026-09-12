package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/spacesarmat/CCML/internal/audio"
	"github.com/spacesarmat/CCML/internal/library"
	"github.com/spacesarmat/CCML/internal/metadata"
	"github.com/spacesarmat/CCML/internal/model"
	"github.com/spacesarmat/CCML/internal/organize"
	"github.com/spacesarmat/CCML/internal/store"
)

// App is the Wails binding exposed to the React frontend.
type App struct {
	ctx           context.Context
	store         *store.Store
	scanMu        sync.Mutex
	scanCancel    context.CancelFunc
	tools         *audio.Toolchain
	scanner       *library.Scanner
	processor     *audio.Processor
	bpmKey        *audio.EssentiaAnalyzer
	metadata      *metadata.Service
	organizer     *organize.Service
	toolUpdater   *audio.ToolUpdater
	toolUpdateMu  sync.Mutex
	toolUpdating  bool
	toolUpdateErr string
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
	const metadataUserAgent = "CCML/0.1 (https://github.com/spacesarmat/CCML)"
	providers := []metadata.Provider{
		metadata.NewMusicBrainzProvider(metadataUserAgent),
		metadata.NewTheAudioDBProvider(os.Getenv("THEAUDIODB_API_KEY")),
		metadata.NewDeezerProvider(),
	}
	if token := strings.TrimSpace(os.Getenv("DISCOGS_TOKEN")); token != "" {
		providers = append(providers, metadata.NewDiscogsProvider(token, metadataUserAgent))
	}
	if token := strings.TrimSpace(os.Getenv("SPOTIFY_ACCESS_TOKEN")); token != "" {
		providers = append(providers, metadata.NewSpotifyProvider(token, os.Getenv("SPOTIFY_MARKET")))
	}
	if token := strings.TrimSpace(os.Getenv("APPLE_MUSIC_DEVELOPER_TOKEN")); token != "" {
		providers = append(providers, metadata.NewAppleMusicProvider(token, os.Getenv("APPLE_MUSIC_STOREFRONT")))
	}
	if key := strings.TrimSpace(os.Getenv("YOUTUBE_API_KEY")); key != "" {
		providers = append(providers, metadata.NewYouTubeProvider(key))
	}
	if token := strings.TrimSpace(os.Getenv("SOUNDCLOUD_ACCESS_TOKEN")); token != "" {
		providers = append(providers, metadata.NewSoundCloudProvider(token))
	}
	metaService := metadata.NewService(providers...)

	app := &App{
		store:     db,
		tools:     tools,
		scanner:   library.NewScanner(db, probe),
		processor: processor,
		bpmKey:    audio.NewEssentiaAnalyzer(),
		metadata:  metaService,
		organizer: organize.NewService(db),
	}
	app.toolUpdater = audio.NewToolUpdater(appDir, tools)
	return app, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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
}

// Close releases application resources.
func (a *App) Close() error {
	if a.store == nil {
		return nil
	}
	return a.store.Close()
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
		MetadataProviders:         a.metadata.ProviderNames(),
	}
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

// LookupMetadata searches configured metadata providers for a track.
func (a *App) LookupMetadata(trackID int64) (model.MetadataLookupResult, error) {
	track, err := a.store.TrackByID(a.context(), trackID)
	if err != nil {
		return model.MetadataLookupResult{}, err
	}
	return a.metadata.Search(a.context(), model.MetadataQuery{
		Title:      track.Title,
		Artist:     track.Artist,
		Album:      track.Album,
		DurationMS: track.DurationMS,
	})
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
