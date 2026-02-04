package main

import (
	"database/sql"
	"time"
)

// Order representa um pedido no sistema (2PC/XA) - sempre 1 unidade por pedido
type Order struct {
	OrderID    string    `json:"order_id"`
	UserID     string    `json:"user_id"`
	ProductID  string    `json:"product_id"`
	TotalPrice int       `json:"total_price"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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

// CreateOrderRequest representa a requisição de criação de pedido (sempre 1 unidade)
type CreateOrderRequest struct {
	UserID    string `json:"user_id"`
	ProductID string `json:"product_id"`
	Amount    int    `json:"amount"`
}

// OrderRepository define as operações de persistência de pedidos
type OrderRepository interface {
	// XA: Cria ordem dentro de transação XA (recebe *sql.DB do DTM)
	CreateOrderXA(db *sql.DB, order *Order) error
}
