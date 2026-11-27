import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location('perf_runner', Path(__file__).with_name('runner.py'))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)
ROOT = Path(__file__).resolve().parents[2]


def workload(name, ready=True):
    return {'kind': 'Deployment', 'metadata': {'name': name, 'generation': 1},
            'spec': {'replicas': 1, 'template': {'spec': {'containers': [{'image': 'test', 'envFrom': [{'configMapRef': {'name': 'gateway-config'}}]}]}}},
            'status': {'readyReplicas': int(ready), 'observedGeneration': 1}}


class RunnerTests(unittest.TestCase):
    def make_measure_runner(self, out):
        r = runner.Runner.__new__(runner.Runner)
        r.out, r.profile, r.prefix = out, 'full', 'fake-perf'
        r.uid, r.gid = os.getuid(), os.getgid()
        r.load_cpus, r.docker_sampler = None, None
        r.base, r.track_bytes, r.image, r.prom_url = 'http://fake', 1024, 'fake-image', 'http://fake-prom'
        r.containers, r.results = [], []
        r.snapshot_pods = Mock()
        return r

    def test_measure_updates_one_html_for_ramp_stress_and_soak_without_real_commands(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            r = self.make_measure_runner(root)
            def fake_process(args, **kwargs):
                if args[0] == 'docker':
                    scenario = next(x.split('=',1)[1] for x in args if x.startswith('SCENARIO='))
                    out = root/f'full-{scenario}-fault0'
                    (out/'summary.json').write_text(json.dumps({'scenario':scenario,'phases':[]}))
                    (out/'http.json').write_text(json.dumps({'requests':10,'error_rate':0,'steps':{}}))
                return SimpleNamespace(returncode=0)
            with patch.object(runner.subprocess,'run',side_effect=fake_process), patch.object(runner,'command',return_value=''):
                for scenario in ('ramp','stress','soak'):
                    r.measure(scenario,0,10)
                    html = (root/'report.html').read_text()
                    self.assertIn(f'full-{scenario}-fault0', html)
            self.assertEqual(len(r.results),3)
            self.assertEqual(r.snapshot_pods.call_count,6)

    def test_analysis_timeout_keeps_partial_html_and_records_exit_code(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            r = self.make_measure_runner(root)
            def fake_process(args, **kwargs):
                if args[0] == 'docker':
                    return SimpleNamespace(returncode=0)
                raise subprocess.TimeoutExpired(args,300)
            with patch.object(runner.subprocess,'run',side_effect=fake_process), patch.object(runner,'command',return_value=''):
                with self.assertRaisesRegex(RuntimeError,'scenario failed'):
                    r.measure('ramp',0,10)
            self.assertTrue((root/'report.html').exists())
            self.assertEqual(json.loads((root/'full-ramp-fault0/run.json').read_text())['analysis_exit'],124)
            self.assertIn('Analysis timed out',(root/'report.html').read_text())

    def test_gateway_alone_does_not_mean_ready(self):
        problems = runner.workload_problems([workload('gateway')])
        self.assertIn('identity: workload missing', problems)
        self.assertIn('postgres: workload missing', problems)

    def test_preflight_is_read_only(self):
        r = runner.Runner.__new__(runner.Runner)
        r.profile, r.scenario, r.base, r.track_bytes = 'quick', 'ramp', 'http://test', 1024
        r.http = Mock()
        items = [workload(name) for name in runner.WORKLOADS]
        r.k = Mock(side_effect=[json.dumps({'items': items}), json.dumps({'data': {'AUTH_RATE_LIMIT': '100000'}})])
        with tempfile.TemporaryDirectory() as directory, patch.object(runner, 'command', return_value='0-3'):
            r.out = Path(directory)
            r.preflight()
        self.assertTrue(all(call.args[0] == 'get' for call in r.k.call_args_list))

    def test_trio_seeds_once_and_runs_exactly_three_scenarios(self):
        r = runner.Runner.__new__(runner.Runner)
        r.preflight, r.seed, r.observe, r.cleanup, r.diagnostics = [Mock() for _ in range(5)]
        r.scenario = 'trio'
        r.isolated, r.load_cpus, r.observe_cpus = False, None, None
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {'ISOLATE_CPUS': '0', 'PREFLIGHT_ONLY': '0'}):
            r.out = Path(directory)
            (r.out / 'knee.json').write_text(json.dumps({'stable_vus': 50}))
            r.measure = Mock(return_value=r.out)
            r.run()
        self.assertEqual([call.args for call in r.measure.call_args_list], [('ramp', 0, 10), ('stress', 0, 50), ('soak', 0, 50)])
        r.seed.assert_called_once()
        r.cleanup.assert_called_once()

    def test_failed_preflight_never_seeds_or_starts_load(self):
        r = runner.Runner.__new__(runner.Runner)
        r.preflight = Mock(side_effect=RuntimeError('not ready'))
        r.seed, r.observe, r.measure, r.cleanup, r.diagnostics = [Mock() for _ in range(5)]
        with tempfile.TemporaryDirectory() as directory:
            r.out = Path(directory)
            with self.assertRaisesRegex(RuntimeError, 'not ready'):
                r.run()
        r.seed.assert_not_called()
        r.observe.assert_not_called()
        r.measure.assert_not_called()
        r.cleanup.assert_called_once()

    def test_failed_ramp_stops_series_without_further_scenarios(self):
        r = runner.Runner.__new__(runner.Runner)
        r.preflight, r.seed, r.observe, r.cleanup, r.diagnostics = [Mock() for _ in range(5)]
        r.measure = Mock(side_effect=RuntimeError('k6 failed'))
        r.scenario = 'all'
        r.isolated, r.load_cpus, r.observe_cpus = False, None, None
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {'ISOLATE_CPUS': '0'}):
            r.out = Path(directory)
            with self.assertRaisesRegex(RuntimeError, 'k6 failed'):
                r.run()
        r.seed.assert_called_once()
        r.measure.assert_called_once_with('ramp', 0, 10)
        r.cleanup.assert_called_once()


