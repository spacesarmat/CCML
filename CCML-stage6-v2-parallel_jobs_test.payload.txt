package jobs

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
	"github.com/spacesarmat/CCML/internal/store"
)

func TestManagerConcurrentRunnerProcessesMultipleItemsAtOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var ids []int64
	root := t.TempDir()
	for _, name := range []string{"one.flac", "two.flac", "three.flac", "four.flac", "five.flac", "six.flac"} {
		id, err := db.UpsertTrack(ctx, model.Track{
			Path:      filepath.Join(root, name),
			FileName:  name,
			Extension: ".flac",
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}

	job, err := db.CreateBackgroundJob(ctx, "parallel-test", "Parallel test", `{}`, ids)
	if err != nil {
		t.Fatal(err)
	}

	var active int32
	var peak int32
	manager := New(db)
	manager.RegisterConcurrent("parallel-test", 3, func(context.Context, model.BackgroundJob, model.BackgroundJobItem) (ItemResult, error) {
		now := atomic.AddInt32(&active, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if now <= old || atomic.CompareAndSwapInt32(&peak, old, now) {
				break
			}
		}
		time.Sleep(80 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return ItemResult{Status: "completed", ResultJSON: `{}`}, nil
	})

	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	manager.Wake()

	final := waitForJob(t, ctx, db, job.ID, func(job model.BackgroundJob) bool {
		return job.Status == "completed"
	})
	if final.CompletedItems != len(ids) {
		t.Fatalf("completed = %d, want %d", final.CompletedItems, len(ids))
	}
	if got := atomic.LoadInt32(&peak); got < 2 {
		t.Fatalf("peak parallelism = %d, want at least 2", got)
	}
	if got := atomic.LoadInt32(&peak); got > 3 {
		t.Fatalf("peak parallelism = %d, want at most 3", got)
	}
}
