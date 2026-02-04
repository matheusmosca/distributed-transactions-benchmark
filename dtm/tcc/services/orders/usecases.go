package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
) // OrderUseCase encapsula a lógica de negócio de pedidos
type OrderUseCase struct {
	repository      OrderRepository
	tccOrchestrator TCCOrchestrator
}

// NewOrderUseCase cria uma nova instância do caso de uso
func NewOrderUseCase(repository OrderRepository, tccOrchestrator TCCOrchestrator) *OrderUseCase {
	return &OrderUseCase{
		repository:      repository,
		tccOrchestrator: tccOrchestrator,
	}
}

// CreateOrder registra as branches TCC e retorna 202 Accepted
func (uc *OrderUseCase) CreateOrder(ctx context.Context, req CreateOrderRequest) (string, string, error) {
	log.Printf("📦 Registering order creation: UserID=%s, ProductID=%s, TotalPrice=%d (1 unit)",
		req.UserID, req.ProductID, req.TotalPrice)

	// Validações básicas
	if req.TotalPrice <= 0 {
		return "", "", ErrInvalidPrice
	}

	// Registra branches TCC no DTM (retorna imediatamente!)
	orderID, traceID, err := uc.tccOrchestrator.CreateOrderTCC(ctx, req)
	if err != nil || orderID == "" {
		if orderID == "" {
			orderID = uuid.New().String()
		}

		_ = uc.repository.CreateOrder(ctx, &Order{
			OrderID:    orderID,
			UserID:     req.UserID,
			ProductID:  req.ProductID,
			TotalPrice: req.TotalPrice,
			Status:     "cancelled",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})

		log.Printf("❌ TCC branch registration failed: %v", err)
		return orderID, traceID, fmt.Errorf("registering failed order to recover dtm failure: %s", err.Error())
	}

	log.Printf("✅ TCC branches registered | OrderID=%s | TraceID=%s (processing asynchronously)", orderID, traceID)
	return orderID, traceID, nil
}

// TryCreateOrder implementa a fase TRY do TCC - cria ordem com status "pending"
func (uc *OrderUseCase) TryCreateOrder(ctx context.Context, req TCCActionRequest) error {
	order := &Order{
		OrderID:    req.OrderID,
		UserID:     req.UserID,
		ProductID:  req.ProductID,
		TotalPrice: req.TotalPrice,
		Status:     "pending",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := uc.repository.CreateOrder(ctx, order); err != nil {
		log.Printf("❌ TRY FAILED: Failed to create pending order | OrderID=%s | Error=%v", req.OrderID, err)
		return err
	}

	log.Printf("✅ TRY SUCCESS: Pending order created | OrderID=%s", req.OrderID)
	return nil
}

// ConfirmCreateOrder implementa a fase CONFIRM do TCC - atualiza ordem para "completed"
func (uc *OrderUseCase) ConfirmCreateOrder(ctx context.Context, req TCCActionRequest) error {

	log.Printf("✅ CONFIRM: Updating order to 'completed' | OrderID=%s", req.OrderID)

	if err := uc.repository.UpdateOrderStatus(ctx, req.OrderID, "completed"); err != nil {
		log.Printf("❌ CONFIRM FAILED: Failed to update order status | OrderID=%s | Error=%v", req.OrderID, err)
		return err
	}

	log.Printf("✅ CONFIRM SUCCESS: Order completed | OrderID=%s", req.OrderID)
	return nil
}

// CancelCreateOrder implementa a fase CANCEL do TCC - atualiza ordem para "cancelled"
func (uc *OrderUseCase) CancelCreateOrder(ctx context.Context, req TCCActionRequest) error {
	log.Printf("CANCEL: Updating order to 'cancelled' | OrderID=%s", req.OrderID)

	if err := uc.repository.UpdateOrderStatus(ctx, req.OrderID, "cancelled"); err != nil {
		log.Printf("❌ CANCEL FAILED: Failed to update order status | OrderID=%s | Error=%v", req.OrderID, err)
		return err
	}

	log.Printf("✅ CANCEL SUCCESS: Order cancelled | OrderID=%s", req.OrderID)
	return nil
}

// extractTraceContext recupera o trace context do payload e injeta no contexto
func extractTraceContext(ctx context.Context, traceIDStr, spanIDStr string) context.Context {
	if traceIDStr == "" || spanIDStr == "" {
		return ctx
	}

	// Parse traceID e spanID
	traceID, err := trace.TraceIDFromHex(traceIDStr)
	if err != nil {
		log.Printf("⚠️  Invalid traceID: %v", err)
		return ctx
	}

	spanID, err := trace.SpanIDFromHex(spanIDStr)
	if err != nil {
		log.Printf("⚠️  Invalid spanID: %v", err)
		return ctx
	}

	// Criar SpanContext
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	// Injetar no contexto
	return trace.ContextWithSpanContext(ctx, spanContext)
}

// Erros customizados
var (
	ErrInvalidPrice = &OrderError{Message: "total price must be greater than 0"}
)

type OrderError struct {
	Message string
}

func (e *OrderError) Error() string {
	return e.Message
}
