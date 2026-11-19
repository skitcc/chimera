#!/usr/bin/env python3
"""Build one offline HTML report for every available scenario of a perf series."""
import base64
import html
import json
import math
from pathlib import Path
import sys


def read(path, default=None):
    try:
        return json.loads(Path(path).read_text())
    except (OSError, ValueError):
        return default


def esc(value):
    return html.escape(str(value))


def number(value, digits=2):
    return '—' if value is None else f'{value:,.{digits}f}'


def table(headers, rows):
    return '<div class="scroll"><table><thead><tr>' + ''.join(f'<th>{esc(x)}</th>' for x in headers) + \
        '</tr></thead><tbody>' + ''.join('<tr>' + ''.join(f'<td>{esc(x)}</td>' for x in row) + '</tr>'
                                       for row in rows) + '</tbody></table></div>'


def cpu_ids(value):
    result = set()
    for part in (value or '').split(','):
        if not part:
            continue
        bounds = list(map(int, part.split('-')))
        result.update(range(bounds[0], bounds[-1] + 1))
    return result


def cpu_description(value, hardware):
    if not value:
        return 'Нет привязки, заданной runner; действуют настройки Docker/хоста'
    ids = cpu_ids(value)
    cores = {(x['socket'], x['core']) for x in hardware.get('cpus', []) if x['cpu'] in ids}
    physical = str(len(cores)) if cores else 'нет снимка топологии'
    return f'CPU {value}: {len(ids)} логических; физических ядер: {physical}'


def svg(points, title, unit, origin, phases=()):
    points = sorted((float(t) - origin, float(v)) for t, v in points if math.isfinite(float(v)))
    if not points:
        return '<p class="warning">Нет сохранённых измерений: ' + esc(title) + '</p>'
    width, height, left, right, top, bottom = 1160, 260, 80, 25, 28, 45
    xmin, xmax = min(0, points[0][0]), max(points[-1][0], 1)
    ymax = max(max(v for _, v in points) * 1.1, .01)
    def x(t): return left + (t - xmin) / (xmax - xmin) * (width - left - right)
    def y(v): return height - bottom - v / ymax * (height - top - bottom)
    body = []
    for phase in phases:
        start, end = phase['start'], phase['end']
        body.append(f'<rect x="{x(start):.2f}" y="{top}" width="{max(0,x(end)-x(start)):.2f}" '
                    f'height="{height-top-bottom}" fill="#eef3f8" stroke="white"/>')
    for i in range(5):
        value = ymax * i / 4
        body.append(f'<line x1="{left}" x2="{width-right}" y1="{y(value):.1f}" y2="{y(value):.1f}" stroke="#dfe6ed"/>'
                    f'<text x="{left-8}" y="{y(value)+4:.1f}" text-anchor="end">{value:.3g}</text>')
        stamp = xmin + (xmax - xmin) * i / 4
        body.append(f'<text x="{x(stamp):.1f}" y="{height-bottom+20}" text-anchor="middle">{stamp:.0f}</text>')
    # Preserve extrema when reducing long series for a self-contained HTML.
    if len(points) > 1800:
        reduced = []
        chunk = math.ceil(len(points) / 900)
        for i in range(0, len(points), chunk):
            group = points[i:i+chunk]
            reduced.extend(sorted({min(group, key=lambda p:p[1]), max(group, key=lambda p:p[1])}))
        points = reduced
    path = ' '.join(f'{x(t):.2f},{y(v):.2f}' for t,v in points)
    return (f'<figure><figcaption>{esc(title)} · {esc(unit)}</figcaption>'
            f'<svg role="img" aria-label="{esc(title)}" viewBox="0 0 {width} {height}">'
            '<style>text{font:12px system-ui;fill:#4b5b6b}</style>' + ''.join(body) +
            f'<polyline points="{path}" fill="none" stroke="#2273b9" stroke-width="1.6"/>'
            f'<text x="{width/2}" y="{height-5}" text-anchor="middle">Время от начала сценария, с</text>'
            '</svg></figure>')


def label(item):
    tags = item.get('labels', {})
    return '/'.join(str(tags.get(k, '')) for k in ('namespace','pod','container') if tags.get(k)) or item.get('label','?')


def restarts(before, after):
    old = {(x['uid'], x['container']): x for x in before}
    rows = []
    for item in after:
        previous = old.get((item['uid'], item['container']))
        delta = item['restart_count'] - previous['restart_count'] if previous else None
        termination = item.get('last_termination') or {}
        if delta or termination or not item.get('ready', False) or previous is None:
            rows.append([item['namespace']+'/'+item['pod']+'/'+item['container'],
                         'новый pod / нет исходного снимка' if delta is None else str(delta),
                         item['restart_count'], 'Да' if item.get('ready') else 'Нет',
                         termination.get('reason', '—'), termination.get('exitCode', '—'),
                         termination.get('finishedAt', '—')])
    return rows


