package main

import (
	"encoding/json"

	"context"
	"fmt"

	"go.uber.org/zap"

	"time"
	"net/http"
	"os"
	"sync"

	// Import your OTEL packages here for instrumentation.
	// The default packages are for manual instrumentation, but you can use
	// auto-instrumentation packages to capture communication at the edge.
	// For more information see https://opentelemetry.io/docs/languages/go/getting-started/

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	// "go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	sdkmetric"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
)

var (
	logger            *zap.Logger
	tracer            trace.Tracer
	resource          *sdkresource.Resource
	initResourcesOnce sync.Once
	meter = otel.Meter("go-example")
	apiCtr metric.Int64Counter
)

func init() {
	var err error

	rawJSON := []byte(`{
        "level": "debug",
        "encoding": "json",
        "outputPaths": ["stdout"],
        "errorOutputPaths": ["stderr"],
        "initialFields": {"service": "go-example"},
        "encoderConfig": {
            "messageKey": "message",
            "levelKey": "level",
            "levelEncoder": "lowercase"
            }
        }
    `)

	var cfg zap.Config

	if err = json.Unmarshal(rawJSON, &cfg); err != nil {
		panic(err)
	}

	logger = zap.Must(cfg.Build())
	defer func() {
		if err := logger.Sync(); err != nil {
			return
		}
	}()
	apiCtr, err = meter.Int64Counter(
		"api.gauge",
		metric.WithDescription("Number of API calls."),
		metric.WithUnit("{call}"),
	)
	if err != nil {
		panic(err)
	}
}

func newTracerProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	r := sdkresource.Default()

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
	)
}

func newMeterProvider(exp sdkmetric.Exporter) *sdkmetric.MeterProvider {
	r := sdkresource.Default()

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(r),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp,
			// Default is 1m. Set to 3s for demonstrative purposes.
			sdkmetric.WithInterval(3*time.Second))),
	)
	return meterProvider
}

func mustMapEnv(target *string, envKey string) {
	v := os.Getenv(envKey)
	if v == "" {
		logger.Sugar().Panicf("environment variable %s not set", envKey)
	}
	*target = v
}

type Response struct {
	Message string `json:"message"`
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Get the tracer context
	ctx, span := tracer.Start(r.Context(), "helloHandler")
	defer span.End()

	span.SetAttributes(
		attribute.String("custom", "hello"),
	)

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	apiCtr.Add(ctx, 1, metric.WithAttributes(semconv.HTTPResponseStatusCode(200)))

	// Create response
	response := Response{
		Message: "Hello, World!",
	}

	// Encode and send response
	// json.NewEncoder(w).Encode(response)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Sugar().Errorf("failed to encode response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func main() {
	var port string
	mustMapEnv(&port, "SERVICE_PORT")

	ctx := context.Background()
	tExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		logger.Fatal("failed to create new otlp exporter", zap.Error(err))
	}

	mExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		logger.Fatal("failed to create new otlp exporter", zap.Error(err))
	}

	mp := newMeterProvider(mExporter)

	// Super important. Without this being set, the meter will not register and
	// metrics will not be actually sent.
	otel.SetMeterProvider(mp)

	// Initialize the OTEL TracerProvider
	tp := newTracerProvider(tExporter)
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			logger.Sugar().Fatalf("failed to shutdown tracer provider: %v", err)
		}
	}()

	tracer = tp.Tracer("go-example")

	http.HandleFunc("/v1/hello", helloHandler)

	addr := fmt.Sprintf(":%s", port)
	logger.Sugar().Infof("starting listening on %q", addr)
	//#nosec G114 -- This should be replaced with actual server configurations before going to production
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Sugar().Fatal(err)
	}
}
