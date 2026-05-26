package business

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	slogctx "github.com/veqryn/slog-context"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/business/server"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/grpc"
	sessionvalkey "github.com/openkcm/session-manager/internal/session/valkey"
	"github.com/openkcm/session-manager/internal/sessionwiring"
)

// Main starts both API servers
func Main(ctx context.Context, cfg *config.Config) error {
	c, cancelCause := sessionmanager.NewContext(ctx)
	defer cancelCause(nil)

	c = config.WithContext(c, cfg)

	if _, err := c.LoadModule(&cfg.Database); err != nil {
		return fmt.Errorf("loading database module: %w", err)
	}

	if _, err := c.LoadModule(&cfg.Trust); err != nil {
		return fmt.Errorf("loading trust module: %w", err)
	}

	stopApps, err := startApps(c, cfg)
	if err != nil {
		return fmt.Errorf("starting apps: %w", err)
	}

	// errChan is used to capture the first error and shutdown the servers.
	errChan := make(chan error, 1)

	// wg is used to wait for all servers to shutdown.
	var wg sync.WaitGroup

	// start public HTTP REST API server
	wg.Go(func() {
		errChan <- publicMain(c, cfg)
	})

	// start internal gRPC API server
	wg.Go(func() {
		errChan <- internalMain(c, cfg)
	})

	// wait for any error to initiate the shutdown
	err = <-errChan
	if err != nil {
		slogctx.Error(ctx, "Shutting down servers", "error", err)
	}

	stopErr := stopApps()
	cancelCause(err)

	// wait for all servers to shutdown
	wg.Wait()

	return errors.Join(err, stopErr)
}

// publicMain starts the HTTP REST public API server.
func publicMain(ctx *sessionmanager.Context, cfg *config.Config) error {
	csrfSecret, err := commoncfg.LoadValueFromSourceRef(cfg.SessionManager.CSRFSecret)
	if err != nil {
		return fmt.Errorf("loading csrf token from source ref: %w", err)
	}
	if len(csrfSecret) < 32 {
		return errors.New("CSRF secret must be at least 32 bytes")
	}

	cfg.SessionManager.CSRFSecretParsed = csrfSecret

	trustMod, err := ctx.GetModule(cfg.Trust.Module())
	if err != nil {
		return fmt.Errorf("getting trust module: %w", err)
	}

	//nolint:forcetypeassert
	trust := trustMod.(sessionmanager.Trust)

	sessionManager, closeFn, err := sessionwiring.InitSessionManager(ctx, cfg, trust)
	if err != nil {
		return fmt.Errorf("failed to initialise the session manager: %w", err)
	}

	defer closeFn()

	return server.StartHTTPServer(ctx, cfg, sessionManager)
}

// internalMain starts the gRPC private API server.
func internalMain(ctx *sessionmanager.Context, cfg *config.Config) error {
	// Create session repository
	valkeyClient, err := sessionwiring.ValkeyClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create valkey client: %w", err)
	}
	defer valkeyClient.Close()
	sessionRepo := sessionvalkey.NewRepository(valkeyClient, cfg.ValKey.Prefix)

	credsBuilder, err := sessionwiring.CredsBuilder(cfg)
	if err != nil {
		return fmt.Errorf("failed to create a credentials builder: %w", err)
	}

	trustMod, err := ctx.GetModule(cfg.Trust.Module())
	if err != nil {
		return fmt.Errorf("getting trust module: %w", err)
	}

	//nolint:forcetypeassert
	trust := trustMod.(sessionmanager.Trust)

	// Initialize the gRPC servers.
	trustsrv := grpc.NewTrustMappingServer(trust)
	sessionsrv := grpc.NewSessionServer(ctx,
		sessionRepo,
		trust,
		cfg.SessionManager.IdleSessionTimeout,
		cfg.SessionManager.ClientAuth.ClientID,
		grpc.WithTransportCredentials(credsBuilder),
	)

	return server.StartGRPCServer(ctx, cfg, trustsrv, sessionsrv)
}
