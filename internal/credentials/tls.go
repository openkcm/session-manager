package credentials

import (
	"crypto/tls"
	"net/http"
)

type TLS struct {
	ClientID  string
	TLSConfig *tls.Config
}

func NewTLS(clientID string, tlsConfig *tls.Config) *TLS {
	return &TLS{
		ClientID:  clientID,
		TLSConfig: tlsConfig,
	}
}

func (c *TLS) Transport() http.RoundTripper {
	return &clientAuthRoundTripper{
		clientID: c.ClientID,
		next: &http.Transport{
			TLSClientConfig: c.TLSConfig,
		},
	}
}
