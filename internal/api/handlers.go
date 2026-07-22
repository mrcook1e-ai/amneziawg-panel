package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/billing"
)

// writeQRChunks serialises a slice of PNG bytes as JSON {"chunks":["base64..."]}.
func writeQRChunks(w http.ResponseWriter, pngs [][]byte) {
	chunks := make([]string, len(pngs))
	for i, p := range pngs {
		chunks[i] = base64.StdEncoding.EncodeToString(p)
	}
	writeJSON(w, 200, map[string]any{"chunks": chunks})
}

type Handlers struct {
	Mgr      *awg.Manager
	Auth     *Auth
	Billing  *billing.Service
	Lang     string
	limiter  *bucketLimiter
	cabLimit *bucketLimiter
}

func (h *Handlers) loginLimiter() *bucketLimiter {
	if h.limiter == nil {
		h.limiter = newLoginLimiter()
	}
	return h.limiter
}

func (h *Handlers) cabinetLimiter() *bucketLimiter {
	if h.cabLimit == nil {
		h.cabLimit = newCabinetLimiter()
	}
	return h.cabLimit
}

func (h *Handlers) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handlers) profilesList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, h.Mgr.ListProfiles())
}

func (h *Handlers) sessionGet(w http.ResponseWriter, r *http.Request) {
	authed := !h.Auth.Required()
	if !authed {
		if c, err := r.Cookie(sessionCookie); err == nil && h.Auth.Valid(c.Value) {
			authed = true
		}
	}
	writeJSON(w, 200, map[string]any{
		"requiresPassword": h.Auth.Required(),
		"authenticated":    authed,
	})
}

func (h *Handlers) sessionPost(w http.ResponseWriter, r *http.Request) {
	if !h.loginLimiter().allow(clientIP(r)) {
		writeJSON(w, 429, map[string]string{"error": "Too many attempts — try again in a minute"})
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Bad Request"})
		return
	}
	tok, ok := h.Auth.Login(in.Password)
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "Incorrect Password"})
		return
	}
	setSessionCookie(w, tok)
	writeJSON(w, 200, map[string]bool{"success": true})
}

func (h *Handlers) sessionDelete(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		h.Auth.Logout(c.Value)
	}
	clearSessionCookie(w)
	writeJSON(w, 200, map[string]bool{"success": true})
}

func (h *Handlers) clientsList(w http.ResponseWriter, r *http.Request) {
	clients, err := h.Mgr.ListClients()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, clients)
}

// Direct admin device creation is gone. Devices are produced by the
// subscriber's cabinet (/api/cabinet/:token/devices). The legacy import
// endpoint stays as a recovery escape hatch — see handlers_admin.go.

func (h *Handlers) clientDelete(w http.ResponseWriter, r *http.Request) {
	// Admin path — pass empty actorSubID so subscriber-scoping is bypassed.
	if err := h.Mgr.DeleteDevice(chi.URLParam(r, "id"), ""); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

func (h *Handlers) clientEnable(w http.ResponseWriter, r *http.Request)  { h.setEnabled(w, r, true) }
func (h *Handlers) clientDisable(w http.ResponseWriter, r *http.Request) { h.setEnabled(w, r, false) }

func (h *Handlers) setEnabled(w http.ResponseWriter, r *http.Request, v bool) {
	if err := h.Mgr.SetEnabled(chi.URLParam(r, "id"), v); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

func (h *Handlers) clientRename(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if err := h.Mgr.Rename(chi.URLParam(r, "id"), in.Name); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

func (h *Handlers) clientAddress(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if err := h.Mgr.SetAddress(chi.URLParam(r, "id"), in.Address); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

var nameSanitize = regexp.MustCompile(`[^a-zA-Z0-9_=+.\-]`)
var dashRuns = regexp.MustCompile(`-{2,}|-$`)

func (h *Handlers) clientConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, conf, err := h.Mgr.ClientConfig(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	name := nameSanitize.ReplaceAllString(c.Name, "-")
	name = dashRuns.ReplaceAllString(name, "-")
	name = strings.TrimSuffix(name, "-")
	if len(name) > 32 {
		name = name[:32]
	}
	if name == "" {
		name = id
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.conf"`)
	w.Write(conf)
}

func (h *Handlers) clientQR(w http.ResponseWriter, r *http.Request) {
	_, conf, err := h.Mgr.ClientConfig(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	png, err := qrcode.Encode(string(conf), qrcode.Medium, 512)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}

func (h *Handlers) clientAmneziaVPN(w http.ResponseWriter, r *http.Request) {
	url, err := h.Mgr.AmneziaVPNURL(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(url))
}

func (h *Handlers) clientAmneziaQR(w http.ResponseWriter, r *http.Request) {
	url, err := h.Mgr.AmneziaVPNURL(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	// vpn:// URLs are long (often 1-2 KB) — High error correction would push us
	// past QR version 40's capacity. Low EC + larger image keeps phone cameras
	// happy.
	png, err := qrcode.Encode(url, qrcode.Low, 768)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}

func (h *Handlers) clientAmneziaQRChunks(w http.ResponseWriter, r *http.Request) {
	pngs, err := h.Mgr.AmneziaVPNChunks(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeQRChunks(w, pngs)
}

// serverResetClients keeps the existing "wipe every client" surface but it now
// wipes across all profiles.
func (h *Handlers) serverResetClients(w http.ResponseWriter, r *http.Request) {
	if err := h.Mgr.ResetClients(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	if awg.IsNotFound(err) || awg.IsSubscriberNotFound(err) {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 500, map[string]string{"error": err.Error()})
}
