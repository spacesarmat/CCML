package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

const backgroundJobColumns = `id, type, title, status, options_json, created_at, started_at, finished_at, updated_at,
    total_items, completed_items, skipped_items, failed_items, cancelled_items, current_item, last_error`

// AllTrackIDs returns the entire library in stable insertion order for whole-library jobs.
func (s *Store) AllTrackIDs(ctx context.Context, maxItems int) ([]int64, error) {
	if maxItems <= 0 {
		maxItems = 100000
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tracks`).Scan(&count); err != nil {
		return nil, fmt.Errorf("count tracks for background job: %w", err)
	}
	if count == 0 {
		return nil, errors.New("library contains no tracks")
	}
	if count > maxItems {
		return nil, fmt.Errorf("library contains %d tracks; one background job is limited to %d", count, maxItems)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM tracks ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list track ids for background job: %w", err)
	}
	defer rows.Close()
	ids := make([]int64, 0, count)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan track id for background job: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate track ids for background job: %w", err)
	}
	return ids, nil
}

// CreateBackgroundJobForLibrary creates one item for every track with a single
// INSERT ... SELECT, avoiding per-track round trips for large libraries.
func (s *Store) CreateBackgroundJobForLibrary(ctx context.Context, jobType, title, optionsJSON string, maxItems int) (model.BackgroundJob, error) {
	jobType = strings.TrimSpace(jobType)
	if jobType == "" {
		return model.BackgroundJob{}, errors.New("background job type is required")
	}
	if strings.TrimSpace(title) == "" {
		title = jobType
	}
	if strings.TrimSpace(optionsJSON) == "" {
		optionsJSON = "{}"
	}
	if maxItems <= 0 {
		maxItems = 100000
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tracks`).Scan(&count); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("count tracks for background job: %w", err)
	}
	if count == 0 {
		return model.BackgroundJob{}, errors.New("library contains no tracks")
	}
	if count > maxItems {
		return model.BackgroundJob{}, fmt.Errorf("library contains %d tracks; one background job is limited to %d", count, maxItems)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("begin library background job transaction: %w", err)
	}
	rollback := true
	defer func() {
		if rollback {
			tx.Rollback()
		}
	}()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
INSERT INTO background_jobs(type, title, status, options_json, created_at, updated_at, total_items)
VALUES (?, ?, 'queued', ?, ?, ?, ?)`, jobType, title, optionsJSON, now, now, count)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("insert library background job: %w", err)
	}
	jobID, err := result.LastInsertId()
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("read library background job id: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO background_job_items(job_id, track_id, path, status, created_at, updated_at)
SELECT ?, id, path, 'queued', ?, ? FROM tracks ORDER BY id`, jobID, now, now); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("insert library background job items: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("commit library background job: %w", err)
	}
	rollback = false
	return s.BackgroundJobByID(ctx, jobID)
}

