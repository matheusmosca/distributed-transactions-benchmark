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

// HandleXADecreaseStock handler para operação XA do 2PC
// Usa dtmcli.XaLocalTransaction para gerenciar XA START/PREPARE/COMMIT/ROLLBACK
func HandleXADecreaseStock(uc *InventoryUseCase, dbConf *dtmcli.DBConf) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extrair trace context do header se presente
		traceparent := c.GetHeader("traceparent")
		ctx := c.Request.Context()
		if traceparent != "" {
			// Parse traceparent formato: 00-{trace-id}-{parent-span-id}-{flags}
			ctx = ExtractTraceContext(ctx, traceparent)
		}

		// dtmcli.XaLocalTransaction gerencia todo o ciclo XA automaticamente
		// Na fase PREPARE: body contém o payload
		// Na fase COMMIT/ROLLBACK: body é nil (DTM chama novamente)
		err := dtmcli.XaLocalTransaction(c.Request.URL.Query(), *dbConf, func(db *sql.DB, xa *dtmcli.Xa) error {
			// Parse body apenas se não for nil (fase PREPARE)
			body := c.Request.Body
			if body == nil {
				// Fase COMMIT/ROLLBACK - DTM gerencia automaticamente
				log.Printf("✅ [XA] Commit/Rollback phase handled by DTM")
				return nil
			}

			var req XAActionRequest
			if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
				log.Printf("❌ Invalid request body: %v", err)
				return err
			}

			// Iniciar span com trace context propagado
			tracer := otel.Tracer("inventory-service")
			spanCtx, span := tracer.Start(ctx, "inventory.xa.decreaseStock")
			defer span.End()

			// Executa a operação XA (PREPARE phase)
			log.Printf("📦 [XA PREPARE] Decreasing stock for ProductID=%s, OrderID=%s", req.ProductID, req.OrderID)
			err := uc.DecreaseStockXA(db, req)
			if err != nil {
				span.RecordError(err)
			}
			_ = spanCtx // Use context if needed
			return err
		})

		if err != nil {
			log.Printf("❌ [XA] Failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "xa_success"})
	}
}

// HandleHealth handler para health check
func HandleHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "inventory-service-2pc"})
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
