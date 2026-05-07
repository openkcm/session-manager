package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/openkcm/common-sdk/pkg/commongrpc"

	sessionv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/session/v1"
	trustmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/trustmapping/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/grpc"
)

func StartGRPCServer(ctx context.Context, cfg *config.Config,
	oidcmappingsrv *grpc.TrustMappingServer,
	sessionsrv *grpc.SessionServer,
) error {
	grpcServer := commongrpc.NewServer(ctx, &cfg.GRPC.GRPCServer)

	// Register OIDC mapping server for the regional tenant manager
	trustmappingv1.RegisterServiceServer(grpcServer, oidcmappingsrv)
	// Register Session server for ExtAuthZ
	sessionv1.RegisterServiceServer(grpcServer, sessionsrv)

	slogctx.Info(ctx, "Starting a listener", "address", cfg.GRPC.Address)

	listener, err := new(net.ListenConfig).Listen(ctx, "tcp", cfg.GRPC.Address)
	if err != nil {
		return fmt.Errorf("failed to create a listener: %w", err)
	}

	slogctx.Info(ctx, "A listener started", "address", listener.Addr().String())

	serverErr := make(chan error, 1)
	go func() {
		slogctx.Info(ctx, "Starting gRPC server", "address", listener.Addr().String())
		serverErr <- grpcServer.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		shutdownCh := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(shutdownCh)
		}()

		select {
		case <-shutdownCh:
			slogctx.Info(ctx, "Completed graceful shutdown of the gRPC server")
		case <-time.After(cfg.GRPC.ShutdownTimeout):
			slogctx.Warn(ctx, "Failed to complete graceful shutdown", "timeout", cfg.GRPC.ShutdownTimeout.String())
			grpcServer.Stop()
		}
		return processGRPCServerError(ctx, <-serverErr)
	case err := <-serverErr:
		return processGRPCServerError(ctx, err)
	}
}

func processGRPCServerError(ctx context.Context, err error) error {
	if err != nil {
		slogctx.Error(ctx, "Error serving gRPC endpoint", "error", err)
		return fmt.Errorf("gRPC server failed: %w", err)
	}
	slogctx.Info(ctx, "gRPC server stopped")
	return nil
}
