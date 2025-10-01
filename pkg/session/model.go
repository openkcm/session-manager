package session

import "time"

// State represents the state of an authentication process according to the OIDC spec.
// It is used to align the auth request with the callback and to store necessary
// information for completing the authentication process.
type State struct {
	ID           string    // State ID to align the auth request with the callback
	TenantID     string    // Tenant ID for which the login is done
	Fingerprint  string    // Fingerprint to bind the login to a specific client
	PKCEVerifier string    // PKCE verifier to validate the PKCE challenge
	RequestURI   string    // Request URI for the eventual redirect
	Expiry       time.Time // Expiry time of the login process
}

// Session represents a user session in our system.
type Session struct {
	ID           string    // Session ID in our system
	TenantID     string    // Tenant ID for which the session is created
	Fingerprint  string    // Fingerprint to bind the session to a specific client
	CSRFToken    string    // CSRF token to prevent CSRF attacks
	Issuer       string    // Issuer of the OIDC tokens
	Claims       string    // JSON string of claims from the ID token
	AccessToken  string    // Access token from the identity provider
	RefreshToken string    // Refresh token from the identity provider
	Expiry       time.Time // Expiry time of the session
}
