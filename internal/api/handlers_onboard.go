package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
)

// ---- admin: manage invite tokens ------------------------------------------

type tokenCreateBody struct {
	Name      string `json:"name"`
	ExpiresIn int    `json:"expiresIn"` // seconds, 0 = no expiry
}

func (h *Handlers) tokenCreate(w http.ResponseWriter, r *http.Request) {
	var in tokenCreateBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	ttl := time.Duration(in.ExpiresIn) * time.Second
	t, err := h.Mgr.CreateToken(in.Name, ttl)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, tokenViewWithURL(r, t))
}

func (h *Handlers) tokenList(w http.ResponseWriter, r *http.Request) {
	out := h.Mgr.ListTokens()
	type augmented struct {
		awg.TokenView
		URL string `json:"url"`
	}
	resp := make([]augmented, 0, len(out))
	for _, t := range out {
		resp = append(resp, augmented{TokenView: t, URL: inviteURL(r, t.Token)})
	}
	writeJSON(w, 200, resp)
}

func (h *Handlers) tokenRevoke(w http.ResponseWriter, r *http.Request) {
	if err := h.Mgr.RevokeToken(chi.URLParam(r, "id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

func tokenViewWithURL(r *http.Request, t *awg.OnboardToken) map[string]any {
	return map[string]any{
		"id":         t.ID,
		"token":      t.Token,
		"name":       t.Name,
		"createdAt":  t.CreatedAt,
		"expiresAt":  t.ExpiresAt,
		"status":     t.Status(time.Now().UTC()),
		"url":        inviteURL(r, t.Token),
	}
}

// inviteURL builds the public link from the incoming request's scheme + host.
// Behind a reverse proxy you'd want X-Forwarded-Proto/Host; the panel already
// uses chi's middleware.RealIP but not full forwarded resolution. Good enough
// for a hosted single-tenant install; can be tightened later if needed.
func inviteURL(r *http.Request, token string) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	return scheme + "://" + host + "/onboard/" + token
}

// ---- public: redeem a token (no auth required) -----------------------------

func (h *Handlers) onboardStatus(w http.ResponseWriter, r *http.Request) {
	st := h.Mgr.TokenStatusPublic(chi.URLParam(r, "token"))
	writeJSON(w, 200, st)
}

type onboardRedeemBody struct {
	Snippet    string `json:"snippet"`
	ClientName string `json:"clientName"`
}

func (h *Handlers) onboardRedeem(w http.ResponseWriter, r *http.Request) {
	var in onboardRedeemBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if strings.TrimSpace(in.Snippet) == "" {
		writeJSON(w, 400, map[string]string{"error": "snippet is required"})
		return
	}
	spec, err := awg.ParseObfuscation(in.Snippet)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "snippet: " + err.Error()})
		return
	}
	_, conf, err := h.Mgr.RedeemToken(chi.URLParam(r, "token"), in.ClientName, spec)
	if err != nil {
		switch {
		case awg.IsTokenNotFound(err):
			writeJSON(w, 404, map[string]string{"error": "invite not found"})
		case awg.IsTokenUsed(err):
			writeJSON(w, 409, map[string]string{"error": "invite already used"})
		case awg.IsTokenExpired(err):
			writeJSON(w, 410, map[string]string{"error": "invite expired"})
		default:
			writeErr(w, err)
		}
		return
	}
	png, err := qrcode.Encode(string(conf), qrcode.Medium, 512)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "qr encode failed"})
		return
	}
	writeJSON(w, 200, awg.RedeemResult{
		Conf:    string(conf),
		QRPng64: base64.StdEncoding.EncodeToString(png),
	})
}
