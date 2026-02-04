package main

import (
	"database/sql"
	"log"
)

// PaymentUseCase encapsula a lógica de negócio de pagamentos (2PC/XA)
type PaymentUseCase struct {
	repository PaymentRepository
}

// NewPaymentUseCase cria uma nova instância do caso de uso
func NewPaymentUseCase(repository PaymentRepository) *PaymentUseCase {
	return &PaymentUseCase{
		repository: repository,
	}
}

// DebitWalletXA debita o saldo (XA)
// Recebe *sql.DB do DTM que já está em transação XA
func (uc *PaymentUseCase) DebitWalletXA(db *sql.DB, req XAActionRequest) error {
	log.Printf("💳 [XA] Debit wallet: UserID=%s, Amount=%d, OrderID=%s",
		req.UserID, req.TotalPrice, req.OrderID)

	if req.TotalPrice <= 0 {
		return ErrInvalidAmount
	}

	if err := uc.repository.DebitBalanceXA(db, req.UserID, req.OrderID, req.TotalPrice); err != nil {
		log.Printf("❌ [XA] Failed to debit wallet: %v", err)
		return err
	}

	return nil
}

// Erros customizados
var (
	ErrInvalidAmount       = &PaymentError{Message: "amount must be greater than 0"}
	ErrInsufficientBalance = &PaymentError{Message: "insufficient balance"}
)

type PaymentError struct {
	Message string
}

func (e *PaymentError) Error() string {
	return e.Message
}
