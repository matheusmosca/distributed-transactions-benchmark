package main

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel/trace"
)

var (
	ErrUnprocessableEntity = fmt.Errorf("unprocessable entity")
)

// InventoryUseCase contém a lógica de negócio do inventário
type InventoryUseCase struct {
	repository InventoryRepository
	tracer     trace.Tracer
}

// NewInventoryUseCase cria uma nova instância de InventoryUseCase
func NewInventoryUseCase(
	repository InventoryRepository,
	tracer trace.Tracer,
) *InventoryUseCase {
	return &InventoryUseCase{
		repository: repository,
		tracer:     tracer,
	}
}

// DecreaseStock diminui o estoque usando Lock Pessimista
func (uc *InventoryUseCase) DecreaseStock(ctx context.Context, req SagaActionRequest) error {
	log.Printf("➡️ [DECREASE STOCK] TraceID: %s | OrderID: %s | ProductID: %s",
		req.TraceID, req.OrderID, req.ProductID)

	// 1. Inicia a transação
	tx, err := uc.repository.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback()

	// 2. Obtém o produto com LOCK PESSIMISTA (SELECT FOR UPDATE)
	// Isso bloqueia a linha no banco até o Commit ou Rollback
	product, err := uc.repository.GetProductForUpdate(ctx, tx, req.ProductID)
	if err != nil {
		log.Printf("❌ DECREASE FAILED: GetProductForUpdate | OrderID=%s | Error=%v", req.OrderID, err)
		return err
	}

	// 3. Verificar idempotência dentro da transação
	exists, err := uc.repository.GetMovementByOrderIDAndType(ctx, tx, req.OrderID, "decreased")
	if err != nil {
		return fmt.Errorf("error to check idempotency: %w", err)
	}

	if exists {
		log.Printf("ℹ️  [IDEMPOTENCY] Movimento de decrease já processado para OrderID=%s", req.OrderID)
		return nil // Retorna sucesso para manter idempotência
	}

	// 4. Regra de Negócio: Verifica estoque
	if product.CurrentStock < 1 {
		log.Printf("❌ DECREASE FAILED: Insufficient stock | ProductID=%s", req.ProductID)
		return fmt.Errorf("insufficient stock for product %s", req.ProductID)
	}

	// 5. Executa a atualização do estoque e cria o registro de movimento
	// Como estamos com Lock Pessimista, não precisamos checar 'version' no WHERE
	if err := uc.repository.DecreaseStock(ctx, tx, req.ProductID, req.OrderID); err != nil {
		log.Printf("❌ [DECREASE] | OrderID=%s Failed to update: %v", req.OrderID, err)
		return err
	}

	// 6. Commit da transação
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao comitar decrease: %w", err)
	}

	log.Printf("✅ [DECREASE] Success: OrderID=%s", req.OrderID)
	return nil
}

// CompensateStock aumenta o estoque de volta (compensação) com idempotência e lock pessimista
func (uc *InventoryUseCase) CompensateStock(ctx context.Context, req SagaActionRequest) error {
	log.Printf("↩️ [COMPENSATE STOCK] TraceID: %s | OrderID: %s | ProductID: %s",
		req.TraceID, req.OrderID, req.ProductID)

	// 1. Inicia a transação
	tx, err := uc.repository.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback()

	// 2. Obtém o produto com LOCK PESSIMISTA (SELECT FOR UPDATE)
	_, err = uc.repository.GetProductForUpdate(ctx, tx, req.ProductID)
	if err != nil {
		log.Printf("❌ COMPENSATE FAILED: GetProductForUpdate | OrderID=%s | Error=%v", req.OrderID, err)
		return err
	}

	// 3. Verificar idempotência - se já existe movimento de 'increased' para este order_id
	exists, err := uc.repository.GetMovementByOrderIDAndType(ctx, tx, req.OrderID, "increased")
	if err != nil {
		return fmt.Errorf("erro ao verificar idempotência: %w", err)
	}

	if exists {
		log.Printf("ℹ️  [IDEMPOTENCY] Movimento de compensação já processado para OrderID=%s", req.OrderID)
		return nil
	}

	// 4. Executa a compensação (aumento) e cria o registro de movimento
	if err := uc.repository.IncreaseStock(ctx, tx, req.ProductID, req.OrderID); err != nil {
		log.Printf("❌ [COMPENSATE] | OrderID=%s Failed to update: %v", req.OrderID, err)
		return err
	}

	// 5. Commit da transação
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao comitar compensação: %w", err)
	}

	log.Printf("✅ [COMPENSATE] Success: OrderID=%s", req.OrderID)
	return nil
}
