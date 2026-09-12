// Package model contains shared domain types used by backend services and Wails bindings.
package model

import "time"

// Track represents one indexed audio file.
type Track struct {
	ID           int64   `json:"id"`
	Path         string  `json:"path"`
	FileName     string  `json:"fileName"`
	Extension    string  `json:"extension"`
	Size         int64   `json:"size"`
	ModifiedUnix int64   `json:"modifiedUnix"`
	Title        string  `json:"title"`
	Artist       string  `json:"artist"`
	Album        string  `json:"album"`
	AlbumArtist  string  `json:"albumArtist"`
	Genre        string  `json:"genre"`
	Year         int     `json:"year"`
	TrackNumber  int     `json:"trackNumber"`
	TrackTotal   int     `json:"trackTotal"`
	DiscNumber   int     `json:"discNumber"`
	DiscTotal    int     `json:"discTotal"`
	Composer     string  `json:"composer"`
	Comment      string  `json:"comment"`
	DurationMS   int64   `json:"durationMs"`
	Codec        string  `json:"codec"`
	SampleRate   int     `json:"sampleRate"`
	Channels     int     `json:"channels"`
	BitRate      int64   `json:"bitRate"`
	BPM          float64 `json:"bpm"`
	Key          string  `json:"key"`
	KeyScale     string  `json:"keyScale"`
	LoudnessI    float64 `json:"loudnessI"`
	TruePeak     float64 `json:"truePeak"`
	LRA          float64 `json:"lra"`
	Threshold    float64 `json:"threshold"`
	ScanError    string  `json:"scanError"`
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
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AlbumArtist string `json:"albumArtist"`
	Genre       string `json:"genre"`
	Composer    string `json:"composer"`
	Comment     string `json:"comment"`
	Year        int    `json:"year"`
	TrackNumber int    `json:"trackNumber"`
	TrackTotal  int    `json:"trackTotal"`
	DiscNumber  int    `json:"discNumber"`
	DiscTotal   int    `json:"discTotal"`
	CoverMIME   string `json:"coverMime"`
	CoverSize   int    `json:"coverSize"`
}

// TagPatch describes a partial metadata edit. Fields lists the values that
// should actually be changed, which makes empty-string clearing and batch edits unambiguous.
type TagPatch struct {
	Fields      []string `json:"fields"`
	Title       string   `json:"title"`
	Artist      string   `json:"artist"`
	Album       string   `json:"album"`
	AlbumArtist string   `json:"albumArtist"`
	Genre       string   `json:"genre"`
	Composer    string   `json:"composer"`
	Comment     string   `json:"comment"`
	Year        int      `json:"year"`
	TrackNumber int      `json:"trackNumber"`
	TrackTotal  int      `json:"trackTotal"`
	DiscNumber  int      `json:"discNumber"`
	DiscTotal   int      `json:"discTotal"`
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
}

// MetadataCandidate is a normalized result from an external metadata provider.
type MetadataCandidate struct {
	Source     string  `json:"source"`
	ExternalID string  `json:"externalId"`
	Title      string  `json:"title"`
	Artist     string  `json:"artist"`
	Album      string  `json:"album"`
	Year       int     `json:"year"`
	Genre      string  `json:"genre"`
	ArtworkURL string  `json:"artworkUrl"`
	DurationMS int64   `json:"durationMs"`
	Confidence float64 `json:"confidence"`
}

// MetadataLookupResult contains provider results plus non-fatal provider warnings.
type MetadataLookupResult struct {
	Candidates []MetadataCandidate `json:"candidates"`
	Warnings   []string            `json:"warnings"`
}

// OrganizeRequest controls mp3tag-style template and regex-based renaming.
type OrganizeRequest struct {
	RootDir      string `json:"rootDir"`
	Template     string `json:"template"`
	RegexPattern string `json:"regexPattern"`
	RegexReplace string `json:"regexReplace"`
	Move         bool   `json:"move"`
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
