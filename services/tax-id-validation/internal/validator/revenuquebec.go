package validator

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

const DefaultRevenuQuebecURL = "https://svcnab2b.revenuquebec.ca/2019/02/ValidationTVQ/"

// RevenuQuebec validates QST numbers. Ruby: QstValidationService.
type RevenuQuebec struct {
	baseURL string
	client  *http.Client
}

func NewQST(client *http.Client, baseURL string) *RevenuQuebec {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &RevenuQuebec{baseURL: baseURL, client: client}
}

func (v *RevenuQuebec) Name() string { return "qst" }

type revenuQuebecResponse struct {
	Resultat struct {
		StatutSousDossierUsager string `json:"StatutSousDossierUsager"`
	} `json:"Resultat"`
}

func (v *RevenuQuebec) Validate(ctx context.Context, id string) (bool, error) {
	if Blank(id) {
		return false, nil
	}
	req, err := newRequest(ctx, http.MethodGet, v.baseURL+url.PathEscape(id), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/json")

	var out revenuQuebecResponse
	status, err := doJSON(v.client, req, &out)
	if err != nil {
		return false, err
	}
	return status == http.StatusOK && out.Resultat.StatutSousDossierUsager == "R", nil
}
