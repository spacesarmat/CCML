package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestEssentiaAnalysisPersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	trackID, err := db.UpsertTrack(ctx, model.Track{
		Path: "test.mp3", FileName: "test.mp3", Extension: ".mp3",
		Artist: "Artist", Title: "Title", DurationMS: 180000,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.PutEssentiaAnalysisRun(ctx, trackID, model.BPMKey{
		BPM: 128.2, Key: "A", Scale: "minor", Strength: 0.84,
	}, "essentia-v3:adaptive:test", "adaptive", "fast"); err != nil {
		t.Fatal(err)
	}

	got, ok, err := db.EssentiaAnalysis(ctx, trackID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Essentia analysis not found")
	}
	if got.TrackID != trackID || got.BPM != 128.2 || got.Key != "A" || got.Scale != "minor" || got.Strength != 0.84 ||
		got.Profile != "essentia-v3:adaptive:test" || got.RequestedMode != "adaptive" || got.EffectiveMode != "fast" || got.AnalyzedAt == "" {
		t.Fatalf("analysis = %+v", got)
	}
}

func TestLegacyEssentiaJobAnalysisDecodesPreviousStageResult(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	trackID, err := db.UpsertTrack(ctx, model.Track{
		Path: "legacy.mp3", FileName: "legacy.mp3", Extension: ".mp3",
		Artist: "Artist", Title: "Legacy", DurationMS: 180000,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := "2026-09-14T19:00:00Z"
	res, err := db.db.ExecContext(ctx, `
INSERT INTO background_jobs(type, title, status, options_json, created_at, updated_at, total_items)
VALUES ('essentia_analysis', 'legacy', 'completed', '{}', ?, ?, 1)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	jobID, _ := res.LastInsertId()
	_, err = db.db.ExecContext(ctx, `
INSERT INTO background_job_items(job_id, track_id, path, status, result_json, created_at, updated_at)
VALUES (?, ?, 'legacy.mp3', 'completed', ?, ?, ?)`,
		jobID, trackID, `{"bpm":126.5,"key":"F#","scale":"minor","strength":0.72}`, now, now)
	if err != nil {
		t.Fatal(err)
	}

	got, ok, err := db.EssentiaAnalysis(ctx, trackID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.BPM != 126.5 || got.Key != "F#" || got.Scale != "minor" || got.Strength != 0.72 {
		t.Fatalf("legacy analysis = %+v, ok=%v", got, ok)
	}
}
