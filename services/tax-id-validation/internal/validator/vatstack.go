package validator

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

const DefaultVatstackURL = "https://api.vatstack.com/v1/validations"

// Vatstack validates Australian ABNs (type au_gst) and Norwegian MVA numbers (type
// no_vat) through api.vatstack.com. Ruby: AbnValidationService, MvaValidationService.
type Vatstack struct {
	name   string
	typ    string
	url    string
	apiKey string
	client *http.Client
}

func NewABN(client *http.Client, apiURL, apiKey string) *Vatstack {
	return &Vatstack{name: "abn", typ: "au_gst", url: apiURL, apiKey: apiKey, client: client}
}

func NewMVA(client *http.Client, apiURL, apiKey string) *Vatstack {
	return &Vatstack{name: "mva", typ: "no_vat", url: apiURL, apiKey: apiKey, client: client}
}

func (v *Vatstack) Name() string { return v.name }

type vatstackResponse struct {
	Code   string `json:"code"`
	Valid  *bool  `json:"valid"`
	Active bool   `json:"active"`
}

func (v *Vatstack) Validate(ctx context.Context, id string) (bool, error) {
	if Blank(id) {
		return false, nil
	}
	form := url.Values{"type": {v.typ}, "query": {id}}
	req, err := newRequest(ctx, http.MethodPost, v.url, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-API-KEY", v.apiKey)

	var out vatstackResponse
	if _, err := doJSON(v.client, req, &out); err != nil {
		return false, err
	}
	if out.Code == "INVALID_INPUT" || out.Valid == nil {
		return false, nil
	}
	return *out.Valid && out.Active, nil
}
