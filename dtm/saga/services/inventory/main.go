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
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	dbPool *pgxpool.Pool
	tracer trace.Tracer
)

// startSpanFromPayload creates a child span linked to the propagated trace context
func startSpanFromPayload(ctx context.Context, operationName string, req SagaActionRequest) (context.Context, trace.Span) {
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

	return tracer.Start(ctx, operationName)
}

type SagaActionRequest struct {
	OrderID   string `json:"order_id" binding:"required"`
	UserID    string `json:"user_id"`
	ProductID string `json:"product_id" binding:"required"`
	Amount    int    `json:"amount"`
	// Manual trace context propagation
	TraceID string `json:"trace_id,omitempty"`
	SpanID  string `json:"span_id,omitempty"`
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

	tracer = tp.Tracer("inventory-service")

	// Initialize database
	dbPool, err = initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbPool.Close()

	// Setup Gin router
	r := gin.Default()

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	repository := NewInventoryRepository(dbPool)

	usecases := NewInventoryUseCase(repository, tracer)

	handler := NewInventoryHandler(usecases, tracer)

	// SAGA action endpoints
	r.POST("/api/inventory/decrease", handler.DecreaseStock)
	r.POST("/api/inventory/compensate", handler.CompensateStock)

	port := getEnv("PORT", "8080")
	log.Printf("🚀 Inventory Service listening on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initDB() (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&pool_max_conns=25&pool_min_conns=5",
		getEnv("DATABASE_USER", "root"),
		getEnv("DATABASE_PASSWORD", "saga_pass"),
		getEnv("DATABASE_HOST", "localhost"),
		getEnv("DATABASE_PORT", "5432"),
		getEnv("DATABASE_NAME", "inventory_db"),
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
			log.Println("✅ Connected to inventory database with connection pool")
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
			semconv.ServiceName(getEnv("SERVICE_NAME", "inventory-service")),
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
