package credentials

import (
	"testing"
)

func TestNewClientSecretPostRS(t *testing.T) {
	const introspectionURL = "https://example.com/introspect"
	srv := newDiscoveryServer(t, introspectionURL)

	got, err := NewClientSecretPostRS(srv.URL, "client-id", "secret")
	if err != nil {
		t.Fatalf("NewClientSecretPostRS() error = %v", err)
	}

	if got.IntrospectionURL() != introspectionURL {
		t.Errorf("IntrospectionURL() = %q, want %q", got.IntrospectionURL(), introspectionURL)
	}

	form := authForm(t, got)
	if form.Get("client_id") != "client-id" {
		t.Errorf("client_id = %q, want %q", form.Get("client_id"), "client-id")
	}
	if form.Get("client_secret") != "secret" {
		t.Errorf("client_secret set = %q, want %q", form.Get("client_secret"), "secret")
	}
}
