package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/antiwork/gumroad/services/ping-delivery/internal/delivery"
)

type fakeDeliverer struct {
	outcome delivery.Outcome
	got     delivery.Request
}

func (f *fakeDeliverer) Deliver(_ context.Context, req delivery.Request) delivery.Outcome {
	f.got = req
	return f.outcome
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

func TestDeliveriesEndpoint(t *testing.T) {
	f := &fakeDeliverer{outcome: delivery.Outcome{Status: 500}}
	srv := httptest.NewServer(New(f, "", nil).Handler())
	defer srv.Close()

	resp, out := post(t, srv.URL+"/v1/deliveries", "", `{"url":"http://notification.com","body":"a=1","content_type":"application/x-www-form-urlencoded"}`)
	if resp.StatusCode != 200 || out["outcome"] != "responded" || out["status"] != float64(500) {
		t.Errorf("responded: %d %v", resp.StatusCode, out)
	}
	if f.got != (delivery.Request{URL: "http://notification.com", Body: "a=1", ContentType: "application/x-www-form-urlencoded"}) {
		t.Errorf("deliverer got %+v", f.got)
	}

	f.outcome = delivery.Outcome{ErrorClass: "Net::ReadTimeout", Retryable: true}
	resp, out = post(t, srv.URL+"/v1/deliveries", "", `{"url":"http://notification.com","body":"","content_type":"application/json"}`)
	if resp.StatusCode != 200 || out["outcome"] != "error" || out["error_class"] != "Net::ReadTimeout" || out["retryable"] != true {
		t.Errorf("error: %d %v", resp.StatusCode, out)
	}

	for _, body := range []string{`{bad`, `{"url":"","content_type":"application/json"}`, `{"url":"http://x"}`} {
		if resp, out := post(t, srv.URL+"/v1/deliveries", "", body); resp.StatusCode != 400 {
			t.Errorf("%s: %d %v", body, resp.StatusCode, out)
		}
	}
}

func TestInternalToken(t *testing.T) {
	srv := httptest.NewServer(New(&fakeDeliverer{outcome: delivery.Outcome{Status: 200}}, "s3cret", nil).Handler())
	defer srv.Close()
	body := `{"url":"http://notification.com","body":"a=1","content_type":"application/json"}`

	if resp, _ := post(t, srv.URL+"/v1/deliveries", "", body); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("missing token: %d", resp.StatusCode)
	}
	if resp, _ := post(t, srv.URL+"/v1/deliveries", "wrong", body); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong token: %d", resp.StatusCode)
	}
	if resp, _ := post(t, srv.URL+"/v1/deliveries", "s3cret", body); resp.StatusCode != http.StatusOK {
		t.Errorf("right token: %d", resp.StatusCode)
	}
	if resp, err := http.Get(srv.URL + "/healthz"); err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("healthz needs no token: %v %v", resp, err)
	}
}
