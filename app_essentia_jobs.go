package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/spacesarmat/CCML/internal/audio"
	jobqueue "github.com/spacesarmat/CCML/internal/jobs"
	"github.com/spacesarmat/CCML/internal/model"
)

const essentiaAnalysisJobType = "essentia_analysis"

type essentiaAnalysisJobOptions struct {
	WriteTags     bool `json:"writeTags"`
	OnlyMissing   bool `json:"onlyMissing"`
	SkipUnchanged bool `json:"skipUnchanged"`
}

type essentiaAnalysisItemResult struct {
	TrackID            int64   `json:"trackId"`
	Path               string  `json:"path"`
	BPM                float64 `json:"bpm"`
	Key                string  `json:"key"`
	Scale              string  `json:"scale"`
	Strength           float64 `json:"strength"`
	Camelot            string  `json:"camelot"`
	OpenKey            string  `json:"openKey"`
	Mode               string  `json:"mode"`
	EffectiveMode      string  `json:"effectiveMode"`
	Profile            string  `json:"profile"`
	Escalated          bool    `json:"escalated"`
	EscalationReason   string  `json:"escalationReason"`
	FastEngine         string  `json:"fastEngine"`
	AccurateEngine     string  `json:"accurateEngine"`
	FastDurationMS     int64   `json:"fastDurationMs"`
	AccurateDurationMS int64   `json:"accurateDurationMs"`
	Cached             bool    `json:"cached"`
	DurationMS         int64   `json:"durationMs"`
	WriteTags          bool    `json:"writeTags"`
	OnlyMissing        bool    `json:"onlyMissing"`
	Written            bool    `json:"written"`
	Skipped            bool    `json:"skipped"`
	ChangeSetID        int64   `json:"changeSetId"`
}

// CreateEssentiaAnalysisJob queues BPM/key analysis for selected tracks.
func (a *App) CreateEssentiaAnalysisJob(trackIDs []int64, writeTags, onlyMissing, skipUnchanged bool) (model.BackgroundJob, error) {
	if a.jobs == nil {
		return model.BackgroundJob{}, errors.New("background job manager is not available")
	}
	if a.bpmKey == nil || !a.bpmKey.Available() {
		return model.BackgroundJob{}, errors.New("Essentia is not configured")
	}
	if len(trackIDs) == 0 {
		return model.BackgroundJob{}, errors.New("no tracks selected")
	}
	opts := essentiaAnalysisJobOptions{WriteTags: writeTags, OnlyMissing: writeTags && onlyMissing, SkipUnchanged: skipUnchanged}
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
func (a *App) CreateLibraryEssentiaAnalysisJob(writeTags, onlyMissing, skipUnchanged bool) (model.BackgroundJob, error) {
	if a.jobs == nil {
		return model.BackgroundJob{}, errors.New("background job manager is not available")
	}
	if a.bpmKey == nil || !a.bpmKey.Available() {
		return model.BackgroundJob{}, errors.New("Essentia is not configured")
	}
	opts := essentiaAnalysisJobOptions{WriteTags: writeTags, OnlyMissing: writeTags && onlyMissing, SkipUnchanged: skipUnchanged}
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
	hint, err := a.essentiaAdaptiveHint(ctx, track.ID)
	if err != nil {
		return jobqueue.ItemResult{}, err
	}
	profile := a.bpmKey.AnalysisProfile(hint)
	performance := a.bpmKey.Performance()
	analysisStarted := time.Now()
	cached := false
	var result model.BPMKey
	var run audio.AnalysisRun

	if opts.SkipUnchanged {
		stored, ok, loadErr := a.store.EssentiaAnalysis(ctx, track.ID)
		if loadErr != nil {
			return jobqueue.ItemResult{}, loadErr
		}
		if ok && essentiaAnalysisFreshForTrack(stored, track, profile) {
			result = model.BPMKey{
				BPM: stored.BPM, Key: stored.Key, Scale: stored.Scale, Strength: stored.Strength,
				Camelot: stored.Camelot, OpenKey: stored.OpenKey,
			}
			if result.Camelot == "" && result.Key != "" {
				result.Camelot, result.OpenKey, _ = audio.DJKeyFormats(result.Key, result.Scale)
			}
			run = audio.AnalysisRun{
				Result: result, RequestedMode: stored.RequestedMode, EffectiveMode: stored.EffectiveMode, Profile: profile,
			}
			if run.RequestedMode == "" {
				run.RequestedMode = performance.Mode
			}
			if run.EffectiveMode == "" {
				run.EffectiveMode = run.RequestedMode
			}
			cached = true
		}
	}

	if !cached {
		run, err = a.bpmKey.AnalyzeDetailed(ctx, track.Path, hint)
		if err != nil {
			return jobqueue.ItemResult{}, err
		}
		result, err = normalizeEssentiaAnalysisResult(run.Result)
		if err != nil {
			return jobqueue.ItemResult{}, err
		}
	}
	analysisDuration := time.Since(analysisStarted).Milliseconds()

	item := essentiaAnalysisItemResult{
		TrackID:            track.ID,
		Path:               track.Path,
		BPM:                result.BPM,
		Key:                result.Key,
		Scale:              result.Scale,
		Strength:           result.Strength,
		Camelot:            result.Camelot,
		OpenKey:            result.OpenKey,
		Mode:               run.RequestedMode,
		EffectiveMode:      run.EffectiveMode,
		Profile:            run.Profile,
		Escalated:          run.Escalated,
		EscalationReason:   run.EscalationReason,
		FastEngine:         run.FastEngine,
		AccurateEngine:     run.AccurateEngine,
		FastDurationMS:     run.FastDurationMS,
		AccurateDurationMS: run.AccurateDurationMS,
		Cached:             cached,
		DurationMS:         analysisDuration,
		WriteTags:          opts.WriteTags,
		OnlyMissing:        opts.OnlyMissing,
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
		item.Skipped = cached
	}
	if err := a.store.PutEssentiaAnalysisRun(
		ctx, track.ID, result, run.Profile, run.RequestedMode, run.EffectiveMode,
	); err != nil {
		return jobqueue.ItemResult{}, err
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

func essentiaAnalysisFreshForTrack(analysis model.EssentiaAnalysis, track model.Track, profile string) bool {
	if strings.TrimSpace(analysis.AnalyzedAt) == "" || track.ModifiedUnix <= 0 {
		return false
	}
	if strings.TrimSpace(profile) == "" || strings.TrimSpace(analysis.Profile) != strings.TrimSpace(profile) {
		return false
	}
	analyzedAt, err := time.Parse(time.RFC3339Nano, analysis.AnalyzedAt)
	if err != nil {
		return false
	}
	return analyzedAt.Unix() >= track.ModifiedUnix
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
		result.Camelot = ""
		result.OpenKey = ""
	} else {
		result.Camelot, result.OpenKey, _ = audio.DJKeyFormats(result.Key, result.Scale)
	}
	if result.BPM == 0 && result.Key == "" {
		return model.BPMKey{}, errors.New("Essentia returned no usable BPM or key")
	}
	return result, nil
}
