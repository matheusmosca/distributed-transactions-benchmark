-- Schema para o banco de dados de Pagamentos (2PC/XA)
-- SEM available_amount (não há reserva explícita)
-- SEM version (não usa lock otimista, XA gerencia isolamento)

\c payments_db;

CREATE TABLE IF NOT EXISTS wallets (
    user_id VARCHAR(128) PRIMARY KEY,
    current_amount INTEGER NOT NULL CHECK (current_amount >= 0),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_wallets_user_id ON wallets (user_id);
CREATE INDEX idx_wallets_amount ON wallets (current_amount);

-- Tabela de transações financeiras
CREATE TABLE IF NOT EXISTS payment_transactions (
    transaction_id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(128) NOT NULL,
    order_id VARCHAR(128) NOT NULL,
    amount INTEGER NOT NULL,
    transaction_type VARCHAR(50) NOT NULL, -- 'debit', 'credit'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES wallets(user_id)
);

CREATE INDEX idx_transactions_user_id ON payment_transactions (user_id);
CREATE INDEX idx_transactions_order_id ON payment_transactions (order_id);
CREATE INDEX idx_transactions_created_at ON payment_transactions (created_at);

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

SELECT 'Payments schema initialized (2PC/XA mode)!' AS status;
