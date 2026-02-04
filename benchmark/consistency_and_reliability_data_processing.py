#!/usr/bin/env python3

import json
import os
import sys
import numpy as np
import matplotlib.pyplot as plt
from collections import defaultdict


plt.rcParams.update({
    "figure.dpi": 100,
    "savefig.dpi": 300,

    "font.size": 15,
    "axes.titlesize": 15,
    "axes.labelsize": 15,
    "legend.fontsize": 15,
    "xtick.labelsize": 15,
    "ytick.labelsize": 15,

    "lines.linewidth": 1.2,

    "axes.linewidth": 0.8,
    "grid.linewidth": 0.5,
    "grid.alpha": 0.3,

    "font.family": "serif",
    "font.serif": ["Times New Roman", "Times", "DejaVu Serif"]
})


def nanoseconds_to_milliseconds(ns_duration):
    return float(ns_duration) / 1_000_000


def process_otlp_batch(batch, trace_data):
    if not isinstance(batch, dict):
        return

    for resource_span in batch.get('resourceSpans', []):
        for scope_span in resource_span.get('scopeSpans', []):
            for span in scope_span.get('spans', []):
                trace_id = span.get('traceId')
                if not trace_id:
                    continue

                try:
                    start_time = int(span.get('startTimeUnixNano', 0))
                    end_time = int(span.get('endTimeUnixNano', 0))
                except Exception:
                    continue

                if start_time == 0 or end_time == 0:
                    continue

                info = trace_data[trace_id]
                info['start_time'] = min(info['start_time'], start_time)
                info['end_time'] = max(info['end_time'], end_time)


def process_tracing_file(file_path):
    trace_data = defaultdict(lambda: {'start_time': float('inf'), 'end_time': 0})

    try:
        with open(file_path, 'r') as f:
            content = f.read().strip()
            if not content:
                return []

            try:
                process_otlp_batch(json.loads(content), trace_data)
            except json.JSONDecodeError:
                for line in content.splitlines():
                    try:
                        process_otlp_batch(json.loads(line), trace_data)
                    except Exception:
                        continue
    except Exception:
        return []

    traces = []
    for trace_id, t in trace_data.items():
        if t['end_time'] > t['start_time'] and t['start_time'] != float('inf'):
            traces.append({
                'trace_id': trace_id,
                'duration_ms': nanoseconds_to_milliseconds(t['end_time'] - t['start_time'])
            })

    return traces


def count_tracings_for_protocol(protocol_name, tracings_base_dir):
    protocol_dir = os.path.join(tracings_base_dir, protocol_name)
    if not os.path.exists(protocol_dir):
        return []

    files = [f for f in os.listdir(protocol_dir) if f.endswith('.json')]
    all_traces = []

    for filename in files:
        all_traces.extend(process_tracing_file(os.path.join(protocol_dir, filename)))

    return all_traces


def consolidate_consistency_data(protocol_name, consistency_base_dir):
    protocol_dir = os.path.join(consistency_base_dir, protocol_name)
    if not os.path.exists(protocol_dir):
        return None

    files = [f for f in os.listdir(protocol_dir) if f.endswith('.json')]
    if not files:
        return None

    consolidated = {
        'dtm_transactions': defaultdict(int),
        'orders': defaultdict(int),
        'inventory_inconsistencies': 0,
        'payment_inconsistencies': 0
    }

    for filename in files:
        try:
            with open(os.path.join(protocol_dir, filename), 'r') as f:
                data = json.load(f)

            for k, v in data.get('dtm_transactions', {}).items():
                if isinstance(v, (int, float)):
                    consolidated['dtm_transactions'][k] += v

            for k, v in data.get('orders', {}).items():
                if isinstance(v, (int, float)):
                    consolidated['orders'][k] += v

            consolidated['inventory_inconsistencies'] += data.get('inventory_inconsistencies', 0)
            consolidated['payment_inconsistencies'] += data.get('payment_inconsistencies', 0)

        except Exception:
            continue

    consolidated['dtm_transactions'] = dict(consolidated['dtm_transactions'])
    consolidated['orders'] = dict(consolidated['orders'])

    return consolidated


