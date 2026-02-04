\c inventory_db;

-- Products Inventory table (SAGA - lock pessimista, sem version)
CREATE TABLE products_inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_name VARCHAR(255) NOT NULL,
    current_stock INTEGER NOT NULL CHECK (current_stock >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_products_inventory_product_name ON products_inventory(product_name);

-- Inventory Movements table
CREATE TYPE movement_type AS ENUM ('increased', 'decreased');

CREATE TABLE inventory_movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inventory_id UUID NOT NULL REFERENCES products_inventory(id) ON DELETE CASCADE,
    order_id UUID NOT NULL,
    change_quantity INTEGER NOT NULL,
    movement_type movement_type NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_inventory_movements_inventory_id ON inventory_movements(inventory_id);
CREATE INDEX idx_inventory_movements_order_id ON inventory_movements(order_id);
CREATE INDEX idx_inventory_movements_created_at ON inventory_movements(created_at DESC);

-- Índice único para garantir idempotência: um order_id só pode ter um movimento de cada tipo
CREATE UNIQUE INDEX idx_inventory_movements_order_movement_type 
ON inventory_movements(order_id, movement_type);