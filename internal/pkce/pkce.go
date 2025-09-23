package pkce

type PKCE struct {
	Verifier  string
	Challenge string
	Method    string
}
