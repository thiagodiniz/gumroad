package validator

import (
	"context"
	"errors"
	"testing"
)

type stub struct {
	name   string
	calls  int
	lastID string
	valid  bool
	err    error
}

func (s *stub) Name() string { return s.name }
func (s *stub) Validate(_ context.Context, id string) (bool, error) {
	s.calls++
	s.lastID = id
	return s.valid, s.err
}

type countryStub struct {
	stub
	lastCountry string
}

func (s *countryStub) Validate(_ context.Context, id, country string) (bool, error) {
	s.lastCountry = country
	return s.stub.Validate(context.Background(), id)
}

func newRouter() (*Router, map[string]*stub, *countryStub) {
	names := []string{"abn", "gst", "qst", "mva", "trn", "kra_pin", "firs_tin", "tra_tin", "oman_vat", "eu_vat"}
	stubs := map[string]*stub{}
	for _, n := range names {
		stubs[n] = &stub{name: n, valid: true}
	}
	pro := &countryStub{stub: stub{name: "tax_id_pro", valid: true}}
	r := &Router{
		ABN: stubs["abn"], GST: stubs["gst"], QST: stubs["qst"], MVA: stubs["mva"], TRN: stubs["trn"],
		KraPin: stubs["kra_pin"], FirsTin: stubs["firs_tin"], TraTin: stubs["tra_tin"], OmanVat: stubs["oman_vat"],
		EUVat: stubs["eu_vat"], TaxIDPro: pro,
	}
	return r, stubs, pro
}

func TestRouterDispatch(t *testing.T) {
	cases := []struct {
		country, state, want string
	}{
		{"AU", "", "abn"},
		{"SG", "", "gst"},
		{"CA", "QC", "qst"},
		{"CA", "ON", "eu_vat"}, // non-Quebec Canada falls through, as in Ruby
		{"NO", "", "mva"},
		{"BH", "", "trn"},
		{"KE", "", "kra_pin"},
		{"NG", "", "firs_tin"},
		{"TZ", "", "tra_tin"},
		{"OM", "", "oman_vat"},
		{"JP", "", "tax_id_pro"},
		{"IN", "", "tax_id_pro"},
		{"KR", "", "tax_id_pro"},
		{"DE", "", "eu_vat"},
		{"GB", "", "eu_vat"},
		{"", "", "eu_vat"},
		{"US", "", "eu_vat"},
	}
	for _, c := range cases {
		r, stubs, pro := newRouter()
		res, err := r.Validate(context.Background(), Request{TaxID: "X", CountryCode: c.country, StateCode: c.state})
		if err != nil {
			t.Fatalf("%s/%s: %v", c.country, c.state, err)
		}
		if res.Validator != c.want {
			t.Errorf("%s/%s routed to %s, want %s", c.country, c.state, res.Validator, c.want)
		}
		if c.want == "tax_id_pro" {
			if pro.calls != 1 || pro.lastCountry != c.country {
				t.Errorf("%s: tax_id_pro calls=%d country=%q", c.country, pro.calls, pro.lastCountry)
			}
			continue
		}
		for name, s := range stubs {
			if (name == c.want) != (s.calls == 1) {
				t.Errorf("%s/%s: validator %s called %d times", c.country, c.state, name, s.calls)
			}
		}
	}
}

func TestRouterBlankID(t *testing.T) {
	r, stubs, _ := newRouter()
	res, err := r.Validate(context.Background(), Request{TaxID: "  ", CountryCode: "AU"})
	if err != nil || res.Valid || res.Validator != "none" {
		t.Fatalf("got %+v, %v", res, err)
	}
	if stubs["abn"].calls != 0 {
		t.Fatal("blank id must short-circuit before any validator")
	}
}

func TestRouterPropagatesUpstreamError(t *testing.T) {
	r, stubs, _ := newRouter()
	stubs["abn"].err = ErrUpstream
	res, err := r.Validate(context.Background(), Request{TaxID: "51824753556", CountryCode: "AU"})
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v", err)
	}
	if res.Validator != "abn" {
		t.Fatalf("validator = %q", res.Validator)
	}
}
