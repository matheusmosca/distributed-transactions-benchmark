import psycopg2
import json
import os
import sys
from datetime import datetime
from collections import defaultdict

# --- Configurações ---
DB_HOST = "localhost"
DB_PORT = "5433"
DB_USER = "root"
DB_PASS = "pass"

INITIAL_VALUE = 900_000_000
PENDING_STATUS = 'pending'
COMPLETED_STATUS = 'completed'

# Protocolos suportados
SUPPORTED_PROTOCOLS = ['2pc', 'tcc', 'saga']

def get_connection(db_name):
    """Cria uma conexão com o banco de dados especificado."""
    try:
        conn = psycopg2.connect(
            host=DB_HOST,
            port=DB_PORT,
            user=DB_USER,
            password=DB_PASS,
            dbname=db_name
        )
        return conn
    except Exception as e:
        print(f"❌ Erro ao conectar no banco {db_name}: {e}")
        exit(1)

def analyze_dtm_transactions():
    """
    Analisa o banco dtm para estatísticas de transações DTM.
    """
    conn = get_connection("dtm")
    cursor = conn.cursor()

    print("\n" + "="*50)
    print("📊 ANÁLISE DTM TRANSACTIONS")
    print("="*50)

    # Query para contar transações por status
    cursor.execute("SELECT status, COUNT(*) FROM trans_global GROUP BY status;")
    status_counts = cursor.fetchall()
    
    # Inicializa contadores
    dtm_stats = {
        "total": 0,
        "succeed": 0,
        "aborting": 0,
        "submitted": 0,
        "prepared": 0,
        "failed": 0,
        "rollbacks": 0
    }
    
    print("📌 Transações DTM por Status:")
    for status, count in status_counts:
        print(f"   - {status}: {count}")
        dtm_stats["total"] += count
        
        if status in dtm_stats:
            dtm_stats[status] = count
            
        # Contabiliza rollbacks (failed + aborting)
        if status in ["failed", "aborting"]:
            dtm_stats["rollbacks"] += count

    cursor.close()
    conn.close()
    return dtm_stats

def analyze_orders(protocol):
    """
    Analisa o banco orders_db com base no protocolo.
    Retorna estatísticas e mapas de consumo para conciliação.
    """
    conn = get_connection("orders_db")
    cursor = conn.cursor()

    print("\n" + "="*50)
    print("📊 ANÁLISE DE ORDENS (orders_db)")
    print("="*50)

    # 1. Total de Ordens
    cursor.execute("SELECT COUNT(*) FROM orders;")
    total_orders = cursor.fetchone()[0]
    print(f"🔹 Total de Ordens: {total_orders}")

    # 2. Ordens por Status
    cursor.execute("SELECT status, COUNT(*) FROM orders GROUP BY status;")
    status_counts = cursor.fetchall()
    
    orders_stats = {
        "total": total_orders,
        "completed": 0,
        "pending": 0,
        "failed": 0
    }
    
    # Status de falha depende do protocolo
    failed_status = "rejected" if protocol == "saga" else "cancelled"
    
    print("\n📌 Total de Ordens por Status:")
    for status, count in status_counts:
        print(f"   - {status}: {count}")
        
        if status == "completed":
            orders_stats["completed"] = count
        elif status == "pending":
            orders_stats["pending"] = count
        elif status in ["rejected", "cancelled"]:
            orders_stats["failed"] += count

    # --- Coleta de Dados para Conciliação ---
    
    # Mapa: user_id -> qtd ordens completed
    user_completed_map = defaultdict(int)
    # Mapa: product_id -> qtd ordens completed
    product_completed_map = defaultdict(int)

    # Busca apenas ordens completed para calcular os débitos/consumos
    cursor.execute(f"SELECT user_id, product_id FROM orders WHERE status = '{COMPLETED_STATUS}';")
    completed_orders = cursor.fetchall()

    for user_id, product_id in completed_orders:
        user_completed_map[user_id] += 1
        product_completed_map[product_id] += 1

    cursor.close()
    conn.close()

    return orders_stats, user_completed_map, product_completed_map

def reconcile_wallets(user_completed_map):
    """
    Concilia o banco payments_db (wallets) com o histórico de ordens.
    Retorna o total de inconsistências.
    """
    conn = get_connection("payments_db")
    cursor = conn.cursor()

    print("\n" + "="*50)
    print("💰 CONCILIAÇÃO DE WALLETS (payments_db)")
    print("="*50)

    cursor.execute("SELECT user_id, current_amount FROM wallets;")
    wallets = cursor.fetchall()

    total_inconsistencies_val = 0
    inconsistent_wallets_count = 0

    print(f"Analizando {len(wallets)} wallets...")

    for user_id, actual_balance in wallets:
        # Quantas ordens completas esse usuário teve?
        completed_orders_count = user_completed_map.get(user_id, 0)
        
        # Cálculo do Ideal: Inicial - (1 * ordens completas)
        expected_balance = INITIAL_VALUE - (completed_orders_count * 1)
        
        # Diferença (Inconsistência) - valor absoluto das diferenças
        diff = abs(actual_balance - expected_balance)
        total_inconsistencies_val += diff

        if diff != 0:
            inconsistent_wallets_count += 1

    print(f"\n🏁 Resultado Wallets:")
    print(f"   🔹 Total de Inconsistências (Valor): {total_inconsistencies_val}")
    print(f"   🔹 Wallets com divergência: {inconsistent_wallets_count}")

    cursor.close()
    conn.close()
    
    return total_inconsistencies_val

