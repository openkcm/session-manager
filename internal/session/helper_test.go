package session_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/openkcm/common-sdk/pkg/oidc"

	"github.com/openkcm/session-manager/internal/session"
)

const oidcAccessToken = "access-token"

// atHashRS256 computes the OIDC at_hash claim for an RS256-signed ID token:
// the base64url-encoded left-most half of the SHA-256 hash of the access token.
func atHashRS256(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:len(sum)/2])
}

func StartOIDCServer(t *testing.T, fail bool, algs ...string) *httptest.Server {
	t.Helper()

	// Sign ID tokens with a freshly generated key whose public half is exposed
	// via the JWKS endpoint, so tokens verify against the advertised keys.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	const kid = "test-kid"

	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.RS256,
		Key: jose.JSONWebKey{
			Key:       key,
			KeyID:     kid,
			Algorithm: string(jose.RS256),
		},
	}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "invalid_request", "error_description": "Token exchange failed"}`))
			return
		}
		// Determine supported algorithms by passed arguments or set to default value
		var algList []string
		if len(algs) == 0 {
			algList = []string{"RS256"}
		} else {
			algList = algs
		}

		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(oidc.Configuration{
				Issuer:                           server.URL,
				AuthorizationEndpoint:            server.URL + "/oauth2/authorize",
				TokenEndpoint:                    server.URL + "/oauth2/token",
				JwksURI:                          server.URL + "/.well-known/jwks.json",
				IDTokenSigningAlgValuesSupported: algList,
			})
		case "/.well-known/jwks.json":
			_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
				Key:       &key.PublicKey,
				KeyID:     kid,
				Algorithm: string(jose.RS256),
				Use:       "sig",
			}}})
		case "/oauth2/token":
			now := time.Now()
			idToken, err := jwt.Signed(signer).Claims(map[string]any{
				"iss":     server.URL,
				"sub":     "jwt-test",
				"sid":     "provider-session-id",
				"aud":     testClientID,
				"iat":     now.Unix(),
				"nbf":     now.Unix(),
				"exp":     now.Add(time.Hour).Unix(),
				"at_hash": atHashRS256(oidcAccessToken),
			}).Serialize()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(session.TokenResponse{
				AccessToken:  oidcAccessToken,
				RefreshToken: "refresh-token",
				IDToken:      idToken,
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			})
		}
	}))

	return server
}

func StartAuditServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success": true}`))
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
}
