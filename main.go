package main

import (
	"encoding/json"

	"context"
	"fmt"

	"go.uber.org/zap"

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
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var (
	logger            *zap.Logger
	tracer            trace.Tracer
	resource          *sdkresource.Resource
	initResourcesOnce sync.Once
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
}

func initResource() (*sdkresource.Resource, error) {
	initResourcesOnce.Do(func() {
		extraResources, err := sdkresource.New(
			context.Background(),
			sdkresource.WithOS(),
			sdkresource.WithProcess(),
			sdkresource.WithContainer(),
			sdkresource.WithHost(),
		)
		if err != nil {
			logger.Sugar().Fatalf("failed to initialize resource: %v", err)
		}

		resource, err = sdkresource.Merge(
			sdkresource.Default(),
			extraResources,
		)
		if err != nil {
			logger.Sugar().Fatalf("failed to initialize resource: %v", err)
		}
	})

	logger.Sugar().Info("resource sdk initialized")
	return resource, nil
}

func initTracerProvider() *sdktrace.TracerProvider {
	ctx := context.Background()

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		logger.Sugar().Fatalf("new otlp trace grpc exporter failed: %v", err)
	}

	rs, err := initResource()
	if err != nil {
		logger.Sugar().Fatal("failed to init resource", zap.Error(err))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(rs),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	logger.Sugar().Infof("tracer provider initialized")
	return tp
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
	ctx := r.Context()
	_, span := tracer.Start(ctx, "helloHandler")
	defer span.End()

	// span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("custom", "hello"),
	)

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

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

	// Initialize the OTEL TracerProvider
	tp := initTracerProvider()
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
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
