package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// CreateOrderRequest representa a requisição para criar um pedido
type CreateOrderRequest struct {
	UserID    string `json:"user_id" binding:"required"`
	ProductID string `json:"product_id" binding:"required"`
	Amount    int    `json:"amount" binding:"required,gt=0"`
}

// SagaActionRequest representa a requisição para ações da SAGA
type SagaActionRequest struct {
	OrderID   string `json:"order_id" binding:"required"`
	UserID    string `json:"user_id" binding:"required"`
	ProductID string `json:"product_id" binding:"required"`
	Amount    int    `json:"amount" binding:"required,gt=0"`
	// Manual trace context propagation (DTM doesn't propagate W3C headers)
	TraceID string `json:"trace_id,omitempty"`
	SpanID  string `json:"span_id,omitempty"`
}

// startSpanFromPayload creates a child span linked to the propagated trace context
func startSpanFromPayload(ctx context.Context, operationName string, req SagaActionRequest) (context.Context, trace.Span) {
	// If we have propagated TraceID and SpanID, reconstruct the trace context
	if req.TraceID != "" && req.SpanID != "" {
		parsedTraceID, _ := trace.TraceIDFromHex(req.TraceID)
		parsedSpanID, _ := trace.SpanIDFromHex(req.SpanID)

		spanContext := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    parsedTraceID,
			SpanID:     parsedSpanID,
			TraceFlags: trace.FlagsSampled,
			Remote:     true,
		})

		ctx = trace.ContextWithSpanContext(ctx, spanContext)
	}

	// Get the global tracer
	tracer := otel.Tracer("orders-service")

	// Create span with the reconstructed context
	return tracer.Start(ctx, operationName)
}

func main() {
	// Initialize OpenTelemetry
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	mp, err := initMetrics()
	if err != nil {
		log.Fatalf("Failed to initialize metrics: %v", err)
	}
	defer func() {
		if err := mp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down meter: %v", err)
		}
	}()

	// Initialize database
	dbPool, err := initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbPool.Close()

	// Initialize dependencies
	repository := NewOrderRepository(dbPool)
	sagaOrchestrator := NewDTMSagaOrchestrator()
	tracer := tp.Tracer("orders-service")
	useCase := NewOrderUseCase(repository, sagaOrchestrator)
	handler := NewOrderHandler(useCase, tracer)

	// Setup Gin router
	r := gin.Default()
	// Middleware otelgin removido para evitar spans automáticos duplicados

	// Health check
	r.GET("/health", handler.HealthCheck)

	// Orchestrator endpoint - initiates SAGA
	r.POST("/api/orders", handler.CreateOrderSaga)

	// SAGA action endpoints
	r.POST("/api/orders/create", handler.CreateOrder)
	r.POST("/api/orders/complete", handler.CompleteOrder)
	r.POST("/api/orders/compensate", handler.CompensateOrder)

	port := getEnv("PORT", "8080")
	log.Printf("🚀 Orders Service listening on port %s", port)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initDB() (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&pool_max_conns=25&pool_min_conns=5",
		getEnv("DATABASE_USER", "root"),
		getEnv("DATABASE_PASSWORD", "pass"),
		getEnv("DATABASE_HOST", "localhost"),
		getEnv("DATABASE_PORT", "5432"),
		getEnv("DATABASE_NAME", "orders_db"),
	)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Configure connection pool
	config.MaxConns = 30
	config.MaxConns = 10
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			log.Println("✅ Connected to orders database with connection pool")
			return pool, nil
		}
		log.Printf("⏳ Waiting for database... (%d/30)", i+1)
		time.Sleep(1 * time.Second)
	}

	pool.Close()
	return nil, fmt.Errorf("failed to connect to database after 30 attempts")
}

func initTracer() (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	otlpEndpoint := getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(getEnv("SERVICE_NAME", "orders-service")),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	otel.SetTracerProvider(tp)

	return tp, nil
}

func initMetrics() (*sdkmetric.MeterProvider, error) {
	ctx := context.Background()

	otlpEndpoint := getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")

	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(otlpEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(getEnv("SERVICE_NAME", "orders-service")),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	return mp, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
