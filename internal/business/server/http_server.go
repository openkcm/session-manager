package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	commonmiddleware "github.com/openkcm/common-sdk/pkg/middleware"
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
		cfg.SessionManager.AllowedRedirectBaseURLs,
	)
	strictHandler := openapi.NewStrictHandler(
		openAPIServer,
		[]openapi.StrictMiddlewareFunc{
			newTraceMiddleware(cfg),
		},
	)

	handler := openapi.Handler(strictHandler)
	handler = middleware.ResponseWriterMiddleware(handler)
	handler = commonmiddleware.SecurityHeadersMiddleware(handler, map[string]string{
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none';",
	})

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
		slogctx.Error(ctx, "failed to create a listener", "error", err, "network", network, "address", server.Addr)
		return fmt.Errorf("failed to create a listener: %w", err)
	}

	slogctx.Info(ctx, "A listener started", "address", listener.Addr().String())

	serverErr := make(chan error, 1)
	go func() {
		slogctx.Info(ctx, "Starting HTTP server", "address", listener.Addr().String())
		serverErr <- server.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer shutdownRelease()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to gracefully shutdown HTTP server: %w", err)
		}
		return processHTTPServerError(ctx, <-serverErr)
	case err := <-serverErr:
		return processHTTPServerError(ctx, err)
	}
}

func processHTTPServerError(ctx context.Context, err error) error {
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slogctx.Error(ctx, "Error serving HTTP endpoint", "error", err)
		return fmt.Errorf("HTTP server failed: %w", err)
	}
	slogctx.Info(ctx, "HTTP server stopped")
	return nil
}
