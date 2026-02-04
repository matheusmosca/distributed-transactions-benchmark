-- Inicialização do banco de dados DTM
-- Este schema é usado pelo DTM para coordenar transações

\c dtm;

-- Tabela principal de transações do DTM (schema completo e atualizado)
CREATE TABLE IF NOT EXISTS trans_global (
    id BIGSERIAL PRIMARY KEY,
    gid VARCHAR(128) UNIQUE NOT NULL,
    trans_type VARCHAR(45) NOT NULL,
    status VARCHAR(12) NOT NULL,
    query_prepared VARCHAR(1024) NOT NULL DEFAULT '',
    protocol VARCHAR(45) NOT NULL DEFAULT '',
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finish_time TIMESTAMP,
    rollback_time TIMESTAMP,
    options VARCHAR(1024) NOT NULL DEFAULT '',
    custom_data VARCHAR(1024) NOT NULL DEFAULT '',
    next_cron_interval BIGINT DEFAULT NULL,
    next_cron_time TIMESTAMP DEFAULT NULL,
    steps JSONB,
    payloads JSONB,
    result VARCHAR(1024) DEFAULT '',
    rollback_reason VARCHAR(1024) DEFAULT '',
    owner VARCHAR(128) DEFAULT '',
    ext_data TEXT DEFAULT ''
);

CREATE INDEX trans_global_create_time_idx ON trans_global(create_time);
CREATE INDEX trans_global_status_idx ON trans_global(status);
CREATE INDEX trans_global_gid_idx ON trans_global(gid);

-- Tabela de branches (sub-transações)
CREATE TABLE IF NOT EXISTS trans_branch (
    id BIGSERIAL PRIMARY KEY,
    gid VARCHAR(128) NOT NULL,
    branch_id VARCHAR(128) NOT NULL,
    trans_type VARCHAR(45) NOT NULL,
    status VARCHAR(12) NOT NULL,
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finish_time TIMESTAMP,
    rollback_time TIMESTAMP,
    url VARCHAR(1024) NOT NULL DEFAULT '',
    data JSONB,
    branch_type VARCHAR(45) NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX trans_branch_gid_branch_id_idx ON trans_branch(gid, branch_id);
CREATE INDEX trans_branch_create_time_idx ON trans_branch(create_time);
CREATE INDEX trans_branch_status_idx ON trans_branch(status);

-- Tabela de operações de branch para TCC (schema completo)
CREATE TABLE IF NOT EXISTS trans_branch_op (
    id BIGSERIAL PRIMARY KEY,
    gid VARCHAR(128) NOT NULL,
    branch_id VARCHAR(128) NOT NULL,
    op VARCHAR(45) NOT NULL,
    status VARCHAR(12) NOT NULL,
    url VARCHAR(1024) DEFAULT '',
    bin_data TEXT DEFAULT '',
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finish_time TIMESTAMP,
    rollback_time TIMESTAMP,
    try_result TEXT DEFAULT '',
    confirm_result TEXT DEFAULT '',
    cancel_result TEXT DEFAULT ''
);

CREATE UNIQUE INDEX trans_branch_op_gid_branch_id_op_idx ON trans_branch_op(gid, branch_id, op);
CREATE INDEX trans_branch_op_create_time_idx ON trans_branch_op(create_time);
CREATE INDEX trans_branch_op_status_idx ON trans_branch_op(status);

-- Tabela kv para key-value storage do DTM
CREATE TABLE IF NOT EXISTS kv (
    id BIGSERIAL PRIMARY KEY,
    cat VARCHAR(45) NOT NULL,
    k VARCHAR(128) NOT NULL,
    v TEXT,
    version BIGINT DEFAULT 1,
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX kv_cat_k_idx ON kv(cat, k);
CREATE INDEX kv_create_time_idx ON kv(create_time);

-- Tabela de barriers para idempotência
CREATE TABLE IF NOT EXISTS dtm_barrier (
    id BIGSERIAL PRIMARY KEY,
    trans_type VARCHAR(45) NOT NULL DEFAULT '',
    gid VARCHAR(128) NOT NULL DEFAULT '',
    branch_id VARCHAR(128) NOT NULL DEFAULT '',
    op VARCHAR(45) NOT NULL DEFAULT '',
    barrier_id VARCHAR(128) NOT NULL DEFAULT '',
    reason VARCHAR(45) NOT NULL DEFAULT '',
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX dtm_barrier_gid_branch_id_op_barrier_id_idx ON dtm_barrier(gid, branch_id, op, barrier_id);
CREATE INDEX dtm_barrier_create_time_idx ON dtm_barrier(create_time);

-- Grants para o usuário root
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO root;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO root;

SELECT 'DTM database initialized successfully!' AS status;
