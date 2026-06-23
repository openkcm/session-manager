package config

import (
	"context"

	sessionmanager "github.com/openkcm/session-manager"
)

// configCtxKey is a private type so config attached via WithContext can only
// be retrieved by callers that share this package.
type configCtxKey struct{}

// WithContext returns a sessionmanager.Context that carries cfg. Apps'
// Provision methods retrieve it via FromContext when they need top-level
// configuration that is not part of their own per-app config block (e.g.
// valkey credentials, audit endpoints).
func WithContext(ctx *sessionmanager.Context, cfg *Config) *sessionmanager.Context {
	return ctx.WithValue(configCtxKey{}, cfg)
}

// FromContext returns the *Config previously attached via WithContext. The
// boolean is false when no config has been attached.
func FromContext(ctx context.Context) (*Config, bool) {
	cfg, ok := ctx.Value(configCtxKey{}).(*Config)
	return cfg, ok
}
