package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresPaymentRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresPaymentRepository(pool *pgxpool.Pool) *PostgresPaymentRepository {
	return &PostgresPaymentRepository{pool: pool}
}

// PostgresTx implementa a interface Tx
type PostgresTx struct {
	tx pgx.Tx
}

func (t *PostgresTx) Commit() error {
	return t.tx.Commit(context.Background())
}

func (t *PostgresTx) Rollback() error {
	return t.tx.Rollback(context.Background())
}

// BeginTx inicia uma nova transação
func (r *PostgresPaymentRepository) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &PostgresTx{tx: tx}, nil
}

// GetWalletForUpdate obtém a carteira com lock pessimista (FOR UPDATE)
func (r *PostgresPaymentRepository) GetWalletForUpdate(ctx context.Context, tx Tx, userID string) (*Wallet, error) {
	pgTx := tx.(*PostgresTx).tx

	query := `
		SELECT user_id, current_amount, available_amount, created_at, updated_at
		FROM wallets
		WHERE user_id = $1
		FOR UPDATE
	`

	var wallet Wallet
	err := pgTx.QueryRow(ctx, query, userID).Scan(
		&wallet.UserID,
		&wallet.CurrentAmount,
		&wallet.AvailableAmount,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %s", userID)
		}
		return nil, fmt.Errorf("failed to get wallet for update: %w", err)
	}

	return &wallet, nil
}

// TryReserveBalance reserva o saldo (TCC TRY) com lock pessimista
func (r *PostgresPaymentRepository) TryReserveBalance(ctx context.Context, tx Tx, userID, orderID string, amount int) error {
	pgTx := tx.(*PostgresTx).tx

	// Atualiza available_amount
	updateQuery := `
		UPDATE wallets
		SET available_amount = available_amount - $1,
			updated_at = NOW()
		WHERE user_id = $2
	`
	_, err := pgTx.Exec(ctx, updateQuery, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to update available_amount: %w", err)
	}

	// Cria registro de transação com status pending
	transactionQuery := `
		INSERT INTO payment_transactions (user_id, order_id, amount, transaction_type, status, created_at)
		VALUES ($1, $2, $3, 'debit', 'pending', NOW())
	`
	_, err = pgTx.Exec(ctx, transactionQuery, userID, orderID, amount)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	log.Printf("✅ [TRY] Reserved balance %d for user %s", amount, userID)
	return nil
}

// ConfirmDebit confirma o débito (TCC CONFIRM) com lock pessimista
func (r *PostgresPaymentRepository) ConfirmDebit(ctx context.Context, tx Tx, userID, orderID string, amount int) error {
	pgTx := tx.(*PostgresTx).tx

	// Atualiza current_amount
	updateQuery := `
		UPDATE wallets
		SET current_amount = current_amount - $1,
			updated_at = NOW()
		WHERE user_id = $2
	`
	_, err := pgTx.Exec(ctx, updateQuery, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to update current_amount: %w", err)
	}

	// Atualiza status da transação para completed
	updateStatusQuery := `
		UPDATE payment_transactions
		SET status = 'completed'
		WHERE order_id = $1 AND status = 'pending'
	`
	_, err = pgTx.Exec(ctx, updateStatusQuery, orderID)
	if err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	log.Printf("✅ [CONFIRM] Debited %d from user %s", amount, userID)
	return nil
}

// CancelReserveBalance cancela a reserva (TCC CANCEL) com lock pessimista
func (r *PostgresPaymentRepository) CancelReserveBalance(ctx context.Context, tx Tx, userID, orderID string, amount int) error {
	pgTx := tx.(*PostgresTx).tx

	// Atualiza available_amount (devolve o valor reservado)
	updateQuery := `
		UPDATE wallets
		SET available_amount = available_amount + $1,
			updated_at = NOW()
		WHERE user_id = $2
	`
	_, err := pgTx.Exec(ctx, updateQuery, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to update available_amount: %w", err)
	}

	// Atualiza status da transação para rejected
	updateStatusQuery := `
		UPDATE payment_transactions
		SET status = 'rejected'
		WHERE order_id = $1 AND status = 'pending'
	`
	_, err = pgTx.Exec(ctx, updateStatusQuery, orderID)
	if err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	log.Printf("✅ [CANCEL] Released balance %d for user %s", amount, userID)
	return nil
}

// GetPaymentTransactionByOrderIDAndStatus verifica se existe transação com orderID e status específicos
func (r *PostgresPaymentRepository) GetPaymentTransactionByOrderIDAndStatus(ctx context.Context, tx Tx, orderID, status string) (bool, error) {
	pgTx := tx.(*PostgresTx).tx

	query := `
		SELECT transaction_id
		FROM payment_transactions
		WHERE order_id = $1 AND status = $2
	`

	var id int64
	err := pgTx.QueryRow(ctx, query, orderID, status).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to query payment transactions by status: %w", err)
	}

	return true, nil
}

// GetPaymentTransactionStatusByOrderID retorna o status da transação pelo orderID
func (r *PostgresPaymentRepository) GetPaymentTransactionStatusByOrderID(ctx context.Context, tx Tx, orderID string) (string, error) {
	pgTx := tx.(*PostgresTx).tx

	query := `
		SELECT status
		FROM payment_transactions
		WHERE order_id = $1
	`

	var status string
	err := pgTx.QueryRow(ctx, query, orderID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil // Sem transação encontrada
		}
		return "", fmt.Errorf("failed to query payment transaction status: %w", err)
	}

	return status, nil
}
