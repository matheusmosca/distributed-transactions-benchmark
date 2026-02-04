package main

import (
	"time"
)

// ProductInventory representa o inventário de um produto
type ProductInventory struct {
	ID           string    `json:"id" db:"id"`
	CurrentStock int       `json:"current_stock" db:"current_stock"`
	Version      int       `json:"version" db:"version"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// NewProductInventory cria uma nova instância de ProductInventory
func NewProductInventory(id string, initialStock int) *ProductInventory {
	return &ProductInventory{
		ID:           id,
		CurrentStock: initialStock,
		Version:      1,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// InventoryMovement representa uma movimentação de estoque
type InventoryMovement struct {
	ID             string    `json:"id" db:"id"`
	InventoryID    string    `json:"inventory_id" db:"inventory_id"`
	ChangeQuantity int       `json:"change_quantity" db:"change_quantity"`
	MovementType   string    `json:"movement_type" db:"movement_type"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// NewInventoryMovement cria uma nova instância de InventoryMovement
func NewInventoryMovement(id, inventoryID string, changeQuantity int, movementType string) *InventoryMovement {
	return &InventoryMovement{
		ID:             id,
		InventoryID:    inventoryID,
		ChangeQuantity: changeQuantity,
		MovementType:   movementType,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// MovementType representa os tipos de movimentação de estoque
const (
	MovementTypeDecreased = "decreased"
	MovementTypeIncreased = "increased"
)
