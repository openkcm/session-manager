package session

import "time"

type Session struct {
	ID          string
	TenantID    string
	Fingerprint string
	Token       string
	Expiry      time.Time
}

type State struct {
	ID           string
	TenantID     string
	Fingerprint  string
	PKCEVerifier string
	RequestURI   string
	Expiry       time.Time
}
