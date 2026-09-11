package session

import (
	"github.com/jellydator/ttlcache/v3"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// IntrospectionCache returns the internal introspectionCache for testing purposes.
func (s *Server) IntrospectionCache() *ttlcache.Cache[string, oidc.IntrospectionResponse] {
	return s.introspectionCache
}

// ToStringSlice exposes the unexported toStringSlice for testing.
var ToStringSlice = toStringSlice
