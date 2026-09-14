// Package model contains shared domain types used by backend services and Wails bindings.
package model

import (
	"strings"
	"time"
)

// Track represents one indexed audio file.
type Track struct {
	ID                       int64   `json:"id"`
	Path                     string  `json:"path"`
	FileName                 string  `json:"fileName"`
	Extension                string  `json:"extension"`
	Size                     int64   `json:"size"`
	ModifiedUnix             int64   `json:"modifiedUnix"`
	Title                    string  `json:"title"`
	Artist                   string  `json:"artist"`
	Album                    string  `json:"album"`
	AlbumArtist              string  `json:"albumArtist"`
	Genre                    string  `json:"genre"`
	Year                     int     `json:"year"`
	TrackNumber              int     `json:"trackNumber"`
	TrackTotal               int     `json:"trackTotal"`
	DiscNumber               int     `json:"discNumber"`
	DiscTotal                int     `json:"discTotal"`
	Composer                 string  `json:"composer"`
	Comment                  string  `json:"comment"`
	Label                    string  `json:"label"`
	CatalogNumber            string  `json:"catalogNumber"`
	ISRC                     string  `json:"isrc"`
	ReleaseDate              string  `json:"releaseDate"`
	DurationMS               int64   `json:"durationMs"`
	Codec                    string  `json:"codec"`
	SampleRate               int     `json:"sampleRate"`
	Channels                 int     `json:"channels"`
	BitRate                  int64   `json:"bitRate"`
	BPM                      float64 `json:"bpm"`
	Key                      string  `json:"key"`
	KeyScale                 string  `json:"keyScale"`
	LoudnessI                float64 `json:"loudnessI"`
	TruePeak                 float64 `json:"truePeak"`
	LRA                      float64 `json:"lra"`
	Threshold                float64 `json:"threshold"`
	ScanError                string  `json:"scanError"`
	LastMetadataJobStatus    string  `json:"lastMetadataJobStatus"`
	LastMetadataJobUpdatedAt string  `json:"lastMetadataJobUpdatedAt"`
}

// ScanResult summarizes a completed or cancelled folder scan.
type ScanResult struct {
	Root      string        `json:"root"`
	Found     int           `json:"found"`
	Indexed   int           `json:"indexed"`
	Added     int           `json:"added"`
	Updated   int           `json:"updated"`
	Skipped   int           `json:"skipped"`
	Removed   int           `json:"removed"`
	Failed    int           `json:"failed"`
	Cancelled bool          `json:"cancelled"`
	Duration  time.Duration `json:"duration"`
	Errors    []string      `json:"errors"`
}

// ScanProgress is emitted while the scanner processes a library root.
type ScanProgress struct {
	Root        string `json:"root"`
	CurrentFile string `json:"currentFile"`
	Found       int    `json:"found"`
	Scanned     int    `json:"scanned"`
	Added       int    `json:"added"`
	Updated     int    `json:"updated"`
	Skipped     int    `json:"skipped"`
	Removed     int    `json:"removed"`
	Failed      int    `json:"failed"`
	Finished    bool   `json:"finished"`
	Cancelled   bool   `json:"cancelled"`
}

// LibraryRoot is a folder managed by the media library.
type LibraryRoot struct {
	Path       string `json:"path"`
	CreatedAt  string `json:"createdAt"`
	LastScanAt string `json:"lastScanAt"`
}

// LibraryStats is a compact summary of the indexed media library.
type LibraryStats struct {
	Tracks          int64 `json:"tracks"`
	Artists         int64 `json:"artists"`
	Albums          int64 `json:"albums"`
	DurationMS      int64 `json:"durationMs"`
	SizeBytes       int64 `json:"sizeBytes"`
	DuplicateGroups int   `json:"duplicateGroups"`
}

// TagSnapshot is the editable metadata state for one audio file.
type TagSnapshot struct {
	Title         string `json:"title"`
	Artist        string `json:"artist"`
	Album         string `json:"album"`
	AlbumArtist   string `json:"albumArtist"`
	Genre         string `json:"genre"`
	Composer      string `json:"composer"`
	Comment       string `json:"comment"`
	Label         string `json:"label"`
	CatalogNumber string `json:"catalogNumber"`
	ISRC          string `json:"isrc"`
	ReleaseDate   string `json:"releaseDate"`
	Year          int    `json:"year"`
	TrackNumber   int    `json:"trackNumber"`
	TrackTotal    int    `json:"trackTotal"`
	DiscNumber    int    `json:"discNumber"`
	DiscTotal     int    `json:"discTotal"`
	CoverMIME     string `json:"coverMime"`
	CoverSize     int    `json:"coverSize"`
}

