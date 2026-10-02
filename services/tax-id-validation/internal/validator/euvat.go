package validator

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"unicode"
)

const DefaultViesURL = "https://ec.europa.eu/taxation_customs/vies/rest-api/check-vat-number"

// vatPatterns are valvat's Syntax::VAT_PATTERNS, keyed by ISO country code. Greece uses
// the EL prefix and Northern Ireland XI; see vatCountryToISO.
var vatPatterns = map[string]*regexp.Regexp{
	"AT": regexp.MustCompile(`\AATU[0-9]{8}\z`),
	"BE": regexp.MustCompile(`\ABE[0-1][0-9]{9}\z`),
	"BG": regexp.MustCompile(`\ABG[0-9]{9,10}\z`),
	"CY": regexp.MustCompile(`\ACY(?:1[013-9]|[02-69][0-9])[0-9]{6}[A-Z]\z`), // valvat: CY(?!12)[0-69][0-9]{7}[A-Z], without lookahead
	"CZ": regexp.MustCompile(`\ACZ[0-9]{8,10}\z`),
	"DE": regexp.MustCompile(`\ADE[0-9]{9}\z`),
	"DK": regexp.MustCompile(`\ADK[0-9]{8}\z`),
	"EE": regexp.MustCompile(`\AEE10[0-9]{7}\z`),
	"GR": regexp.MustCompile(`\AEL[0-9]{9}\z`),
	"ES": regexp.MustCompile(`\AES(?:[A-Z][0-9]{8}|[0-9]{8}[A-Z]|[A-Z][0-9]{7}[A-Z])\z`),
	"FI": regexp.MustCompile(`\AFI[0-9]{8}\z`),
	"FR": regexp.MustCompile(`\AFR[A-HJ-NP-Z0-9]{2}[0-9]{9}\z`),
	"GB": regexp.MustCompile(`\A(?:GB|XI)(?:[0-9]{9}|[0-9]{12}|(?:HA|GD)[0-9]{3})\z`),
	"HR": regexp.MustCompile(`\AHR[0-9]{11}\z`),
	"HU": regexp.MustCompile(`\AHU[0-9]{8}\z`),
	"IE": regexp.MustCompile(`\AIE(?:[0-9][A-Z][0-9]{5}|[0-9]{7}[A-Z]?)[A-Z]\z`),
	"IT": regexp.MustCompile(`\AIT[0-9]{11}\z`),
	"LT": regexp.MustCompile(`\ALT(?:[0-9]{7}1[0-9]|[0-9]{10}1[0-9])\z`),
	"LU": regexp.MustCompile(`\ALU[0-9]{8}\z`),
	"LV": regexp.MustCompile(`\ALV[0-9]{11}\z`),
	"MT": regexp.MustCompile(`\AMT[0-9]{8}\z`),
	"NL": regexp.MustCompile(`\ANL[0-9]{9}B[0-9]{2}\z`),
	"PL": regexp.MustCompile(`\APL[0-9]{10}\z`),
	"PT": regexp.MustCompile(`\APT[0-9]{9}\z`),
	"RO": regexp.MustCompile(`\ARO[1-9][0-9]{1,9}\z`),
	"SE": regexp.MustCompile(`\ASE[0-9]{10}01\z`),
	"SI": regexp.MustCompile(`\ASI[0-9]{8}\z`),
	"SK": regexp.MustCompile(`\ASK[0-9]{10}\z`),
}

func vatCountryToISO(prefix string) string {
	switch prefix {
	case "EL":
		return "GR"
	case "XI":
		return "GB"
	}
	return prefix
}

