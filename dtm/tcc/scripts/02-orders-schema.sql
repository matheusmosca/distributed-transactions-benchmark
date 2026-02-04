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

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO root;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO root;

SELECT 'Orders schema initialized!' AS status;
