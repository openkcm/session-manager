package session

import "github.com/openkcm/session-manager/internal/credentials"

type Option func(*Server)

func WithQueryParametersIntrospect(params []string) Option {
	return func(s *Server) {
		s.queryParametersIntrospect = params
	}
}

func WithAllowHttpScheme(allow bool) Option {
	return func(s *Server) {
		s.allowHttpScheme = allow
	}
}

func WithTransportCredentials(b credentials.Builder) Option {
	return func(s *Server) {
		s.newCreds = b
	}
}
