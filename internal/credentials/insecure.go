package credentials

import "net/http"

type Insecure struct {
	clientID string
}

func NewInsecure(clientID string) TransportCredentials {
	return &Insecure{clientID: clientID}
}

func (c *Insecure) Transport() http.RoundTripper {
	return &clientAuthRoundTripper{
		clientID: c.clientID,
		next:     http.DefaultTransport,
	}
}
