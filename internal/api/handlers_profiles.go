package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
)

func (h *Handlers) profilesList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, h.Mgr.ListProfiles())
}

func (h *Handlers) profileGet(w http.ResponseWriter, r *http.Request) {
	v, err := h.Mgr.ProfileInfo(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

type profileCreateBody struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	I1          string `json:"i1"`
	I2          string `json:"i2"`
	I3          string `json:"i3"`
	I4          string `json:"i4"`
	I5          string `json:"i5"`
}

func (h *Handlers) profileCreate(w http.ResponseWriter, r *http.Request) {
	var in profileCreateBody
	_ = json.NewDecoder(r.Body).Decode(&in)
	v, err := h.Mgr.CreateProfile(awg.ProfileSpec{
		ID: in.ID, Name: in.Name, Description: in.Description,
		I1: in.I1, I2: in.I2, I3: in.I3, I4: in.I4, I5: in.I5,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

type profilePatchBody struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	I1          *string `json:"i1"`
	I2          *string `json:"i2"`
	I3          *string `json:"i3"`
	I4          *string `json:"i4"`
	I5          *string `json:"i5"`
}

func (h *Handlers) profilePatch(w http.ResponseWriter, r *http.Request) {
	var in profilePatchBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Bad Request"})
		return
	}
	v, err := h.Mgr.PatchProfile(chi.URLParam(r, "id"), awg.ProfilePatch{
		Name: in.Name, Description: in.Description,
		I1: in.I1, I2: in.I2, I3: in.I3, I4: in.I4, I5: in.I5,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

func (h *Handlers) profileDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.Mgr.DeleteProfile(chi.URLParam(r, "id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

func (h *Handlers) profileRegenMagic(w http.ResponseWriter, r *http.Request) {
	v, err := h.Mgr.RegenerateMagic(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

func (h *Handlers) profileRestart(w http.ResponseWriter, r *http.Request) {
	if err := h.Mgr.RestartInterface(chi.URLParam(r, "id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}