// CreateBackgroundJob persists a job and one item for every selected track.
func (s *Store) CreateBackgroundJob(ctx context.Context, jobType, title, optionsJSON string, trackIDs []int64) (model.BackgroundJob, error) {
	jobType = strings.TrimSpace(jobType)
	if jobType == "" {
		return model.BackgroundJob{}, errors.New("background job type is required")
	}
	if len(trackIDs) == 0 {
		return model.BackgroundJob{}, errors.New("background job requires at least one track")
	}
	if len(trackIDs) > 100000 {
		return model.BackgroundJob{}, fmt.Errorf("background job is limited to 100000 tracks")
	}
	if strings.TrimSpace(title) == "" {
		title = jobType
	}
	if strings.TrimSpace(optionsJSON) == "" {
		optionsJSON = "{}"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("begin background job transaction: %w", err)
	}
	rollback := true
	defer func() {
		if rollback {
			tx.Rollback()
		}
	}()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
INSERT INTO background_jobs(type, title, status, options_json, created_at, updated_at, total_items)
VALUES (?, ?, 'queued', ?, ?, ?, ?)`, jobType, title, optionsJSON, now, now, len(trackIDs))
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("insert background job: %w", err)
	}
	jobID, err := result.LastInsertId()
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("read background job id: %w", err)
	}

	itemStmt, err := tx.PrepareContext(ctx, `
INSERT INTO background_job_items(job_id, track_id, path, status, created_at, updated_at)
VALUES (?, ?, ?, 'queued', ?, ?)`)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("prepare background job items: %w", err)
	}
	defer itemStmt.Close()

	seen := make(map[int64]struct{}, len(trackIDs))
	inserted := 0
	for _, trackID := range trackIDs {
		if trackID <= 0 {
			continue
		}
		if _, ok := seen[trackID]; ok {
			continue
		}
		seen[trackID] = struct{}{}
		var path string
		if err := tx.QueryRowContext(ctx, `SELECT path FROM tracks WHERE id = ?`, trackID).Scan(&path); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return model.BackgroundJob{}, fmt.Errorf("track %d not found", trackID)
			}
			return model.BackgroundJob{}, fmt.Errorf("load track %d for background job: %w", trackID, err)
		}
		if _, err := itemStmt.ExecContext(ctx, jobID, trackID, path, now, now); err != nil {
			return model.BackgroundJob{}, fmt.Errorf("insert background job item for track %d: %w", trackID, err)
		}
		inserted++
	}
	if inserted == 0 {
		return model.BackgroundJob{}, errors.New("background job contains no valid tracks")
	}
	if inserted != len(trackIDs) {
		if _, err := tx.ExecContext(ctx, `UPDATE background_jobs SET total_items = ?, updated_at = ? WHERE id = ?`, inserted, now, jobID); err != nil {
			return model.BackgroundJob{}, fmt.Errorf("update background job item count: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("commit background job: %w", err)
	}
	rollback = false
	return s.BackgroundJobByID(ctx, jobID)
}

// BackgroundJobByID returns one job with derived progress.
func (s *Store) BackgroundJobByID(ctx context.Context, jobID int64) (model.BackgroundJob, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+backgroundJobColumns+` FROM background_jobs WHERE id = ?`, jobID)
	job, err := scanBackgroundJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.BackgroundJob{}, fmt.Errorf("background job %d not found", jobID)
		}
		return model.BackgroundJob{}, fmt.Errorf("load background job %d: %w", jobID, err)
	}
	return job, nil
}

// ListBackgroundJobs returns newest jobs first.
func (s *Store) ListBackgroundJobs(ctx context.Context, limit int) ([]model.BackgroundJob, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+backgroundJobColumns+` FROM background_jobs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list background jobs: %w", err)
	}
	defer rows.Close()
	jobs := make([]model.BackgroundJob, 0, limit)
	for rows.Next() {
		job, err := scanBackgroundJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan background job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate background jobs: %w", err)
	}
	return jobs, nil
}

// ListBackgroundJobItems returns items in processing order.
func (s *Store) ListBackgroundJobItems(ctx context.Context, jobID int64, limit, offset int) ([]model.BackgroundJobItem, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, job_id, track_id, path, status, attempts, error, result_json, created_at, started_at, finished_at, updated_at
FROM background_job_items WHERE job_id = ? ORDER BY id LIMIT ? OFFSET ?`, jobID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list background job items: %w", err)
	}
	defer rows.Close()
	items := make([]model.BackgroundJobItem, 0, limit)
	for rows.Next() {
		var item model.BackgroundJobItem
		if err := rows.Scan(&item.ID, &item.JobID, &item.TrackID, &item.Path, &item.Status, &item.Attempts, &item.Error,
			&item.ResultJSON, &item.CreatedAt, &item.StartedAt, &item.FinishedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan background job item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate background job items: %w", err)
	}
	return items, nil
}

// ListRunningBackgroundJobItems returns only items that are actively executing.
// It is intentionally bounded because the manager itself has bounded concurrency.
func (s *Store) ListRunningBackgroundJobItems(ctx context.Context, jobID int64, limit int) ([]model.BackgroundJobItem, error) {
	if limit <= 0 || limit > 64 {
		limit = 32
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, job_id, track_id, path, status, attempts, error, result_json, created_at, started_at, finished_at, updated_at
FROM background_job_items
WHERE job_id = ? AND status = 'running'
ORDER BY id
LIMIT ?`, jobID, limit)
	if err != nil {
		return nil, fmt.Errorf("list running background job items: %w", err)
	}
	defer rows.Close()

	items := make([]model.BackgroundJobItem, 0, limit)
	for rows.Next() {
		var item model.BackgroundJobItem
		if err := rows.Scan(&item.ID, &item.JobID, &item.TrackID, &item.Path, &item.Status, &item.Attempts, &item.Error,
			&item.ResultJSON, &item.CreatedAt, &item.StartedAt, &item.FinishedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan running background job item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate running background job items: %w", err)
	}
	return items, nil
}

