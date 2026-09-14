package audio

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// AdaptiveHint contains previously captured provider-only consensus. Only
// multi-source evidence is strong enough to force Accurate verification.
type AdaptiveHint struct {
	PoolBPM        float64
	PoolBPMSupport int
	PoolCamelot    string
	PoolKeySupport int
}

// AnalysisRun describes how one local analysis was produced.
type AnalysisRun struct {
	Result             model.BPMKey
	RequestedMode      string
	EffectiveMode      string
	Profile            string
	Escalated          bool
	EscalationReason   string
	FastDurationMS     int64
	AccurateDurationMS int64
}

// AnalysisProfile returns a stable cache identity for the current settings and
// the provider hint that can affect Adaptive escalation.
func (a *EssentiaAnalyzer) AnalysisProfile(hint AdaptiveHint) string {
	config := a.Performance()
	switch config.Mode {
	case "accurate":
		return "essentia-v3:accurate"
	case "fast":
		return fmt.Sprintf("essentia-v3:fast:%d", config.FastSeconds)
	default:
		return fmt.Sprintf(
			"essentia-v3:adaptive:%d:%.3f:pool:%.1f/%d:%s/%d",
			config.FastSeconds,
			config.MinKeyStrength,
			hint.PoolBPM,
			hint.PoolBPMSupport,
			strings.ToUpper(strings.TrimSpace(hint.PoolCamelot)),
			hint.PoolKeySupport,
		)
	}
}

// AnalyzeDetailed applies Fast, Accurate, or Adaptive policy and records enough
// telemetry for Jobs and profile-aware caching.
func (a *EssentiaAnalyzer) AnalyzeDetailed(ctx context.Context, input string, hint AdaptiveHint) (AnalysisRun, error) {
	path := a.Path()
	if path == "" {
		return AnalysisRun{}, errors.New("Essentia is not configured; choose essentia_streaming_extractor_music in Settings, set CCML_ESSENTIA, or install it on PATH")
	}
	config := a.Performance()
	run := AnalysisRun{
		RequestedMode: config.Mode,
		EffectiveMode: config.Mode,
		Profile:       a.AnalysisProfile(hint),
	}

	if config.Mode == "accurate" {
		started := time.Now()
		result, err := a.analyzeEssentiaFull(ctx, path, input)
		run.AccurateDurationMS = time.Since(started).Milliseconds()
		run.Result = result
		run.EffectiveMode = "accurate"
		return run, err
	}

	fastPath, cleanup, used, prepErr := a.prepareEssentiaFastWAV(ctx, input)
	if prepErr == nil && used {
		started := time.Now()
		fastResult, _, fastErr := runEssentiaExtractor(ctx, path, fastPath)
		run.FastDurationMS = time.Since(started).Milliseconds()
		cleanup()
		if fastErr == nil {
			fastResult = sanitizeAdaptiveEvidence(fastResult)
			if config.Mode == "fast" {
				run.Result = fastResult
				run.EffectiveMode = "fast"
				return run, nil
			}
			if reason := adaptiveEscalationReason(fastResult, hint, config.MinKeyStrength); reason == "" {
				run.Result = fastResult
				run.EffectiveMode = "fast"
				return run, nil
			} else {
				run.Escalated = true
				run.EscalationReason = reason
			}
		} else {
			if ctx.Err() != nil {
				return AnalysisRun{}, ctx.Err()
			}
			run.Escalated = config.Mode == "adaptive"
			run.EscalationReason = "fast_failed"
		}
	} else if config.Mode == "adaptive" {
		run.Escalated = prepErr != nil
		if prepErr != nil {
			run.EscalationReason = "fast_unavailable"
		}
	}

	started := time.Now()
	result, err := a.analyzeEssentiaFull(ctx, path, input)
	run.AccurateDurationMS = time.Since(started).Milliseconds()
	run.Result = result
	run.EffectiveMode = "accurate"
	return run, err
}

func (a *EssentiaAnalyzer) analyzeEssentiaFull(ctx context.Context, executable, input string) (model.BPMKey, error) {
	result, output, err := runEssentiaExtractor(ctx, executable, input)
	if err == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		return model.BPMKey{}, ctx.Err()
	}
	directErr := fmt.Errorf("run Essentia for %q: %w: %s", filepath.Base(input), err, tail(output, 3000))
	if !essentiaNeedsFFmpegFallback(output) {
		return model.BPMKey{}, directErr
	}

	ffmpeg := a.ffmpegPath()
	if ffmpeg == "" {
		return model.BPMKey{}, fmt.Errorf("%w; CCML FFmpeg compatibility fallback is unavailable", directErr)
	}
	wavPath, cleanup, prepErr := prepareEssentiaFallbackWAV(ctx, ffmpeg, input)
	if prepErr != nil {
		return model.BPMKey{}, fmt.Errorf("%w; prepare FFmpeg compatibility WAV: %v", directErr, prepErr)
	}
	defer cleanup()

	fallback, fallbackOutput, fallbackErr := runEssentiaExtractor(ctx, executable, wavPath)
	if fallbackErr != nil {
		if ctx.Err() != nil {
			return model.BPMKey{}, ctx.Err()
		}
		return model.BPMKey{}, fmt.Errorf(
			"%w; Essentia retry through CCML FFmpeg WAV failed: %v: %s",
			directErr,
			fallbackErr,
			tail(fallbackOutput, 3000),
		)
	}
	return fallback, nil
}

func adaptiveEscalationReason(result model.BPMKey, hint AdaptiveHint, minKeyStrength float64) string {
	result = sanitizeAdaptiveEvidence(result)
	if result.BPM <= 0 {
		return "missing_bpm"
	}
	if strings.TrimSpace(result.Key) == "" || strings.TrimSpace(result.Camelot) == "" {
		return "missing_key"
	}
	if result.Strength < minKeyStrength {
		return "low_key_strength"
	}
	if hint.PoolBPMSupport >= 2 && hint.PoolBPM > 0 {
		if math.Abs(result.BPM-hint.PoolBPM) > 0.5 {
			if math.Abs(result.BPM*2-hint.PoolBPM) <= 1.0 || math.Abs(hint.PoolBPM*2-result.BPM) <= 1.0 {
				return "dj_pool_bpm_half_double"
			}
			return "dj_pool_bpm_conflict"
		}
	}
	if hint.PoolKeySupport >= 2 && strings.TrimSpace(hint.PoolCamelot) != "" {
		if !strings.EqualFold(strings.TrimSpace(result.Camelot), strings.TrimSpace(hint.PoolCamelot)) {
			return "dj_pool_key_conflict"
		}
	}
	return ""
}

func sanitizeAdaptiveEvidence(result model.BPMKey) model.BPMKey {
	if math.IsNaN(result.BPM) || math.IsInf(result.BPM, 0) || result.BPM < 20 || result.BPM > 400 {
		result.BPM = 0
	}
	if math.IsNaN(result.Strength) || math.IsInf(result.Strength, 0) {
		result.Strength = 0
	}
	if result.Strength < 0 {
		result.Strength = 0
	}
	if result.Strength > 1 {
		result.Strength = 1
	}
	result.Key = strings.TrimSpace(result.Key)
	result.Scale = strings.ToLower(strings.TrimSpace(result.Scale))
	if result.Key == "" {
		result.Camelot = ""
		result.OpenKey = ""
		return result
	}
	result.Camelot, result.OpenKey, _ = DJKeyFormats(result.Key, result.Scale)
	return result
}
