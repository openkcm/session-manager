package session_test

import (
	"testing"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/pkg/oidc"

	sessionmock "github.com/openkcm/session-manager/internal/session/mock"
	"github.com/openkcm/session-manager/modules/grpc/session"
	mocktrust "github.com/openkcm/session-manager/modules/oidctrust/mocks"
)

func TestIntrospectionCache_DisableTouchOnHit(t *testing.T) {
	t.Run("Server introspectionCache does not extend TTL on Get", func(t *testing.T) {
		ctx := t.Context()

		// Create a real Server to get its actual cache configuration
		sessionRepo := sessionmock.NewInMemRepository()
		trustRepo := mocktrust.NewInMemRepository()
		idleSessionTimeout := 90 * time.Minute

		// newTrust is imported via go:linkname in import_test.go
		server := session.NewServer(ctx, sessionRepo, newTrust(trustRepo), idleSessionTimeout)
		require.NotNil(t, server)

		// Get the actual cache from the Server
		cache := server.IntrospectionCache()
		require.NotNil(t, cache)

		// Set an item with a short TTL for testing
		key := "hashed-token-key"
		value := oidc.NewIntrospectionResponse()
		value.SetActive(true)
		cache.Set(key, value, 100*time.Millisecond)

		// Verify item is retrievable
		item := cache.Get(key)
		require.NotNil(t, item)
		assert.Equal(t, value, item.Value())

		// Record the initial expiration time
		initialExpiry := item.ExpiresAt()

		// Wait a bit and access the item
		time.Sleep(30 * time.Millisecond)
		item = cache.Get(key)
		require.NotNil(t, item, "item should still exist")

		// Verify expiration time has NOT changed (touch disabled)
		assert.Equal(t, initialExpiry, item.ExpiresAt(),
			"expiration time should not change on Get when WithDisableTouchOnHit is configured")

		// Access again after more time
		time.Sleep(30 * time.Millisecond)
		item = cache.Get(key)
		require.NotNil(t, item, "item should still exist")
		assert.Equal(t, initialExpiry, item.ExpiresAt(),
			"expiration time should remain unchanged after multiple Gets")

		// Wait for item to expire
		time.Sleep(50 * time.Millisecond)

		// Verify item has expired
		item = cache.Get(key)
		assert.Nil(t, item, "item should have expired after TTL")
	})

	t.Run("contrast: default cache behavior extends TTL on Get", func(t *testing.T) {
		// This test demonstrates the default behavior that we're avoiding
		// by using WithDisableTouchOnHit in production code
		const ttl = 100 * time.Millisecond

		// Create cache WITHOUT WithDisableTouchOnHit (default behavior)
		cacheWithTouch := ttlcache.New(
			ttlcache.WithTTL[string, oidc.IntrospectionResponse](ttl),
		)
		go cacheWithTouch.Start()
		defer cacheWithTouch.Stop()

		key := "hashed-token-key"
		value := oidc.NewIntrospectionResponse()
		value.SetActive(true)
		cacheWithTouch.Set(key, value, ttlcache.DefaultTTL)

		item := cacheWithTouch.Get(key)
		require.NotNil(t, item)
		initialExpiry := item.ExpiresAt()

		// Wait and access
		time.Sleep(30 * time.Millisecond)
		item = cacheWithTouch.Get(key)
		require.NotNil(t, item)

		// With default behavior, expiry should be extended
		assert.True(t, item.ExpiresAt().After(initialExpiry),
			"with default touch behavior, expiration should extend on Get")
	})
}
