\c payments_db;

-- Wallets table (SAGA - lock pessimista, sem version)
CREATE TABLE wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE,
    current_amount INTEGER NOT NULL CHECK (current_amount >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wallets_user_id ON wallets(user_id);

-- User Payments table
CREATE TYPE payment_type AS ENUM ('credit', 'debit');

CREATE TABLE user_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    order_id UUID NOT NULL,
    amount INTEGER NOT NULL CHECK (amount > 0),
    type payment_type NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_payments_wallet_id ON user_payments(wallet_id);
CREATE INDEX idx_user_payments_order_id ON user_payments(order_id);
CREATE INDEX idx_user_payments_created_at ON user_payments(created_at DESC);

-- Índice único para garantir idempotência: um order_id só pode ter um pagamento de cada tipo
CREATE UNIQUE INDEX idx_user_payments_order_payment_type 
ON user_payments(order_id, type);