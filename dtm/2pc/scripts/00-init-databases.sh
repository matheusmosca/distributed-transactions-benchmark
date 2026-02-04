#!/bin/bash
set -e

# Script para criar múltiplos databases no mesmo container Postgres

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    CREATE DATABASE dtm;
    CREATE DATABASE orders_db;
    CREATE DATABASE inventory_db;
    CREATE DATABASE payments_db;
EOSQL

echo "✅ Multiple databases created successfully!"
