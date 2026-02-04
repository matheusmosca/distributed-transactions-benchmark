package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InventoryRepository define a interface para operações de banco de dados de inventário
type InventoryRepository interface {
	GetProductInventory(ctx context.Context, productID string) (*ProductInventory, error)
	GetProductForUpdate(ctx context.Context, tx Tx, productID string) (*ProductInventory, error)
	GetMovementByOrderIDAndType(ctx context.Context, tx Tx, orderID string, movementType string) (bool, error)
	DecreaseStock(ctx context.Context, tx Tx, productID string, orderID string) error
	IncreaseStock(ctx context.Context, tx Tx, productID string, orderID string) error
	BeginTx(ctx context.Context) (Tx, error)
}

// Tx interface para transações
type Tx interface {
	Commit() error
	Rollback() error
}

// PostgresInventoryRepository implementa InventoryRepository usando PostgreSQL
type PostgresInventoryRepository struct {
	db *pgxpool.Pool
}

// NewInventoryRepository cria uma nova instância de PostgresInventoryRepository
func NewInventoryRepository(db *pgxpool.Pool) InventoryRepository {
	return &PostgresInventoryRepository{
		db: db,
	}
}

// GetProductInventory busca o inventário de um produto
func (r *PostgresInventoryRepository) GetProductInventory(ctx context.Context, productID string) (*ProductInventory, error) {
	var inventory ProductInventory
	err := r.db.QueryRow(ctx, `
		SELECT id, current_stock, created_at, updated_at
		FROM products_inventory 
		WHERE id = $1
	`, productID).Scan(&inventory.ID, &inventory.CurrentStock, &inventory.CreatedAt, &inventory.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return &inventory, nil
}

// GetMovementByOrderIDAndType verifica se já existe um movimento para o order_id e tipo especificados
func (r *PostgresInventoryRepository) GetMovementByOrderIDAndType(ctx context.Context, tx Tx, orderID string, movementType string) (bool, error) {
	pgTx := tx.(*PostgresTx).tx

	query := `
		SELECT EXISTS(
			SELECT 1 FROM inventory_movements 
			WHERE order_id = $1 AND movement_type = $2
		)
	`

	var exists bool
	err := pgTx.QueryRow(ctx, query, orderID, movementType).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
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
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &PostgresTx{tx: tx}, nil
}

// GetProductForUpdate obtém o produto com lock pessimista (FOR UPDATE)
func (r *PostgresInventoryRepository) GetProductForUpdate(ctx context.Context, tx Tx, productID string) (*ProductInventory, error) {
	pgTx := tx.(*PostgresTx).tx

	query := `
		SELECT id, current_stock, created_at, updated_at
		FROM products_inventory
		WHERE id = $1
		FOR UPDATE
	`

	var inventory ProductInventory
	err := pgTx.QueryRow(ctx, query, productID).Scan(
		&inventory.ID,
		&inventory.CurrentStock,
		&inventory.CreatedAt,
		&inventory.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get product with lock: %w", err)
	}

	return &inventory, nil
}

// DecreaseStock diminui o estoque e registra o movimento
func (r *PostgresInventoryRepository) DecreaseStock(ctx context.Context, tx Tx, productID string, orderID string) error {
	pgTx := tx.(*PostgresTx).tx

	// 1. Atualiza o estoque do produto
	updateQuery := `
		UPDATE products_inventory
		SET current_stock = current_stock - 1,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := pgTx.Exec(ctx, updateQuery, productID)
	if err != nil {
		return fmt.Errorf("failed to decrease stock: %w", err)
	}

	// 2. Insere o registro de movimentação
	movementID := uuid.New().String()
	insertQuery := `
		INSERT INTO inventory_movements (id, inventory_id, order_id, change_quantity, movement_type)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err = pgTx.Exec(ctx, insertQuery, movementID, productID, orderID, 1, "decreased")
	if err != nil {
		return fmt.Errorf("failed to insert movement record: %w", err)
	}

	return nil
}

// IncreaseStock aumenta o estoque e registra o movimento
func (r *PostgresInventoryRepository) IncreaseStock(ctx context.Context, tx Tx, productID string, orderID string) error {
	pgTx := tx.(*PostgresTx).tx

	// 1. Atualiza o estoque do produto
	updateQuery := `
		UPDATE products_inventory
		SET current_stock = current_stock + 1,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := pgTx.Exec(ctx, updateQuery, productID)
	if err != nil {
		return fmt.Errorf("failed to increase stock: %w", err)
	}

	// 2. Insere o registro de movimentação
	movementID := uuid.New().String()
	insertQuery := `
		INSERT INTO inventory_movements (id, inventory_id, order_id, change_quantity, movement_type)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err = pgTx.Exec(ctx, insertQuery, movementID, productID, orderID, 1, "increased")
	if err != nil {
		return fmt.Errorf("failed to insert movement record: %w", err)
	}

	return nil
}
