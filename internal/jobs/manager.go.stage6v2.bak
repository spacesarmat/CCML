// Package jobs runs persistent background library operations.
package jobs

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
	"github.com/spacesarmat/CCML/internal/store"
)

const pollInterval = 2 * time.Second

// ItemResult describes the terminal state and optional JSON result of one job item.
type ItemResult struct {
	Status     string
	ResultJSON string
}

// Runner processes one background job item.
type Runner func(context.Context, model.BackgroundJob, model.BackgroundJobItem) (ItemResult, error)

// EventEmitter forwards job state changes to the UI without coupling this package to Wails.
type EventEmitter func(name string, payload any)

// Manager serially executes persistent jobs. Provider-level operations may do their own
// controlled concurrency, while serial job execution keeps rate-limited catalog work predictable.
type Manager struct {
	store *store.Store

	mu      sync.Mutex
	runners map[string]Runner
	emitter EventEmitter
	current map[int64]context.CancelFunc
	ctx     context.Context
	cancel  context.CancelFunc
	wake    chan struct{}
	wg      sync.WaitGroup
	started bool
}

func New(db *store.Store) *Manager {
	return &Manager{
		store:   db,
		runners: make(map[string]Runner),
		current: make(map[int64]context.CancelFunc),
		wake:    make(chan struct{}, 1),
	}
}

func (m *Manager) Register(jobType string, runner Runner) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runners[jobType] = runner
}

func (m *Manager) SetEmitter(emitter EventEmitter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emitter = emitter
}

// Start recovers work interrupted by the previous process and begins the worker loop.
func (m *Manager) Start(parent context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	m.ctx, m.cancel, m.started = ctx, cancel, true
	m.mu.Unlock()

	if err := m.store.RecoverInterruptedBackgroundJobs(ctx); err != nil {
		m.mu.Lock()
		m.started = false
		m.mu.Unlock()
		cancel()
		return err
	}
	m.wg.Add(1)
	go m.loop(ctx)
	m.Wake()
	return nil
}

// Stop preserves current work as queued so it can resume after the next launch.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return
	}
	cancel := m.cancel
	m.started = false
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.cancelAllCurrent()
	m.wg.Wait()
}

func (m *Manager) Wake() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) Pause(ctx context.Context, jobID int64) (model.BackgroundJob, error) {
	job, err := m.store.PauseBackgroundJob(ctx, jobID)
	if err != nil {
		return model.BackgroundJob{}, err
	}
	m.cancelCurrent(jobID)
	m.emit("jobs:updated", job)
	return job, nil
}

func (m *Manager) Resume(ctx context.Context, jobID int64) (model.BackgroundJob, error) {
	job, err := m.store.ResumeBackgroundJob(ctx, jobID)
	if err != nil {
		return model.BackgroundJob{}, err
	}
	m.emit("jobs:updated", job)
	m.Wake()
	return job, nil
}

func (m *Manager) Cancel(ctx context.Context, jobID int64) (model.BackgroundJob, error) {
	job, err := m.store.CancelBackgroundJob(ctx, jobID)
	if err != nil {
		return model.BackgroundJob{}, err
	}
	m.cancelCurrent(jobID)
	m.emit("jobs:updated", job)
	return job, nil
}

func (m *Manager) RetryFailed(ctx context.Context, jobID int64) (model.BackgroundJob, error) {
	job, err := m.store.RetryFailedBackgroundJob(ctx, jobID)
	if err != nil {
		return model.BackgroundJob{}, err
	}
	m.emit("jobs:updated", job)
	m.Wake()
	return job, nil
}

func (m *Manager) loop(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
		case <-ticker.C:
		}
		if err := m.drain(ctx); err != nil && !errors.Is(err, context.Canceled) {
			m.emit("jobs:error", err.Error())
		}
	}
}

func (m *Manager) drain(ctx context.Context) error {
	for ctx.Err() == nil {
		job, ok, err := m.store.NextQueuedBackgroundJob(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := m.runJob(ctx, job); err != nil {
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return ctx.Err()
			}
			m.emit("jobs:error", err.Error())
		}
	}
	return ctx.Err()
}

