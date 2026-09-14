package main

import (
	"math"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestNormalizeEssentiaAnalysisResultClampsAndTrims(t *testing.T) {
	t.Parallel()

	got, err := normalizeEssentiaAnalysisResult(model.BPMKey{
		BPM:      128.25,
		Key:      "  A  ",
		Scale:    " MAJOR ",
		Strength: 1.4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.BPM != 128.25 || got.Key != "A" || got.Scale != "major" || got.Strength != 1 {
		t.Fatalf("normalized result = %+v", got)
	}
}

func TestNormalizeEssentiaAnalysisResultRejectsNonFiniteNumbersSafely(t *testing.T) {
	t.Parallel()

	got, err := normalizeEssentiaAnalysisResult(model.BPMKey{
		BPM:      math.Inf(1),
		Key:      "C",
		Scale:    "minor",
		Strength: math.NaN(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.BPM != 0 || got.Strength != 0 || got.Key != "C" {
		t.Fatalf("non-finite result was not sanitized: %+v", got)
	}
}

func TestNormalizeEssentiaAnalysisResultRequiresUsableEvidence(t *testing.T) {
	t.Parallel()

	if _, err := normalizeEssentiaAnalysisResult(model.BPMKey{BPM: math.Inf(1)}); err == nil {
		t.Fatal("expected unusable Essentia result error")
	}
}
