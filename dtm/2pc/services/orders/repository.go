package main

import (
	"database/sql"
	"fmt"
	"log"
)

type PostgresOrderRepository struct{}

func NewPostgresOrderRepository() *PostgresOrderRepository {
	return &PostgresOrderRepository{}
}

// CreateOrderXA cria ordem dentro de transação XA
// DTM gerencia PREPARE/COMMIT automaticamente via XaLocalTransaction
// Recebe *sql.DB que já está dentro de uma transação XA gerenciada pelo DTM
func (r *PostgresOrderRepository) CreateOrderXA(db *sql.DB, order *Order) error {
	query := `
		INSERT INTO orders (order_id, user_id, product_id, total_price, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := db.Exec(query,
		order.OrderID,
		order.UserID,
		order.ProductID,
		order.TotalPrice,
		order.Status,
		order.CreatedAt,
		order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	log.Printf("✅ [XA] Created order %s with status '%s'", order.OrderID, order.Status)
	return nil
}
