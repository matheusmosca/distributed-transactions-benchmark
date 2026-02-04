-- Schema para o banco de dados de Inventário (2PC/XA)
-- SEM stock_available (não há reserva explícita)
-- SEM version (não usa lock otimista, XA gerencia isolamento)

\c inventory_db;

CREATE TABLE IF NOT EXISTS products_inventory (
    product_id VARCHAR(128) PRIMARY KEY,
    product_name VARCHAR(255) NOT NULL,
    current_stock INTEGER NOT NULL CHECK (current_stock >= 0),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_inventory_product_id ON products_inventory (product_id);
CREATE INDEX idx_inventory_stock ON products_inventory (current_stock);

-- Tabela de movimentações de estoque (sempre 1 unidade por movimento)
CREATE TABLE IF NOT EXISTS inventory_movements (
    movement_id BIGSERIAL PRIMARY KEY,
    product_id VARCHAR(128) NOT NULL,
    order_id VARCHAR(128) NOT NULL,
    movement_type VARCHAR(50) NOT NULL, -- 'decrease', 'increase'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (product_id) REFERENCES products_inventory(product_id)
);

CREATE INDEX idx_movements_product_id ON inventory_movements (product_id);
CREATE INDEX idx_movements_order_id ON inventory_movements (order_id);
CREATE INDEX idx_movements_created_at ON inventory_movements (created_at);

-- Configuração para suportar XA transactions
ALTER SYSTEM SET max_prepared_transactions = 100;

-- Schema e tabela de barriers para idempotência do DTM XA
-- DTM espera schema 'dtm_barrier' com tabela 'barrier' dentro
CREATE SCHEMA IF NOT EXISTS dtm_barrier;

CREATE TABLE IF NOT EXISTS dtm_barrier.barrier (
    id BIGSERIAL PRIMARY KEY,
    trans_type VARCHAR(45) NOT NULL DEFAULT '',
    gid VARCHAR(128) NOT NULL DEFAULT '',
    branch_id VARCHAR(128) NOT NULL DEFAULT '',
    op VARCHAR(45) NOT NULL DEFAULT '',
    barrier_id VARCHAR(128) NOT NULL DEFAULT '',
    reason VARCHAR(45) NOT NULL DEFAULT '',
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uniq_barrier UNIQUE (gid, branch_id, op, barrier_id)
);

CREATE INDEX idx_barrier_create_time ON dtm_barrier.barrier(create_time);

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO root;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO root;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA dtm_barrier TO root;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA dtm_barrier TO root;

SELECT 'Inventory schema initialized (2PC/XA mode)!' AS status;
