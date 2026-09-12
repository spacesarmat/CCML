package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

// Probe extracts technical information and embedded metadata using ffprobe.
type Probe struct {
	tools *Toolchain
}

// NewProbe creates an ffprobe metadata reader.
func NewProbe(tools *Toolchain) *Probe {
	return &Probe{tools: tools}
}

type probeOutput struct {
	Streams []struct {
		CodecType  string            `json:"codec_type"`
		CodecName  string            `json:"codec_name"`
		SampleRate string            `json:"sample_rate"`
		Channels   int               `json:"channels"`
		BitRate    string            `json:"bit_rate"`
		Duration   string            `json:"duration"`
		Tags       map[string]string `json:"tags"`
	} `json:"streams"`
	Format struct {
		Duration string            `json:"duration"`
		BitRate  string            `json:"bit_rate"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
}

// Read returns metadata for a supported audio file.
func (p *Probe) Read(ctx context.Context, path string) (model.Track, error) {
	if p.tools == nil || p.tools.FFprobePath() == "" {
		return model.Track{}, errors.New("ffprobe is not available")
	}

	cmd := exec.CommandContext(ctx, p.tools.FFprobePath(),
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return model.Track{}, fmt.Errorf("ffprobe %q: %w: %s", path, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return model.Track{}, fmt.Errorf("ffprobe %q: %w", path, err)
	}

	var parsed probeOutput
	if err := json.Unmarshal(output, &parsed); err != nil {
		return model.Track{}, fmt.Errorf("decode ffprobe JSON for %q: %w", path, err)
	}

	var audioStream *struct {
		CodecType  string            `json:"codec_type"`
		CodecName  string            `json:"codec_name"`
		SampleRate string            `json:"sample_rate"`
		Channels   int               `json:"channels"`
		BitRate    string            `json:"bit_rate"`
		Duration   string            `json:"duration"`
		Tags       map[string]string `json:"tags"`
	}
	for i := range parsed.Streams {
		if parsed.Streams[i].CodecType == "audio" {
			audioStream = &parsed.Streams[i]
			break
		}
	}
	if audioStream == nil {
		return model.Track{}, fmt.Errorf("no audio stream found in %q", path)
	}

	tags := mergeTags(parsed.Format.Tags, audioStream.Tags)
	title := tagValue(tags, "title")
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	duration := parseSecondsMS(parsed.Format.Duration)
	if duration == 0 {
		duration = parseSecondsMS(audioStream.Duration)
	}
	bitRate := parseInt64(audioStream.BitRate)
	if bitRate == 0 {
		bitRate = parseInt64(parsed.Format.BitRate)
	}

	return model.Track{
		Path:        path,
		FileName:    filepath.Base(path),
		Extension:   strings.ToLower(filepath.Ext(path)),
		Title:       title,
		Artist:      tagValue(tags, "artist"),
		Album:       tagValue(tags, "album"),
		AlbumArtist: firstTag(tags, "album_artist", "albumartist", "album artist"),
		Genre:       tagValue(tags, "genre"),
		Composer:    tagValue(tags, "composer"),
		Comment:     firstTag(tags, "comment", "description"),
		Year:        parseYear(firstTag(tags, "date", "year")),
		TrackNumber: parsePairPart(firstTag(tags, "track", "tracknumber"), 0),
		TrackTotal:  parsePairPart(firstTag(tags, "track", "tracknumber"), 1),
		DiscNumber:  parsePairPart(firstTag(tags, "disc", "discnumber"), 0),
		DiscTotal:   parsePairPart(firstTag(tags, "disc", "discnumber"), 1),
		DurationMS:  duration,
		Codec:       audioStream.CodecName,
		SampleRate:  int(parseInt64(audioStream.SampleRate)),
		Channels:    audioStream.Channels,
		BitRate:     bitRate,
	}, nil
}

func mergeTags(formatTags, streamTags map[string]string) map[string]string {
	out := make(map[string]string, len(formatTags)+len(streamTags))
	for key, value := range formatTags {
		out[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	for key, value := range streamTags {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if _, exists := out[normalized]; !exists {
			out[normalized] = strings.TrimSpace(value)
		}
	}
	return out
}

func tagValue(tags map[string]string, key string) string {
	return strings.TrimSpace(tags[strings.ToLower(key)])
}

func firstTag(tags map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := tagValue(tags, key); value != "" {
			return value
		}
	}
	return ""
}

func parseSecondsMS(raw string) int64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value <= 0 {
		return 0
	}
	return int64(value*1000 + 0.5)
}

func parseInt64(raw string) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func parseYear(raw string) int {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 4 {
		raw = raw[:4]
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1000 || value > 9999 {
		return 0
	}
	return value
}

func parsePairPart(raw string, part int) int {
	raw = strings.TrimSpace(raw)
	parts := strings.SplitN(raw, "/", 2)
	if part < 0 || part >= len(parts) {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(parts[part]))
	if err != nil || value < 0 {
		return 0
	}
	return value
}
