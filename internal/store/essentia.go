package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// PutEssentiaAnalysis persists local analyzer evidence separately from file tags.
// Older callers remain compatible but produce an unprofiled row, which is never
// reused by Stage 19.5 profile-aware caching.
func (s *Store) PutEssentiaAnalysis(ctx context.Context, trackID int64, result model.BPMKey) error {
	return s.PutEssentiaAnalysisRun(ctx, trackID, result, "", "", "")
}

// PutEssentiaAnalysisRun stores the result together with the exact analysis
// profile that produced it.
func (s *Store) PutEssentiaAnalysisRun(
	ctx context.Context,
	trackID int64,
	result model.BPMKey,
	profile, requestedMode, effectiveMode string,
) error {
	result.Key = strings.TrimSpace(result.Key)
	result.Scale = strings.ToLower(strings.TrimSpace(result.Scale))
	if math.IsNaN(result.BPM) || math.IsInf(result.BPM, 0) {
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
	if result.BPM <= 0 && result.Key == "" {
		return fmt.Errorf("store Essentia analysis for track %d: no usable BPM/key evidence", trackID)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO essentia_analysis(
    track_id, bpm, musical_key, key_scale, strength, profile, requested_mode, effective_mode, analyzed_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(track_id) DO UPDATE SET
    bpm=excluded.bpm,
    musical_key=excluded.musical_key,
    key_scale=excluded.key_scale,
    strength=excluded.strength,
    profile=excluded.profile,
    requested_mode=excluded.requested_mode,
    effective_mode=excluded.effective_mode,
    analyzed_at=excluded.analyzed_at`,
		trackID, result.BPM, result.Key, result.Scale, result.Strength,
		strings.TrimSpace(profile), strings.TrimSpace(requestedMode), strings.TrimSpace(effectiveMode), now)
	if err != nil {
		return fmt.Errorf("store Essentia analysis for track %d: %w", trackID, err)
	}
	return nil
}

// EssentiaAnalysis returns the latest persisted evidence. Stage 19.1 job JSON is
// used as a compatibility fallback for analyses completed before Stage 19.2.
func (s *Store) EssentiaAnalysis(ctx context.Context, trackID int64) (model.EssentiaAnalysis, bool, error) {
	var analysis model.EssentiaAnalysis
	err := s.db.QueryRowContext(ctx, `
SELECT track_id, bpm, musical_key, key_scale, strength, profile, requested_mode, effective_mode, analyzed_at
FROM essentia_analysis WHERE track_id = ?`, trackID).Scan(
		&analysis.TrackID,
		&analysis.BPM,
		&analysis.Key,
		&analysis.Scale,
		&analysis.Strength,
		&analysis.Profile,
		&analysis.RequestedMode,
		&analysis.EffectiveMode,
		&analysis.AnalyzedAt,
	)
	if err == nil {
		return analysis, true, nil
	}
	if err != sql.ErrNoRows {
		return model.EssentiaAnalysis{}, false, fmt.Errorf("load Essentia analysis for track %d: %w", trackID, err)
	}
	return s.legacyEssentiaJobAnalysis(ctx, trackID)
}

func (s *Store) legacyEssentiaJobAnalysis(ctx context.Context, trackID int64) (model.EssentiaAnalysis, bool, error) {
	var raw, analyzedAt string
	err := s.db.QueryRowContext(ctx, `
SELECT bji.result_json, bji.updated_at
FROM background_job_items bji
JOIN background_jobs bj ON bj.id = bji.job_id
WHERE bji.track_id = ?
  AND bj.type = 'essentia_analysis'
  AND bji.result_json <> ''
ORDER BY bji.id DESC
LIMIT 1`, trackID).Scan(&raw, &analyzedAt)
	if err == sql.ErrNoRows {
		return model.EssentiaAnalysis{}, false, nil
	}
	if err != nil {
		return model.EssentiaAnalysis{}, false, fmt.Errorf("load legacy Essentia job result for track %d: %w", trackID, err)
	}
	var payload struct {
		BPM           float64 `json:"bpm"`
		Key           string  `json:"key"`
		Scale         string  `json:"scale"`
		Strength      float64 `json:"strength"`
		Profile       string  `json:"profile"`
		Mode          string  `json:"mode"`
		EffectiveMode string  `json:"effectiveMode"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return model.EssentiaAnalysis{}, false, nil
	}
	if payload.BPM <= 0 && strings.TrimSpace(payload.Key) == "" {
		return model.EssentiaAnalysis{}, false, nil
	}
	if math.IsNaN(payload.Strength) || math.IsInf(payload.Strength, 0) {
		payload.Strength = 0
	}
	if payload.Strength < 0 {
		payload.Strength = 0
	}
	if payload.Strength > 1 {
		payload.Strength = 1
	}
	return model.EssentiaAnalysis{
		TrackID:       trackID,
		BPM:           payload.BPM,
		Key:           strings.TrimSpace(payload.Key),
		Scale:         strings.ToLower(strings.TrimSpace(payload.Scale)),
		Strength:      payload.Strength,
		Profile:       strings.TrimSpace(payload.Profile),
		RequestedMode: strings.TrimSpace(payload.Mode),
		EffectiveMode: strings.TrimSpace(payload.EffectiveMode),
		AnalyzedAt:    analyzedAt,
	}, true, nil
}
