// Package httpapi exposes the oEmbed finder over HTTP for the Rails client
// (app/services/o_embed_service_client.rb).
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/antiwork/gumroad/services/oembed/internal/oembed"
)

const maxRequestBody = 8 << 10

type Finder interface {
	Lookup(ctx context.Context, pageURL string, maxWidth int) (*oembed.Embeddable, error)
}

type Server struct {
	finder Finder
	token  string
	log    *slog.Logger
}

func New(finder Finder, internalToken string, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{finder: finder, token: internalToken, log: log}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.healthz)
	mux.Handle("POST /v1/embeds", s.authenticated(http.HandlerFunc(s.embed)))
	return s.logging(mux)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type embedRequest struct {
	URL      string `json:"url"`
	MaxWidth int    `json:"maxwidth"`
}

// embedResponse.Embeddable is null when the URL is not embeddable; that is a definitive
// verdict, not an error.
type embedResponse struct {
	Embeddable *oembed.Embeddable `json:"embeddable"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (s *Server) embed(w http.ResponseWriter, r *http.Request) {
	var in embedRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
	if err != nil || json.Unmarshal(body, &in) != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_json"})
		return
	}
	if u, err := url.Parse(in.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_request"})
		return
	}

	emb, err := s.finder.Lookup(r.Context(), in.URL, in.MaxWidth)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal_error"
		if errors.Is(err, oembed.ErrUpstream) || errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusBadGateway
			code = "upstream_unavailable"
		}
		s.log.WarnContext(r.Context(), "lookup failed", "url", in.URL, "error", err)
		writeJSON(w, status, errorResponse{Error: code})
		return
	}
	writeJSON(w, http.StatusOK, embedResponse{Embeddable: emb})
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
