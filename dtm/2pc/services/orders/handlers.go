package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// HandleCreateOrder handler para criação de pedidos - executa XA (2PC) síncrono
func HandleCreateOrder(uc *OrderUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "orders.CreateOrder")
		defer span.End()

		var req CreateOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("❌ Invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
			return
		}

		orderID, traceID, err := uc.CreateOrder(ctx, req)
		if err != nil {
			log.Printf("❌ Failed to create order with XA: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order", "details": err.Error()})
			return
		}

		// Retorna 200 OK - processamento síncrono via 2PC/XA
		c.JSON(http.StatusOK, gin.H{
			"order_id": orderID,
			"trace_id": traceID,
			"status":   "completed",
			"message":  "Order created successfully via 2PC/XA",
		})
	}
}

// HandleXACreateOrder handler para operação XA usando dtmcli.XaLocalTransaction
func HandleXACreateOrder(uc *OrderUseCase, dbConf *dtmcli.DBConf) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("🔄 XA Handler: Received XA request | Path=%s | Query=%s", c.Request.URL.Path, c.Request.URL.RawQuery)

		// Extrair trace context do header se presente
		traceparent := c.GetHeader("traceparent")
		ctx := c.Request.Context()
		if traceparent != "" {
			// Parse traceparent formato: 00-{trace-id}-{parent-span-id}-{flags}
			ctx = ExtractTraceContext(ctx, traceparent)
		}

		// XaLocalTransaction gerencia PREPARE e COMMIT/ROLLBACK automaticamente
		err := dtmcli.XaLocalTransaction(c.Request.URL.Query(), *dbConf, func(db *sql.DB, xa *dtmcli.Xa) error {
			// PREPARE phase: body tem payload
			// COMMIT/ROLLBACK phase: body é nil
			if c.Request.Body == nil {
				log.Printf("⚠️ XA: COMMIT/ROLLBACK phase - DTM handling automatically")
				return nil
			}

			var req XAActionRequest
			if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
				log.Printf("❌ XA: Failed to decode request | Error=%v", err)
				return err
			}

			// Iniciar span com trace context propagado
			tracer := otel.Tracer("orders-service")
			spanCtx, span := tracer.Start(ctx, "orders.xa.createOrder")
			defer span.End()

			log.Printf("🔄 XA PREPARE: Creating order | OrderID=%s", req.OrderID)
			err := uc.CreateOrderXA(db, req)
			if err != nil {
				span.RecordError(err)
			}
			_ = spanCtx // Use context if needed
			return err
		})

		if err != nil {
			log.Printf("❌ XA FAILED: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "XA operation failed", "details": err.Error()})
			return
		}

		log.Printf("✅ XA Handler: Transaction successful")
		c.JSON(http.StatusOK, gin.H{"status": "xa_success"})
	}
}

// HandleHealth handler para health check
func HandleHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "orders-service-2pc"})
	}
}

// ExtractTraceContext extrai trace context do header traceparent formato W3C
// Format: 00-{trace-id}-{parent-span-id}-{flags}
func ExtractTraceContext(ctx context.Context, traceparent string) context.Context {
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return ctx
	}

	traceIDStr := parts[1]
	spanIDStr := parts[2]

	traceID, err := trace.TraceIDFromHex(traceIDStr)
	if err != nil {
		return ctx
	}

	spanID, err := trace.SpanIDFromHex(spanIDStr)
	if err != nil {
		return ctx
	}

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	return trace.ContextWithSpanContext(ctx, spanContext)
}
