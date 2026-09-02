package credentials

import (
	"testing"
)

func TestNewInsecureRS(t *testing.T) {
	const introspectionURL = "https://example.com/introspect"
	srv := newDiscoveryServer(t, introspectionURL)

	got, err := NewInsecureRS("client-id", srv.URL)
	if err != nil {
		t.Fatalf("NewInsecureRS() error = %v", err)
	}

	if got.IntrospectionURL() != introspectionURL {
		t.Errorf("IntrospectionURL() = %q, want %q", got.IntrospectionURL(), introspectionURL)
	}

	form := authForm(t, got)
	if form.Get("client_id") != "client-id" {
		t.Errorf("client_id = %q, want %q", form.Get("client_id"), "client-id")
	}
	if form.Has("client_secret") {
		t.Errorf("client_secret set = %q, want none", form.Get("client_secret"))
	}
}

func TestNewInsecureRP(t *testing.T) {
	srv := newDiscoveryServer(t, "https://example.com/introspect")

	got, err := NewInsecureRP("client-id", srv.URL, "https://app.example.com/callback")
	if err != nil {
		t.Fatalf("NewInsecureRP() error = %v", err)
	}
	if got == nil {
		t.Fatal("NewInsecureRP() = nil relying party")
	}
	if got.OAuthConfig().ClientID != "client-id" {
		t.Errorf("ClientID = %q, want %q", got.OAuthConfig().ClientID, "client-id")
	}
	if got.OAuthConfig().ClientSecret != "" {
		t.Errorf("ClientSecret = %q, want empty", got.OAuthConfig().ClientSecret)
	}
}
