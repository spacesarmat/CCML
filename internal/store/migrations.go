package store

import (
	"database/sql"
	"fmt"
)

func ensureColumn(db *sql.DB, table, column, ddl string) error {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return fmt.Errorf("inspect sqlite table %s: %w", table, err)
	}

	found := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				return fmt.Errorf("scan sqlite table-info row: %w (close rows: %v)", err, closeErr)
			}
			return fmt.Errorf("scan sqlite table-info row: %w", err)
		}
		if name == column {
			found = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		if closeErr := rows.Close(); closeErr != nil {
			return fmt.Errorf("iterate sqlite table-info rows: %w (close rows: %v)", err, closeErr)
		}
		return fmt.Errorf("iterate sqlite table-info rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close sqlite table-info rows: %w", err)
	}
	if found {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + ddl); err != nil {
		return fmt.Errorf("add sqlite column %s.%s: %w", table, column, err)
	}
	return nil
}
