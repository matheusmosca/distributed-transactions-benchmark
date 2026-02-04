package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

// HandleCreateOrder handler para criação de pedidos - apenas registra branches TCC
func HandleCreateOrder(uc *OrderUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("❌ Invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
			return
		}

		ctx, span := startSpanFromPayload(c, "orders.CreateOrder", req)
		defer span.End()

		orderID, traceID, err := uc.CreateOrder(ctx, req)
		if err != nil {
			log.Printf("❌ Failed to register TCC branches: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register TCC branches", "details": err.Error()})
			return
		}

		// Retorna 202 Accepted - processamento assíncrono via DTM
		c.JSON(http.StatusAccepted, gin.H{
			"order_id": orderID,
			"trace_id": traceID,
			"status":   "processing",
			"message":  "Order is being processed asynchronously via TCC",
		})
	}
}

// HandleTryCreateOrder handler para fase TRY do TCC
func HandleTryCreateOrder(uc *OrderUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {

		var req TCCActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("❌ TRY: Invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		ctx, span := startSpanFromActionPayload(c, "orders.TryCreateOrder", req)
		defer span.End()

		if err := uc.TryCreateOrder(ctx, req); err != nil {
			log.Printf("❌ TRY ORDER_ID %s | FAILED : %v", req.OrderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "TRY phase failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "TRY success"})
	}
}

// HandleConfirmCreateOrder handler para fase CONFIRM do TCC
func HandleConfirmCreateOrder(uc *OrderUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TCCActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("❌ CONFIRM: Invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		ctx, span := startSpanFromActionPayload(c, "orders.ConfirmCreateOrder", req)
		defer span.End()

		if err := uc.ConfirmCreateOrder(ctx, req); err != nil {
			log.Printf("❌ CONFIRM ORDER_ID %s | FAILED: %v", req.OrderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "CONFIRM phase failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "CONFIRM success"})
	}
}

// HandleCancelCreateOrder handler para fase CANCEL do TCC
func HandleCancelCreateOrder(uc *OrderUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TCCActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("❌ CANCEL: Invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		ctx, span := startSpanFromActionPayload(c, "orders.CancelCreateOrder", req)
		defer span.End()

		if err := uc.CancelCreateOrder(ctx, req); err != nil {
			log.Printf("❌ CANCEL FAILED: ORDER_ID %s | %v", req.OrderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "CANCEL phase failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "CANCEL success"})
	}
}

// HandleHealth handler para health check
func HandleHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "orders-service-tcc"})
	}
}

func startSpanFromPayload(c *gin.Context, operationName string, req CreateOrderRequest) (context.Context, trace.Span) {
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

func startSpanFromActionPayload(c *gin.Context, operationName string, req TCCActionRequest) (context.Context, trace.Span) {
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
