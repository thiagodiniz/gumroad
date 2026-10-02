// Package oembed ports lib/utilities/o_embed_finder.rb and the provider registry that
// config/initializers/oembed.rb builds on top of the ruby-oembed gem.
package oembed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ErrUpstream marks transport failures and 5xx answers from a provider. The HTTP layer maps
// it to 502 so Rails falls back to the in-process gem; every other failure is a plain "not
// embeddable" verdict, exactly like the Ruby `rescue StandardError → nil`.
var ErrUpstream = errors.New("oembed upstream unavailable")

const (
	DefaultMaxWidth     = 670 // AssetPreview::DEFAULT_DISPLAY_WIDTH
	DefaultMaxRedirects = 4   // OEmbed::HttpHelper default
	maxBody             = 1 << 20
	userAgent           = "Mozilla/5.0 (compatible; ruby-oembed/0.17.0)"
)

// Provider is one oEmbed endpoint and the URL patterns it serves. Patterns are Ruby regexp
// sources as produced by OEmbed::Provider#<<; they are RE2-compatible.
type Provider struct {
	Endpoint string
	Patterns []string
}

type compiledProvider struct {
	endpoint string
	patterns []*regexp.Regexp
}

// Registry resolves a URL to its provider using the gem's precedence: patterns in
// registration order, first match wins.
type Registry struct {
	providers []compiledProvider
}

func NewRegistry(providers []Provider) (*Registry, error) {
	r := &Registry{}
	for _, p := range providers {
		cp := compiledProvider{endpoint: p.Endpoint}
		for _, src := range p.Patterns {
			re, err := regexp.Compile(src)
			if err != nil {
				return nil, fmt.Errorf("provider %s: pattern %q: %w", p.Endpoint, src, err)
			}
			cp.patterns = append(cp.patterns, re)
		}
		r.providers = append(r.providers, cp)
	}
	return r, nil
}

// DefaultRegistry mirrors the registry Rails has after boot.
func DefaultRegistry() *Registry {
	r, err := NewRegistry(builtinProviders)
	if err != nil {
		panic(err)
	}
	return r
}

// Find returns the endpoint for u, or "" when no registered provider claims it.
func (r *Registry) Find(u string) string {
	for _, p := range r.providers {
		for _, re := range p.patterns {
			if re.MatchString(u) {
				return p.endpoint
			}
		}
	}
	return ""
}

// Embeddable is what OEmbedFinder.embeddable_from_url returns: the (post-processed) embed
// HTML plus the width/height/thumbnail_url fields exactly as the provider sent them.
type Embeddable struct {
	HTML string                     `json:"html"`
	Info map[string]json.RawMessage `json:"info"`
}

type Finder struct {
	registry *Registry
	client   *http.Client
}

// NewFinder wraps client with the gem's redirect limit. The client's Timeout stands in for
// Net::HTTP's open/read timeouts.
func NewFinder(registry *Registry, client *http.Client, maxRedirects int) *Finder {
	c := *client
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return &Finder{registry: registry, client: &c}
}

// Lookup returns nil, nil when the URL is not embeddable (no provider, provider rejected
// it, or the response is a photo/link). It returns ErrUpstream only when the provider could
// not be reached or answered 5xx.
func (f *Finder) Lookup(ctx context.Context, pageURL string, maxWidth int) (*Embeddable, error) {
	endpoint := f.registry.Find(pageURL)
	if endpoint == "" {
		return nil, nil
	}
	if maxWidth <= 0 {
		maxWidth = DefaultMaxWidth
	}

	reqURL := buildRequestURL(endpoint, pageURL, maxWidth)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, nil
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: %s answered %d", ErrUpstream, endpoint, resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("%w: reading body: %v", ErrUpstream, err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, nil
	}
	return FromFields(fields), nil
}

// buildRequestURL mirrors OEmbed::Provider#build: "{format}" in the endpoint is replaced,
// otherwise format=json is sent as a query parameter.
func buildRequestURL(endpoint, pageURL string, maxWidth int) string {
	q := url.Values{"url": {pageURL}, "maxwidth": {strconv.Itoa(maxWidth)}}
	if strings.Contains(endpoint, "{format}") {
		endpoint = strings.ReplaceAll(endpoint, "{format}", "json")
	} else {
		q.Set("format", "json")
	}
	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	return endpoint + sep + q.Encode()
}

var soundcloudParams = []string{"auto_play", "show_artwork", "show_comments", "buying", "sharing", "download", "show_playcount", "show_user", "liking"}

var soundcloudPayload = func() string {
	parts := make([]string, len(soundcloudParams))
	for i, k := range soundcloudParams {
		parts[i] = k + "=false"
	}
	return strings.Join(parts, "&")
}()

// FromFields applies OEmbedFinder's post-processing to a decoded oEmbed response. Only
// video and rich responses are embeddable; photo/link return nil so Rails downloads the
// URL as an image instead.
func FromFields(fields map[string]json.RawMessage) *Embeddable {
	var typ, html string
	if raw, ok := fields["type"]; ok {
		_ = json.Unmarshal(raw, &typ)
	}
	if typ != "video" && typ != "rich" {
		return nil
	}
	if raw, ok := fields["html"]; ok {
		_ = json.Unmarshal(raw, &html)
	}

	switch {
	case strings.Contains(html, "api.soundcloud.com"):
		html = strings.ReplaceAll(html, "http://w.soundcloud.com", "https://w.soundcloud.com")
		html = strings.ReplaceAll(html, "show_artwork=true", soundcloudPayload)
	case strings.Contains(html, "youtube.com/embed"):
		html = strings.ReplaceAll(html, "http://", "https://")
		html = strings.ReplaceAll(html, "feature=oembed", "feature=oembed&showinfo=0&controls=0&rel=0")
	case strings.Contains(html, "wistia"), strings.Contains(html, "sketchfab"), strings.Contains(html, "vimeo"):
		html = strings.ReplaceAll(html, "http://", "https://")
	}

	info := map[string]json.RawMessage{}
	for _, k := range []string{"width", "height", "thumbnail_url"} {
		if v, ok := fields[k]; ok {
			info[k] = v
		}
	}
	return &Embeddable{HTML: html, Info: info}
}
