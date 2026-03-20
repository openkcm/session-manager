package credentials

import (
	"net/http"
)

type TransportCredentials interface {
	Transport() http.RoundTripper
}

type clientAuthRoundTripper struct {
	clientID     string
	clientSecret string
	next         http.RoundTripper
}

func (rt *clientAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	q := req.URL.Query()

	q.Set("client_id", rt.clientID)
	if rt.clientSecret != "" {
		q.Set("client_secret", rt.clientSecret)
	}
	req.URL.RawQuery = q.Encode()

	return rt.next.RoundTrip(req)
}
