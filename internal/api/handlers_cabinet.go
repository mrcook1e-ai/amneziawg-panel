package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
)

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
	if strings.TrimSpace(in.Snippet) == "" {
		writeJSON(w, 400, map[string]string{"error": "snippet is required"})
		return
	}
	spec, err := awg.ParseObfuscation(in.Snippet)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "snippet: " + err.Error()})
		return
	}

	sub, err := h.Mgr.FindSubscriberByToken(token)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "cabinet not found"})
		return
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
