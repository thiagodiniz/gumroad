// Package httpapi exposes the deliverer over HTTP for the Rails client
// (app/services/ping_delivery_service_client.rb).
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/antiwork/gumroad/services/ping-delivery/internal/delivery"
)

const maxRequestBody = 4 << 20

type Deliverer interface {
	Deliver(ctx context.Context, req delivery.Request) delivery.Outcome
}

type Server struct {
	deliverer Deliverer
	token     string
	log       *slog.Logger
}

func New(deliverer Deliverer, internalToken string, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{deliverer: deliverer, token: internalToken, log: log}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.healthz)
	mux.Handle("POST /v1/deliveries", s.authenticated(http.HandlerFunc(s.deliver)))
	return s.logging(mux)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type deliveryRequest struct {
	URL         string `json:"url"`
	Body        string `json:"body"`
	ContentType string `json:"content_type"`
}

// deliveryResponse is a verdict either way: Rails records and retries from it exactly as it
// would from the in-process SsrfFilter.post result.
type deliveryResponse struct {
	Outcome    string `json:"outcome"` // "responded" | "error"
	Status     int    `json:"status,omitempty"`
	ErrorClass string `json:"error_class,omitempty"`
	Retryable  bool   `json:"retryable"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (s *Server) deliver(w http.ResponseWriter, r *http.Request) {
	var in deliveryRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
	if err != nil || json.Unmarshal(body, &in) != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_json"})
		return
	}
	if in.URL == "" || in.ContentType == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_request"})
		return
	}

	outcome := s.deliverer.Deliver(r.Context(), delivery.Request{URL: in.URL, Body: in.Body, ContentType: in.ContentType})
	resp := deliveryResponse{Outcome: "responded", Status: outcome.Status}
	if outcome.ErrorClass != "" {
		resp = deliveryResponse{Outcome: "error", ErrorClass: outcome.ErrorClass, Retryable: outcome.Retryable}
	}
	// Endpoint URLs and payloads carry license keys and buyer emails; only the verdict is logged.
	s.log.InfoContext(r.Context(), "delivery", "outcome", resp.Outcome, "status", resp.Status, "error_class", resp.ErrorClass)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" {
			got := r.Header.Get("X-Internal-Token")
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
				writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.InfoContext(r.Context(), "request",
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
