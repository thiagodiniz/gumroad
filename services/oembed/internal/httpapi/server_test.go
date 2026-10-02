package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/antiwork/gumroad/services/oembed/internal/oembed"
)

type fakeFinder struct {
	emb      *oembed.Embeddable
	err      error
	gotURL   string
	gotWidth int
}

func (f *fakeFinder) Lookup(_ context.Context, pageURL string, maxWidth int) (*oembed.Embeddable, error) {
	f.gotURL, f.gotWidth = pageURL, maxWidth
	return f.emb, f.err
}

func post(t *testing.T, url, token, body string) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Internal-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestEmbedEndpoint(t *testing.T) {
	f := &fakeFinder{emb: &oembed.Embeddable{HTML: "<iframe/>", Info: map[string]json.RawMessage{"width": json.RawMessage(`670`)}}}
	srv := httptest.NewServer(New(f, "", nil).Handler())
	defer srv.Close()

	resp, out := post(t, srv.URL+"/v1/embeds", "", `{"url":"https://vimeo.com/1","maxwidth":500}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%v", resp.StatusCode, out)
	}
	emb, _ := out["embeddable"].(map[string]any)
	if emb["html"] != "<iframe/>" || emb["info"].(map[string]any)["width"] != float64(670) {
		t.Errorf("body=%v", out)
	}
	if f.gotURL != "https://vimeo.com/1" || f.gotWidth != 500 {
		t.Errorf("finder got %q %d", f.gotURL, f.gotWidth)
	}

	f.emb = nil
	resp, out = post(t, srv.URL+"/v1/embeds", "", `{"url":"https://example.com/x"}`)
	if resp.StatusCode != http.StatusOK || out["embeddable"] != nil {
		t.Errorf("not embeddable: status=%d body=%v", resp.StatusCode, out)
	}
	if _, ok := out["embeddable"]; !ok {
		t.Errorf("embeddable key must be present (null): %v", out)
	}

	for _, body := range []string{`{bad json`, `{"url":""}`, `{"url":"ftp://x/y"}`, `{"url":"not a url"}`} {
		resp, out = post(t, srv.URL+"/v1/embeds", "", body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status=%d body=%v", body, resp.StatusCode, out)
		}
	}

	f.err = oembed.ErrUpstream
	resp, out = post(t, srv.URL+"/v1/embeds", "", `{"url":"https://vimeo.com/1"}`)
	if resp.StatusCode != http.StatusBadGateway || out["error"] != "upstream_unavailable" {
		t.Errorf("upstream: status=%d body=%v", resp.StatusCode, out)
	}
}

func TestInternalToken(t *testing.T) {
	srv := httptest.NewServer(New(&fakeFinder{}, "s3cret", nil).Handler())
	defer srv.Close()

	if resp, _ := post(t, srv.URL+"/v1/embeds", "", `{"url":"https://vimeo.com/1"}`); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("missing token: %d", resp.StatusCode)
	}
	if resp, _ := post(t, srv.URL+"/v1/embeds", "wrong", `{"url":"https://vimeo.com/1"}`); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong token: %d", resp.StatusCode)
	}
	if resp, _ := post(t, srv.URL+"/v1/embeds", "s3cret", `{"url":"https://vimeo.com/1"}`); resp.StatusCode != http.StatusOK {
		t.Errorf("right token: %d", resp.StatusCode)
	}
	if resp, err := http.Get(srv.URL + "/healthz"); err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("healthz needs no token: %v %v", resp, err)
	}
}
