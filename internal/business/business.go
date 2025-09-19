package business

import (
	"context"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
)

func Main(ctx context.Context, cfg *config.Config) error {
	slogctx.Info(ctx, "Starting business logic", "name", cfg.Application.Name)

	return nil
}
