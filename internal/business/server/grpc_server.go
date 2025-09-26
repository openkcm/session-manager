package server

import (
	"context"
	"net"

	"github.com/openkcm/common-sdk/pkg/commongrpc"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/samber/oops"

	slogctx "github.com/veqryn/slog-context"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/openkcm/session-manager/internal/config"
)

func StartGRPCServer(ctx context.Context, cfg *config.Config) error {
	grpcServer := commongrpc.NewServer(ctx, &cfg.GRPC.GRPCServer)

	healthpb.RegisterHealthServer(grpcServer, &health.GRPCServer{})

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
