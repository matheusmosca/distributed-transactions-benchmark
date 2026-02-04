package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// OrderUseCase encapsula a lógica de negócio de pedidos (2PC/XA)
type OrderUseCase struct {
	repository     OrderRepository
	xaOrchestrator XAOrchestrator
}

// NewOrderUseCase cria uma nova instância do caso de uso
func NewOrderUseCase(repository OrderRepository, xaOrchestrator XAOrchestrator) *OrderUseCase {
	return &OrderUseCase{
		repository:     repository,
		xaOrchestrator: xaOrchestrator,
	}
}

// CreateOrder registra as branches XA e retorna após completar (síncrono)
func (uc *OrderUseCase) CreateOrder(ctx context.Context, req CreateOrderRequest) (string, string, error) {
	tracer := otel.Tracer("order-service")

	// Criar span para toda a operação XA (2PC)
	ctx, orderSpan := tracer.Start(ctx, "CreateOrder-XA")
	defer orderSpan.End()

	orderSpan.SetAttributes(
		attribute.String("order.user_id", req.UserID),
		attribute.String("order.product_id", req.ProductID),
		attribute.Int("order.total_price", req.Amount),
	)

	log.Printf("📦 Creating order with XA (2PC): UserID=%s, ProductID=%s, Amount=%d (1 unit)",
		req.UserID, req.ProductID, req.Amount)

	// Validações básicas
	if req.Amount <= 0 {
		orderSpan.RecordError(ErrInvalidPrice)
		return "", "", ErrInvalidPrice
	}

	// Executa transação XA (2PC - síncrono)
	orderID, traceID, err := uc.xaOrchestrator.CreateOrderXA(ctx, req)
	if err != nil {
		log.Printf("❌ XA transaction failed: %v", err)
		orderSpan.RecordError(err)
		return orderID, traceID, err
	}

	orderSpan.SetAttributes(
		attribute.String("order.id", orderID),
		attribute.String("trace.id", traceID),
	)

	log.Printf("✅ XA transaction completed | OrderID=%s | TraceID=%s", orderID, traceID)
	return orderID, traceID, nil
}

// CreateOrderXA implementa a operação XA - cria ordem com status "completed"
// Recebe *sql.DB do DTM que já está em transação XA
func (uc *OrderUseCase) CreateOrderXA(db *sql.DB, req XAActionRequest) error {
	log.Printf("🔄 XA: Creating order with status 'completed' | OrderID=%s", req.OrderID)

	order := &Order{
		OrderID:    req.OrderID,
		UserID:     req.UserID,
		ProductID:  req.ProductID,
		TotalPrice: req.TotalPrice,
		Status:     "completed", // 2PC cria diretamente como completed
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := uc.repository.CreateOrderXA(db, order); err != nil {
		log.Printf("❌ XA FAILED: Failed to create order | OrderID=%s | Error=%v", req.OrderID, err)
		return err
	}

	log.Printf("✅ XA SUCCESS: Order created with status 'completed' | OrderID=%s", req.OrderID)
	return nil
}

// extractTraceContext extrai o trace context do payload
func extractTraceContext(ctx context.Context, traceIDHex, spanIDHex string) context.Context {
	if traceIDHex == "" || spanIDHex == "" {
		return ctx
	}

	traceID, err := trace.TraceIDFromHex(traceIDHex)
	if err != nil {
		return ctx
	}

	spanID, err := trace.SpanIDFromHex(spanIDHex)
	if err != nil {
		return ctx
	}

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	return trace.ContextWithSpanContext(ctx, spanContext)
}

// Erros customizados
var (
	ErrInvalidPrice = &OrderError{Message: "amount must be greater than 0"}
)

type OrderError struct {
	Message string
}

func (e *OrderError) Error() string {
	return e.Message
}
