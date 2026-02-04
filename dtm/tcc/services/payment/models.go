package main

import (
	"context"
	"time"
)

// Wallet representa uma carteira de usuário
type Wallet struct {
	UserID          string    `json:"user_id"`
	CurrentAmount   int       `json:"current_amount"`
	AvailableAmount int       `json:"available_amount"` // TCC: saldo disponível
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TCCActionRequest representa o payload das requisições TCC
type TCCActionRequest struct {
	OrderID    string `json:"order_id"`
	UserID     string `json:"user_id"`
	ProductID  string `json:"product_id"`
	Quantity   int    `json:"quantity"`
	TotalPrice int    `json:"total_price"`
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
}

// PaymentRepository define as operações de persistência de pagamentos
type PaymentRepository interface {
	// Gerenciamento de transação
	BeginTx(ctx context.Context) (Tx, error)

	// Lock pessimista
	GetWalletForUpdate(ctx context.Context, tx Tx, userID string) (*Wallet, error)

	// TRY: Reserva o saldo (decrementa available_amount)
	TryReserveBalance(ctx context.Context, tx Tx, userID, orderID string, amount int) error

	// CONFIRM: Confirma o débito (decrementa current_amount, cria transação)
	ConfirmDebit(ctx context.Context, tx Tx, userID, orderID string, amount int) error

	// CANCEL: Cancela a reserva (incrementa available_amount)
	CancelReserveBalance(ctx context.Context, tx Tx, userID, orderID string, amount int) error

	// Métodos para verificação de status das transações (dentro da transação)
	GetPaymentTransactionByOrderIDAndStatus(ctx context.Context, tx Tx, orderID, status string) (bool, error)
	GetPaymentTransactionStatusByOrderID(ctx context.Context, tx Tx, orderID string) (string, error)
}

// Tx representa uma transação de banco de dados
type Tx interface {
	Commit() error
	Rollback() error
}