def resources(out, origin, phases):
    detailed_cpu = read(out/'series/cpu_detailed.json', [])
    detailed_ram = read(out/'series/ram_detailed.json', [])
    legacy = not detailed_cpu and not detailed_ram
    cpu = detailed_cpu or read(out/'series/cpu.json', [])
    ram = detailed_ram or read(out/'series/ram.json', [])
    entries = {}
    for metric, series in [('cpu',cpu), ('ram',ram)]:
        for item in series:
            entries.setdefault(label(item), {})[metric] = item.get('points', [])
    docker = read(out/'docker-resources.json', {})
    if not docker.get('samples'):
        run = read(out/'run.json', {})
        all_samples = read(out.parent/'docker-resources.json', {}).get('samples', [])
        docker = {'samples':[x for x in all_samples
                             if run.get('start_unix', math.inf) <= x['time'] <= run.get('end_unix', -math.inf)]}
    for sample in docker.get('samples', []):
        entry = entries.setdefault('Docker/'+sample['container'], {})
        entry.setdefault('cpu', []).append([sample['time'], sample['cpu_cores']])
        entry.setdefault('ram', []).append([sample['time'], sample['memory_bytes']])
    body = '<h3>CPU и память отдельных контейнеров</h3>'
    if legacy:
        body += '<p class="warning">Старый прогон: доступны только агрегаты по имени контейнера. '
        body += 'Отдельные PostgreSQL-реплики и namespaces восстановить нельзя. Ряды старых экземпляров после рестартов могли суммироваться.</p>'
    if not docker.get('samples'):
        body += '<p class="warning">В этом прогоне не сохранены Docker CPU/RAM k6, тестового Prometheus и общего Minikube.</p>'
    if docker.get('errors'):
        body += f'<p class="warning">Ошибок сбора Docker stats: {len(docker["errors"])}. '
        body += 'Графики содержат только полученные точки; пропуски не считаются нулевой нагрузкой.</p>'
    body += '<p>CPU — эквивалент занятых логических CPU: 1,0 соответствует одному CPU, занятому всё время. '
    body += 'SMT-потоки делят ресурсы физических ядер. cAdvisor CPU сглажен за 30 с, память — working set. '
    body += 'Docker CPU/RAM измеряются docker stats; частота включает время работы Docker CLI. '
    body += 'Minikube total включает Kubernetes-контейнеры: складывать его с ними нельзя. '
    body += 'Это учёт контейнеров, без профилирования отдельных PID и процессов ядра хоста.</p>'
    rows = []
    panels = []
    budgets = {'/'.join((x['namespace'],x['pod'],x['container'])):x.get('resources', {})
               for x in read(out/'pods-before.json', [])}
    for name, metrics in sorted(entries.items()):
        def stats(key, divisor=1):
            values = [v/divisor for _,v in metrics.get(key, [])]
            return [number(sum(values)/len(values)), number(max(values))] if values else ['—','—']
        budget = budgets.get(name, {})
        memory_budget = budget.get('requests',{}).get('memory','—') + ' / ' + budget.get('limits',{}).get('memory','—')
        rows.append([name, *stats('cpu'), *stats('ram',1024**3), memory_budget])
        graphs = svg(metrics.get('cpu',[]), name+' — CPU', 'логические CPU', origin, phases)
        graphs += svg([(t,v/1024**3) for t,v in metrics.get('ram',[])], name+' — RAM', 'ГиБ', origin, phases)
        panels.append(f'<details><summary>{esc(name)} — CPU и RAM</summary>{graphs}</details>')
    body += table(['Контейнер','CPU среднее','CPU максимум','RAM средняя, ГиБ','RAM максимум, ГиБ',
                   'RAM request / limit'], rows)
    return body + ''.join(panels)


