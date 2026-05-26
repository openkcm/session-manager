// Package sessionwiring centralises the construction of long-lived
// session-manager dependencies (valkey client, credentials builder, the
// session.Manager itself) so callers in cmd/, internal/business, and apps
// configured via the apps: lifecycle loop can build them identically.
package sessionwiring

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/valkey-io/valkey-go"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
	"github.com/openkcm/session-manager/internal/session"
	sessionvalkey "github.com/openkcm/session-manager/internal/session/valkey"
)

const (
	insecure         = "insecure"
	mtls             = "mtls"
	clientSecret     = "client_secret" // Alias for clientSecretPost.
	clientSecretPost = "client_secret_post"
)

// InitSessionManager builds a session.Manager from the supplied config and
// trust module. The returned closeFn must be invoked once the manager is no
// longer in use to release the underlying valkey client.
func InitSessionManager(ctx context.Context, cfg *config.Config, trust sessionmanager.Trust) (_ *session.Manager, closeFn func(), _ error) {
	valkeyClient, err := ValkeyClient(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create valkey client: %w", err)
	}
	sessionRepo := sessionvalkey.NewRepository(valkeyClient, cfg.ValKey.Prefix)

	credsBuilder, err := CredsBuilder(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load http client: %w", err)
	}

	auditLogger, err := otlpaudit.NewLogger(&cfg.Audit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	sessManager, err := session.NewManager(ctx,
		&cfg.SessionManager,
		trust,
		sessionRepo,
		auditLogger,
		session.WithTransportCredentials(credsBuilder),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	return sessManager, valkeyClient.Close, nil
}

// ValkeyClient creates a valkey client from the valkey-related fields on cfg.
func ValkeyClient(cfg *config.Config) (valkey.Client, error) {
	valkeyHost, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to load valkey host: %w", err)
	}

	valkeyUsername, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.User)
	if err != nil {
		return nil, fmt.Errorf("failed to load valkey username: %w", err)
	}

	valkeyPassword, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to load valkey password: %w", err)
	}

	valkeyOpts := valkey.ClientOption{
		InitAddress: []string{string(valkeyHost)},
		Username:    string(valkeyUsername),
		Password:    string(valkeyPassword),
	}

	if cfg.ValKey.SecretRef.Type == commoncfg.MTLSSecretType {
		tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.ValKey.SecretRef.MTLS)
		if err != nil {
			return nil, fmt.Errorf("failed to load valkey mTLS config from secret ref: %w", err)
		}
		valkeyOpts.TLSConfig = tlsConfig
	}

	valkeyClient, err := valkey.NewClient(valkeyOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new valkey client: %w", err)
	}
	return valkeyClient, nil
}

// CredsBuilder returns a credentials.Builder that matches the configured
// client-auth strategy.
func CredsBuilder(cfg *config.Config) (credentials.Builder, error) {
	switch cfg.SessionManager.ClientAuth.Type {
	case mtls:
		tlsConfig, err := commoncfg.LoadMTLSConfig(cfg.SessionManager.ClientAuth.MTLS)
		if err != nil {
			return nil, fmt.Errorf("failed to load mTLS config: %w", err)
		}

		return func(clientID string) credentials.TransportCredentials { return credentials.NewTLS(clientID, tlsConfig) }, nil
	case clientSecretPost, clientSecret:
		secret, err := commoncfg.LoadValueFromSourceRef(cfg.SessionManager.ClientAuth.ClientSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to load client secret: %w", err)
		}

		return func(clientID string) credentials.TransportCredentials {
			return credentials.NewClientSecretPost(clientID, string(secret))
		}, nil
	case insecure:
		slog.Warn("insecure credentials are used. Do not use this in production")
		return func(clientID string) credentials.TransportCredentials { return credentials.NewInsecure(clientID) }, nil
	default:
		return nil, errors.New("unknown Client Auth type")
	}
}
