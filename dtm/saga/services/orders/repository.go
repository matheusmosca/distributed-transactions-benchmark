package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository define a interface para operações de banco de dados de pedidos
type Repository interface {
	// OrderExists verifica se um pedido já existe (para idempotência)
	OrderExists(ctx context.Context, orderID string) (bool, error)

	// CreateOrder cria um novo pedido no banco de dados
	CreateOrder(ctx context.Context, order *Order) error

	// UpdateOrderStatus atualiza o status de um pedido
	UpdateOrderStatus(ctx context.Context, orderID string, status string) error

	// GetOrder busca um pedido pelo ID
	GetOrder(ctx context.Context, orderID string) (*Order, error)
}

// OrderRepository implementa Repository usando PostgreSQL
type OrderRepository struct {
	db *pgxpool.Pool
}

// NewOrderRepository cria uma nova instância de OrderRepository
func NewOrderRepository(db *pgxpool.Pool) Repository {
	return &OrderRepository{
		db: db,
	}
}

// OrderExists verifica se um pedido já existe (para idempotência)
func (r *OrderRepository) OrderExists(ctx context.Context, orderID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM orders WHERE id = $1)", orderID).Scan(&exists)
	return exists, err
}

// CreateOrder cria um novo pedido no banco de dados
func (r *OrderRepository) CreateOrder(ctx context.Context, order *Order) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO orders (id, user_id, product_id, amount, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, order.ID, order.UserID, order.ProductID, order.Amount, order.Status, order.CreatedAt, order.UpdatedAt)
	return err
}

// UpdateOrderStatus atualiza o status de um pedido
func (r *OrderRepository) UpdateOrderStatus(ctx context.Context, orderID string, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE orders 
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND status != $1
	`, status, orderID)
	return err
}

// GetOrder busca um pedido pelo ID
func (r *OrderRepository) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	var order Order
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, product_id, amount, status, created_at, updated_at
		FROM orders WHERE id = $1
	`, orderID).Scan(&order.ID, &order.UserID, &order.ProductID, &order.Amount, &order.Status, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return &order, nil
}
