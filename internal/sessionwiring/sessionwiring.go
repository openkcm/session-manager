// Package sessionwiring centralises the construction of the long-lived
// session.Manager that the HTTP API server and the housekeeper subcommand
// share. The Valkey-backed session.Repository and OAuth2 credentials.Provider
// session manager requires come from the module registry,
// loaded by business.Main before this is invoked.
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

	auditLogger, err := otlpaudit.NewLogger(&cfg.Audit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	cProvider, err := sessionmanager.GetModuleAs[credentials.Provider](ctx, cfg.Credentials.Module())
	if err != nil {
		return nil, nil, fmt.Errorf("getting credentials provider module %q: %w", cfg.Credentials.Module(), err)
	}

	sessManager, err := session.NewManager(ctx,
		&cfg.SessionManager,
		trust,
		repo,
		auditLogger,
		session.WithAllowHttpScheme(cfg.SessionManager.AllowHttpScheme),
		session.WithCredentialsProvider(cProvider),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	return sessManager, func() {}, nil
}

// SessionRepository resolves the session repository module loaded under the
// ID configured in cfg.ValKey.Module() and returns its session.Repository.
func SessionRepository(ctx *sessionmanager.Context, cfg *config.Config) (session.Repository, error) {
	repo, err := sessionmanager.GetModuleAs[session.Repository](ctx, cfg.ValKey.Module())
	if err != nil {
		return nil, fmt.Errorf("getting session-store module %q: %w", cfg.ValKey.Module(), err)
	}
	return repo, nil
}

// Reference to context.Context to keep imports stable for callers using
// (ctx context.Context) signatures.
var _ context.Context = (*sessionmanager.Context)(nil)
