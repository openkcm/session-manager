package oidc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"
)

type Provider struct {
	IssuerURL string
	Blocked   bool
	JWKSURIs  []string
	Audiences []string
}

// RefreshToken refreshes the access token using the given refresh token for the tenant.
func (p *Provider) RefreshToken(ctx context.Context, refreshToken string, clientID string) (TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/token", bytes.NewBufferString(data.Encode()))
	if err != nil {
		return TokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TokenResponse{}, errors.New("token endpoint returned non-200 status")
	}

	var respData struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return TokenResponse{}, err
	}

	return TokenResponse{
		AccessToken:  respData.AccessToken,
		RefreshToken: respData.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(respData.ExpiresIn) * time.Second),
	}, nil
}

// TokenResponse represents the result of a token refresh operation.
type TokenResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}
