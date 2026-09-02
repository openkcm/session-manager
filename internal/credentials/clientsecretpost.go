package credentials

import (
	"github.com/zitadel/oidc/pkg/client/rp"
	"github.com/zitadel/oidc/pkg/client/rs"

	httphelper "github.com/zitadel/oidc/pkg/http"
)

// NewClientSecretPostRS returns an rs.ResourceServer that follows the
// 'client_secret_post' client authentication method defined by the OIDC
// specification: the client_id and client_secret are sent in the request body.
// https://openid.net/specs/openid-connect-core-1_0.html#ClientAuthentication
func NewClientSecretPostRS(issuer, clientID, clientSecret string) (rs.ResourceServer, error) {
	httpClient := *httphelper.DefaultHTTPClient // Create a copy
	httpClient.Transport = debugTransport(httpClient.Transport)
	return newResourceServer(issuer, &httpClient, clientSecretPostAuth(clientID, clientSecret))
}

// NewClientSecretPostRP returns an rp.RelyingParty that follows the
// 'client_secret_post' client authentication method defined by the OIDC
// specification: the client_id and client_secret are sent in the request body.
// https://openid.net/specs/openid-connect-core-1_0.html#ClientAuthentication
func NewClientSecretPostRP(issuer, clientID, clientSecret, redirectURI string) (rp.RelyingParty, error) {
	httpClient := *httphelper.DefaultHTTPClient // Create a copy
	httpClient.Transport = debugTransport(httpClient.Transport)
	return rp.NewRelyingPartyOIDC(issuer, clientID, clientSecret, redirectURI, nil, rp.WithHTTPClient(&httpClient))
}
