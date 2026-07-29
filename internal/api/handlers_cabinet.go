package api

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
)

// sanitizeAllowedIPs accepts a comma-separated AllowedIPs string from an
// untrusted source (cabinet query string) and returns the normalized form
// or "" if the input is empty / fully invalid. Each entry must parse as a
// CIDR (net.ParseCIDR). Invalid entries are dropped silently — partial
// validity beats a 400 here because the cabinet UI builds the string from
// a curated CIDR list, and we want graceful degradation if a stale list
// leaks a malformed entry. Hard cap at 4096 entries / 32 KiB total to keep
// the URL bounded.
func sanitizeAllowedIPs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 32*1024 {
		return ""
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 4096 {
		parts = parts[:4096]
	}
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		// Accept both "1.2.3.4/24" and the IPv4-default sentinel "0.0.0.0/0".
		_, ipnet, err := net.ParseCIDR(t)
		if err != nil {
			continue
		}
		norm := ipnet.String()
		if _, dup := seen[norm]; dup {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}
	return strings.Join(out, ", ")
}

// Public cabinet — auth is the token in URL (magic-link). Subscriber sees
// their own devices, can add new ones (each device gets its own awgN
// interface with its own obfuscation snippet), and remove individual devices.

func (h *Handlers) cabinetGet(w http.ResponseWriter, r *http.Request) {
	v, err := h.Mgr.CabinetSnapshot(chi.URLParam(r, "token"))
	if err != nil {
		if awg.IsSubscriberNotFound(err) {
			writeJSON(w, 404, map[string]string{"error": "cabinet not found"})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

type cabinetAddDeviceBody struct {
	// Preset = network situation (auto|stealth|fast), not OS/device.
	// Default auto («обычная сеть») — WAN-safe; cabinet UI labels map 1:1.
	Preset string `json:"preset"`
	// Snippet is legacy: only used when Preset is empty. New cabinet UI
	// never sends it; server generation is the source of truth.
	Snippet    string `json:"snippet"`
	DeviceName string `json:"deviceName"`
}

func (h *Handlers) cabinetAddDevice(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	// Per-token throttle: even if a token leaks, an attacker can't drain the
	// port pool (10 ifaces) or hammer the key generator. Falls back to per-IP
	// when token is empty (router won't route empty, but defensive).
	rlKey := token
	if rlKey == "" {
		rlKey = clientIP(r)
	}
	if !h.cabinetLimiter().allow(rlKey) {
		writeJSON(w, 429, map[string]string{"error": "Too many device creations — try again in a minute"})
		return
	}
	var in cabinetAddDeviceBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}

	var spec awg.ObfuscationSpec
	var err error
	preset := strings.TrimSpace(strings.ToLower(in.Preset))
	if preset != "" || strings.TrimSpace(in.Snippet) == "" {
		// Server-side generation (preferred). Empty preset → auto.
		if preset == "" {
			preset = awg.PresetAuto
		}
		spec, err = awg.GenerateObfuscation(preset)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "obfuscation generate failed"})
			return
		}
	} else {
		spec, err = awg.ParseObfuscation(in.Snippet)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "snippet: " + err.Error()})
			return
		}
	}

	sub, err := h.Mgr.FindSubscriberByToken(token)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "cabinet not found"})
		return
	}
	if h.Billing != nil {
		allowed, err := h.Billing.SubscriberAccessAllowed(r.Context(), sub.ID)
		if err != nil {
			writeErr(w, err)
			return
		}
		if !allowed {
			writeJSON(w, http.StatusPaymentRequired, map[string]string{"error": "оплатите просроченный счёт перед добавлением устройства"})
			return
		}
	}

	c, conf, err := h.Mgr.AddDevice(sub.ID, in.DeviceName, spec)
	if err != nil {
		writeErr(w, err)
		return
	}
	png, err := qrcode.Encode(string(conf), qrcode.Medium, 512)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "qr encode failed"})
		return
	}
	writeJSON(w, 200, map[string]any{
		"deviceId": c.ID,
		"name":     c.Name,
		"address":  c.Address,
		"conf":     string(conf),
		"qrPng64":  base64.StdEncoding.EncodeToString(png),
	})
}

func (h *Handlers) cabinetDeviceDelete(w http.ResponseWriter, r *http.Request) {
	sub, err := h.Mgr.FindSubscriberByToken(chi.URLParam(r, "token"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "cabinet not found"})
		return
	}
	if err := h.Mgr.DeleteDevice(chi.URLParam(r, "devId"), sub.ID); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

