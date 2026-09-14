package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestBackgroundJobLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	trackID, err := db.UpsertTrack(ctx, model.Track{Path: filepath.Join(t.TempDir(), "song.flac"), FileName: "song.flac", Extension: ".flac"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.CreateBackgroundJob(ctx, "metadata_enrichment", "Metadata enrichment", `{}`, []int64{trackID})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "queued" || job.TotalItems != 1 {
		t.Fatalf("unexpected new job: %+v", job)
	}
	if err := db.MarkBackgroundJobRunning(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	item, ok, err := db.NextQueuedBackgroundJobItem(ctx, job.ID)
	if err != nil || !ok {
		t.Fatalf("next item: ok=%v err=%v", ok, err)
	}
	if err := db.MarkBackgroundJobItemRunning(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
	job, err = db.FinishBackgroundJobItem(ctx, item.ID, "completed", "", `{"applied":true}`)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "completed" || job.CompletedItems != 1 || job.Progress != 1 {
		t.Fatalf("unexpected completed job: %+v", job)
	}
}

func TestRecoverInterruptedBackgroundJob(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	trackID, err := db.UpsertTrack(ctx, model.Track{Path: filepath.Join(t.TempDir(), "song.mp3"), FileName: "song.mp3", Extension: ".mp3"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.CreateBackgroundJob(ctx, "metadata_enrichment", "Metadata enrichment", `{}`, []int64{trackID})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.MarkBackgroundJobRunning(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	item, ok, err := db.NextQueuedBackgroundJobItem(ctx, job.ID)
	if err != nil || !ok {
		t.Fatalf("next item: ok=%v err=%v", ok, err)
	}
	if err := db.MarkBackgroundJobItemRunning(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.RecoverInterruptedBackgroundJobs(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := db.BackgroundJobByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != "queued" {
		t.Fatalf("expected queued after recovery, got %q", recovered.Status)
	}
	item, ok, err = db.NextQueuedBackgroundJobItem(ctx, job.ID)
	if err != nil || !ok || item.Status != "queued" {
		t.Fatalf("expected queued recovered item: item=%+v ok=%v err=%v", item, ok, err)
	}
}

func TestPauseResumeAndRetryBackgroundJob(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	trackID, err := db.UpsertTrack(ctx, model.Track{Path: filepath.Join(t.TempDir(), "song.ogg"), FileName: "song.ogg", Extension: ".ogg"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.CreateBackgroundJob(ctx, "metadata_enrichment", "Metadata enrichment", `{}`, []int64{trackID})
	if err != nil {
		t.Fatal(err)
	}
	job, err = db.PauseBackgroundJob(ctx, job.ID)
	if err != nil || job.Status != "paused" {
		t.Fatalf("pause: %+v err=%v", job, err)
	}
	job, err = db.ResumeBackgroundJob(ctx, job.ID)
	if err != nil || job.Status != "queued" {
		t.Fatalf("resume: %+v err=%v", job, err)
	}
	if err := db.MarkBackgroundJobRunning(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	item, ok, err := db.NextQueuedBackgroundJobItem(ctx, job.ID)
	if err != nil || !ok {
		t.Fatal("missing job item")
	}
	if err := db.MarkBackgroundJobItemRunning(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
	job, err = db.FinishBackgroundJobItem(ctx, item.ID, "failed", "temporary failure", "")
	if err != nil || job.Status != "failed" {
		t.Fatalf("failed job: %+v err=%v", job, err)
	}
	job, err = db.RetryFailedBackgroundJob(ctx, job.ID)
	if err != nil || job.Status != "queued" {
		t.Fatalf("retry: %+v err=%v", job, err)
	}
}

func TestCancelBackgroundJobCountsCancelledItems(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var ids []int64
	for i := 0; i < 3; i++ {
		id, err := db.UpsertTrack(ctx, model.Track{Path: filepath.Join(t.TempDir(), "cancel-"+string(rune('a'+i))+".flac"), FileName: "cancel.flac", Extension: ".flac"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	job, err := db.CreateBackgroundJob(ctx, "metadata_enrichment", "Metadata enrichment", `{}`, ids)
	if err != nil {
		t.Fatal(err)
	}
	job, err = db.CancelBackgroundJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "cancelled" || job.CancelledItems != 3 || job.Progress != 1 {
		t.Fatalf("unexpected cancelled job: %+v", job)
	}
}

func TestAllTrackIDs(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, name := range []string{"a.mp3", "b.flac", "c.m4a"} {
		if _, err := db.UpsertTrack(ctx, model.Track{Path: filepath.Join(t.TempDir(), name), FileName: name, Extension: filepath.Ext(name)}); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := db.AllTrackIDs(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] >= ids[1] || ids[1] >= ids[2] {
		t.Fatalf("unexpected track ids: %v", ids)
	}
	if _, err := db.AllTrackIDs(ctx, 2); err == nil {
		t.Fatal("expected max-items error")
	}
}

func TestRecoverCancelledRunningItemAsCancelled(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	trackID, err := db.UpsertTrack(ctx, model.Track{Path: filepath.Join(t.TempDir(), "cancel-recovery.flac"), FileName: "cancel-recovery.flac", Extension: ".flac"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.CreateBackgroundJob(ctx, "metadata_enrichment", "Metadata enrichment", `{}`, []int64{trackID})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.MarkBackgroundJobRunning(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	item, ok, err := db.NextQueuedBackgroundJobItem(ctx, job.ID)
	if err != nil || !ok {
		t.Fatalf("next item: ok=%v err=%v", ok, err)
	}
	if err := db.MarkBackgroundJobItemRunning(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CancelBackgroundJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.RecoverInterruptedBackgroundJobs(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := db.ListBackgroundJobItems(ctx, job.ID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != "cancelled" {
		t.Fatalf("expected cancelled recovered item, got %+v", items)
	}
	final, err := db.RecomputeBackgroundJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != "cancelled" || final.CancelledItems != 1 || final.Progress != 1 {
		t.Fatalf("unexpected recovered cancelled job: %+v", final)
	}
}

func TestCreateBackgroundJobForLibrary(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, name := range []string{"library-a.mp3", "library-b.flac", "library-c.m4a"} {
		if _, err := db.UpsertTrack(ctx, model.Track{Path: filepath.Join(t.TempDir(), name), FileName: name, Extension: filepath.Ext(name)}); err != nil {
			t.Fatal(err)
		}
	}
	job, err := db.CreateBackgroundJobForLibrary(ctx, "metadata_enrichment", "Whole library", `{}`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if job.TotalItems != 3 || job.Status != "queued" {
		t.Fatalf("unexpected whole-library job: %+v", job)
	}
	items, err := db.ListBackgroundJobItems(ctx, job.ID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 job items, got %d", len(items))
	}
	if _, err := db.CreateBackgroundJobForLibrary(ctx, "metadata_enrichment", "Too large", `{}`, 2); err == nil {
		t.Fatal("expected max-items error")
	}
}

func TestTrackCarriesLatestMetadataEnrichmentStatus(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	trackID, err := db.UpsertTrack(ctx, model.Track{Path: filepath.Join(t.TempDir(), "status.flac"), FileName: "status.flac", Extension: ".flac"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.CreateBackgroundJob(ctx, "metadata_enrichment", "Metadata enrichment", `{}`, []int64{trackID})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.MarkBackgroundJobRunning(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	item, ok, err := db.NextQueuedBackgroundJobItem(ctx, job.ID)
	if err != nil || !ok {
		t.Fatalf("next item: ok=%v err=%v", ok, err)
	}
	if err := db.MarkBackgroundJobItemRunning(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.FinishBackgroundJobItem(ctx, item.ID, "skipped", "", `{"skipped":true}`); err != nil {
		t.Fatal(err)
	}

	track, err := db.TrackByID(ctx, trackID)
	if err != nil {
		t.Fatal(err)
	}
	if track.LastMetadataJobStatus != "skipped" || track.LastMetadataJobUpdatedAt == "" {
		t.Fatalf("unexpected latest enrichment status: %+v", track)
	}

	newJob, err := db.CreateBackgroundJob(ctx, "metadata_enrichment", "Metadata enrichment", `{}`, []int64{trackID})
	if err != nil {
		t.Fatal(err)
	}
	track, err = db.TrackByID(ctx, trackID)
	if err != nil {
		t.Fatal(err)
	}
	if track.LastMetadataJobStatus != "queued" {
		t.Fatalf("expected newest queued job to clear stale skipped highlight, got %q (job %d)", track.LastMetadataJobStatus, newJob.ID)
	}
}

func TestListRunningBackgroundJobItems(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var ids []int64
	for _, name := range []string{"parallel-a.mp3", "parallel-b.mp3", "parallel-c.mp3"} {
		id, err := db.UpsertTrack(ctx, model.Track{
			Path: filepath.Join(t.TempDir(), name), FileName: name, Extension: ".mp3",
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}

	job, err := db.CreateBackgroundJob(ctx, "metadata_enrichment", "Metadata enrichment", `{}`, ids)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.MarkBackgroundJobRunning(ctx, job.ID); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		item, ok, err := db.NextQueuedBackgroundJobItem(ctx, job.ID)
		if err != nil || !ok {
			t.Fatalf("next item %d: ok=%v err=%v", i, ok, err)
		}
		if err := db.MarkBackgroundJobItemRunning(ctx, item.ID); err != nil {
			t.Fatal(err)
		}
	}

	running, err := db.ListRunningBackgroundJobItems(ctx, job.ID, 32)
	if err != nil {
		t.Fatal(err)
	}
	if len(running) != 2 {
		t.Fatalf("running items = %d, want 2: %+v", len(running), running)
	}
	for _, item := range running {
		if item.Status != "running" {
			t.Fatalf("unexpected running item status: %+v", item)
		}
	}
}
