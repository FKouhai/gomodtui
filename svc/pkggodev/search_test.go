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

func newSearchServer(t *testing.T, wantPath string, wantQuery func(url.Values) error, status int, body string) *httptest.Server {
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

func TestSearchWithContext(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		resp := PaginatedSearch{
			Items: []SearchResult{{PackagePath: "github.com/nats-io/nats.go", ModulePath: "github.com/nats-io/nats.go", Version: "v1.53.1", Synopsis: "A Go client"}},
			Total: 1,
		}
		b, _ := json.Marshal(resp)
		srv := newSearchServer(t, "/v1/search", func(q url.Values) error {
			if q.Get("q") != "nats" {
				return errors.New("q = " + q.Get("q") + " want nats")
			}
			if q.Get("limit") != "25" {
				return errors.New("limit = " + q.Get("limit") + " want 25")
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
		got, err := SearchWithContext(context.Background(), client, "nats")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Items) != 1 || got.Items[0].PackagePath != "github.com/nats-io/nats.go" {
			t.Fatalf("got %+v, want 1 item", got)
		}
		if got.Total != 1 {
			t.Fatalf("total = %d, want 1", got.Total)
		}
	})

	t.Run("empty_query", func(t *testing.T) {
		t.Parallel()
		resp := PaginatedSearch{Items: []SearchResult{}, Total: 0}
		b, _ := json.Marshal(resp)
		srv := newSearchServer(t, "/v1/search", func(q url.Values) error {
			if q.Get("q") != "" {
				return errors.New("q should be empty")
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
		_, err := SearchWithContext(context.Background(), client, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("query_encoding", func(t *testing.T) {
		t.Parallel()
		resp := PaginatedSearch{Items: []SearchResult{}, Total: 0}
		b, _ := json.Marshal(resp)
		srv := newSearchServer(t, "/v1/search", func(q url.Values) error {
			if q.Get("q") != "http client" {
				return errors.New("q = " + q.Get("q") + " want 'http client'")
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
		_, err := SearchWithContext(context.Background(), client, "http client")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("404_apiErr", func(t *testing.T) {
		t.Parallel()
		body := `{"code":404,"message":"not found","fixes":["retry"]}`
		srv := newSearchServer(t, "/v1/search", nil, http.StatusNotFound, body)
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
		_, err := SearchWithContext(context.Background(), client, "nats")
		if err == nil || !strings.Contains(err.Error(), "api error 404") {
			t.Fatalf("error = %v, want api error 404", err)
		}
	})

	t.Run("500_text", func(t *testing.T) {
		t.Parallel()
		srv := newSearchServer(t, "/v1/search", nil, http.StatusInternalServerError, "internal error")
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
		_, err := SearchWithContext(context.Background(), client, "nats")
		if err == nil || !strings.Contains(err.Error(), "unexpected status 500") {
			t.Fatalf("error = %v, want unexpected status 500", err)
		}
	})

	t.Run("200_invalid_json", func(t *testing.T) {
		t.Parallel()
		srv := newSearchServer(t, "/v1/search", nil, http.StatusOK, "{invalid")
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
		_, err := SearchWithContext(context.Background(), client, "nats")
		if err == nil || !strings.Contains(err.Error(), "decode response") {
			t.Fatalf("error = %v, want decode response", err)
		}
	})

	t.Run("context_canceled", func(t *testing.T) {
		t.Parallel()
		srv := newSearchServer(t, "/v1/search", nil, http.StatusOK, "{}")
		defer srv.Close()
		client := srv.Client()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := SearchWithContext(ctx, client, "nats")
		if err == nil {
			t.Fatal("expected context canceled error")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "canceled") && !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context canceled", err)
		}
	})

	t.Run("network_error", func(t *testing.T) {
		t.Parallel()
		client := &http.Client{Transport: errorRoundTripper{err: errors.New("fail")}}
		_, err := SearchWithContext(context.Background(), client, "nats")
		if err == nil || !strings.Contains(err.Error(), "fail") {
			t.Fatalf("error = %v, want fail", err)
		}
	})
}
