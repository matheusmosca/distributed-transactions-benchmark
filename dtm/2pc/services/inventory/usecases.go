package main

import (
	"database/sql"
	"log"
)

// InventoryUseCase encapsula a lógica de negócio de inventário (2PC/XA)
type InventoryUseCase struct {
	repository InventoryRepository
}

// NewInventoryUseCase cria uma nova instância do caso de uso
func NewInventoryUseCase(repository InventoryRepository) *InventoryUseCase {
	return &InventoryUseCase{
		repository: repository,
	}
}

// DecreaseStockXA decrementa 1 unidade do estoque (XA)
// Recebe *sql.DB do DTM que já está em transação XA
func (uc *InventoryUseCase) DecreaseStockXA(db *sql.DB, req XAActionRequest) error {
	log.Printf("📦 [XA] Decrease stock: ProductID=%s, Quantity=1, OrderID=%s",
		req.ProductID, req.OrderID)

	if err := uc.repository.DecreaseStockXA(db, req.ProductID, req.OrderID); err != nil {
		log.Printf("❌ [XA] Failed to decrease stock: %v", err)
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
