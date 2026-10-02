package validator

import (
	"context"
	"net/http"
	"net/url"
)

const DefaultTaxIDProURL = "https://v3.api.taxid.pro/validate"

// TaxIDPro validates TINs for the countries listed in countries.go via taxid.pro.
// Ruby: TaxIdValidationService.
type TaxIDPro struct {
	url    string
	apiKey string
	client *http.Client
}

func NewTaxIDPro(client *http.Client, apiURL, apiKey string) *TaxIDPro {
	return &TaxIDPro{url: apiURL, apiKey: apiKey, client: client}
}

func (v *TaxIDPro) Name() string { return "tax_id_pro" }

type taxIDProResponse struct {
	IsValid bool `json:"is_valid"`
}

func (v *TaxIDPro) Validate(ctx context.Context, id, countryCode string) (bool, error) {
	if Blank(id) || Blank(countryCode) {
		return false, nil
	}
	q := url.Values{"country": {countryCode}, "tin": {id}}
	req, err := newRequest(ctx, http.MethodGet, v.url+"?"+q.Encode(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+v.apiKey)

	var out taxIDProResponse
	status, err := doJSON(v.client, req, &out)
	if err != nil {
		return false, err
	}
	return status == http.StatusOK && out.IsValid, nil
}
