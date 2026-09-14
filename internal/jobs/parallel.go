package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

type concurrentItemResult struct {
	item   model.BackgroundJobItem
	result ItemResult
	err    error
}

// runJobConcurrent executes a bounded number of items from one persistent job at
// the same time. DB claiming/finishing remains coordinated here; the expensive
// runner work happens concurrently.
func (m *Manager) runJobConcurrent(root context.Context, job model.BackgroundJob, runner Runner, workers int) error {
	if workers < 2 {
		return errors.New("parallel background job requires at least 2 workers")
	}

	jobCtx, cancelJob := context.WithCancel(root)
	m.setCurrent(job.ID, cancelJob)
	defer func() {
		cancelJob()
		m.clearCurrent(job.ID)
	}()

	results := make(chan concurrentItemResult, workers)
	inFlight := 0
	exhausted := false
	stopping := false

	launch := func(item model.BackgroundJobItem) {
		inFlight++
		go func() {
			result, err := runner(jobCtx, job, item)
			results <- concurrentItemResult{item: item, result: result, err: err}
		}()
	}

	for {
		if root.Err() != nil {
			cancelJob()
			return m.preserveConcurrentInterrupted(job.ID, inFlight, results, root.Err())
		}

		fresh, err := m.store.BackgroundJobByID(root, job.ID)
		if err != nil {
			cancelJob()
			return err
		}
		if fresh.Status == "paused" || fresh.Status == "cancelled" {
			stopping = true
			cancelJob()
		}

		for !stopping && !exhausted && inFlight < workers {
			item, ok, err := m.store.NextQueuedBackgroundJobItem(root, job.ID)
			if err != nil {
				cancelJob()
				return err
			}
			if !ok {
				exhausted = true
				break
			}

			if err := m.store.MarkBackgroundJobItemRunning(root, item.ID); err != nil {
				cancelJob()
				return err
			}
			if err := m.store.SetBackgroundJobCurrentItem(root, job.ID, item.Path); err != nil {
				cancelJob()
				return err
			}
			item.Status = "running"
			item.Attempts++
			launch(item)
		}

		if inFlight == 0 {
			if stopping {
				fresh, err := m.store.BackgroundJobByID(root, job.ID)
				if err == nil {
					m.emit("jobs:updated", fresh)
				}
				return err
			}
			final, err := m.store.RecomputeBackgroundJob(root, job.ID)
			if err == nil {
				m.emit("jobs:updated", final)
			}
			return err
		}

		var completed concurrentItemResult
		select {
		case <-root.Done():
			cancelJob()
			return m.preserveConcurrentInterrupted(job.ID, inFlight, results, root.Err())
		case completed = <-results:
			inFlight--
		}

		if root.Err() != nil {
			cancelJob()
			if err := m.preserveOneInterrupted(job.ID, completed.item.ID); err != nil {
				return err
			}
			return m.preserveConcurrentInterrupted(job.ID, inFlight, results, root.Err())
		}

		fresh, err = m.store.BackgroundJobByID(root, job.ID)
		if err != nil {
			cancelJob()
			return err
		}

		status := completed.result.Status
		if completed.err != nil {
			status = "failed"
		}
		if status == "" {
			status = "completed"
		}
		errorText := ""
		if completed.err != nil {
			errorText = completed.err.Error()
		}

		switch fresh.Status {
		case "cancelled":
			stopping = true
			cancelJob()
			if completed.err != nil {
				status = "cancelled"
				errorText = ""
			}
			updated, finishErr := m.store.FinishBackgroundJobItem(root, completed.item.ID, status, errorText, completed.result.ResultJSON)
			if finishErr != nil {
				return finishErr
			}
			m.emit("jobs:updated", updated)

		case "paused":
			stopping = true
			cancelJob()
			if completed.err == nil {
				updated, finishErr := m.store.FinishBackgroundJobItem(root, completed.item.ID, status, "", completed.result.ResultJSON)
				if finishErr != nil {
					return finishErr
				}
				m.emit("jobs:updated", updated)
			} else {
				if err := m.store.RequeueBackgroundJobItem(root, completed.item.ID); err != nil {
					return err
				}
				paused, loadErr := m.store.BackgroundJobByID(root, job.ID)
				if loadErr != nil {
					return loadErr
				}
				m.emit("jobs:updated", paused)
			}

		default:
			updated, finishErr := m.store.FinishBackgroundJobItem(root, completed.item.ID, status, errorText, completed.result.ResultJSON)
			if finishErr != nil {
				return finishErr
			}
			m.emit("jobs:updated", updated)
		}
	}
}

func (m *Manager) preserveConcurrentInterrupted(jobID int64, inFlight int, results <-chan concurrentItemResult, cause error) error {
	for i := 0; i < inFlight; i++ {
		completed := <-results
		if err := m.preserveOneInterrupted(jobID, completed.item.ID); err != nil {
			return err
		}
	}
	return cause
}

func (m *Manager) preserveOneInterrupted(jobID, itemID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return m.store.RequeueInterruptedBackgroundJob(ctx, jobID, itemID)
}
