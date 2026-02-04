package main

import (
	"context"
	"time"
)

// ProductInventory representa um produto no inventário
type ProductInventory struct {
	ProductID      string    `json:"product_id"`
	ProductName    string    `json:"product_name"`
	CurrentStock   int       `json:"current_stock"`
	StockAvailable int       `json:"stock_available"` // TCC: estoque disponível para venda
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TCCActionRequest representa o payload das requisições TCC (sempre 1 unidade)
type TCCActionRequest struct {
	OrderID    string `json:"order_id"`
	UserID     string `json:"user_id"`
	ProductID  string `json:"product_id"`
	TotalPrice int    `json:"total_price"`
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
}

// InventoryMovement representa um movimento de estoque (entrada/saída)
type InventoryMovement struct {
	ID           string    `json:"id"`
	ProductID    string    `json:"product_id"`
	OrderID      string    `json:"order_id"`
	MovementType string    `json:"movement_type"` // exemplo: "reserve", "confirm", "cancel"
	CreatedAt    time.Time `json:"created_at"`
}

// InventoryRepository define as operações de persistência de inventário (sempre 1 unidade)
type InventoryRepository interface {
	// Gerenciamento de transação
	BeginTx(ctx context.Context) (Tx, error)

	// Lock pessimista
	GetProductForUpdate(ctx context.Context, tx Tx, productID string) (*ProductInventory, error)

	// TRY: Reserva 1 unidade do estoque (decrementa stock_available)
	TryReserveStock(ctx context.Context, tx Tx, productID, orderID string) error

	// CONFIRM: Confirma a venda de 1 unidade (decrementa current_stock)
	ConfirmReserveStock(ctx context.Context, tx Tx, productID, orderID string) error

	// CANCEL: Cancela a reserva de 1 unidade (incrementa stock_available)
	CancelReserveStock(ctx context.Context, tx Tx, productID, orderID string) error

	// Verificações de idempotência (dentro da transação)
	GetInventoryMovementByOrderIDAndStatus(ctx context.Context, tx Tx, orderID, status string) (bool, error)
	GetInventoryMovementStatusByOrderID(ctx context.Context, tx Tx, orderID string) (string, error)
}

// Tx representa uma transação de banco de dados
type Tx interface {
	Commit() error
	Rollback() error
}
