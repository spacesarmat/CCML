package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestTrackCoverPresenceRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "cover-index.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	root := filepath.Join(t.TempDir(), "Music")
	path := filepath.Join(root, "cover.mp3")
	id, err := db.UpsertTrack(ctx, model.Track{
		Path:         path,
		FileName:     "cover.mp3",
		Extension:    ".mp3",
		Size:         100,
		ModifiedUnix: 10,
		Title:        "Cover",
	})
	if err != nil {
		t.Fatalf("UpsertTrack() error = %v", err)
	}

	if err := db.UpdateTrackCoverPresence(ctx, id, true); err != nil {
		t.Fatalf("UpdateTrackCoverPresence(true) error = %v", err)
	}

	got, err := db.TrackByID(ctx, id)
	if err != nil {
		t.Fatalf("TrackByID() error = %v", err)
	}
	if !got.HasCover || !got.CoverIndexed {
		t.Fatalf("cover state = has:%v indexed:%v, want true/true", got.HasCover, got.CoverIndexed)
	}

	states, err := db.TrackStatesUnderRoot(ctx, root)
	if err != nil {
		t.Fatalf("TrackStatesUnderRoot() error = %v", err)
	}
	if state, ok := states[path]; !ok || !state.CoverIndexed {
		t.Fatalf("scan state = %+v, exists=%v, want indexed", state, ok)
	}

	if err := db.UpdateTrackCoverPresence(ctx, id, false); err != nil {
		t.Fatalf("UpdateTrackCoverPresence(false) error = %v", err)
	}
	got, err = db.TrackByID(ctx, id)
	if err != nil {
		t.Fatalf("TrackByID() after false error = %v", err)
	}
	if got.HasCover || !got.CoverIndexed {
		t.Fatalf("after false = has:%v indexed:%v, want false/true", got.HasCover, got.CoverIndexed)
	}

	if err := db.InvalidateTrackCoverPresence(ctx, id); err != nil {
		t.Fatalf("InvalidateTrackCoverPresence() error = %v", err)
	}
	got, err = db.TrackByID(ctx, id)
	if err != nil {
		t.Fatalf("TrackByID() after invalidate error = %v", err)
	}
	if got.CoverIndexed {
		t.Fatalf("after invalidate indexed = true, want false")
	}
}
