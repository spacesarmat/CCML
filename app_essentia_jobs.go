package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	jobqueue "github.com/spacesarmat/CCML/internal/jobs"
	"github.com/spacesarmat/CCML/internal/model"
)

const essentiaAnalysisJobType = "essentia_analysis"

type essentiaAnalysisJobOptions struct {
	WriteTags   bool `json:"writeTags"`
	OnlyMissing bool `json:"onlyMissing"`
}

type essentiaAnalysisItemResult struct {
	TrackID     int64   `json:"trackId"`
	Path        string  `json:"path"`
	BPM         float64 `json:"bpm"`
	Key         string  `json:"key"`
	Scale       string  `json:"scale"`
	Strength    float64 `json:"strength"`
	WriteTags   bool    `json:"writeTags"`
	OnlyMissing bool    `json:"onlyMissing"`
	Written     bool    `json:"written"`
	Skipped     bool    `json:"skipped"`
	ChangeSetID int64   `json:"changeSetId"`
}

// CreateEssentiaAnalysisJob queues BPM/key analysis for selected tracks.
func (a *App) CreateEssentiaAnalysisJob(trackIDs []int64, writeTags, onlyMissing bool) (model.BackgroundJob, error) {
	if a.jobs == nil {
		return model.BackgroundJob{}, errors.New("background job manager is not available")
	}
	if a.bpmKey == nil || !a.bpmKey.Available() {
		return model.BackgroundJob{}, errors.New("Essentia is not configured")
	}
	if len(trackIDs) == 0 {
		return model.BackgroundJob{}, errors.New("no tracks selected")
	}
	opts := essentiaAnalysisJobOptions{WriteTags: writeTags, OnlyMissing: writeTags && onlyMissing}
	raw, err := json.Marshal(opts)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("encode Essentia analysis options: %w", err)
	}
	job, err := a.store.CreateBackgroundJob(a.context(), essentiaAnalysisJobType, "Essentia BPM/Key analysis", string(raw), trackIDs)
	if err != nil {
		return model.BackgroundJob{}, err
	}
	a.jobs.Wake()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "jobs:created", job)
	}
	return job, nil
}

// CreateLibraryEssentiaAnalysisJob queues BPM/key analysis for the entire library.
func (a *App) CreateLibraryEssentiaAnalysisJob(writeTags, onlyMissing bool) (model.BackgroundJob, error) {
	if a.jobs == nil {
		return model.BackgroundJob{}, errors.New("background job manager is not available")
	}
	if a.bpmKey == nil || !a.bpmKey.Available() {
		return model.BackgroundJob{}, errors.New("Essentia is not configured")
	}
	opts := essentiaAnalysisJobOptions{WriteTags: writeTags, OnlyMissing: writeTags && onlyMissing}
	raw, err := json.Marshal(opts)
	if err != nil {
		return model.BackgroundJob{}, fmt.Errorf("encode Essentia analysis options: %w", err)
	}
	job, err := a.store.CreateBackgroundJobForLibrary(a.context(), essentiaAnalysisJobType, "Essentia BPM/Key analysis — entire library", string(raw), 100000)
	if err != nil {
		return model.BackgroundJob{}, err
	}
	a.jobs.Wake()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "jobs:created", job)
	}
	return job, nil
}

func (a *App) runEssentiaJobItem(ctx context.Context, job model.BackgroundJob, work model.BackgroundJobItem) (jobqueue.ItemResult, error) {
	var opts essentiaAnalysisJobOptions
	if err := json.Unmarshal([]byte(job.OptionsJSON), &opts); err != nil {
		return jobqueue.ItemResult{}, fmt.Errorf("decode Essentia analysis options: %w", err)
	}
	if !opts.WriteTags {
		opts.OnlyMissing = false
	}

	track, err := a.store.TrackByID(ctx, work.TrackID)
	if err != nil {
		return jobqueue.ItemResult{}, err
	}
	result, err := a.bpmKey.Analyze(ctx, track.Path)
	if err != nil {
		return jobqueue.ItemResult{}, err
	}
	result, err = normalizeEssentiaAnalysisResult(result)
	if err != nil {
		return jobqueue.ItemResult{}, err
	}

	item := essentiaAnalysisItemResult{
		TrackID:     track.ID,
		Path:        track.Path,
		BPM:         result.BPM,
		Key:         result.Key,
		Scale:       result.Scale,
		Strength:    result.Strength,
		WriteTags:   opts.WriteTags,
		OnlyMissing: opts.OnlyMissing,
	}

	if opts.WriteTags {
		applied, applyErr := a.tagEditor.ApplyBPMKeyAnalysis(ctx, track.ID, result, opts.OnlyMissing)
		if applyErr != nil {
			raw, _ := json.Marshal(item)
			return jobqueue.ItemResult{ResultJSON: string(raw)}, applyErr
		}
		item.Written = applied.Changed > 0
		item.ChangeSetID = applied.ChangeSetID
		item.Skipped = !item.Written
	} else {
		if err := a.store.UpdateBPMKey(ctx, track.ID, result); err != nil {
			return jobqueue.ItemResult{}, err
		}
	}

	raw, err := json.Marshal(item)
	if err != nil {
		return jobqueue.ItemResult{}, fmt.Errorf("encode Essentia analysis result: %w", err)
	}
	status := "completed"
	if item.Skipped {
		status = "skipped"
	}
	return jobqueue.ItemResult{Status: status, ResultJSON: string(raw)}, nil
}

func normalizeEssentiaAnalysisResult(result model.BPMKey) (model.BPMKey, error) {
	result.Key = strings.TrimSpace(result.Key)
	result.Scale = strings.ToLower(strings.TrimSpace(result.Scale))

	if math.IsNaN(result.BPM) || math.IsInf(result.BPM, 0) || result.BPM < 20 || result.BPM > 400 {
		result.BPM = 0
	}
	if math.IsNaN(result.Strength) || math.IsInf(result.Strength, 0) {
		result.Strength = 0
	}
	if result.Strength < 0 {
		result.Strength = 0
	}
	if result.Strength > 1 {
		result.Strength = 1
	}
	if result.Key == "" {
		result.Scale = ""
		result.Strength = 0
	}
	if result.BPM == 0 && result.Key == "" {
		return model.BPMKey{}, errors.New("Essentia returned no usable BPM or key")
	}
	return result, nil
}
