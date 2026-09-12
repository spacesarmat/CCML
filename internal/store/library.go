package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// TrackState contains the file attributes used by incremental scanning.
type TrackState struct {
	Size         int64
	ModifiedUnix int64
}

// TrackStatesUnderRoot returns known file states below root keyed by absolute path.
func (s *Store) TrackStatesUnderRoot(ctx context.Context, root string) (states map[string]TrackState, resultErr error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, size, modified_unix FROM tracks WHERE path LIKE ? ESCAPE '!' COLLATE NOCASE`, rootPattern(root))
	if err != nil {
		return nil, fmt.Errorf("load track states for %q: %w", root, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close track-state rows: %w", err))
		}
	}()

	states = make(map[string]TrackState)
	for rows.Next() {
		var path string
		var state TrackState
		if err := rows.Scan(&path, &state.Size, &state.ModifiedUnix); err != nil {
			return nil, fmt.Errorf("scan track state: %w", err)
		}
		states[path] = state
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate track states: %w", err)
	}
	return states, nil
}

// MarkTrackSeen records that path was present during scanID for root.
func (s *Store) MarkTrackSeen(ctx context.Context, path, root, scanID string) error {
	const query = `
INSERT INTO scan_entries(path, root_path, last_seen_scan)
VALUES (?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
    root_path=excluded.root_path,
    last_seen_scan=excluded.last_seen_scan`
	if _, err := s.db.ExecContext(ctx, query, path, root, scanID); err != nil {
		return fmt.Errorf("mark track seen %q: %w", path, err)
	}
	return nil
}

// DeleteUnseenTracks removes files below root that were not observed in scanID.
// Call this only after a complete, non-cancelled directory walk.
func (s *Store) DeleteUnseenTracks(ctx context.Context, root, scanID string) (int64, error) {
	const query = `
DELETE FROM tracks
WHERE path LIKE ? ESCAPE '!' COLLATE NOCASE
  AND NOT EXISTS (
      SELECT 1
      FROM scan_entries
      WHERE scan_entries.path = tracks.path
        AND scan_entries.root_path = ?
        AND scan_entries.last_seen_scan = ?
  )`
	res, err := s.db.ExecContext(ctx, query, rootPattern(root), root, scanID)
	if err != nil {
		return 0, fmt.Errorf("remove missing tracks below %q: %w", root, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read removed-track count: %w", err)
	}
	return n, nil
}

// UpsertLibraryRoot adds root to the managed folder list if it is not already present.
func (s *Store) UpsertLibraryRoot(ctx context.Context, root string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	const query = `
INSERT INTO library_roots(path, created_at, last_scan_at)
VALUES (?, ?, '')
ON CONFLICT(path) DO NOTHING`
	if _, err := s.db.ExecContext(ctx, query, root, now); err != nil {
		return fmt.Errorf("save library root %q: %w", root, err)
	}
	return nil
}

// MarkLibraryRootScanned stores the successful scan timestamp for root.
func (s *Store) MarkLibraryRootScanned(ctx context.Context, root string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE library_roots SET last_scan_at=? WHERE path=?`, time.Now().UTC().Format(time.RFC3339Nano), root)
	if err != nil {
		return fmt.Errorf("update library root %q: %w", root, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected library-root rows: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("library root not found: %s", root)
	}
	return nil
}

// ListLibraryRoots returns all managed music folders.
func (s *Store) ListLibraryRoots(ctx context.Context) (roots []model.LibraryRoot, resultErr error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, created_at, last_scan_at FROM library_roots ORDER BY path COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list library roots: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close library-root rows: %w", err))
		}
	}()

	for rows.Next() {
		var root model.LibraryRoot
		if err := rows.Scan(&root.Path, &root.CreatedAt, &root.LastScanAt); err != nil {
			return nil, fmt.Errorf("scan library root: %w", err)
		}
		roots = append(roots, root)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate library roots: %w", err)
	}
	return roots, nil
}

// RemoveLibraryRoot removes a managed folder. When deleteTracks is true, indexed
// tracks below the root are removed from the database, but files on disk are untouched.
func (s *Store) RemoveLibraryRoot(ctx context.Context, root string, deleteTracks bool) (resultErr error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin remove-library-root transaction: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, fmt.Errorf("rollback remove-library-root transaction: %w", err))
		}
	}()

	if deleteTracks {
		if _, err := tx.ExecContext(ctx, `DELETE FROM tracks WHERE path LIKE ? ESCAPE '!' COLLATE NOCASE`, rootPattern(root)); err != nil {
			return fmt.Errorf("delete tracks below %q: %w", root, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM library_roots WHERE path=?`, root); err != nil {
		return fmt.Errorf("delete library root %q: %w", root, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit remove-library-root transaction: %w", err)
	}
	committed = true
	return nil
}

// LibraryStatistics returns aggregate counts and storage totals.
func (s *Store) LibraryStatistics(ctx context.Context) (model.LibraryStats, error) {
	const query = `
SELECT
    COUNT(*),
    COUNT(DISTINCT CASE WHEN TRIM(artist) <> '' THEN LOWER(TRIM(artist)) END),
    COUNT(DISTINCT CASE WHEN TRIM(album) <> '' THEN LOWER(TRIM(album_artist)) || char(0) || LOWER(TRIM(album)) END),
    COALESCE(SUM(duration_ms), 0),
    COALESCE(SUM(size), 0)
FROM tracks`
	var stats model.LibraryStats
	if err := s.db.QueryRowContext(ctx, query).Scan(&stats.Tracks, &stats.Artists, &stats.Albums, &stats.DurationMS, &stats.SizeBytes); err != nil {
		return model.LibraryStats{}, fmt.Errorf("load library statistics: %w", err)
	}
	return stats, nil
}

func rootPattern(root string) string {
	prefix := filepath.Clean(root) + string(filepath.Separator)
	replacer := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")
	return replacer.Replace(prefix) + "%"
}
