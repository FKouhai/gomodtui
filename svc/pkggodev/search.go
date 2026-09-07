package pkggodev

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// SearchResult is the structured output of the pkg.go.dev API of the search endpoint
type SearchResult struct {
	PackagePath string `json:"packagePath"`
	ModulePath  string `json:"modulePath"`
	Version     string `json:"version"`
	Synopsis    string `json:"synopsis"`
}

// PaginatedSearch for the pagianted search
type PaginatedSearch struct {
	Items         []SearchResult `json:"items"`
	Total         int            `json:"total"`
	NextPageToken string         `json:"nextPageToken"`
}

// SearchWithContext runs the http request on the search ednpoint with a given context
func SearchWithContext(ctx context.Context, httpClient *http.Client, query string) (*PaginatedSearch, error) {
	values := url.Values{}
	values.Set("q", query)
	values.Set("limit", "25")

	req, err := RequestBuilder(ctx, http.MethodGet, "search", values, nil)
	if err != nil {
		return nil, fmt.Errorf("http client builder: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Try to parse structured API error
		var apiErr apiErr
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("api error %d: %s (fixes: %v) body: %s", apiErr.Code, apiErr.Message, apiErr.Fixes, string(body))
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result PaginatedSearch
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}
