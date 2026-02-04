package main

import (
	"database/sql"
	"fmt"
	"log"
)

type PostgresPaymentRepository struct{}

func NewPostgresPaymentRepository() *PostgresPaymentRepository {
	return &PostgresPaymentRepository{}
}

// DebitBalanceXA debita o saldo dentro de transação XA
// DTM gerencia PREPARE/COMMIT automaticamente via XaLocalTransaction
// Recebe *sql.DB que já está dentro de uma transação XA gerenciada pelo DTM
func (r *PostgresPaymentRepository) DebitBalanceXA(db *sql.DB, userID, orderID string, amount int) error {
	// Atualiza current_amount (já dentro de transação XA do DTM)
	updateQuery := `
		UPDATE wallets
		SET current_amount = current_amount - $1,
			updated_at = NOW()
		WHERE user_id = $2
			AND current_amount >= $1
	`
	result, err := db.Exec(updateQuery, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to debit balance: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("insufficient balance or user not found: %s", userID)
	}

	// Cria registro de transação
	transactionQuery := `
		INSERT INTO payment_transactions (user_id, order_id, amount, transaction_type, created_at)
		VALUES ($1, $2, $3, 'debit', NOW())
	`
	_, err = db.Exec(transactionQuery, userID, orderID, amount)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	log.Printf("✅ [XA] Debited %d from user %s", amount, userID)
	return nil
}
