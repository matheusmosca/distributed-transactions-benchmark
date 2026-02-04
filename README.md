

### How to run

Required dependencies to run the benchmark
- [docker](https://www.docker.com/) 29.2.0 or above
- [docker compose](https://docs.docker.com/compose/) v5.0.2 or above
- [k6](https://github.com/grafana/k6)
- [Python](https://www.python.org/) 3.14.2 or above

To run the benchmark, you need to execute the python `run_benchmark.py` passing the protocol desired to benchmark (saga, 2pc or tcc)
```bash
# inside /benchmark

# install python required packages
pip install matplotlib
pip install psycopg2

# run the benchmark for a specific protocol
python3 run_benchmark.py <tcc|2pc|saga>

# examples:

# initialize benchmark with saga
python3 run_benchmark.py saga

# initialize benchmark with 2pc
python3 run_benchmark.py 2pc

# initialize benchmark with tcc
python3 run_benchmark.py tcc

```

To generate the analysis and the graphs visualization, execute the python scripts below. The results appears in the /benchmark/results folder
```bash
# inside /benchmark

# to generate the reliability graphs
python reliability_data_processing.py

# to generate the latency plots
python latency_data_processing.py

```