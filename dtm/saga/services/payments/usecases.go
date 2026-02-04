package main

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel/metric"
)

var (
	ErrUnprocessableEntity = fmt.Errorf("unprocessable entity")
)

// PaymentUseCase contém a lógica de negócio de pagamentos
type PaymentUseCase struct {
	repository                   PaymentRepository
	paymentDebitCounter          metric.Int64Counter
	paymentCompensationCounter   metric.Int64Counter
	paymentOptimisticLockRetries metric.Int64Counter
}

// NewPaymentUseCase cria uma nova instância de PaymentUseCase
func NewPaymentUseCase(
	repository PaymentRepository,
) *PaymentUseCase {
	return &PaymentUseCase{
		repository: repository,
	}
}

// DebitPayment debita um valor da carteira do usuário usando Lock Pessimista
func (uc *PaymentUseCase) DebitPayment(ctx context.Context, req SagaActionRequest) error {
	log.Printf("➡️ [DEBIT PAYMENT] TraceID: %s | OrderID: %s | UserID: %s | Amount: %d",
		req.TraceID, req.OrderID, req.UserID, req.Amount)

	// 1. Inicia a transação
	tx, err := uc.repository.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback()

	// 2. Obtém a carteira com LOCK PESSIMISTA (SELECT FOR UPDATE)
	// Isso bloqueia a linha no banco até o Commit ou Rollback
	wallet, err := uc.repository.GetWalletForUpdate(ctx, tx, req.UserID)
	if err != nil {
		log.Printf("❌ DEBIT FAILED: GetWalletForUpdate | OrderID=%s | Error=%v", req.OrderID, err)
		return err
	}

	// 3. Verificar idempotência dentro da transação
	// Buscamos se já existe um registro de débito para este OrderID
	exists, err := uc.repository.GetPaymentByOrderIDAndType(ctx, tx, req.OrderID, "debit")
	if err != nil {
		return fmt.Errorf("erro ao verificar idempotência: %w", err)
	}

	if exists {
		log.Printf("ℹ️ [IDEMPOTENCY] Débito já realizado para OrderID=%s", req.OrderID)
		return nil // Retorna sucesso pois o trabalho já foi feito
	}

	// 4. Regra de Negócio: Verifica saldo
	if wallet.CurrentAmount < req.Amount {
		log.Printf("❌ DEBIT FAILED: Insufficient funds | UserID=%s", req.UserID)
		return fmt.Errorf("insufficient funds")
	}

	// 5. Executa a atualização do saldo e cria o registro de pagamento
	// Como estamos com Lock Pessimista, não precisamos checar 'version' no WHERE
	if err := uc.repository.DebitWallet(ctx, tx, req.UserID, req.OrderID, req.Amount); err != nil {
		log.Printf("❌ [DEBIT] | OrderID=%s Failed to update: %v", req.OrderID, err)
		return err
	}

	// 6. Commit da transação
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao comitar débito: %w", err)
	}

	log.Printf("✅ [DEBIT] Success: OrderID=%s", req.OrderID)
	return nil
}

// CompensatePayment credita um valor de volta na carteira do usuário (compensação) com idempotência e lock pessimista
func (uc *PaymentUseCase) CompensatePayment(ctx context.Context, req SagaActionRequest) error {
	log.Printf("↩️ [COMPENSATE PAYMENT] TraceID: %s | OrderID: %s | UserID: %s | Amount: %d",
		req.TraceID, req.OrderID, req.UserID, req.Amount)

	// 1. Inicia a transação
	tx, err := uc.repository.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback()

	// 2. Obtém a carteira com LOCK PESSIMISTA (SELECT FOR UPDATE)
	_, err = uc.repository.GetWalletForUpdate(ctx, tx, req.UserID)
	if err != nil {
		log.Printf("❌ COMPENSATE FAILED: GetWalletForUpdate | OrderID=%s | Error=%v", req.OrderID, err)
		return err
	}

	// 3. Verificar idempotência - se já existe pagamento de 'credit' para este order_id
	exists, err := uc.repository.GetPaymentByOrderIDAndType(ctx, tx, req.OrderID, "credit")
	if err != nil {
		return fmt.Errorf("erro ao verificar idempotência: %w", err)
	}

	if exists {
		log.Printf("ℹ️  [IDEMPOTENCY] Pagamento de compensação já processado para OrderID=%s", req.OrderID)
		return nil
	}

	// 4. Executa a compensação (crédito) e cria o registro de pagamento
	if err := uc.repository.CreditWallet(ctx, tx, req.UserID, req.OrderID, req.Amount); err != nil {
		log.Printf("❌ [COMPENSATE] | OrderID=%s Failed to update: %v", req.OrderID, err)
		return err
	}

	// 5. Commit da transação
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao comitar compensação: %w", err)
	}

	log.Printf("✅ [COMPENSATE] Success: OrderID=%s", req.OrderID)
	return nil
}
