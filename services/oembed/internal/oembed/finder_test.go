package oembed

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryMatchesGemPrecedence(t *testing.T) {
	r := DefaultRegistry()
	cases := map[string]string{
		"https://www.youtube.com/watch?v=jNQXAC9IVRw":        "https://www.youtube.com/oembed/?scheme=https",
		"https://vimeo.com/71588076":                         "https://vimeo.com/api/oembed.{format}",
		"https://soundcloud.com/seller/track":                "https://soundcloud.com/oembed",
		"https://fast.wistia.com/embed/medias/abc":           "http://fast.wistia.com/oembed",
		"https://sketchfab.com/models/abc":                   "https://sketchfab.com/oembed",
		"https://framerate.tv/watch/AB3peBMp":                "https://framerate.tv/api/oembed",
		"https://example.com/no-registered-provider-matches": "",
		"https://www.instagram.com/p/abc":                    "", // needs an access token Rails never sets
	}
	for u, want := range cases {
		if got := r.Find(u); got != want {
			t.Errorf("Find(%q) = %q, want %q", u, got, want)
		}
	}
}

// Cases lifted from spec/lib/utilities/o_embed_finder_spec.rb.
func TestFromFieldsPostProcessing(t *testing.T) {
	cases := []struct {
		name, typ, html, want string
	}{
		{"plain", "video", "<oembed/>", "<oembed/>"},
		{"soundcloud https", "video",
			"<oembed><author_url>http://w.soundcloud.com</author_url><provider_url>api.soundcloud.com</provider_url></oembed>",
			"<oembed><author_url>https://w.soundcloud.com</author_url><provider_url>api.soundcloud.com</provider_url></oembed>"},
		{"soundcloud payload", "video",
			"<oembed><author_url>http://w.soundcloud.com?show_artwork=true</author_url><provider_url>api.soundcloud.com</provider_url></oembed>",
			"<oembed><author_url>https://w.soundcloud.com?" + soundcloudPayload + "</author_url><provider_url>api.soundcloud.com</provider_url></oembed>"},
		{"youtube https", "video",
			"<oembed><author_url>http://www.youtube.com/embed</author_url></oembed>",
			"<oembed><author_url>https://www.youtube.com/embed</author_url></oembed>"},
		{"youtube params", "video",
			"<oembed><author_url>https://www.youtube.com/embed?feature=oembed</author_url></oembed>",
			"<oembed><author_url>https://www.youtube.com/embed?feature=oembed&showinfo=0&controls=0&rel=0</author_url></oembed>"},
		{"vimeo https", "video",
			"<oembed><author_url>http://player.vimeo.com/video/71588076</author_url></oembed>",
			"<oembed><author_url>https://player.vimeo.com/video/71588076</author_url></oembed>"},
		{"rich is embeddable", "rich", "<iframe src=\"http://fast.wistia.net/x\"></iframe>", "<iframe src=\"https://fast.wistia.net/x\"></iframe>"},
	}
	for _, c := range cases {
		fields := map[string]json.RawMessage{
			"type":             json.RawMessage(`"` + c.typ + `"`),
			"html":             mustJSON(t, c.html),
			"width":            json.RawMessage(`600`),
			"height":           json.RawMessage(`400`),
			"thumbnail_url":    json.RawMessage(`"http://example.com/url-to-thumbnail.jpg"`),
			"thumbnail_width":  json.RawMessage(`200`),
			"thumbnail_height": json.RawMessage(`133`),
		}
		got := FromFields(fields)
		if got == nil {
			t.Fatalf("%s: got nil", c.name)
		}
		if got.HTML != c.want {
			t.Errorf("%s: html = %q, want %q", c.name, got.HTML, c.want)
		}
		if len(got.Info) != 3 || string(got.Info["width"]) != "600" || string(got.Info["height"]) != "400" ||
			string(got.Info["thumbnail_url"]) != `"http://example.com/url-to-thumbnail.jpg"` {
			t.Errorf("%s: info = %v", c.name, got.Info)
		}
	}
}

