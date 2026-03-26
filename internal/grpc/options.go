package grpc

import "github.com/openkcm/session-manager/internal/credentials"

type SessionServerOption func(*SessionServer)

func WithQueryParametersIntrospect(params []string) SessionServerOption {
	return func(s *SessionServer) {
		s.queryParametersIntrospect = params
	}
}

func WithAllowHttpScheme(allow bool) SessionServerOption {
	return func(s *SessionServer) {
		s.allowHttpScheme = allow
	}
}

func WithTransportCredentials(b credentials.Builder) SessionServerOption {
	return func(s *SessionServer) {
		s.newCreds = b
	}
}
