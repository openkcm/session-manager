package credentials

import "net/http"

type Default struct{}

func NewDefault() TransportCredentials {
	return &Default{}
}

func (c *Default) Transport() http.RoundTripper {
	return http.DefaultTransport
}
