package oidc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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

// RefreshToken refreshes the access token using the given refresh token for the tenant.
func (s *Service) RefreshToken(ctx context.Context, issuerUrl string, refreshToken string, clientID string) (TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", clientID)

	tokenEndpoint := strings.TrimRight(issuerUrl, "/") + "/token"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, bytes.NewBufferString(data.Encode()))
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
