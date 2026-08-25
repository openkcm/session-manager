package session_test

import (
	"testing"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/pkg/oidc"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/session"
	sessionmock "github.com/openkcm/session-manager/internal/session/mock"
	mocktrust "github.com/openkcm/session-manager/modules/oidctrust/mocks"
)

func TestWKOCCache_DisableTouchOnHit(t *testing.T) {
	t.Run("Manager wkocCache does not extend TTL on Get", func(t *testing.T) {
		ctx := t.Context()

		// Create a real Manager to get its actual cache configuration
		sessionRepo := sessionmock.NewInMemRepository()
		trustRepo := mocktrust.NewInMemRepository()

		cfg := &config.SessionManager{
			CallbackURL:      "http://localhost/callback",
			SessionDuration:  30 * time.Minute,
			CSRFSecretParsed: []byte("12345678901234567890123456789012"),
		}

		// newTrust is imported via go:linkname in import_test.go
		manager, err := session.NewManager(ctx, cfg, newTrust(trustRepo), sessionRepo, nil)
		require.NoError(t, err)

		// Get the actual cache from the Manager
		cache := manager.WKOCCache()
		require.NotNil(t, cache)

		// Set an item with a short TTL for testing
		key := "test-issuer-key"
		value := &oidc.DiscoveryConfiguration{Issuer: "https://test.example.com"}
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
			ttlcache.WithTTL[string, *oidc.DiscoveryConfiguration](ttl),
		)
		go cacheWithTouch.Start()
		defer cacheWithTouch.Stop()

		key := "test-issuer"
		value := &oidc.DiscoveryConfiguration{Issuer: "https://test.example.com"}
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
