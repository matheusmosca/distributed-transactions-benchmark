import random
import time
import subprocess
import datetime
import signal
import sys

# =======================================================
# CONFIGURAÇÕES DO EXPERIMENTO
# =======================================================
SEED = 44  
K6_SCRIPT = "k6.js"
TARGETS = ["dtm", "inventory_service", "payments_service"]

# Tempos de controle
TEMPO_ESPERA_K6_START = 65  # Espera 65s de carga antes do caos
MIN_FORA_DO_AR = 2
MAX_FORA_DO_AR = 8
MIN_INTERVALO_PAZ = 5
MAX_INTERVALO_PAZ = 10

# Tempo específico para o DTM
TEMPO_MORTO_DTM = 1

# =======================================================
# VARIAVEIS GLOBAIS E UTILITÁRIOS
# =======================================================
k6_process = None
random.seed(SEED)

def log(mensagem):
    now = datetime.datetime.now().strftime("%H:%M:%S")
    print(f"[{now}] {mensagem}")

def restaurar_ambiente():
    """Garante que todos os containers estejam rodando."""
    log("🛡️  Limpando ambiente: Subindo todos os containers...")
    for container in TARGETS:
        subprocess.run(["docker", "start", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    log("✅ Ambiente restaurado.")

def finalizar_k6():
    """Interrompe o processo do K6 se ele ainda estiver rodando."""
    global k6_process
    if k6_process and k6_process.poll() is None:
        log("🛑 Encerrando processo K6...")
        k6_process.terminate()
        try:
            k6_process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            k6_process.kill()

def handle_exit(sig, frame):
    """Callback para Ctrl+C ou encerramento do sistema."""
    print("\n")
    log("⚠️  Interrupção detectada!")
    finalizar_k6()
    restaurar_ambiente()
    sys.exit(0)

# Registrar o handler de saída
signal.signal(signal.SIGINT, handle_exit)
signal.signal(signal.SIGTERM, handle_exit)

# =======================================================
# EXECUÇÃO DO TESTE
# =======================================================

log("🚀 Iniciando Teste de Carga K6...")
k6_cmd = ["k6", "run", "--out", "json=resultados_load_test.json", K6_SCRIPT]
k6_process = subprocess.Popen(k6_cmd)

log(f"⏳ Aguardando {TEMPO_ESPERA_K6_START}s de estabilização do K6 antes do caos...")
time.sleep(TEMPO_ESPERA_K6_START)

log(f"💥 Iniciando Injeção de Caos (SEED: {SEED})")

try:
    # O loop roda enquanto o K6 estiver vivo
    while k6_process.poll() is None:
        # 1. Sorteio Determinístico
        container_alvo = random.choice(TARGETS)
        tempo_morto = random.randint(MIN_FORA_DO_AR, MAX_FORA_DO_AR)
        intervalo_paz = random.randint(MIN_INTERVALO_PAZ, MAX_INTERVALO_PAZ)

        # AJUSTE ESPECÍFICO PARA O DTM
        if container_alvo == "dtm":
            tempo_morto = TEMPO_MORTO_DTM

        log(f"🔥 ATAQUE: {container_alvo} (Morto por {tempo_morto}s)")
        subprocess.run(["docker", "kill", container_alvo], stdout=subprocess.DEVNULL)

        # 2. Espera durante a queda (ou até o K6 acabar)
        start_wait = time.time()
        while time.time() - start_wait < tempo_morto:
            if k6_process.poll() is not None: break # Sai se o K6 terminar
            time.sleep(0.5)

        # 3. Restauração
        log(f"🔄 RESTAURANDO: {container_alvo}")
        subprocess.run(["docker", "start", container_alvo], stdout=subprocess.DEVNULL)

        # 4. Intervalo de Paz (ou até o K6 acabar)
        log(f"☕ PAZ: Aguardando {intervalo_paz}s para o próximo ataque...")
        start_paz = time.time()
        while time.time() - start_paz < intervalo_paz:
            if k6_process.poll() is not None: break
            time.sleep(0.5)

    log("🏁 K6 finalizou naturalmente.")

except Exception as e:
    log(f"❌ Erro durante a execução: {e}")

finally:
    finalizar_k6()
    restaurar_ambiente()
    log("🔚 Teste finalizado com sucesso.")