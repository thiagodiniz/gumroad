package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// maxBody bounds registry responses; every registry we call answers in well under 1 MiB.
const maxBody = 1 << 20

// doJSON performs req and decodes a 2xx JSON body into out. Transport failures and
// unparseable 2xx bodies are reported as ErrUpstream; the status code is returned so callers
// can apply registry-specific rules (taxid.pro and Revenu Québec treat non-200 as invalid,
// not as an outage).
func doJSON(client *http.Client, req *http.Request, out any) (int, error) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return resp.StatusCode, fmt.Errorf("%w: reading body: %v", ErrUpstream, err)
	}
	// Error bodies are often HTML/text; callers decide what a non-2xx status means.
	if len(body) == 0 || resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return resp.StatusCode, fmt.Errorf("%w: decoding body (status %d): %v", ErrUpstream, resp.StatusCode, err)
	}
	return resp.StatusCode, nil
}

func newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("%w: building request: %v", ErrUpstream, err)
	}
	return req, nil
}