// NormalizeVat upper-cases and strips punctuation, control and whitespace characters,
// matching Valvat::Utils.normalize.
func NormalizeVat(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(raw) {
		if unicode.IsPunct(r) || unicode.IsControl(r) || unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// SplitVat returns the ISO country, the VAT prefix as written, and the number without
// the prefix. ok is false when the prefix is not a VIES/HMRC-supported state.
func SplitVat(normalized string) (iso, prefix, number string, ok bool) {
	if len(normalized) < 3 {
		return "", "", "", false
	}
	prefix = normalized[:2]
	iso = vatCountryToISO(prefix)
	if _, supported := vatPatterns[iso]; !supported {
		return "", "", "", false
	}
	return iso, prefix, normalized[2:], true
}

// VatSyntaxValid is Valvat#valid?: the number matches its country's pattern.
func VatSyntaxValid(normalized string) bool {
	iso, _, _, ok := SplitVat(normalized)
	if !ok {
		return false
	}
	return vatPatterns[iso].MatchString(normalized)
}

// EUVat ports VatValidationService: syntax check first, then a VIES lookup for EU
// numbers. GB has no public lookup, so it stays syntax-only. When VIES cannot answer
// (outage, rate limit, member-state unavailable) the syntax verdict is returned, which is
// what Valvat#exists? falling back to Valvat#valid? did in Ruby.
type EUVat struct {
	viesURL   string
	requester string
	client    *http.Client
	log       *slog.Logger
}

// NewEUVat takes the VIES endpoint, Gumroad's own VAT registration number (sent as the
// requester so VIES issues a consultation number), and an HTTP client whose timeout is
// the VIES timeout (30s in Ruby).
func NewEUVat(client *http.Client, viesURL, requester string, log *slog.Logger) *EUVat {
	if log == nil {
		log = slog.Default()
	}
	return &EUVat{viesURL: viesURL, requester: requester, client: client, log: log}
}

func (v *EUVat) Name() string { return "eu_vat" }

type viesRequest struct {
	CountryCode              string `json:"countryCode"`
	VatNumber                string `json:"vatNumber"`
	RequesterMemberStateCode string `json:"requesterMemberStateCode,omitempty"`
	RequesterNumber          string `json:"requesterNumber,omitempty"`
}

type viesResponse struct {
	Valid     *bool  `json:"valid"`
	UserError string `json:"userError"`
	// Set on 4xx/5xx envelopes.
	ErrorWrappers []struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	} `json:"errorWrappers"`
}

func (v *EUVat) Validate(ctx context.Context, id string) (bool, error) {
	normalized := NormalizeVat(id)
	if normalized == "" {
		return false, nil
	}
	iso, prefix, number, ok := SplitVat(normalized)
	if !ok || !vatPatterns[iso].MatchString(normalized) {
		return false, nil
	}
	if iso == "GB" {
		return true, nil
	}

	valid, err := v.lookup(ctx, prefix, number)
	if err != nil {
		v.log.WarnContext(ctx, "VIES unavailable, falling back to syntax validation",
			"country", iso, "error", err)
		return true, nil
	}
	return valid, nil
}

func (v *EUVat) lookup(ctx context.Context, countryCode, number string) (bool, error) {
	payload := viesRequest{CountryCode: countryCode, VatNumber: number}
	if v.requester != "" {
		req := NormalizeVat(v.requester)
		if len(req) > 2 {
			payload.RequesterMemberStateCode = req[:2]
			payload.RequesterNumber = req[2:]
		}
	}
	body, _ := json.Marshal(payload)
	req, err := newRequest(ctx, http.MethodPost, v.viesURL, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var out viesResponse
	status, err := doJSON(v.client, req, &out)
	if err != nil {
		return false, err
	}
	if status != http.StatusOK || out.Valid == nil {
		return false, &viesError{status: status, userError: out.UserError}
	}
	switch out.UserError {
	case "", "VALID", "INVALID":
		return *out.Valid, nil
	}
	// MS_UNAVAILABLE, SERVICE_UNAVAILABLE, MS_MAX_CONCURRENT_REQ, TIMEOUT, ...
	return false, &viesError{status: status, userError: out.UserError}
}

type viesError struct {
	status    int
	userError string
}

func (e *viesError) Error() string {
	return "vies: status " + http.StatusText(e.status) + " userError=" + e.userError
}

func (e *viesError) Unwrap() error { return ErrUpstream }
