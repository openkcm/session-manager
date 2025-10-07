package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Provider struct {
	IssuerURL string
	Blocked   bool
	JWKSURIs  []string
	Audiences []string
}

type TokenSet struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

func (p *Provider) Exchange(ctx context.Context, code, codeVerifier, redirectURI string, clientID string) (*TokenSet, error) {
	tokenEndpoint := p.IssuerURL + "/token"

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", clientID)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var tokenSet TokenSet
	if err := json.NewDecoder(resp.Body).Decode(&tokenSet); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &tokenSet, nil
}
