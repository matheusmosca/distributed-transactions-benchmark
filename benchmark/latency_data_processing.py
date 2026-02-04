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


def remove_outliers(traces):
    if not traces:
        return []

    durations = np.array([t['duration_ms'] for t in traces])
    q1, q3 = np.percentile(durations, [25, 75])
    iqr = q3 - q1
    lo, hi = q1 - 1.5 * iqr, q3 + 1.5 * iqr

    return [t for t in traces if lo <= t['duration_ms'] <= hi]


# =========================================================
# TIME SERIES — WITHOUT CHAOS
# =========================================================
def plot_comparison_without_chaos(protocol_data, output_dir):
    plt.figure(figsize=(7.5, 4.6))

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
        filtered = remove_outliers(filtered)
        if not filtered:
            continue

        plt.plot(
            [t['relative_time_sec'] for t in filtered],
            [t['duration_ms'] for t in filtered],
            label=protocol.upper(),
            color=colors[protocol],
            linewidth=1.5,
            alpha=0.85
        )

    plt.xlabel("Time since benchmark start (s)")
    plt.ylabel("Duration (ms)")
    plt.grid(True, linestyle="--", alpha=0.3)

    # 🔹 LEGENDA FORA, À DIREITA
    plt.legend(
        loc="center left",
        bbox_to_anchor=(1.02, 0.5),
        frameon=False
    )

    plt.savefig(
        os.path.join(output_dir, "protocols_comparison_without_chaos.pdf"),
        bbox_inches="tight"
    )
    plt.close()


# =========================================================
# STATISTICS BARS — PAPER FRIENDLY
# =========================================================
def plot_statistics_bars(protocol_data, output_dir, suffix, time_filter):
    metrics = ['mean', 'median', 'p95', 'p99'] if "Without" in suffix else ['p95', 'p99']
    labels = [m.upper() for m in metrics]
    
    # Determine unit and label based on suffix
    if "Without" in suffix:
        unit_label = "Duration (ms)"
        convert_to_seconds = False
    else:
        unit_label = "Duration (s)"
        convert_to_seconds = True

    stats = {}
    for protocol, traces in protocol_data.items():
        values = [t['duration_ms'] for t in traces if time_filter(t)]
        if not values:
            continue
        
        # Convert to seconds if needed
        if convert_to_seconds:
            values = [v / 1000.0 for v in values]

        stats[protocol.upper()] = {
            'mean': np.mean(values),
            'median': np.median(values),
            'p90': np.percentile(values, 90),
            'p95': np.percentile(values, 95),
            'p99': np.percentile(values, 99),
        }

    if not stats:
        return

    x = np.arange(len(metrics))
    width = 0.25

    fig, ax = plt.subplots(figsize=(7.5, 4.6))

    colors = {
        '2PC': '#1f77b4',
        'SAGA': '#2ca02c',
        'TCC': '#d62728'
    }

    for i, (proto, s) in enumerate(stats.items()):
        bars = ax.bar(
            x + (i - len(stats)/2 + 0.5) * width,
            [s[m] for m in metrics],
            width,
            label=proto,
            color=colors[proto],
            edgecolor="black",
            linewidth=0.6
        )

        for bar in bars:
            height = bar.get_height()
            ax.text(
                bar.get_x() + bar.get_width()/2.,
                height,
                f'{height:.2f}',
                ha='center',
                va='bottom',
                fontsize=10
            )

    ax.set_xlabel("Metric")
    ax.set_ylabel(unit_label)
    ax.set_xticks(x)
    ax.set_xticklabels(labels)
    ax.legend(
        loc="center left",
        bbox_to_anchor=(1.02, 0.5),
        frameon=False
    )
    ax.grid(axis="y", linestyle="--", alpha=0.3)

    plt.savefig(
        os.path.join(output_dir, f"statistics_{suffix.lower().replace(' ', '_')}.pdf"),
        bbox_inches="tight"
    )
    plt.close()


def main():
    base = os.path.dirname(os.path.abspath(__file__))
    tracings_dir = os.path.join(base, "tracings")
    output_dir = os.path.join(base, "results")
    os.makedirs(output_dir, exist_ok=True)

    protocols = ['2pc', 'saga', 'tcc']
    protocol_data = {}

    for p in protocols:
        data = process_protocol_directory(os.path.join(tracings_dir, p))
        if data:
            protocol_data[p] = data

    plot_comparison_without_chaos(protocol_data, output_dir)

    plot_statistics_bars(
        protocol_data,
        output_dir,
        "Period Without Chaos Injection",
        lambda t: t['relative_end_sec'] <= 60
    )

    plot_statistics_bars(
        protocol_data,
        output_dir,
        "Period With Chaos Injection",
        lambda t: t['relative_time_sec'] >= 69
    )

    print("✅ ANALYSIS COMPLETED")


if __name__ == "__main__":
    main()
