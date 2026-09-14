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
	fresh := model.EssentiaAnalysis{AnalyzedAt: modified.Add(time.Minute).Format(time.RFC3339Nano)}
	stale := model.EssentiaAnalysis{AnalyzedAt: modified.Add(-time.Minute).Format(time.RFC3339Nano)}

	if !essentiaAnalysisFreshForTrack(fresh, track) {
		t.Fatal("newer analysis must be fresh")
	}
	if essentiaAnalysisFreshForTrack(stale, track) {
		t.Fatal("older analysis must be stale")
	}
	if essentiaAnalysisFreshForTrack(model.EssentiaAnalysis{AnalyzedAt: "bad"}, track) {
		t.Fatal("invalid timestamp must not be fresh")
	}
	if essentiaAnalysisFreshForTrack(fresh, model.Track{}) {
		t.Fatal("track without modification time must not be cached")
	}
}
