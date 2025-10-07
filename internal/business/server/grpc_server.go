package server

import (
	"context"
	"net"

	"github.com/openkcm/common-sdk/pkg/commongrpc"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/samber/oops"

	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	oidcproviderv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcprovider/v1"
	slogctx "github.com/veqryn/slog-context"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/grpc"
)

func StartGRPCServer(ctx context.Context, cfg *config.Config,
	oidcprovidersrv *grpc.OIDCProviderServer,
	oidcmappingsrv *grpc.OIDCMappingServer,
) error {
	grpcServer := commongrpc.NewServer(ctx, &cfg.GRPC.GRPCServer)

	// Register health server
	healthpb.RegisterHealthServer(grpcServer, &health.GRPCServer{})
	// Register OIDC provider server for ExtAuthZ
	oidcproviderv1.RegisterOIDCProviderServer(grpcServer, oidcprovidersrv)
	// Register OIDC mapping server for the regional tenant manager
	oidcmappingv1.RegisterOIDCMappingServer(grpcServer, oidcmappingsrv)

	listener, err := new(net.ListenConfig).Listen(ctx, "tcp", cfg.GRPC.Address)
	if err != nil {
		return oops.In("gRPC Server").
			WithContext(ctx).
			Wrapf(err, "creating listener")
	}

	go func() {
		slogctx.Info(ctx, "Starting GRPC server", "address", cfg.GRPC.Address)

		if err := grpcServer.Serve(listener); err != nil {
			slogctx.Error(ctx, "Failed to serve gRPC endpoint", "error", err)
		}

		slogctx.Info(ctx, "Stopped gRPC server")
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(ctx, cfg.GRPC.ShutdownTimeout)
	defer cancel()

	grpcServer.GracefulStop()
	slogctx.Info(shutdownCtx, "Completed graceful shutdown of gRPC server")

	return nil
}
