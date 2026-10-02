package validator

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

const (
	IrasProductionURL = "https://apiservices.iras.gov.sg/iras/prod/GSTListing/SearchGSTRegistered"
	IrasSandboxURL    = "https://apisandbox.iras.gov.sg/iras/sb/GSTListing/SearchGSTRegistered"
)

// IRAS validates Singapore GST registration numbers. Ruby: GstValidationService.
type IRAS struct {
	url      string
	clientID string
	secret   string
	client   *http.Client
}

func NewGST(client *http.Client, apiURL, clientID, secret string) *IRAS {
	return &IRAS{url: apiURL, clientID: clientID, secret: secret, client: client}
}

func (v *IRAS) Name() string { return "gst" }

type irasResponse struct {
	ReturnCode string `json:"returnCode"`
	Data       struct {
		Status string `json:"Status"`
	} `json:"data"`
}

func (v *IRAS) Validate(ctx context.Context, id string) (bool, error) {
	if Blank(id) {
		return false, nil
	}
	body, _ := json.Marshal(map[string]string{"clientID": v.clientID, "regID": id})
	req, err := newRequest(ctx, http.MethodPost, v.url, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("X-IBM-Client-Id", v.clientID)
	req.Header.Set("X-IBM-Client-Secret", v.secret)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	var out irasResponse
	if _, err := doJSON(v.client, req, &out); err != nil {
		return false, err
	}
	return out.ReturnCode == "10" && out.Data.Status == "Registered", nil
}
