package validator

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serve(t *testing.T, h http.HandlerFunc) (*httptest.Server, *http.Client) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, &http.Client{Timeout: 2 * time.Second}
}

func TestVatstack(t *testing.T) {
	var gotType, gotQuery, gotKey string
	respond := map[string]any{"valid": true, "active": true}
	srv, client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotType, gotQuery, gotKey = r.FormValue("type"), r.FormValue("query"), r.Header.Get("X-API-KEY")
		_ = json.NewEncoder(w).Encode(respond)
	})

	abn := NewABN(client, srv.URL, "key")
	ok, err := abn.Validate(context.Background(), "51824753556")
	if err != nil || !ok {
		t.Fatalf("valid+active: got %v, %v", ok, err)
	}
	if gotType != "au_gst" || gotQuery != "51824753556" || gotKey != "key" {
		t.Fatalf("request: type=%q query=%q key=%q", gotType, gotQuery, gotKey)
	}

	respond = map[string]any{"valid": true, "active": false}
	if ok, _ := abn.Validate(context.Background(), "x"); ok {
		t.Fatal("inactive registration must be invalid")
	}
	respond = map[string]any{"valid": nil, "active": true}
	if ok, _ := abn.Validate(context.Background(), "x"); ok {
		t.Fatal("nil valid (registry down) must be invalid")
	}
	respond = map[string]any{"code": "INVALID_INPUT", "valid": true, "active": true}
	if ok, _ := abn.Validate(context.Background(), "x"); ok {
		t.Fatal("INVALID_INPUT must be invalid")
	}

	respond = map[string]any{"valid": true, "active": true}
	if _, err := NewMVA(client, srv.URL, "key").Validate(context.Background(), "NO123"); err != nil || gotType != "no_vat" {
		t.Fatalf("mva: type=%q err=%v", gotType, err)
	}
}

func TestVatstackTransportError(t *testing.T) {
	srv, client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not json")
	})
	_, err := NewABN(client, srv.URL, "key").Validate(context.Background(), "x")
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v", err)
	}
	srv.Close()
	_, err = NewABN(client, srv.URL, "key").Validate(context.Background(), "x")
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("connection refused should be ErrUpstream, got %v", err)
	}
}

func TestIRAS(t *testing.T) {
	var body map[string]string
	var headers http.Header
	respond := map[string]any{"returnCode": "10", "data": map[string]any{"Status": "Registered"}}
	srv, client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(respond)
	})
	gst := NewGST(client, srv.URL, "id", "secret")
	ok, err := gst.Validate(context.Background(), "M90000000A")
	if err != nil || !ok {
		t.Fatalf("got %v, %v", ok, err)
	}
	if body["clientID"] != "id" || body["regID"] != "M90000000A" {
		t.Fatalf("body = %v", body)
	}
	if headers.Get("X-IBM-Client-Id") != "id" || headers.Get("X-IBM-Client-Secret") != "secret" {
		t.Fatalf("headers = %v", headers)
	}
	respond = map[string]any{"returnCode": "10", "data": map[string]any{"Status": "De-registered"}}
	if ok, _ := gst.Validate(context.Background(), "x"); ok {
		t.Fatal("non-Registered status must be invalid")
	}
	respond = map[string]any{"returnCode": "20", "data": map[string]any{"Status": "Registered"}}
	if ok, _ := gst.Validate(context.Background(), "x"); ok {
		t.Fatal("non-10 return code must be invalid")
	}
}

func TestTaxIDPro(t *testing.T) {
	var gotURL, gotAuth string
	status := http.StatusOK
	respond := map[string]any{"is_valid": true}
	srv, client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotURL, gotAuth = r.URL.String(), r.Header.Get("Authorization")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(respond)
	})
	pro := NewTaxIDPro(client, srv.URL, "key")
	ok, err := pro.Validate(context.Background(), "1234567890123", "JP")
	if err != nil || !ok {
		t.Fatalf("got %v, %v", ok, err)
	}
	if gotURL != "/?country=JP&tin=1234567890123" || gotAuth != "Bearer key" {
		t.Fatalf("url=%q auth=%q", gotURL, gotAuth)
	}
	status = http.StatusUnprocessableEntity
	if ok, err := pro.Validate(context.Background(), "x", "JP"); ok || err != nil {
		t.Fatalf("non-200 must be invalid without error: %v %v", ok, err)
	}
	if ok, err := pro.Validate(context.Background(), "x", ""); ok || err != nil {
		t.Fatal("blank country is invalid")
	}
}

