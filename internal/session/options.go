package session

import "github.com/openkcm/session-manager/internal/credentials"

type ManagerOption func(*Manager)

func WithAllowHttpScheme(allowHttpScheme bool) ManagerOption {
	return func(m *Manager) {
		m.allowHttpScheme = allowHttpScheme
	}
}

type CredentialsBuilder func(clientID string) credentials.TransportCredentials

func WithTransportCredentials(b CredentialsBuilder) ManagerOption {
	return func(m *Manager) {
		m.newCreds = b
	}
}