// TagPatch describes a partial metadata edit. Fields lists the values that
// should actually be changed, which makes empty-string clearing and batch edits unambiguous.
type TagPatch struct {
	Fields        []string `json:"fields"`
	Title         string   `json:"title"`
	Artist        string   `json:"artist"`
	Album         string   `json:"album"`
	AlbumArtist   string   `json:"albumArtist"`
	Genre         string   `json:"genre"`
	Composer      string   `json:"composer"`
	Comment       string   `json:"comment"`
	Label         string   `json:"label"`
	CatalogNumber string   `json:"catalogNumber"`
	ISRC          string   `json:"isrc"`
	ReleaseDate   string   `json:"releaseDate"`
	Year          int      `json:"year"`
	TrackNumber   int      `json:"trackNumber"`
	TrackTotal    int      `json:"trackTotal"`
	DiscNumber    int      `json:"discNumber"`
	DiscTotal     int      `json:"discTotal"`
}

// TagPreview shows the before/after state of a proposed edit.
type TagPreview struct {
	TrackID  int64       `json:"trackId"`
	Path     string      `json:"path"`
	Before   TagSnapshot `json:"before"`
	After    TagSnapshot `json:"after"`
	Warnings []string    `json:"warnings"`
}

// TagApplyResult summarizes a tag, cover-art, metadata-provider, or undo operation.
type TagApplyResult struct {
	ChangeSetID int64    `json:"changeSetId"`
	Changed     int      `json:"changed"`
	Failed      int      `json:"failed"`
	Errors      []string `json:"errors"`
}

// TagHistory describes one reversible metadata change set.
type TagHistory struct {
	ID            int64  `json:"id"`
	CreatedAt     string `json:"createdAt"`
	Label         string `json:"label"`
	Status        string `json:"status"`
	AffectedCount int    `json:"affectedCount"`
}

// DuplicateGroup is a probable set of duplicate tracks.
type DuplicateGroup struct {
	Artist     string  `json:"artist"`
	Title      string  `json:"title"`
	DurationMS int64   `json:"durationMs"`
	Tracks     []Track `json:"tracks"`
}

// Loudness contains EBU R128 measurements.
type Loudness struct {
	InputI         float64 `json:"inputI"`
	InputTP        float64 `json:"inputTP"`
	InputLRA       float64 `json:"inputLRA"`
	InputThreshold float64 `json:"inputThreshold"`
	TargetOffset   float64 `json:"targetOffset"`
}

// ProcessingOptions controls the non-destructive audio processing chain.
type ProcessingOptions struct {
	OutputPath        string  `json:"outputPath"`
	TargetLUFS        float64 `json:"targetLUFS"`
	TargetTruePeakDB  float64 `json:"targetTruePeakDb"`
	TargetLRA         float64 `json:"targetLRA"`
	PreGainDB         float64 `json:"preGainDb"`
	RepairClipping    bool    `json:"repairClipping"`
	MultibandCompress bool    `json:"multibandCompress"`
	Limit             bool    `json:"limit"`
	PitchSemitones    float64 `json:"pitchSemitones"`
	KeepOriginal      bool    `json:"keepOriginal"`
}

// ProcessingResult describes a rendered audio file and the measurements used.
type ProcessingResult struct {
	InputPath   string   `json:"inputPath"`
	OutputPath  string   `json:"outputPath"`
	Measurement Loudness `json:"measurement"`
	FilterGraph string   `json:"filterGraph"`
}

// TrackMedia contains private loopback URLs used by the Inspector player.
type TrackMedia struct {
	AudioURL   string `json:"audioUrl"`
	CoverURL   string `json:"coverUrl"`
	DurationMS int64  `json:"durationMs"`
	IsPreview  bool   `json:"isPreview"`
}

