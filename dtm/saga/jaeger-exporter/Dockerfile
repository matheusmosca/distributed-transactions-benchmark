FROM python:3.11-slim

WORKDIR /app

# Instala dependências
RUN pip install --no-cache-dir \
    prometheus-client==0.19.0 \
    requests==2.31.0 \
    numpy==1.26.2

# Copia o exporter
COPY exporter.py .

# Expõe porta de métricas
EXPOSE 8000

# Roda o exporter
CMD ["python", "-u", "exporter.py"]
