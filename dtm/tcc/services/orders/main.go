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

func main() {
	// Initialize OpenTelemetry Tracer (sem metrics)
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	tracer = tp.Tracer("orders-service-tcc")

	// Initialize database
	dbPool, err = initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbPool.Close()

	// Setup repositories and use cases
	orderRepository := NewPostgresOrderRepository(dbPool)
	tccOrchestrator := NewDTMTCCOrchestrator()
	orderUseCase := NewOrderUseCase(orderRepository, tccOrchestrator)

	// Setup Gin router
	r := gin.New()

	// Add middleware
	r.Use(gin.Logger())
	r.Use(gin.RecoveryWithWriter(gin.DefaultWriter, func(c *gin.Context, recovered interface{}) {
		log.Printf("🚨 PANIC RECOVERED: %v", recovered)
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	// r.Use(otelgin.Middleware(getEnv("SERVICE_NAME", "orders-service-tcc")))

	// Health check
	r.GET("/health", HandleHealth())

	// TCC orchestrator endpoint - initiates TCC transaction (retorna 202 Accepted)
	r.POST("/api/orders", HandleCreateOrder(orderUseCase))

	// TCC participant endpoints - chamados pelo DTM
	r.POST("/api/orders/try", HandleTryCreateOrder(orderUseCase))
	r.POST("/api/orders/confirm", HandleConfirmCreateOrder(orderUseCase))
	r.POST("/api/orders/cancel", HandleCancelCreateOrder(orderUseCase))

	port := getEnv("PORT", "8080")
	log.Printf("🚀 Orders Service (TCC) listening on port %s", port)
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
		getEnv("DATABASE_HOST", "postgres"),
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

	otlpEndpoint := getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4318")

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(getEnv("SERVICE_NAME", "orders-service-tcc")),
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

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