// SpectrogramComparison contains generated before/after spectrogram images.
type SpectrogramComparison struct {
	BeforeURL  string `json:"beforeUrl"`
	AfterURL   string `json:"afterUrl"`
	BeforePath string `json:"beforePath"`
	AfterPath  string `json:"afterPath"`
}

// BPMKey is optional Essentia analysis output.
type BPMKey struct {
	BPM      float64 `json:"bpm"`
	Key      string  `json:"key"`
	Scale    string  `json:"scale"`
	Strength float64 `json:"strength"`
}

// MetadataQuery describes a lookup request sent to external providers.
type MetadataQuery struct {
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	DurationMS int64  `json:"durationMs"`
	ISRC       string `json:"isrc"`
}

// MetadataScore explains how a metadata candidate matched the local track.
type MetadataScore struct {
	Title        float64 `json:"title"`
	Artist       float64 `json:"artist"`
	Album        float64 `json:"album"`
	Version      float64 `json:"version"`
	Duration     float64 `json:"duration"`
	Identifier   float64 `json:"identifier"`
	Completeness float64 `json:"completeness"`
	Total        float64 `json:"total"`
}

// MetadataCandidate is a normalized result from an external metadata provider.
type MetadataCandidate struct {
	Source            string        `json:"source"`
	ExternalID        string        `json:"externalId"`
	SourceURL         string        `json:"sourceUrl"`
	Title             string        `json:"title"`
	Artist            string        `json:"artist"`
	Album             string        `json:"album"`
	AlbumArtist       string        `json:"albumArtist"`
	ReleaseDate       string        `json:"releaseDate"`
	Year              int           `json:"year"`
	Genre             string        `json:"genre"`
	Label             string        `json:"label"`
	CatalogNumber     string        `json:"catalogNumber"`
	ISRC              string        `json:"isrc"`
	TrackNumber       int           `json:"trackNumber"`
	TrackTotal        int           `json:"trackTotal"`
	DiscNumber        int           `json:"discNumber"`
	DiscTotal         int           `json:"discTotal"`
	ArtworkURL        string        `json:"artworkUrl"`
	ArtworkWidth      int           `json:"artworkWidth"`
	ArtworkHeight     int           `json:"artworkHeight"`
	ArtworkEmbeddable bool          `json:"artworkEmbeddable"`
	DurationMS        int64         `json:"durationMs"`
	Confidence        float64       `json:"confidence"`
	MatchClass        string        `json:"matchClass"`
	MatchIssues       []string      `json:"matchIssues"`
	Score             MetadataScore `json:"score"`
}

// MetadataFieldOption is one source/value choice for the merge editor.
type MetadataFieldOption struct {
	Field      string  `json:"field"`
	Value      string  `json:"value"`
	Number     int     `json:"number"`
	Source     string  `json:"source"`
	ExternalID string  `json:"externalId"`
	Confidence float64 `json:"confidence"`
}

// MetadataProviderReport describes one provider attempt during a lookup.
type MetadataProviderReport struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Candidates int    `json:"candidates"`
	DurationMS int64  `json:"durationMs"`
	Error      string `json:"error"`
	Retryable  bool   `json:"retryable"`
}

// MetadataLookupResult contains ranked provider results, provider diagnostics,
// and a cross-provider suggestion.
type MetadataLookupResult struct {
	Candidates      []MetadataCandidate      `json:"candidates"`
	Suggested       MetadataCandidate        `json:"suggested"`
	FieldOptions    []MetadataFieldOption    `json:"fieldOptions"`
	ProviderReports []MetadataProviderReport `json:"providerReports"`
	Warnings        []string                 `json:"warnings"`
	Cached          bool                     `json:"cached"`
	CacheAgeSeconds int64                    `json:"cacheAgeSeconds"`
}

// MetadataEnrichmentOptions controls automatic enrichment for selected tracks.
type MetadataEnrichmentOptions struct {
	MinimumConfidence float64 `json:"minimumConfidence"`
	IncludeArtwork    bool    `json:"includeArtwork"`
	OnlyMissing       bool    `json:"onlyMissing"`
	SearchMode        string  `json:"searchMode"`
}

