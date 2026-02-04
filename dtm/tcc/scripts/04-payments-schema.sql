-- Schema para o banco de dados de Pagamentos
-- TCC-specific: Adiciona coluna available_amount para reservas

\c payments_db;

CREATE TABLE IF NOT EXISTS wallets (
    user_id VARCHAR(128) PRIMARY KEY,
    current_amount INTEGER NOT NULL CHECK (current_amount >= 0),
    available_amount INTEGER NOT NULL CHECK (available_amount >= 0),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT check_available_lte_current CHECK (available_amount <= current_amount)
);

CREATE INDEX idx_wallets_user_id ON wallets (user_id);
CREATE INDEX idx_wallets_available ON wallets (available_amount);

-- Enum para status das transações
CREATE TYPE transaction_status AS ENUM ('pending', 'completed', 'rejected');

-- Tabela de transações financeiras (apenas para CONFIRM)
CREATE TABLE IF NOT EXISTS payment_transactions (
    transaction_id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(128) NOT NULL,
    order_id VARCHAR(128) NOT NULL,
    amount INTEGER NOT NULL,
    transaction_type VARCHAR(50) NOT NULL, -- 'debit', 'credit'
    status transaction_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES wallets(user_id)
);

CREATE INDEX idx_transactions_user_id ON payment_transactions (user_id);


ALTER TABLE payment_transactions
ADD CONSTRAINT unique_order
UNIQUE (order_id);

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO root;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO root;

SELECT 'Payments schema initialized!' AS status;
