package trust

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
		if ok {
			params[parameter] = value
		}
	}
	return params
}
