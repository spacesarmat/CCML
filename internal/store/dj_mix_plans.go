package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// SaveDJMixPlan inserts or updates one named planner snapshot.
func (s *Store) SaveDJMixPlan(ctx context.Context, saved model.DJMixSavedPlan) (model.DJMixSavedPlan, error) {
	saved.Name = strings.TrimSpace(saved.Name)
	if saved.Name == "" {
		return model.DJMixSavedPlan{}, errors.New("DJ mix plan name is required")
	}
	if len([]rune(saved.Name)) > 120 {
		return model.DJMixSavedPlan{}, errors.New("DJ mix plan name is limited to 120 characters")
	}
	saved.ScopeTrackIDs = canonicalDJMixScopeIDs(saved.ScopeTrackIDs)

	scopeRaw, err := json.Marshal(saved.ScopeTrackIDs)
	if err != nil {
		return model.DJMixSavedPlan{}, fmt.Errorf("encode DJ mix plan scope: %w", err)
	}
	optionsRaw, err := json.Marshal(saved.Options)
	if err != nil {
		return model.DJMixSavedPlan{}, fmt.Errorf("encode DJ mix plan options: %w", err)
	}
	planRaw, err := json.Marshal(saved.Plan)
	if err != nil {
		return model.DJMixSavedPlan{}, fmt.Errorf("encode DJ mix plan snapshot: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	if saved.ID <= 0 {
		result, err := s.db.ExecContext(ctx, `
INSERT INTO dj_mix_plans(name, scope_track_ids_json, options_json, plan_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`,
			saved.Name, string(scopeRaw), string(optionsRaw), string(planRaw), now, now)
		if err != nil {
			return model.DJMixSavedPlan{}, fmt.Errorf("insert DJ mix plan: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return model.DJMixSavedPlan{}, fmt.Errorf("read DJ mix plan id: %w", err)
		}
		return s.DJMixPlanByID(ctx, id)
	}

	result, err := s.db.ExecContext(ctx, `
UPDATE dj_mix_plans
SET name = ?, scope_track_ids_json = ?, options_json = ?, plan_json = ?, updated_at = ?
WHERE id = ?`,
		saved.Name, string(scopeRaw), string(optionsRaw), string(planRaw), now, saved.ID)
	if err != nil {
		return model.DJMixSavedPlan{}, fmt.Errorf("update DJ mix plan %d: %w", saved.ID, err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return model.DJMixSavedPlan{}, fmt.Errorf("read DJ mix plan update count: %w", err)
	}
	if changed == 0 {
		return model.DJMixSavedPlan{}, fmt.Errorf("DJ mix plan %d not found", saved.ID)
	}
	return s.DJMixPlanByID(ctx, saved.ID)
}

// DJMixPlanByID loads one persisted planner snapshot.
func (s *Store) DJMixPlanByID(ctx context.Context, id int64) (model.DJMixSavedPlan, error) {
	if id <= 0 {
		return model.DJMixSavedPlan{}, errors.New("DJ mix plan id is required")
	}
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, scope_track_ids_json, options_json, plan_json, created_at, updated_at
FROM dj_mix_plans
WHERE id = ?`, id)
	saved, err := scanDJMixSavedPlan(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.DJMixSavedPlan{}, fmt.Errorf("DJ mix plan %d not found: %w", id, err)
		}
		return model.DJMixSavedPlan{}, fmt.Errorf("load DJ mix plan %d: %w", id, err)
	}
	return saved, nil
}

// ListDJMixPlans returns recently updated plans first.
func (s *Store) ListDJMixPlans(ctx context.Context, limit int) (plans []model.DJMixSavedPlan, resultErr error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, scope_track_ids_json, options_json, plan_json, created_at, updated_at
FROM dj_mix_plans
ORDER BY updated_at DESC, id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list DJ mix plans: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close DJ mix plan rows: %w", err))
		}
	}()

	for rows.Next() {
		saved, err := scanDJMixSavedPlan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan DJ mix plan: %w", err)
		}
		plans = append(plans, saved)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate DJ mix plans: %w", err)
	}
	return plans, nil
}

// DeleteDJMixPlan deletes only the planner snapshot. Tracks and files are untouched.
func (s *Store) DeleteDJMixPlan(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("DJ mix plan id is required")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM dj_mix_plans WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete DJ mix plan %d: %w", id, err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read DJ mix plan delete count: %w", err)
	}
	if changed == 0 {
		return fmt.Errorf("DJ mix plan %d not found", id)
	}
	return nil
}

type djMixPlanScanner interface {
	Scan(dest ...any) error
}

func scanDJMixSavedPlan(scanner djMixPlanScanner) (model.DJMixSavedPlan, error) {
	var saved model.DJMixSavedPlan
	var scopeJSON, optionsJSON, planJSON string
	if err := scanner.Scan(
		&saved.ID,
		&saved.Name,
		&scopeJSON,
		&optionsJSON,
		&planJSON,
		&saved.CreatedAt,
		&saved.UpdatedAt,
	); err != nil {
		return model.DJMixSavedPlan{}, err
	}
	if err := json.Unmarshal([]byte(scopeJSON), &saved.ScopeTrackIDs); err != nil {
		return model.DJMixSavedPlan{}, fmt.Errorf("decode DJ mix plan scope: %w", err)
	}
	if err := json.Unmarshal([]byte(optionsJSON), &saved.Options); err != nil {
		return model.DJMixSavedPlan{}, fmt.Errorf("decode DJ mix plan options: %w", err)
	}
	if err := json.Unmarshal([]byte(planJSON), &saved.Plan); err != nil {
		return model.DJMixSavedPlan{}, fmt.Errorf("decode DJ mix plan snapshot: %w", err)
	}
	saved.ScopeTrackIDs = canonicalDJMixScopeIDs(saved.ScopeTrackIDs)
	return saved, nil
}

func canonicalDJMixScopeIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{}
	}
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
