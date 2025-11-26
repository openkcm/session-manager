package oidc

type Provider struct {
	IssuerURL  string
	Blocked    bool
	JWKSURI    string
	Audiences  []string
	Properties map[string]string
}
