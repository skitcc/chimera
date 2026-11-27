"""Read-only host/container telemetry; never creates a workload."""
import json
import re
import threading
import time


def memory_bytes(text):
    match = re.fullmatch(r'\s*([\d.]+)\s*([kmgt]?i?b)\s*', text, re.I)
    if not match:
        raise ValueError(f'Unsupported memory unit: {text}')
    number, unit = float(match[1]), match[2].lower()
    powers = {'b': 1, 'kb': 1000, 'mb': 1000**2, 'gb': 1000**3, 'tb': 1000**4,
              'kib': 1024, 'mib': 1024**2, 'gib': 1024**3, 'tib': 1024**4}
    return number * powers[unit]


def topology(command):
    try:
        rows = command(['lscpu', '-p=CPU,CORE,SOCKET']).splitlines()
        cpus = []
        for row in rows:
            if not row or row.startswith('#'):
                continue
            cpu, core, socket = map(int, row.split(','))
            cpus.append({'cpu': cpu, 'core': core, 'socket': socket})
        return {'logical_cpus': len(cpus),
                'physical_cores': len({(x['socket'], x['core']) for x in cpus}),
                'cpus': cpus, 'source': 'lscpu -p=CPU,CORE,SOCKET'}
    except Exception as exc:
        return {'error': str(exc), 'cpus': []}


class DockerSampler:
    """Stats for host minikube and this runner's k6/Prometheus containers.

    Kubernetes leaf containers are measured separately through cAdvisor. The
    minikube total includes them and must not be added to their individual use.
    Docker CLI sampling has overhead; timestamps record the actual cadence.
    """
    def __init__(self, prefix, command):
        self.prefix, self.command = prefix, command
        self.stop = threading.Event()
        self.lock = threading.Lock()
        self.samples, self.errors = [], []
        self.thread = threading.Thread(target=self.sample, daemon=True)

    def start(self):
        self.thread.start()

    def sample_once(self):
        names = self.command(['docker', 'ps', '--format', '{{.Names}}'], timeout=4).splitlines()
        names = [x for x in names if x == 'minikube' or x.startswith(self.prefix + '-')]
        if not names:
            return
        output = self.command(['docker', 'stats', '--no-stream', '--format', '{{json .}}', *names], timeout=8)
        timestamp = time.time()
        rows = []
        for line in output.splitlines():
            item = json.loads(line)
            name = item['Name']
            label = 'minikube (total)' if name == 'minikube' else name.removeprefix(self.prefix + '-')
            rows.append({'time': timestamp, 'container': label,
                         'cpu_cores': float(item['CPUPerc'].rstrip('%')) / 100,
                         'memory_bytes': memory_bytes(item['MemUsage'].split('/')[0]),
                         'source': 'docker stats'})
        with self.lock:
            self.samples.extend(rows)

    def sample(self):
        while not self.stop.is_set():
            try:
                self.sample_once()
            except Exception as exc:
                with self.lock:
                    # The generator may terminate between docker ps and stats.
                    self.errors.append({'time': time.time(), 'error': str(exc)[:300]})
            self.stop.wait(1)

    def snapshot(self, start=None, end=None):
        with self.lock:
            return {'source': 'docker stats --no-stream',
                    'interval_note': '1s pause between calls; actual cadence includes Docker overhead',
                    'samples': [x for x in self.samples if (start is None or x['time'] >= start)
                                and (end is None or x['time'] <= end)],
                    'errors': [x for x in self.errors if (start is None or x['time'] >= start)
                               and (end is None or x['time'] <= end)]}

    def finish(self):
        self.stop.set()
        self.thread.join(timeout=13)
        return self.snapshot()
