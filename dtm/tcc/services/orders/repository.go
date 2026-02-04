package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresOrderRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOrderRepository(pool *pgxpool.Pool) *PostgresOrderRepository {
	return &PostgresOrderRepository{pool: pool}
}

func (r *PostgresOrderRepository) CreateOrder(ctx context.Context, order *Order) error {
	query := `
		INSERT INTO orders (order_id, user_id, product_id, total_price, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.pool.Exec(ctx, query,
		order.OrderID,
		order.UserID,
		order.ProductID,
		order.TotalPrice,
		order.Status,
		order.CreatedAt,
		order.UpdatedAt,
	)
	return err
}

func (r *PostgresOrderRepository) GetOrderByID(ctx context.Context, orderID string) (*Order, error) {
	query := `
		SELECT order_id, user_id, product_id, total_price, status, created_at, updated_at
		FROM orders
		WHERE order_id = $1
	`
	var order Order
	err := r.pool.QueryRow(ctx, query, orderID).Scan(
		&order.OrderID,
		&order.UserID,
		&order.ProductID,
		&order.TotalPrice,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *PostgresOrderRepository) UpdateOrderStatus(ctx context.Context, orderID string, status string) error {
	query := `
		UPDATE orders
		SET status = $1, updated_at = NOW()
		WHERE order_id = $2
	`
	_, err := r.pool.Exec(ctx, query, status, orderID)
	return err
}
