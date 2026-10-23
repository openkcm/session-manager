// Package sessionwiring centralises the construction of the long-lived
// session.Manager that the HTTP API server and the housekeeper subcommand
// share. The Valkey-backed session.Repository and OAuth2 credentials.Provider
// session manager requires come from the module registry,
// loaded by business.Main before this is invoked.
package sessionwiring

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
	"github.com/openkcm/session-manager/internal/session"
)

func makeAuditLogger(conf *commoncfg.Audit) (*otlpaudit.QueuedAuditLogger, error) {
	if conf.Endpoint == "" {
		//nolint:nilnil
		return nil, nil
	}

	auditLogger, err := otlpaudit.NewLogger(conf)
	if err != nil {
		return nil, fmt.Errorf("creating audit logger: %w", err)
	}

	return otlpaudit.NewQueuedLogger(auditLogger, otlpaudit.QueuedOptions{
		OnError: logAuditDeliveryError,
	}), nil
}

// logAuditDeliveryError is the QueuedAuditLogger OnError handler: it reports
// events dropped after the background workers exhausted their retries.
func logAuditDeliveryError(err error) {
	slog.Error("audit event delivery failed", "error", err)
}

// closeAuditLogger drains a queued audit logger on shutdown, giving in-flight
// and buffered events a bounded window to flush. A nil logger (auditing
// disabled because no endpoint was configured) is a no-op.
func closeAuditLogger(auditLogger *otlpaudit.QueuedAuditLogger) {
	if auditLogger == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := auditLogger.Close(ctx); err != nil {
		slog.Error("audit logger did not drain cleanly on shutdown", "error", err)
	}
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

	auditLogger, err := makeAuditLogger(&cfg.Audit)
	if err != nil {
		return nil, nil, fmt.Errorf("making audit logger: %w", err)
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
		session.WithGlobalUserIDClaimKeys(cfg.SessionManager.GlobalUserIDClaimKeys),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	return sessManager, func() { closeAuditLogger(auditLogger) }, nil
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
