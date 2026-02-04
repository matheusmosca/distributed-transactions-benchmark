package main

import (
	"testing"
	"time"
)

func TestNewOrder(t *testing.T) {
	// Arrange
	id := "test-order-123"
	userID := "user-456"
	productID := "product-789"
	amount := 2

	// Act
	order := NewOrder(id, userID, productID, amount)

	// Assert
	if order.ID != id {
		t.Errorf("Expected ID %s, got %s", id, order.ID)
	}
	if order.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, order.UserID)
	}
	if order.ProductID != productID {
		t.Errorf("Expected ProductID %s, got %s", productID, order.ProductID)
	}
	if order.Amount != amount {
		t.Errorf("Expected Amount %d, got %d", amount, order.Amount)
	}
	if order.Status != OrderStatusPending {
		t.Errorf("Expected Status %s, got %s", OrderStatusPending, order.Status)
	}
	if order.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
	if order.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}

	// Verify that CreatedAt and UpdatedAt are within a reasonable time range
	now := time.Now()
	if order.CreatedAt.After(now) || order.CreatedAt.Before(now.Add(-time.Second)) {
		t.Error("CreatedAt is not within expected time range")
	}
	if order.UpdatedAt.After(now) || order.UpdatedAt.Before(now.Add(-time.Second)) {
		t.Error("UpdatedAt is not within expected time range")
	}
}

func TestOrderStatus(t *testing.T) {
	// Test that constants are defined correctly
	if OrderStatusPending != "pending" {
		t.Errorf("Expected OrderStatusPending to be 'pending', got %s", OrderStatusPending)
	}
	if OrderStatusCompleted != "completed" {
		t.Errorf("Expected OrderStatusCompleted to be 'completed', got %s", OrderStatusCompleted)
	}
	if OrderStatusRejected != "rejected" {
		t.Errorf("Expected OrderStatusRejected to be 'rejected', got %s", OrderStatusRejected)
	}
}
