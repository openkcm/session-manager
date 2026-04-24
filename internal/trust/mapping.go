package trust

type OIDCMapping struct {
	IssuerURL  string
	Blocked    bool
	JWKSURI    string
	Audiences  []string
	Properties map[string]string

	// ClientID is a mandatory property used for authentication.
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
