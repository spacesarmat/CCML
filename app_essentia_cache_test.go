package main

import (
	"testing"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestEssentiaAnalysisFreshForTrack(t *testing.T) {
	t.Parallel()

	modified := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	track := model.Track{ModifiedUnix: modified.Unix()}
	const profile = "essentia-v3:adaptive:test"
	fresh := model.EssentiaAnalysis{Profile: profile, AnalyzedAt: modified.Add(time.Minute).Format(time.RFC3339Nano)}
	stale := model.EssentiaAnalysis{Profile: profile, AnalyzedAt: modified.Add(-time.Minute).Format(time.RFC3339Nano)}

	if !essentiaAnalysisFreshForTrack(fresh, track, profile) {
		t.Fatal("newer analysis with matching profile must be fresh")
	}
	if essentiaAnalysisFreshForTrack(stale, track, profile) {
		t.Fatal("older analysis must be stale")
	}
	if essentiaAnalysisFreshForTrack(model.EssentiaAnalysis{Profile: profile, AnalyzedAt: "bad"}, track, profile) {
		t.Fatal("invalid timestamp must not be fresh")
	}
	if essentiaAnalysisFreshForTrack(fresh, model.Track{}, profile) {
		t.Fatal("track without modification time must not be cached")
	}
	if essentiaAnalysisFreshForTrack(fresh, track, "different-profile") {
		t.Fatal("analysis from a different profile must not be reused")
	}
}
