package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// XAOrchestrator abstrai as operações XA do DTM (2PC)
type XAOrchestrator interface {
	CreateOrderXA(ctx context.Context, req CreateOrderRequest) (string, string, error)
}

// DTMXAOrchestrator implementa XAOrchestrator usando DTM (2PC/XA)
type DTMXAOrchestrator struct{}

// NewDTMXAOrchestrator cria uma nova instância do orquestrador XA
func NewDTMXAOrchestrator() *DTMXAOrchestrator {
	return &DTMXAOrchestrator{}
}

// CreateOrderXA registra as branches XA usando 2PC
func (xo *DTMXAOrchestrator) CreateOrderXA(ctx context.Context, req CreateOrderRequest) (string, string, error) {
	tracer := otel.Tracer("dtm-xa-orchestrator")

	// Criar span para o REGISTRO das branches XA (2PC)
	ctx, registrationSpan := tracer.Start(ctx, "XA-Registration")
	defer registrationSpan.End()

	// Gerar OrderID ANTES de registrar as branches
	orderID := uuid.New().String()
	defer func() {
		if r := recover(); r != nil {
			registrationSpan.RecordError(fmt.Errorf("panic in MustGenGid due to unavailable dtm: %v", r))
			registrationSpan.SetStatus(codes.Error, "panic in MustGenGid due to unavailable dtm")
		}
	}()
	gid := dtmcli.MustGenGid(getEnv("DTM_SERVER", "http://dtm:36789/api/dtmsvr"))
	if gid == "" {
		return orderID, "", fmt.Errorf("internal error: failed to generate GID")
	}

	// Extract trace context from the incoming context
	var traceID, spanID string
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
		spanID = span.SpanContext().SpanID().String()
	}

	// Adicionar atributos ao span de registro
	registrationSpan.SetAttributes(
		attribute.String("xa.gid", gid),
		attribute.String("xa.order_id", orderID),
		attribute.String("xa.user_id", req.UserID),
		attribute.String("xa.product_id", req.ProductID),
		attribute.String("xa.trace_id", traceID),
		attribute.Int("xa.participants", 3), // Orders, Inventory, Payment
	)

	log.Printf("🚀 Registering XA branches (2PC) | TraceID: %s | GID: %s | OrderID: %s", traceID, gid, orderID)

	// Preparar payload com trace context
	payload := XAActionRequest{
		OrderID:    orderID,
		UserID:     req.UserID,
		ProductID:  req.ProductID,
		TotalPrice: req.Amount,
		TraceID:    traceID,
		SpanID:     spanID,
	}

	// URLs dos serviços
	ordersServiceURL := getEnv("ORDERS_SERVICE_URL", "http://orders-service:8080")
	inventoryServiceURL := getEnv("INVENTORY_SERVICE_URL", "http://inventory-service:8081")
	paymentServiceURL := getEnv("PAYMENT_SERVICE_URL", "http://payment-service:8082")

	// Criar transação XA usando dtmcli.XaGlobalTransaction2 (2PC via HTTP)
	// XaGlobalTransaction2 permite configurar headers para propagação de trace
	err := dtmcli.XaGlobalTransaction2(getEnv("DTM_SERVER", "http://dtm:36789/api/dtmsvr"), gid, func(xa *dtmcli.Xa) {
		// Configurar headers de trace para propagação entre serviços
		xa.BranchHeaders = map[string]string{
			"traceparent": fmt.Sprintf("00-%s-%s-01", traceID, spanID),
		}
	}, func(xa *dtmcli.Xa) (*resty.Response, error) {
		// Branch 1: Orders - cria a ordem
		resp, err := xa.CallBranch(&payload, ordersServiceURL+"/api/orders/xa")
		if err != nil {
			registrationSpan.AddEvent("Orders XA branch failed")
			return resp, fmt.Errorf("orders XA branch failed: %w", err)
		}
		registrationSpan.AddEvent("Orders XA branch registered")

		// Branch 2: Inventory - decrementa estoque
		resp, err = xa.CallBranch(&payload, inventoryServiceURL+"/api/inventory/xa")
		if err != nil {
			registrationSpan.AddEvent("Inventory XA branch failed")
			return resp, fmt.Errorf("inventory XA branch failed: %w", err)
		}
		registrationSpan.AddEvent("Inventory XA branch registered")

		// Branch 3: Payment - debita saldo
		resp, err = xa.CallBranch(&payload, paymentServiceURL+"/api/payment/xa")
		if err != nil {
			registrationSpan.AddEvent("Payment XA branch failed")
			return resp, fmt.Errorf("payment XA branch failed: %w", err)
		}
		registrationSpan.AddEvent("Payment XA branch registered")

		return resp, nil
	})

	if err != nil {
		registrationSpan.RecordError(err)
		registrationSpan.SetStatus(codes.Error, "XA transaction failed")
		log.Printf("❌ XA TRANSACTION FAILED | TraceID: %s | GID: %s | Error: %v", traceID, gid, err)
		return orderID, traceID, fmt.Errorf("XA transaction failed: %w", err)
	}

	registrationSpan.SetStatus(codes.Ok, "XA transaction completed successfully")
	log.Printf("✅ XA COMPLETED | TraceID: %s | GID: %s | OrderID: %s", traceID, gid, orderID)
	return orderID, traceID, nil
}
