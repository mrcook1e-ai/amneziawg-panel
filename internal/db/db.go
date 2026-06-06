// Package db owns the SQLite store used for metrics, events and any other
// time-series data the panel needs. Client configuration itself still lives
// in the JSON state file owned by internal/awg — keep these two stores
// separate: SQLite for append-heavy time data, JSON for the authoritative
// config that gets rendered to .conf.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB with retention helpers.
type DB struct{ *sql.DB }

// Open opens (and creates if missing) the sqlite file at dir/panel.db, sets
// WAL + reasonable pragmas, and runs migrations.
func Open(dir string) (*DB, error) {
	path := filepath.Join(dir, "panel.db")
	// _pragma=busy_timeout=5000 prevents transient "database is locked" under
	// the collector+API write/read contention. WAL keeps reads non-blocking.
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc sqlite is single-connection friendly; cap to avoid lock storms.
	sqldb.SetMaxOpenConns(4)
	sqldb.SetMaxIdleConns(2)
	sqldb.SetConnMaxLifetime(time.Hour)

	if err := migrate(sqldb); err != nil {
		sqldb.Close()
		return nil, err
	}
	return &DB{sqldb}, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS peer_samples (
			client_id  TEXT    NOT NULL,
			ts         INTEGER NOT NULL,
			rx_delta   INTEGER NOT NULL,
			tx_delta   INTEGER NOT NULL,
			handshake  INTEGER,
			PRIMARY KEY (client_id, ts)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_samples_ts ON peer_samples(ts)`,

		`CREATE TABLE IF NOT EXISTS peer_daily (
			client_id      TEXT    NOT NULL,
			day            INTEGER NOT NULL,
			rx             INTEGER NOT NULL,
			tx             INTEGER NOT NULL,
			online_seconds INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (client_id, day)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_day ON peer_daily(day)`,

		`CREATE TABLE IF NOT EXISTS events (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			ts        INTEGER NOT NULL,
			kind      TEXT    NOT NULL,
			client_id TEXT,
			payload   TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts)`,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("migrate: %w (%s)", err, s)
		}
	}
	return nil
}

// SampleRetentionDays defines how long raw per-bucket samples are kept.
// Daily aggregates outlive them — see DailyRetentionDays.
const (
	SampleRetentionDays = 7
	DailyRetentionDays  = 365
	EventsRetentionDays = 30
)

// Prune drops rows past retention windows. Cheap when there's nothing to drop.
func (d *DB) Prune(ctx context.Context, now time.Time) error {
	cutS := now.Add(-time.Duration(SampleRetentionDays) * 24 * time.Hour).Unix()
	cutD := now.Add(-time.Duration(DailyRetentionDays) * 24 * time.Hour).Unix()
	cutE := now.Add(-time.Duration(EventsRetentionDays) * 24 * time.Hour).Unix()
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM peer_samples WHERE ts < ?`, cutS); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM peer_daily WHERE day < ?`, cutD); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM events WHERE ts < ?`, cutE); err != nil {
		return err
	}
	return tx.Commit()
}
