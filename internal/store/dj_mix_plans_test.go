package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestDJMixPlanPersistenceLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	saved, err := db.SaveDJMixPlan(ctx, model.DJMixSavedPlan{
		Name:          "Friday Set",
		ScopeTrackIDs: []int64{3, 2, 3, 0},
		Options: model.DJMixPlanOptions{
			StartTrackID: 3,
			Limit:        20,
			Lookahead:    3,
			PinnedTracks: []model.DJMixPin{{TrackID: 2, Position: 2}},
		},
		Plan: model.DJMixPlan{
			StartTrackID: 3,
			Lookahead:    3,
			Steps: []model.DJMixPlanStep{
				{Position: 1, Track: model.Track{ID: 3, Path: "a.mp3", Artist: "A", Title: "One", BPM: 128}},
				{Position: 2, Track: model.Track{ID: 2, Path: "b.mp3", Artist: "B", Title: "Two", BPM: 129}, Pinned: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID <= 0 || saved.CreatedAt == "" || saved.UpdatedAt == "" {
		t.Fatalf("saved = %+v", saved)
	}
	if len(saved.ScopeTrackIDs) != 2 || saved.ScopeTrackIDs[0] != 3 || saved.ScopeTrackIDs[1] != 2 {
		t.Fatalf("scope = %#v", saved.ScopeTrackIDs)
	}

	loaded, err := db.DJMixPlanByID(ctx, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "Friday Set" || len(loaded.Plan.Steps) != 2 || loaded.Options.Lookahead != 3 {
		t.Fatalf("loaded = %+v", loaded)
	}

	loaded.Name = "Saturday Set"
	updated, err := db.SaveDJMixPlan(ctx, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != saved.ID || updated.Name != "Saturday Set" || updated.CreatedAt != saved.CreatedAt {
		t.Fatalf("updated = %+v", updated)
	}

	list, err := db.ListDJMixPlans(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("list = %+v", list)
	}

	if err := db.DeleteDJMixPlan(ctx, saved.ID); err != nil {
		t.Fatal(err)
	}
	list, err = db.ListDJMixPlans(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("list after delete = %+v", list)
	}
}
