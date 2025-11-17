package session_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/pkg/session"
)

func StartOIDCServer(t *testing.T, fail bool) *httptest.Server {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	kid := "test-kid"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "invalid_request", "error_description": "Token exchange failed"}`))
			return
		}

		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(oidc.Configuration{
				Issuer:                server.URL,
				AuthorizationEndpoint: server.URL + "/oauth2/authorize",
				TokenEndpoint:         server.URL + "/oauth2/token",
				JwksURI:               server.URL + "/.well-known/jwks.json",
			})
		case "/.well-known/jwks.json":
			jwk := jose.JSONWebKey{Key: &priv.PublicKey, KeyID: kid, Algorithm: string(jose.RS256), Use: "sig"}
			jwkSet := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
			jwkSetBytes, err := json.Marshal(jwkSet)
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			assert.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(jwkSetBytes)

		case "/oauth2/token":
			now := time.Now()
			claims := map[string]any{
				"sub": "jwt-test",
				"iss": server.URL,
				"aud": []string{"client-id"},
				"iat": now.Unix(),
				"exp": now.Add(time.Hour).Unix(),
				"nbf": now.Unix(),
				"jti": "test-jti",
			}
			signer, err := jose.NewSigner(
				jose.SigningKey{Algorithm: jose.RS256, Key: priv},
				(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid),
			)
			assert.NoError(t, err)
			rawJWT, err := jwt.Signed(signer).Claims(claims).Serialize()
			assert.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(session.TokenResponse{
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
				IDToken:      rawJWT,
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			})
		default:
			http.NotFound(w, r)
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
