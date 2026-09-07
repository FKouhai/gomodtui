package pkggodev

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Symbol represents a single Go symbol.
type Symbol struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"` // Constant, Variable, Function, Type, Field, Method
	Synopsis string `json:"synopsis"`
	Parent   string `json:"parent"`
}

// PackageSymbols is the response for /v1/symbols/{path}
type PackageSymbols struct {
	ModulePath string `json:"modulePath"`
	Version    string `json:"version"`
	Symbols    struct {
		Items         []Symbol `json:"items"`
		Total         int      `json:"total"`
		NextPageToken string   `json:"nextPageToken"`
	} `json:"symbols"`
}

// SymbolsOptions query options for symbols endpoint
type SymbolsOptions struct {
	Module  string // ?module
	Version string // ?version
	GOOS    string // ?goos
	GOARCH  string // ?goarch
	Filter  string // ?filter (Go expression, e.g. `kind == "Function"` or contains)
	Limit   int    // ?limit
	Token   string // ?token pagination
}

// GetSymbolsWithContext fetches symbols for a package path.
func GetSymbolsWithContext(ctx context.Context, client *http.Client, importPath string, opts *SymbolsOptions) (*PackageSymbols, error) {
	values := url.Values{}
	if opts != nil {
		if opts.Module != "" {
			values.Set("module", opts.Module)
		}
		if opts.Version != "" {
			values.Set("version", opts.Version)
		}
		if opts.GOOS != "" {
			values.Set("goos", opts.GOOS)
		}
		if opts.GOARCH != "" {
			values.Set("goarch", opts.GOARCH)
		}
		if opts.Filter != "" {
			values.Set("filter", opts.Filter)
		}
		if opts.Limit > 0 {
			values.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
		if opts.Token != "" {
			values.Set("token", opts.Token)
		}
	}
	endpoint := "symbols/" + importPath
	req, err := RequestBuilder(ctx, http.MethodGet, endpoint, values, nil)
	if err != nil {
		return nil, fmt.Errorf("symbols request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("symbols response: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var apiErr apiErr
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("api error %d: %s (fixes: %v) body: %s", apiErr.Code, apiErr.Message, apiErr.Fixes, string(body))
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
	var result PackageSymbols
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}
