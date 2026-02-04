package main

import (
	"database/sql"
	"time"
)

// Wallet representa uma carteira de usuário (2PC/XA)
// SEM available_amount e SEM version (não precisa de optimistic locking)
type Wallet struct {
	UserID        string    `json:"user_id"`
	CurrentAmount int       `json:"current_amount"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// XAActionRequest representa o payload das requisições XA
type XAActionRequest struct {
	OrderID    string `json:"order_id"`
	UserID     string `json:"user_id"`
	ProductID  string `json:"product_id"`
	TotalPrice int    `json:"total_price"`
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
}

// PaymentRepository define as operações de persistência de pagamentos
type PaymentRepository interface {
	// XA: Debita o saldo dentro de transação XA (recebe *sql.DB do DTM)
	DebitBalanceXA(db *sql.DB, userID, orderID string, amount int) error
}
