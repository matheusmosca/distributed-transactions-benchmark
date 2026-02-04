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

// HandleTryDebitWallet handler para fase TRY do TCC
func HandleTryDebitWallet(uc *PaymentUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TCCActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("❌ Invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		ctx, span := startSpanFromPayload(c, "payment.TryDebitWallet", req)
		defer span.End()

		err := uc.TryDebitWallet(ctx, req)
		if err != nil {
			log.Printf("❌ [TRY] ORDER_ID %s | Failed: %v", req.OrderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "try_success", "order_id": req.OrderID})
	}
}

// HandleConfirmDebitWallet handler para fase CONFIRM do TCC
func HandleConfirmDebitWallet(uc *PaymentUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TCCActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("❌ Invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		ctx, span := startSpanFromPayload(c, "payment.ConfirmDebitWallet", req)
		defer span.End()

		err := uc.ConfirmDebitWallet(ctx, req)
		if err != nil {
			log.Printf("❌ [CONFIRM] ORDER_ID %s | Failed: %v", req.OrderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "confirm_success", "order_id": req.OrderID})
	}
}

// HandleCancelDebitWallet handler para fase CANCEL do TCC
func HandleCancelDebitWallet(uc *PaymentUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TCCActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("❌ Invalid request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		ctx, span := startSpanFromPayload(c, "payment.CancelDebitWallet", req)
		defer span.End()

		err := uc.CancelDebitWallet(ctx, req)
		if err != nil {
			log.Printf("❌ [CANCEL] ORDER_ID %s | Failed: %v", req.OrderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "cancel_success", "order_id": req.OrderID})
	}
}

// HandleHealth handler para health check
func HandleHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "payment-service-tcc"})
	}
}
