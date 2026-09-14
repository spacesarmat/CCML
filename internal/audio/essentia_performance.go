package audio

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"context"

	"github.com/spacesarmat/CCML/internal/model"
)

const (
	essentiaPerformanceConfigFileName = "essentia-performance.json"
	defaultEssentiaMode               = "fast"
	defaultEssentiaWorkers            = 2
	defaultEssentiaFastSeconds        = 120
)

func normalizeEssentiaPerformance(config model.EssentiaPerformance) model.EssentiaPerformance {
	switch strings.ToLower(strings.TrimSpace(config.Mode)) {
	case "accurate":
		config.Mode = "accurate"
	default:
		config.Mode = defaultEssentiaMode
	}
	if config.Workers < 1 {
		config.Workers = defaultEssentiaWorkers
	}
	if config.Workers > 4 {
		config.Workers = 4
	}
	if config.FastSeconds < 30 {
		config.FastSeconds = defaultEssentiaFastSeconds
	}
	if config.FastSeconds > 300 {
		config.FastSeconds = 300
	}
	return config
}

func (a *EssentiaAnalyzer) loadPerformance() {
	if a == nil {
		return
	}
	config := model.EssentiaPerformance{
		Mode: defaultEssentiaMode, Workers: defaultEssentiaWorkers, FastSeconds: defaultEssentiaFastSeconds,
	}
	if strings.TrimSpace(a.performancePath) != "" {
		if raw, err := os.ReadFile(a.performancePath); err == nil {
			_ = json.Unmarshal(raw, &config)
		}
	}
	config = normalizeEssentiaPerformance(config)
	a.mu.Lock()
	a.mode = config.Mode
	a.workers = config.Workers
	a.fastSeconds = config.FastSeconds
	a.mu.Unlock()
}

// Performance returns the current local analysis settings.
func (a *EssentiaAnalyzer) Performance() model.EssentiaPerformance {
	if a == nil {
		return normalizeEssentiaPerformance(model.EssentiaPerformance{})
	}
	a.mu.RLock()
	config := model.EssentiaPerformance{Mode: a.mode, Workers: a.workers, FastSeconds: a.fastSeconds}
	a.mu.RUnlock()
	return normalizeEssentiaPerformance(config)
}

// ConfigurePerformance stores Fast/Accurate settings and bounded job concurrency.
func (a *EssentiaAnalyzer) ConfigurePerformance(config model.EssentiaPerformance) (model.EssentiaPerformance, error) {
	if a == nil {
		return model.EssentiaPerformance{}, fmt.Errorf("Essentia analyzer is not available")
	}
	config = normalizeEssentiaPerformance(config)
	if strings.TrimSpace(a.performancePath) != "" {
		raw, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return model.EssentiaPerformance{}, fmt.Errorf("encode Essentia performance config: %w", err)
		}
		if err := os.WriteFile(a.performancePath, raw, 0o600); err != nil {
			return model.EssentiaPerformance{}, fmt.Errorf("save Essentia performance config: %w", err)
		}
	}
	a.mu.Lock()
	a.mode = config.Mode
	a.workers = config.Workers
	a.fastSeconds = config.FastSeconds
	a.mu.Unlock()
	return config, nil
}

func (a *EssentiaAnalyzer) ffprobePath() string {
	if a == nil {
		return ""
	}
	a.mu.RLock()
	tools := a.tools
	a.mu.RUnlock()
	if tools == nil {
		return ""
	}
	return strings.TrimSpace(tools.FFprobePath())
}

// prepareEssentiaFastWAV extracts a central mono PCM segment for long tracks.
// Short tracks keep the direct path because transcoding would not save work.
func (a *EssentiaAnalyzer) prepareEssentiaFastWAV(ctx context.Context, input string) (string, func(), bool, error) {
	performance := a.Performance()
	if performance.Mode != "fast" {
		return "", func() {}, false, nil
	}
	ffmpeg := a.ffmpegPath()
	ffprobe := a.ffprobePath()
	if ffmpeg == "" || ffprobe == "" {
		return "", func() {}, false, nil
	}

	duration, err := essentiaProbeDuration(ctx, ffprobe, input)
	if err != nil {
		return "", func() {}, false, err
	}
	window := float64(performance.FastSeconds)
	if duration <= window+15 {
		return "", func() {}, false, nil
	}
	start := (duration - window) / 2
	if start < 0 {
		start = 0
	}

	tempDir, err := os.MkdirTemp("", "ccml-essentia-fast-*")
	if err != nil {
		return "", func() {}, false, fmt.Errorf("create Essentia fast directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	outputPath := filepath.Join(tempDir, "center.wav")
	cmd := exec.CommandContext(
		ctx, ffmpeg,
		"-v", "warning",
		"-nostdin",
		"-y",
		"-ss", strconv.FormatFloat(start, 'f', 3, 64),
		"-i", input,
		"-t", strconv.Itoa(performance.FastSeconds),
		"-map", "0:a:0",
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		"-vn", "-sn", "-dn",
		"-ac", "1",
		"-ar", "44100",
		"-c:a", "pcm_s16le",
		"-threads", "1",
		outputPath,
	)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		cleanup()
		if ctx.Err() != nil {
			return "", func() {}, false, ctx.Err()
		}
		return "", func() {}, false, fmt.Errorf("prepare Essentia fast WAV: %w: %s", err, tail(string(raw), 2000))
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		cleanup()
		return "", func() {}, false, fmt.Errorf("inspect Essentia fast WAV: %w", err)
	}
	if info.Size() <= 44 {
		cleanup()
		return "", func() {}, false, fmt.Errorf("Essentia fast WAV contains no decoded audio")
	}
	return outputPath, cleanup, true, nil
}

func essentiaProbeDuration(ctx context.Context, ffprobe, input string) (float64, error) {
	cmd := exec.CommandContext(
		ctx, ffprobe,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		input,
	)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("probe audio duration: %w: %s", err, tail(string(raw), 1000))
	}
	value := strings.TrimSpace(string(raw))
	duration, err := strconv.ParseFloat(value, 64)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid ffprobe duration %q", value)
	}
	return duration, nil
}