def plot_reliability(reliability_stats, output_dir):
    metrics = ['rollback_rate', 'failure_rate']
    labels = ['Rollback Rate', 'Technical Failure Rate']
    x = np.arange(len(metrics))
    width = 0.25

    fig, ax = plt.subplots(figsize=(10, 4.6))

    colors = {
        '2pc': '#1f77b4',
        'saga': '#2ca02c',
        'tcc': '#d62728'
    }

    for i, protocol in enumerate(['2pc', 'saga', 'tcc']):
        if protocol not in reliability_stats:
            continue

        values = [reliability_stats[protocol][m] for m in metrics]
        bars = ax.bar(
            x + (i - 1) * width,
            values,
            width,
            label=protocol.upper(),
            color=colors[protocol],
            edgecolor="black",
            linewidth=0.6
        )

        for bar in bars:
            h = bar.get_height()
            ax.text(
                bar.get_x() + bar.get_width() / 2,
                h,
                f"{h:.2f}%",
                ha="center",
                va="bottom",
                fontsize=12
            )

    ax.set_ylabel("Percentage (%)")
    ax.set_xticks(x)
    ax.set_xticklabels(labels)

    ax.legend(
        loc="center left",
        bbox_to_anchor=(1.02, 0.5),
        frameon=False
    )

    ax.grid(axis="y", linestyle="--", alpha=0.3)

    plt.savefig(
        os.path.join(output_dir, "comparison_reliability.jpg"),
        bbox_inches="tight"
    )
    plt.close()


def main():
    base = os.path.dirname(os.path.abspath(__file__))
    tracings_dir = os.path.join(base, "tracings")
    consistency_dir = os.path.join(base, "consistency")
    output_dir = os.path.join(base, "results")
    os.makedirs(output_dir, exist_ok=True)

    protocols = ['2pc', 'saga', 'tcc']
    tracing_stats = {}
    consistency_data = {}
    reliability_stats = {}

    for protocol in protocols:
        traces = count_tracings_for_protocol(protocol, tracings_dir)
        if not traces:
            continue

        durations = [t['duration_ms'] for t in traces]
        tracing_stats[protocol] = {
            'count': len(traces),
            'p90': np.percentile(durations, 90),
            'p95': np.percentile(durations, 95),
            'p99': np.percentile(durations, 99)
        }

        consistency = consolidate_consistency_data(protocol, consistency_dir)
        if not consistency:
            continue

        consistency_data[protocol] = consistency

        total = tracing_stats[protocol]['count']
        rollbacks = consistency['dtm_transactions'].get('rollbacks', 0)
        rollback_rate = (rollbacks / total) * 100

        if protocol == "2pc":
            total_orders = consistency['orders'].get('total', 0)
            total_failures = total - total_orders
            # Failure rate excludes rollbacks (rollbacks are separate metric)
            failures_excluding_rollbacks = total_failures - rollbacks
            failure_rate = (failures_excluding_rollbacks / total) * 100 if total > 0 else 0
        else:
            failed = consistency['orders'].get('failed', 0)
            # Failure rate excludes rollbacks (rollbacks are separate metric)
            failures_excluding_rollbacks = failed - rollbacks
            failure_rate = (failures_excluding_rollbacks / total) * 100 if total > 0 else 0

        reliability_stats[protocol] = {
            'rollback_rate': rollback_rate,
            'failure_rate': failure_rate,
            'total_failure_rate': rollback_rate + failure_rate
        }

        with open(os.path.join(output_dir, f"{protocol}_consistency_consolidated.json"), "w") as f:
            json.dump({
                'protocol': protocol,
                **consistency
            }, f, indent=4)

    if not reliability_stats:
        sys.exit(1)

    plot_reliability(reliability_stats, output_dir)


if __name__ == "__main__":
    main()
