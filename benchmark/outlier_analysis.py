#!/usr/bin/env python3

import json
import os
import numpy as np
import matplotlib.pyplot as plt
from collections import defaultdict

# =========================================================
# GLOBAL PLOT CONFIG — Academic / Paper Style
# =========================================================
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
# =========================================================


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


def process_file(file_path):
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
                'start_time_ns': t['start_time'],
                'end_time_ns': t['end_time'],
                'duration_ns': t['end_time'] - t['start_time']
            })

    return traces


def process_protocol_directory(protocol_dir):
    if not os.path.exists(protocol_dir):
        return []

    files = [f for f in os.listdir(protocol_dir) if f.endswith('.json')]
    all_traces = []

    for filename in files:
        traces = process_file(os.path.join(protocol_dir, filename))
        if not traces:
            continue

        file_start = min(t['start_time_ns'] for t in traces)
        for t in traces:
            all_traces.append({
                'trace_id': t['trace_id'],
                'relative_time_sec': (t['start_time_ns'] - file_start) / 1e9,
                'relative_end_sec': (t['end_time_ns'] - file_start) / 1e9,
                'duration_ms': nanoseconds_to_milliseconds(t['duration_ns'])
            })

    all_traces.sort(key=lambda x: x['relative_time_sec'])
    return all_traces


def plot_scatter_without_chaos(protocol_data, output_dir):
    """Scatter plot for period WITHOUT chaos (5-60s) - all protocols in one plot"""
    
    colors = {
        '2pc': '#1f77b4',
        'saga': '#2ca02c', 
        'tcc': '#d62728'
    }

    plt.figure(figsize=(12, 8))
    
    for protocol, traces in protocol_data.items():
        filtered = [
            t for t in traces
            if t['relative_time_sec'] >= 5 and t['relative_end_sec'] <= 60
        ]
        
        if not filtered:
            continue

        times = [t['relative_time_sec'] for t in filtered]
        durations = [t['duration_ms'] for t in filtered]
        
        plt.scatter(times, durations, 
                   color=colors[protocol], 
                   alpha=0.6, 
                   s=20,
                   label=protocol.upper(),
                   edgecolors='none')
    
    plt.xlabel("Time since benchmark start (s)")
    plt.ylabel("Duration (ms)")
    plt.grid(True, linestyle="--", alpha=0.3)
    plt.legend(
        loc="center left",
        bbox_to_anchor=(1.02, 0.5),
        frameon=False
    )
    
    plt.savefig(
        os.path.join(output_dir, "scatter_without_chaos_all_protocols.pdf"),
        bbox_inches="tight"
    )
    plt.close()


def plot_scatter_with_chaos(protocol_data, output_dir):
    """Scatter plot for period WITH chaos (69s+) - all protocols in one plot"""
    
    colors = {
        '2pc': '#1f77b4',
        'saga': '#2ca02c',
        'tcc': '#d62728'
    }

    plt.figure(figsize=(12, 8))

    for protocol, traces in protocol_data.items():
        filtered = [
            t for t in traces
            if t['relative_time_sec'] >= 69 and t['duration_ms'] > 1000
        ]
        
        if not filtered:
            continue

        times = [t['relative_time_sec'] for t in filtered]
        durations = [t['duration_ms'] / 1000.0 for t in filtered]  # Convert to seconds
        
        plt.scatter(times, durations, 
                   color=colors[protocol], 
                   alpha=0.6, 
                   s=20,
                   label=protocol.upper(),
                   edgecolors='none')
    
    plt.xlabel("Time since benchmark start (s)")
    plt.ylabel("Duration (s)")
    plt.grid(True, linestyle="--", alpha=0.3)
    plt.legend(
        loc="center left",
        bbox_to_anchor=(1.02, 0.5),
        frameon=False
    )
    
    plt.savefig(
        os.path.join(output_dir, "scatter_with_chaos_all_protocols.pdf"),
        bbox_inches="tight"
    )
    plt.close()


def main():
    base = os.path.dirname(os.path.abspath(__file__))
    tracings_dir = os.path.join(base, "tracings")
    output_dir = os.path.join(base, "outliers")
    os.makedirs(output_dir, exist_ok=True)

    protocols = ['2pc', 'saga', 'tcc']
    protocol_data = {}

    print("Processing protocol data...")
    for p in protocols:
        data = process_protocol_directory(os.path.join(tracings_dir, p))
        if data:
            protocol_data[p] = data
            print(f"  {p.upper()}: {len(data)} traces")

    if not protocol_data:
        print("No data found!")
        return

    print("\nGenerating scatter plots...")
    
    print("  Without chaos period (5-60s)...")
    plot_scatter_without_chaos(protocol_data, output_dir)
    
    print("  With chaos period (69s+)...")
    plot_scatter_with_chaos(protocol_data, output_dir)
    
    print(f"\nScatter plots saved in: {output_dir}")
    print("Files generated:")
    print("  - scatter_without_chaos_all_protocols.pdf")
    print("  - scatter_with_chaos_all_protocols.pdf")


if __name__ == "__main__":
    main()