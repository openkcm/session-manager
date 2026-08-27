package session

import (
	"github.com/jellydator/ttlcache/v3"
	"github.com/zitadel/oidc/pkg/oidc"
)

// IntrospectionCache returns the internal introspectionCache for testing purposes.
func (s *Server) IntrospectionCache() *ttlcache.Cache[string, oidc.IntrospectionResponse] {
	return s.introspectionCache
}
