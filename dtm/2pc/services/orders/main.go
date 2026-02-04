package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer trace.Tracer
)

func main() {
	// Initialize OpenTelemetry Tracer
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	tracer = tp.Tracer("orders-service-2pc")

	// Initialize database for XA
	dbConf, err := initDBForXA()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Setup repositories and use cases
	// Note: repository doesn't hold the connection, it receives *sql.DB from DTM
	orderRepository := NewPostgresOrderRepository()
	xaOrchestrator := NewDTMXAOrchestrator()
	orderUseCase := NewOrderUseCase(orderRepository, xaOrchestrator)

	// Setup Gin router
	r := gin.Default()
	r.Use(otelgin.Middleware(getEnv("SERVICE_NAME", "orders-service-2pc")))

	// Health check
	r.GET("/health", HandleHealth())

	// XA orchestrator endpoint - initiates XA transaction (retorna 200 OK após completar)
	r.POST("/api/orders", HandleCreateOrder(orderUseCase))

	// XA participant endpoint - chamado pelo DTM
	r.POST("/api/orders/xa", HandleXACreateOrder(orderUseCase, dbConf))

	port := getEnv("PORT", "8080")
	log.Printf("🚀 Orders Service (2PC/XA) listening on port %s", port)
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

func initDBForXA() (*dtmcli.DBConf, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DATABASE_HOST", "postgres"),
		getEnv("DATABASE_PORT", "5432"),
		getEnv("DATABASE_USER", "root"),
		getEnv("DATABASE_PASSWORD", "pass"),
		getEnv("DATABASE_NAME", "orders_db"),
	)

	// Test connection using database/sql
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Wait for database to be ready
	ctx := context.Background()
	for i := 0; i < 30; i++ {
		if err := db.PingContext(ctx); err == nil {
			log.Println("✅ Connected to orders database (XA-enabled with database/sql)")
			break
		}
		log.Printf("⏳ Waiting for database... (%d/30)", i+1)
		time.Sleep(1 * time.Second)
		if i == 29 {
			return nil, fmt.Errorf("failed to connect to database after 30 attempts")
		}
	}

	// Create DBConf for DTM XA
	// IMPORTANTE: search_path precisa incluir 'public' e 'dtm_barrier'
	dbConf := &dtmcli.DBConf{
		Driver: "postgres",
		Host:   getEnv("DATABASE_HOST", "postgres"),
		Port:   5432,
		User:   getEnv("DATABASE_USER", "root"),
		Db:     getEnv("DATABASE_NAME", "orders_db"),
	}
	dbConf.Password = getEnv("DATABASE_PASSWORD", "pass")

	// Set search_path to include both public and dtm_barrier schemas
	dbConf.Schema = "public,dtm_barrier"

	log.Printf("✅ DBConf created for XA: Driver=%s, Host=%s, DB=%s, Schema=%s", dbConf.Driver, dbConf.Host, dbConf.Db, dbConf.Schema)
	return dbConf, nil
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
			semconv.ServiceName(getEnv("SERVICE_NAME", "orders-service-2pc")),
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
