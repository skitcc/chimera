#!/usr/bin/env python3
"""Draw the perf-run figures and the JSON summaries from Prometheus and k6."""

import gzip
import datetime
import json
import math
import os
import re
import sys
import urllib.parse
import urllib.request
from pathlib import Path

WINDOW_SEC = 1
PERCENTILES = (0.5, 0.75, 0.9, 0.95, 0.99)
PLOT_PERCENTILES = (0.5, 0.9, 0.99)
PLOT_FILES = {
    'latency_over_time.png', 'latency_distribution.png',
    'latency_percentiles.png', 'latency_histogram.png',
    'cpu_over_time.png', 'ram_over_time.png', 'rps_over_time.png',
}
OBSOLETE_PLOT_FILES = {
    'latency_by_step.png', 'sessions_over_time.png',
    'stream_bytes_over_time.png', 'errors_over_time.png', 'expected_faults_over_time.png',
    'disk_read_over_time.png', 'disk_write_over_time.png',
    'host_disk_over_time.png', 'loadavg_over_time.png',
}


def operation_report(raw, start=-math.inf, end=math.inf):
    rows = [r for r in raw.get('http_req_duration', []) if start <= r[0] < end]
    failures = [r for r in raw.get('http_req_failed', []) if start <= r[0] < end]
    result = {}
    for name in sorted({r[2].get('step', 'unknown') for r in rows}):
        selected = [r for r in rows if r[2].get('step', 'unknown') == name]
        ordered = sorted(r[1] for r in selected)
        idx = (len(ordered) - 1) * .95
        lo = int(idx)
        statuses = {}
        for _, _, tags in selected:
            status = tags.get('status', 'unknown')
            statuses[status] = statuses.get(status, 0) + 1
        bad = sum(r[1] for r in failures if r[2].get('step', 'unknown') == name)
        result[name] = {
            'requests': len(selected), 'unexpected_failures': bad,
            'error_rate': bad / len(selected), 'statuses': statuses,
            'p95_ms': ordered[lo] + (ordered[min(lo+1, len(ordered)-1)] - ordered[lo]) * (idx-lo),
        }
    return result


def load_json(path):
    file = Path(path)
    if not file.exists():
        return None
    return json.loads(file.read_text())


def write_json(path, body):
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    Path(path).write_text(json.dumps(body, indent=2) + "\n")


def scenario_start(out):
    text = (out / "k6.log").read_text(errors="replace") if (out / "k6.log").exists() else ""
    marks = [int(m.group(1)) / 1000 for m in re.finditer(r"SCENARIO_START (\d+)", text)]
    if marks:
        return min(marks)
    run = load_json(out / "run.json") or {}
    summary = load_json(out / "summary.json") or {}
    end = run.get("end_unix")
    duration_ms = summary.get("test_run_ms")
    if end and duration_ms:
        return float(end) - float(duration_ms) / 1000
    return run.get("start_unix")


def query_range(base, query, start, end, step=WINDOW_SEC):
    if start is None or end is None or end <= start:
        return []
    params = urllib.parse.urlencode({
        "query": query,
        "start": f"{start:.3f}",
        "end": f"{end:.3f}",
        "step": str(step),
    })
    url = f"{base.rstrip('/')}/api/v1/query_range?{params}"
    try:
        with urllib.request.urlopen(url, timeout=30) as response:
            payload = json.loads(response.read().decode())
    except (OSError, json.JSONDecodeError, TimeoutError):
        return []
    if payload.get("status") != "success":
        return []
    series = []
    for item in payload.get("data", {}).get("result", []):
        points = []
        for ts, raw in item.get("values", []):
            try:
                value = float(raw)
            except (TypeError, ValueError):
                continue
            if math.isnan(value) or math.isinf(value):
                continue
            points.append((float(ts), value))
        if points:
            labels = item.get("metric", {})
            series.append({"labels": labels, "points": points})
    return series


def first_query(base, queries, start, end, step=WINDOW_SEC):
    for query in queries:
        found = query_range(base, query, start, end, step)
        if found:
            return found
    return []


def save_series(out, name, series):
    body = []
    for item in series:
        label = item["labels"].get("container") or item["labels"].get("step") or item["labels"].get("le") or item.get("label") or "value"
        body.append({"label": label, "labels": item["labels"], "points": item["points"]})
    write_json(out / "series" / f"{name}.json", body)


def summarize(points):
    values = [value for _, value in points]
    if not values:
        return None
    ordered = sorted(values)

    def pct(p):
        index = min(len(ordered) - 1, max(0, int(round(p * (len(ordered) - 1)))))
        return ordered[index]

    stat = {"min": ordered[0], "max": ordered[-1], "avg": sum(ordered) / len(ordered)}
    for p in PERCENTILES:
        stat[f"p{int(p * 100)}"] = pct(p)
    stat["p50"] = stat["p50"]
    return stat


def percentile_from_buckets(series, p):
    """Interpolate cumulative buckets, including the total in +Inf."""
    by_time = {}
    for item in series:
        edge = float(item["labels"]["le"])
        for ts, value in item["points"]:
            by_time.setdefault(ts, []).append((edge, value))
    points = []
    for ts, rows in sorted(by_time.items()):
        rows.sort()
        total = rows[-1][1]
        if total <= 0:
            continue
        target = total * p
        low, count = 0.0, 0.0
        for edge, cumulative in rows:
            if cumulative >= target:
                value = low if math.isinf(edge) else low + (edge - low) * (target - count) / max(cumulative - count, 1e-12)
                points.append((ts, value))
                break
            low, count = edge, cumulative
    return points


