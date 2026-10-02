// Package httpapi exposes the validator router over HTTP for the Rails client
// (app/services/tax_id_validation_service_client.rb).
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/antiwork/gumroad/services/tax-id-validation/internal/validator"
)

const maxRequestBody = 4 << 10

type Router interface {
	Validate(ctx context.Context, req validator.Request) (validator.Result, error)
}

type Server struct {
	router Router
	token  string
	log    *slog.Logger
}

func New(router Router, internalToken string, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{router: router, token: internalToken, log: log}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.healthz)
	mux.Handle("POST /v1/validations", s.authenticated(http.HandlerFunc(s.validate)))
	return s.logging(mux)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type validateRequest struct {
	TaxID       string `json:"tax_id"`
	CountryCode string `json:"country_code"`
	StateCode   string `json:"state_code"`
}

type validateResponse struct {
	Valid     bool   `json:"valid"`
	Validator string `json:"validator"`
}

type errorResponse struct {
	Error     string `json:"error"`
	Validator string `json:"validator,omitempty"`
}

func (s *Server) validate(w http.ResponseWriter, r *http.Request) {
	var in validateRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
	if err != nil || json.Unmarshal(body, &in) != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_json"})
		return
	}
	if strings.TrimSpace(in.TaxID) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_request"})
		return
	}

	res, err := s.router.Validate(r.Context(), validator.Request{
		TaxID:       in.TaxID,
		CountryCode: in.CountryCode,
		StateCode:   in.StateCode,
	})
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal_error"
		if errors.Is(err, validator.ErrUpstream) || errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusBadGateway
			code = "upstream_unavailable"
		}
		s.log.WarnContext(r.Context(), "validation failed",
			"validator", res.Validator, "country", in.CountryCode, "error", err)
		writeJSON(w, status, errorResponse{Error: code, Validator: res.Validator})
		return
	}
	writeJSON(w, http.StatusOK, validateResponse{Valid: res.Valid, Validator: res.Validator})
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
