package audio

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestAdaptiveAcceptsStrongFastEvidence(t *testing.T) {
	t.Parallel()

	result := model.BPMKey{BPM: 128, Key: "A", Scale: "minor", Strength: 0.8, Camelot: "8A"}
	if reason := adaptiveEscalationReason(result, AdaptiveHint{}, 0.62); reason != "" {
		t.Fatalf("unexpected escalation: %s", reason)
	}
}

func TestAdaptiveEscalatesLowKeyConfidence(t *testing.T) {
	t.Parallel()

	result := model.BPMKey{BPM: 128, Key: "A", Scale: "minor", Strength: 0.4, Camelot: "8A"}
	if reason := adaptiveEscalationReason(result, AdaptiveHint{}, 0.62); reason != "low_key_strength" {
		t.Fatalf("reason = %q", reason)
	}
}

func TestAdaptiveEscalatesMultiSourcePoolConflict(t *testing.T) {
	t.Parallel()

	result := model.BPMKey{BPM: 126, Key: "A", Scale: "minor", Strength: 0.8, Camelot: "8A"}
	hint := AdaptiveHint{PoolBPM: 128, PoolBPMSupport: 2, PoolCamelot: "9A", PoolKeySupport: 2}
	if reason := adaptiveEscalationReason(result, hint, 0.62); reason != "dj_pool_bpm_conflict" {
		t.Fatalf("reason = %q", reason)
	}
}

func TestAdaptiveDetectsHalfDoubleTempo(t *testing.T) {
	t.Parallel()

	result := model.BPMKey{BPM: 64, Key: "A", Scale: "minor", Strength: 0.8, Camelot: "8A"}
	hint := AdaptiveHint{PoolBPM: 128, PoolBPMSupport: 2}
	if reason := adaptiveEscalationReason(result, hint, 0.62); reason != "dj_pool_bpm_half_double" {
		t.Fatalf("reason = %q", reason)
	}
}

func TestAnalysisProfileChangesWithAdaptiveInputs(t *testing.T) {
	t.Parallel()

	analyzer := &EssentiaAnalyzer{mode: "adaptive", workers: 2, fastSeconds: 120, minKeyStrength: 0.62}
	base := analyzer.AnalysisProfile(AdaptiveHint{})
	changedThreshold := &EssentiaAnalyzer{mode: "adaptive", workers: 2, fastSeconds: 120, minKeyStrength: 0.70}
	if base == changedThreshold.AnalysisProfile(AdaptiveHint{}) {
		t.Fatal("threshold change must invalidate profile")
	}
	if base == analyzer.AnalysisProfile(AdaptiveHint{PoolBPM: 128, PoolBPMSupport: 2}) {
		t.Fatal("DJ Pool hint change must invalidate adaptive profile")
	}
}
