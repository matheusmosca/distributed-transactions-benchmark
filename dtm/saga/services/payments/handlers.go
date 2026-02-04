package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// PaymentHandler contém os handlers HTTP para pagamentos
type PaymentHandler struct {
	useCase *PaymentUseCase
	tracer  trace.Tracer
}

// NewPaymentHandler cria uma nova instância de PaymentHandler
func NewPaymentHandler(useCase *PaymentUseCase, tracer trace.Tracer) *PaymentHandler {
	return &PaymentHandler{
		useCase: useCase,
		tracer:  tracer,
	}
}

// DebitPayment é o endpoint da ação SAGA para debitar pagamento
func (h *PaymentHandler) DebitPayment(c *gin.Context) {
	var req SagaActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, span := getOrStartSpanFromPayload(c.Request.Context(), "debit_payment", req)
	defer span.End()

	span.SetAttributes(
		attribute.String("order_id", req.OrderID),
		attribute.String("user_id", req.UserID),
		attribute.String("amount", fmt.Sprintf("%d", req.Amount)),
		attribute.String("trace_id", req.TraceID),
	)

	err := h.useCase.DebitPayment(ctx, req)
	if err != nil {
		log.Printf("ℹ️ [DEBIT] FAILED for OrderID=%s : %s", req.OrderID, err)
		// Determina o código de erro baseado na mensagem
		if containsAny(err.Error(), []string{"wallet not found", "insufficient funds"}) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to debit payment"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

// CompensatePayment é o endpoint da ação SAGA para compensar pagamento
func (h *PaymentHandler) CompensatePayment(c *gin.Context) {
	var req SagaActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, span := getOrStartSpanFromPayload(c.Request.Context(), "compensate_payment", req)
	defer span.End()

	span.SetAttributes(
		attribute.String("order_id", req.OrderID),
		attribute.String("user_id", req.UserID),
		attribute.String("trace_id", req.TraceID),
	)

	err := h.useCase.CompensatePayment(ctx, req)
	if err != nil {
		log.Printf("ℹ️ [COMPENSATE DEBIT] FAILED for OrderID=%s : %s", req.OrderID, err)

		// Determina o código de erro baseado na mensagem
		if containsAny(err.Error(), []string{"version conflict", "max retries exceeded"}) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to compensate payment"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

// HealthCheck é o endpoint de health check
func (h *PaymentHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// containsAny verifica se a string contém alguma das substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// getOrStartSpanFromPayload garante que sempre retorna um span filho do tracing atual (ou cria um novo se não houver)
func getOrStartSpanFromPayload(ctx context.Context, operationName string, req SagaActionRequest) (context.Context, trace.Span) {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return startSpanFromPayload(ctx, operationName, req)
	}
	// Se já existe um span válido, apenas o renomeia e retorna o contexto atual
	span.SetName(operationName)
	return ctx, span
}
