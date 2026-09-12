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
	DiscNumber   int     `json:"discNumber"`
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

// ScanResult summarizes a folder scan.
type ScanResult struct {
	Root     string        `json:"root"`
	Found    int           `json:"found"`
	Indexed  int           `json:"indexed"`
	Failed   int           `json:"failed"`
	Duration time.Duration `json:"duration"`
	Errors   []string      `json:"errors"`
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
	FFmpegPath        string   `json:"ffmpegPath"`
	FFprobePath       string   `json:"ffprobePath"`
	EssentiaPath      string   `json:"essentiaPath"`
	FFmpegReady       bool     `json:"ffmpegReady"`
	EssentiaReady     bool     `json:"essentiaReady"`
	MetadataProviders []string `json:"metadataProviders"`
}
