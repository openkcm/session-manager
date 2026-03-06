package trust

import (
	"context"

	slogctx "github.com/veqryn/slog-context"
)

type OIDCMapping struct {
	IssuerURL  string
	Blocked    bool
	JWKSURI    string
	Audiences  []string
	Properties map[string]string

	QueryParametersIntrospect []string
}

func (p *OIDCMapping) GetIntrospectParameters(keys []string) map[string]string {
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
