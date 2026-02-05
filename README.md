# Distributed Transactions Benchmark

This project goal is to develop an experimental e-commerce platform composed of three microservices: orders, inventory, and payments. Each service has its own PostgreSQL database and is implemented in Go. The system explores distributed transaction management by integrating an orchestrator with one of three protocols: 2PC, TCC, or Saga. During benchmarking, a load test is performed and a script periodically injects failures, including container crashes and temporary unavailability. At the end of each run, metrics for latency, consistency, and resilience are collected. The benchmark is intended to be executed multiple times for each protocol, so that the provided Python scripts can analyze all generated data and perform comprehensive protocol comparisons.

## Project Structure

This repository is organized into two main folders:

- **/benchmark/**  
  Contains scripts and tools for running benchmarks, including load testing, failure injection, and graph generation for analysis. All benchmarking logic and result processing is centralized here.

- **/dtm/**  
  Contains the implementations for each distributed transaction protocol using the [DTM Labs](https://github.com/dtm-labs/dtm) orchestrator.  
  - `dtm/saga/` – Saga protocol implementation  
  - `dtm/2pc/` – Two-Phase Commit (2PC) protocol implementation  
  - `dtm/tcc/` – Try-Confirm-Cancel (TCC) protocol implementation  

Each protocol subfolder (`2pc`, `saga`, `tcc`) follows a similar structure. For example, the `saga` folder contains:

```
├── docker-compose.yml
├── scripts
│   ├── 00-init-databases.sh
│   ├── 01-dtm-init.sql
│   ├── 02-orders-schema.sql
│   ├── 03-inventory-schema.sql
│   ├── 04-payments-schema.sql
│   └── 05-seed.sql
├── services
│   ├── inventory
│   │   ├── Dockerfile
│   │   ├── entities.go
│   │   ├── handlers.go
│   │   ├── main.go
│   │   ├── repository.go
│   │   └── usecases.go
│   ├── orders
│   │   ├── Dockerfile
│   │   ├── dtm.go
│   │   ├── dtm_instrumentation.go
│   │   ├── entities.go
│   │   ├── entities_test.go
│   │   ├── handlers.go
│   │   ├── main.go
│   │   ├── repository.go
│   │   ├── repository_test.go
│   │   └── usecases.go
│   └── payments
│       ├── Dockerfile
│       ├── entities.go
│       ├── handlers.go
│       ├── main.go
│       ├── repository.go
│       └── usecases.go
└── traces_output
    └── all_traces_otlp.json
```

- The `docker-compose.yml` file is specialized for each protocol and is used to deploy all required services.
- The `services` folder contains subfolders for each microservice (`orders`, `inventory`, `payments`), each one with its own `Dockerfile` and source code.
- The `scripts` folder contains SQL scripts used to initialize each database and create the necessary tables.
- The orchestrator (DTM) is defined in the `docker-compose.yml` using the official DTM Labs image, along with observability tools such as OpenTelemetry (OTel) and Jaeger for tracing.

---

## Requirements

To run the benchmark, you need the following dependencies:
- [Docker](https://www.docker.com/) v29.2.0 or above
- [Docker Compose](https://docs.docker.com/compose/) v5.0.2 or above
- [k6](https://github.com/grafana/k6)
- [Python](https://www.python.org/) v3.14.2 or above

---

## Running the Benchmark

1. **Install Python dependencies:**
   ```bash
   pip install matplotlib psycopg2
   ```

2. **Run the benchmark for a specific protocol:**
   ```bash
   # Inside /benchmark
   python3 run_benchmark.py <tcc|2pc|saga>
   ```

   **Examples:**
   ```bash
   python3 run_benchmark.py saga
   python3 run_benchmark.py 2pc
   python3 run_benchmark.py tcc
   ```

---

## Generating Analysis and Graphs

After running the benchmarks, you can generate visualizations and analysis using the scripts in `/benchmark`. The results will appear in the `/benchmark/results` folder.

```bash
# Inside /benchmark

# Generate reliability graphs
python consistency_and_reliability_data_processing.py

# Generate latency plots
python latency_data_processing.py
```

---