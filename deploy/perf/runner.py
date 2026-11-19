#!/usr/bin/env python3
"""One bounded lifecycle: preflight, seed once, measure, collect, clean own resources."""
import contextlib
import datetime as dt
import fcntl
import json
import os
from pathlib import Path
import pwd
import shlex
import shutil
import signal
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[2]
WORKLOADS = {'gateway', 'identity', 'catalog', 'social', 'edge', 'postgres', 'postgres-replica', 'kafka', 'rustfs'}
PASSWORD = 'password1'


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


def command(argv, timeout=30, **kwargs):
    return subprocess.run([str(x) for x in argv], check=True, text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout, **kwargs).stdout


def workload_problems(items):
    workloads = {x['metadata']['name']: x for x in items if x['kind'] in ('Deployment', 'StatefulSet')}
    problems = []
    for name in sorted(WORKLOADS):
        obj = workloads.get(name)
        if not obj:
            problems.append(f'{name}: workload missing')
            continue
        desired = obj['spec'].get('replicas', 1)
        status = obj.get('status', {})
        ready = status.get('readyReplicas', 0)
        if desired < 1 or ready < desired or status.get('observedGeneration', 0) < obj['metadata'].get('generation', 0):
            problems.append(f'{name}: ready {ready}/{desired}')
    for pod in (x for x in items if x['kind'] == 'Pod' and not x['metadata'].get('deletionTimestamp')):
        if pod['metadata'].get('labels', {}).get('app.kubernetes.io/name') not in WORKLOADS:
            continue
        for container in pod.get('status', {}).get('containerStatuses', []):
            waiting = container.get('state', {}).get('waiting', {})
            if waiting.get('reason') in ('CrashLoopBackOff', 'ImagePullBackOff', 'ErrImagePull'):
                problems.append(f"{pod['metadata']['name']}/{container['name']}: {waiting['reason']}")
    return problems


