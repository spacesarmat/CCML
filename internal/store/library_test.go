package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestIncrementalScanStateAndCleanup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	root := filepath.Join(t.TempDir(), "Music")
	firstPath := filepath.Join(root, "Artist", "first.flac")
	secondPath := filepath.Join(root, "Artist", "second.mp3")

	for _, track := range []model.Track{
		{Path: firstPath, FileName: "first.flac", Extension: ".flac", Size: 100, ModifiedUnix: 10, Artist: "Artist", Album: "Album", DurationMS: 1000},
		{Path: secondPath, FileName: "second.mp3", Extension: ".mp3", Size: 200, ModifiedUnix: 20, Artist: "Artist", Album: "Album", DurationMS: 2000},
	} {
		if _, err := db.UpsertTrack(ctx, track); err != nil {
			t.Fatalf("UpsertTrack(%q) error = %v", track.Path, err)
		}
	}
	if err := db.UpsertLibraryRoot(ctx, root); err != nil {
		t.Fatalf("UpsertLibraryRoot() error = %v", err)
	}

	states, err := db.TrackStatesUnderRoot(ctx, root)
	if err != nil {
		t.Fatalf("TrackStatesUnderRoot() error = %v", err)
	}
	if got := states[firstPath]; got.Size != 100 || got.ModifiedUnix != 10 {
		t.Fatalf("first track state = %+v, want size=100 modified=10", got)
	}

	const scanID = "scan-1"
	if err := db.MarkTrackSeen(ctx, firstPath, root, scanID); err != nil {
		t.Fatalf("MarkTrackSeen() error = %v", err)
	}
	removed, err := db.DeleteUnseenTracks(ctx, root, scanID)
	if err != nil {
		t.Fatalf("DeleteUnseenTracks() error = %v", err)
	}
	if removed != 1 {
		t.Fatalf("DeleteUnseenTracks() removed = %d, want 1", removed)
	}

	stats, err := db.LibraryStatistics(ctx)
	if err != nil {
		t.Fatalf("LibraryStatistics() error = %v", err)
	}
	if stats.Tracks != 1 || stats.Artists != 1 || stats.Albums != 1 || stats.DurationMS != 1000 || stats.SizeBytes != 100 {
		t.Fatalf("LibraryStatistics() = %+v", stats)
	}
}
