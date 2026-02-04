package main

import (
	"time"
)

// Wallet representa uma carteira de usuário
type Wallet struct {
	ID            string    `json:"id" db:"id"`
	UserID        string    `json:"user_id" db:"user_id"`
	CurrentAmount int       `json:"current_amount" db:"current_amount"`
	Version       int       `json:"version" db:"version"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// NewWallet cria uma nova instância de Wallet
func NewWallet(id, userID string, initialAmount int) *Wallet {
	return &Wallet{
		ID:            id,
		UserID:        userID,
		CurrentAmount: initialAmount,
		Version:       1,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// UserPayment representa um pagamento do usuário
type UserPayment struct {
	ID        string    `json:"id" db:"id"`
	WalletID  string    `json:"wallet_id" db:"wallet_id"`
	Amount    int       `json:"amount" db:"amount"`
	Type      string    `json:"type" db:"type"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// NewUserPayment cria uma nova instância de UserPayment
func NewUserPayment(id, walletID string, amount int, paymentType string) *UserPayment {
	return &UserPayment{
		ID:        id,
		WalletID:  walletID,
		Amount:    amount,
		Type:      paymentType,
		CreatedAt: time.Now(),
	}
}

// PaymentType representa os tipos de pagamento
const (
	PaymentTypeDebit  = "debit"
	PaymentTypeCredit = "credit"
)
