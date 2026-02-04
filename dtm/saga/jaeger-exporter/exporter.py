#!/usr/bin/env python3
"""
Jaeger Exporter - Analisa traces do Jaeger e exporta métricas para Prometheus
Conta TRANSAÇÕES únicas (não spans), calculando:
- Total de transações
- Transações com rollback
- Taxa de rollback
- P50, P95, P99 de duração POR TRANSAÇÃO
"""

import os
import time
import logging
import requests
from collections import defaultdict
from prometheus_client import start_http_server, Gauge, Counter, Histogram
import numpy as np

# Configuração de logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Configuração
JAEGER_URL = os.getenv('JAEGER_URL', 'http://jaeger:16686')
SERVICE_NAME = os.getenv('SERVICE_NAME', 'orders-service')
SCRAPE_INTERVAL = int(os.getenv('SCRAPE_INTERVAL', '60'))  # segundos
LOOKBACK = os.getenv('LOOKBACK', '5m')  # quanto histórico analisar
LIMIT = int(os.getenv('LIMIT', '1000'))  # máximo de traces por scrape

# Métricas Prometheus (orientadas a TRANSAÇÃO)
transactions_total = Counter(
    'saga_transactions_analyzed_total',
    'Total de transações SAGA analisadas',
    ['has_rollback']
)

rollback_rate = Gauge(
    'saga_rollback_rate_percent',
    'Porcentagem de transações com rollback'
)

transaction_duration_p50 = Gauge(
    'saga_transaction_duration_p50_ms',
    'P50 (mediana) da duração total por transação'
)

transaction_duration_p95 = Gauge(
    'saga_transaction_duration_p95_ms',
    'P95 da duração total por transação'
)

transaction_duration_p99 = Gauge(
    'saga_transaction_duration_p99_ms',
    'P99 da duração total por transação'
)

transaction_duration_mean = Gauge(
    'saga_transaction_duration_mean_ms',
    'Média da duração total por transação'
)

transactions_in_window = Gauge(
    'saga_transactions_in_analysis_window',
    'Total de transações analisadas na última janela'
)

def fetch_traces():
    """Busca traces do Jaeger"""
    url = f"{JAEGER_URL}/api/traces"
    params = {
        'service': SERVICE_NAME,
        'lookback': LOOKBACK,
        'limit': LIMIT
    }
    
    try:
        response = requests.get(url, params=params, timeout=10)
        response.raise_for_status()
        return response.json().get('data', [])
    except Exception as e:
        logger.error(f"Erro ao buscar traces: {e}")
        return []

def analyze_trace(trace):
    """
    Analisa um trace individual
    Retorna: (duration_ms, has_rollback)
    """
    spans = trace.get('spans', [])
    trace_id = trace.get('traceID', 'unknown')
    
    # Detecta rollback: presença de spans de compensação
    has_rollback = any(
        span.get('operationName', '').startswith('saga_compensation_')
        for span in spans
    )
    
    # Calcula duração TOTAL da transação:
    # Soma apenas spans de ações SAGA (não spans HTTP, etc)
    saga_spans = [
        span for span in spans
        if span.get('operationName', '').startswith('saga_action_')
    ]
    
    if not saga_spans:
        logger.warning(f"Trace {trace_id} sem spans saga_action_*")
        return None, has_rollback
    
    # Duração total = soma dos spans de ação (em microsegundos)
    total_duration_us = sum(span.get('duration', 0) for span in saga_spans)
    total_duration_ms = total_duration_us / 1000.0
    
    return total_duration_ms, has_rollback

def analyze_and_export():
    """Analisa traces e atualiza métricas Prometheus"""
    logger.info(f"Iniciando análise de traces (lookback={LOOKBACK}, limit={LIMIT})")
    
    traces = fetch_traces()
    
    if not traces:
        logger.warning("Nenhum trace encontrado")
        return
    
    logger.info(f"Analisando {len(traces)} traces...")
    
    durations = []
    rollback_count = 0
    total_count = 0
    
    for trace in traces:
        duration_ms, has_rollback = analyze_trace(trace)
        
        if duration_ms is None:
            continue
        
        total_count += 1
        durations.append(duration_ms)
        
        # Incrementa contador por tipo
        transactions_total.labels(has_rollback=str(has_rollback).lower()).inc()
        
        if has_rollback:
            rollback_count += 1
    
    if total_count == 0:
        logger.warning("Nenhuma transação válida encontrada")
        return
    
    # Calcula estatísticas
    durations_array = np.array(durations)
    p50 = np.percentile(durations_array, 50)
    p95 = np.percentile(durations_array, 95)
    p99 = np.percentile(durations_array, 99)
    mean = np.mean(durations_array)
    rollback_pct = (rollback_count / total_count) * 100
    
    # Atualiza métricas Prometheus
    transaction_duration_p50.set(p50)
    transaction_duration_p95.set(p95)
    transaction_duration_p99.set(p99)
    transaction_duration_mean.set(mean)
    rollback_rate.set(rollback_pct)
    transactions_in_window.set(total_count)
    
    logger.info(f"""
    📊 Análise Completa:
    - Total transações: {total_count}
    - Com rollback: {rollback_count} ({rollback_pct:.2f}%)
    - P50: {p50:.2f}ms
    - P95: {p95:.2f}ms
    - P99: {p99:.2f}ms
    - Média: {mean:.2f}ms
    """)

def main():
    """Loop principal do exporter"""
    port = int(os.getenv('PORT', '8000'))
    
    logger.info(f"🚀 Iniciando Jaeger Exporter na porta {port}")
    logger.info(f"📡 Jaeger URL: {JAEGER_URL}")
    logger.info(f"🔍 Service: {SERVICE_NAME}")
    logger.info(f"⏱️  Intervalo de scrape: {SCRAPE_INTERVAL}s")
    
    # Inicia servidor Prometheus
    start_http_server(port)
    logger.info(f"✅ Servidor Prometheus rodando em http://0.0.0.0:{port}/metrics")
    
    # Loop de análise
    while True:
        try:
            analyze_and_export()
        except Exception as e:
            logger.error(f"Erro na análise: {e}", exc_info=True)
        
        time.sleep(SCRAPE_INTERVAL)

if __name__ == '__main__':
    main()
