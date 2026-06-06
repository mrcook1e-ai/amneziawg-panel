package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/db"
	"github.com/mrcook1e/amneziawg-panel/internal/events"
	"github.com/mrcook1e/amneziawg-panel/internal/stats"
)

// StatsHandlers groups everything that depends on the metrics DB / event log.
// Kept separate from the original Handlers struct so the existing API surface
// stays untouched when these services aren't wired (e.g. in tests).
type StatsHandlers struct {
	Mgr    *awg.Manager
	DB     *db.DB
	Events *events.Log
}

func (s *StatsHandlers) overview(w http.ResponseWriter, r *http.Request) {
	ov, err := stats.GetOverview(r.Context(), s.DB)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, ov)
}

func (s *StatsHandlers) series(w http.ResponseWriter, r *http.Request) {
	win := parseDuration(r.URL.Query().Get("range"), 24*time.Hour)
	bucket := pickBucket(win)
	pts, err := stats.Series(r.Context(), s.DB, "", win, bucket)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{
		"bucketSeconds": int(bucket.Seconds()),
		"points":        pts,
	})
}

func (s *StatsHandlers) clientStats(w http.ResponseWriter, r *http.Request) {
	cs, err := stats.GetClientStats(r.Context(), s.DB, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, cs)
}

func (s *StatsHandlers) clientEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	ev, err := s.Events.Tail(r.Context(), chi.URLParam(r, "id"), limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, ev)
}

func (s *StatsHandlers) eventsTail(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	ev, err := s.Events.Tail(r.Context(), "", limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, ev)
}

func (s *StatsHandlers) clientPatch(w http.ResponseWriter, r *http.Request) {
	var in awg.ClientPatch
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Bad Request"})
		return
	}
	c, err := s.Mgr.PatchClient(chi.URLParam(r, "id"), in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, c)
}

// parseDuration parses suffix-style ranges: "5m", "1h", "24h", "7d", "30d".
// Falls back to def on anything unrecognised. Clamped to [5m, 90d] to keep
// the queries fast.
func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	if d, err := time.ParseDuration(s); err == nil {
		return clampDur(d)
	}
	// custom "d" suffix
	if len(s) > 1 && s[len(s)-1] == 'd' {
		if n, err := strconv.Atoi(s[:len(s)-1]); err == nil {
			return clampDur(time.Duration(n) * 24 * time.Hour)
		}
	}
	return def
}

func clampDur(d time.Duration) time.Duration {
	switch {
	case d < 5*time.Minute:
		return 5 * time.Minute
	case d > 90*24*time.Hour:
		return 90 * 24 * time.Hour
	default:
		return d
	}
}

// pickBucket chooses a sensible bucket width given the window — we want
// roughly 50–200 points back to draw a clean sparkline.
func pickBucket(window time.Duration) time.Duration {
	switch {
	case window <= time.Hour:
		return time.Minute
	case window <= 6*time.Hour:
		return 5 * time.Minute
	case window <= 24*time.Hour:
		return 15 * time.Minute
	case window <= 7*24*time.Hour:
		return time.Hour
	default:
		return 6 * time.Hour
	}
}
