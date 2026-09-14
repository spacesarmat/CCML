// Package store implements the SQLite media-library persistence layer.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/spacesarmat/CCML/internal/model"
)

// Store wraps a SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens a SQLite database and applies the schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	if err := configure(db); err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close database after configure failure: %w", closeErr))
		}
		return nil, err
	}

	if err := migrate(db); err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close database after migration failure: %w", closeErr))
		}
		return nil, err
	}
	return &Store{db: db}, nil
}

func configure(db *sql.DB) error {
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("configure sqlite (%s): %w", pragma, err)
		}
	}
	return nil
}

func migrate(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    file_name TEXT NOT NULL,
    extension TEXT NOT NULL,
    size INTEGER NOT NULL DEFAULT 0,
    modified_unix INTEGER NOT NULL DEFAULT 0,
    title TEXT NOT NULL DEFAULT '',
    artist TEXT NOT NULL DEFAULT '',
    album TEXT NOT NULL DEFAULT '',
    album_artist TEXT NOT NULL DEFAULT '',
    genre TEXT NOT NULL DEFAULT '',
    year INTEGER NOT NULL DEFAULT 0,
    track_number INTEGER NOT NULL DEFAULT 0,
    track_total INTEGER NOT NULL DEFAULT 0,
    disc_number INTEGER NOT NULL DEFAULT 0,
    disc_total INTEGER NOT NULL DEFAULT 0,
    composer TEXT NOT NULL DEFAULT '',
    comment TEXT NOT NULL DEFAULT '',
    label TEXT NOT NULL DEFAULT '',
    catalog_number TEXT NOT NULL DEFAULT '',
    isrc TEXT NOT NULL DEFAULT '',
    release_date TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    codec TEXT NOT NULL DEFAULT '',
    sample_rate INTEGER NOT NULL DEFAULT 0,
    channels INTEGER NOT NULL DEFAULT 0,
    bit_rate INTEGER NOT NULL DEFAULT 0,
    bpm REAL NOT NULL DEFAULT 0,
    musical_key TEXT NOT NULL DEFAULT '',
    key_scale TEXT NOT NULL DEFAULT '',
    loudness_i REAL NOT NULL DEFAULT 0,
    true_peak REAL NOT NULL DEFAULT 0,
    lra REAL NOT NULL DEFAULT 0,
    threshold REAL NOT NULL DEFAULT 0,
    scan_error TEXT NOT NULL DEFAULT '',
    has_cover INTEGER NOT NULL DEFAULT 0,
    cover_indexed INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tracks_artist_title ON tracks(artist, title);
CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album);
CREATE INDEX IF NOT EXISTS idx_tracks_duration ON tracks(duration_ms);

CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS library_roots (
    path TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    last_scan_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS scan_entries (
    path TEXT PRIMARY KEY,
    root_path TEXT NOT NULL,
    last_seen_scan TEXT NOT NULL,
    FOREIGN KEY(path) REFERENCES tracks(path) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_scan_entries_root ON scan_entries(root_path, last_seen_scan);

CREATE TABLE IF NOT EXISTS tag_change_sets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TEXT NOT NULL,
    label TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'applied',
    affected_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS tag_change_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    change_set_id INTEGER NOT NULL,
    track_id INTEGER NOT NULL,
    before_json TEXT NOT NULL,
    after_json TEXT NOT NULL,
    before_cover_path TEXT NOT NULL DEFAULT '',
    cover_changed INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY(change_set_id) REFERENCES tag_change_sets(id) ON DELETE CASCADE,
    FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_tag_change_items_set ON tag_change_items(change_set_id);

CREATE TABLE IF NOT EXISTS metadata_lookup_cache (
    cache_key TEXT PRIMARY KEY,
    result_json TEXT NOT NULL,
    created_unix INTEGER NOT NULL,
    expires_unix INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_metadata_lookup_cache_expires ON metadata_lookup_cache(expires_unix);

CREATE TABLE IF NOT EXISTS essentia_analysis (
    track_id INTEGER PRIMARY KEY,
    bpm REAL NOT NULL DEFAULT 0,
    musical_key TEXT NOT NULL DEFAULT '',
    key_scale TEXT NOT NULL DEFAULT '',
    strength REAL NOT NULL DEFAULT 0,
    profile TEXT NOT NULL DEFAULT '',
    requested_mode TEXT NOT NULL DEFAULT '',
    effective_mode TEXT NOT NULL DEFAULT '',
    analyzed_at TEXT NOT NULL,
    FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS dj_pool_consensus (
    track_id INTEGER PRIMARY KEY,
    bpm REAL NOT NULL DEFAULT 0,
    bpm_support INTEGER NOT NULL DEFAULT 0,
    bpm_quality REAL NOT NULL DEFAULT 0,
    camelot TEXT NOT NULL DEFAULT '',
    key_support INTEGER NOT NULL DEFAULT 0,
    key_quality REAL NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS background_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    options_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL,
    total_items INTEGER NOT NULL DEFAULT 0,
    completed_items INTEGER NOT NULL DEFAULT 0,
    skipped_items INTEGER NOT NULL DEFAULT 0,
    failed_items INTEGER NOT NULL DEFAULT 0,
    cancelled_items INTEGER NOT NULL DEFAULT 0,
    current_item TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_background_jobs_status ON background_jobs(status, id);

CREATE TABLE IF NOT EXISTS background_job_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL,
    track_id INTEGER NOT NULL,
    path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    attempts INTEGER NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    result_json TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL,
    FOREIGN KEY(job_id) REFERENCES background_jobs(id) ON DELETE CASCADE,
    FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_background_job_items_job_status ON background_job_items(job_id, status, id);
CREATE INDEX IF NOT EXISTS idx_background_job_items_track ON background_job_items(track_id, id DESC);
`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate sqlite schema: %w", err)
	}
	for _, column := range []struct {
		table string
		name  string
		ddl   string
	}{
		{table: "tracks", name: "track_total", ddl: "INTEGER NOT NULL DEFAULT 0"},
		{table: "tracks", name: "disc_total", ddl: "INTEGER NOT NULL DEFAULT 0"},
		{table: "tracks", name: "composer", ddl: "TEXT NOT NULL DEFAULT ''"},
		{table: "tracks", name: "comment", ddl: "TEXT NOT NULL DEFAULT ''"},
		{table: "tracks", name: "label", ddl: "TEXT NOT NULL DEFAULT ''"},
		{table: "tracks", name: "catalog_number", ddl: "TEXT NOT NULL DEFAULT ''"},
		{table: "tracks", name: "isrc", ddl: "TEXT NOT NULL DEFAULT ''"},
		{table: "tracks", name: "release_date", ddl: "TEXT NOT NULL DEFAULT ''"},
		{table: "tracks", name: "has_cover", ddl: "INTEGER NOT NULL DEFAULT 0"},
		{table: "tracks", name: "cover_indexed", ddl: "INTEGER NOT NULL DEFAULT 0"},
		{table: "tag_change_items", name: "cover_changed", ddl: "INTEGER NOT NULL DEFAULT 0"},
		{table: "background_jobs", name: "cancelled_items", ddl: "INTEGER NOT NULL DEFAULT 0"},
		{table: "essentia_analysis", name: "profile", ddl: "TEXT NOT NULL DEFAULT ''"},
		{table: "essentia_analysis", name: "requested_mode", ddl: "TEXT NOT NULL DEFAULT ''"},
		{table: "essentia_analysis", name: "effective_mode", ddl: "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := ensureColumn(db, column.table, column.name, column.ddl); err != nil {
			return err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT OR IGNORE INTO schema_version(version, applied_at) VALUES (1, ?), (2, ?), (3, ?), (4, ?), (5, ?), (6, ?), (7, ?), (8, ?), (9, ?), (10, ?), (11, ?)`, now, now, now, now, now, now, now, now, now, now, now); err != nil {
		return fmt.Errorf("record sqlite schema version: %w", err)
	}
	return nil
}

// Close closes the SQLite database.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close sqlite database: %w", err)
	}
	return nil
}

// UpsertTrack inserts or updates a scanned track.
func (s *Store) UpsertTrack(ctx context.Context, t model.Track) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	const query = `
INSERT INTO tracks (
    path, file_name, extension, size, modified_unix,
    title, artist, album, album_artist, genre, year, track_number, track_total, disc_number, disc_total, composer, comment,
    duration_ms, codec, sample_rate, channels, bit_rate, scan_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
    file_name=excluded.file_name,
    extension=excluded.extension,
    size=excluded.size,
    modified_unix=excluded.modified_unix,
    title=excluded.title,
    artist=excluded.artist,
    album=excluded.album,
    album_artist=excluded.album_artist,
    genre=excluded.genre,
    year=excluded.year,
    track_number=excluded.track_number,
    track_total=excluded.track_total,
    disc_number=excluded.disc_number,
    disc_total=excluded.disc_total,
    composer=excluded.composer,
    comment=excluded.comment,
    duration_ms=excluded.duration_ms,
    codec=excluded.codec,
    sample_rate=excluded.sample_rate,
    channels=excluded.channels,
    bit_rate=excluded.bit_rate,
    scan_error=excluded.scan_error,
    updated_at=excluded.updated_at
RETURNING id`

	var id int64
	err := s.db.QueryRowContext(ctx, query,
		t.Path, t.FileName, t.Extension, t.Size, t.ModifiedUnix,
		t.Title, t.Artist, t.Album, t.AlbumArtist, t.Genre, t.Year, t.TrackNumber, t.TrackTotal, t.DiscNumber, t.DiscTotal, t.Composer, t.Comment,
		t.DurationMS, t.Codec, t.SampleRate, t.Channels, t.BitRate, t.ScanError, now, now,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert track %q: %w", t.Path, err)
	}
	return id, nil
}

