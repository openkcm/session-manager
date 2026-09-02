package session

import "github.com/openkcm/session-manager/internal/credentials"

type Option func(*Server)

func WithAllowHttpScheme(allow bool) Option {
	return func(s *Server) {
		s.allowHttpScheme = allow
	}
}

func WithCredentialsProvider(provider credentials.Provider) Option {
	return func(s *Server) {
		s.cProvider = provider
	}
}
