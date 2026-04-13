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
	"github.com/openkcm/common-sdk/pkg/oidc"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/session"
)

// StartOIDCServer creates a test OIDC server with signed ID tokens for testing.
func StartOIDCServer(t *testing.T, fail bool, algs ...string) *httptest.Server {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "generating RSA key for OIDC test server")

	const keyID = "test-kid"
	signingKey := jose.SigningKey{
		Algorithm: jose.RS256,
		Key: jose.JSONWebKey{
			Key:       privateKey,
			KeyID:     keyID,
			Algorithm: string(jose.RS256),
		},
	}
	signer, err := jose.NewSigner(signingKey, (&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err, "creating JWT signer")

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "invalid_request", "error_description": "Token exchange failed"}`))
			return
		}
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
			publicJWKS := jose.JSONWebKeySet{
				Keys: []jose.JSONWebKey{{
					Key:       &privateKey.PublicKey,
					KeyID:     keyID,
					Algorithm: string(jose.RS256),
					Use:       "sig",
				}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(publicJWKS)

		case "/oauth2/token":
			w.Header().Set("Content-Type", "application/json")

			now := time.Now()
			claims := map[string]any{
				"iss":         server.URL,
				"sub":         "jwt-test",
				"aud":         []string{""},
				"exp":         now.Add(time.Hour).Unix(),
				"iat":         now.Unix(),
				"sid":         "test-sid",
				"user_uuid":   "test-uuid",
				"given_name":  "John",
				"family_name": "Doe",
				"email":       "john.doe@example.com",
				"groups":      []string{"admin"},
			}

			idToken, signErr := jwt.Signed(signer).Claims(claims).Serialize()
			if signErr != nil {
				http.Error(w, "id_token sign error", http.StatusInternalServerError)
				return
			}

			tokenResponse := session.TokenResponse{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				IDToken:      idToken,
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}
			_ = json.NewEncoder(w).Encode(tokenResponse)
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
