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

	// ClientID is a client_id property used for authentication.
	// It is an optional value for the trust config. If the trust's client id is not specified,
	// the application-global client id is used.
	ClientID string
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
