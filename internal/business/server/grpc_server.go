package server

import (
	"context"
	"net"

	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/samber/oops"
	"google.golang.org/grpc"

	slogctx "github.com/veqryn/slog-context"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/openkcm/session-manager/internal/config"
)

func createGRPCServer(_ context.Context, _ *config.Config) (*grpc.Server, error) {
	var opts []grpc.ServerOption

	opts = append(opts, grpc.StatsHandler(otlp.NewServerHandler()))

	grpcServer := grpc.NewServer(opts...)

	return grpcServer, nil
}

func StartGRPCServer(ctx context.Context, cfg *config.Config) error {
	grpcServer, err := createGRPCServer(ctx, cfg)
	if err != nil {
		return oops.In("gRPC Server").
			WithContext(ctx).
			Wrapf(err, "gRPC server creation")
	}

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
