package pkggodev

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type errorRoundTripper struct{ err error }

func (e errorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, e.err
}

func newPackageServer(t *testing.T, wantPath string, wantQuery func(url.Values) error, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		if wantQuery != nil {
			if err := wantQuery(r.URL.Query()); err != nil {
				t.Error(err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestGetPackageWithContext(t *testing.T) {
	t.Parallel()

	t.Run("nil_opts_success", func(t *testing.T) {
		t.Parallel()
		pkg := Package{Path: "golang.org/x/time/rate", Name: "rate", Version: "v0.6.0", Synopsis: "Package rate provides a rate limiter."}
		b, _ := json.Marshal(pkg)
		srv := newPackageServer(t, "/v1/package/golang.org/x/time/rate", func(q url.Values) error {
			if len(q) != 0 {
				return errors.New("expected empty query for nil opts, got " + q.Encode())
			}
			return nil
		}, http.StatusOK, string(b))
		defer srv.Close()

		client := srv.Client()
		// Need to override base endpoint for test via RequestBuilder? Instead we hijack by using srv URL via custom helper:
		// For isolation we call GetPackageWithContext with a client that rewrites request URL to server.
		// Simpler: we test via direct RequestBuilder integration by not checking full host, only path, so we need to allow GetPackageWithContext to hit test server.
		// We achieve by temporarily not using gopkgAPIEndpoint but we can't patch const easily.
		// Alternative: we test the function by creating a test that validates logic via httptest by making client Transport rewrite URL.
		// To keep stdlib, we use a custom Transport that replaces scheme/host with test server's.
		u, _ := url.Parse(srv.URL)
		origTransport := client.Transport
		if origTransport == nil {
			origTransport = http.DefaultTransport
		}
		client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			return origTransport.RoundTrip(req)
		})

		got, err := GetPackageWithContext(context.Background(), client, "golang.org/x/time/rate", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Path != pkg.Path || got.Name != pkg.Name {
			t.Fatalf("got %+v, want %+v", got, pkg)
		}
	})

	t.Run("all_opts_encoded", func(t *testing.T) {
		t.Parallel()
		opts := &PackageOptions{
			Module:   "golang.org/x/time",
			Version:  "v0.6.0",
			GOOS:     "linux",
			GOARCH:   "amd64",
			Doc:      "text",
			Examples: true,
			Imports:  true,
			Licenses: false,
		}
		pkg := Package{Path: "golang.org/x/time/rate", ModulePath: "golang.org/x/time"}
		b, _ := json.Marshal(pkg)
		srv := newPackageServer(t, "/v1/package/golang.org/x/time/rate", func(q url.Values) error {
			if q.Get("module") != "golang.org/x/time" {
				return errors.New("module = " + q.Get("module"))
			}
			if q.Get("version") != "v0.6.0" {
				return errors.New("version = " + q.Get("version"))
			}
			if q.Get("goos") != "linux" {
				return errors.New("goos = " + q.Get("goos"))
			}
			if q.Get("goarch") != "amd64" {
				return errors.New("goarch = " + q.Get("goarch"))
			}
			if q.Get("doc") != "text" {
				return errors.New("doc = " + q.Get("doc"))
			}
			if q.Get("examples") != "true" {
				return errors.New("examples = " + q.Get("examples"))
			}
			if q.Get("imports") != "true" {
				return errors.New("imports = " + q.Get("imports"))
			}
			if q.Has("licenses") {
				return errors.New("licenses should be omitted when false")
			}
			return nil
		}, http.StatusOK, string(b))
		defer srv.Close()
		client := srv.Client()
		u, _ := url.Parse(srv.URL)
		origTransport := client.Transport
		if origTransport == nil {
			origTransport = http.DefaultTransport
		}
		client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			return origTransport.RoundTrip(req)
		})
		_, err := GetPackageWithContext(context.Background(), client, "golang.org/x/time/rate", opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty_omitted", func(t *testing.T) {
		t.Parallel()
		opts := &PackageOptions{Version: "", Doc: "", GOOS: ""}
		pkg := Package{Path: "example.com/foo"}
		b, _ := json.Marshal(pkg)
		srv := newPackageServer(t, "/v1/package/example.com/foo", func(q url.Values) error {
			if q.Has("version") {
				return errors.New("version should be omitted")
			}
			if q.Has("doc") {
				return errors.New("doc should be omitted")
			}
			if q.Has("goos") {
				return errors.New("goos should be omitted")
			}
			return nil
		}, http.StatusOK, string(b))
		defer srv.Close()
		client := srv.Client()
		u, _ := url.Parse(srv.URL)
		origTransport := client.Transport
		if origTransport == nil {
			origTransport = http.DefaultTransport
		}
		client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			return origTransport.RoundTrip(req)
		})
		_, err := GetPackageWithContext(context.Background(), client, "example.com/foo", opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("bool_true_only", func(t *testing.T) {
		t.Parallel()
		t.Run("imports_true", func(t *testing.T) {
			opts := &PackageOptions{Imports: true}
			pkg := Package{Path: "example.com/foo"}
			b, _ := json.Marshal(pkg)
			srv := newPackageServer(t, "/v1/package/example.com/foo", func(q url.Values) error {
				if q.Get("imports") != "true" {
					return errors.New("imports != true")
				}
				return nil
			}, http.StatusOK, string(b))
			defer srv.Close()
			client := srv.Client()
			u, _ := url.Parse(srv.URL)
			origTransport := client.Transport
			if origTransport == nil {
				origTransport = http.DefaultTransport
			}
			client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = u.Scheme
				req.URL.Host = u.Host
				return origTransport.RoundTrip(req)
			})
			_, err := GetPackageWithContext(context.Background(), client, "example.com/foo", opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		t.Run("licenses_false_omitted", func(t *testing.T) {
			opts := &PackageOptions{Licenses: false}
			pkg := Package{Path: "example.com/foo"}
			b, _ := json.Marshal(pkg)
			srv := newPackageServer(t, "/v1/package/example.com/foo", func(q url.Values) error {
				if q.Has("licenses") {
					return errors.New("licenses should be omitted when false")
				}
				return nil
			}, http.StatusOK, string(b))
			defer srv.Close()
			client := srv.Client()
			u, _ := url.Parse(srv.URL)
			origTransport := client.Transport
			if origTransport == nil {
				origTransport = http.DefaultTransport
			}
			client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = u.Scheme
				req.URL.Host = u.Host
				return origTransport.RoundTrip(req)
			})
			_, err := GetPackageWithContext(context.Background(), client, "example.com/foo", opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})

	t.Run("stdlib_path", func(t *testing.T) {
		t.Parallel()
		pkg := Package{Path: "net/http", Name: "http"}
		b, _ := json.Marshal(pkg)
		srv := newPackageServer(t, "/v1/package/net/http", nil, http.StatusOK, string(b))
		defer srv.Close()
		client := srv.Client()
		u, _ := url.Parse(srv.URL)
		origTransport := client.Transport
		if origTransport == nil {
			origTransport = http.DefaultTransport
		}
		client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			return origTransport.RoundTrip(req)
		})
		got, err := GetPackageWithContext(context.Background(), client, "net/http", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Path != "net/http" {
			t.Fatalf("path = %q, want net/http", got.Path)
		}
	})

	t.Run("404_apiErr", func(t *testing.T) {
		t.Parallel()
		body := `{"code":404,"message":"not found","fixes":["retry with module"]}`
		srv := newPackageServer(t, "/v1/package/not/exist", nil, http.StatusNotFound, body)
		defer srv.Close()
		client := srv.Client()
		u, _ := url.Parse(srv.URL)
		origTransport := client.Transport
		if origTransport == nil {
			origTransport = http.DefaultTransport
		}
		client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			return origTransport.RoundTrip(req)
		})
		_, err := GetPackageWithContext(context.Background(), client, "not/exist", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "api error 404") {
			t.Fatalf("error = %q, want api error 404", err.Error())
		}
	})

	t.Run("500_text", func(t *testing.T) {
		t.Parallel()
		srv := newPackageServer(t, "/v1/package/example.com/foo", nil, http.StatusInternalServerError, "internal error")
		defer srv.Close()
		client := srv.Client()
		u, _ := url.Parse(srv.URL)
		origTransport := client.Transport
		if origTransport == nil {
			origTransport = http.DefaultTransport
		}
		client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			return origTransport.RoundTrip(req)
		})
		_, err := GetPackageWithContext(context.Background(), client, "example.com/foo", nil)
		if err == nil || !strings.Contains(err.Error(), "unexpected status 500") {
			t.Fatalf("error = %v, want unexpected status 500", err)
		}
	})

	t.Run("200_invalid_json", func(t *testing.T) {
		t.Parallel()
		srv := newPackageServer(t, "/v1/package/example.com/foo", nil, http.StatusOK, "{invalid")
		defer srv.Close()
		client := srv.Client()
		u, _ := url.Parse(srv.URL)
		origTransport := client.Transport
		if origTransport == nil {
			origTransport = http.DefaultTransport
		}
		client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			return origTransport.RoundTrip(req)
		})
		_, err := GetPackageWithContext(context.Background(), client, "example.com/foo", nil)
		if err == nil || !strings.Contains(err.Error(), "decode response") {
			t.Fatalf("error = %v, want decode response", err)
		}
	})

	t.Run("context_canceled", func(t *testing.T) {
		t.Parallel()
		srv := newPackageServer(t, "/v1/package/example.com/foo", nil, http.StatusOK, "{}")
		defer srv.Close()
		client := srv.Client()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := GetPackageWithContext(ctx, client, "example.com/foo", nil)
		if err == nil {
			t.Fatal("expected context canceled error")
		}
		if !strings.Contains(err.Error(), "context canceled") && !errors.Is(err, context.Canceled) {
			// RequestBuilder wraps, so check contains
			if !strings.Contains(strings.ToLower(err.Error()), "canceled") {
				t.Fatalf("error = %v, want context canceled", err)
			}
		}
	})

	t.Run("network_error", func(t *testing.T) {
		t.Parallel()
		client := &http.Client{Transport: errorRoundTripper{err: errors.New("fail")}}
		_, err := GetPackageWithContext(context.Background(), client, "example.com/foo", nil)
		if err == nil || !strings.Contains(err.Error(), "fail") {
			t.Fatalf("error = %v, want fail", err)
		}
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
