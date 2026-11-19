"""Bounded adaptive search for a sustained HTTP rate under explicit SLOs."""
import gzip
import json
import math
import os
from pathlib import Path
import threading
import time
import urllib.request
import urllib.error


def write(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def cpu_count(cpus):
    if not cpus:
        return len(os.sched_getaffinity(0))
    values = set()
    for part in cpus.split(','):
        bounds = [int(x) for x in part.split('-')]
        values.update(range(bounds[0], bounds[-1] + 1))
    return len(values)


class GeneratorMonitor:
    def __init__(self, name, command):
        self.name, self.command = name, command
        self.stop = threading.Event()
        self.samples = []
        self.thread = threading.Thread(target=self.sample, daemon=True)

    def sample(self):
        while not self.stop.is_set():
            try:
                body = json.loads(self.command(['docker', 'stats', '--no-stream', '--format', '{{json .}}', self.name], timeout=4))
                self.samples.append({'time': time.time(), 'cpu_cores': float(body['CPUPerc'].rstrip('%')) / 100,
                                     'memory': body['MemUsage']})
            except Exception:
                pass  # Container is not created yet, or has just finished.
            self.stop.wait(3)

    def start(self):
        self.thread.start()

    def finish(self):
        self.stop.set()
        self.thread.join(timeout=5)
        return self.samples


def classify(http, summary, completion_rps, generator, available_cpus, latency_slo=500):
    target = summary['target_rps']
    dropped = (summary.get('dropped_iterations') or {}).get('count', 0)
    started = (summary.get('started_requests') or {}).get('count', 0)
    delivered = started / summary['measurement_sec']
    peak = max((x['cpu_cores'] for x in generator), default=None)
    reasons = []
    if http['p95_ms'] > latency_slo:
        reasons.append('p95_above_slo')
    if http['error_rate'] >= .01:
        reasons.append('unexpected_errors>=1%')
    if delivered < target * .98:
        reasons.append('offered_rate_not_reached')
    if completion_rps < target * .98:
        reasons.append('completed_rate_below_98%')
    if dropped:
        reasons.append('dropped_iterations')
    generator_limited = peak is None or peak >= available_cpus * .9
    inconclusive = generator_limited or (bool(dropped) and http['p95_ms'] <= latency_slo and http['error_rate'] < .01)
    if generator_limited:
        reasons.append('generator_cpu_limit_or_missing_telemetry')
    return {'target_rps': target, 'started_rps': delivered, 'completed_rps': completion_rps,
            'p95_ms': http['p95_ms'], 'error_rate': http['error_rate'], 'dropped_iterations': dropped,
            'generator_peak_cores': peak, 'generator_available_cpus': available_cpus,
            'passed': not reasons, 'inconclusive': inconclusive, 'reasons': reasons}


class Search:
    def __init__(self, runner):
        self.r = runner
        self.minimum = int(os.getenv('CAP_START_RPS', '50'))
        self.ceiling = int(os.getenv('CAP_MAX_RPS', '1600'))
        self.resolution = int(os.getenv('CAP_RESOLUTION_RPS', '25'))
        self.seconds = int(os.getenv('CAP_STEP_SEC', '60'))
        self.confirm_seconds = int(os.getenv('CAP_CONFIRM_SEC', '180'))
        self.warmup = int(os.getenv('CAP_WARMUP_SEC', '10'))
        self.max_vus = int(os.getenv('CAP_MAX_VUS', '600'))
        self.max_trials = int(os.getenv('CAP_MAX_TRIALS', '12'))
        self.slo = float(os.getenv('LATENCY_SLO_MS', '500'))
        self.workload = os.getenv('CAP_WORKLOAD', 'mixed')
        if not (1 <= self.minimum <= self.ceiling <= 20000 and 1 <= self.resolution and
                20 <= self.seconds <= 600 and self.seconds <= self.confirm_seconds <= 900 and
                0 <= self.warmup <= 120 and 10 <= self.max_vus <= 2000 and 1 <= self.max_trials <= 20):
            raise ValueError('invalid capacity bounds/durations/VUs; see docs/perf.md')
        if self.workload not in ('mixed', 'catalog'):
            raise ValueError('CAP_WORKLOAD=mixed|catalog')
        self.trials = []
        self.dataset = None
        self.inconclusive_rate = None

    def prepare(self):
        self.dataset = json.loads((self.r.out / 'dataset.json').read_text())
        self.dataset['accounts'] = []
        for role in ('listeners', 'subscribers', 'authors'):
            for user in self.dataset[role]:
                self.dataset['accounts'].append({**user, 'token': self.r.login(user)})
        self.dataset['track_bytes'] = self.r.track_bytes

    def refresh_links(self):
        urls = []
        if self.workload == 'mixed':
            opener = urllib.request.build_opener(NoRedirect)
            for track in self.dataset['tracks'][:10]:
                try:
                    opener.open(self.r.base + '/api/v1/tracks/' + track + '/stream', timeout=5)
                except urllib.error.HTTPError as response:
                    if response.code != 302 or not response.headers.get('Location'):
                        raise RuntimeError('cannot prepare a fresh signed stream URL')
                    urls.append(response.headers['Location'])
        self.dataset['stream_urls'] = urls
        path = self.r.out / 'capacity-dataset.json'
        write(path, self.dataset)
        os.chmod(path, 0o600)
        os.chown(path, self.r.uid, self.r.gid)

    def recover(self):
        deadline = time.monotonic() + 60
        started = time.monotonic()
        streak, samples = 0, []
        author = self.dataset['authors'][0]
        while time.monotonic() < deadline:
            begin = time.monotonic()
            healthy, error, timings = True, None, {}
            operations = [
                ('catalog', lambda: self.r.http('GET', '/tracks?limit=1', timeout=3)),
                ('identity', lambda: self.r.http('GET', '/users?limit=1', timeout=3)),
                ('social', lambda: self.r.http('GET', '/playlists?owner_id=' + author['id'], timeout=3)),
                ('login', lambda: self.r.login(author, timeout=3)),
            ]
            try:
                for name, operation in operations:
                    request_start = time.monotonic()
                    operation()
                    timings[name] = (time.monotonic() - request_start) * 1000
            except Exception as exc:
                healthy, error = False, type(exc).__name__
            elapsed = (time.monotonic() - begin) * 1000
            healthy = healthy and len(timings) == 4 and all(value <= self.slo for value in timings.values())
            samples.append({'after_sec': time.monotonic() - started, 'healthy': healthy,
                            'round_ms': elapsed, 'request_ms': timings, 'error': error})
            streak = streak + 1 if healthy else 0
            if streak >= 3:
                return {'healthy': True, 'seconds': time.monotonic() - started, 'rounds': samples}
            time.sleep(2)
        return {'healthy': False, 'seconds': time.monotonic() - started, 'rounds': samples}

    def trial(self, rate, confirm=False):
        self.refresh_links()
        duration = self.confirm_seconds if confirm else self.seconds
        pre_vus = min(self.max_vus, max(20, math.ceil(rate * self.slo / 1000 * 1.5)))
        overrides = {'CAP_TARGET_RPS': str(rate), 'CAP_STEP_SEC': str(duration), 'CAP_WARMUP_SEC': str(self.warmup),
                     'CAP_PRE_VUS': str(pre_vus), 'CAP_MAX_VUS': str(self.max_vus), 'CAP_WORKLOAD': self.workload,
                     'DATASET_FILE': '/data/dataset.json', 'WARMUP_SEC': str(self.warmup)}
        label = f'capacity-{len(self.trials)+1:02d}-{rate}rps' + ('-confirm' if confirm else '')
        out = self.r.measure('capacity', 0, rate, overrides=overrides, label=label,
                             script='capacity.js', dataset='capacity-dataset.json', monitor_generator=True)
        http = json.loads((out / 'http.json').read_text())
        summary = json.loads((out / 'summary.json').read_text())
        origin = None
        import re
        match = re.search(r'SCENARIO_START (\d+)', (out / 'k6.log').read_text())
        if match:
            origin = int(match[1]) / 1000
        if origin is None:
            raise RuntimeError('capacity scenario origin is missing')
        completed = 0
        with gzip.open(out / 'samples.json.gz', 'rt') as source:
            import datetime
            for line in source:
                row = json.loads(line)
                if row.get('type') != 'Point' or row.get('metric') != 'http_req_duration':
                    continue
                d = row['data']
                ts = datetime.datetime.fromisoformat(d['time'].replace('Z', '+00:00')).timestamp()
                if d.get('tags', {}).get('phase') == 'measure' and origin + self.warmup <= ts < origin + self.warmup + duration:
                    completed += 1
        generator = [x for x in json.loads((out / 'generator.json').read_text()) if origin + self.warmup <= x['time'] <= origin + self.warmup + duration]
        result = classify(http, summary, completed / duration, generator, cpu_count(self.r.load_cpus), self.slo)
        result.update(directory=str(out), confirmation=confirm, measurement_sec=duration, operations=http['steps'])
        print(f"capacity {rate}: {'PASS' if result['passed'] else 'INCONCLUSIVE' if result['inconclusive'] else 'FAIL'}; completed={result['completed_rps']:.1f} RPS p95={result['p95_ms']:.1f}ms errors={result['error_rate']:.2%} dropped={result['dropped_iterations']}", flush=True)
        self.trials.append(result)
        write(out / 'capacity-trial.json', result)
        self.save()
        recovery = self.recover()
        result['recovery_probe'] = recovery
        write(out / 'capacity-trial.json', result)
        self.save()
        if not recovery['healthy']:
            raise RuntimeError('application did not pass recovery probes in 60s; search stopped')
        return result

    def save(self, confirmed=None, upper=None, reason=None):
        body = {'workload': self.workload, 'latency_slo_ms': self.slo, 'error_slo': .01,
                'resolution_rps': self.resolution, 'confirmed_rps': confirmed,
                'first_failing_rps': upper, 'inconclusive_rate': self.inconclusive_rate, 'reason': reason, 'trials': self.trials,
                'method': 'one HTTP per constant-arrival-rate iteration; 98% delivery/completion; no dropped iterations; generator CPU <90%'}
        write(self.r.out / 'capacity.json', body)
        lines = ['# Поиск пропускной способности', '', f'Нагрузка: {self.workload}; p95 ≤{self.slo:g} мс, неожиданные ошибки <1%.', '',
                 '| Цель RPS | Начато/с | Завершено/с | p95, мс | Ошибки | Пропущено | Статус |', '| --- | --- | --- | --- | --- | --- | --- |']
        for trial in self.trials:
            state = 'PASS' if trial['passed'] else 'INCONCLUSIVE' if trial['inconclusive'] else 'FAIL'
            lines.append(f"| {trial['target_rps']} | {trial['started_rps']:.1f} | {trial['completed_rps']:.1f} | {trial['p95_ms']:.1f} | {trial['error_rate']:.2%} | {trial['dropped_iterations']} | {state}{' (confirmation)' if trial['confirmation'] else ''} |")
        lines += ['', f'Подтверждённая нагрузка: {confirmed if confirmed is not None else "пока не подтверждена"} HTTP RPS.',
                  f'Верхняя неуспешная проверенная точка: {upper if upper is not None else "не найдена"}.', reason or '', '',
                  'Это результат конкретного состава запросов, данных и ресурсов; не универсальное число пользователей и не доказательство многочасовой устойчивости. Детали операций и CPU/RAM — во вложенных отчётах.']
        (self.r.out / 'capacity-report.md').write_text('\n'.join(lines) + '\n')

    def run(self):
        self.prepare()
        low, high = 0, None
        rate = self.minimum
        while len(self.trials) < self.max_trials:
            result = self.trial(rate)
            if result['inconclusive']:
                self.inconclusive_rate = rate
                break
            if result['passed']:
                low = rate
                if rate >= self.ceiling:
                    break
                rate = min(self.ceiling, rate * 2)
            else:
                high = rate
                break
        while self.inconclusive_rate is None and high is not None and (high - low > self.resolution or low == 0) and len(self.trials) < self.max_trials:
            rate = max(1, (low + high) // 2)
            result = self.trial(rate)
            if result['inconclusive']:
                self.inconclusive_rate = rate
                break
            if result['passed']:
                low = rate
            else:
                high = rate
        candidates = sorted({t['target_rps'] for t in self.trials if t['passed']}, reverse=True)
        for candidate in candidates[:2]:
            result = self.trial(candidate, confirm=True)
            if result['passed']:
                self.save(confirmed=candidate, upper=high,
                          reason='Generator/VU limit: only the confirmed lower bound is valid.' if self.inconclusive_rate else 'All tested rates passed: ceiling not reached, only a confirmed lower bound is known.' if high is None else 'Boundary bracketed under the declared SLO; see confirmation duration.')
                return
            if result['inconclusive']:
                break
            high = candidate
        self.save(upper=high, reason='No sustained passing rate was confirmed; do not call the short passing stage a maximum.')
