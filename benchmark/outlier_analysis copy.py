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

    "font.size": 10,
    "axes.titlesize": 12,
    "axes.labelsize": 11,
    "legend.fontsize": 10,
    "xtick.labelsize": 10,
    "ytick.labelsize": 10,

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
    """Scatter plot for period WITHOUT chaos (5-60s) - no outlier removal"""
    
    colors = {
        '2pc': '#1f77b4',
        'saga': '#2ca02c', 
        'tcc': '#d62728'
    }

    for protocol, traces in protocol_data.items():
        filtered = [
            t for t in traces
            if t['relative_time_sec'] >= 5 and t['relative_end_sec'] <= 60
        ]
        
        if not filtered:
            continue

        plt.figure(figsize=(10, 6))
        
        times = [t['relative_time_sec'] for t in filtered]
        durations = [t['duration_ms'] for t in filtered]
        
        plt.scatter(times, durations, 
                   color=colors[protocol], 
                   alpha=0.6, 
                   s=20,
                   edgecolors='none')
        
        plt.xlabel("Time since benchmark start (s)")
        plt.ylabel("Duration (ms)")
        plt.grid(True, linestyle="--", alpha=0.3)
        
        # Add statistics info
        mean_duration = np.mean(durations)
        p95_duration = np.percentile(durations, 95)
        p99_duration = np.percentile(durations, 99)
        
        plt.text(0.02, 0.98, 
                f'Protocol: {protocol.upper()}\n'
                f'Traces: {len(filtered)}\n'
                f'Mean: {mean_duration:.2f} ms\n'
                f'P95: {p95_duration:.2f} ms\n'
                f'P99: {p99_duration:.2f} ms',
                transform=plt.gca().transAxes,
                verticalalignment='top',
                bbox=dict(boxstyle='round', facecolor='white', alpha=0.8))
        
        plt.savefig(
            os.path.join(output_dir, f"scatter_without_chaos_{protocol}.pdf"),
            bbox_inches="tight"
        )
        plt.close()


def plot_scatter_with_chaos(protocol_data, output_dir):
    """Scatter plot for period WITH chaos (69s+) - no outlier removal"""
    
    colors = {
        '2pc': '#1f77b4',
        'saga': '#2ca02c',
        'tcc': '#d62728'
    }

    for protocol, traces in protocol_data.items():
        filtered = [
            t for t in traces
            if t['relative_time_sec'] >= 69
        ]
        
        if not filtered:
            continue

        plt.figure(figsize=(10, 6))
        
        times = [t['relative_time_sec'] for t in filtered]
        durations = [t['duration_ms'] for t in filtered]
        
        plt.scatter(times, durations, 
                   color=colors[protocol], 
                   alpha=0.6, 
                   s=20,
                   edgecolors='none')
        
        plt.xlabel("Time since benchmark start (s)")
        plt.ylabel("Duration (ms)")
        plt.grid(True, linestyle="--", alpha=0.3)
        
        # Add statistics info
        mean_duration = np.mean(durations)
        p95_duration = np.percentile(durations, 95)
        p99_duration = np.percentile(durations, 99)
        
        plt.text(0.02, 0.98, 
                f'Protocol: {protocol.upper()}\n'
                f'Traces: {len(filtered)}\n'
                f'Mean: {mean_duration:.2f} ms\n'
                f'P95: {p95_duration:.2f} ms\n'
                f'P99: {p99_duration:.2f} ms',
                transform=plt.gca().transAxes,
                verticalalignment='top',
                bbox=dict(boxstyle='round', facecolor='white', alpha=0.8))
        
        plt.savefig(
            os.path.join(output_dir, f"scatter_with_chaos_{protocol}.pdf"),
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
    for protocol in protocols:
        if protocol in protocol_data:
            print(f"  - scatter_without_chaos_{protocol}.pdf")
            print(f"  - scatter_with_chaos_{protocol}.pdf")


if __name__ == "__main__":
    main()