func TestFromFieldsRejectsPhotoAndLink(t *testing.T) {
	for _, typ := range []string{"photo", "link", "", "unknown"} {
		fields := map[string]json.RawMessage{"type": mustJSON(t, typ), "html": json.RawMessage(`"Some image"`)}
		if got := FromFields(fields); got != nil {
			t.Errorf("type %q: got %+v, want nil", typ, got)
		}
	}
}

func TestLookup(t *testing.T) {
	var gotURL string
	status := http.StatusOK
	body := `{"version":"1.0","type":"video","html":"<iframe src=\"https://framerate.tv/embed/AB3peBMp\"></iframe>","width":670,"height":377,"thumbnail_url":"https://cdn.framerate.tv/t.jpg"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		if r.Header.Get("User-Agent") != userAgent {
			t.Errorf("user agent = %q", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	reg, err := NewRegistry([]Provider{
		{Endpoint: srv.URL + "/api/oembed", Patterns: []string{`^https://([^\.]+\.)?framerate\.tv/watch/(.*?)`}},
		{Endpoint: srv.URL + "/oembed.{format}?x=1", Patterns: []string{`^https://vimeo\.com/(.*?)`}},
	})
	if err != nil {
		t.Fatal(err)
	}
	f := NewFinder(reg, srv.Client(), DefaultMaxRedirects)

	emb, err := f.Lookup(context.Background(), "https://framerate.tv/watch/AB3peBMp", 0)
	if err != nil || emb == nil {
		t.Fatalf("emb=%v err=%v", emb, err)
	}
	if !strings.Contains(emb.HTML, "framerate.tv/embed/") || string(emb.Info["width"]) != "670" {
		t.Errorf("unexpected embeddable %+v", emb)
	}
	if want := "/api/oembed?format=json&maxwidth=670&url=https%3A%2F%2Fframerate.tv%2Fwatch%2FAB3peBMp"; gotURL != want {
		t.Errorf("request url = %s, want %s", gotURL, want)
	}

	if _, err := f.Lookup(context.Background(), "https://vimeo.com/1", 300); err != nil {
		t.Fatal(err)
	}
	if want := "/oembed.json?x=1&maxwidth=300&url=https%3A%2F%2Fvimeo.com%2F1"; gotURL != want {
		t.Errorf("{format} url = %s, want %s", gotURL, want)
	}

	if emb, err := f.Lookup(context.Background(), "https://example.com/nothing", 0); emb != nil || err != nil {
		t.Errorf("unregistered: emb=%v err=%v", emb, err)
	}

	status, body = http.StatusNotFound, "not found"
	if emb, err := f.Lookup(context.Background(), "https://vimeo.com/1", 0); emb != nil || err != nil {
		t.Errorf("404: emb=%v err=%v, want nil verdict", emb, err)
	}

	status, body = http.StatusOK, "<html>not json</html>"
	if emb, err := f.Lookup(context.Background(), "https://vimeo.com/1", 0); emb != nil || err != nil {
		t.Errorf("bad json: emb=%v err=%v, want nil verdict", emb, err)
	}

	status, body = http.StatusBadGateway, ""
	if _, err := f.Lookup(context.Background(), "https://vimeo.com/1", 0); !errors.Is(err, ErrUpstream) {
		t.Errorf("5xx: err=%v, want ErrUpstream", err)
	}

	srv.Close()
	if _, err := f.Lookup(context.Background(), "https://vimeo.com/1", 0); !errors.Is(err, ErrUpstream) {
		t.Errorf("connection refused: err=%v, want ErrUpstream", err)
	}
}

func TestLookupStopsFollowingRedirects(t *testing.T) {
	var mux http.ServeMux
	srv := httptest.NewServer(&mux)
	defer srv.Close()
	mux.HandleFunc("/hop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/hop", http.StatusFound)
	})
	reg, _ := NewRegistry([]Provider{{Endpoint: srv.URL + "/hop", Patterns: []string{`^https://vimeo\.com/(.*?)`}}})
	f := NewFinder(reg, srv.Client(), 2)

	emb, err := f.Lookup(context.Background(), "https://vimeo.com/1", 0)
	if emb != nil || err != nil {
		t.Fatalf("redirect loop: emb=%v err=%v, want nil verdict", emb, err)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
