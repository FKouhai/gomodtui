package pkggodev

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// PackageOptions query options for search packages
type PackageOptions struct {
	Module                      string // ?module
	Version                     string // ?version
	GOOS, GOARCH                string // ?goos ?goarch
	Doc                         string // ?doc=text|html|md
	Examples, Imports, Licenses bool   // ?examples ?imports ?licenses
}

// Package structured response from the package endpoint
type Package struct {
	Path              string   `json:"path"`
	ModulePath        string   `json:"modulePath"`
	Name              string   `json:"name"`
	Version           string   `json:"version"`
	Synopsis          string   `json:"synopsis"`
	Docs              string   `json:"docs"`
	Imports           []string `json:"imports"`
	IsLatest          bool     `json:"isLatest"`
	IsRedistributable bool     `json:"isRedistributable"`
	// Licenses []License etc. minimal for test
}

// GetPackageWithContext runs the http request on the package endpoint
func GetPackageWithContext(ctx context.Context, client *http.Client, importPath string, opts *PackageOptions) (*Package, error) {
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
		if opts.Doc != "" {
			values.Set("doc", opts.Doc)
		}
		if opts.Examples {
			values.Set("examples", "true")
		}
		if opts.Imports {
			values.Set("imports", "true")
		}
		if opts.Licenses {
			values.Set("licenses", "true")
		}
	}
	packageEndpoint := "package/" + importPath

	req, err := RequestBuilder(ctx, http.MethodGet, packageEndpoint, values, nil)
	if err != nil {
		return nil, fmt.Errorf("package search: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("package search response: %w", err)
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
	var result Package
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}