// MetadataEnrichmentItem reports the enrichment outcome for one track.
type MetadataEnrichmentItem struct {
	TrackID            int64   `json:"trackId"`
	Path               string  `json:"path"`
	Source             string  `json:"source"`
	Confidence         float64 `json:"confidence"`
	Applied            bool    `json:"applied"`
	Skipped            bool    `json:"skipped"`
	Warning            string  `json:"warning"`
	SearchMode         string  `json:"searchMode"`
	SearchDurationMS   int64   `json:"searchDurationMs"`
	ProvidersResponded int     `json:"providersResponded"`
	ProvidersSkipped   int     `json:"providersSkipped"`
	EarlyStopped       bool    `json:"earlyStopped"`
	Error              string  `json:"error"`
}

// MetadataEnrichmentResult summarizes a batch metadata enrichment operation.
type MetadataEnrichmentResult struct {
	Processed int                      `json:"processed"`
	Applied   int                      `json:"applied"`
	Skipped   int                      `json:"skipped"`
	Failed    int                      `json:"failed"`
	Items     []MetadataEnrichmentItem `json:"items"`
}

// BackgroundJob is one persistent asynchronous library operation.
type BackgroundJob struct {
	ID             int64   `json:"id"`
	Type           string  `json:"type"`
	Title          string  `json:"title"`
	Status         string  `json:"status"`
	OptionsJSON    string  `json:"optionsJson"`
	CreatedAt      string  `json:"createdAt"`
	StartedAt      string  `json:"startedAt"`
	FinishedAt     string  `json:"finishedAt"`
	UpdatedAt      string  `json:"updatedAt"`
	TotalItems     int     `json:"totalItems"`
	CompletedItems int     `json:"completedItems"`
	SkippedItems   int     `json:"skippedItems"`
	FailedItems    int     `json:"failedItems"`
	CancelledItems int     `json:"cancelledItems"`
	CurrentItem    string  `json:"currentItem"`
	LastError      string  `json:"lastError"`
	Progress       float64 `json:"progress"`
}

