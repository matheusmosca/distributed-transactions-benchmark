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
)

var (
	dbConf *dtmcli.DBConf
)

func main() {
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	// Initialize database connection for XA
	dbConf, err = initDBForXA()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Setup repositories and use cases
	inventoryRepository := NewPostgresInventoryRepository()
	inventoryUseCase := NewInventoryUseCase(inventoryRepository)

	// Setup Gin router
	r := gin.Default()
	r.Use(otelgin.Middleware(getEnv("SERVICE_NAME", "inventory-service-2pc")))

	// Health check
	r.GET("/health", HandleHealth())

	// XA endpoint (2PC)
	r.POST("/api/inventory/xa", HandleXADecreaseStock(inventoryUseCase, dbConf))

	port := getEnv("PORT", "8081")
	log.Printf("🚀 Inventory Service (2PC/XA) listening on port %s", port)

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
	// Construir DSN para database/sql (formato PostgreSQL lib/pq)
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DATABASE_HOST", "postgres"),
		getEnv("DATABASE_PORT", "5432"),
		getEnv("DATABASE_USER", "root"),
		getEnv("DATABASE_PASSWORD", "pass"),
		getEnv("DATABASE_NAME", "inventory_db"),
	)

	// Criar DBConf para DTM XA
	dbConf := &dtmcli.DBConf{
		Driver:   "postgres",
		Host:     getEnv("DATABASE_HOST", "postgres"),
		Port:     5432,
		User:     getEnv("DATABASE_USER", "root"),
		Password: getEnv("DATABASE_PASSWORD", "pass"),
		Db:       getEnv("DATABASE_NAME", "inventory_db"),
		Schema:   "public,dtm_barrier", // search_path para encontrar tabelas
	}

	testDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer testDB.Close()

	testDB.SetMaxOpenConns(25)
	testDB.SetMaxIdleConns(5)
	testDB.SetConnMaxLifetime(time.Hour)

	// Testar conectividade
	for i := 0; i < 30; i++ {
		if err := testDB.Ping(); err == nil {
			log.Println("✅ Connected to inventory database (XA mode)")
			return dbConf, nil
		}
		log.Printf("⏳ Waiting for database... (%d/30)", i+1)
		time.Sleep(1 * time.Second)
	}

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
			semconv.ServiceName(getEnv("SERVICE_NAME", "inventory-service-2pc")),
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