func TestRevenuQuebec(t *testing.T) {
	var gotPath string
	status := http.StatusOK
	respond := map[string]any{"Resultat": map[string]any{"StatutSousDossierUsager": "R"}}
	srv, client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(respond)
	})
	qst := NewQST(client, srv.URL)
	ok, err := qst.Validate(context.Background(), "1234567890TQ0001")
	if err != nil || !ok {
		t.Fatalf("got %v, %v", ok, err)
	}
	if gotPath != "/1234567890TQ0001" {
		t.Fatalf("path = %q", gotPath)
	}
	respond = map[string]any{"Resultat": map[string]any{"StatutSousDossierUsager": "N"}}
	if ok, _ := qst.Validate(context.Background(), "x"); ok {
		t.Fatal("status N must be invalid")
	}
	status = http.StatusNotFound
	if ok, err := qst.Validate(context.Background(), "x"); ok || err != nil {
		t.Fatalf("404 must be invalid without error: %v %v", ok, err)
	}
}

func TestEUVat(t *testing.T) {
	var got viesRequest
	status := http.StatusOK
	respond := map[string]any{"valid": true}
	calls := 0
	srv, client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(respond)
	})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	v := NewEUVat(client, srv.URL, "EU372030009", log)

	ok, err := v.Validate(context.Background(), "IE6388047V")
	if err != nil || !ok {
		t.Fatalf("VIES valid: got %v, %v", ok, err)
	}
	if got.CountryCode != "IE" || got.VatNumber != "6388047V" || got.RequesterMemberStateCode != "EU" || got.RequesterNumber != "372030009" {
		t.Fatalf("request = %+v", got)
	}

	respond = map[string]any{"valid": false, "userError": "INVALID"}
	if ok, _ := v.Validate(context.Background(), "IE6388047V"); ok {
		t.Fatal("VIES invalid must be invalid")
	}

	// Outage paths fall back to the syntax verdict (true for a well-formed number).
	respond = map[string]any{"valid": false, "userError": "MS_UNAVAILABLE"}
	if ok, err := v.Validate(context.Background(), "IE6388047V"); !ok || err != nil {
		t.Fatalf("MS_UNAVAILABLE fallback: %v %v", ok, err)
	}
	status = http.StatusInternalServerError
	respond = map[string]any{"errorWrappers": []map[string]string{{"error": "SERVICE_UNAVAILABLE"}}}
	if ok, err := v.Validate(context.Background(), "IE6388047V"); !ok || err != nil {
		t.Fatalf("500 fallback: %v %v", ok, err)
	}
	srv.Close()
	if ok, err := v.Validate(context.Background(), "IE6388047V"); !ok || err != nil {
		t.Fatalf("connection error fallback: %v %v", ok, err)
	}

	// Syntax failures and GB never hit the network.
	before := calls
	if ok, _ := v.Validate(context.Background(), "xxx"); ok {
		t.Fatal("malformed must be invalid")
	}
	if ok, _ := v.Validate(context.Background(), "IE638804"); ok {
		t.Fatal("wrong length must be invalid")
	}
	if ok, _ := v.Validate(context.Background(), "GB902194939"); !ok {
		t.Fatal("GB is syntax-only and must be valid")
	}
	if ok, _ := v.Validate(context.Background(), ""); ok {
		t.Fatal("blank must be invalid")
	}
	if calls != before {
		t.Fatalf("expected no VIES calls, got %d", calls-before)
	}
}

func TestNon2xxBodiesAreNotDecoded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("<html>not found</html>"))
	}))
	defer srv.Close()

	for name, run := range map[string]func() (bool, error){
		"tax_id_pro": func() (bool, error) {
			return NewTaxIDPro(srv.Client(), srv.URL, "key").Validate(context.Background(), "123", "JP")
		},
		"qst": func() (bool, error) { return NewQST(srv.Client(), srv.URL).Validate(context.Background(), "123") },
	} {
		valid, err := run()
		if err != nil || valid {
			t.Fatalf("%s: valid=%v err=%v, want invalid verdict without upstream error", name, valid, err)
		}
	}
}
