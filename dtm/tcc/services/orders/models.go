package main

import (
	"context"
	"time"
)

// Order representa um pedido no sistema (sempre 1 unidade por pedido)
type Order struct {
	OrderID    string    `json:"order_id"`
	UserID     string    `json:"user_id"`
	ProductID  string    `json:"product_id"`
	TotalPrice int       `json:"total_price"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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

// CreateOrderRequest representa a requisição de criação de pedido (sempre 1 unidade)
type CreateOrderRequest struct {
	UserID     string `json:"user_id"`
	ProductID  string `json:"product_id"`
	TotalPrice int    `json:"amount"`
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
}

// OrderRepository define as operações de persistência de pedidos
type OrderRepository interface {
	CreateOrder(ctx context.Context, order *Order) error
	GetOrderByID(ctx context.Context, orderID string) (*Order, error)
	UpdateOrderStatus(ctx context.Context, orderID string, status string) error
}
