// Package stats owns the time-series side of the panel: a periodic ticker
// that diffs `awg show dump` against the previous reading and writes per-
// bucket bytes deltas into peer_samples, plus a daily roll-up into
// peer_daily for cheap long-range queries.
//
// Design notes:
//   - The kernel byte counters in `awg show dump` are cumulative since
//     `awg-quick up`. They reset when the interface is restarted. We detect
//     a reset as "current < last" and treat it as "delta = current" rather
//     than producing a negative spike.
//   - The collector is the single writer to peer_samples / peer_daily, so
//     no transactions across multiple inserts are necessary for correctness;
//     we still batch per-tick into one tx for speed.
//   - All time math is unix seconds in UTC. Display formatting lives in the
//     frontend.
package stats

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/db"
	"github.com/mrcook1e/amneziawg-panel/internal/events"
)

const (
	// BucketSeconds is the resolution we store at: one row per peer per minute.
	// A week of 1-minute samples per peer = ~10k rows; trivially queryable.
	BucketSeconds = 60

	// OnlineWindow defines how recently a handshake must be for a peer to
	// count as "online" in the live dashboard tile. WireGuard's keepalive is
	// usually 25s — a 3-minute window tolerates one missed rotation.
	OnlineWindow = 3 * time.Minute
)

// Collector is started once at boot and runs until ctx is cancelled.
type Collector struct {
	DB     *db.DB
	Mgr    *awg.Manager
	Events *events.Log
	Tick time.Duration // typically 30s
	Bin  string        // path to awg binary

	mu   sync.Mutex
	prev map[string]peerCounters // keyed by public key, populated lazily
}

type peerCounters struct {
	rx, tx uint64
}

func (c *Collector) Run(ctx context.Context) {
	if c.Tick == 0 {
		c.Tick = 30 * time.Second
	}
	c.prev = make(map[string]peerCounters)

	// One immediate tick so the dashboard isn't empty on first paint.
	c.tickOnce(ctx)

	t := time.NewTicker(c.Tick)
	defer t.Stop()

	// Independent slower tickers for housekeeping. We piggyback on the same
	// goroutine to keep ordering simple.
	expiryT := time.NewTicker(5 * time.Minute)
	defer expiryT.Stop()
	pruneT := time.NewTicker(1 * time.Hour)
	defer pruneT.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.tickOnce(ctx)
		case now := <-expiryT.C:
			c.runExpiry(now)
		case now := <-pruneT.C:
			if err := c.DB.Prune(ctx, now); err != nil {
				log.Printf("stats: prune: %v", err)
			}
		}
	}
}

func (c *Collector) tickOnce(ctx context.Context) {
	status := map[string]awg.PeerStatus{}
	for _, iface := range c.Mgr.IfaceNames() {
		st, err := awg.ShowDump(c.Bin, iface)
		if err != nil {
			continue
		}
		for k, v := range st {
			status[k] = v
		}
	}
	snap := c.Mgr.Snapshot()
	now := time.Now().UTC()
	bucket := now.Unix() / BucketSeconds * BucketSeconds

	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	upsert, err := tx.PrepareContext(ctx, `
		INSERT INTO peer_samples(client_id, ts, rx_delta, tx_delta, handshake)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(client_id, ts) DO UPDATE SET
			rx_delta = rx_delta + excluded.rx_delta,
			tx_delta = tx_delta + excluded.tx_delta,
			handshake = COALESCE(excluded.handshake, peer_samples.handshake)
	`)
	if err != nil {
		return
	}
	defer upsert.Close()

	c.mu.Lock()
	defer c.mu.Unlock()

	for pubKey, cl := range snap {
		ps, present := status[pubKey]
		if !present {
			continue
		}
		prev := c.prev[pubKey]
		rxDelta, txDelta := deltaOrReset(prev.rx, ps.RxBytes), deltaOrReset(prev.tx, ps.TxBytes)
		c.prev[pubKey] = peerCounters{rx: ps.RxBytes, tx: ps.TxBytes}

		var hs sql.NullInt64
		if ps.LatestHandshake != nil {
			hs.Valid = true
			hs.Int64 = ps.LatestHandshake.Unix()
		}

		if rxDelta == 0 && txDelta == 0 && !hs.Valid {
			continue
		}

		if _, err := upsert.ExecContext(ctx, cl.ID, bucket, rxDelta, txDelta, hs); err != nil {
			log.Printf("stats: upsert sample: %v", err)
			continue
		}

		// Apply lifetime totals + last_handshake back onto the JSON store so
		// the values survive restarts and show up in /api/wireguard/client.
		c.Mgr.ApplyTraffic(cl.ID, rxDelta, txDelta, ps.LatestHandshake)
	}

	if err := tx.Commit(); err != nil {
		return
	}
	committed = true

	// Daily rollup: idempotent UPSERT keyed by (client_id, day).
	day := dayBucket(now)
	_, _ = c.DB.ExecContext(ctx, `
		INSERT INTO peer_daily(client_id, day, rx, tx, online_seconds)
		SELECT client_id, ?, COALESCE(SUM(rx_delta),0), COALESCE(SUM(tx_delta),0),
			   COUNT(*) * ?
		FROM peer_samples
		WHERE ts >= ? AND ts < ?
		GROUP BY client_id
		ON CONFLICT(client_id, day) DO UPDATE SET
			rx = excluded.rx,
			tx = excluded.tx,
			online_seconds = excluded.online_seconds
	`, day, BucketSeconds, day, day+86400)
}

func (c *Collector) runExpiry(now time.Time) {
	flipped := c.Mgr.DisableExpired(now)
	for _, id := range flipped {
		c.Events.Append(events.KindClientExpired, id, nil)
	}
}

// deltaOrReset returns curr-prev unless an interface restart cleared the
// counter (curr<prev), in which case curr is the delta since the reset.
func deltaOrReset(prev, curr uint64) uint64 {
	if curr < prev {
		return curr
	}
	return curr - prev
}

// dayBucket returns midnight UTC for the day containing t (unix seconds).
func dayBucket(t time.Time) int64 {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix()
}
