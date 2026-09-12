package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/your-github/ccml/internal/audio"
	"github.com/your-github/ccml/internal/library"
	"github.com/your-github/ccml/internal/metadata"
	"github.com/your-github/ccml/internal/model"
	"github.com/your-github/ccml/internal/organize"
	"github.com/your-github/ccml/internal/store"
)

// App is the Wails binding exposed to the React frontend.
type App struct {
	ctx       context.Context
	store     *store.Store
	tools     *audio.Toolchain
	scanner   *library.Scanner
	processor *audio.Processor
	bpmKey    *audio.EssentiaAnalyzer
	metadata  *metadata.Service
	organizer *organize.Service
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
	const metadataUserAgent = "CCML/0.1 (https://github.com/your-github/ccml)"
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

	return &App{
		store:     db,
		tools:     tools,
		scanner:   library.NewScanner(db, probe),
		processor: processor,
		bpmKey:    audio.NewEssentiaAnalyzer(),
		metadata:  metaService,
		organizer: organize.NewService(db),
	}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(_ context.Context) {
	// Database cleanup is performed by Close after wails.Run returns.
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
	return model.SystemStatus{
		FFmpegPath:        a.tools.FFmpeg,
		FFprobePath:       a.tools.FFprobe,
		EssentiaPath:      a.bpmKey.Path(),
		FFmpegReady:       a.tools.Ready(),
		EssentiaReady:     a.bpmKey.Available(),
		MetadataProviders: a.metadata.ProviderNames(),
	}
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

// ScanFolder scans supported audio files and updates the SQLite media library.
func (a *App) ScanFolder(root string) (model.ScanResult, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return model.ScanResult{}, errors.New("music folder is required")
	}
	if !a.tools.Ready() {
		return model.ScanResult{}, errors.New("ffmpeg/ffprobe not found; install FFmpeg or configure CCML_FFMPEG and CCML_FFPROBE")
	}
	return a.scanner.Scan(a.context(), root)
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
