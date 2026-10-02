package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/antiwork/gumroad/services/tax-id-validation/internal/validator"
)

type fakeRouter struct {
	res validator.Result
	err error
	got validator.Request
}

func (f *fakeRouter) Validate(_ context.Context, req validator.Request) (validator.Result, error) {
	f.got = req
	return f.res, f.err
}

func newTestServer(t *testing.T, r *fakeRouter, token string) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(New(r, token, log).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, url, token, body string) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
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

func TestValidateEndpoint(t *testing.T) {
	r := &fakeRouter{res: validator.Result{Valid: true, Validator: "abn"}}
	srv := newTestServer(t, r, "")

	resp, out := post(t, srv.URL+"/v1/validations", "", `{"tax_id":"51824753556","country_code":"AU","state_code":null}`)
	if resp.StatusCode != http.StatusOK || out["valid"] != true || out["validator"] != "abn" {
		t.Fatalf("status=%d body=%v", resp.StatusCode, out)
	}
	if r.got != (validator.Request{TaxID: "51824753556", CountryCode: "AU"}) {
		t.Fatalf("router got %+v", r.got)
	}

	resp, out = post(t, srv.URL+"/v1/validations", "", `{bad json`)
	if resp.StatusCode != http.StatusBadRequest || out["error"] != "invalid_json" {
		t.Fatalf("status=%d body=%v", resp.StatusCode, out)
	}

	resp, out = post(t, srv.URL+"/v1/validations", "", `{"tax_id":"  ","country_code":"AU"}`)
	if resp.StatusCode != http.StatusBadRequest || out["error"] != "invalid_request" {
		t.Fatalf("blank tax_id: status=%d body=%v", resp.StatusCode, out)
	}

	r.err = validator.ErrUpstream
	r.res = validator.Result{Validator: "abn"}
	resp, out = post(t, srv.URL+"/v1/validations", "", `{"tax_id":"x","country_code":"AU"}`)
	if resp.StatusCode != http.StatusBadGateway || out["error"] != "upstream_unavailable" || out["validator"] != "abn" {
		t.Fatalf("status=%d body=%v", resp.StatusCode, out)
	}
}

func TestInternalToken(t *testing.T) {
	r := &fakeRouter{res: validator.Result{Valid: true, Validator: "trn"}}
	srv := newTestServer(t, r, "s3cret")

	resp, out := post(t, srv.URL+"/v1/validations", "", `{"tax_id":"x","country_code":"BH"}`)
	if resp.StatusCode != http.StatusUnauthorized || out["error"] != "unauthorized" {
		t.Fatalf("missing token: status=%d body=%v", resp.StatusCode, out)
	}
	resp, _ = post(t, srv.URL+"/v1/validations", "wrong", `{"tax_id":"x","country_code":"BH"}`)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token: status=%d", resp.StatusCode)
	}
	resp, _ = post(t, srv.URL+"/v1/validations", "s3cret", `{"tax_id":"x","country_code":"BH"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("right token: status=%d", resp.StatusCode)
	}

	// Probes stay unauthenticated so kubelet can reach them.
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %v %v", resp, err)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv := newTestServer(t, &fakeRouter{}, "")
	resp, err := http.Get(srv.URL + "/v1/validations")
	if err != nil || resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET on validations: %v %v", resp.StatusCode, err)
	}
}
