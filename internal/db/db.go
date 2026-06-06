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
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB with retention helpers.
type DB struct{ *sql.DB }

// Open opens (and creates if missing) the sqlite file at dir/panel.db, sets
// WAL + reasonable pragmas, and runs migrations.
//
// Важно: DSN — простой путь без "file:" префикса и URI-параметров. У modernc
// /sqlite парсер раньше путался на конструкциях вроде ?_pragma=journal_mode(WAL),
// открывая в результате не тот файл (миграции писались в правильный panel.db,
// а runtime-запросы летели в анонимную инстанцию). Pragma применяем через
// Exec после открытия — поведение одинаковое на всех версиях драйвера.
func Open(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir state dir: %w", err)
	}
	path := filepath.Join(dir, "panel.db")
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Одно соединение: SQLite сам сериализует доступ, лишние коннекты
	// провоцируют "database is locked" под нагрузкой коллектор+API.
	sqldb.SetMaxOpenConns(1)
	sqldb.SetMaxIdleConns(1)
	sqldb.SetConnMaxLifetime(0)

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := sqldb.Exec(p); err != nil {
			sqldb.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}

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

// Reset очищает все таблицы метрик и журнал событий. Используется factory-
// reset'ом из API. Структуру таблиц не трогаем — данные удаляем, схема жива.
func (d *DB) Reset(ctx context.Context) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range []string{"peer_samples", "peer_daily", "events"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+t); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// VACUUM нельзя внутри транзакции, и он медленный — пропускаем.
	return nil
}

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