def raw_metrics(out):
    """Exact HTTP latency windows from k6 samples; setup and warmup excluded."""
    path = out / "samples.json.gz"
    if not path.exists():
        raise ValueError("raw k6 samples missing")
    metrics = {}
    with gzip.open(path, 'rt') as source:
        for line in source:
            row = json.loads(line)
            if row.get('type') != 'Point':
                continue
            data = row['data']
            if data.get('tags', {}).get('phase') != 'measure':
                continue
            ts = datetime.datetime.fromisoformat(data['time'].replace('Z', '+00:00')).timestamp()
            metrics.setdefault(row['metric'], []).append((ts, float(data['value']), data.get('tags', {})))
    if not metrics.get('http_req_duration') or not metrics.get('sessions'):
        raise ValueError("no measured HTTP requests or completed sessions: setup/scenario failed")
    return metrics


def raw_windows(samples, origin, end, percentile=None, include_empty=False):
    window_count = int(math.floor((end - origin) / WINDOW_SEC + 1e-9))
    if window_count <= 0:
        return []
    windows = [[] for _ in range(window_count)]
    for timestamp, value, _ in samples:
        if timestamp < origin:
            continue
        index = int((timestamp - origin) // WINDOW_SEC)
        if 0 <= index < window_count:
            windows[index].append(value)

    points = []
    for index, values in enumerate(windows):
        cursor = origin + (index + 1) * WINDOW_SEC
        if not values:
            if include_empty and percentile is None:
                points.append((cursor, 0))
            continue
        if percentile is None:
            value = sum(values) / WINDOW_SEC
        else:
            ordered = sorted(values)
            rank = (len(ordered) - 1) * percentile
            lo = int(rank)
            value = ordered[lo] + (ordered[min(lo + 1, len(ordered)-1)] - ordered[lo]) * (rank-lo)
        points.append((cursor, value))
    return points


def slice_points(points, start, end):
    return [(ts, value) for ts, value in points if start <= ts < end]


def phase_bounds(summary, origin):
    bounds = []
    cursor = origin or 0
    for phase in summary.get("phases") or []:
        nxt = cursor + float(phase["sec"])
        rate = phase.get("rps")
        target = rate if rate is not None else phase.get("vus")
        bound = {
            "start": cursor, "end": nxt, "vus": phase.get("vus", target),
            "sec": phase["sec"],
            "label": f"{float(rate):g} RPS" if rate is not None else f"{phase.get('vus')} VU",
        }
        if rate is not None:
            bound["rps"] = rate
        bounds.append(bound)
        cursor = nxt
    return bounds


def offered_rate_points(summary, origin):
    points = []
    for bound in phase_bounds(summary, origin):
        if bound.get("rps") is None:
            return []
        points.append((bound["start"], float(bound["rps"])))
        points.append((bound["end"], float(bound["rps"])))
    return points


def stage_report(summary, origin, latency, errors, rps, raw=None):
    warmup = float(summary.get("warmup_sec") or 0)
    latency_slo = float(os.environ.get("LATENCY_SLO_MS", "500"))
    stages = []
    previous = None
    for index, bound in enumerate(phase_bounds(summary, origin)):
        start = bound["start"] + (warmup if index == 0 else 0)
        lat = summarize(slice_points(latency, start, bound["end"]))
        err = summarize(slice_points(errors, start, bound["end"]))
        rate = summarize(slice_points(rps, start, bound["end"]))
        if raw:
            stage_values = [v for ts, v, _ in raw['http_req_duration'] if start <= ts < bound['end']]
            if stage_values:
                lat = {'avg': 0}
                rate = {'avg': 0}
        target = bound["rps"] if bound.get("rps") is not None else bound.get("vus")
        if lat is None or rate is None:
            empty = {
                "vus": bound["vus"], "p95_ms": None, "error_rate": None, "rps": None,
                "stable": False, "failed": False,
            }
            if bound.get("rps") is not None:
                empty["target_rps"] = bound["rps"]
            stages.append(empty)
            continue
        p95 = lat["avg"]
        error_rate = err["avg"] if err else 0
        throughput = rate["avg"] if rate else 0
        if raw:
            requests = [v for ts, v, _ in raw['http_req_duration'] if start <= ts < bound['end']]
            failures = [v for ts, v, _ in raw.get('http_req_failed', []) if start <= ts < bound['end']]
            if requests:
                ordered = sorted(requests)
                idx = (len(ordered) - 1) * .95
                lo = int(idx)
                p95 = ordered[lo] + (ordered[min(lo+1, len(ordered)-1)] - ordered[lo]) * (idx-lo)
                error_rate = sum(failures) / len(requests)
                throughput = len(requests) / (bound['end'] - start)
        # Relative jumps on short stages are noisy; they are a warning, not
        # evidence that throughput saturated. Judge the configured absolute SLO.
        previous_target = previous["target"] if previous else None
        growing = previous is None or target <= previous_target or throughput > previous["rps"]
        shortfall = bound.get("rps") is not None and throughput < 0.8 * float(bound["rps"])
        failed = error_rate >= 0.01 or p95 > latency_slo or not growing or shortfall
        growth = p95 / previous["p95"] if previous and previous["p95"] else None
        row = {"vus": bound["vus"], "p95_ms": p95, "error_rate": error_rate,
               "rps": throughput, "stable": not failed, "failed": failed,
               "p95_growth": growth, "latency_growth_warning": growth is not None and growth > 2}
        if bound.get("rps") is not None:
            row["target_rps"] = bound["rps"]
        if raw:
            row['operations'] = operation_report(raw, start, bound['end'])
            row['object_store_rps'] = sum(values['requests'] for name, values in row['operations'].items() if name in ('upload', 'stream-body')) / (bound['end'] - start)
            row['api_rps'] = throughput - row['object_store_rps']
            for values in row['operations'].values():
                values['rps'] = values['requests'] / (bound['end'] - start)
            row['failed_operations'] = [name for name, values in row['operations'].items() if values['error_rate'] >= .01]
        row['failure_reasons'] = (["unexpected_errors>=1%"] if error_rate >= .01 else []) + (["p95_above_slo"] if p95 > latency_slo else []) + (["rps_did_not_grow"] if not growing else []) + (["achieved_below_80pct_of_target"] if shortfall else [])
        stages.append(row)
        previous = {"p95": p95, "rps": throughput, "vus": bound["vus"], "target": target}
    stable_vus = None
    failed_vus = None
    stable_http = None
    uses_rps = any(row.get("target_rps") is not None for row in stages)
    for row in stages:
        key = row["target_rps"] if row.get("target_rps") is not None else row["vus"]
        if row["stable"]:
            stable_vus = key
            stable_http = row["rps"]
        elif row["failed"] and failed_vus is None:
            failed_vus = key
            break
    return {"stable_vus": stable_vus, "failed_vus": failed_vus, "latency_slo_ms": latency_slo,
            "stable_target_rps": stable_vus if uses_rps else None,
            "failed_target_rps": failed_vus if uses_rps else None,
            "stable_http_rps": stable_http if uses_rps else None,
            "capacity_found": failed_vus is not None,
            "max_tested_vus": max((x.get("target_rps", x["vus"]) for x in stages if x.get("target_rps", x["vus"]) is not None), default=None),
            "stages": stages}


def recovery_report(summary, origin, latency, errors, raw=None):
    bounds = phase_bounds(summary, origin)
    if len(bounds) < 3:
        return {"recovered": False, "reason": "stress phases are missing"}
    warmup = float(summary.get("warmup_sec") or 0)
    base = summarize(slice_points(latency, bounds[0]["start"] + warmup, bounds[0]["end"]))
    base_p95 = base["avg"] if base else None
    if raw:
        values = sorted(v for ts, v, _ in raw['http_req_duration'] if bounds[0]['start'] + warmup <= ts < bounds[0]['end'])
        if values:
            idx = (len(values) - 1) * .95
            lo = int(idx)
            base_p95 = values[lo] + (values[min(lo + 1, len(values) - 1)] - values[lo]) * (idx-lo)
    return_at = bounds[1]["end"]
    deadline = bounds[2]["end"]
    if base_p95 is None:
        return {"recovered": False, "reason": "baseline p95 is missing", "return_unix": return_at}
    degraded = any(return_at >= ts >= bounds[1]['start'] + WINDOW_SEC and (value > base_p95 * 1.2 or dict(errors).get(ts, 0) >= .01) for ts, value in latency)
    details = {'baseline_p95_ms': base_p95, 'threshold_p95_ms': base_p95 * 1.2,
               'window_sec': WINDOW_SEC, 'error_threshold': .01, 'overload_degraded': degraded,
               'return_unix': return_at, 'observation_sec': deadline - return_at}
    for ts, value in latency:
        if ts < return_at + WINDOW_SEC or ts > deadline:
            continue
        window_err = dict(errors).get(ts)
        if window_err is None:
            continue
        err = window_err
        if err < 0.01 and value <= base_p95 * 1.2:
            relapses = [{'end_sec_since_return': later_ts - return_at, 'p95_ms': later_value,
                         'error_rate': dict(errors).get(later_ts)}
                        for later_ts, later_value in latency if ts < later_ts <= deadline
                        and (later_value > base_p95 * 1.2 or dict(errors).get(later_ts, 0) >= .01)]
            return {
                **details, "recovered": True, "recovery_mode": "first_healthy_1s_window",
                "relapse_windows": relapses, "no_observed_relapse": not relapses,
                "recovery_sec": ts - return_at,
                "window_p95_ms": value, "window_error_rate": err,
            }
    return {**details, "recovered": False, "recovery_sec": None, "reason": "did not recover within the window after the overload"}


def write_readable_report(out, measured, knee, recovery, phases=None):
    lines = ['# Результат нагрузки', '',
             f"Измерено {measured['requests']} HTTP-запросов; неожиданные отказы: {int(measured['unexpected_failures'])} ({measured['error_rate']:.2%}).",
             f"p50={measured['p50_ms']:.2f} мс; p75={measured['p75_ms']:.2f}; p90={measured['p90_ms']:.2f}; p95={measured['p95_ms']:.2f}; p99={measured['p99_ms']:.2f}.", '',
             'RPS включает HTTP к API и к объектному хранилищу. Ошибки HTTP и превышение порога задержки не равны падению процесса; рестарты проверяются по Kubernetes и логам.', '']
    if knee:
        offered = knee.get('stable_target_rps') is not None or any(row.get('target_rps') is not None for row in knee['stages'])
        goal = 'Цель, HTTP RPS' if offered else 'VU'
        lines += ['## Ступени ramp', '', f'| {goal} | HTTP RPS всего | API RPS | p95, мс | Ошибки | Причины нарушения |', '| --- | --- | --- | --- | --- | --- |']
        for row in knee['stages']:
            name = row['target_rps'] if row.get('target_rps') is not None else row['vus']
            if row['p95_ms'] is None:
                lines.append(f"| {name} | — | — | — | — | Недостаточно данных |")
            else:
                lines.append(f"| {name} | {row['rps']:.2f} | {row.get('api_rps', row['rps']):.2f} | {row['p95_ms']:.2f} | {row['error_rate']:.2%} | {', '.join(row.get('failure_reasons', [])) or 'Нет'} |")
        if offered and not knee['capacity_found']:
            lines += ['', f"До цели {knee['max_tested_vus']} HTTP RPS предел не определён. Это максимум проверенной нагрузки, а не предел системы. Последняя устойчивая цель дала {knee.get('stable_http_rps')} измеренных HTTP RPS."]
        elif not knee['capacity_found']:
            lines += ['', f"До {knee['max_tested_vus']} VU предел не определён. Это максимум проверенной нагрузки, а не предел системы."]
        elif offered and knee.get('stable_target_rps') is None:
            lines += ['', f"Уже первая цель {knee.get('failed_target_rps')} HTTP RPS нарушает критерий. Причины и операции — в knee.json."]
        elif offered:
            measured = knee.get('stable_http_rps')
            shown = f"{measured:.2f}" if isinstance(measured, (int, float)) else measured
            lines += ['', f"Последняя устойчивая цель: {knee.get('stable_target_rps')} HTTP RPS (измерено {shown}). Первая цель с нарушением: {knee.get('failed_target_rps')} HTTP RPS. Причины и операции — в knee.json."]
        else:
            lines += ['', f"Первая ступень с нарушением критерия: {knee['failed_vus']} VU. Точные причины, RPS и проблемные операции — в knee.json."]
    if recovery and phases:
        offered = any(row.get('target_rps') is not None for row in phases)
        goal = 'Цель, HTTP RPS' if offered else 'VU'
        lines += ['', '## Фазы stress', '', f'| {goal} | HTTP RPS | API RPS | p95, мс | Ошибки |', '| --- | --- | --- | --- | --- |']
        for row in phases:
            if row['p95_ms'] is not None:
                name = row['target_rps'] if row.get('target_rps') is not None else row['vus']
                lines.append(f"| {name} | {row['rps']:.2f} | {row.get('api_rps', row['rps']):.2f} | {row['p95_ms']:.2f} | {row['error_rate']:.2%} |")
    if recovery:
        lines += ['', '## Восстановление stress', '']
        if recovery.get('recovered'):
            lines += [f"Критерий recovered выполнен через {recovery['recovery_sec']:.0f} с после снятия перегруза.",
                      f"Базовый p95: {recovery['baseline_p95_ms']:.2f} мс; порог: {recovery['threshold_p95_ms']:.2f} мс; окно: {recovery['window_sec']} с."]
            if recovery.get('relapse_windows'):
                lines += ['После первого здорового окна порог снова нарушался. Флаг recovered не означает устойчивую нормализацию до конца прогона; см. relapse_windows в recovery.json.']
            if not recovery.get('overload_degraded'):
                lines += ['Во время перегруза ухудшение по этому критерию не зафиксировано; это не доказательство предшествовавшего отказа.']
        else:
            lines += [f"Восстановление в окне наблюдения не подтверждено: {recovery.get('reason')}."]
        lines += ['Проверяется одно полное окно API. Очереди Kafka, состояние всех подов и длительная стабильность этим флагом не проверяются.']
    lines += ['', '## Операции', '', '| Шаг | Запросы | Неожиданные отказы | p95, мс |', '| --- | --- | --- | --- |']
    for name, values in measured['steps'].items():
        lines.append(f"| {name} | {values['requests']} | {int(values['unexpected_failures'])} | {values['p95_ms']:.2f} |")
    lines += ['', 'HTTP-статусы по операциям — http.json; по ступеням — knee.json; по 1-секундным окнам — windows.json. CPU и RAM — resources.json и plots/.']
    (out / 'report.md').write_text('\n'.join(lines) + '\n')


def resource_report(cpu, ram, read, write):
    by_name = {}

    def index(series):
        out = {}
        for item in series:
            name = resource_name(item)
            if name:
                out[name] = item["points"]
        return out

    cpu_i, ram_i, read_i, write_i = index(cpu), index(ram), index(read), index(write)
    report = {"gpu": None, "components": []}
    for name in sorted(set(cpu_i) | set(ram_i) | set(read_i) | set(write_i)):
        report["components"].append({
            "component": name,
            "cpu_cores": summarize(cpu_i.get(name, [])),
            "ram_bytes": summarize(ram_i.get(name, [])),
            "disk_read_bps": summarize(read_i.get(name, [])),
            "disk_write_bps": summarize(write_i.get(name, [])),
        })
    return report


def resource_name(item):
    tags = item.get('labels', {})
    return '/'.join(tags[key] for key in ('namespace', 'container') if tags.get(key))


def parse_iostat(text):
    header = None
    block = []
    samples = []

    def flush():
        if not block:
            return
        samples.append(max(block, key=lambda row: row["util"]))

    for line in text.splitlines():
        if "Device" in line and "%util" in line:
            flush()
            header = line.split()
            block = []
            continue
        if not header:
            continue
        parts = line.split()
        if len(parts) != len(header):
            continue
        row = dict(zip(header, parts))
        device = row.get("Device", "")
        if device.startswith("loop") or device.startswith("ram"):
            continue
        try:
            samples_row = {
                "device": device,
                "util": float(row.get("%util", "0")),
                "read_kb": float(row.get("rkB/s", row.get("rKB/s", "0"))),
                "write_kb": float(row.get("wkB/s", row.get("wKB/s", "0"))),
            }
        except ValueError:
            continue
        block.append(samples_row)
    flush()
    return samples


def parse_uptime(text):
    points = []
    for index, line in enumerate(text.splitlines()):
        match = re.search(r"load average: ([0-9.]+), ([0-9.]+), ([0-9.]+)", line)
        if match:
            points.append((index * 5, float(match.group(1))))
    return points


def grouped_events(vlines):
    grouped = {}
    for x, label in vlines or []:
        labels = grouped.setdefault(round(float(x), 6), [])
        if label not in labels:
            labels.append(label)
    return [(x, '; '.join(labels)) for x, labels in sorted(grouped.items())]


def draw(path, title, lines, ylabel, vlines=None, phases=None):
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    from matplotlib.ticker import FuncFormatter, MaxNLocator

    events = grouped_events(vlines)
    active = sum(bool(points) for _, points in lines)
    legend_rows = math.ceil(active / 3)
    fig = plt.figure(figsize=(12, 7.5 + max(0, legend_rows - 1) * .24))
    grid = fig.add_gridspec(4, 1, height_ratios=[.65, 4.7, max(.4, legend_rows * .3),
                                              max(.9, len(events) * .4)], hspace=.4)
    ax = fig.add_subplot(grid[1])
    band = fig.add_subplot(grid[0], sharex=ax)
    legend = fig.add_subplot(grid[2])
    legend.axis('off')
    notes = fig.add_subplot(grid[3])
    fig.suptitle(title, fontsize=12, fontweight='bold', y=.98)
    ax.set_ylabel(ylabel)
    if ylabel == "RAM, ГиБ":
        ax.yaxis.set_major_locator(MaxNLocator(nbins=6))
        ax.yaxis.set_major_formatter(FuncFormatter(lambda value, _: f"{value:.1f}"))
    ax.set_xlabel('Время от начала сценария, с')
    ax.grid(alpha=.2)
    plotted = False
    for label, points in lines:
        if not points:
            continue
        ax.plot([ts for ts, _ in points], [value for _, value in points], label=label)
        plotted = True
    if plotted:
        handles, labels = ax.get_legend_handles_labels()
        legend.legend(handles, labels, loc='center', ncol=min(3, active),
                      fontsize=8, frameon=False)
    else:
        ax.text(.5, .5, 'Нет измерений', ha='center', va='center', transform=ax.transAxes)
    # Phase duration/target goes into a dedicated horizontal band, outside data.
    band.set_ylim(0, 1)
    band.axis('off')
    colors = ['#e5eef8', '#fff0d4']
    for index, phase in enumerate(phases or []):
        start, end = phase['start'], phase['end']
        band.axvspan(start, end, color=colors[index % 2], ec='white')
        target_label = phase['label'] if 'label' in phase else f"{phase['vus']} VU"
        band.text((start + end) / 2, .5, f"{start:g}–{end:g} с\n{target_label}",
                  ha='center', va='center', fontsize=9)
        if index:
            ax.axvline(start, color='#aaa', linewidth=.7, linestyle=':')
    if phases:
        ax.axvline(phases[-1]['end'], color='#aaa', linewidth=.7, linestyle=':')
        max_time = max((ts for _, points in lines for ts, _ in points), default=phases[-1]['end'])
        if max_time > phases[-1]['end']:
            band.text((phases[-1]['end'] + max_time) / 2, .5, 'Финиш', ha='center', fontsize=8)
    notes.axis('off')
    for index, (x, label) in enumerate(events, 1):
        ax.axvline(x, color='#40556b', linewidth=1, linestyle='--')
        ax.annotate(str(index), xy=(x, 1), xycoords=('data', 'axes fraction'),
                    xytext=(0, 4), textcoords='offset points', ha='center', va='bottom',
                    fontsize=9, bbox={'boxstyle': 'circle,pad=.2', 'fc': 'white', 'ec': '#40556b'})
        notes.text(.01, .88 - (index - 1) / max(len(events), 1), f'{index}. {x:g} с — {label}',
                   va='top', fontsize=10, transform=notes.transAxes)
    fig.subplots_adjust(top=.91, bottom=.04, left=.09, right=.98)
    fig.savefig(path, dpi=150)
    plt.close(fig)


def rel(points, origin):
    if origin is None:
        return points
    return [(ts - origin, value) for ts, value in points]


def title_of(summary, text):
    return f"{summary.get('profile', '')} {summary.get('scenario', '')} fault {summary.get('fault_rate', 0)} — {text}"


def run_dir(out):
    out = Path(out)
    plots = out / "plots"
    plots.mkdir(parents=True, exist_ok=True)
    # Remove only the known generated charts retired from the report.
    for name in OBSOLETE_PLOT_FILES:
        (plots / name).unlink(missing_ok=True)
    (out / "series").mkdir(parents=True, exist_ok=True)
    summary = load_json(out / "summary.json") or {}
    run = load_json(out / "run.json") or {}
    origin = scenario_start(out)
    end = run.get("end_unix") or (origin + 1 if origin else None)
    prom = os.environ.get("PROM_URL")
    step = WINDOW_SEC
    raw = raw_metrics(out)

    # Request/session plots use the raw file. Only container resources need Prometheus.
    def resource_series(name, queries):
        found = first_query(prom, queries, origin, end, step) if prom else []
        if found:
            return found
        return load_json(out / 'series' / f'{name}.json') or []

    container = 'container!="",container!="POD",pod!=""'
    cpu = resource_series("cpu", [
        f'sum by (namespace, container) (max by (namespace, pod, container) (rate(container_cpu_usage_seconds_total{{{container}}}[30s])))',
    ])
    ram = resource_series("ram", [
        f'sum by (namespace, container) (max by (namespace, pod, container) (container_memory_working_set_bytes{{{container}}}))',
    ])
    cpu_detailed = resource_series('cpu_detailed', [
        f'max by (namespace, pod, container) (rate(container_cpu_usage_seconds_total{{{container}}}[30s]))',
    ])
    ram_detailed = resource_series('ram_detailed', [
        f'max by (namespace, pod, container) (container_memory_working_set_bytes{{{container}}})',
    ])
    save_series(out, 'cpu_detailed', cpu_detailed)
    save_series(out, 'ram_detailed', ram_detailed)
    disk_read = resource_series("disk_read", [
        f'sum by (namespace, container) (rate(container_fs_reads_bytes_total{{{container}}}[30s]))',
        f'sum by (namespace, container) (rate(container_blkio_device_usage_bytes_total{{{container},operation="read"}}[30s]))',
    ])
    disk_write = resource_series("disk_write", [
        f'sum by (namespace, container) (rate(container_fs_writes_bytes_total{{{container}}}[30s]))',
        f'sum by (namespace, container) (rate(container_blkio_device_usage_bytes_total{{{container},operation="write"}}[30s]))',
    ])
    for name, series in (("cpu", cpu), ("ram", ram), ("disk_read", disk_read), ("disk_write", disk_write)):
        save_series(out, name, series)

    def one(series):
        return series[0]["points"] if series else []

    def raw_series(metric):
        points = raw_windows(raw.get(metric, []), origin, end, include_empty=True)
        measure_from = origin + float(summary.get('warmup_sec', 0))
        return [{"labels": {}, "points": [(ts, value) for ts, value in points if ts - WINDOW_SEC >= measure_from]}]

    rps = raw_series('http_reqs')
    sessions = raw_series('sessions')
    stream = raw_series('stream_bytes')
    faults = raw_series('expected_fault')
    rps_pts = one(rps)
    latencies = raw['http_req_duration']
    def latency_stats(rows):
        ordered = sorted(value for _, value, _ in rows)
        stats = {"requests": len(ordered), "avg_ms": sum(ordered) / len(ordered)}
        for p in PERCENTILES:
            idx = (len(ordered) - 1) * p
            lo = int(idx)
            stats[f"p{int(p*100)}_ms"] = ordered[lo] + (ordered[min(lo+1, len(ordered)-1)] - ordered[lo]) * (idx-lo)
        return stats

    statuses = {}
    for _, _, tags in latencies:
        status = tags.get('status', 'unknown')
        statuses[status] = statuses.get(status, 0) + 1
    measured = latency_stats(latencies)
    measured['unexpected_failures'] = sum(v for _, v, _ in raw.get('http_req_failed', []))
    measured['error_rate'] = measured['unexpected_failures'] / len(latencies)
    measured['sessions'] = sum(v for _, v, _ in raw.get('sessions', []))
    measured['failed_logins'] = sum(v for _, v, _ in raw.get('failed_sessions', []))
    measured['expected_fault_actions'] = sum(v for _, v, _ in raw.get('expected_fault', []))
    measured['stream_bytes'] = sum(v for _, v, _ in raw.get('stream_bytes', []))
    measured['statuses'] = statuses
    measured['steps'] = {name: latency_stats([r for r in latencies if r[2].get('step') == name])
                         for name in sorted({r[2].get('step') for r in latencies})}
    for name, values in operation_report(raw).items():
        measured['steps'][name].update(values)
    write_json(out / 'http.json', measured)
    p50 = raw_windows(latencies, origin, end, 0.5)
    p75 = raw_windows(latencies, origin, end, 0.75)
    p90 = raw_windows(latencies, origin, end, 0.9)
    p95 = raw_windows(latencies, origin, end, 0.95)
    p99 = raw_windows(latencies, origin, end, 0.99)
    bad = raw_windows(raw.get('http_req_failed', []), origin, end)
    err_pts = [(ts, value / dict(rps_pts)[ts]) for ts, value in bad if dict(rps_pts).get(ts)]
    for name, series in (("rps", rps), ("sessions", sessions), ("stream_bytes", stream), ("expected_fault", faults)):
        save_series(out, name, series)
    save_series(out, "latency_p95", [{"labels": {}, "points": p95}])
    save_series(out, "error_share", [{"labels": {}, "points": err_pts}])
    measure_start = origin + float(summary.get('warmup_sec', 0))
    cpu = [{**item, 'points': [(ts, v) for ts, v in item['points'] if ts >= measure_start]} for item in cpu]
    disk_read = [{**item, "points": [(ts, v) for ts, v in item["points"] if ts >= measure_start]} for item in disk_read]
    disk_write = [{**item, "points": [(ts, v) for ts, v in item["points"] if ts >= measure_start]} for item in disk_write]
    ram = [{**item, 'points': [(ts, v) for ts, v in item['points'] if ts >= measure_start]} for item in ram]
    if not any(item['points'] for item in cpu) or not any(item['points'] for item in ram):
        raise ValueError("cAdvisor CPU/RAM samples missing; report is incomplete")

    # Index once: rescanning every request for each second becomes quadratic
    # on long full/soak runs and used to exceed the analysis timeout.
    latency_windows, failure_windows = {}, {}
    for metric, destination in (('http_req_duration', latency_windows), ('http_req_failed', failure_windows)):
        for row in raw.get(metric, []):
            bucket = math.floor((row[0] - origin) / WINDOW_SEC)
            destination.setdefault(bucket, []).append(row)
    windows = []
    p95_by_time = dict(p95)
    for ts, rps_value in rps_pts:
        bucket = round((ts - origin) / WINDOW_SEC) - 1
        selected = latency_windows.get(bucket, [])
        failed = failure_windows.get(bucket, [])
        bad_count = sum(v for _, v, _ in failed)
        windows.append({'end_sec': ts - origin, 'rps': rps_value, 'requests': len(selected),
                        'unexpected_failures': bad_count, 'error_rate': bad_count / len(selected) if selected else None,
                        'p95_ms': p95_by_time.get(ts),
                        'operations': operation_report({'http_req_duration': selected, 'http_req_failed': failed})})
    write_json(out / 'windows.json', windows)

    resources = resource_report(cpu, ram, disk_read, disk_write)
    write_json(out / "resources.json", resources)
    write_json(out / "throughput.json", {
        "rps": summarize(rps_pts),
        "sessions_per_sec": summarize(one(sessions)),
        "stream_bytes_per_sec": summarize(one(stream)),
        "gpu": None,
    })

    vlines = []
    phase_bands = [{**bound, "start": bound["start"] - origin, "end": bound["end"] - origin} for bound in phase_bounds(summary, origin)] if origin else []

    knee = None
    if summary.get("scenario") == "ramp" and origin:
        knee = stage_report(summary, origin, p95, err_pts, rps_pts, raw)
        write_json(out / "knee.json", knee)
        if knee.get("stable_vus") is not None:
            for bound in phase_bounds(summary, origin):
                marker = bound.get("rps", bound["vus"])
                if marker == knee["stable_vus"]:
                    unit = "RPS" if bound.get("rps") is not None else "VU"
                    vlines.append((bound["end"] - origin, f"Конец последней пройденной ступени: {marker:g} {unit}"))
                    break
    recovery = None
    if summary.get("scenario") == "stress" and origin:
        recovery = recovery_report(summary, origin, p95, err_pts, raw)
        write_json(out / "recovery.json", recovery)
        if recovery.get("return_unix"):
            open_rate = any(phase.get("rps") is not None for phase in summary.get("phases") or [])
            vlines.append((recovery["return_unix"] - origin, "Перегрузка снята: возврат к базовому RPS" if open_rate else "Перегрузка снята: возврат к базовому числу VU"))
        if recovery.get("recovered"):
            vlines.append((recovery["return_unix"] - origin + recovery["recovery_sec"], "Первое здоровое 1-секундное окно (recovered); не гарантия устойчивости"))

    phases = stage_report(summary, origin, p95, err_pts, rps_pts, raw)['stages'] if summary.get('phases') else []
    write_json(out / 'phases.json', phases)
    write_readable_report(out, measured, knee, recovery, phases)

    try:
        import matplotlib
        matplotlib.use("Agg")
    except ImportError:
        print("matplotlib is not installed, JSON is written and PNG files were skipped", file=sys.stderr)
        return

    heading = title_of(summary, "")
    latency_points = {.5:p50, .9:p90, .99:p99}
    draw(plots / "latency_over_time.png", heading + "latency p50/p90/p99",
         [(f'p{int(p*100)}', rel(latency_points[p], origin)) for p in PLOT_PERCENTILES],
         "Задержка HTTP, мс", vlines, phase_bands)
    api_requests = [(ts, 1, tags) for ts, _, tags in latencies if tags.get('step') not in ('upload', 'stream-body')]
    api_rps = [(ts, value) for ts, value in raw_windows(api_requests, origin, end, include_empty=True)
               if ts - WINDOW_SEC >= origin + float(summary.get('warmup_sec', 0))]
    save_series(out, 'api_rps', [{'labels': {}, 'points': api_rps}])
    throughput_lines = [
        ('Все HTTP: API + S3', rel(rps_pts, origin)),
        ('Только API', rel(api_rps, origin)),
    ]
    offered = offered_rate_points(summary, origin)
    if offered:
        throughput_lines.append(('Цель', rel(offered, origin)))
    draw(plots / 'rps_over_time.png', heading + 'HTTP throughput', throughput_lines, 'Завершённых запросов в секунду', vlines, phase_bands)
    draw(plots / "cpu_over_time.png", heading + "CPU", [(resource_name(item), rel(item["points"], origin)) for item in cpu], "CPU, логические ядра", vlines, phase_bands)
    gib = 1024 ** 3
    ram_gib = [(resource_name(item), [(ts, value / gib) for ts, value in rel(item["points"], origin)]) for item in ram]
    draw(plots / "ram_over_time.png", heading + "RAM", ram_gib, "RAM, ГиБ", vlines, phase_bands)

    duration = (summary.get("http_req_duration") or {})
    bars = [(f'p{int(p*100)}', duration.get(f'p({int(p*100)})')) for p in PERCENTILES]
    import matplotlib.pyplot as plt
    fig, ax = plt.subplots(figsize=(8, 4))
    ax.set_title(heading + "percentiles")
    names = [name for name, _ in bars]
    values = [value or 0 for _, value in bars]
    ax.bar(names, values)
    ax.set_ylabel("ms")
    fig.tight_layout()
    fig.savefig(plots / "latency_percentiles.png")
    plt.close(fig)

    # Every measured request contributes once, regardless of its operation type.
    ordered = sorted(value for _, value, _ in latencies)
    fig, ax = plt.subplots(figsize=(10, 4.5))
    ranks = [100 * (i + 1) / len(ordered) for i in range(len(ordered))]
    ax.plot(ranks, ordered, label=f'{len(ordered)} requests')
    for p in PERCENTILES:
        value = measured[f'p{int(p*100)}_ms']
        ax.scatter([p*100], [value], s=20)
        ax.annotate(f'p{int(p*100)}: {value:.2f} ms', (p*100, value), xytext=(-8, 10), textcoords='offset points', ha='right', fontsize=8)
    ax.set_xlabel('Доля измеренных запросов (%)')
    ax.set_ylabel('Задержка HTTP (мс)')
    ax.set_title(heading + f'Распределение задержек запросов\nИзмерено {len(ordered):,} запросов')
    ax.grid(alpha=.2)
    ax.secondary_xaxis('top', functions=(lambda x: x * len(ordered) / 100, lambda x: x * 100 / len(ordered))).set_xlabel('Число запросов в распределении')
    fig.tight_layout()
    fig.savefig(plots / 'latency_distribution.png')
    plt.close(fig)

    edges = [5, 10, 20, 50, 100, 200, 400, 800, 1500, 3000, 5000, float('inf')]
    labels = ['≤5', '5–10', '10–20', '20–50', '50–100', '100–200',
              '200–400', '400–800', '800–1500', '1500–3000', '3000–5000', '>5000']
    hist_counts = [0] * len(edges)
    for _, value, _ in latencies:
        index = next(i for i, edge in enumerate(edges) if value <= edge)
        hist_counts[index] += 1
    fig, ax = plt.subplots(figsize=(8, 4))
    ax.set_title(heading + "latency histogram")
    if hist_counts:
        ax.bar(labels, hist_counts)
        ax.set_xlabel('Задержка HTTP, диапазон (мс)')
        ax.set_ylabel('Количество запросов')
        ax.tick_params(axis='x', labelrotation=35)
    else:
        ax.text(0.5, 0.5, "no samples", ha="center", va="center", transform=ax.transAxes)
    fig.tight_layout()
    fig.savefig(plots / "latency_histogram.png")
    plt.close(fig)

    iostat_path = out / "iostat.log" if (out / "iostat.log").exists() else out.parent / "iostat.log"
    iostat_text = iostat_path.read_text(errors="replace") if iostat_path.exists() else ""
    uptime_path = out / "uptime.log" if (out / "uptime.log").exists() else out.parent / "uptime.log"
    uptime_text = uptime_path.read_text(errors="replace") if uptime_path.exists() else ""
    io_samples = parse_iostat(iostat_text)
    write_json(out / "series" / "host.json", {"iostat": io_samples, "load": parse_uptime(uptime_text)})
    print(f"wrote {plots}")


def compare(dirs, dest):
    dest = Path(dest)
    dest.mkdir(parents=True, exist_ok=True)
    rows = []
    for directory in dirs:
        out = Path(directory)
        run = load_json(out / 'run.json') or {}
        if not run.get('success'):
            raise ValueError(f'cannot compare unsuccessful/incomplete run: {out}')
        summary = load_json(out / 'summary.json') or {}
        http = load_json(out / 'http.json') or {}
        rows.append({'directory': str(out), 'fault_rate': summary.get('fault_rate'),
                     'p95_ms': http.get('p95_ms'), 'error_rate': http.get('error_rate'),
                     'rps': (load_json(out / 'throughput.json') or {}).get('rps'),
                     'resources': load_json(out / 'resources.json')})
    write_json(dest / 'compare.json', rows)
    for name in ('soak_fault_compare.png', 'soak_ram_end.png'):
        (dest / name).unlink(missing_ok=True)
    print(f'wrote {dest / "compare.json"}')


def main():
    if len(sys.argv) < 3:
        print("usage: plot.py run DIR | plot.py compare DEST DIR [DIR...]", file=sys.stderr)
        return 2
    if sys.argv[1] == "run":
        run_dir(sys.argv[2])
        return 0
    if sys.argv[1] == "compare":
        compare(sys.argv[3:], sys.argv[2])
        return 0
    print("unknown command", file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())
