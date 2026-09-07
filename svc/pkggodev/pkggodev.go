package pkggodev

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const gopkgAPIEndpoint = "https://pkg.go.dev/v1"

type apiErr struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Fixes   []string `json:"fixes"`
}

// RequestBuilder builds the http request for a given endpoint
func RequestBuilder(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	if method != http.MethodGet && method != http.MethodPost {
		return nil, fmt.Errorf("unsupported method: %s", method)
	}

	// Ensure base ends with slash so ResolveReference appends instead of replacing last segment
	baseStr := gopkgAPIEndpoint
	if baseStr[len(baseStr)-1] != '/' {
		baseStr += "/"
	}
	base, err := url.Parse(baseStr)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}

	rel := &url.URL{Path: path}
	if query != nil {
		rel.RawQuery = query.Encode()
	}
	destURI := base.ResolveReference(rel).String()

	req, err := http.NewRequestWithContext(ctx, method, destURI, body)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	return req, nil
}
