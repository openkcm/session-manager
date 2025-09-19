package server

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
)

func pingHandlerFunc(cfg *config.Config) func(http.ResponseWriter, *http.Request) {
	traceAttrs := otlp.CreateAttributesFrom(cfg.Application,
		attribute.String(commoncfg.AttrOperation, "ping"),
	)

	tracer := otel.Tracer("PingHandler", trace.WithInstrumentationAttributes(traceAttrs...))

	return func(w http.ResponseWriter, req *http.Request) {
		// Request Id will be propagated through all method calls propagated of this HTTP handler
		ctx := slogctx.With(req.Context(),
			commoncfg.AttrRequestID, uuid.New().String(),
			commoncfg.AttrOperation, "ping",
		)

		// Manual OTEL Tracing
		parentCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(req.Header))

		ctx, span := tracer.Start(
			parentCtx,
			"ping-span",
			trace.WithAttributes(traceAttrs...),
		)
		defer span.End()

		// Metrics
		requestStartTime := time.Now()

		defer func() {
			elapsedTime := time.Since(requestStartTime) / time.Millisecond

			// Metrics logic
			attrs := metric.WithAttributes(
				otlp.CreateAttributesFrom(cfg.Application,
					attribute.String("userAgent", req.UserAgent()),
					attribute.String(commoncfg.AttrOperation, "ping"),
				)...,
			)

			counter.Add(ctx, 1, attrs)
			hist.Record(ctx, int64(elapsedTime), attrs)
		}()

		// Business Logic
		slogctx.Info(ctx, "Starting ping request")
		{
			w.Header().Set("Content-Type", "application/json")

			_, err := w.Write([]byte("{ \"result\": \"ping\" }"))
			if err != nil {
				return
			}
		}

		DoSomething(ctx)
		slogctx.Info(ctx, "Finished ping request")
		// End Business Logic
	}
}

func DoSomething(ctx context.Context) {
	slogctx.Info(ctx, "Method DoSomething has been called")
}
