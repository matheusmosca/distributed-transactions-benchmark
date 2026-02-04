package main

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPgxPool simula o pool de conexões PostgreSQL
type MockPgxPool struct {
	mock.Mock
}

func (m *MockPgxPool) QueryRow(ctx context.Context, sql string, args ...interface{}) *MockRow {
	mockArgs := m.Called(ctx, sql, args)
	return mockArgs.Get(0).(*MockRow)
}

func (m *MockPgxPool) Exec(ctx context.Context, sql string, args ...interface{}) (*MockCommandTag, error) {
	mockArgs := m.Called(ctx, sql, args)
	return mockArgs.Get(0).(*MockCommandTag), mockArgs.Error(1)
}

// MockRow simula uma linha de resultado
type MockRow struct {
	mock.Mock
	scanFunc func(dest ...interface{}) error
}

func (m *MockRow) Scan(dest ...interface{}) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	args := m.Called(dest)
	return args.Error(0)
}

// MockCommandTag simula o resultado de comandos SQL
type MockCommandTag struct {
	mock.Mock
	rowsAffected int64
}

func (m *MockCommandTag) RowsAffected() int64 {
	return m.rowsAffected
}

// MockRepository para testes que não precisam de banco real
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) OrderExists(ctx context.Context, orderID string) (bool, error) {
	args := m.Called(ctx, orderID)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepository) CreateOrder(ctx context.Context, order *Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}

func (m *MockRepository) UpdateOrderStatus(ctx context.Context, orderID string, status string) error {
	args := m.Called(ctx, orderID, status)
	return args.Error(0)
}

func (m *MockRepository) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	args := m.Called(ctx, orderID)
	return args.Get(0).(*Order), args.Error(1)
}

func TestNewOrderRepository(t *testing.T) {
	// Arrange
	var db *pgxpool.Pool // Mock pool

	// Act
	repo := NewOrderRepository(db)

	// Assert
	assert.NotNil(t, repo)
	assert.IsType(t, &OrderRepository{}, repo)
}

func TestMockRepository_OrderExists(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	ctx := context.Background()
	orderID := "test-order-123"

	mockRepo.On("OrderExists", ctx, orderID).Return(true, nil)

	// Act
	exists, err := mockRepo.OrderExists(ctx, orderID)

	// Assert
	assert.NoError(t, err)
	assert.True(t, exists)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_CreateOrder(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	ctx := context.Background()
	order := NewOrder("test-order-123", "user-456", "product-789", 2)

	mockRepo.On("CreateOrder", ctx, order).Return(nil)

	// Act
	err := mockRepo.CreateOrder(ctx, order)

	// Assert
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_UpdateOrderStatus(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	ctx := context.Background()
	orderID := "test-order-123"
	status := OrderStatusCompleted

	mockRepo.On("UpdateOrderStatus", ctx, orderID, status).Return(nil)

	// Act
	err := mockRepo.UpdateOrderStatus(ctx, orderID, status)

	// Assert
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_GetOrder(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	ctx := context.Background()
	orderID := "test-order-123"
	expectedOrder := &Order{
		ID:        orderID,
		UserID:    "user-456",
		ProductID: "product-789",
		Amount:    2,
		Status:    OrderStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.On("GetOrder", ctx, orderID).Return(expectedOrder, nil)

	// Act
	order, err := mockRepo.GetOrder(ctx, orderID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedOrder, order)
	mockRepo.AssertExpectations(t)
}
