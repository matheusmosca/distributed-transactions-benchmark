package main

import (
	"context"
	"errors"
	"log"
)

var ErrNotFound = errors.New("not found")

// InventoryUseCase encapsula a lógica de negócio de inventário
type InventoryUseCase struct {
	repository InventoryRepository
}

// NewInventoryUseCase cria uma nova instância do caso de uso
func NewInventoryUseCase(repository InventoryRepository) *InventoryUseCase {
	return &InventoryUseCase{
		repository: repository,
	}
}

// TryDecreaseStock fase TRY do TCC - reserva 1 unidade do estoque
func (uc *InventoryUseCase) TryDecreaseStock(ctx context.Context, req TCCActionRequest) error {
	log.Printf("📦 [TRY] Reserve stock: ProductID=%s, Quantity=1, OrderID=%s",
		req.ProductID, req.OrderID)

	// Inicia transação
	tx, err := uc.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Obtém o produto com lock pessimista
	product, err := uc.repository.GetProductForUpdate(ctx, tx, req.ProductID)
	if err != nil {
		return err
	}

	// Valida estoque disponível
	if product.StockAvailable < 1 {
		log.Printf("❌ [TRY] Insufficient stock for product %s: available=%d", req.ProductID, product.StockAvailable)
		return ErrInsufficientStock
	}

	// Verifica idempotência - se já existe movimentação com status pending
	exists, err := uc.repository.GetInventoryMovementByOrderIDAndStatus(ctx, tx, req.OrderID, "pending")
	if err != nil {
		return err
	}

	if exists {
		log.Printf("ℹ️ [TRY] Inventory movement already pending for OrderID=%s", req.OrderID)
		return nil
	}

	// Executa a reserva
	if err := uc.repository.TryReserveStock(ctx, tx, req.ProductID, req.OrderID); err != nil {
		log.Printf("❌ [TRY] Failed to reserve stock: %v", err)
		return err
	}

	// Commit da transação
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// ConfirmDecreaseStock fase CONFIRM do TCC - confirma a venda de 1 unidade
func (uc *InventoryUseCase) ConfirmDecreaseStock(ctx context.Context, req TCCActionRequest) error {
	log.Printf("✅ [CONFIRM] Confirm stock decrease: ProductID=%s, Quantity=1, OrderID=%s",
		req.ProductID, req.OrderID)

	// Inicia transação
	tx, err := uc.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Obtém o produto com lock pessimista
	_, err = uc.repository.GetProductForUpdate(ctx, tx, req.ProductID)
	if err != nil {
		return err
	}

	// Verifica idempotência - se já existe movimentação com status completed
	exists, err := uc.repository.GetInventoryMovementByOrderIDAndStatus(ctx, tx, req.OrderID, "completed")
	if err != nil {
		return err
	}

	if exists {
		log.Printf("ℹ️ [CONFIRM] Inventory movement already completed for OrderID=%s", req.OrderID)
		return nil
	}

	// Executa a confirmação
	if err := uc.repository.ConfirmReserveStock(ctx, tx, req.ProductID, req.OrderID); err != nil {
		log.Printf("❌ [CONFIRM] Failed to decrease stock: %v", err)
		return err
	}

	// Commit da transação
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// CancelDecreaseStock fase CANCEL do TCC - cancela a reserva de 1 unidade
func (uc *InventoryUseCase) CancelDecreaseStock(ctx context.Context, req TCCActionRequest) error {
	log.Printf("🔄 [CANCEL] Cancel stock reservation: ProductID=%s, Quantity=1, OrderID=%s",
		req.ProductID, req.OrderID)

	// Inicia transação
	tx, err := uc.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Obtém o produto com lock pessimista
	_, err = uc.repository.GetProductForUpdate(ctx, tx, req.ProductID)
	if err != nil {
		return err
	}

	// Verifica idempotência - se já existe movimentação com status rejected
	status, err := uc.repository.GetInventoryMovementStatusByOrderID(ctx, tx, req.OrderID)
	if err != nil {
		return err
	}

	// there is nothing to cancel
	if status == "" {
		log.Printf("ℹ️ [CANCEL] there is nothing to cancel for OrderID=%s", req.OrderID)
		return nil
	}

	// idempotency
	if status == "rejected" {
		return nil
	}

	if status == "completed" {
		return errors.New("invalid status to cancel")
	}

	// Executa o cancelamento
	if err := uc.repository.CancelReserveStock(ctx, tx, req.ProductID, req.OrderID); err != nil {
		log.Printf("❌ [CANCEL] Failed to release stock: %v", err)
		return err
	}

	// Commit da transação
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// Erros customizados
var (
	ErrInsufficientStock = &InventoryError{Message: "insufficient stock"}
)

type InventoryError struct {
	Message string
}

func (e *InventoryError) Error() string {
	return e.Message
}
