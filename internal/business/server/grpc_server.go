package server

import (
	"context"
	"net"

	"github.com/openkcm/common-sdk/pkg/commongrpc"
	"github.com/samber/oops"

	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	oidcproviderv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcprovider/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/grpc"
)

func StartGRPCServer(ctx context.Context, cfg *config.Config,
	oidcprovidersrv *grpc.OIDCProviderServer,
	oidcmappingsrv *grpc.OIDCMappingServer,
) error {
	grpcServer := commongrpc.NewServer(ctx, &cfg.GRPC.GRPCServer)

	// Register OIDC provider server for ExtAuthZ
	oidcproviderv1.RegisterServiceServer(grpcServer, oidcprovidersrv)
	// Register OIDC mapping server for the regional tenant manager
	oidcmappingv1.RegisterServiceServer(grpcServer, oidcmappingsrv)

	slogctx.Info(ctx, "Starting a listener", "address", cfg.GRPC.Address)

	listener, err := new(net.ListenConfig).Listen(ctx, "tcp", cfg.GRPC.Address)
	if err != nil {
		return oops.In("gRPC Server").
			WithContext(ctx).
			Wrapf(err, "creating listener")
	}

	slogctx.Info(ctx, "A listener started", "address", listener.Addr().String())

	go func() {
		slogctx.Info(ctx, "Serving a gRPC server", "address", listener.Addr().String())
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