// cabinetDeviceConfig re-renders the .conf for a device the subscriber owns —
// lets them re-download from the cabinet after the original creation. Auth is
// the token; we verify the device belongs to the holder of that token.
func (h *Handlers) cabinetDeviceConfig(w http.ResponseWriter, r *http.Request) {
	sub, err := h.Mgr.FindSubscriberByToken(chi.URLParam(r, "token"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "cabinet not found"})
		return
	}
	devID := chi.URLParam(r, "devId")
	c, conf, err := h.Mgr.ClientConfig(devID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if c.SubscriberID != sub.ID {
		writeJSON(w, 404, map[string]string{"error": "device not in this cabinet"})
		return
	}
	name := nameSanitize.ReplaceAllString(c.Name, "-")
	name = dashRuns.ReplaceAllString(name, "-")
	name = strings.TrimSuffix(name, "-")
	if name == "" {
		name = devID
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.conf"`)
	w.Write(conf)
}

// cabinetDeviceAmneziaVPN / QR — same data as the admin endpoint but auth via
// cabinet token, and verifies ownership before exposing the URL.
func (h *Handlers) cabinetDeviceAmneziaVPN(w http.ResponseWriter, r *http.Request) {
	sub, err := h.Mgr.FindSubscriberByToken(chi.URLParam(r, "token"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "cabinet not found"})
		return
	}
	devID := chi.URLParam(r, "devId")
	c, _, err := h.Mgr.ClientConfig(devID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if c.SubscriberID != sub.ID {
		writeJSON(w, 404, map[string]string{"error": "device not in this cabinet"})
		return
	}
	// Optional ?allowed_ips=… — sanitized CIDR list from cabinet split-tunnel UI.
	override := sanitizeAllowedIPs(r.URL.Query().Get("allowed_ips"))
	url, err := h.Mgr.AmneziaVPNURLWith(devID, override)
	if err != nil {
		writeErr(w, err)
		return
	}
	// Override-bearing responses must not be cached — different requests
	// for the same device can carry different AllowedIPs.
	if override != "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(url))
}

func (h *Handlers) cabinetDeviceAmneziaQR(w http.ResponseWriter, r *http.Request) {
	sub, err := h.Mgr.FindSubscriberByToken(chi.URLParam(r, "token"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "cabinet not found"})
		return
	}
	devID := chi.URLParam(r, "devId")
	c, _, err := h.Mgr.ClientConfig(devID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if c.SubscriberID != sub.ID {
		writeJSON(w, 404, map[string]string{"error": "device not in this cabinet"})
		return
	}
	override := sanitizeAllowedIPs(r.URL.Query().Get("allowed_ips"))
	url, err := h.Mgr.AmneziaVPNURLWith(devID, override)
	if err != nil {
		writeErr(w, err)
		return
	}
	png, err := qrcode.Encode(url, qrcode.Low, 768)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "qr encode failed"})
		return
	}
	if override != "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}

func (h *Handlers) cabinetDeviceAmneziaQRChunks(w http.ResponseWriter, r *http.Request) {
	sub, err := h.Mgr.FindSubscriberByToken(chi.URLParam(r, "token"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "cabinet not found"})
		return
	}
	devID := chi.URLParam(r, "devId")
	c, _, err := h.Mgr.ClientConfig(devID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if c.SubscriberID != sub.ID {
		writeJSON(w, 404, map[string]string{"error": "device not in this cabinet"})
		return
	}
	override := sanitizeAllowedIPs(r.URL.Query().Get("allowed_ips"))
	pngs, err := h.Mgr.AmneziaVPNChunksWith(devID, override)
	if err != nil {
		writeErr(w, err)
		return
	}
	if override != "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	writeQRChunks(w, pngs)
}

func (h *Handlers) cabinetDeviceQR(w http.ResponseWriter, r *http.Request) {
	sub, err := h.Mgr.FindSubscriberByToken(chi.URLParam(r, "token"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "cabinet not found"})
		return
	}
	devID := chi.URLParam(r, "devId")
	c, conf, err := h.Mgr.ClientConfig(devID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if c.SubscriberID != sub.ID {
		writeJSON(w, 404, map[string]string{"error": "device not in this cabinet"})
		return
	}
	png, err := qrcode.Encode(string(conf), qrcode.Medium, 512)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "qr encode failed"})
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}