// RecoverInterruptedBackgroundJobs makes persisted in-flight work runnable after a restart.
func (s *Store) RecoverInterruptedBackgroundJobs(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin background job recovery: %w", err)
	}
	rollback := true
	defer func() {
		if rollback {
			tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `
UPDATE background_job_items
SET status='cancelled', finished_at=?, updated_at=?, error=CASE WHEN error='' THEN 'Cancelled before application restart completed' ELSE error END
WHERE status='running' AND job_id IN (SELECT id FROM background_jobs WHERE status='cancelled')`, now, now); err != nil {
		return fmt.Errorf("recover cancelled background job items: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE background_job_items
SET status='queued', started_at='', finished_at='', updated_at=?, error=CASE WHEN error='' THEN 'Interrupted by application restart' ELSE error END
WHERE status='running' AND job_id IN (SELECT id FROM background_jobs WHERE status IN ('running','paused'))`, now); err != nil {
		return fmt.Errorf("recover background job items: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE background_jobs
SET status='queued', current_item='', updated_at=?, last_error=CASE WHEN last_error='' THEN 'Interrupted by application restart' ELSE last_error END
WHERE status='running'`, now); err != nil {
		return fmt.Errorf("recover background jobs: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit background job recovery: %w", err)
	}
	rollback = false
	return nil
}

// NextQueuedBackgroundJob returns the oldest runnable job.
func (s *Store) NextQueuedBackgroundJob(ctx context.Context) (model.BackgroundJob, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+backgroundJobColumns+` FROM background_jobs WHERE status='queued' ORDER BY id LIMIT 1`)
	job, err := scanBackgroundJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.BackgroundJob{}, false, nil
	}
	if err != nil {
		return model.BackgroundJob{}, false, fmt.Errorf("load next background job: %w", err)
	}
	return job, true, nil
}

// NextQueuedBackgroundJobItem returns the next pending item for one job.
func (s *Store) NextQueuedBackgroundJobItem(ctx context.Context, jobID int64) (model.BackgroundJobItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, job_id, track_id, path, status, attempts, error, result_json, created_at, started_at, finished_at, updated_at
FROM background_job_items WHERE job_id=? AND status='queued' ORDER BY id LIMIT 1`, jobID)
	var item model.BackgroundJobItem
	err := row.Scan(&item.ID, &item.JobID, &item.TrackID, &item.Path, &item.Status, &item.Attempts, &item.Error,
		&item.ResultJSON, &item.CreatedAt, &item.StartedAt, &item.FinishedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.BackgroundJobItem{}, false, nil
	}
	if err != nil {
		return model.BackgroundJobItem{}, false, fmt.Errorf("load next background job item: %w", err)
	}
	return item, true, nil
}

func (s *Store) MarkBackgroundJobRunning(ctx context.Context, jobID int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
UPDATE background_jobs SET status='running', started_at=CASE WHEN started_at='' THEN ? ELSE started_at END,
    finished_at='', updated_at=?, last_error='' WHERE id=? AND status='queued'`, now, now, jobID)
	if err != nil {
		return fmt.Errorf("mark background job running: %w", err)
	}
	return nil
}

