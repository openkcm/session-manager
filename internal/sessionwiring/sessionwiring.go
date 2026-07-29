// Package sessionwiring centralises the construction of the long-lived
// session.Manager that the HTTP API server and the housekeeper subcommand
// share. The Valkey-backed session.Repository and OAuth2 credentials.Builder
// it needs are no longer built here; both come from the module registry,
// loaded by business.Main (or the housekeeper subcommand) before this is
// invoked.
package sessionwiring

import (
	"context"
	"fmt"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
	"github.com/openkcm/session-manager/internal/session"
)

// credentialsBuilder is the interface satisfied by a credentials module
// (e.g. credentials.module.oauth2). Defined locally so this package does not
// need to import the credentials module.
type credentialsBuilder interface {
	Builder() credentials.Builder
}

// InitSessionManager builds a session.Manager from the supplied config and
// trust module, using session repository and credential modules already loaded
// in ctx. The returned closeFn is a no-op kept for API compatibility — the
// underlying valkey client is owned by the sessionstore module and closed by
// the framework's reverse-load-order shutdown.
func InitSessionManager(ctx *sessionmanager.Context, cfg *config.Config, trust sessionmanager.Trust) (_ *session.Manager, closeFn func(), _ error) {
	repo, err := SessionRepository(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("getting session repository: %w", err)
	}

	credsBuilder, err := CredsBuilder(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("getting credentials builder: %w", err)
	}

	auditLogger, err := otlpaudit.NewLogger(&cfg.Audit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	sessManager, err := session.NewManager(ctx,
		&cfg.SessionManager,
		trust,
		repo,
		auditLogger,
		session.WithTransportCredentials(credsBuilder),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	return sessManager, func() {}, nil
}

// SessionRepository resolves the session repository module loaded under the
// ID configured in cfg.ValKey.Module() and returns its session.Repository.
func SessionRepository(ctx *sessionmanager.Context, cfg *config.Config) (session.Repository, error) {
	mod, err := ctx.GetModule(cfg.ValKey.Module())
	if err != nil {
		return nil, fmt.Errorf("getting session-store module %q: %w", cfg.ValKey.Module(), err)
	}
	repo, ok := mod.(session.Repository)
	if !ok {
		return nil, fmt.Errorf("module %q does not implement session.Repository", cfg.ValKey.Module())
	}
	return repo, nil
}

// CredsBuilder resolves the credentials module loaded under the ID configured
// in cfg.Credentials.Module() and returns its credentials.Builder.
func CredsBuilder(ctx *sessionmanager.Context, cfg *config.Config) (credentials.Builder, error) {
	mod, err := ctx.GetModule(cfg.Credentials.Module())
	if err != nil {
		return nil, fmt.Errorf("getting credentials module %q: %w", cfg.Credentials.Module(), err)
	}
	cb, ok := mod.(credentialsBuilder)
	if !ok {
		return nil, fmt.Errorf("module %q does not expose Builder()", cfg.Credentials.Module())
	}
	return cb.Builder(), nil
}

// Reference to context.Context to keep imports stable for callers using
// (ctx context.Context) signatures.
var _ context.Context = (*sessionmanager.Context)(nil)
