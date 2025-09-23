package oidc

type Provider struct {
	IssuerURL string
	Blocked   bool
	JWKSURIs  []string
	Audiences []string
}
