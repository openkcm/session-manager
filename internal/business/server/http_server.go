package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/openkcm/common-sdk/pkg/fingerprint"
	"github.com/samber/oops"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/middleware"
	"github.com/openkcm/session-manager/internal/openapi"
	"github.com/openkcm/session-manager/internal/session"
)

// createStatusServer creates an API http server using the given config
func createHTTPServer(_ context.Context, cfg *config.Config, sManager *session.Manager) (*http.Server, error) {
	openAPIServer := newOpenAPIServer(
		sManager,
		cfg.SessionManager.CSRFSecretParsed,
		cfg.SessionManager.SessionCookieTemplate.Name,
		cfg.SessionManager.CSRFCookieTemplate.Name,
	)
	strictHandler := openapi.NewStrictHandler(
		openAPIServer,
		[]openapi.StrictMiddlewareFunc{
			newTraceMiddleware(cfg),
		},
	)

	handler := fingerprint.FingerprintCtxMiddleware(openapi.Handler(strictHandler))
	handler = middleware.ResponseWriterMiddleware(handler)

	return &http.Server{
		Addr:    cfg.HTTP.Address,
		Handler: handler,
	}, nil
}

// StartHTTPServer starts the gRPC server using the given config.
func StartHTTPServer(ctx context.Context, cfg *config.Config, sManager *session.Manager) error {
	err := initMeters(ctx, cfg)
	if err != nil {
		return err
	}

	server, err := createHTTPServer(ctx, cfg, sManager)
	if err != nil {
		return fmt.Errorf("creating http server: %w", err)
	}

	slogctx.Info(ctx, "Starting a listener", "address", server.Addr)

	// Parse network if the address if provided in the format of network://address.
	// Otherwise use tcp network by default. Some integration tests are easier to implement
	// by binding a listener to a unix socket rather than a TCP port,
	// since we don't need to look up for a free port or scan /proc/net on Linux or call sysctl on macOS
	// to discover which port the process is bound to.
	network := "tcp"
	if idx := strings.IndexRune(server.Addr, ':'); idx != -1 && len(server.Addr) > idx+3 && server.Addr[idx:idx+3] == "://" {
		network = server.Addr[:idx]
		server.Addr = server.Addr[idx+3:]
	}

	listener, err := new(net.ListenConfig).Listen(ctx, network, server.Addr)
	if err != nil {
		return oops.In("HTTP Server").
			WithContext(ctx).
			Wrapf(err, "Failed to create a listener")
	}

	slogctx.Info(ctx, "A listener started", "address", listener.Addr().String())

	go func() {
		slogctx.Info(ctx, "Serving an HTTP server", "address", listener.Addr().String())
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slogctx.Error(ctx, "Failed to serve an HTTP server", "error", err)
		}

		slogctx.Info(ctx, "Stopped an HTTP server")
	}()

	<-ctx.Done()

	shutdownCtx, shutdownRelease := context.WithTimeout(ctx, cfg.HTTP.ShutdownTimeout)
	defer shutdownRelease()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		return oops.In("HTTP Server").
			WithContext(ctx).
			Wrapf(err, "Failed shutting down HTTP server")
	}

	slogctx.Info(ctx, "Completed graceful shutdown of HTTP server")

	return nil
}
