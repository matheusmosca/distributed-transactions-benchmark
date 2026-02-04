-- Schema para o banco de dados de Pedidos (Orders)
-- Armazena informações dos pedidos criados

\c orders_db;

-- Cada pedido é sempre de 1 unidade do produto
-- Status: pending (TRY) → completed (CONFIRM) ou cancelled (CANCEL)
CREATE TABLE IF NOT EXISTS orders (
    order_id VARCHAR(128) PRIMARY KEY,
    user_id VARCHAR(128) NOT NULL,
    product_id VARCHAR(128) NOT NULL,
    total_price INTEGER NOT NULL CHECK (total_price > 0),
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'cancelled')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_orders_user_id ON orders (user_id);
CREATE INDEX idx_orders_created_at ON orders (created_at);
CREATE INDEX idx_orders_status ON orders (status);

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

SELECT 'Orders schema initialized!' AS status;
