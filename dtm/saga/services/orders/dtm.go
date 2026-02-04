package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

// SagaOrchestrator abstrai as operações SAGA do DTM
type SagaOrchestrator interface {
	CreateOrderSaga(ctx context.Context, req CreateOrderRequest) (string, string, error)
}

// DTMSagaOrchestrator implementa SagaOrchestrator usando DTM
type DTMSagaOrchestrator struct{}

// NewDTMSagaOrchestrator cria uma nova instância do orquestrador SAGA
func NewDTMSagaOrchestrator() *DTMSagaOrchestrator {
	return &DTMSagaOrchestrator{}
}

// CreateOrderSaga orquestra a transação SAGA
func (so *DTMSagaOrchestrator) CreateOrderSaga(ctx context.Context, req CreateOrderRequest) (string, string, error) {
	orderID := uuid.New().String()

	// Extract trace context from the incoming context
	var traceID, spanID string
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
		spanID = span.SpanContext().SpanID().String()
	}

	defer func() {
		if r := recover(); r != nil {
		}
	}()
	gid := dtmcli.MustGenGid(getEnv("DTM_SERVER", "http://dtm:36789/api/dtmsvr"))

	log.Printf("🚀 Starting SAGA | TraceID: %s | GID: %s | OrderID: %s", traceID, gid, orderID)

	saga := dtmcli.NewSaga(getEnv("DTM_SERVER", "http://dtm:36789/api/dtmsvr"), gid).
		Add(
			getEnv("SERVICE_URL", "http://orders-service:8080")+"/api/orders/create",
			getEnv("SERVICE_URL", "http://orders-service:8080")+"/api/orders/compensate",
			&SagaActionRequest{
				OrderID:   orderID,
				UserID:    req.UserID,
				ProductID: req.ProductID,
				Amount:    req.Amount,
				TraceID:   traceID,
				SpanID:    spanID,
			},
		).
		Add(
			getEnv("INVENTORY_SERVICE_URL", "http://inventory-service:8080")+"/api/inventory/decrease",
			getEnv("INVENTORY_SERVICE_URL", "http://inventory-service:8080")+"/api/inventory/compensate",
			&SagaActionRequest{
				OrderID:   orderID,
				UserID:    req.UserID,
				ProductID: req.ProductID,
				Amount:    req.Amount,
				TraceID:   traceID,
				SpanID:    spanID,
			},
		).
		Add(
			getEnv("PAYMENTS_SERVICE_URL", "http://payments-service:8080")+"/api/payments/debit",
			getEnv("PAYMENTS_SERVICE_URL", "http://payments-service:8080")+"/api/payments/compensate",
			&SagaActionRequest{
				OrderID:   orderID,
				UserID:    req.UserID,
				ProductID: req.ProductID,
				Amount:    req.Amount,
				TraceID:   traceID,
				SpanID:    spanID,
			},
		).
		Add(
			getEnv("SERVICE_URL", "http://orders-service:8080")+"/api/orders/complete",
			"",
			&SagaActionRequest{
				OrderID:   orderID,
				UserID:    req.UserID,
				ProductID: req.ProductID,
				Amount:    req.Amount,
				TraceID:   traceID,
				SpanID:    spanID,
			},
		)

	// saga.WithRetryLimit(30)
	// saga.RetryInterval = 60

	err := saga.Submit()

	if err != nil {
		log.Printf("❌ SAGA failed: %v", err)
		return orderID, gid, fmt.Errorf("failed to process order: %w", err)
	}

	log.Printf("✅ SAGA submitted successfully - GID: %s, OrderID: %s", gid, orderID)

	return orderID, gid, nil
}
