package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestTagHistoryLifecycle(t *testing.T) {
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

	trackID, err := db.UpsertTrack(ctx, model.Track{
		Path:         filepath.Join(t.TempDir(), "track.flac"),
		FileName:     "track.flac",
		Extension:    ".flac",
		Title:        "Before",
		Artist:       "Artist",
		TrackNumber:  1,
		TrackTotal:   10,
		DiscNumber:   1,
		DiscTotal:    2,
		Composer:     "Composer",
		Comment:      "Before comment",
		Size:         100,
		ModifiedUnix: 10,
	})
	if err != nil {
		t.Fatalf("UpsertTrack() error = %v", err)
	}

	before := model.TagSnapshot{
		Title: "Before", Artist: "Artist", TrackNumber: 1, TrackTotal: 10,
		DiscNumber: 1, DiscTotal: 2, Composer: "Composer", Comment: "Before comment",
	}
	after := before
	after.Title = "After"
	after.Comment = "After comment"

	if err := db.UpdateTrackTags(ctx, trackID, after, 120, 20); err != nil {
		t.Fatalf("UpdateTrackTags() error = %v", err)
	}
	track, err := db.TrackByID(ctx, trackID)
	if err != nil {
		t.Fatalf("TrackByID() error = %v", err)
	}
	if track.Title != "After" || track.Comment != "After comment" || track.TrackTotal != 10 || track.DiscTotal != 2 {
		t.Fatalf("TrackByID() = %+v, want updated tag fields", track)
	}

	changeSetID, err := db.BeginTagChange(ctx, "tags.edit")
	if err != nil {
		t.Fatalf("BeginTagChange() error = %v", err)
	}
	if err := db.AddTagChangeItem(ctx, changeSetID, trackID, before, after, "", false); err != nil {
		t.Fatalf("AddTagChangeItem() error = %v", err)
	}
	if err := db.FinishTagChange(ctx, changeSetID, 1, "applied"); err != nil {
		t.Fatalf("FinishTagChange() error = %v", err)
	}

	history, err := db.ListTagHistory(ctx, 10)
	if err != nil {
		t.Fatalf("ListTagHistory() error = %v", err)
	}
	if len(history) != 1 || history[0].ID != changeSetID || history[0].AffectedCount != 1 || history[0].Status != "applied" {
		t.Fatalf("ListTagHistory() = %+v, want one applied change", history)
	}

	items, err := db.TagChangeItems(ctx, changeSetID)
	if err != nil {
		t.Fatalf("TagChangeItems() error = %v", err)
	}
	if len(items) != 1 || items[0].Before.Title != "Before" || items[0].After.Title != "After" {
		t.Fatalf("TagChangeItems() = %+v, want persisted before/after", items)
	}

	if err := db.MarkTagChangeUndone(ctx, changeSetID); err != nil {
		t.Fatalf("MarkTagChangeUndone() error = %v", err)
	}
	if _, err := db.TagChangeItems(ctx, changeSetID); err == nil {
		t.Fatal("TagChangeItems() after undo error = nil, want status error")
	}
}
