package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// localRoundTripper is an http.RoundTripper that executes HTTP transactions by
// using handler directly, instead of going over an HTTP connection.
type localRoundTripper struct {
	handler http.Handler
}

func (l localRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	l.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

func TestGetOpenIDConfig(t *testing.T) {
	const (
		issuerURL = "https://example.com"
	)
	tests := []struct {
		name      string
		issuerURL string
		config    Configuration
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "Invalid issuer URL",
			issuerURL: "+adf",
			config:    Configuration{},
			wantErr:   assert.Error,
		}, {
			name:      "Valid response",
			issuerURL: issuerURL,
			config: Configuration{
				Issuer:                issuerURL,
				IntrospectionEndpoint: issuerURL + "/oauth2/introspect",
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			httpClient := &http.Client{
				Transport: localRoundTripper{
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						err := json.NewEncoder(w).Encode(tt.config)
						if err != nil {
							w.WriteHeader(http.StatusInternalServerError)
						}
					}),
				},
			}
			provider := &Provider{
				IssuerURL: tt.issuerURL,
			}

			// Act
			got, err := provider.GetOpenIDConfig(t.Context(), httpClient)

			// Assert
			if !tt.wantErr(t, err) {
				return
			}
			assert.Equal(t, tt.config, got)
		})
	}
}

func TestIntrospectToken(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err)

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":  "me",
		"mail": "me@my.world",
		"iss":  "https://example.com/",
		"exp":  time.Now().Add(48 * time.Hour).Unix(),
	})
	rawToken, err := token.SignedString(priv)
	require.NoError(t, err)

	tests := []struct {
		name       string
		rawToken   string
		active     bool
		wantActive bool
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "Introspect active token",
			active:     true,
			rawToken:   rawToken,
			wantActive: true,
			wantErr:    assert.NoError,
		}, {
			name:       "Introspect inactive token",
			active:     false,
			rawToken:   rawToken,
			wantActive: false,
			wantErr:    assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			httpClient := &http.Client{
				Transport: localRoundTripper{
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						err := json.NewEncoder(w).Encode(Introspection{Active: tt.active})
						if err != nil {
							w.WriteHeader(http.StatusInternalServerError)
						}
					}),
				},
			}
			provider := &Provider{}

			// Act
			got, err := provider.IntrospectToken(t.Context(), httpClient, "http://example.com/introspect", tt.rawToken)

			// Assert
			if !tt.wantErr(t, err) {
				return
			}
			assert.Equal(t, tt.wantActive, got.Active)
		})
	}
}
