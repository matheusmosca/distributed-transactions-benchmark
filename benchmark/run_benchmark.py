#!/usr/bin/env python3

import subprocess
import sys
import os
import time
import shutil
import glob
from datetime import datetime

# Protocolos suportados
SUPPORTED_PROTOCOLS = ['2pc', 'tcc', 'saga']

# Caminhos dos docker-compose files (ajustados para execução do diretório benchmark)
DOCKER_COMPOSE_PATHS = {
    '2pc': '../dtm/2pc/docker-compose.yml',
    'tcc': '../dtm/tcc/docker-compose.yml', 
    'saga': '../dtm/saga/docker-compose.yml'
}

# Caminhos dos arquivos de tracing (ajustados para execução do diretório benchmark) 
TRACINGS_OUTPUT_PATHS = {
    '2pc': '../dtm/2pc/traces_output/all_traces_otlp.json',
    'tcc': '../dtm/tcc/traces_output/all_traces_otlp.json', 
    'saga': '../dtm/saga/traces_output/all_traces_otlp.json'
}


def log(message):
    """Log com timestamp"""
    timestamp = datetime.now().strftime("%H:%M:%S")
    print(f"[{timestamp}] {message}")

def countdown(seconds):
    """Exibe um contador regressivo no console"""
    for i in range(seconds, 0, -1):
        sys.stdout.write(f"\r⏳ Aguardando: {i}s... ")
        sys.stdout.flush()
        time.sleep(1)
    print("\r✅ Tempo de espera concluído.           ")

def cleanup_docker():
    """Derruba todos os docker-compose dos protocolos e remove volumes"""
    log("🧹 Limpando ambiente Docker...")

    for protocol, compose_path in DOCKER_COMPOSE_PATHS.items():
        if os.path.exists(compose_path):
            log(f"   - Derrubando docker-compose para {protocol.upper()}...")
            # Removido capture_output para mostrar o progresso no console
            subprocess.run(
                ["docker-compose", "-f", compose_path, "down", "-v", "--remove-orphans"]
            )
        else:
            log(f"   - Arquivo docker-compose não encontrado para {protocol.upper()}: {compose_path}")

    log("✅ Ambiente Docker limpo")

def start_docker_compose(protocol):
    """Inicia o docker-compose do protocolo especificado"""
    compose_path = DOCKER_COMPOSE_PATHS[protocol]
    
    if not os.path.exists(compose_path):
        log(f"❌ Arquivo docker-compose não encontrado: {compose_path}")
        sys.exit(1)
    
    log(f"🚀 Iniciando docker-compose para protocolo {protocol.upper()}...")
    log(f"   - Arquivo: {compose_path}")
    
    # Executa docker-compose up -d mostrando o pull/start das imagens
    result = subprocess.run([
        "docker-compose", "-f", compose_path, "up", "-d"
    ])
    
    if result.returncode != 0:
        log(f"❌ Erro ao iniciar docker-compose. Verifique os logs acima.")
        sys.exit(1)
    
    log("✅ Docker-compose solicitado com sucesso")
    log("⏳ Estabilizando serviços...")
    countdown(5)

def run_chaos_test():
    """Executa o script chaos.py"""
    log("💥 Iniciando teste de caos...")
    
    if not os.path.exists("chaos.py"):
        log("❌ Arquivo chaos.py não encontrado no diretório atual")
        sys.exit(1)
    
    # Executa o chaos.py e transmite os logs dele em tempo real
    result = subprocess.run(["python3", "chaos.py"])
    
    if result.returncode != 0:
        log(f"⚠️  O script chaos.py terminou com erro (code {result.returncode})")
    else:
        log("✅ Teste de caos finalizado")

def run_reconciliation(protocol):
    """Executa o script reconciliation.py"""
    log("📊 Iniciando análise de reconciliação...")
    
    if not os.path.exists("reconciliation.py"):
        log("❌ Arquivo reconciliation.py não encontrado no diretório atual")
        sys.exit(1)
    
    # Executa o reconciliation.py transmitindo logs em tempo real
    result = subprocess.run(["python3", "reconciliation.py", protocol])
    
    if result.returncode != 0:
        log(f"❌ Erro ao executar reconciliation.py (code {result.returncode})")
        sys.exit(1)
    
    log("✅ Análise de reconciliação finalizada")

def copy_tracing_files(protocol):
    """Copia arquivos de tracing para o diretório tracings"""
    log("📋 Copiando arquivos de tracing...")
    
    # Arquivo de origem específico do protocolo
    traces_source = TRACINGS_OUTPUT_PATHS[protocol]
    
    # Diretório de destino (relativo ao diretório benchmark onde o script roda)
    traces_dest_dir = f"tracings/{protocol}"
    os.makedirs(traces_dest_dir, exist_ok=True)

    log(f"   - Origem: {traces_source}")
    log(f"   - Destino: {traces_dest_dir}")
    
    if not os.path.exists(traces_source):
        log(f"⚠️  Arquivo de traces não encontrado: {traces_source}")
        log(f"   - Caminho absoluto tentado: {os.path.abspath(traces_source)}")
        return
    
    # Nome do arquivo de destino com timestamp
    original_filename = os.path.basename(traces_source)
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    filename_with_timestamp = f"{timestamp}_{original_filename}"
    dest_file = os.path.join(traces_dest_dir, filename_with_timestamp)
    
    try:
        shutil.copy2(traces_source, dest_file)
        log(f"   ✅ Copiado: {filename_with_timestamp}")
        log(f"   - Arquivo criado em: {os.path.abspath(dest_file)}")
    except Exception as e:
        log(f"   ❌ Erro ao copiar {filename_with_timestamp}: {e}")
    
    log("✅ Arquivos de tracing copiados")

def main():
    if len(sys.argv) != 2:
        log("❌ Uso: python run_benchmark.py <protocol>")
        sys.exit(1)
    
    protocol = sys.argv[1].lower()
    
    if protocol not in SUPPORTED_PROTOCOLS:
        log(f"❌ Protocolo '{protocol}' não suportado")
        sys.exit(1)
    
    log("="*60)
    log(f"🎯 INICIANDO BENCHMARK PARA PROTOCOLO: {protocol.upper()}")
    log("="*60)

    try:
        cleanup_docker()
        start_docker_compose(protocol)
        run_chaos_test()

        log("⏳ Aguardando janela de 60 segundos para estabilização final...")
        countdown(60)
        
        copy_tracing_files(protocol)
        run_reconciliation(protocol)
        
        log("="*60)
        log("🎉 BENCHMARK CONCLUÍDO COM SUCESSO!")
        log("="*60)
        
    except KeyboardInterrupt:
        log("\n⚠️  Benchmark interrompido pelo usuário")
        sys.exit(1)
    except Exception as e:
        log(f"❌ Erro inesperado: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()