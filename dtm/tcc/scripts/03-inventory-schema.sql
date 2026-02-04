-- Schema para o banco de dados de Inventário
-- TCC-specific: Adiciona coluna stock_available para reservas

\c inventory_db;

CREATE TABLE IF NOT EXISTS products_inventory (
    product_id VARCHAR(128) PRIMARY KEY,
    product_name VARCHAR(255) NOT NULL,
    current_stock INTEGER NOT NULL CHECK (current_stock >= 0),
    stock_available INTEGER NOT NULL CHECK (stock_available >= 0),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT check_available_lte_current CHECK (stock_available <= current_stock)
);

CREATE INDEX idx_inventory_product_id ON products_inventory (product_id);
CREATE INDEX idx_inventory_available ON products_inventory (stock_available);

-- Enum para status das movimentações
CREATE TYPE movement_status AS ENUM ('pending', 'completed', 'rejected');

-- Tabela de movimentações de estoque (sempre 1 unidade por movimento)
CREATE TABLE IF NOT EXISTS inventory_movements (
    movement_id BIGSERIAL PRIMARY KEY,
    product_id VARCHAR(128) NOT NULL,
    order_id VARCHAR(128) NOT NULL,
    movement_type VARCHAR(50) NOT NULL, -- 'decrease', 'increase'
    status movement_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (product_id) REFERENCES products_inventory(product_id)
);

CREATE INDEX idx_movements_product_id ON inventory_movements (product_id);
CREATE INDEX idx_movements_created_at ON inventory_movements (created_at);

ALTER TABLE inventory_movements
ADD CONSTRAINT unique_order
UNIQUE (order_id);

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO root;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO root;

SELECT 'Inventory schema initialized!' AS status;
