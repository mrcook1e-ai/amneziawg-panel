package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/mrcook1e/amneziawg-panel/internal/billing"
)

type HandlersBilling struct {
	Service *billing.Service
}

// RegisterBillingRoutes registers both admin and public endpoints.
func (h *HandlersBilling) RegisterBillingRoutes(r chi.Router, auth *Auth) {
	// Admin billing endpoints
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware)
		r.Route("/api/billing", func(r chi.Router) {
			r.Get("/cycles", h.listCycles)
			r.Post("/cycles", h.createCycle)
			r.Get("/cycles/{id}", h.getCycle)
			r.Get("/cycles/{id}/preview", h.previewSplit)
			r.Post("/cycles/{id}/publish", h.publishCycle)
			r.Post("/cycles/{id}/close", h.closeCycle)
			r.Delete("/cycles/{id}", h.deleteCycle)
			r.Post("/invoices/{id}/pay", h.markInvoicePaid)
			r.Post("/invoices/{id}/cancel", h.cancelInvoice)
			r.Get("/summary", h.getSummary)
		})
	})

	// Public billing endpoints
	r.Get("/api/cabinet/{token}/billing", h.getCabinetSummary)
	r.Post("/api/cabinet/{token}/billing/checkout", h.initiateCheckout)
	r.Post("/api/billing/yookassa/webhook", h.yookassaWebhook)
	r.Get("/payment/return/{publicToken}", h.paymentReturn)
}

func (h *HandlersBilling) listCycles(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	cycles, err := h.Service.ListCycles(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cycles)
}

func (h *HandlersBilling) createCycle(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	var in struct {
		Title        string `json:"title"`
		PeriodStart  int64  `json:"periodStart"`
		PeriodEnd    int64  `json:"periodEnd"`
		PaymentDueAt int64  `json:"paymentDueAt"`
		GraceEndsAt  int64  `json:"graceEndsAt"`
		TotalAmount  int64  `json:"totalAmount"`
		SplitMode    string `json:"splitMode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	c, err := h.Service.CreateDraftCycle(r.Context(), in.Title, in.PeriodStart, in.PeriodEnd, in.PaymentDueAt, in.GraceEndsAt, in.TotalAmount, in.SplitMode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(c)
}

func (h *HandlersBilling) getCycle(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}

	c, err := h.Service.GetCycleDetail(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.Error(w, "cycle not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

func (h *HandlersBilling) previewSplit(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	lines, err := h.Service.PreviewSplit(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(lines)
}

func (h *HandlersBilling) publishCycle(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.Service.PublishCycle(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *HandlersBilling) markInvoicePaid(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.Service.MarkInvoicePaid(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *HandlersBilling) cancelInvoice(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.Service.CancelInvoice(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *HandlersBilling) closeCycle(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.Service.CloseCycle(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *HandlersBilling) deleteCycle(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.Service.DeleteCycle(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *HandlersBilling) getSummary(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	sum, err := h.Service.GetSummary(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sum)
}

func (h *HandlersBilling) getCabinetSummary(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	token := chi.URLParam(r, "token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}

	sum, err := h.Service.GetCabinetSummary(r.Context(), token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sum)
}

func (h *HandlersBilling) initiateCheckout(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	var in struct {
		InvoiceID int64  `json:"invoiceId"`
		Email     string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	in.Email = strings.TrimSpace(in.Email)
	if _, err := mail.ParseAddress(in.Email); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите корректный email для чека"})
		return
	}

	url, err := h.Service.InitiateCheckout(r.Context(), chi.URLParam(r, "token"), in.InvoiceID, in.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"confirmationUrl": url})
}

func (h *HandlersBilling) yookassaWebhook(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	// Limit body to avoid memory exhaustion
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	if err := h.Service.HandleYookassaWebhook(r.Context(), body); err != nil {
		log.Printf("webhook error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *HandlersBilling) paymentReturn(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "billing service disabled", http.StatusNotImplemented)
		return
	}
	publicToken := chi.URLParam(r, "publicToken")
	if publicToken == "" {
		http.Error(w, "publicToken required", http.StatusBadRequest)
		return
	}

	accessToken, paymentStatus, err := h.Service.ReconcilePaymentByPublicToken(r.Context(), publicToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	baseURL := h.Service.Cfg.PublicURL
	if baseURL == "" {
		// Use request host/origin
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		baseURL = scheme + "://" + r.Host
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	targetURL := fmt.Sprintf("%s/cabinet/%s?payment=%s", baseURL, accessToken, paymentStatus)
	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}
