package session

import "github.com/openkcm/session-manager/internal/credentials"

type ManagerOption func(*Manager)

func WithAllowHttpScheme(allowHttpScheme bool) ManagerOption {
	return func(m *Manager) {
		m.allowHttpScheme = allowHttpScheme
	}
}

func WithCredentialsProvider(p credentials.Provider) ManagerOption {
	return func(m *Manager) {
		m.cProvider = p
	}
}