func (s *Store) MarkBackgroundJobItemRunning(ctx context.Context, itemID int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
UPDATE background_job_items SET status='running', attempts=attempts+1, error='', result_json='',
    started_at=?, finished_at='', updated_at=? WHERE id=? AND status='queued'`, now, now, itemID)
	if err != nil {
		return fmt.Errorf("mark background job item running: %w", err)
	}
	return nil
}

func (s *Store) SetBackgroundJobCurrentItem(ctx context.Context, jobID int64, path string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `UPDATE background_jobs SET current_item=?, updated_at=? WHERE id=?`, path, now, jobID); err != nil {
		return fmt.Errorf("update background job current item: %w", err)
	}
	return nil
}

// FinishBackgroundJobItem marks one item terminal and recomputes job counters.
func (s *Store) FinishBackgroundJobItem(ctx context.Context, itemID int64, status, errorText, resultJSON string) (model.BackgroundJob, error) {
	if status != "completed" && status != "skipped" && status != "failed" && status != "cancelled" {
		return model.BackgroundJob{}, fmt.Errorf("invalid background job item status %q", status)
	}
	var jobID int64
	if err := s.db.QueryRowContext(ctx, `SELECT job_id FROM background_job_items WHERE id=?`, itemID).Scan(&jobID); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("load background job item owner: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `
UPDATE background_job_items SET status=?, error=?, result_json=?, finished_at=?, updated_at=? WHERE id=?`,
		status, errorText, resultJSON, now, now, itemID); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("finish background job item: %w", err)
	}
	return s.RecomputeBackgroundJob(ctx, jobID)
}

// RequeueBackgroundJobItem returns an interrupted item to the pending state.
func (s *Store) RequeueBackgroundJobItem(ctx context.Context, itemID int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `
UPDATE background_job_items SET status='queued', started_at='', finished_at='', updated_at=? WHERE id=? AND status IN ('running','cancelled')`, now, itemID); err != nil {
		return fmt.Errorf("requeue background job item: %w", err)
	}
	return nil
}

// RecomputeBackgroundJob refreshes counters without changing an explicitly paused/cancelled job.
func (s *Store) RecomputeBackgroundJob(ctx context.Context, jobID int64) (model.BackgroundJob, error) {
	var completed, skipped, failed, cancelled, running, queued int
	if err := s.db.QueryRowContext(ctx, `
SELECT
    COALESCE(SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN status='skipped' THEN 1 ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN status='cancelled' THEN 1 ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN status='running' THEN 1 ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN status='queued' THEN 1 ELSE 0 END), 0)
FROM background_job_items WHERE job_id=?`, jobID).Scan(&completed, &skipped, &failed, &cancelled, &running, &queued); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("count background job items: %w", err)
	}
	job, err := s.BackgroundJobByID(ctx, jobID)
	if err != nil {
		return model.BackgroundJob{}, err
	}
	status := job.Status
	finishedAt := job.FinishedAt
	currentItem := job.CurrentItem
	lastError := job.LastError
	if status != "paused" && status != "cancelled" {
		if running == 0 && queued == 0 {
			if failed > 0 {
				status = "failed"
				lastError = fmt.Sprintf("%d item(s) failed", failed)
			} else {
				status = "completed"
				lastError = ""
			}
			finishedAt = time.Now().UTC().Format(time.RFC3339Nano)
			currentItem = ""
		} else if status == "queued" && running > 0 {
			status = "running"
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `
UPDATE background_jobs SET status=?, completed_items=?, skipped_items=?, failed_items=?, cancelled_items=?, current_item=?,
    last_error=?, finished_at=?, updated_at=? WHERE id=?`,
		status, completed, skipped, failed, cancelled, currentItem, lastError, finishedAt, now, jobID); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("recompute background job: %w", err)
	}
	return s.BackgroundJobByID(ctx, jobID)
}

func (s *Store) PauseBackgroundJob(ctx context.Context, jobID int64) (model.BackgroundJob, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `UPDATE background_jobs SET status='paused', current_item='', updated_at=? WHERE id=? AND status IN ('queued','running')`, now, jobID)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("pause background job: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return s.BackgroundJobByID(ctx, jobID)
	}
	return s.BackgroundJobByID(ctx, jobID)
}

func (s *Store) ResumeBackgroundJob(ctx context.Context, jobID int64) (model.BackgroundJob, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `
UPDATE background_jobs SET status='queued', finished_at='', current_item='', last_error='', updated_at=? WHERE id=? AND status='paused'`, now, jobID); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("resume background job: %w", err)
	}
	return s.BackgroundJobByID(ctx, jobID)
}

func (s *Store) CancelBackgroundJob(ctx context.Context, jobID int64) (model.BackgroundJob, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("begin cancel background job: %w", err)
	}
	rollback := true
	defer func() {
		if rollback {
			tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `
