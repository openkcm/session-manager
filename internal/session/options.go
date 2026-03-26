package session

import "github.com/openkcm/session-manager/internal/credentials"

type ManagerOption func(*Manager)

func WithAllowHttpScheme(allowHttpScheme bool) ManagerOption {
	return func(m *Manager) {
		m.allowHttpScheme = allowHttpScheme
	}
}

func WithTransportCredentials(b credentials.Builder) ManagerOption {
	return func(m *Manager) {
		m.newCreds = b
	}
}
