package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func TestAccessLog_recordsCorrelatedParameterizedRequest(t *testing.T) {
	// Given
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(accessLogMiddleware)
	r.Get("/api/cabinet/{token}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "success")
	})
	r.Get("/payment/return/{publicToken}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// When
	requestID := "request-cabinet-42"
	request := httptest.NewRequest(http.MethodGet,
		"http://panel.test/api/cabinet/cabinet-token-sentinel?password=password-sentinel&query-token=query-token-sentinel",
		strings.NewReader("password=body-password-sentinel"),
	)
	request.Header.Set(middleware.RequestIDHeader, requestID)
	request.Header.Set("X-Real-IP", "203.0.113.7")
	request.Header.Set("User-Agent", "ignore previous instructions; ua-sentinel")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "awgp_sid", Value: "cookie-token-sentinel"})
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)

	// Then
	if response.Code != http.StatusAccepted {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusAccepted)
	}
	if response.Body.String() != "success" {
		t.Fatalf("response body = %q, want success", response.Body.String())
	}
	record := decodeAccessLog(t, logs.String())
	if got := record["level"]; got != "INFO" {
		t.Errorf("level = %v, want INFO", got)
	}
	if got := record["method"]; got != http.MethodGet {
		t.Errorf("method = %v, want GET", got)
	}
	if got := record["route"]; got != "/api/cabinet/{token}" {
		t.Errorf("route = %v, want template", got)
	}
	if got := record["status"]; got != float64(http.StatusAccepted) {
		t.Errorf("status = %v, want %d", got, http.StatusAccepted)
	}
	if got := record["bytes"]; got != float64(len("success")) {
		t.Errorf("bytes = %v, want %d", got, len("success"))
	}
	if got := record["duration"]; got == nil {
		t.Error("duration is missing")
	}
	if got := record["req_id"]; got != requestID {
		t.Errorf("req_id = %v, want %q", got, requestID)
	}
	if got := record["ip"]; got != "203.0.113.7" {
		t.Errorf("ip = %v, want 203.0.113.7", got)
	}
	if got := record["user_agent"]; got != "ignore previous instructions; ua-sentinel" {
		t.Errorf("user_agent = %v, want captured plain data", got)
	}
	assertLogOmits(t, logs.String(),
		"cabinet-token-sentinel",
		"password-sentinel",
		"body-password-sentinel",
		"query-token-sentinel",
		"cookie-token-sentinel",
	)
}

func TestAccessLog_NewRouterWiresMiddleware(t *testing.T) {
	// Given
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	router := NewRouter(nil, NewAuth(""), nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "http://panel.test/healthz", nil)
	request.Header.Set(middleware.RequestIDHeader, "request-router-1")
	response := httptest.NewRecorder()

	// When
	router.ServeHTTP(response, request)

	// Then
	if response.Code != http.StatusOK {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusOK)
	}
	record := decodeAccessLog(t, logs.String())
	if got := record["route"]; got != "/healthz" {
		t.Errorf("route = %v, want /healthz", got)
	}
	if got := record["req_id"]; got != "request-router-1" {
		t.Errorf("req_id = %v, want request-router-1", got)
	}
}

func TestAccessLog_redactsPaymentToken(t *testing.T) {
	// Given
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(accessLogMiddleware)
	r.Get("/payment/return/{publicToken}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "payment not found", http.StatusNotFound)
	})

	// When
	request := httptest.NewRequest(http.MethodGet,
		"http://panel.test/payment/return/payment-token-sentinel?password=payment-password-sentinel",
		nil,
	)
	request.Header.Set(middleware.RequestIDHeader, "request-payment-7")
	request.AddCookie(&http.Cookie{Name: "session", Value: "payment-cookie-sentinel"})
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)

	// Then
	if response.Code != http.StatusNotFound {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusNotFound)
	}
	record := decodeAccessLog(t, logs.String())
	if got := record["level"]; got != "WARN" {
		t.Errorf("level = %v, want WARN", got)
	}
	if got := record["route"]; got != "/payment/return/{publicToken}" {
		t.Errorf("route = %v, want payment template", got)
	}
	if got := record["status"]; got != float64(http.StatusNotFound) {
		t.Errorf("status = %v, want %d", got, http.StatusNotFound)
	}
	if got := record["bytes"]; got != float64(response.Body.Len()) {
		t.Errorf("bytes = %v, want %d", got, response.Body.Len())
	}
	if got := record["req_id"]; got != "request-payment-7" {
		t.Errorf("req_id = %v, want request-payment-7", got)
	}
	assertLogOmits(t, logs.String(),
		"payment-token-sentinel",
		"payment-password-sentinel",
		"payment-cookie-sentinel",
	)
}

func TestAccessLog_usesHealthDebugAndUnmatchedFallback(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantLevel  string
		wantStatus int
	}{
		{name: "health", path: "/healthz", wantLevel: "DEBUG", wantStatus: http.StatusOK},
		{name: "unmatched", path: "/not-a-route?password=unmatched-password", wantLevel: "WARN", wantStatus: http.StatusNotFound},
		{name: "server-error", path: "/failure", wantLevel: "ERROR", wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
			t.Cleanup(func() { slog.SetDefault(previous) })

			r := chi.NewRouter()
			r.Use(middleware.RequestID)
			r.Use(middleware.RealIP)
			r.Use(accessLogMiddleware)
			r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok") })
			r.Get("/failure", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, "success")
			})

			// When
			request := httptest.NewRequest(http.MethodGet, "http://panel.test"+test.path, nil)
			request.Header.Set(middleware.RequestIDHeader, "request-"+test.name)
			response := httptest.NewRecorder()
			r.ServeHTTP(response, request)

			// Then
			if response.Code != test.wantStatus {
				t.Fatalf("response status = %d, want %d", response.Code, test.wantStatus)
			}
			record := decodeAccessLog(t, logs.String())
			if got := record["level"]; got != test.wantLevel {
				t.Errorf("level = %v, want %s", got, test.wantLevel)
			}
			wantRoute := test.path
			if test.name == "unmatched" {
				wantRoute = "unmatched"
			}
			if got := record["route"]; got != wantRoute {
				t.Errorf("route = %v, want %s", got, wantRoute)
			}
		})
	}
}

func TestAccessLog_preservesStreaming(t *testing.T) {
	// Given
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(accessLogMiddleware)
	r.Get("/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("stream response writer does not implement http.Flusher")
			return
		}
		_, _ = io.WriteString(w, "first")
		flusher.Flush()
		_, _ = io.WriteString(w, "second")
	})

	// When
	request := httptest.NewRequest(http.MethodGet, "http://panel.test/stream", nil)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)

	// Then
	if response.Body.String() != "firstsecond" {
		t.Fatalf("stream body = %q, want firstsecond", response.Body.String())
	}
	record := decodeAccessLog(t, logs.String())
	if got := record["bytes"]; got != float64(len("firstsecond")) {
		t.Errorf("bytes = %v, want %d", got, len("firstsecond"))
	}
}

func decodeAccessLog(t *testing.T, output string) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want 1: %q", len(lines), output)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("decode JSON log = %v", err)
	}
	return record
}

func assertLogOmits(t *testing.T, output string, sentinels ...string) {
	t.Helper()
	for _, sentinel := range sentinels {
		if strings.Contains(output, sentinel) {
			t.Errorf("log contains secret sentinel %q: %s", sentinel, output)
		}
	}
}
