// Package events is a tiny append-and-tail journal backed by SQLite. Used
// for the activity feed on the dashboard and for audit-style "who flipped
// what when" lookups in per-client views.
package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/mrcook1e/amneziawg-panel/internal/db"
)

// Kinds — kept as plain strings so the UI can pattern-match without a shared
// enum module. Add freely; old kinds stay readable.
const (
	KindClientCreated  = "client.created"
	KindClientDeleted  = "client.deleted"
	KindClientEnabled  = "client.enabled"
	KindClientDisabled = "client.disabled"
	KindClientRenamed  = "client.renamed"
	KindClientExpired  = "client.expired"
	KindClientPatched  = "client.patched"
	KindServerRestart  = "server.restart"
	KindServerMagic    = "server.regen_magic"
	KindServerReset    = "server.reset_clients"
)

type Event struct {
	ID       int64           `json:"id"`
	Ts       time.Time       `json:"ts"`
	Kind     string          `json:"kind"`
	ClientID string          `json:"clientId,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

type Log struct {
	db   *db.DB
	mu   sync.RWMutex
	subs []func(Event)
}

func New(d *db.DB) *Log { return &Log{db: d} }

// Subscribe регистрирует callback, который будет вызван (синхронно, но из
// горутины Append) сразу после записи события в БД. Используется SSE-брокером
// для realtime-push. Отписки нет — рассчитан на единичных подписчиков уровня
// процесса.
func (l *Log) Subscribe(fn func(Event)) {
	l.mu.Lock()
	l.subs = append(l.subs, fn)
	l.mu.Unlock()
}

// Append writes one event. Errors are silently swallowed at the call site —
// missing an audit row is never worth failing a user request over. Caller can
// pass nil payload for plain events.
func (l *Log) Append(kind, clientID string, payload any) {
	if l == nil || l.db == nil {
		return
	}
	var raw []byte
	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		if err != nil {
			slog.Warn("event payload encoding failed", slog.String("component", "events"), slog.String("operation", "marshal"), slog.Any("error", err))
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	now := time.Now()
	res, err := l.db.ExecContext(ctx,
		`INSERT INTO events(ts, kind, client_id, payload) VALUES(?, ?, ?, ?)`,
		now.Unix(), kind, nullStr(clientID), nullBytes(raw),
	)
	if err != nil {
		slog.Error("event append failed", slog.String("component", "events"), slog.String("operation", "append"), slog.Any("error", err))
	}
	// Broadcast подписчикам (SSE). Не блокируем при ошибке записи в БД —
	// audit-сообщение всё равно ценно для UI.
	l.mu.RLock()
	subs := l.subs
	l.mu.RUnlock()
	if len(subs) == 0 {
		return
	}
	var id int64
	if res != nil {
		id, err = res.LastInsertId()
		if err != nil {
			slog.Warn("event identifier unavailable", slog.String("component", "events"), slog.String("operation", "last_insert_id"), slog.Any("error", err))
		}
	}
	ev := Event{ID: id, Ts: now.UTC(), Kind: kind, ClientID: clientID, Payload: raw}
	for _, fn := range subs {
		fn(ev)
	}
}

// Tail returns the most recent N events, newest first. Optional clientID
// filter — empty means "all events".
func (l *Log) Tail(ctx context.Context, clientID string, limit int) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	var (
		rows *sql.Rows
		err  error
	)
	if clientID == "" {
		rows, err = l.db.QueryContext(ctx,
			`SELECT id, ts, kind, COALESCE(client_id,''), COALESCE(payload, x'')
			 FROM events ORDER BY id DESC LIMIT ?`, limit)
	} else {
		rows, err = l.db.QueryContext(ctx,
			`SELECT id, ts, kind, COALESCE(client_id,''), COALESCE(payload, x'')
			 FROM events WHERE client_id = ? ORDER BY id DESC LIMIT ?`, clientID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var ts int64
		var raw []byte
		if err := rows.Scan(&e.ID, &ts, &e.Kind, &e.ClientID, &raw); err != nil {
			return nil, err
		}
		e.Ts = time.Unix(ts, 0).UTC()
		if len(raw) > 0 {
			e.Payload = raw
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
