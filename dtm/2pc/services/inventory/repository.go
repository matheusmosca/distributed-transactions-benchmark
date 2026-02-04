package main

import (
	"database/sql"
	"fmt"
	"log"
)

type PostgresInventoryRepository struct{}

func NewPostgresInventoryRepository() *PostgresInventoryRepository {
	return &PostgresInventoryRepository{}
}

// DecreaseStockXA decrementa 1 unidade do estoque dentro de transação XA
// DTM gerencia PREPARE/COMMIT automaticamente via XaLocalTransaction
// Recebe *sql.DB que já está dentro de uma transação XA gerenciada pelo DTM
func (r *PostgresInventoryRepository) DecreaseStockXA(db *sql.DB, productID, orderID string) error {
	// Atualiza current_stock (já dentro de transação XA do DTM)
	updateQuery := `
		UPDATE products_inventory
		SET current_stock = current_stock - 1,
			updated_at = NOW()
		WHERE product_id = $1
			AND current_stock >= 1
	`
	result, err := db.Exec(updateQuery, productID)
	if err != nil {
		return fmt.Errorf("failed to decrease stock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("insufficient stock or product not found: %s", productID)
	}

	// Cria registro de movimentação
	movementQuery := `
		INSERT INTO inventory_movements (product_id, order_id, movement_type, created_at)
		VALUES ($1, $2, 'decrease', NOW())
	`
	_, err = db.Exec(movementQuery, productID, orderID)
	if err != nil {
		return fmt.Errorf("failed to create movement: %w", err)
	}

	log.Printf("✅ [XA] Decreased 1 unit of %s", productID)
	return nil
}
