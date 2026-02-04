package main

import (
	"context"
	"fmt"
	"log"
)

// PaymentUseCase encapsula a lógica de negócio de pagamentos
type PaymentUseCase struct {
	repository PaymentRepository
}

// NewPaymentUseCase cria uma nova instância do caso de uso
func NewPaymentUseCase(repository PaymentRepository) *PaymentUseCase {
	return &PaymentUseCase{
		repository: repository,
	}
}

// TryDebitWallet fase TRY do TCC - reserva o saldo
func (uc *PaymentUseCase) TryDebitWallet(ctx context.Context, req TCCActionRequest) error {
	log.Printf("💳 [TRY] Reserve balance: UserID=%s, Amount=%d, OrderID=%s",
		req.UserID, req.TotalPrice, req.OrderID)

	if req.TotalPrice <= 0 {
		return ErrInvalidAmount
	}

	// Inicia transação
	tx, err := uc.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Obtém a carteira com lock pessimista
	wallet, err := uc.repository.GetWalletForUpdate(ctx, tx, req.UserID)
	if err != nil {
		log.Printf("❌ TRY FAILED: GetWalletForUpdate | OrderID=%s | Error=%v", req.OrderID, err)
		return err
	}

	// Verifica idempotência - se já existe transação com status pending
	exists, err := uc.repository.GetPaymentTransactionByOrderIDAndStatus(ctx, tx, req.OrderID, "pending")
	if err != nil {
		log.Printf("❌ TRY FAILED: GetPaymentTransactionByOrderIDAndStatus | OrderID=%s | Error=%v", req.OrderID, err)

		return err
	}

	if exists {
		log.Printf("ℹ️ [TRY] Payment transaction already pending for OrderID=%s", req.OrderID)
		return nil
	}

	// Valida saldo disponível
	if wallet.AvailableAmount < req.TotalPrice {
		log.Printf("❌ [TRY] Insufficient balance for user %s: available=%d, required=%d", req.UserID, wallet.AvailableAmount, req.TotalPrice)
		return ErrInsufficientBalance
	}

	// Executa a reserva
	if err := uc.repository.TryReserveBalance(ctx, tx, req.UserID, req.OrderID, req.TotalPrice); err != nil {
		log.Printf("❌ [TRY] Failed to reserve balance: %v", err)
		return err
	}

	// Commit da transação
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// ConfirmDebitWallet fase CONFIRM do TCC - confirma o débito
func (uc *PaymentUseCase) ConfirmDebitWallet(ctx context.Context, req TCCActionRequest) error {
	log.Printf("✅ [CONFIRM] Confirm debit: UserID=%s, Amount=%d, OrderID=%s",
		req.UserID, req.TotalPrice, req.OrderID)

	// Inicia transação
	tx, err := uc.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Obtém a carteira com lock pessimista
	_, err = uc.repository.GetWalletForUpdate(ctx, tx, req.UserID)
	if err != nil {
		log.Printf("❌ CONFIRM FAILED: GetWalletForUpdate | OrderID=%s | Error=%v", req.OrderID, err)

		return err
	}

	// Verifica idempotência - se já existe transação com status completed
	exists, err := uc.repository.GetPaymentTransactionByOrderIDAndStatus(ctx, tx, req.OrderID, "completed")
	if err != nil {
		log.Printf("❌ CONFIRM FAILED: GetPaymentTransactionByOrderIDAndStatus | OrderID=%s | Error=%v", req.OrderID, err)

		return err
	}

	if exists {
		log.Printf("ℹ️ [CONFIRM] Payment transaction already completed for OrderID=%s", req.OrderID)
		return nil
	}

	// Executa a confirmação
	if err := uc.repository.ConfirmDebit(ctx, tx, req.UserID, req.OrderID, req.TotalPrice); err != nil {
		log.Printf("❌ [CONFIRM] | OrderID=%s Failed to debit: %v", req.OrderID, err)
		return err
	}

	// Commit da transação
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// CancelDebitWallet fase CANCEL do TCC - cancela a reserva
func (uc *PaymentUseCase) CancelDebitWallet(ctx context.Context, req TCCActionRequest) error {
	log.Printf("🔄 [CANCEL] Cancel balance reservation: UserID=%s, Amount=%d, OrderID=%s",
		req.UserID, req.TotalPrice, req.OrderID)

	// Inicia transação
	tx, err := uc.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Obtém a carteira com lock pessimista
	_, err = uc.repository.GetWalletForUpdate(ctx, tx, req.UserID)
	if err != nil {
		log.Printf("❌ CANCEL FAILED: GetWalletForUpdate | OrderID=%s | Error=%v", req.OrderID, err)

		return err
	}

	// Verifica o status atual da transação
	status, err := uc.repository.GetPaymentTransactionStatusByOrderID(ctx, tx, req.OrderID)
	if err != nil {
		log.Printf("❌ CANCEL FAILED: GetPaymentTransactionStatusByOrderID | OrderID=%s | Error=%v", req.OrderID, err)
		return err
	}

	// Se não encontrar registro na tabela pelo order_id, pode retornar nil
	if status == "" {
		log.Printf("ℹ️ [CANCEL] No payment transaction found for OrderID=%s", req.OrderID)
		return nil
	}

	// Se já foi rejeitado, idempotência
	if status == "rejected" {
		log.Printf("ℹ️ [CANCEL] Payment transaction already rejected for OrderID=%s", req.OrderID)
		return nil
	}

	// Se foi completado, retorna erro pois não pode ser revertido
	if status == "completed" {
		log.Printf("❌ [CANCEL] Cannot cancel completed transaction for OrderID=%s", req.OrderID)
		return fmt.Errorf("cannot cancel completed transaction for order %s", req.OrderID)
	}

	// Se está pending, pode cancelar
	if err := uc.repository.CancelReserveBalance(ctx, tx, req.UserID, req.OrderID, req.TotalPrice); err != nil {
		log.Printf("❌ [CANCEL] ORDER_ID %s | Failed to release balance: %v", req.OrderID, err)
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
	ErrInvalidAmount       = &PaymentError{Message: "amount must be greater than 0"}
	ErrInsufficientBalance = &PaymentError{Message: "insufficient balance"}
)

type PaymentError struct {
	Message string
}

func (e *PaymentError) Error() string {
	return e.Message
}
