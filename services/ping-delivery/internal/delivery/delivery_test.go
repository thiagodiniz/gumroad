package delivery

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"
	"time"
)

type seen struct {
	mu   sync.Mutex
	reqs []recorded
}

type recorded struct {
	path, host, body, contentType, auth string
}

func (s *seen) add(r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqs = append(s.reqs, recorded{r.URL.Path, r.Host, string(b), r.Header.Get("Content-Type"), r.Header.Get("Authorization")})
}

// newDeliverer resolves every hostname to a public address and dials the test server
// instead, so redirects between "hosts" can be exercised against one httptest server.
func newDeliverer(t *testing.T, srv *httptest.Server, private map[string]bool) *Deliverer {
	t.Helper()
	d := New(3, 2*time.Second, 500*time.Millisecond)
	d.Resolve = func(_ context.Context, host string) ([]netip.Addr, error) {
		switch {
		case private[host]:
			return []netip.Addr{netip.MustParseAddr("10.0.0.5")}, nil
		case host == "nxdomain.example":
			return nil, nil
		}
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	}
	d.Dial = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	return d
}

func TestDeliver(t *testing.T) {
	var s seen
	mux := http.NewServeMux()
	mux.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) { s.add(r); w.WriteHeader(200) })
	mux.HandleFunc("/hook/", func(w http.ResponseWriter, r *http.Request) {
		s.add(r)
		w.Header().Set("Location", "/hook")
		w.WriteHeader(308)
	})
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) {
		s.add(r)
		w.Header().Set("Location", "/loop")
		w.WriteHeader(302)
	})
	mux.HandleFunc("/private", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://internal.example.net/")
		w.WriteHeader(302)
	})
	mux.HandleFunc("/other-origin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://other.example.com/hook")
		w.WriteHeader(302)
	})
	mux.HandleFunc("/relative", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "hook")
		w.WriteHeader(302)
	})
	mux.HandleFunc("/no-location", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(304) })
	mux.HandleFunc("/500", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) { time.Sleep(2 * time.Second) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	d := newDeliverer(t, srv, map[string]bool{"internal.example.net": true})
	ctx := context.Background()
	req := func(u string) Request {
		return Request{URL: u, Body: "a=1", ContentType: "application/x-www-form-urlencoded"}
	}

	if o := d.Deliver(ctx, req("http://example.com/hook")); o != (Outcome{Status: 200}) {
		t.Fatalf("plain: %+v", o)
	}
	if got := s.reqs[len(s.reqs)-1]; got.body != "a=1" || got.contentType != "application/x-www-form-urlencoded" || got.host != "example.com" {
		t.Errorf("plain request: %+v", got)
	}

	if o := d.Deliver(ctx, req("http://example.com:8080/hook")); o.Status != 200 || s.reqs[len(s.reqs)-1].host != "example.com:8080" {
		t.Errorf("non-default port host header: %+v %+v", o, s.reqs[len(s.reqs)-1])
	}

	n := len(s.reqs)
	if o := d.Deliver(ctx, req("http://example.com/hook/")); o != (Outcome{Status: 200}) {
		t.Errorf("trailing slash redirect: %+v", o)
	}
	if got := s.reqs[n:]; len(got) != 2 || got[1].path != "/hook" || got[1].body != "a=1" {
		t.Errorf("redirect should re-send the payload: %+v", got)
	}

	n = len(s.reqs)
	if o := d.Deliver(ctx, req("http://example.com/loop")); o != (Outcome{Status: 302}) {
		t.Errorf("redirect limit should hand back the 3xx: %+v", o)
	}
	if hops := len(s.reqs) - n; hops != 4 {
		t.Errorf("expected max_redirects+1 = 4 hops, got %d", hops)
	}

	if o := d.Deliver(ctx, req("http://example.com/private")); o != (Outcome{ErrorClass: "SsrfFilter::PrivateIPAddress"}) {
		t.Errorf("private redirect: %+v", o)
	}
	if o := d.Deliver(ctx, req("http://example.com/relative")); o != (Outcome{ErrorClass: "SsrfFilter::InvalidUriScheme"}) {
		t.Errorf("relative redirect without leading slash: %+v", o)
	}
	if o := d.Deliver(ctx, req("http://example.com/no-location")); o != (Outcome{Status: 304}) {
		t.Errorf("3xx without location: %+v", o)
	}
	if o := d.Deliver(ctx, req("http://example.com/500")); o != (Outcome{Status: 500}) {
		t.Errorf("500: %+v", o)
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:secret"))
	if o := d.Deliver(ctx, req("http://user:secret@example.com/hook")); o.Status != 200 || s.reqs[len(s.reqs)-1].auth != auth {
		t.Errorf("basic auth from userinfo: %+v %+v", o, s.reqs[len(s.reqs)-1])
	}
	if o := d.Deliver(ctx, req("http://user:secret@example.com/other-origin")); o.Status != 200 || s.reqs[len(s.reqs)-1].auth != "" || s.reqs[len(s.reqs)-1].host != "other.example.com" {
		t.Errorf("cross-origin redirect must drop credentials: %+v %+v", o, s.reqs[len(s.reqs)-1])
	}

	if o := d.Deliver(ctx, req("http://nxdomain.example/hook")); o != (Outcome{ErrorClass: "SsrfFilter::UnresolvedHostname", Retryable: true}) {
		t.Errorf("unresolved: %+v", o)
	}
	if o := d.Deliver(ctx, req("http://internal.example.net/hook")); o != (Outcome{ErrorClass: "SsrfFilter::PrivateIPAddress"}) {
		t.Errorf("private at connect: %+v", o)
	}
	if o := d.Deliver(ctx, req("ftp://example.com/hook")); o != (Outcome{ErrorClass: "SsrfFilter::InvalidUriScheme"}) {
		t.Errorf("scheme: %+v", o)
	}
	if o := d.Deliver(ctx, req("http://exa mple.com/hook")); o != (Outcome{ErrorClass: "URI::InvalidURIError"}) {
		t.Errorf("invalid url: %+v", o)
	}
	if o := d.Deliver(ctx, req("http://example.com/slow")); o != (Outcome{ErrorClass: "Net::ReadTimeout", Retryable: true}) {
		t.Errorf("read timeout: %+v", o)
	}

	srv.Close()
	if o := d.Deliver(ctx, req("http://example.com/hook")); o != (Outcome{ErrorClass: "Errno::ECONNREFUSED", Retryable: true}) {
		t.Errorf("refused: %+v", o)
	}
}

func TestBudget(t *testing.T) {
	if got := New(3, 5*time.Second, 5*time.Second).Budget(); got != 40*time.Second {
		t.Errorf("budget = %s", got)
	}
}
