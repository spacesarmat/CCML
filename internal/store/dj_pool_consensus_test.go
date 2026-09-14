package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestDJPoolConsensusPersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	trackID, err := db.UpsertTrack(ctx, model.Track{
		Path: "pool.mp3", FileName: "pool.mp3", Extension: ".mp3",
		Artist: "Artist", Title: "Pool", DurationMS: 180000,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := model.DJPoolConsensus{
		TrackID: trackID, BPM: 128, BPMSupport: 2, BPMQuality: 1.08,
		Camelot: "8A", KeySupport: 2, KeyQuality: 1.05,
	}
	if err := db.PutDJPoolConsensus(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.DJPoolConsensus(ctx, trackID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("consensus not found")
	}
	if got.BPM != want.BPM || got.BPMSupport != 2 || got.Camelot != "8A" || got.KeySupport != 2 || got.UpdatedAt == "" {
		t.Fatalf("consensus = %+v", got)
	}
}
