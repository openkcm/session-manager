package credentials

import (
	"net/url"
	"testing"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
)

func TestInsecureProvider(t *testing.T) {
	srv := newDiscoveryServer(t, "https://example.com/introspect")

	p := InsecureProvider{}

	rsrv, err := p.ResourceServer("client-id", srv.URL)
	if err != nil {
		t.Fatalf("ResourceServer() error = %v", err)
	}
	if rsrv == nil {
		t.Fatal("ResourceServer() = nil")
	}

	issuer := srv.URL
	clientID := "client-id"
	oidc := oidcv1.OIDC_builder{Issuer: &issuer, ClientId: &clientID}.Build()

	relyingParty, err := p.RelyingParty(oidc, "https://app.example.com/callback")
	if err != nil {
		t.Fatalf("RelyingParty() error = %v", err)
	}
	if relyingParty == nil {
		t.Fatal("RelyingParty() = nil")
	}
}

func TestClientSecretPostAuth(t *testing.T) {
	form := url.Values{}
	clientSecretPostAuth("client-id", "secret")(form)

	if form.Get("client_id") != "client-id" {
		t.Errorf("client_id = %q, want %q", form.Get("client_id"), "client-id")
	}
	if form.Get("client_secret") != "secret" {
		t.Errorf("client_secret = %q, want %q", form.Get("client_secret"), "secret")
	}

	// With no secret, only the client_id is set.
	form = url.Values{}
	clientSecretPostAuth("client-id", "")(form)
	if form.Has("client_secret") {
		t.Errorf("client_secret set = %q, want none", form.Get("client_secret"))
	}
}

func TestNewClientSecretPostRP(t *testing.T) {
	srv := newDiscoveryServer(t, "https://example.com/introspect")

	got, err := NewClientSecretPostRP(srv.URL, "client-id", "secret", "https://app.example.com/callback")
	if err != nil {
		t.Fatalf("NewClientSecretPostRP() error = %v", err)
	}
	if got == nil {
		t.Fatal("NewClientSecretPostRP() = nil relying party")
	}
	if got.OAuthConfig().ClientSecret != "secret" {
		t.Errorf("ClientSecret = %q, want %q", got.OAuthConfig().ClientSecret, "secret")
	}
}
