package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresInventoryRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresInventoryRepository(pool *pgxpool.Pool) *PostgresInventoryRepository {
	return &PostgresInventoryRepository{pool: pool}
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
func (r *PostgresInventoryRepository) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &PostgresTx{tx: tx}, nil
}

// GetProductForUpdate obtém o produto com lock pessimista (FOR UPDATE)
func (r *PostgresInventoryRepository) GetProductForUpdate(ctx context.Context, tx Tx, productID string) (*ProductInventory, error) {
	pgTx := tx.(*PostgresTx).tx

	query := `
		SELECT product_id, product_name, current_stock, stock_available, created_at, updated_at
		FROM products_inventory
		WHERE product_id = $1
		FOR UPDATE
	`

	var product ProductInventory
	err := pgTx.QueryRow(ctx, query, productID).Scan(
		&product.ProductID,
		&product.ProductName,
		&product.CurrentStock,
		&product.StockAvailable,
		&product.CreatedAt,
		&product.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("product not found: %s", productID)
		}
		return nil, fmt.Errorf("failed to get product for update: %w", err)
	}

	return &product, nil
}

// TryReserveStock reserva 1 unidade do estoque (TCC TRY) com lock pessimista
func (r *PostgresInventoryRepository) TryReserveStock(ctx context.Context, tx Tx, productID, orderID string) error {
	pgTx := tx.(*PostgresTx).tx

	// Atualiza stock_available (sempre 1 unidade)
	updateQuery := `
		UPDATE products_inventory
		SET stock_available = stock_available - 1,
			updated_at = NOW()
		WHERE product_id = $1
	`
	_, err := pgTx.Exec(ctx, updateQuery, productID)
	if err != nil {
		return fmt.Errorf("failed to update stock_available: %w", err)
	}

	// Cria registro de movimentação com status pending
	movementQuery := `
		INSERT INTO inventory_movements (product_id, order_id, movement_type, status, created_at)
		VALUES ($1, $2, 'decrease', 'pending', NOW())
	`
	_, err = pgTx.Exec(ctx, movementQuery, productID, orderID)
	if err != nil {
		return fmt.Errorf("failed to create movement: %w", err)
	}

	log.Printf("✅ [TRY] Reserved 1 unit of %s", productID)
	return nil
}

// GetInventoryMovementStatusByOrderID retorna o status da movimentação baseado no orderID
func (r *PostgresInventoryRepository) GetInventoryMovementStatusByOrderID(ctx context.Context, tx Tx, orderID string) (string, error) {
	pgTx := tx.(*PostgresTx).tx

	query := `
		SELECT status
		FROM inventory_movements
		WHERE order_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var status string
	err := pgTx.QueryRow(ctx, query, orderID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to query inventory movement status: %w", err)
	}

	return status, nil
}

// GetInventoryMovementByOrderIDAndStatus verifica se existe movimentação com orderID e status específicos
func (r *PostgresInventoryRepository) GetInventoryMovementByOrderIDAndStatus(ctx context.Context, tx Tx, orderID, status string) (bool, error) {
	pgTx := tx.(*PostgresTx).tx

	query := `
		SELECT movement_id
		FROM inventory_movements
		WHERE order_id = $1 AND status = $2
	`

	var id int64
	err := pgTx.QueryRow(ctx, query, orderID, status).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to query inventory movements by status: %w", err)
	}

	return true, nil
}

// ConfirmReserveStock confirma a venda de 1 unidade (TCC CONFIRM) com lock pessimista
func (r *PostgresInventoryRepository) ConfirmReserveStock(ctx context.Context, tx Tx, productID, orderID string) error {
	pgTx := tx.(*PostgresTx).tx

	// Atualiza current_stock (sempre 1 unidade)
	updateQuery := `
		UPDATE products_inventory
		SET current_stock = current_stock - 1,
			updated_at = NOW()
		WHERE product_id = $1
	`
	_, err := pgTx.Exec(ctx, updateQuery, productID)
	if err != nil {
		return fmt.Errorf("failed to update current_stock: %w", err)
	}

	// Atualiza status da movimentação para completed
	updateStatusQuery := `
		UPDATE inventory_movements
		SET status = 'completed'
		WHERE order_id = $1 AND status = 'pending'
	`
	_, err = pgTx.Exec(ctx, updateStatusQuery, orderID)
	if err != nil {
		return fmt.Errorf("failed to update movement status: %w", err)
	}

	log.Printf("✅ [CONFIRM] Decreased 1 unit of %s", productID)
	return nil
}

// CancelReserveStock cancela a reserva de 1 unidade (TCC CANCEL) com lock pessimista
func (r *PostgresInventoryRepository) CancelReserveStock(ctx context.Context, tx Tx, productID, orderID string) error {
	pgTx := tx.(*PostgresTx).tx

	// Atualiza stock_available (devolve a unidade reservada)
	updateQuery := `
		UPDATE products_inventory
		SET stock_available = stock_available + 1,
			updated_at = NOW()
		WHERE product_id = $1
	`
	_, err := pgTx.Exec(ctx, updateQuery, productID)
	if err != nil {
		return fmt.Errorf("failed to update stock_available: %w", err)
	}

	// Atualiza status da movimentação para rejected
	updateStatusQuery := `
		UPDATE inventory_movements
		SET status = 'rejected'
		WHERE order_id = $1 AND status = 'pending'
	`
	_, err = pgTx.Exec(ctx, updateStatusQuery, orderID)
	if err != nil {
		return fmt.Errorf("failed to update movement status: %w", err)
	}

	log.Printf("✅ [CANCEL] Released 1 unit of %s", productID)
	return nil
}
