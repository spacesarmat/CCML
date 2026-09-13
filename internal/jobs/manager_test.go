package jobs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
	"github.com/spacesarmat/CCML/internal/store"
)

func TestManagerCompletesPersistentJob(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var ids []int64
	for _, name := range []string{"one.flac", "two.flac"} {
		id, err := db.UpsertTrack(ctx, model.Track{Path: filepath.Join(t.TempDir(), name), FileName: name, Extension: ".flac"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	job, err := db.CreateBackgroundJob(ctx, "test", "Test", `{}`, ids)
	if err != nil {
		t.Fatal(err)
	}

	manager := New(db)
	manager.Register("test", func(context.Context, model.BackgroundJob, model.BackgroundJobItem) (ItemResult, error) {
		return ItemResult{Status: "completed", ResultJSON: `{}`}, nil
	})
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	manager.Wake()

	final := waitForJob(t, ctx, db, job.ID, func(job model.BackgroundJob) bool { return job.Status == "completed" })
	if final.CompletedItems != 2 || final.Progress != 1 {
		t.Fatalf("unexpected completed job: %+v", final)
	}
}

func TestManagerCancelInterruptsCurrentItem(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	trackID, err := db.UpsertTrack(ctx, model.Track{Path: filepath.Join(t.TempDir(), "cancel.flac"), FileName: "cancel.flac", Extension: ".flac"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.CreateBackgroundJob(ctx, "test", "Test", `{}`, []int64{trackID})
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	manager := New(db)
	manager.Register("test", func(itemCtx context.Context, _ model.BackgroundJob, _ model.BackgroundJobItem) (ItemResult, error) {
		close(started)
		<-itemCtx.Done()
		return ItemResult{}, itemCtx.Err()
	})
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	manager.Wake()

	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("job runner did not start")
	}
	cancelled, err := manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("unexpected cancel result: %+v", cancelled)
	}
	final := waitForJob(t, ctx, db, job.ID, func(job model.BackgroundJob) bool { return job.Status == "cancelled" && job.CancelledItems == 1 })
	if final.Progress != 1 {
		t.Fatalf("expected cancelled job progress 1, got %+v", final)
	}
}

func waitForJob(t *testing.T, ctx context.Context, db *store.Store, jobID int64, ready func(model.BackgroundJob) bool) model.BackgroundJob {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		job, err := db.BackgroundJobByID(ctx, jobID)
		if err != nil {
			t.Fatal(err)
		}
		if ready(job) {
			return job
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for job %d: %+v", jobID, job)
		case <-ticker.C:
		}
	}
}
