package credentials

import "net/http"

type ClientSecretPost struct {
	ClientID     string
	ClientSecret string
}

// NewClientSecretPost returns a credentials implementation that
// follows the client authentication method 'client_secret_post', defined by the OIDC specification:
// https://openid.net/specs/openid-connect-core-1_0.html#ClientAuthentication
func NewClientSecretPost(clientID, clientSecret string) *ClientSecretPost {
	return &ClientSecretPost{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
}

func (c *ClientSecretPost) Transport() http.RoundTripper {
	return &clientAuthRoundTripper{
		clientID:     c.ClientID,
		clientSecret: c.ClientSecret,
		next:         http.DefaultTransport,
	}
}