// TrackByID loads a track by primary key.
func (s *Store) TrackByID(ctx context.Context, id int64) (model.Track, error) {
	const query = `SELECT ` + trackColumns + ` FROM tracks WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	t, err := scanTrack(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Track{}, fmt.Errorf("track %d not found: %w", id, err)
		}
		return model.Track{}, fmt.Errorf("load track %d: %w", id, err)
	}
	return t, nil
}

// ListTracks returns a filtered, paginated track list.
func (s *Store) ListTracks(ctx context.Context, search string, limit, offset int) (tracks []model.Track, resultErr error) {
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	if offset < 0 {
		offset = 0
	}

	search = strings.TrimSpace(search)
	args := make([]any, 0, 5)
	query := `SELECT ` + trackColumns + ` FROM tracks`
	if search != "" {
		like := "%" + search + "%"
		query += ` WHERE title LIKE ? COLLATE NOCASE OR artist LIKE ? COLLATE NOCASE OR album LIKE ? COLLATE NOCASE OR path LIKE ? COLLATE NOCASE`
		args = append(args, like, like, like, like)
	}
	query += ` ORDER BY artist COLLATE NOCASE, album COLLATE NOCASE, disc_number, track_number, title COLLATE NOCASE LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tracks: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close track rows: %w", err))
		}
	}()

	tracks = make([]model.Track, 0, limit)
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, fmt.Errorf("scan track row: %w", err)
		}
		tracks = append(tracks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tracks: %w", err)
	}
	return tracks, nil
}

// AllTracks loads the complete library for duplicate analysis.
func (s *Store) AllTracks(ctx context.Context) (tracks []model.Track, resultErr error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+trackColumns+` FROM tracks ORDER BY artist, title, duration_ms`)
	if err != nil {
		return nil, fmt.Errorf("load all tracks: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close all-track rows: %w", err))
		}
	}()

	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, fmt.Errorf("scan track row: %w", err)
		}
		tracks = append(tracks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all tracks: %w", err)
	}
	return tracks, nil
}

// UpdateLoudness stores EBU R128 measurements.
func (s *Store) UpdateLoudness(ctx context.Context, id int64, l model.Loudness) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tracks SET loudness_i=?, true_peak=?, lra=?, threshold=?, updated_at=? WHERE id=?`,
		l.InputI, l.InputTP, l.InputLRA, l.InputThreshold, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("update loudness for track %d: %w", id, err)
	}
	return ensureAffected(res, id)
}

// UpdateBPMKey stores optional Essentia analysis.
func (s *Store) UpdateBPMKey(ctx context.Context, id int64, result model.BPMKey) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tracks SET bpm=?, musical_key=?, key_scale=?, updated_at=? WHERE id=?`,
		result.BPM, result.Key, result.Scale, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("update BPM/key for track %d: %w", id, err)
	}
	return ensureAffected(res, id)
}

// UpdateTrackPath updates a path after a successful move/rename operation.
func (s *Store) UpdateTrackPath(ctx context.Context, id int64, path, fileName, extension string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tracks SET path=?, file_name=?, extension=?, updated_at=? WHERE id=?`,
		path, fileName, extension, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("update path for track %d: %w", id, err)
	}
	return ensureAffected(res, id)
}

func ensureAffected(res sql.Result, id int64) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows for track %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("track %d not found", id)
	}
	return nil
}

const trackColumns = `id, path, file_name, extension, size, modified_unix,
 title, artist, album, album_artist, genre, year, track_number, track_total, disc_number, disc_total, composer, comment, label, catalog_number, isrc, release_date,
 duration_ms, codec, sample_rate, channels, bit_rate,
 bpm, musical_key, key_scale, loudness_i, true_peak, lra, threshold, scan_error,
 has_cover, cover_indexed,
 COALESCE((SELECT bji.status FROM background_job_items bji JOIN background_jobs bj ON bj.id=bji.job_id WHERE bji.track_id=tracks.id AND bj.type='metadata_enrichment' ORDER BY bji.id DESC LIMIT 1), ''),
 COALESCE((SELECT bji.updated_at FROM background_job_items bji JOIN background_jobs bj ON bj.id=bji.job_id WHERE bji.track_id=tracks.id AND bj.type='metadata_enrichment' ORDER BY bji.id DESC LIMIT 1), '')`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTrack(row rowScanner) (model.Track, error) {
	var t model.Track
	var hasCover, coverIndexed int
	err := row.Scan(
		&t.ID, &t.Path, &t.FileName, &t.Extension, &t.Size, &t.ModifiedUnix,
		&t.Title, &t.Artist, &t.Album, &t.AlbumArtist, &t.Genre, &t.Year, &t.TrackNumber, &t.TrackTotal, &t.DiscNumber, &t.DiscTotal, &t.Composer, &t.Comment, &t.Label, &t.CatalogNumber, &t.ISRC, &t.ReleaseDate,
		&t.DurationMS, &t.Codec, &t.SampleRate, &t.Channels, &t.BitRate,
		&t.BPM, &t.Key, &t.KeyScale, &t.LoudnessI, &t.TruePeak, &t.LRA, &t.Threshold, &t.ScanError,
		&hasCover, &coverIndexed,
		&t.LastMetadataJobStatus, &t.LastMetadataJobUpdatedAt,
	)
	if err != nil {
		return model.Track{}, err
	}
	t.HasCover = hasCover != 0
	t.CoverIndexed = coverIndexed != 0
	return t, nil
}
