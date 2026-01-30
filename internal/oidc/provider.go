package oidc

import (
	"context"

	slogctx "github.com/veqryn/slog-context"
)

type Provider struct {
	IssuerURL  string
	Blocked    bool
	JWKSURI    string
	Audiences  []string
	Properties map[string]string

	QueryParametersIntrospect []string
}

func (p *Provider) GetIntrospectParameters(keys []string) map[string]string {
	params := make(map[string]string, len(keys))
	for _, parameter := range keys {
		value, ok := p.Properties[parameter]
		if !ok {
			slogctx.Error(context.Background(), "Missing introspect parameter", "parameter", parameter)
			continue
		}
		params[parameter] = value
	}
	return params
}