// BackgroundJobItem is one track-sized unit of work inside a background job.
type BackgroundJobItem struct {
	ID         int64  `json:"id"`
	JobID      int64  `json:"jobId"`
	TrackID    int64  `json:"trackId"`
	Path       string `json:"path"`
	Status     string `json:"status"`
	Attempts   int    `json:"attempts"`
	Error      string `json:"error"`
	ResultJSON string `json:"resultJson"`
	CreatedAt  string `json:"createdAt"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	UpdatedAt  string `json:"updatedAt"`
}

// OrganizeRequest controls mp3tag-style template and regex-based renaming.
type OrganizeRequest struct {
	RootDir      string `json:"rootDir"`
	Template     string `json:"template"`
	RegexPattern string `json:"regexPattern"`
	RegexReplace string `json:"regexReplace"`
	Move         bool   `json:"move"`
}

// MetadataSettings controls external metadata providers and their credentials.
// Secrets are stored locally in the CCML user configuration directory.
type MetadataSettings struct {
	MusicBrainzEnabled bool `json:"musicBrainzEnabled"`
	TheAudioDBEnabled  bool `json:"theAudioDBEnabled"`
	DeezerEnabled      bool `json:"deezerEnabled"`
	ITunesEnabled      bool `json:"iTunesEnabled"`
	DiscogsEnabled     bool `json:"discogsEnabled"`
	SpotifyEnabled     bool `json:"spotifyEnabled"`
	AppleMusicEnabled  bool `json:"appleMusicEnabled"`
	YouTubeEnabled     bool `json:"youtubeEnabled"`
	SoundCloudEnabled  bool `json:"soundCloudEnabled"`
	YandexMusicEnabled bool `json:"yandexMusicEnabled"`
	TraxsourceEnabled  bool `json:"traxsourceEnabled"`

	TraxsourceAPIKey string `json:"traxsourceApiKey"`

	// MetadataEnrichmentConcurrency controls how many tracks a background
	// metadata-enrichment job may search/apply concurrently. Provider-specific
	// rate limiters still apply inside each track lookup.
	MetadataEnrichmentConcurrency int `json:"metadataEnrichmentConcurrency"`

	TheAudioDBAPIKey         string `json:"theAudioDBApiKey"`
	ITunesCountry            string `json:"iTunesCountry"`
	DiscogsToken             string `json:"discogsToken"`
	SpotifyAccessToken       string `json:"spotifyAccessToken"`
	SpotifyClientID          string `json:"spotifyClientId"`
	SpotifyClientSecret      string `json:"spotifyClientSecret"`
	SpotifyMarket            string `json:"spotifyMarket"`
	AppleMusicDeveloperToken string `json:"appleMusicDeveloperToken"`
	AppleMusicStorefront     string `json:"appleMusicStorefront"`
	YouTubeAPIKey            string `json:"youTubeApiKey"`
	SoundCloudAccessToken    string `json:"soundCloudAccessToken"`
	YandexMusicToken         string `json:"yandexMusicToken"`
	YandexMusicLanguage      string `json:"yandexMusicLanguage"`
}

// Normalize trims credential fields and applies safe regional defaults.
func (s *MetadataSettings) Normalize() {
	s.TheAudioDBAPIKey = strings.TrimSpace(s.TheAudioDBAPIKey)
	s.ITunesCountry = strings.ToUpper(strings.TrimSpace(s.ITunesCountry))
	s.DiscogsToken = strings.TrimSpace(s.DiscogsToken)
	s.SpotifyAccessToken = strings.TrimSpace(s.SpotifyAccessToken)
	s.SpotifyClientID = strings.TrimSpace(s.SpotifyClientID)
	s.SpotifyClientSecret = strings.TrimSpace(s.SpotifyClientSecret)
	s.SpotifyMarket = strings.ToUpper(strings.TrimSpace(s.SpotifyMarket))
	s.AppleMusicDeveloperToken = strings.TrimSpace(s.AppleMusicDeveloperToken)
	s.AppleMusicStorefront = strings.ToLower(strings.TrimSpace(s.AppleMusicStorefront))
	s.YouTubeAPIKey = strings.TrimSpace(s.YouTubeAPIKey)
	s.SoundCloudAccessToken = strings.TrimSpace(s.SoundCloudAccessToken)
	s.YandexMusicToken = strings.TrimSpace(s.YandexMusicToken)
	s.YandexMusicLanguage = strings.ToLower(strings.TrimSpace(s.YandexMusicLanguage))
	s.TraxsourceAPIKey = strings.TrimSpace(s.TraxsourceAPIKey)

	// Zero migrates settings files created before this option existed.
	if s.MetadataEnrichmentConcurrency == 0 {
		s.MetadataEnrichmentConcurrency = 10
	} else if s.MetadataEnrichmentConcurrency < 1 {
		s.MetadataEnrichmentConcurrency = 1
	} else if s.MetadataEnrichmentConcurrency > 16 {
		s.MetadataEnrichmentConcurrency = 16
	}

	if s.TheAudioDBAPIKey == "" {
		s.TheAudioDBAPIKey = "123"
	}
	if s.ITunesCountry == "" {
		s.ITunesCountry = "US"
	}
	if s.SpotifyMarket == "" {
		s.SpotifyMarket = "US"
	}
	if s.AppleMusicStorefront == "" {
		s.AppleMusicStorefront = "us"
	}
	if s.YandexMusicLanguage == "" {
		s.YandexMusicLanguage = "ru"
	}
}

// SystemStatus reports optional runtime dependencies.
type SystemStatus struct {
	FFmpegPath                string   `json:"ffmpegPath"`
	FFprobePath               string   `json:"ffprobePath"`
	FFmpegVersion             string   `json:"ffmpegVersion"`
	FFmpegSource              string   `json:"ffmpegSource"`
	FFmpegReady               bool     `json:"ffmpegReady"`
	FFmpegUpdating            bool     `json:"ffmpegUpdating"`
	FFmpegUpdateError         string   `json:"ffmpegUpdateError"`
	FFmpegAutoUpdateSupported bool     `json:"ffmpegAutoUpdateSupported"`
	EssentiaPath              string   `json:"essentiaPath"`
	EssentiaReady             bool     `json:"essentiaReady"`
	MetadataProviders         []string `json:"metadataProviders"`
}

// FFmpegUpdateResult reports the result of a managed FFmpeg update.
type FFmpegUpdateResult struct {
	Version     string `json:"version"`
	Changed     bool   `json:"changed"`
	FFmpegPath  string `json:"ffmpegPath"`
	FFprobePath string `json:"ffprobePath"`
	Source      string `json:"source"`
}
