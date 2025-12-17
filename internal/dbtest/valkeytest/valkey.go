package valkeytest

import (
	"context"
	"net"

	"github.com/docker/go-connections/nat"
	"github.com/valkey-io/valkey-go"

	valkeycontainer "github.com/testcontainers/testcontainers-go/modules/valkey"
	slogctx "github.com/veqryn/slog-context"
)

// Start initialises a ValKey instance and returns a client, database port, and termination function.
func Start(ctx context.Context) (valkey.Client, nat.Port, func(ctx context.Context)) {
	valkeyContainer, err := valkeycontainer.Run(ctx, "valkey/valkey:8-alpine")
	if err != nil {
		slogctx.Error(ctx, "Failed to start ValKey container", "error", err)
		panic(err)
	}

	port, err := valkeyContainer.MappedPort(ctx, nat.Port("6379"))
	if err != nil {
		slogctx.Error(ctx, "Failed to map a port for the ValKey container", "error", err)
		panic(err)
	}

	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{net.JoinHostPort("localhost", port.Port())},
	})
	if err != nil {
		slogctx.Error(ctx, "Failed to initialise a ValKey client", "error", err)
	}

	terminate := func(ctx context.Context) {
		err := valkeyContainer.Terminate(ctx)
		if err != nil {
			slogctx.Error(ctx, "Failed to terminate ValKey container", "error", err)
			panic(err)
		}
	}

	return client, port, terminate
}
