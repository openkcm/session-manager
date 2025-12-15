package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/serviceerr"
)

type Provider struct {
	IssuerURL  string
	Blocked    bool
	JWKSURI    string
	Audiences  []string
	Properties map[string]string

	QueryParametersIntrospect []string
}

func (p *Provider) GetOpenIDConfig(ctx context.Context, httpClient *http.Client) (Configuration, error) {
	const wellKnownOpenIDConfigPath = "/.well-known/openid-configuration"

	u, err := url.JoinPath(p.IssuerURL, wellKnownOpenIDConfigPath)
	if err != nil {
		return Configuration{}, fmt.Errorf("building path to the well-known openid-config endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Configuration{}, fmt.Errorf("creating an HTTP request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return Configuration{}, fmt.Errorf("doing an HTTP request: %w", err)
	}

	var conf Configuration
	err = json.NewDecoder(resp.Body).Decode(&conf)
	if err != nil {
		return Configuration{}, fmt.Errorf("decoding a well-known openid config: %w", err)
	}

	// Validate the configuration
	if conf.Issuer != p.IssuerURL {
		return Configuration{}, serviceerr.ErrInvalidOIDCProvider
	}

	return conf, nil
}

func (p *Provider) IntrospectToken(ctx context.Context, httpClient *http.Client, endpoint, token string) (Introspection, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return Introspection{}, fmt.Errorf("creating http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	q := req.URL.Query()
	q.Set("token", token)
	for _, parameter := range p.QueryParametersIntrospect {
		value, ok := p.Properties[parameter]
		if !ok {
			return Introspection{}, fmt.Errorf("missing introspect parameter: %s", parameter)
		}
		q.Set(parameter, value)
	}

	req.URL.RawQuery = q.Encode()

	resp, err := httpClient.Do(req)
	if err != nil {
		return Introspection{}, fmt.Errorf("executing http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Introspection{}, fmt.Errorf("reading introspection response body: %w", err)
	}

	var result Introspection
	err = json.Unmarshal(body, &result)
	if err != nil {
		slogctx.Error(ctx, "Failed to unmarshal introspection response", "body", string(body), "error", err)
		return Introspection{}, fmt.Errorf("decoding introspection response: %w", err)
	}

	return result, nil
}

type Introspection struct {
	Active bool `json:"active"`
	// Error response fields e.g. bad credentials
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}