class PostgresEntrypointTests(unittest.TestCase):
    def test_primary_executes_official_entrypoint_without_readiness_watchdog(self):
        with tempfile.TemporaryDirectory() as directory:
            tmp = Path(directory)
            executable = tmp / 'docker-entrypoint.sh'
            executable.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > "$ARGUMENTS"\n')
            executable.chmod(0o755)
            env = {**os.environ, 'PATH': f'{tmp}:' + os.environ['PATH'], 'ARGUMENTS': str(tmp / 'args'),
                   'POSTGRES_REPLICATION_USER': 'replicator', 'POSTGRES_REPLICATION_PASSWORD': 'test-only'}
            subprocess.run(['sh', ROOT / 'deploy/postgres/primary.sh'], env=env, check=True, timeout=2)
            self.assertEqual((tmp / 'args').read_text().splitlines()[0], 'postgres')
            self.assertIn('wal_level=replica', (tmp / 'args').read_text())

    def test_replica_publishes_only_completed_clone_with_fast_checkpoint(self):
        with tempfile.TemporaryDirectory() as directory:
            tmp = Path(directory)
            backup = tmp / 'pg_basebackup'
            backup.write_text('''#!/bin/sh
printf '%s\\n' "$@" >> "$ARGUMENTS"
while [ "$#" -gt 0 ]; do
  if [ "$1" = -D ]; then shift; dest=$1; fi
  shift
done
printf '17' > "$dest/PG_VERSION"
touch "$dest/standby.signal" "$dest/backup_manifest"
''')
            postgres = tmp / 'postgres'
            postgres.write_text('#!/bin/sh\ntest -f "$PGDATA/.chimera-clone-complete"\n')
            backup.chmod(0o755)
            postgres.chmod(0o755)
            env = {**os.environ, 'PATH': f'{tmp}:' + os.environ['PATH'], 'ARGUMENTS': str(tmp / 'args'),
                   'PGDATA': str(tmp / 'data'), 'POSTGRES_REPLICATION_USER': 'replicator',
                   'POSTGRES_REPLICATION_PASSWORD': 'test-only'}
            subprocess.run(['sh', ROOT / 'deploy/postgres/replica.sh'], env=env, check=True, timeout=2)
            self.assertTrue((tmp / 'data/.chimera-clone-complete').exists())
            self.assertFalse((tmp / 'data.bootstrap').exists())
            self.assertIn('--checkpoint=fast', (tmp / 'args').read_text())

    def test_replication_role_is_created_during_official_initialization(self):
        with tempfile.TemporaryDirectory() as directory:
            tmp = Path(directory)
            executable = tmp / 'psql'
            executable.write_text('#!/bin/sh\ncat >> "$SQL_CAPTURE"\n')
            executable.chmod(0o755)
            data = tmp / 'data'
            data.mkdir()
            env = {**os.environ, 'PATH': f'{tmp}:' + os.environ['PATH'], 'SQL_CAPTURE': str(tmp / 'sql'),
                   'PGDATA': str(data), 'POSTGRES_USER': 'chimera',
                   'POSTGRES_REPLICATION_USER': 'replicator', 'POSTGRES_REPLICATION_PASSWORD': 'test-only',
                   'IDENTITY_DB_PASSWORD': 'test-only', 'CATALOG_DB_PASSWORD': 'test-only', 'SOCIAL_DB_PASSWORD': 'test-only'}
            subprocess.run(['sh', ROOT / 'deploy/postgres/init-databases.sh'], env=env, check=True, timeout=2)
            self.assertIn('WITH REPLICATION LOGIN', (tmp / 'sql').read_text())
            self.assertEqual((data / 'pg_hba.conf').read_text(), 'host replication replicator all scram-sha-256\n')

    def test_partial_old_replica_is_rejected_instead_of_started(self):
        with tempfile.TemporaryDirectory() as directory:
            data = Path(directory) / 'data'
            data.mkdir()
            (data / 'PG_VERSION').write_text('17')
            env = {**os.environ, 'PGDATA': str(data), 'POSTGRES_REPLICATION_USER': 'replicator',
                   'POSTGRES_REPLICATION_PASSWORD': 'test-only'}
            result = subprocess.run(['sh', ROOT / 'deploy/postgres/replica.sh'], env=env, capture_output=True, timeout=2)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b'incomplete replica', result.stderr)


if __name__ == '__main__':
    unittest.main()