func (m *Manager) runJob(root context.Context, job model.BackgroundJob) error {
	runner := m.runner(job.Type)
	if runner == nil {
		item, ok, err := m.store.NextQueuedBackgroundJobItem(root, job.ID)
		if err != nil {
			return err
		}
		if ok {
			_, err = m.store.FinishBackgroundJobItem(root, item.ID, "failed", "Unsupported job type: "+job.Type, "")
			return err
		}
		return nil
	}
	if err := m.store.MarkBackgroundJobRunning(root, job.ID); err != nil {
		return err
	}
	job, err := m.store.BackgroundJobByID(root, job.ID)
	if err != nil {
		return err
	}
	m.emit("jobs:updated", job)

	for root.Err() == nil {
		fresh, err := m.store.BackgroundJobByID(root, job.ID)
		if err != nil {
			return err
		}
		if fresh.Status == "paused" || fresh.Status == "cancelled" {
			m.emit("jobs:updated", fresh)
			return nil
		}

		item, ok, err := m.store.NextQueuedBackgroundJobItem(root, job.ID)
		if err != nil {
			return err
		}
		if !ok {
			final, err := m.store.RecomputeBackgroundJob(root, job.ID)
			if err == nil {
				m.emit("jobs:updated", final)
			}
			return err
		}

		if err := m.store.MarkBackgroundJobItemRunning(root, item.ID); err != nil {
			return err
		}
		if err := m.store.SetBackgroundJobCurrentItem(root, job.ID, item.Path); err != nil {
			return err
		}
		item.Status = "running"
		item.Attempts++
		job, err = m.store.BackgroundJobByID(root, job.ID)
		if err != nil {
			return err
		}
		m.emit("jobs:updated", job)

		itemCtx, cancel := context.WithCancel(root)
		m.setCurrent(job.ID, cancel)
		result, runErr := runner(itemCtx, job, item)
		cancel()
		m.clearCurrent(job.ID)

		if root.Err() != nil {
			preserveCtx, preserveCancel := context.WithTimeout(context.Background(), 2*time.Second)
			preserveErr := m.store.RequeueInterruptedBackgroundJob(preserveCtx, job.ID, item.ID)
			preserveCancel()
			if preserveErr != nil {
				return preserveErr
			}
			return root.Err()
		}

		fresh, err = m.store.BackgroundJobByID(root, job.ID)
		if err != nil {
			return err
		}

		status := result.Status
		if runErr != nil {
			status = "failed"
		}
		if status == "" {
			status = "completed"
		}
		errorText := ""
		if runErr != nil {
			errorText = runErr.Error()
		}

		if fresh.Status == "cancelled" {
			// Cancellation races with external I/O. If the runner finished successfully
			// before observing cancellation, preserve that success instead of claiming
			// the already-applied operation was cancelled.
			if runErr != nil {
				status = "cancelled"
				errorText = ""
			}
			updated, finishErr := m.store.FinishBackgroundJobItem(root, item.ID, status, errorText, result.ResultJSON)
			if finishErr != nil {
				return finishErr
			}
			m.emit("jobs:updated", updated)
			return nil
		}
		if fresh.Status == "paused" {
			// A successful item is terminal even if Pause was pressed at the same time.
			// Only unfinished/cancelled work is returned to the queue.
			if runErr == nil {
				updated, finishErr := m.store.FinishBackgroundJobItem(root, item.ID, status, "", result.ResultJSON)
				if finishErr != nil {
					return finishErr
				}
				m.emit("jobs:updated", updated)
				return nil
			}
			if err := m.store.RequeueBackgroundJobItem(root, item.ID); err != nil {
				return err
			}
			paused, loadErr := m.store.BackgroundJobByID(root, job.ID)
			if loadErr != nil {
				return loadErr
			}
			m.emit("jobs:updated", paused)
			return nil
		}

		updated, err := m.store.FinishBackgroundJobItem(root, item.ID, status, errorText, result.ResultJSON)
		if err != nil {
			return err
		}
		m.emit("jobs:updated", updated)
	}
	return root.Err()
}

func (m *Manager) runner(jobType string) Runner {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runners[jobType]
}

func (m *Manager) setCurrent(jobID int64, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current[jobID] = cancel
}

func (m *Manager) clearCurrent(jobID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.current, jobID)
}

func (m *Manager) cancelCurrent(jobID int64) {
	m.mu.Lock()
	cancel := m.current[jobID]
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) cancelAllCurrent() {
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.current))
	for _, cancel := range m.current {
		cancels = append(cancels, cancel)
	}
	m.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (m *Manager) emit(name string, payload any) {
	m.mu.Lock()
	emitter := m.emitter
	m.mu.Unlock()
	if emitter != nil {
		emitter(name, payload)
	}
}