class Runner:
    def __init__(self):
        self.profile = os.getenv('PROFILE', 'quick')
        self.scenario = os.getenv('SCENARIO', 'ramp')
        self.fault = float(os.getenv('FAULT_RATE', '0'))
        if self.profile not in ('quick', 'full') or self.scenario not in ('ramp', 'stress', 'soak', 'trio', 'all'):
            raise ValueError('PROFILE=quick|full; SCENARIO=ramp|stress|soak|trio|all')
        if self.fault not in (0, .1, .2):
            raise ValueError('FAULT_RATE must be 0, 0.1 or 0.2')
        self.kubectl = shlex.split(os.getenv('KUBECTL', 'minikube kubectl --'))
        self.ns = os.getenv('K8S_NAMESPACE', 'chimera')
        self.base = os.getenv('BASE_URL', 'http://chimera.192.168.49.2.nip.io').rstrip('/')
        self.track_bytes = int(os.getenv('TRACK_BYTES', '65536'))
        if not 1 <= self.track_bytes <= 104857600:
            raise ValueError('TRACK_BYTES must be between 1 and 104857600')
        self.uid = int(os.getenv('SUDO_UID', os.getuid()))
        self.gid = int(os.getenv('SUDO_GID', os.getgid()))
        if os.getuid() == 0 and os.getenv('SUDO_USER'):
            invoker = pwd.getpwnam(os.environ['SUDO_USER']).pw_dir
            os.environ.setdefault('MINIKUBE_HOME', invoker)
            os.environ.setdefault('KUBECONFIG', f'{invoker}/.kube/config')
        self.id = dt.datetime.now(dt.timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '-' + uuid.uuid4().hex[:6]
        self.out = Path(os.getenv('PERF_OUT', str(ROOT / 'perf-results'))).resolve() / self.id
        self.out.mkdir(parents=True)
        self.prefix = f'chimera-perf-{self.id.lower()}'
        self.processes = []
        self.containers = []
        self.users = []
        self.results = []
        self.isolated = False
        self.load_cpus = None
        self.observe_cpus = None
        self.log_files = []
        self.docker_sampler = None
        self.image = os.getenv('K6_IMAGE', 'grafana/k6:0.57.0')
        os.chown(self.out, self.uid, self.gid)

    def k(self, *args, timeout=30):
        return command(self.kubectl + ['-n', self.ns] + list(args), timeout=timeout)

    def snapshot_pods(self, target):
        """Store statuses/limits, excluding pod environment and secrets."""
        try:
            pods = json.loads(command(self.kubectl + ['get', 'pods', '--all-namespaces', '-o', 'json'], timeout=10))['items']
            rows = []
            for pod in pods:
                specs = {c['name']: c for c in pod['spec'].get('containers', [])}
                for container in pod.get('status', {}).get('containerStatuses', []):
                    rows.append({'namespace': pod['metadata']['namespace'], 'pod': pod['metadata']['name'],
                                 'uid': pod['metadata']['uid'], 'container': container['name'],
                                 'restart_count': container.get('restartCount', 0),
                                 'ready': container.get('ready', False),
                                 'last_termination': container.get('lastState', {}).get('terminated'),
                                 'state': container.get('state', {}),
                                 'resources': specs.get(container['name'], {}).get('resources', {})})
            write_json(target, rows)
        except Exception as exc:
            target.with_suffix('.error.txt').write_text(str(exc) + '\n')

    def build_report(self):
        try:
            from report import generate
            path = generate(self.out)
            os.chown(path, getattr(self, 'uid', os.getuid()), getattr(self, 'gid', os.getgid()))
            print(f'HTML report: {path}', flush=True)
        except Exception as exc:
            (self.out / 'html-error.txt').write_text(str(exc) + '\n')
            print(f'HTML report failed: {exc}; see html-error.txt', file=sys.stderr)

    def http(self, method, path, body=None, token=None, expected=(200,), timeout=20):
        url = path if path.startswith('http') else self.base + '/api/v1' + path
        headers = {}
        if isinstance(body, dict):
            body = json.dumps(body).encode()
            headers['Content-Type'] = 'application/json'
        elif isinstance(body, bytes):
            headers['Content-Type'] = 'application/octet-stream'
        if token:
            headers['Authorization'] = 'Bearer ' + token
        req = urllib.request.Request(url, data=body, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=timeout) as res:
                status, content = res.status, res.read()
        except urllib.error.HTTPError as exc:
            status, content = exc.code, exc.read()
        if status not in expected:
            # Avoid dumping signed object URLs or JWTs into diagnostics.
            raise RuntimeError(f'{method} {path.split("?")[0] if not path.startswith("http") else "object"}: HTTP {status}: {content[:200]!r}')
        return json.loads(content) if content and content[:1] in (b'{', b'[') else content

    def diagnostics(self):
        deadline = time.monotonic() + 15
        terminations = []
        for name, args in [('pods', ['get', 'pods', '-o', 'wide']),
                           ('events', ['get', 'events', '--sort-by=.lastTimestamp'])]:
            with contextlib.suppress(Exception):
                (self.out / f'{name}.txt').write_text(self.k(*args, timeout=3))
        for svc in sorted(WORKLOADS):
            if time.monotonic() >= deadline:
                break
            with contextlib.suppress(Exception):
                pods = json.loads(self.k('get', 'pods', '-l', f'app.kubernetes.io/name={svc}', '-o', 'json', timeout=3))['items']
                for pod in pods:
                    for container in pod.get('status', {}).get('containerStatuses', []):
                        terminations.append({
                            'pod': pod['metadata']['name'], 'container': container['name'],
                            'restart_count': container.get('restartCount', 0),
                            'last_termination': container.get('lastState', {}).get('terminated'),
                            'state': container.get('state', {}),
                        })
                    write_json(self.out / 'container-terminations.json', terminations)
                    for c in pod['spec']['containers']:
                        for previous in (False, True):
                            args = ['logs', pod['metadata']['name'], '-c', c['name'], '--tail=60']
                            if previous:
                                args.append('--previous')
                            if time.monotonic() >= deadline:
                                return
                            with contextlib.suppress(Exception):
                                text = self.k(*args, timeout=3)
                                (self.out / f"{pod['metadata']['name']}-{c['name']}-{'previous' if previous else 'current'}.log").write_text(text)

    def preflight(self):
        deadline = time.monotonic() + int(os.getenv('READY_TIMEOUT', '90'))
        while True:
            items = json.loads(self.k('get', 'deployments,statefulsets,pods', '-o', 'json'))['items']
            problems = workload_problems(items)
            if not problems:
                break
            print('preflight: ' + '; '.join(problems), flush=True)
            if any('BackOff' in p or 'ErrImagePull' in p for p in problems) or time.monotonic() >= deadline:
                raise RuntimeError('stand is not ready; no load started. See diagnostics. Run perf-prepare separately after fixing the stand.')
            time.sleep(min(5, max(0, deadline - time.monotonic())))
        deployment = next(x for x in items if x['kind'] == 'Deployment' and x['metadata']['name'] == 'gateway')
        refs = deployment['spec']['template']['spec']['containers'][0].get('envFrom', [])
        rate = 0
        for ref in refs:
            if 'configMapRef' in ref:
                data = json.loads(self.k('get', 'configmap', ref['configMapRef']['name'], '-o', 'json')).get('data', {})
                rate = int(data.get('AUTH_RATE_LIMIT', rate))
        if rate < 10000:
            raise RuntimeError(f'Gateway AUTH_RATE_LIMIT={rate}; use make -C backend perf-prepare once before measuring')
        self.http('GET', '/tracks?limit=1')
        write_json(self.out / 'environment.json', {
            'base_url': self.base, 'profile': self.profile, 'scenario': self.scenario,
            'auth_rate_limit': rate, 'track_bytes': self.track_bytes,
            'workloads': [{ 'name': x['metadata']['name'], 'kind': x['kind'],
                'replicas': x['spec'].get('replicas'), 'images': [c['image'] for c in x['spec']['template']['spec']['containers']]}
                for x in items if x['kind'] in ('Deployment', 'StatefulSet') and x['metadata']['name'] in WORKLOADS],
            'minikube_cpuset': command(['docker', 'inspect', '-f', '{{.HostConfig.CpusetCpus}}', 'minikube']).strip(),
            'infrastructure_reset': False,
        })
        print('preflight passed; infrastructure is unchanged', flush=True)

    def login(self, user, timeout=20):
        return self.http('POST', '/sessions', {'email': user['email'], 'password': PASSWORD}, expected=(201,), timeout=timeout)['token']

    def seed(self):
        dataset = {}
        for role, count in [('listeners', 30), ('subscribers', 10), ('authors', 10)]:
            dataset[role] = []
            for index in range(count):
                email = f'perf-{self.id}-{role}-{index}@example.com'
                res = self.http('POST', '/users', {'email': email, 'name': role, 'password': PASSWORD}, expected=(201,))
                user = {'email': email, 'id': res['id']}
                self.users.append(user)
                dataset[role].append(user)
        tokens = {u['id']: self.login(u) for u in dataset['authors']}
        dataset.update(tracks=[], playlists=[])
        for index in range(40):
            author = dataset['authors'][index % len(dataset['authors'])]
            token = tokens[author['id']]
            created = self.http('POST', '/tracks', {'title': f'Seed {self.id} {index}', 'artist': 'Seed', 'size': self.track_bytes}, token, (201,))
            track = created['track']['id']
            self.http('PUT', created['upload_url'], b'x' * self.track_bytes, expected=(200, 204))
            self.http('PATCH', f'/tracks/{track}', {'status': 'ready'}, token)
            # Replica lag must not make dataset construction flaky.
            for attempt in range(50):
                if self.http('GET', f'/tracks/{track}', token=token).get('status') == 'ready':
                    break
                time.sleep(.1)
            else:
                raise RuntimeError('published seed track did not become visible within 5 seconds')
            dataset['tracks'].append(track)
            if index % 8 == 0:
                playlist = self.http('POST', '/playlists', {'title': f'Seed {self.id} {index}', 'is_public': True}, token, (201,))
                self.http('POST', f"/playlists/{playlist['id']}/tracks", {'track_id': track, 'position': 0}, token, (200, 201, 204))
                dataset['playlists'].append({'id': playlist['id'], 'owner': author['id']})
        write_json(self.out / 'dataset.json', dataset)
        print('dataset ready: 50 users, 40 tracks, 5 playlists; reused across all scenarios', flush=True)

    def start_process(self, argv, log):
        handle = log.open('w')
        self.log_files.append(handle)
        if self.observe_cpus:
            argv = ['taskset', '-c', self.observe_cpus] + argv
        process = subprocess.Popen(argv, stdout=handle, stderr=subprocess.STDOUT, start_new_session=True)
        self.processes.append(process)
        return process

    def observe(self):
        proxy_port, prom_port = free_port(), free_port()
        self.prom_url = f'http://127.0.0.1:{prom_port}'
        self.start_process(self.kubectl + ['proxy', f'--port={proxy_port}', '--address=127.0.0.1'], self.out / 'proxy.log')
        node = json.loads(command(self.kubectl + ['get', 'nodes', '-o', 'json']))['items'][0]['metadata']['name']
        config = (ROOT / 'deploy/perf/prometheus.yml').read_text().replace('NODE_NAME', node).replace('127.0.0.1:8001', f'127.0.0.1:{proxy_port}')
        (self.out / 'prometheus.yml').write_text(config)
        name = self.prefix + '-prometheus'
        self.containers.append(name)
        args = ['docker', 'run', '-d', '--name', name, '--network', 'host']
        if self.observe_cpus:
            args += ['--cpuset-cpus=' + self.observe_cpus]
            if os.getuid() == 0:
                args += ['--cgroup-parent=chimera-perf-observe.slice']
        command(args + ['-v', f'{self.out}/prometheus.yml:/etc/prometheus/prometheus.yml:ro',
                        os.getenv('PROM_IMAGE', 'prom/prometheus:v2.55.1'), '--config.file=/etc/prometheus/prometheus.yml',
                        f'--web.listen-address=:{prom_port}', '--web.enable-remote-write-receiver', '--enable-feature=native-histograms'])
        for attempt in range(30):
            try:
                with urllib.request.urlopen(self.prom_url + '/api/v1/targets', timeout=2) as res:
                    targets = json.load(res)['data']['activeTargets']
                if targets and all(t['health'] == 'up' for t in targets):
                    break
            except (OSError, ValueError, KeyError):
                pass
            time.sleep(1)
        else:
            raise RuntimeError('Prometheus cannot scrape cAdvisor; see proxy.log')
        if shutil.which('iostat'):
            self.start_process(['iostat', '-y', '-dx', '1'], self.out / 'iostat.log')
        from observers import DockerSampler
        def observe_command(argv, **kwargs):
            if self.observe_cpus:
                argv = ['taskset', '-c', self.observe_cpus] + argv
            return command(argv, **kwargs)
        self.docker_sampler = DockerSampler(self.prefix, observe_command)
        self.docker_sampler.start()
        print(f'observation ready: {self.prom_url}', flush=True)

    def measure(self, scenario, fault, stable, overrides=None, label=None, script='session.js', dataset='dataset.json'):
        out = self.out / (label or f'{self.profile}-{scenario}-fault{int(fault*100)}')
        out.mkdir()
        os.chown(out, self.uid, self.gid)
        self.snapshot_pods(out / 'pods-before.json')
        name = self.prefix + '-k6'
        self.containers.append(name)
        args = ['docker', 'run', '--name', name, '--network', 'host', '--user', f'{self.uid}:{self.gid}']
        if self.load_cpus:
            args += ['--cpuset-cpus=' + self.load_cpus]
            if os.getuid() == 0:
                args += ['--cgroup-parent=chimera-perf-load.slice']
        values = {'BASE_URL': self.base, 'PROFILE': self.profile, 'SCENARIO': scenario,
                  'FAULT_RATE': str(fault), 'STABLE_VUS': str(stable), 'SUMMARY_DIR': '/out',
                  'WARMUP_SEC': os.getenv('WARMUP_SEC', '10' if self.profile == 'quick' else '30'),
                  'RAMP_STAGE_SEC': os.getenv('RAMP_STAGE_SEC', ''),
                  'TRACK_BYTES': str(self.track_bytes), 'DATASET_FILE': '/data/dataset.json',
                  'SOAK_DURATION': os.getenv('SOAK_DURATION', ''),
                  'K6_PROMETHEUS_RW_SERVER_URL': self.prom_url + '/api/v1/write',
                  'K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM': 'true'}
        values.update(overrides or {})
        for key, value in values.items():
            args += ['-e', f'{key}={value}']
        args += ['-v', f'{ROOT}/loadtest:/loadtest:ro', '-v', f'{self.out}/{dataset}:/data/dataset.json:ro',
                 '-v', f'{out}:/out', self.image, 'run', '--out', 'experimental-prometheus-rw',
                 '--out', 'json=/out/samples.json.gz', f'/loadtest/perf/{script}']
        description = f'baseline={stable} VU'
        print(f'measure: {scenario}, fault={fault}, {description}', flush=True)
        started = time.time()
        result = {'profile': self.profile, 'scenario': scenario, 'fault_rate': fault, 'stable_vus': stable,
                  'start_unix': started, 'k6_exit': None, 'success': False}
        write_json(out / 'run.json', result)
        status = None
        try:
            with (out / 'k6.log').open('w') as log:
                # The Docker CLI directly returns k6's exit status.
                status = subprocess.run(args, stdout=log, stderr=subprocess.STDOUT, timeout=3600 if self.profile == 'full' else 900).returncode
        finally:
            ended = time.time()
            result.update(end_unix=ended, k6_exit=status)
            write_json(out / 'run.json', result)
            self.snapshot_pods(out / 'pods-after.json')
            if self.docker_sampler:
                write_json(out / 'docker-resources.json', self.docker_sampler.snapshot(started, ended))
        command(['docker', 'rm', name])
        self.containers.remove(name)
        with (out / 'analysis.log').open('w') as log:
            env = {**os.environ, 'PROM_URL': self.prom_url, 'MPLCONFIGDIR': str(self.out / 'matplotlib-cache')}
            try:
                analysis = subprocess.run([sys.executable, str(ROOT / 'deploy/perf/plot.py'), 'run', str(out)],
                                          env=env, stdout=log, stderr=subprocess.STDOUT,
                                          timeout=int(os.getenv('ANALYSIS_TIMEOUT', '300'))).returncode
            except subprocess.TimeoutExpired:
                analysis = 124
                (out / 'analysis-error.txt').write_text('Analysis timed out; raw k6 and any collected resource series are preserved.\n')
        result.update(analysis_exit=analysis, success=status == 0 and analysis == 0)
        write_json(out / 'run.json', result)
        self.results.append({**result, 'directory': str(out)})
        write_json(self.out / 'results.json', self.results)
        self.build_report()
        print(f"{scenario}: k6={status}, analysis={analysis}; {out}", flush=True)
        if not result['success']:
            raise RuntimeError(f'scenario failed; see {out}/k6.log and analysis.log; remaining scenarios skipped')
        return out

    def cleanup(self):
        # Only resources created by this invocation. No kubectl apply/scale/delete.
        if self.docker_sampler:
            write_json(self.out / 'docker-resources.json', self.docker_sampler.finish())
        for name in reversed(self.containers):
            with contextlib.suppress(Exception):
                command(['docker', 'rm', '-f', name])
        for process in reversed(self.processes):
            if process.poll() is None:
                with contextlib.suppress(ProcessLookupError):
                    os.killpg(process.pid, 15)
                with contextlib.suppress(subprocess.TimeoutExpired):
                    process.wait(timeout=5)
        for handle in self.log_files:
            handle.close()
        deadline = time.monotonic() + 30
        remaining = []
        for user in self.users:
            if time.monotonic() >= deadline:
                remaining.append(user)
                continue
            try:
                token = self.login(user, timeout=3)
                self.http('DELETE', f"/users/{user['id']}", token=token, expected=(204,), timeout=3)
            except Exception as exc:
                remaining.append(user)
                print(f"seed cleanup: {user['id']}: {exc}", file=sys.stderr)
        write_json(self.out / 'cleanup.json', {'remaining_users': remaining})
        if remaining:
            print(f'cleanup incomplete: {len(remaining)} users; see cleanup.json', file=sys.stderr)
        if self.isolated:
            with contextlib.suppress(Exception):
                command(['sh', ROOT / 'deploy/perf/isolate.sh', 'release'])
        for directory, _, files in os.walk(self.out):
            with contextlib.suppress(OSError):
                os.chown(directory, self.uid, self.gid)
            for file in files:
                with contextlib.suppress(OSError):
                    os.chown(Path(directory) / file, self.uid, self.gid)

    def run(self):
        print(f'results: {self.out}', flush=True)
        try:
            self.preflight()
            if os.getenv('PREFLIGHT_ONLY') == '1':
                return
            from observers import topology
            write_json(self.out / 'hardware.json', topology(command))
            # Isolation is optional and applied only after the existing stand is ready.
            if os.getenv('ISOLATE_CPUS') == '1':
                command(['sh', ROOT / 'deploy/perf/isolate.sh', 'apply'])
                self.isolated = True
                self.load_cpus = command(['sh', ROOT / 'deploy/perf/isolate.sh', 'cpus', 'load']).strip()
                self.observe_cpus = command(['sh', ROOT / 'deploy/perf/isolate.sh', 'cpus', 'observe']).strip()
            environment = json.loads((self.out / 'environment.json').read_text()) if (self.out / 'environment.json').exists() else {}
            environment['cpu_isolation'] = {'enabled': self.isolated, 'load_cpus': self.load_cpus, 'observe_cpus': self.observe_cpus,
                                            'app_cpus': command(['sh', ROOT / 'deploy/perf/isolate.sh', 'cpus', 'app']).strip() if self.isolated else None}
            write_json(self.out / 'environment.json', environment)
            self.seed()
            self.observe()
            if self.scenario in ('trio', 'all'):
                ramp = self.measure('ramp', 0, 10)
                stable = json.loads((ramp / 'knee.json').read_text()).get('stable_vus')
                if not stable:
                    raise RuntimeError('ramp found no stable stage; stress/soak are not valid and will not run')
                self.measure('stress', 0, stable)
                faults = (0, .1, .2) if self.scenario == 'all' else (0,)
                soaks = [self.measure('soak', fault, stable) for fault in faults]
                if len(soaks) > 1:
                    command([sys.executable, ROOT / 'deploy/perf/plot.py', 'compare', self.out / 'compare', *soaks])
            else:
                stable = int(os.getenv('STABLE_VUS', '10'))
                self.measure(self.scenario, self.fault, stable)
        except BaseException as exc:
            (self.out / 'error.txt').write_text(str(exc) + '\n')
            self.diagnostics()
            raise
        finally:
            try:
                self.cleanup()
            finally:
                self.build_report()


def main():
    lock = (ROOT / '.perf-run.lock').open('a')
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        raise SystemExit('another perf run is active')
    runner = Runner()
    signal.signal(signal.SIGTERM, lambda signum, frame: (_ for _ in ()).throw(KeyboardInterrupt('terminated')))
    try:
        runner.run()
    except (Exception, KeyboardInterrupt) as exc:
        print(f'ERROR: {exc}\nArtifacts: {runner.out}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
