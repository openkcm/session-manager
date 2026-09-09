package credentials

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zitadel/oidc/v3/pkg/oidc"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func Test_buildRPHTTPClient_withDiscoveryConfig(t *testing.T) {
	disc := &oidc.DiscoveryConfiguration{
		Issuer:        "https://example.com",
		TokenEndpoint: "https://example.com/token",
	}

	called := false
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	client, err := buildRPHTTPClient("https://example.com", base, []Option{WithDiscoveryConfig(disc)})
	if err != nil {
		t.Fatalf("buildRPHTTPClient() error = %v", err)
	}
	cdt, ok := client.Transport.(*cachedDiscoveryTransport)
	if !ok {
		t.Fatalf("Transport is %T, want *cachedDiscoveryTransport", client.Transport)
	}
	if len(cdt.body) == 0 {
		t.Error("cached discovery body should not be empty")
	}

	// Verify base transport is forwarded for non-discovery URLs.
	req := httptest.NewRequest(http.MethodGet, "https://example.com/token", nil)
	if _, err := cdt.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	if !called {
		t.Error("base RoundTrip not called for non-well-known URL")
	}
}

func Test_buildRPHTTPClient_withoutDiscoveryConfig(t *testing.T) {
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	client, err := buildRPHTTPClient("https://example.com", base, nil)
	if err != nil {
		t.Fatalf("buildRPHTTPClient() error = %v", err)
	}
	if _, ok := client.Transport.(roundTripFunc); !ok {
		t.Fatalf("Transport is %T, want roundTripFunc", client.Transport)
	}
}

func Test_cachedDiscoveryTransport_wellKnownURL(t *testing.T) {
	const wellKnownURL = "https://example.com/.well-known/openid-configuration"
	body := []byte(`{"issuer":"https://example.com"}`)

	cdt := &cachedDiscoveryTransport{
		wellKnownURL: wellKnownURL,
		body:         body,
	}

	req := httptest.NewRequest(http.MethodGet, wellKnownURL, nil)
	resp, err := cdt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", resp.Header.Get("Content-Type"))
	}
}
