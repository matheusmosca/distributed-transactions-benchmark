package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

// startSpanFromPayload creates a child span linked to the propagated trace context
func startSpanFromPayload(c *gin.Context, operationName string, req TCCActionRequest) (context.Context, trace.Span) {
	ctx := c.Request.Context()

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

// HandleTryDecreaseStock handler para fase TRY do TCC
func HandleTryDecreaseStock(uc *InventoryUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TCCActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("❌ Invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		ctx, span := startSpanFromPayload(c, "inventory.TryDecreaseStock", req)
		defer span.End()

		err := uc.TryDecreaseStock(ctx, req)
		if err != nil {
			log.Printf("❌ [TRY]  ORDER_ID %s | Failed: %v", req.OrderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "try_success", "order_id": req.OrderID})
	}
}

// HandleConfirmDecreaseStock handler para fase CONFIRM do TCC
func HandleConfirmDecreaseStock(uc *InventoryUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TCCActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("❌ Invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		ctx, span := startSpanFromPayload(c, "inventory.ConfirmDecreaseStock", req)
		defer span.End()

		err := uc.ConfirmDecreaseStock(ctx, req)
		if err != nil {
			log.Printf("❌ [CONFIRM] ORDER_ID %s | Failed: %v", req.OrderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "confirm_success", "order_id": req.OrderID})
	}
}

// HandleCancelDecreaseStock handler para fase CANCEL do TCC
func HandleCancelDecreaseStock(uc *InventoryUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TCCActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("❌ Invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		ctx, span := startSpanFromPayload(c, "inventory.CancelDecreaseStock", req)
		defer span.End()

		err := uc.CancelDecreaseStock(ctx, req)
		if err != nil {
			log.Printf("❌ ORDER_ID %s | [CANCEL] Failed: %v", req.OrderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "cancel_success", "order_id": req.OrderID})
	}
}

// HandleHealth handler para health check
func HandleHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "inventory-service-tcc"})
	}
}
