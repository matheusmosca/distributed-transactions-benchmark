package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
)

// OrderUseCase contém a lógica de negócio dos pedidos
type OrderUseCase struct {
	repository       Repository
	sagaOrchestrator SagaOrchestrator
}

// NewOrderUseCase cria uma nova instância de OrderUseCase
func NewOrderUseCase(
	repository Repository,
	sagaOrchestrator SagaOrchestrator,
) *OrderUseCase {
	return &OrderUseCase{
		repository:       repository,
		sagaOrchestrator: sagaOrchestrator,
	}
}

// CreateOrderSaga orquestra a transação SAGA
func (uc *OrderUseCase) CreateOrderSaga(ctx context.Context, req CreateOrderRequest) (string, string, error) {
	orderID, gid, err := uc.sagaOrchestrator.CreateOrderSaga(ctx, req)
	if err != nil || orderID == "" {
		if orderID == "" {
			orderID = uuid.New().String()
		}

		_ = uc.CreateFailedOrder(ctx, req, orderID)
		return "", "", fmt.Errorf("registering failed order to recover saga failure: %s", err.Error())
	}

	return orderID, gid, nil
}

func (uc *OrderUseCase) CreateFailedOrder(ctx context.Context, req CreateOrderRequest, orderID string) error {
	order := NewOrder(orderID, req.UserID, req.ProductID, req.Amount)

	err := order.Fail()
	if err != nil {
		return fmt.Errorf("registering failed order: %w", err)
	}

	err = uc.repository.CreateOrder(ctx, order)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	return nil
}

// CreateOrder é a ação SAGA para criar um pedido
func (uc *OrderUseCase) CreateOrder(ctx context.Context, req SagaActionRequest) error {
	log.Printf("➡️ [CREATE ORDER] OrderID: %s", req.OrderID)

	// Create order
	order := NewOrder(req.OrderID, req.UserID, req.ProductID, req.Amount)
	err := uc.repository.CreateOrder(ctx, order)
	if err != nil {
		log.Printf("❌ Failed to create order: %v", err)
		return fmt.Errorf("failed to create order: %w", err)
	}

	log.Printf("✅ Order created: %s", req.OrderID)
	return nil
}

// CompleteOrder marca o pedido como completado
func (uc *OrderUseCase) CompleteOrder(ctx context.Context, req SagaActionRequest) error {
	log.Printf("✅ [COMPLETE ORDER] OrderID: %s", req.OrderID)

	err := uc.repository.UpdateOrderStatus(ctx, req.OrderID, OrderStatusCompleted)
	if err != nil {
		log.Printf("❌ Failed to complete order: %v", err)
		return fmt.Errorf("failed to complete order: %w", err)
	}

	log.Printf("✅ Order completed: %s", req.OrderID)
	return nil
}

// CancelOrder marca o pedido como rejeitado (compensação)
func (uc *OrderUseCase) CancelOrder(ctx context.Context, req SagaActionRequest) error {
	log.Printf("↩️ [COMPENSATE ORDER] OrderID: %s", req.OrderID)

	err := uc.repository.UpdateOrderStatus(ctx, req.OrderID, OrderStatusRejected)
	if err != nil {
		log.Printf("❌ Failed to compensate order: %v", err)
		return fmt.Errorf("failed to compensate order: %w", err)
	}

	log.Printf("♻️  Order compensated (rejected): %s", req.OrderID)
	return nil
}
