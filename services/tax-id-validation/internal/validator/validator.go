// Package validator ports the Rails tax ID validators (app/services/*_validation_service.rb)
// and the RegionalVatIdValidationService dispatcher that fronts them.
package validator

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

// ErrUpstream marks failures of a third-party registry (timeout, 5xx, malformed body).
// Callers distinguish it from a definitive "invalid" so they can degrade instead of
// denying a tax exemption.
var ErrUpstream = errors.New("upstream validation service unavailable")

// Validator answers whether a single tax identifier is valid for one jurisdiction.
type Validator interface {
	Name() string
	Validate(ctx context.Context, id string) (bool, error)
}

// Blank mirrors ActiveSupport's `blank?` for the string inputs we accept.
func Blank(s string) bool { return strings.TrimSpace(s) == "" }

// Pattern is a pure-format validator: no network, no cache.
type Pattern struct {
	name string
	re   *regexp.Regexp
}

func NewPattern(name string, re *regexp.Regexp) *Pattern { return &Pattern{name: name, re: re} }

func (p *Pattern) Name() string { return p.name }

func (p *Pattern) Validate(_ context.Context, id string) (bool, error) {
	if Blank(id) {
		return false, nil
	}
	return p.re.MatchString(id), nil
}

// Format-only validators. Regexes are copied verbatim from the Rails services so the
// two implementations can be diffed line by line during the migration.
var (
	// Nigeria FIRS TIN. Ruby used ^...$ (line anchors); input is a single line so \A..\z is equivalent.
	FirsTin = NewPattern("firs_tin", regexp.MustCompile(`\A\d{8}-\d{4}\z`))
	// Kenya KRA PIN.
	KraPin = NewPattern("kra_pin", regexp.MustCompile(`\A[A-Z]\d{9}[A-Z]\z`))
	// Oman VAT number.
	OmanVat = NewPattern("oman_vat", regexp.MustCompile(`\AOM\d{10}\z`))
	// Tanzania TRA TIN.
	TraTin = NewPattern("tra_tin", regexp.MustCompile(`\A\d{2}-\d{6}-[A-Z]\z`))
)

// Trn validates a Bahrain TRN. The Rails service only checks the length, which counts
// characters (not bytes); keep that so multibyte input is judged the same way.
type Trn struct{}

func (Trn) Name() string { return "trn" }

func (Trn) Validate(_ context.Context, id string) (bool, error) {
	if Blank(id) {
		return false, nil
	}
	return len([]rune(id)) == 15, nil
}
