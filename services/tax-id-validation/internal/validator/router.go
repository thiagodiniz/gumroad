package validator

import "context"

// Request is one tax ID to validate, plus the buyer location that picks the registry.
type Request struct {
	TaxID       string
	CountryCode string
	StateCode   string
}

// Result is the verdict together with which validator produced it, so the Rails side
// can log and compare against its in-process result during the migration.
type Result struct {
	Valid     bool
	Validator string
}

// CountryValidator is the taxid.pro shape: the registry needs the country as well.
type CountryValidator interface {
	Name() string
	Validate(ctx context.Context, id, countryCode string) (bool, error)
}

// Router is the Go counterpart of RegionalVatIdValidationService: it dispatches on
// (country, state) to the registry-specific validator. The dispatch order is the same as
// the Ruby `if/elsif` chain, which matters for Canada (Quebec only) and for the taxid.pro
// branch sitting before the EU VAT fallthrough.
type Router struct {
	ABN      Validator
	GST      Validator
	QST      Validator
	MVA      Validator
	TRN      Validator
	KraPin   Validator
	FirsTin  Validator
	TraTin   Validator
	OmanVat  Validator
	TaxIDPro CountryValidator
	EUVat    Validator
}

func (r *Router) Validate(ctx context.Context, req Request) (Result, error) {
	if Blank(req.TaxID) {
		return Result{Valid: false, Validator: "none"}, nil
	}

	var v Validator
	switch {
	case req.CountryCode == countryAustralia:
		v = r.ABN
	case req.CountryCode == countrySingapore:
		v = r.GST
	case req.CountryCode == countryCanada && req.StateCode == stateQuebec:
		v = r.QST
	case req.CountryCode == countryNorway:
		v = r.MVA
	case req.CountryCode == countryBahrain:
		v = r.TRN
	case req.CountryCode == countryKenya:
		v = r.KraPin
	case req.CountryCode == countryNigeria:
		v = r.FirsTin
	case req.CountryCode == countryTanzania:
		v = r.TraTin
	case req.CountryCode == countryOman:
		v = r.OmanVat
	case UsesTaxIDPro(req.CountryCode):
		valid, err := r.TaxIDPro.Validate(ctx, req.TaxID, req.CountryCode)
		return Result{Valid: valid, Validator: r.TaxIDPro.Name()}, err
	default:
		v = r.EUVat
	}

	valid, err := v.Validate(ctx, req.TaxID)
	return Result{Valid: valid, Validator: v.Name()}, err
}
