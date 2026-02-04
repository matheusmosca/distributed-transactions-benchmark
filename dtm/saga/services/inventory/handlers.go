package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// InventoryHandler contém os handlers HTTP para inventário
type InventoryHandler struct {
	useCase *InventoryUseCase
	tracer  trace.Tracer
}

// NewInventoryHandler cria uma nova instância de InventoryHandler
func NewInventoryHandler(useCase *InventoryUseCase, tracer trace.Tracer) *InventoryHandler {
	return &InventoryHandler{
		useCase: useCase,
		tracer:  tracer,
	}
}

// DecreaseStock é o endpoint da ação SAGA para diminuir estoque
func (h *InventoryHandler) DecreaseStock(c *gin.Context) {
	var req SagaActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, span := getOrStartSpanFromPayload(c.Request.Context(), "decrease_inventory", req)
	defer span.End()

	span.SetAttributes(
		attribute.String("order_id", req.OrderID),
		attribute.String("product_id", req.ProductID),
		attribute.String("trace_id", req.TraceID),
	)

	err := h.useCase.DecreaseStock(ctx, req)
	if err != nil {
		log.Printf("ℹ️ [STOCK] FAILED for OrderID=%s : %s", req.OrderID, err)

		// Determina o código de erro baseado na mensagem
		if containsAny(err.Error(), []string{"product not found", "insufficient stock"}) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decrease stock"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

// CompensateStock é o endpoint da ação SAGA para compensar estoque
func (h *InventoryHandler) CompensateStock(c *gin.Context) {
	var req SagaActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, span := getOrStartSpanFromPayload(c.Request.Context(), "compensate_inventory", req)
	defer span.End()

	span.SetAttributes(
		attribute.String("order_id", req.OrderID),
		attribute.String("product_id", req.ProductID),
		attribute.String("trace_id", req.TraceID),
	)

	err := h.useCase.CompensateStock(ctx, req)
	if err != nil {
		log.Printf("ℹ️ [COMPENSATE STOCK] FAILED for OrderID=%s : %s", req.OrderID, err)

		// Determina o código de erro baseado na mensagem
		if containsAny(err.Error(), []string{"version conflict", "max retries exceeded"}) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to compensate stock"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

// HealthCheck é o endpoint de health check
func (h *InventoryHandler) HealthCheck(c *gin.Context) {
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
