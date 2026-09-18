package session

import (
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	"github.com/openkcm/session-manager/internal/credentials"
)

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

func WithApplication(app commoncfg.Application) ManagerOption {
	return func(m *Manager) {
		m.application = app
	}
}
