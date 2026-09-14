package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// UpdateTrackTags synchronizes editable metadata and file state after a successful tag write.
func (s *Store) UpdateTrackTags(ctx context.Context, id int64, tags model.TagSnapshot, size, modifiedUnix int64) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE tracks SET
    title=?, artist=?, album=?, album_artist=?, genre=?, year=?,
    track_number=?, track_total=?, disc_number=?, disc_total=?, composer=?, comment=?,
    label=?, catalog_number=?, isrc=?, release_date=?,
    size=?, modified_unix=?, updated_at=?
WHERE id=?`,
		tags.Title, tags.Artist, tags.Album, tags.AlbumArtist, tags.Genre, tags.Year,
		tags.TrackNumber, tags.TrackTotal, tags.DiscNumber, tags.DiscTotal, tags.Composer, tags.Comment,
		tags.Label, tags.CatalogNumber, tags.ISRC, tags.ReleaseDate,
		size, modifiedUnix, time.Now().UTC().Format(time.RFC3339Nano), id,
	)
	if err != nil {
		return fmt.Errorf("update tags for track %d: %w", id, err)
	}
	return ensureAffected(res, id)
}

// UpdateTrackCoverPresence stores a known embedded-artwork presence result.
func (s *Store) UpdateTrackCoverPresence(ctx context.Context, id int64, hasCover bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tracks SET has_cover=?, cover_indexed=1 WHERE id=?`, boolToInt(hasCover), id)
	if err != nil {
		return fmt.Errorf("update cover presence for track %d: %w", id, err)
	}
	return ensureAffected(res, id)
}

// InvalidateTrackCoverPresence marks the cached artwork presence as unknown.
func (s *Store) InvalidateTrackCoverPresence(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tracks SET cover_indexed=0 WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("invalidate cover presence for track %d: %w", id, err)
	}
	return ensureAffected(res, id)
}

// BeginTagChange creates a reversible metadata change set.
func (s *Store) BeginTagChange(ctx context.Context, label string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO tag_change_sets(created_at, label, status, affected_count) VALUES (?, ?, 'pending', 0)`,
		time.Now().UTC().Format(time.RFC3339Nano), label)
	if err != nil {
		return 0, fmt.Errorf("create tag change set: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read tag change-set id: %w", err)
	}
	return id, nil
}

// AddTagChangeItem records one successfully changed track inside a change set.
func (s *Store) AddTagChangeItem(ctx context.Context, changeSetID, trackID int64, before, after model.TagSnapshot, beforeCoverPath string, coverChanged bool) error {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("encode before-tag snapshot: %w", err)
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode after-tag snapshot: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO tag_change_items(change_set_id, track_id, before_json, after_json, before_cover_path, cover_changed)
VALUES (?, ?, ?, ?, ?, ?)`, changeSetID, trackID, string(beforeJSON), string(afterJSON), beforeCoverPath, boolToInt(coverChanged)); err != nil {
		return fmt.Errorf("record tag change for track %d: %w", trackID, err)
	}
	return nil
}

// FinishTagChange finalizes a change set. Empty sets are deleted rather than cluttering history.
func (s *Store) FinishTagChange(ctx context.Context, id int64, affected int, status string) error {
	if affected == 0 {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM tag_change_sets WHERE id=?`, id); err != nil {
			return fmt.Errorf("delete empty tag change set %d: %w", id, err)
		}
		return nil
	}
	res, err := s.db.ExecContext(ctx, `UPDATE tag_change_sets SET status=?, affected_count=? WHERE id=?`, status, affected, id)
	if err != nil {
		return fmt.Errorf("finalize tag change set %d: %w", id, err)
	}
	return ensureChangeSetAffected(res, id)
}

// ListTagHistory returns recent reversible change sets, newest first.
func (s *Store) ListTagHistory(ctx context.Context, limit int) (history []model.TagHistory, resultErr error) {
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, created_at, label, status, affected_count
FROM tag_change_sets
WHERE affected_count > 0
ORDER BY id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list tag history: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close tag-history rows: %w", err))
		}
	}()

	for rows.Next() {
		var item model.TagHistory
		if err := rows.Scan(&item.ID, &item.CreatedAt, &item.Label, &item.Status, &item.AffectedCount); err != nil {
			return nil, fmt.Errorf("scan tag history: %w", err)
		}
		history = append(history, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tag history: %w", err)
	}
	return history, nil
}

// TagChangeItem is the persisted state required to undo one track.
type TagChangeItem struct {
	TrackID         int64
	Before          model.TagSnapshot
	After           model.TagSnapshot
	BeforeCoverPath string
	CoverChanged    bool
}

// TagChangeItems loads all items belonging to a change set.
func (s *Store) TagChangeItems(ctx context.Context, changeSetID int64) (items []TagChangeItem, resultErr error) {
	var status string
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM tag_change_sets WHERE id=?`, changeSetID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("tag change set %d not found: %w", changeSetID, err)
		}
		return nil, fmt.Errorf("load tag change set %d: %w", changeSetID, err)
	}
	if status != "applied" {
		return nil, fmt.Errorf("tag change set %d cannot be undone from status %q", changeSetID, status)
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT track_id, before_json, after_json, before_cover_path, cover_changed
FROM tag_change_items
WHERE change_set_id=?
ORDER BY id DESC`, changeSetID)
	if err != nil {
		return nil, fmt.Errorf("load tag change items: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close tag-change rows: %w", err))
		}
	}()

	for rows.Next() {
		var item TagChangeItem
		var beforeJSON, afterJSON string
		var coverChanged int
		if err := rows.Scan(&item.TrackID, &beforeJSON, &afterJSON, &item.BeforeCoverPath, &coverChanged); err != nil {
			return nil, fmt.Errorf("scan tag change item: %w", err)
		}
		if err := json.Unmarshal([]byte(beforeJSON), &item.Before); err != nil {
			return nil, fmt.Errorf("decode before-tag snapshot: %w", err)
		}
		if err := json.Unmarshal([]byte(afterJSON), &item.After); err != nil {
			return nil, fmt.Errorf("decode after-tag snapshot: %w", err)
		}
		item.CoverChanged = coverChanged != 0
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tag change items: %w", err)
	}
	return items, nil
}

// MarkTagChangeUndone prevents a change set from being undone twice.
func (s *Store) MarkTagChangeUndone(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tag_change_sets SET status='undone' WHERE id=? AND status='applied'`, id)
	if err != nil {
		return fmt.Errorf("mark tag change set %d undone: %w", id, err)
	}
	return ensureChangeSetAffected(res, id)
}

func ensureChangeSetAffected(res sql.Result, id int64) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows for tag change set %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("tag change set %d not found or state changed", id)
	}
	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
