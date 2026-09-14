package audio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

// analyzeEssentiaFastLean first asks the music extractor to perform its own
// slice using a lean YAML profile. This avoids decoding a PCM WAV up front.
// Older/incompatible extractor builds transparently fall back to the Stage 19.4
// FFmpeg center-window path.
func (a *EssentiaAnalyzer) analyzeEssentiaFastLean(
	ctx context.Context,
	executable, input string,
) (model.BPMKey, string, bool, error) {
	profilePath, cleanupProfile, profileErr := a.prepareEssentiaLeanProfile(ctx, input, true)
	if profileErr == nil {
		result, _, err := runEssentiaExtractor(ctx, executable, input, profilePath)
		cleanupProfile()
		if err == nil {
			return result, "profile", true, nil
		}
		if ctx.Err() != nil {
			return model.BPMKey{}, "profile", false, ctx.Err()
		}
	} else {
		cleanupProfile()
	}

	fastPath, cleanupWAV, used, prepErr := a.prepareEssentiaFastWAV(ctx, input)
	if prepErr != nil || !used {
		if prepErr != nil {
			return model.BPMKey{}, "ffmpeg", false, prepErr
		}
		if profileErr != nil {
			return model.BPMKey{}, "profile", false, profileErr
		}
		return model.BPMKey{}, "direct", false, nil
	}
	defer cleanupWAV()

	result, _, err := runEssentiaExtractor(ctx, executable, fastPath)
	if err != nil {
		if ctx.Err() != nil {
			return model.BPMKey{}, "ffmpeg", false, ctx.Err()
		}
		return model.BPMKey{}, "ffmpeg", false, err
	}
	return result, "ffmpeg", true, nil
}

// analyzeEssentiaAccurateLean keeps the entire track but removes extractor work
// CCML never consumes. Any profile incompatibility falls back to the pre-19.6
// full extractor path, including Hotfix 19.2.1 decoder recovery.
func (a *EssentiaAnalyzer) analyzeEssentiaAccurateLean(
	ctx context.Context,
	executable, input string,
) (model.BPMKey, string, error) {
	profilePath, cleanup, profileErr := writeEssentiaLeanProfile(nil, nil)
	if profileErr == nil {
		result, _, err := runEssentiaExtractor(ctx, executable, input, profilePath)
		cleanup()
		if err == nil {
			return result, "profile", nil
		}
		if ctx.Err() != nil {
			return model.BPMKey{}, "profile", ctx.Err()
		}
	} else {
		cleanup()
	}

	result, err := a.analyzeEssentiaFull(ctx, executable, input)
	return result, "direct", err
}

// prepareEssentiaLeanProfile builds a per-track profile for the Fast center
// window. The official music extractor CLI accepts profile as its third
// argument: input, output, profile.
func (a *EssentiaAnalyzer) prepareEssentiaLeanProfile(
	ctx context.Context,
	input string,
	fast bool,
) (string, func(), error) {
	if !fast {
		return writeEssentiaLeanProfile(nil, nil)
	}
	performance := a.Performance()
	ffprobe := a.ffprobePath()
	if ffprobe == "" {
		return "", func() {}, fmt.Errorf("ffprobe is unavailable for Essentia Fast slicing")
	}
	duration, err := essentiaProbeDuration(ctx, ffprobe, input)
	if err != nil {
		return "", func() {}, err
	}

	window := float64(performance.FastSeconds)
	start := 0.0
	end := duration
	if duration > window+15 {
		start = (duration - window) / 2
		end = start + window
	}
	return writeEssentiaLeanProfile(&start, &end)
}

func writeEssentiaLeanProfile(start, end *float64) (string, func(), error) {
	tempDir, err := os.MkdirTemp("", "ccml-essentia-profile-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create Essentia profile directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	profilePath := filepath.Join(tempDir, "bpm-key.yaml")
	content := essentiaLeanProfileYAML(start, end)
	if err := os.WriteFile(profilePath, []byte(content), 0o600); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("write Essentia profile: %w", err)
	}
	return profilePath, cleanup, nil
}

func essentiaLeanProfileYAML(start, end *float64) string {
	var b strings.Builder
	b.WriteString("outputFormat: json\n")
	b.WriteString("outputFrames: 0\n")
	b.WriteString("requireMbid: false\n")
	b.WriteString("indent: 0\n")
	b.WriteString("analysisSampleRate: 44100.0\n")
	if start != nil && end != nil {
		b.WriteString("startTime: ")
		b.WriteString(strconv.FormatFloat(*start, 'f', 3, 64))
		b.WriteString("\nendTime: ")
		b.WriteString(strconv.FormatFloat(*end, 'f', 3, 64))
		b.WriteString("\n")
	}
	b.WriteString(`
lowlevel:
  frameSize: 4096
  hopSize: 4096
  zeroPadding: 0
  windowType: blackmanharris62
  silentFrames: noise
  stats: ["mean"]

average_loudness:
  frameSize: 88200
  hopSize: 44100
  windowType: hann
  silentFrames: noise

rhythm:
  method: degara
  minTempo: 40
  maxTempo: 208
  stats: ["mean"]

tonal:
  frameSize: 4096
  hopSize: 2048
  zeroPadding: 0
  windowType: blackmanharris62
  silentFrames: noise
  stats: ["mean"]

highlevel:
  compute: 0
`)
	return b.String()
}
