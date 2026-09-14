package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// DeleteTracksByID removes indexed tracks in one SQLite transaction.
// Foreign-key cascades clean dependent scan/tag/job rows.
func (s *Store) DeleteTracksByID(ctx context.Context, ids []int64) (resultErr error) {
	if len(ids) == 0 {
		return errors.New("no track ids supplied")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete-tracks transaction: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, fmt.Errorf("rollback delete-tracks transaction: %w", err))
		}
	}()

	for _, id := range ids {
		res, err := tx.ExecContext(ctx, `DELETE FROM tracks WHERE id=?`, id)
		if err != nil {
			return fmt.Errorf("delete track %d: %w", id, err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("read deleted rows for track %d: %w", id, err)
		}
		if affected != 1 {
			return fmt.Errorf("track %d not found while deleting", id)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete-tracks transaction: %w", err)
	}
	committed = true
	return nil
}
