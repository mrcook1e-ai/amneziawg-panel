package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
)

// ---- admin: subscribers ----------------------------------------------------

type subscriberCreateBody struct {
	Name        string `json:"name"`
	Notes       string `json:"notes"`
	BillingRole string `json:"billingRole"`
}

func (h *Handlers) subscriberCreate(w http.ResponseWriter, r *http.Request) {
	var in subscriberCreateBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	s, err := h.Mgr.CreateSubscriber(in.Name, in.Notes, in.BillingRole)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, subscriberOut(r, s))
}

func (h *Handlers) subscribersList(w http.ResponseWriter, r *http.Request) {
	views := h.Mgr.ListSubscribers()
	out := make([]map[string]any, 0, len(views))
	for _, v := range views {
		out = append(out, subscriberViewOut(r, v))
	}
	writeJSON(w, 200, out)
}

func (h *Handlers) subscriberGet(w http.ResponseWriter, r *http.Request) {
	v, err := h.Mgr.SubscriberDetail(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, subscriberViewOut(r, v))
}

type subscriberPatchBody struct {
	Name        *string `json:"name"`
	Notes       *string `json:"notes"`
	BillingRole *string `json:"billingRole"`
}

func (h *Handlers) subscriberPatch(w http.ResponseWriter, r *http.Request) {
	var in subscriberPatchBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	s, err := h.Mgr.PatchSubscriber(chi.URLParam(r, "id"), in.Name, in.Notes, in.BillingRole)
	if err != nil {
		writeErr(w, err)
		return
	}
	if in.BillingRole != nil && s.BillingRole != awg.BillingRolePayer {
		if err := h.Mgr.ResumeSubscriberClients(s.ID); err != nil {
			writeErr(w, err)
			return
		}
	}
	writeJSON(w, 200, subscriberOut(r, s))
}

func (h *Handlers) subscriberRegenToken(w http.ResponseWriter, r *http.Request) {
	s, err := h.Mgr.RegenerateAccessToken(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, subscriberOut(r, s))
}

func (h *Handlers) subscriberDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.Mgr.DeleteSubscriber(chi.URLParam(r, "id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

// ---- response shaping helpers ---------------------------------------------

func subscriberOut(r *http.Request, s *awg.Subscriber) map[string]any {
	role := s.BillingRole
	if role == "" {
		role = "trusted"
	}
	return map[string]any{
		"id":          s.ID,
		"name":        s.Name,
		"accessToken": s.AccessToken,
		"url":         cabinetURL(r, s.AccessToken),
		"notes":       s.Notes,
		"createdAt":   s.CreatedAt,
		"billingRole": role,
	}
}

func subscriberViewOut(r *http.Request, v awg.SubscriberView) map[string]any {
	role := v.BillingRole
	if role == "" {
		role = "trusted"
	}
	out := map[string]any{
		"id":          v.ID,
		"name":        v.Name,
		"accessToken": v.AccessToken,
		"url":         cabinetURL(r, v.AccessToken),
		"notes":       v.Notes,
		"createdAt":   v.CreatedAt,
		"deviceCount": v.DeviceCount,
		"billingRole": role,
	}
	if v.Devices != nil {
		out["devices"] = v.Devices
	}
	return out
}

func cabinetURL(r *http.Request, token string) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	return scheme + "://" + host + "/cabinet/" + token
}
