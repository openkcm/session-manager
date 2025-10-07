package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type TokenSet struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type Service struct {
	repository ProviderRepository
}

func NewService(repo ProviderRepository) *Service {
	return &Service{
		repository: repo,
	}
}

func (s *Service) GetProvider(ctx context.Context, issuer string) (Provider, error) {
	provider, err := s.repository.Get(ctx, issuer)
	if err != nil {
		return Provider{}, fmt.Errorf("getting provider by issuer URL: %w", err)
	}

	return provider, nil
}

func (s *Service) ApplyMapping(ctx context.Context, tenantID string, provider Provider) error {
	_, err := s.repository.GetForTenant(ctx, tenantID)
	if err != nil {
		err = s.repository.Create(ctx, tenantID, provider)
		if err != nil {
			return fmt.Errorf("creating provider for tenant: %w", err)
		}
	} else {
		err = s.repository.Update(ctx, tenantID, provider)
		if err != nil {
			return fmt.Errorf("updating provider for tenant: %w", err)
		}
	}

	return nil
}

func (s *Service) RemoveMapping(ctx context.Context, tenantID string) error {
	provider, err := s.repository.GetForTenant(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("getting provider for tenant: %w", err)
	}
	err = s.repository.Delete(ctx, tenantID, provider)
	if err != nil {
		return fmt.Errorf("deleting provider for tenant: %w", err)
	}

	return nil
}

func (p *Provider) Exchange(ctx context.Context, code, codeVerifier, redirectURI string, clientID string) (*TokenSet, error) {
	tokenEndpoint := p.IssuerURL + "/token"

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(data.Encode()))
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
