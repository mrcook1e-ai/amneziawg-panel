package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
)

type Handlers struct {
	Mgr     *awg.Manager
	Auth    *Auth
	Lang    string
	limiter *loginLimiter
}

func (h *Handlers) loginLimiter() *loginLimiter {
	if h.limiter == nil {
		h.limiter = newLoginLimiter()
	}
	return h.limiter
}

func (h *Handlers) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
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

func (h *Handlers) clientCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name      string `json:"name"`
		ProfileID string `json:"profileId"`
		Notes     string `json:"notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if _, err := h.Mgr.CreateClient(awg.CreateClientArgs{
		Name:      in.Name,
		ProfileID: in.ProfileID,
		Notes:     in.Notes,
	}); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

func (h *Handlers) clientMove(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProfileID string `json:"profileId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Bad Request"})
		return
	}
	if err := h.Mgr.MoveClient(chi.URLParam(r, "id"), in.ProfileID); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

func (h *Handlers) clientDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.Mgr.DeleteClient(chi.URLParam(r, "id")); err != nil {
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
	_ = json.NewDecoder(r.Body).Decode(&in)
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
	_ = json.NewDecoder(r.Body).Decode(&in)
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

func (h *Handlers) clientVPN(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, link, err := h.Mgr.ClientAmneziaVPN(id, "AmneziaWG Panel")
	if err != nil {
		writeErr(w, err)
		return
	}
	name := nameSanitize.ReplaceAllString(c.Name, "-")
	name = dashRuns.ReplaceAllString(name, "-")
	name = strings.TrimSuffix(name, "-")
	if name == "" {
		name = id
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.vpn"`)
	w.Write([]byte(link))
}

func (h *Handlers) clientVPNQR(w http.ResponseWriter, r *http.Request) {
	_, link, err := h.Mgr.ClientAmneziaVPN(chi.URLParam(r, "id"), "AmneziaWG Panel")
	if err != nil {
		writeErr(w, err)
		return
	}
	png, err := qrcode.Encode(link, qrcode.Low, 1024)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
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
	if awg.IsNotFound(err) {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	if awg.IsProfileHasClients(err) {
		writeJSON(w, 409, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 500, map[string]string{"error": err.Error()})
}
