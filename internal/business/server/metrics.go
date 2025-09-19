package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/strictmiddleware/nethttp"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/samber/oops"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
)

var (
	counter metric.Int64Counter
	hist    metric.Int64Histogram
)

func initMeters(ctx context.Context, cfg *config.Config) error {
	meter := otel.Meter(
		"kms20/"+cfg.Application.Name,
		metric.WithInstrumentationVersion(otel.Version()),
		metric.WithInstrumentationAttributes(otlp.CreateAttributesFrom(cfg.Application)...),
	)

	var err error

	counter, err = meter.Int64Counter(
		"http.request_count",
		metric.WithDescription("Incoming request count"),
		metric.WithUnit("request"),
	)
	if err != nil {
		return oops.In("HTTP Server").
			WithContext(ctx).
			Wrapf(err, "creating request_count meter")
	}

	hist, err = meter.Int64Histogram(
		"http.duration",
		metric.WithDescription("Incoming end to end duration"),
		metric.WithUnit("milliseconds"),
	)
	if err != nil {
		return oops.In("HTTP Server").
			WithContext(ctx).
			Wrapf(err, "creating duration meter")
	}

	return nil
}

// newTraceMiddleware covers the openapi.StrictServerInterface with tracing.
func newTraceMiddleware(cfg *config.Config) nethttp.StrictHTTPMiddlewareFunc {
	return func(f nethttp.StrictHTTPHandlerFunc, operationID string) nethttp.StrictHTTPHandlerFunc {
		traceAttrs := otlp.CreateAttributesFrom(cfg.Application, attribute.String(commoncfg.AttrOperation, operationID))
		tracer := otel.Tracer(operationID, trace.WithInstrumentationAttributes(traceAttrs...))

		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (interface{}, error) {
			ctx = slogctx.With(ctx,
				commoncfg.AttrRequestID, uuid.NewString(),
				commoncfg.AttrOperation, operationID,
			)

			parentCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

			ctx, span := tracer.Start(parentCtx, operationID+"-span", trace.WithAttributes(traceAttrs...))
			defer span.End()

			requestStartTime := time.Now()

			defer func() {
				elapsedTime := time.Since(requestStartTime)

				// Metrics logic
				attrs := metric.WithAttributes(
					otlp.CreateAttributesFrom(cfg.Application,
						attribute.String("userAgent", r.UserAgent()),
						attribute.String(commoncfg.AttrOperation, "ping"),
					)...,
				)

				counter.Add(ctx, 1, attrs)
				hist.Record(ctx, elapsedTime.Milliseconds(), attrs)
			}()

			slogctx.Info(ctx, fmt.Sprintf("Processing %s request", operationID))
			response, err := f(ctx, w, r, request)
			slogctx.Info(ctx, fmt.Sprintf("Finished %s request", operationID))

			return response, err
		}
	}
}
