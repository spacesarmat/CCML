package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// MetadataLookupCache returns a non-expired cached lookup result.
func (s *Store) MetadataLookupCache(ctx context.Context, key string) (model.MetadataLookupResult, time.Duration, bool, error) {
	var raw string
	var createdUnix, expiresUnix int64
	err := s.db.QueryRowContext(ctx, `SELECT result_json, created_unix, expires_unix FROM metadata_lookup_cache WHERE cache_key = ?`, key).Scan(&raw, &createdUnix, &expiresUnix)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.MetadataLookupResult{}, 0, false, nil
		}
		return model.MetadataLookupResult{}, 0, false, fmt.Errorf("read metadata cache: %w", err)
	}
	now := time.Now().Unix()
	if expiresUnix <= now {
		if _, deleteErr := s.db.ExecContext(ctx, `DELETE FROM metadata_lookup_cache WHERE cache_key = ?`, key); deleteErr != nil {
			return model.MetadataLookupResult{}, 0, false, fmt.Errorf("delete expired metadata cache: %w", deleteErr)
		}
		return model.MetadataLookupResult{}, 0, false, nil
	}
	var result model.MetadataLookupResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		if _, deleteErr := s.db.ExecContext(ctx, `DELETE FROM metadata_lookup_cache WHERE cache_key = ?`, key); deleteErr != nil {
			return model.MetadataLookupResult{}, 0, false, fmt.Errorf("decode metadata cache: %w (cleanup: %v)", err, deleteErr)
		}
		return model.MetadataLookupResult{}, 0, false, nil
	}
	age := time.Since(time.Unix(createdUnix, 0))
	if age < 0 {
		age = 0
	}
	return result, age, true, nil
}

// PutMetadataLookupCache persists a lookup result for reuse across restarts.
func (s *Store) PutMetadataLookupCache(ctx context.Context, key string, result model.MetadataLookupResult, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	result.Cached = false
	result.CacheAgeSeconds = 0
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode metadata cache: %w", err)
	}
	now := time.Now().Unix()
	expires := time.Now().Add(ttl).Unix()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO metadata_lookup_cache(cache_key, result_json, created_unix, expires_unix)
VALUES (?, ?, ?, ?)
ON CONFLICT(cache_key) DO UPDATE SET
    result_json=excluded.result_json,
    created_unix=excluded.created_unix,
    expires_unix=excluded.expires_unix`, key, string(raw), now, expires)
	if err != nil {
		return fmt.Errorf("write metadata cache: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM metadata_lookup_cache WHERE expires_unix <= ?`, now); err != nil {
		return fmt.Errorf("prune metadata cache: %w", err)
	}
	return nil
}

// ClearMetadataLookupCache invalidates cached searches, for example after
// provider settings change.
func (s *Store) ClearMetadataLookupCache(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM metadata_lookup_cache`); err != nil {
		return fmt.Errorf("clear metadata cache: %w", err)
	}
	return nil
}
