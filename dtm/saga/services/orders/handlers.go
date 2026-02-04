package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// OrderUseCaseInterface define a interface para o use case
type OrderUseCaseInterface interface {
	CreateOrderSaga(ctx context.Context, req CreateOrderRequest) (string, string, error)
	CreateOrder(ctx context.Context, req SagaActionRequest) error
	CompleteOrder(ctx context.Context, req SagaActionRequest) error
	CancelOrder(ctx context.Context, req SagaActionRequest) error
}

// OrderHandler contém os handlers HTTP
type OrderHandler struct {
	useCase OrderUseCaseInterface
	tracer  trace.Tracer
}

// NewOrderHandler cria uma nova instância de OrderHandler
func NewOrderHandler(useCase OrderUseCaseInterface, tracer trace.Tracer) *OrderHandler {
	return &OrderHandler{
		useCase: useCase,
		tracer:  tracer,
	}
}

// CreateOrderSaga inicia uma transação SAGA para criar um pedido
func (h *OrderHandler) CreateOrderSaga(c *gin.Context) {
	// Span principal que engloba toda a transação SAGA
	ctx, span := h.tracer.Start(c.Request.Context(), "create_order_saga")
	defer span.End()

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	span.SetAttributes(
		attribute.String("user_id", req.UserID),
		attribute.String("product_id", req.ProductID),
		attribute.Int("amount", req.Amount),
	)

	// Criar span filho para o processamento do DTM SAGA
	ctxDTM, spanDTM := h.tracer.Start(ctx, "dtm.orchestration")
	spanDTM.SetAttributes(
		attribute.String("component", "dtm-coordinator"),
	)

	orderID, gid, err := h.useCase.CreateOrderSaga(ctxDTM, req)

	if err != nil {
		spanDTM.RecordError(err)
		spanDTM.End()
		span.RecordError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	spanDTM.SetAttributes(
		attribute.String("order_id", orderID),
		attribute.String("dtm_gid", gid),
	)
	spanDTM.End()

	span.SetAttributes(
		attribute.String("order_id", orderID),
		attribute.String("dtm_gid", gid),
	)

	c.JSON(http.StatusOK, gin.H{
		"order_id": orderID,
		"saga_gid": gid,
		"message":  "Order SAGA initiated successfully",
	})
}

// CreateOrder é um endpoint SAGA para criar um pedido
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req SagaActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, span := getOrStartSpanFromPayload(c.Request.Context(), "create_order", req)
	defer span.End()

	span.SetAttributes(
		attribute.String("order_id", req.OrderID),
		attribute.String("user_id", req.UserID),
		attribute.String("product_id", req.ProductID),
		attribute.Int("amount", req.Amount),
		attribute.String("trace_id", req.TraceID),
	)

	err := h.useCase.CreateOrder(ctx, req)
	if err != nil {
		span.RecordError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

// CompleteOrder marca o pedido como completado
func (h *OrderHandler) CompleteOrder(c *gin.Context) {
	var req SagaActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, span := getOrStartSpanFromPayload(c.Request.Context(), "complete_order", req)
	defer span.End()

	span.SetAttributes(
		attribute.String("order_id", req.OrderID),
		attribute.String("trace_id", req.TraceID),
	)

	err := h.useCase.CompleteOrder(ctx, req)
	if err != nil {
		span.RecordError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

// CompensateOrder compensa a criação do pedido (marca como rejeitado)
func (h *OrderHandler) CompensateOrder(c *gin.Context) {
	var req SagaActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, span := getOrStartSpanFromPayload(c.Request.Context(), "compensate_order", req)
	defer span.End()

	span.SetAttributes(
		attribute.String("order_id", req.OrderID),
		attribute.String("trace_id", req.TraceID),
	)

	err := h.useCase.CancelOrder(ctx, req)
	if err != nil {
		span.RecordError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

// HealthCheck verifica a saúde do serviço
func (h *OrderHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "orders-service",
	})
}

// getOrStartSpanFromPayload garante que sempre retorna um span filho do tracing atual (ou cria um novo se não houver)
func getOrStartSpanFromPayload(ctx context.Context, operationName string, req SagaActionRequest) (context.Context, trace.Span) {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return startSpanFromPayload(ctx, operationName, req)
	}
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer("")
	return tracer.Start(ctx, operationName)
}