def generate(series):
    series = Path(series).resolve()
    series.mkdir(parents=True, exist_ok=True)
    environment = read(series/'environment.json', {})
    hardware = read(series/'hardware.json', {})
    isolation = environment.get('cpu_isolation', {})
    body = [f'<h1>Нагрузочное тестирование · {esc(series.name)}</h1>',
            '<p>Все доступные сценарии одной серии. Графики встроены в HTML; для просмотра не нужны Prometheus или интернет.</p>']
    if (series/'error.txt').exists():
        body.append('<p class="warning">Runner завершился с ошибкой: '+esc((series/'error.txt').read_text())+'</p>')
    body.append('<h2>Ресурсы стенда</h2>')
    body.append(f'<p>Хост: {esc(hardware.get("logical_cpus","нет снимка"))} логических CPU; '
                f'{esc(hardware.get("physical_cores","нет снимка"))} физических ядер. '
                f'Изоляция runner: {"включена" if isolation.get("enabled") else "выключена / нет данных"}.</p>')
    if hardware.get('captured_after_run'):
        body.append('<p class="warning">Для старой серии топология CPU снята при повторном построении отчёта, '
                    'а не в момент нагрузки. Привязка CPU взята из исторического environment.json.</p>')
    body.append(table(['Назначение','Привязка'], [
        ['Весь Minikube',cpu_description(isolation.get('app_cpus') or environment.get('minikube_cpuset'),hardware)],
        ['k6',cpu_description(isolation.get('load_cpus'),hardware)],
        ['Наблюдение',cpu_description(isolation.get('observe_cpus'),hardware)]]))
    body.append('<p>CPU-привязка общая для всего Minikube. Физические ядра указаны по снимку lscpu, '
                'а не получены делением числа CPU на два. Привязка не исключает конкуренцию с другими процессами хоста.</p>')
    analysis = read(series/'analysis.json', {})
    if analysis.get('paragraphs'):
        body.append('<h2>'+esc(analysis.get('title','Разбор результатов'))+'</h2>')
        body.extend('<p>'+esc(paragraph)+'</p>' for paragraph in analysis['paragraphs'])
    candidates = sorted((p for p in series.iterdir() if p.is_dir() and
                         any((p/n).exists() for n in ('run.json','summary.json','report.md','k6.log'))),
                        key=lambda p: (read(p/'run.json',{}).get('start_unix',0),p.name))
    body.append('<nav>'+''.join(f'<a href="#scenario-{i}">{esc(p.name)}</a> ' for i,p in enumerate(candidates))+'</nav>')
    for index,out in enumerate(candidates):
        run, summary = read(out/'run.json',{}), read(out/'summary.json',{})
        measured = read(out/'http.json',{})
        body.append(f'<section id="scenario-{index}"><h2>{esc(out.name)}</h2>')
        body.append(f'<p>k6 exit: {esc(run.get("k6_exit","нет данных"))}; '
                    f'анализ exit: {esc(run.get("analysis_exit","нет данных"))}; '
                    f'сценарий: {esc(summary.get("scenario",run.get("scenario","?")))}; '
                    f'ошибочные действия: {esc(run.get("fault_rate",summary.get("fault_rate",0)))}.</p>')
        body.append('<p>Успешный выход k6 означает завершение скрипта; прохождение требований оценивается по p95, ошибкам и ступеням.</p>')
        if measured:
            body.append(table(['Запросы','Ошибки','p50, мс','p75, мс','p90, мс','p95, мс','p99, мс'], [[
                measured.get('requests','—'),number(measured.get('error_rate',0)*100)+'%',
                *[number(measured.get(f'p{n}_ms')) for n in (50,75,90,95,99)]]]))
        else:
            body.append('<p class="warning">HTTP-анализ ещё не завершён или исходные данные отсутствуют. Показаны доступные артефакты.</p>')
        phases = read(out/'phases.json',[]) or (read(out/'knee.json',{}) or {}).get('stages',[])
        if phases:
            body.append(table(['VU','Все HTTP RPS','API RPS','p95, мс','Ошибки','Причины нарушения'],[
                [p.get('vus'),number(p.get('rps')),number(p.get('api_rps')),number(p.get('p95_ms')),
                 number(p.get('error_rate')*100 if p.get('error_rate') is not None else None)+'%',
                 ', '.join(p.get('failure_reasons',[])) or '—'] for p in phases]))
        recovery = read(out/'recovery.json')
        if recovery:
            body.append('<h3>Восстановление после stress</h3>'+table(
                ['Восстановление отмечено','Время, с','Окно, с','Повторное ухудшение'],[[
                    recovery.get('recovered'),recovery.get('recovery_sec'),recovery.get('window_sec'),
                    len(recovery.get('relapse_windows',[]))]]))
        body.append('<h3>Задержки и пропускная способность</h3>')
        for filename in ('latency_over_time.png','latency_distribution.png','latency_percentiles.png',
                         'latency_histogram.png','rps_over_time.png','cpu_over_time.png','ram_over_time.png'):
            path = out/'plots'/filename
            if path.exists():
                data = base64.b64encode(path.read_bytes()).decode()
                body.append(f'<figure><figcaption>{esc(filename)}</figcaption><img alt="{esc(filename)}" '
                            f'src="data:image/png;base64,{data}"></figure>')
        origin = run.get('start_unix',0)
        # Align resource plots to the actual k6 scenario origin when available.
        import re
        log = out/'k6.log'
        if log.exists():
            marks = re.findall(r'SCENARIO_START (\d+)',log.read_text(errors='replace'))
            if marks:
                origin = min(map(int,marks))/1000
        bands, cursor = [],0
        for phase in summary.get('phases',[]):
            bands.append({'start':cursor,'end':cursor+phase['sec']})
            cursor += phase['sec']
        body.append(resources(out, origin, bands))
        before, after = read(out/'pods-before.json',[]), read(out/'pods-after.json',[])
        body.append('<h3>Рестарты и завершения контейнеров</h3>')
        if before and after:
            changes = restarts(before,after)
            body.append(table(['Контейнер','Новых рестартов','Всего','Ready','Последняя причина',
                               'Exit','Время последнего завершения UTC'],changes) if changes else
                        '<p>Новых рестартов и неготовых контейнеров в конечном снимке не обнаружено.</p>')
            body.append('<p>Последняя причина может относиться к историческому рестарту: сравнивайте время и дельту. '
                        'Краткие сбои Ready между снимками и уже исчезнувшие pod могут не попасть в таблицу.</p>')
        else:
            body.append('<p class="warning">Нет пары снимков до/после: изменения restartCount восстановить нельзя.</p>')
        steps = measured.get('steps',{})
        if steps:
            body.append('<h3>Операции</h3>'+table(['Операция','Запросы','Отказы','p95, мс','p99, мс'],[
                [name,item.get('requests'),item.get('unexpected_failures'),number(item.get('p95_ms')),number(item.get('p99_ms'))]
                for name,item in sorted(steps.items())]))
        for name in ('telemetry-error.txt','analysis-error.txt'):
            if (out/name).exists():
                body.append('<p class="warning">'+esc((out/name).read_text())+'</p>')
        body.append('</section>')
    if not candidates:
        body.append('<p class="warning">Нагрузочные сценарии ещё не запускались: доступны только данные подготовки или ошибка preflight.</p>')
    investigation = read(series/'rustfs-oom-investigation.json')
    if investigation:
        body.append('<h2>Сохранённое расследование RustFS</h2><pre>'+esc(json.dumps(investigation,ensure_ascii=False,indent=2))+'</pre>')
    body.append('<p>Причина прошлых 409 в author: playlist-add вызывался для pending-трека после failed upload. '
                'Сценарий исправлен: новый трек используется только после публикации. Исторические измерения не изменены. '
                'Увеличение лимита RustFS до 2 ГиБ уменьшает риск прежнего OOM, но не гарантирует отсутствие утечки памяти, '
                'нового OOM, сбоя диска, процесса или probe. Устойчивость требует наблюдения при следующих пользовательских запусках.</p>')
    css = '''body{font:16px/1.5 system-ui;color:#1b2b3c;background:#f4f7fa;max-width:1200px;margin:auto;padding:24px}
    h1,h2,h3{line-height:1.25}h2{margin-top:2em;border-bottom:1px solid #cbd7e2;padding-bottom:8px}
    table{border-collapse:collapse;width:100%;background:white;margin:14px 0}td,th{border:1px solid #d6e0e8;padding:8px;text-align:left}th{background:#eaf0f6}
    .scroll{overflow-x:auto}.warning{background:#fff0d6;padding:14px;border-left:4px solid #c78100}
    figure{display:block;width:100%;box-sizing:border-box;margin:20px 0;background:white;border:1px solid #d6e0e8;padding:10px}
    img,svg{display:block;width:100%;height:auto}figcaption{font-weight:600;margin-bottom:8px;overflow-wrap:anywhere}
    details{background:white;border:1px solid #d6e0e8;margin:12px 0;padding:10px}summary{cursor:pointer;font-weight:600;overflow-wrap:anywhere}
    nav a{display:inline-block;margin:6px;padding:6px 10px;background:#e4edf7}pre{white-space:pre-wrap;overflow-wrap:anywhere}
    @media(max-width:600px){body{padding:10px}}'''
    document = '<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">' \
        '<title>Нагрузочная серия '+esc(series.name)+'</title><style>'+css+'</style></head><body>'+''.join(body)+'</body></html>'
    target = series/'report.html'
    temporary = target.with_suffix('.html.tmp')
    temporary.write_text(document)
    temporary.replace(target)
    return target


if __name__ == '__main__':
    print(generate(sys.argv[1]))
