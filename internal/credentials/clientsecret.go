package credentials

import "net/http"

type ClientSecret struct {
	ClientID     string
	ClientSecret string
}

func NewClientSecret(clientID, clientSecret string) *ClientSecret {
	return &ClientSecret{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
}

func (c *ClientSecret) Transport() http.RoundTripper {
	return &clientAuthRoundTripper{
		clientID:     c.ClientID,
		clientSecret: c.ClientSecret,
		next:         http.DefaultTransport,
	}
}
