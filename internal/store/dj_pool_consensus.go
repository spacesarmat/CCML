package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// PutDJPoolConsensus persists provider-only evidence captured during Find metadata.
func (s *Store) PutDJPoolConsensus(ctx context.Context, consensus model.DJPoolConsensus) error {
	if consensus.TrackID <= 0 {
		return fmt.Errorf("store DJ Pool consensus: invalid track id")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO dj_pool_consensus(track_id, bpm, bpm_support, bpm_quality, camelot, key_support, key_quality, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(track_id) DO UPDATE SET
    bpm=excluded.bpm,
    bpm_support=excluded.bpm_support,
    bpm_quality=excluded.bpm_quality,
    camelot=excluded.camelot,
    key_support=excluded.key_support,
    key_quality=excluded.key_quality,
    updated_at=excluded.updated_at`,
		consensus.TrackID,
		consensus.BPM,
		consensus.BPMSupport,
		consensus.BPMQuality,
		strings.ToUpper(strings.TrimSpace(consensus.Camelot)),
		consensus.KeySupport,
		consensus.KeyQuality,
		now,
	)
	if err != nil {
		return fmt.Errorf("store DJ Pool consensus for track %d: %w", consensus.TrackID, err)
	}
	return nil
}

// DJPoolConsensus returns the last provider-only consensus captured for a track.
func (s *Store) DJPoolConsensus(ctx context.Context, trackID int64) (model.DJPoolConsensus, bool, error) {
	var consensus model.DJPoolConsensus
	err := s.db.QueryRowContext(ctx, `
SELECT track_id, bpm, bpm_support, bpm_quality, camelot, key_support, key_quality, updated_at
FROM dj_pool_consensus
WHERE track_id = ?`, trackID).Scan(
		&consensus.TrackID,
		&consensus.BPM,
		&consensus.BPMSupport,
		&consensus.BPMQuality,
		&consensus.Camelot,
		&consensus.KeySupport,
		&consensus.KeyQuality,
		&consensus.UpdatedAt,
	)
	if err == nil {
		return consensus, true, nil
	}
	if err == sql.ErrNoRows {
		return model.DJPoolConsensus{}, false, nil
	}
	return model.DJPoolConsensus{}, false, fmt.Errorf("load DJ Pool consensus for track %d: %w", trackID, err)
}