def reconcile_inventory(product_completed_map, protocol):
    """
    Concilia o banco inventory_db (products_inventory) com o histórico de ordens.
    Retorna o total de inconsistências.
    """
    conn = get_connection("inventory_db")
    cursor = conn.cursor()

    print("\n" + "="*50)
    print("📦 CONCILIAÇÃO DE ESTOQUE (inventory_db)")
    print("="*50)

    # Para SAGA, a coluna é 'id', para outros protocolos é 'product_id'
    product_id_column = "id" if protocol == "saga" else "product_id"
    
    cursor.execute(f"SELECT {product_id_column}, current_stock FROM products_inventory;")
    products = cursor.fetchall()

    total_inconsistencies_val = 0
    inconsistent_products_count = 0

    print(f"Analizando {len(products)} produtos...")

    for product_id, actual_stock in products:
        # Quantas vezes esse produto foi vendido (completed)?
        sold_count = product_completed_map.get(product_id, 0)

        # Cálculo do Ideal: Inicial - (1 * ordens completas)
        expected_stock = INITIAL_VALUE - (sold_count * 1)

        # Diferença (Inconsistência) - valor absoluto das diferenças
        diff = abs(actual_stock - expected_stock)
        total_inconsistencies_val += diff

        if diff != 0:
            inconsistent_products_count += 1

    print(f"\n🏁 Resultado Estoque:")
    print(f"   🔹 Total de Inconsistências (Valor): {total_inconsistencies_val}")
    print(f"   🔹 Produtos com divergência: {inconsistent_products_count}")

    cursor.close()
    conn.close()
    
    return total_inconsistencies_val

def create_output_directory(protocol):
    """
    Cria o diretório de saída baseado no protocolo.
    """
    base_dir = "consistency"
    protocol_dir = f"{base_dir}/{protocol}"
    
    if not os.path.exists(protocol_dir):
        os.makedirs(protocol_dir)
        print(f"📁 Diretório '{protocol_dir}' criado.")

def save_results_to_json(protocol, results):
    """
    Salva os resultados em um arquivo JSON com timestamp.
    """
    create_output_directory(protocol)
    
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    filename = f"consistency/{protocol}/{timestamp}.json"
    
    with open(filename, 'w', encoding='utf-8') as f:
        json.dump(results, f, indent=2, ensure_ascii=False)
    
    print(f"💾 Resultados salvos em: {filename}")
    return filename

def main():
    # Verifica argumentos da linha de comando
    if len(sys.argv) != 2:
        print("❌ Uso: python reconciliation.py <protocol>")
        print(f"Protocolos suportados: {', '.join(SUPPORTED_PROTOCOLS)}")
        sys.exit(1)
    
    protocol = sys.argv[1].lower()
    
    if protocol not in SUPPORTED_PROTOCOLS:
        print(f"❌ Protocolo '{protocol}' não suportado.")
        print(f"Protocolos suportados: {', '.join(SUPPORTED_PROTOCOLS)}")
        sys.exit(1)
    
    print(f"🔍 Analisando protocolo: {protocol.upper()}")
    
    # 1. Analisa transações DTM
    dtm_stats = analyze_dtm_transactions()
    
    # 2. Analisa Ordens e obtém mapas de consumo
    orders_stats, user_map, product_map = analyze_orders(protocol)

    # 3. Concilia Wallets
    payment_inconsistencies = reconcile_wallets(user_map)

    # 4. Concilia Estoque
    inventory_inconsistencies = reconcile_inventory(product_map, protocol)

    # 5. Monta o resultado consolidado
    results = {
        "protocol": protocol,
        "dtm_transactions": dtm_stats,
        "orders": orders_stats,
        "inventory_inconsistencies": inventory_inconsistencies,
        "payment_inconsistencies": payment_inconsistencies
    }
    
    # 6. Salva em JSON
    output_file = save_results_to_json(protocol, results)
    
    print("\n" + "="*50)
    print("✅ ANÁLISE COMPLETA")
    print("="*50)
    print(f"📊 Protocolo: {protocol.upper()}")
    print(f"📁 Arquivo gerado: {output_file}")
    print(f"🔢 DTM Total: {dtm_stats['total']} | Rollbacks: {dtm_stats['rollbacks']}")
    print(f"📦 Ordens Total: {orders_stats['total']} | Completadas: {orders_stats['completed']}")
    print(f"💰 Inconsistências Pagamentos: {payment_inconsistencies}")
    print(f"📋 Inconsistências Estoque: {inventory_inconsistencies}")

if __name__ == "__main__":
    main()