UPDATE background_job_items SET status='cancelled', finished_at=?, updated_at=? WHERE job_id=? AND status='queued'`, now, now, jobID); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("cancel background job items: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE background_jobs SET status='cancelled', current_item='', finished_at=?, updated_at=? WHERE id=? AND status NOT IN ('completed','cancelled')`, now, now, jobID); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("cancel background job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("commit cancel background job: %w", err)
	}
	rollback = false
	return s.RecomputeBackgroundJob(ctx, jobID)
}

func (s *Store) RetryFailedBackgroundJob(ctx context.Context, jobID int64) (model.BackgroundJob, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("begin retry background job: %w", err)
	}
	rollback := true
	defer func() {
		if rollback {
			tx.Rollback()
		}
	}()
	result, err := tx.ExecContext(ctx, `
UPDATE background_job_items SET status='queued', error='', result_json='', started_at='', finished_at='', updated_at=?
WHERE job_id=? AND status='failed'`, now, jobID)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("retry background job items: %w", err)
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		tx.Rollback()
		rollback = false
		return s.BackgroundJobByID(ctx, jobID)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE background_jobs SET status='queued', finished_at='', current_item='', last_error='', updated_at=? WHERE id=?`, now, jobID); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("retry background job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.BackgroundJob{}, fmt.Errorf("commit retry background job: %w", err)
	}
	rollback = false
	job, err := s.RecomputeBackgroundJob(ctx, jobID)
	if err != nil {
		return model.BackgroundJob{}, err
	}
	if job.Status == "failed" {
		if _, err := s.db.ExecContext(ctx, `UPDATE background_jobs SET status='queued', finished_at='', updated_at=? WHERE id=?`, now, jobID); err != nil {
			return model.BackgroundJob{}, err
		}
		return s.BackgroundJobByID(ctx, jobID)
	}
	return job, nil
}

// RequeueInterruptedBackgroundJob preserves unfinished work during app shutdown.
func (s *Store) RequeueInterruptedBackgroundJob(ctx context.Context, jobID, itemID int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	rollback := true
	defer func() {
		if rollback {
			tx.Rollback()
		}
	}()
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM background_jobs WHERE id=?`, jobID).Scan(&status); err != nil {
		return err
	}
	if itemID > 0 {
		itemStatus := "queued"
		if status == "cancelled" {
			itemStatus = "cancelled"
		}
		if _, err := tx.ExecContext(ctx, `UPDATE background_job_items SET status=?, started_at='', finished_at=CASE WHEN ?='cancelled' THEN ? ELSE '' END, updated_at=? WHERE id=? AND status='running'`, itemStatus, itemStatus, now, now, itemID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE background_jobs SET status='queued', current_item='', updated_at=? WHERE id=? AND status='running'`, now, jobID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	rollback = false
	return nil
}

func scanBackgroundJob(scanner interface{ Scan(dest ...any) error }) (model.BackgroundJob, error) {
	var job model.BackgroundJob
	if err := scanner.Scan(&job.ID, &job.Type, &job.Title, &job.Status, &job.OptionsJSON, &job.CreatedAt, &job.StartedAt,
		&job.FinishedAt, &job.UpdatedAt, &job.TotalItems, &job.CompletedItems, &job.SkippedItems, &job.FailedItems,
		&job.CancelledItems, &job.CurrentItem, &job.LastError); err != nil {
		return model.BackgroundJob{}, err
	}
	processed := job.CompletedItems + job.SkippedItems + job.FailedItems + job.CancelledItems
	if job.TotalItems > 0 {
		job.Progress = float64(processed) / float64(job.TotalItems)
		if job.Progress > 1 {
			job.Progress = 1
		}
	}
	return job, nil
}
