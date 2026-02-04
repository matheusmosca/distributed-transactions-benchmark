package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PaymentRepository define a interface para operações de banco de dados de pagamentos
type PaymentRepository interface {
	GetWalletByUserID(ctx context.Context, userID string) (*Wallet, error)
	GetWalletForUpdate(ctx context.Context, tx Tx, userID string) (*Wallet, error)
	GetPaymentByOrderIDAndType(ctx context.Context, tx Tx, orderID string, paymentType string) (bool, error)
	DebitWallet(ctx context.Context, tx Tx, userID string, orderID string, amount int) error
	CreditWallet(ctx context.Context, tx Tx, userID string, orderID string, amount int) error
	BeginTx(ctx context.Context) (Tx, error)
}

// Tx interface para transações
type Tx interface {
	Commit() error
	Rollback() error
}

// PostgresPaymentRepository implementa PaymentRepository usando PostgreSQL
type PostgresPaymentRepository struct {
	db *pgxpool.Pool
}

// NewPaymentRepository cria uma nova instância de PostgresPaymentRepository
func NewPaymentRepository(db *pgxpool.Pool) PaymentRepository {
	return &PostgresPaymentRepository{
		db: db,
	}
}

// GetWalletByUserID busca a carteira do usuário
func (r *PostgresPaymentRepository) GetWalletByUserID(ctx context.Context, userID string) (*Wallet, error) {
	var wallet Wallet
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, current_amount, created_at, updated_at
		FROM wallets 
		WHERE user_id = $1
	`, userID).Scan(&wallet.ID, &wallet.UserID, &wallet.CurrentAmount, &wallet.CreatedAt, &wallet.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return &wallet, nil
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
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &PostgresTx{tx: tx}, nil
}

// GetWalletForUpdate obtém a carteira com lock pessimista (FOR UPDATE)
func (r *PostgresPaymentRepository) GetWalletForUpdate(ctx context.Context, tx Tx, userID string) (*Wallet, error) {
	pgTx := tx.(*PostgresTx).tx

	query := `
		SELECT id, user_id, current_amount, created_at, updated_at
		FROM wallets
		WHERE user_id = $1
		FOR UPDATE
	`

	var wallet Wallet
	err := pgTx.QueryRow(ctx, query, userID).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.CurrentAmount,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get wallet with lock: %w", err)
	}

	return &wallet, nil
}

// DebitWallet debita um valor da carteira e registra o pagamento
func (r *PostgresPaymentRepository) DebitWallet(ctx context.Context, tx Tx, userID string, orderID string, amount int) error {
	pgTx := tx.(*PostgresTx).tx

	// 1. Atualiza o saldo da carteira
	updateQuery := `
		UPDATE wallets
		SET current_amount = current_amount - $1,
		    updated_at = NOW()
		WHERE user_id = $2
	`

	_, err := pgTx.Exec(ctx, updateQuery, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to debit wallet: %w", err)
	}

	// 2. Busca o wallet_id
	var walletID string
	err = pgTx.QueryRow(ctx, `SELECT id FROM wallets WHERE user_id = $1`, userID).Scan(&walletID)
	if err != nil {
		return fmt.Errorf("failed to get wallet id: %w", err)
	}

	// 3. Insere o registro de pagamento
	paymentID := uuid.New().String()
	insertQuery := `
		INSERT INTO user_payments (id, wallet_id, order_id, amount, type)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err = pgTx.Exec(ctx, insertQuery, paymentID, walletID, orderID, amount, "debit")
	if err != nil {
		return fmt.Errorf("failed to insert payment record: %w", err)
	}

	return nil
}

// CreditWallet credita um valor na carteira e registra o pagamento
func (r *PostgresPaymentRepository) CreditWallet(ctx context.Context, tx Tx, userID string, orderID string, amount int) error {
	pgTx := tx.(*PostgresTx).tx

	// 1. Atualiza o saldo da carteira
	updateQuery := `
		UPDATE wallets
		SET current_amount = current_amount + $1,
		    updated_at = NOW()
		WHERE user_id = $2
	`

	_, err := pgTx.Exec(ctx, updateQuery, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to credit wallet: %w", err)
	}

	// 2. Busca o wallet_id
	var walletID string
	err = pgTx.QueryRow(ctx, `SELECT id FROM wallets WHERE user_id = $1`, userID).Scan(&walletID)
	if err != nil {
		return fmt.Errorf("failed to get wallet id: %w", err)
	}

	// 3. Insere o registro de pagamento
	paymentID := uuid.New().String()
	insertQuery := `
		INSERT INTO user_payments (id, wallet_id, order_id, amount, type)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err = pgTx.Exec(ctx, insertQuery, paymentID, walletID, orderID, amount, "credit")
	if err != nil {
		return fmt.Errorf("failed to insert payment record: %w", err)
	}

	return nil
}

// GetPaymentByOrderIDAndType verifica se já existe um pagamento para o order_id e tipo especificados
func (r *PostgresPaymentRepository) GetPaymentByOrderIDAndType(ctx context.Context, tx Tx, orderID string, paymentType string) (bool, error) {
	pgTx := tx.(*PostgresTx).tx

	query := `
		SELECT EXISTS(
			SELECT 1 FROM user_payments 
			WHERE order_id = $1 AND type = $2
		)
	`

	var exists bool
	err := pgTx.QueryRow(ctx, query, orderID, paymentType).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// abs retorna o valor absoluto de um inteiro
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
