package stats

import (
	"context"
	"fmt"
	"time"

	"github.com/mrcook1e/amneziawg-panel/internal/db"
)

// Overview is the headline dashboard tile data.
type Overview struct {
	WindowSeconds int       `json:"windowSeconds"` // 300 (last 5 min)
	RxLast        uint64    `json:"rxLast"`        // bytes inbound in the window
	TxLast        uint64    `json:"txLast"`        // bytes outbound in the window
	RxToday       uint64    `json:"rxToday"`
	TxToday       uint64    `json:"txToday"`
	Top           []TopRow  `json:"top"`          // top talkers, 24h
	Asof          time.Time `json:"asof"`
}

type TopRow struct {
	ClientID string `json:"clientId"`
	Rx       uint64 `json:"rx"`
	Tx       uint64 `json:"tx"`
}

// Point is one bucket in a timeseries. Server emits zero-filled rows so the
// UI sparkline doesn't need to gap-fill.
type Point struct {
	Ts int64  `json:"ts"`
	Rx uint64 `json:"rx"`
	Tx uint64 `json:"tx"`
}

// Series returns aggregated rx/tx across all clients in fixed-width buckets
// spanning [now-window, now]. ClientID empty = all clients.
func Series(ctx context.Context, d *db.DB, clientID string, window time.Duration, bucket time.Duration) ([]Point, error) {
	if bucket < time.Minute {
		bucket = time.Minute
	}
	bSec := int64(bucket.Seconds())
	now := time.Now().UTC()
	from := now.Add(-window).Unix() / bSec * bSec
	to := now.Unix()/bSec*bSec + bSec

	args := []any{bSec, bSec, from, to}
	where := "ts >= ? AND ts < ?"
	if clientID != "" {
		where += " AND client_id = ?"
		args = []any{bSec, bSec, from, to, clientID}
	}

	q := fmt.Sprintf(`
		SELECT (ts / ?) * ? AS b,
		       COALESCE(SUM(rx_delta),0),
		       COALESCE(SUM(tx_delta),0)
		FROM peer_samples
		WHERE %s
		GROUP BY b
		ORDER BY b ASC
	`, where)

	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	have := make(map[int64]Point)
	for rows.Next() {
		var p Point
		if err := rows.Scan(&p.Ts, &p.Rx, &p.Tx); err != nil {
			return nil, err
		}
		have[p.Ts] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Zero-fill so the UI always gets a uniform array.
	out := make([]Point, 0, (to-from)/bSec)
	for t := from; t < to; t += bSec {
		if p, ok := have[t]; ok {
			out = append(out, p)
		} else {
			out = append(out, Point{Ts: t})
		}
	}
	return out, nil
}

// GetOverview computes the headline dashboard tiles in one shot.
func GetOverview(ctx context.Context, d *db.DB) (Overview, error) {
	now := time.Now().UTC()
	winSec := int64(300) // 5 min
	dayStart := dayBucket(now)

	out := Overview{WindowSeconds: int(winSec), Asof: now}

	if err := d.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(rx_delta),0), COALESCE(SUM(tx_delta),0)
		 FROM peer_samples WHERE ts >= ?`,
		now.Unix()-winSec,
	).Scan(&out.RxLast, &out.TxLast); err != nil {
		return out, err
	}

	if err := d.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(rx),0), COALESCE(SUM(tx),0)
		 FROM peer_daily WHERE day = ?`, dayStart,
	).Scan(&out.RxToday, &out.TxToday); err != nil {
		return out, err
	}

	rows, err := d.QueryContext(ctx, `
		SELECT client_id, SUM(rx_delta) AS rx, SUM(tx_delta) AS tx
		FROM peer_samples
		WHERE ts >= ?
		GROUP BY client_id
		ORDER BY (rx + tx) DESC
		LIMIT 3
	`, now.Unix()-86400)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var r TopRow
		if err := rows.Scan(&r.ClientID, &r.Rx, &r.Tx); err != nil {
			return out, err
		}
		out.Top = append(out.Top, r)
	}
	return out, rows.Err()
}

// ClientStats is the per-client summary panel.
type ClientStats struct {
	WindowSeconds  int     `json:"windowSeconds"`
	RxLast         uint64  `json:"rxLast"`
	TxLast         uint64  `json:"txLast"`
	Rx24h          uint64  `json:"rx24h"`
	Tx24h          uint64  `json:"tx24h"`
	Rx7d           uint64  `json:"rx7d"`
	Tx7d           uint64  `json:"tx7d"`
	OnlineRatio7d  float64 `json:"onlineRatio7d"`
	Series         []Point `json:"series"`
}

func GetClientStats(ctx context.Context, d *db.DB, clientID string) (ClientStats, error) {
	now := time.Now().UTC()
	out := ClientStats{WindowSeconds: 300}

	row := d.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(rx_delta),0), COALESCE(SUM(tx_delta),0)
		 FROM peer_samples WHERE client_id = ? AND ts >= ?`,
		clientID, now.Unix()-300)
	if err := row.Scan(&out.RxLast, &out.TxLast); err != nil {
		return out, err
	}

	row = d.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(rx_delta),0), COALESCE(SUM(tx_delta),0)
		 FROM peer_samples WHERE client_id = ? AND ts >= ?`,
		clientID, now.Unix()-86400)
	if err := row.Scan(&out.Rx24h, &out.Tx24h); err != nil {
		return out, err
	}

	day7 := dayBucket(now.Add(-7 * 24 * time.Hour))
	row = d.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(rx),0), COALESCE(SUM(tx),0), COALESCE(SUM(online_seconds),0)
		 FROM peer_daily WHERE client_id = ? AND day >= ?`,
		clientID, day7)
	var online7 int64
	if err := row.Scan(&out.Rx7d, &out.Tx7d, &online7); err != nil {
		return out, err
	}
	out.OnlineRatio7d = float64(online7) / float64(7*86400)
	if out.OnlineRatio7d > 1 {
		out.OnlineRatio7d = 1
	}

	pts, err := Series(ctx, d, clientID, 24*time.Hour, 15*time.Minute)
	if err != nil {
		return out, err
	}
	out.Series = pts
	return out, nil
}
