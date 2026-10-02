// Package delivery ports the network half of PostToIndividualPingEndpointWorker: one
// SSRF-filtered POST, re-sent across a bounded number of redirects, whose result is reported
// as a verdict (status code, or a Ruby exception class plus whether Rails should retry).
// Encoding the payload, recording the attempt and scheduling retries stay in Rails.
package delivery

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/antiwork/gumroad/services/ping-delivery/internal/ssrf"
)

type Request struct {
	URL         string
	Body        string
	ContentType string
}

// Outcome is either a final HTTP status (ErrorClass empty) or a failure named after the Ruby
// exception the in-process path would have raised. Retryable mirrors the worker's
// RETRYABLE_EXCEPTIONS vs INTERNET_EXCEPTIONS split.
type Outcome struct {
	Status     int
	ErrorClass string
	Retryable  bool
}

type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

type Deliverer struct {
	Resolve      ssrf.Resolver
	Dial         DialFunc // dials the validated ip:port; tests swap it for an httptest address
	MaxRedirects int
	OpenTimeout  time.Duration
	ReadTimeout  time.Duration
	userAgent    string
}

func New(maxRedirects int, openTimeout, readTimeout time.Duration) *Deliverer {
	return &Deliverer{
		Resolve:      ssrf.DefaultResolver,
		Dial:         (&net.Dialer{Timeout: openTimeout}).DialContext,
		MaxRedirects: maxRedirects,
		OpenTimeout:  openTimeout,
		ReadTimeout:  readTimeout,
	}
}

// Budget is the worst case a single delivery can take: every hop may spend the full open and
// read timeouts.
func (d *Deliverer) Budget() time.Duration {
	return time.Duration(d.MaxRedirects+1) * (d.OpenTimeout + d.ReadTimeout)
}

var userinfoRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://([^/?#@]*)@`)

func (d *Deliverer) Deliver(ctx context.Context, req Request) Outcome {
	ctx, cancel := context.WithTimeout(ctx, d.Budget())
	defer cancel()

	original, err := url.Parse(req.URL)
	if err != nil {
		return Outcome{ErrorClass: "URI::InvalidURIError"}
	}
	// Ruby splits the raw userinfo without percent-decoding it, so do the same.
	var basicAuth string
	if m := userinfoRe.FindStringSubmatch(req.URL); m != nil {
		basicAuth = m[1]
	}

	current := req.URL
	var last *http.Response
	for hop := 0; hop <= d.MaxRedirects; hop++ {
		u, err := url.Parse(current)
		if err != nil {
			return Outcome{ErrorClass: "URI::InvalidURIError"}
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return Outcome{ErrorClass: "SsrfFilter::InvalidUriScheme"}
		}
		if u.Hostname() == "" {
			return Outcome{ErrorClass: "URI::InvalidURIError"}
		}

		ip, err := ssrf.PublicAddress(ctx, d.Resolve, u.Hostname())
		if err != nil {
			return Outcome{ErrorClass: err.Error(), Retryable: errors.Is(err, ssrf.ErrUnresolvedHostname)}
		}

		resp, err := d.once(ctx, u, ip, req, basicAuth, differentOrigin(original, u))
		if err != nil {
			class, retryable := classify(ctx, err)
			return Outcome{ErrorClass: class, Retryable: retryable}
		}
		last = resp

		loc := resp.Header.Get("Location")
		if resp.StatusCode < 300 || resp.StatusCode > 399 || loc == "" {
			return Outcome{Status: resp.StatusCode}
		}
		if strings.HasPrefix(loc, "/") {
			loc = u.Scheme + "://" + normalizedHostname(u) + loc
		}
		current = loc
	}
	// allow_unfollowed_redirects: hand the 3xx back so Rails can log and record it.
	return Outcome{Status: last.StatusCode}
}

func (d *Deliverer) once(ctx context.Context, u *url.URL, ip netip.Addr, req Request, basicAuth string, crossOrigin bool) (*http.Response, error) {
	target := *u
	target.User = nil
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), strings.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	httpReq.Host = normalizedHostname(u)
	httpReq.Header.Set("Content-Type", req.ContentType)
	if basicAuth != "" && !crossOrigin {
		user, pass, _ := strings.Cut(basicAuth, ":")
		httpReq.SetBasicAuth(user, pass)
	}

	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	addr := net.JoinHostPort(ip.String(), port)
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return d.Dial(ctx, network, addr)
		},
		TLSClientConfig:       &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   d.OpenTimeout,
		ResponseHeaderTimeout: d.ReadTimeout,
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     false,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return resp, nil
}

func differentOrigin(a, b *url.URL) bool {
	return a.Scheme != b.Scheme || a.Hostname() != b.Hostname() || portOf(a) != portOf(b)
}

func portOf(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

// normalizedHostname drops the default port, as the gem's Host header does.
func normalizedHostname(u *url.URL) string {
	host := u.Hostname()
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	port := u.Port()
	if port == "" || (port == "80" && u.Scheme == "http") || (port == "443" && u.Scheme == "https") {
		return host
	}
	return host + ":" + port
}

func classify(ctx context.Context, err error) (string, bool) {
	var opErr *net.OpError
	isDial := errors.As(err, &opErr) && opErr.Op == "dial"

	var netErr net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil:
		if isDial {
			return "Net::OpenTimeout", true
		}
		return "Net::ReadTimeout", true
	case errors.As(err, &netErr) && netErr.Timeout():
		if isDial {
			return "Net::OpenTimeout", true
		}
		return "Net::ReadTimeout", true
	case errors.Is(err, syscall.ECONNREFUSED):
		return "Errno::ECONNREFUSED", true
	case errors.Is(err, syscall.ECONNRESET):
		return "Errno::ECONNRESET", true
	case errors.Is(err, syscall.ENETUNREACH):
		return "Errno::ENETUNREACH", true
	case errors.Is(err, syscall.EHOSTUNREACH):
		return "Errno::EHOSTUNREACH", true
	case errors.Is(err, syscall.EADDRNOTAVAIL):
		return "Errno::EADDRNOTAVAIL", true
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "EOFError", true
	case isTLSError(err):
		return "OpenSSL::SSL::SSLError", true
	case strings.Contains(err.Error(), "malformed HTTP"):
		return "Net::HTTPBadResponse", false
	}
	return "HTTP::ConnectionError", true
}

func isTLSError(err error) bool {
	var certErr *tls.CertificateVerificationError
	var recErr tls.RecordHeaderError
	var alert tls.AlertError
	return errors.As(err, &certErr) || errors.As(err, &recErr) || errors.As(err, &alert)
}
