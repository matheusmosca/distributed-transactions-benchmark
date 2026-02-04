package main

import (
	"database/sql"
	"time"
)

// ProductInventory representa um produto no inventário (2PC/XA)
// SEM stock_available e SEM version (não precisa de optimistic locking)
type ProductInventory struct {
	ProductID    string    `json:"product_id"`
	ProductName  string    `json:"product_name"`
	CurrentStock int       `json:"current_stock"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// XAActionRequest representa o payload das requisições XA (sempre 1 unidade)
type XAActionRequest struct {
	OrderID    string `json:"order_id"`
	UserID     string `json:"user_id"`
	ProductID  string `json:"product_id"`
	TotalPrice int    `json:"total_price"`
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
}

// InventoryRepository define as operações de persistência de inventário (sempre 1 unidade)
type InventoryRepository interface {
	// XA: Decrementa 1 unidade do estoque dentro de transação XA (recebe *sql.DB do DTM)
	DecreaseStockXA(db *sql.DB, productID, orderID string) error
}
