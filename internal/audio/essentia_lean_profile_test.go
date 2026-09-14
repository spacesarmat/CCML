package audio

import (
	"strings"
	"testing"
)

func TestEssentiaLeanProfileFastSlice(t *testing.T) {
	t.Parallel()

	start := 90.25
	end := 210.25
	profile := essentiaLeanProfileYAML(&start, &end)
	for _, want := range []string{
		"outputFormat: json",
		"outputFrames: 0",
		"startTime: 90.250",
		"endTime: 210.250",
		"highlevel:\n  compute: 0",
		"lowlevel:",
		`stats: ["mean"]`,
		"rhythm:",
		"tonal:",
	} {
		if !strings.Contains(profile, want) {
			t.Fatalf("profile missing %q:\n%s", want, profile)
		}
	}
}

func TestEssentiaLeanProfileAccurateHasNoSlice(t *testing.T) {
	t.Parallel()

	profile := essentiaLeanProfileYAML(nil, nil)
	if strings.Contains(profile, "startTime:") || strings.Contains(profile, "endTime:") {
		t.Fatalf("full-track profile unexpectedly contains a slice:\n%s", profile)
	}
}

func TestEssentiaLeanProfileCacheVersion(t *testing.T) {
	t.Parallel()

	analyzer := &EssentiaAnalyzer{
		mode: "adaptive", workers: 2, fastSeconds: 120, minKeyStrength: 0.62,
	}
	profile := analyzer.AnalysisProfile(AdaptiveHint{})
	if !strings.HasPrefix(profile, "essentia-v4:lean-profile:adaptive:") {
		t.Fatalf("profile = %q", profile)
	}
}
