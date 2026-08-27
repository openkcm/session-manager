package credentials

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestNewTLSRS(t *testing.T) {
	const introspectionURL = "https://example.com/introspect"
	srv := newDiscoveryServer(t, introspectionURL)

	tlsConfig := &tls.Config{}
	got, err := NewTLSRS("client-id", srv.URL, tlsConfig)
	if err != nil {
		t.Fatalf("NewTLSRS() error = %v", err)
	}

	if got.IntrospectionURL() != introspectionURL {
		t.Errorf("IntrospectionURL() = %q, want %q", got.IntrospectionURL(), introspectionURL)
	}

	// mTLS sends only the client_id in the body; the certificate authenticates.
	form := authForm(t, got)
	if form.Get("client_id") != "client-id" {
		t.Errorf("client_id = %q, want %q", form.Get("client_id"), "client-id")
	}
	if form.Has("client_secret") {
		t.Errorf("client_secret set = %q, want none", form.Get("client_secret"))
	}

	// The configured TLS config must be wired into the HTTP client's transport.
	transport, ok := got.HttpClient().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport is %T, want *http.Transport", got.HttpClient().Transport)
	}
	if transport.TLSClientConfig != tlsConfig {
		t.Errorf("TLSClientConfig = %p, want %p", transport.TLSClientConfig, tlsConfig)
	}
}

func TestNewTLSRS_DiscoveryError(t *testing.T) {
	if _, err := NewTLSRS("client-id", "http://127.0.0.1:0", &tls.Config{}); err == nil {
		t.Error("NewTLSRS() error = nil, want error")
	}
}

func TestNewTLSRP(t *testing.T) {
	srv := newDiscoveryServer(t, "https://example.com/introspect")

	tlsConfig := &tls.Config{}
	got, err := NewTLSRP("client-id", srv.URL, "https://app.example.com/callback", tlsConfig)
	if err != nil {
		t.Fatalf("NewTLSRP() error = %v", err)
	}
	if got == nil {
		t.Fatal("NewTLSRP() = nil relying party")
	}
	// mTLS authenticates via the client certificate, so no client secret is set.
	if got.OAuthConfig().ClientSecret != "" {
		t.Errorf("ClientSecret = %q, want empty", got.OAuthConfig().ClientSecret)
	}
	// The mTLS transport must be wired into the relying party's HTTP client.
	transport, ok := got.HttpClient().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport is %T, want *http.Transport", got.HttpClient().Transport)
	}
	if transport.TLSClientConfig != tlsConfig {
		t.Errorf("TLSClientConfig = %p, want %p", transport.TLSClientConfig, tlsConfig)
	}
}
