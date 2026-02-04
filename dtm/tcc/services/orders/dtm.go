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

// TCCOrchestrator abstrai as operações TCC do DTM
type TCCOrchestrator interface {
	CreateOrderTCC(ctx context.Context, req CreateOrderRequest) (string, string, error)
}

// DTMTCCOrchestrator implementa TCCOrchestrator usando DTM
type DTMTCCOrchestrator struct{}

// NewDTMTCCOrchestrator cria uma nova instância do orquestrador TCC
func NewDTMTCCOrchestrator() *DTMTCCOrchestrator {
	return &DTMTCCOrchestrator{}
}

// CreateOrderTCC registra as branches TCC e retorna imediatamente (assíncrono)
func (to *DTMTCCOrchestrator) CreateOrderTCC(ctx context.Context, req CreateOrderRequest) (string, string, error) {
	tracer := otel.Tracer("dtm-tcc-orchestrator")

	// Criar span apenas para o REGISTRO das branches (rápido!)
	ctx, registrationSpan := tracer.Start(ctx, "TCC-Registration")
	defer registrationSpan.End()

	// Gerar OrderID ANTES de registrar as branches
	orderID := uuid.New().String()
	var gid string

	// Extract trace context from the incoming context
	var traceID, spanID string
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
		spanID = span.SpanContext().SpanID().String()
	}

	registrationSpan.SetAttributes(
		attribute.String("tcc.trace_id", traceID),
	)

	defer func() {
		if r := recover(); r != nil {
			registrationSpan.RecordError(fmt.Errorf("panic in MustGenGid due to unavailable dtm: %v", r))
			registrationSpan.SetStatus(codes.Error, "panic in MustGenGid due to unavailable dtm")
		}
	}()
	gid = dtmcli.MustGenGid(getEnv("DTM_SERVER", "http://dtm:36789/api/dtmsvr"))
	if gid == "" {
		return orderID, "", fmt.Errorf("internal error: failed to generate GID")
	}

	// Adicionar atributos ao span de registro
	registrationSpan.SetAttributes(
		attribute.String("tcc.gid", gid),
		attribute.String("tcc.order_id", orderID),
		attribute.String("tcc.user_id", req.UserID),
		attribute.String("tcc.product_id", req.ProductID),
		attribute.Int("tcc.participants", 3), // Orders, Inventory, Payment
	)

	log.Printf("🚀 Registering TCC branches | TraceID: %s | GID: %s | OrderID: %s", traceID, gid, orderID)

	// Preparar payload com trace context (sempre 1 unidade por pedido)
	payload := TCCActionRequest{
		OrderID:    orderID,
		UserID:     req.UserID,
		ProductID:  req.ProductID,
		TotalPrice: req.TotalPrice,
		TraceID:    traceID,
		SpanID:     spanID,
	}

	// Criar transação TCC usando TccGlobalTransaction
	ordersServiceURL := getEnv("ORDERS_SERVICE_URL", "http://orders-service:8080")
	inventoryServiceURL := getEnv("INVENTORY_SERVICE_URL", "http://inventory-service:8081")
	paymentServiceURL := getEnv("PAYMENT_SERVICE_URL", "http://payment-service:8082")

	// Registrar as 3 branches no DTM (retorna rápido, apenas registro!)
	err := dtmcli.TccGlobalTransaction(getEnv("DTM_SERVER", "http://dtm:36789/api/dtmsvr"), gid, func(tcc *dtmcli.Tcc) (*resty.Response, error) {
		// Branch 1: Orders - cria a ordem
		resp, err := tcc.CallBranch(
			&payload,
			ordersServiceURL+"/api/orders/try",
			ordersServiceURL+"/api/orders/confirm",
			ordersServiceURL+"/api/orders/cancel",
		)
		if err != nil {
			registrationSpan.AddEvent("Orders branch registration failed")
			return resp, fmt.Errorf("failed to register orders TCC branch: %w", err)
		}
		registrationSpan.AddEvent("Orders branch registered")

		// Branch 2: Inventory - reserva estoque
		resp, err = tcc.CallBranch(
			&payload,
			inventoryServiceURL+"/api/inventory/try",
			inventoryServiceURL+"/api/inventory/confirm",
			inventoryServiceURL+"/api/inventory/cancel",
		)
		if err != nil {
			registrationSpan.AddEvent("Inventory branch registration failed")
			return resp, fmt.Errorf("failed to register inventory TCC branch: %w", err)
		}
		registrationSpan.AddEvent("Inventory branch registered")

		// Branch 3: Payment - processa pagamento
		resp, err = tcc.CallBranch(
			&payload,
			paymentServiceURL+"/api/payment/try",
			paymentServiceURL+"/api/payment/confirm",
			paymentServiceURL+"/api/payment/cancel",
		)
		if err != nil {
			registrationSpan.AddEvent("Payment branch registration failed")
			return resp, fmt.Errorf("failed to register payment TCC branch: %w", err)
		}
		registrationSpan.AddEvent("Payment branch registered")

		registrationSpan.AddEvent("All 3 TCC branches registered - DTM will execute asynchronously")
		return resp, nil
	})

	if err != nil {
		registrationSpan.RecordError(err)
		registrationSpan.SetStatus(codes.Error, "TCC branch registration failed")
		log.Printf("❌ TCC REGISTRATION FAILED | TraceID: %s | GID: %s | Error: %v", traceID, gid, err)
		return orderID, traceID, fmt.Errorf("TCC branch registration failed: %w", err)
	}

	registrationSpan.SetStatus(codes.Ok, "TCC branches registered successfully")
	log.Printf("✅ TCC REGISTERED | TraceID: %s | GID: %s | OrderID: %s (DTM executing asynchronously)", traceID, gid, orderID)
	return orderID, traceID, nil
}
