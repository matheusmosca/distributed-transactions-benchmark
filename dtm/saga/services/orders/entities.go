package main

import (
	"errors"
	"time"
)

// Order representa um pedido no sistema
type Order struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	ProductID string    `json:"product_id" db:"product_id"`
	Amount    int       `json:"amount" db:"amount"`
	Status    string    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewOrder cria uma nova instância de Order
func NewOrder(id, userID, productID string, amount int) *Order {
	return &Order{
		ID:        id,
		UserID:    userID,
		ProductID: productID,
		Amount:    amount,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (o *Order) Fail() error {
	if o.Status != OrderStatusPending {
		return errors.New("only pending orders can be marked as failed")
	}

	o.Status = OrderStatusRejected
	return nil
}

// OrderStatus representa os possíveis status de um pedido
const (
	OrderStatusPending   = "pending"
	OrderStatusCompleted = "completed"
	OrderStatusRejected  = "rejected"
)
