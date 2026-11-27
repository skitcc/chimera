import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('perf_html', Path(__file__).with_name('report.py'))
report = importlib.util.module_from_spec(spec)
spec.loader.exec_module(report)


class ReportTests(unittest.TestCase):
    def test_partial_series_lists_only_existing_scenarios_and_embeds_charts(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root/'environment.json').write_text(json.dumps({'scenario':'all'}))
            for name in ('full-ramp-fault0', 'full-stress-fault0'):
                out = root/name
                (out/'plots').mkdir(parents=True)
                (out/'summary.json').write_text(json.dumps({'scenario':name,'phases':[]}))
                (out/'http.json').write_text(json.dumps({'requests':10,'error_rate':0,'steps':{}}))
                (out/'plots/latency_percentiles.png').write_bytes(b'image-test')
            (root/'error.txt').write_text('<failed after stress>')
            body = report.generate(root).read_text()
            self.assertIn('full-ramp-fault0', body)
            self.assertIn('full-stress-fault0', body)
            self.assertNotIn('full-soak-fault0', body)
            self.assertIn('&lt;failed after stress&gt;', body)
            self.assertEqual(body.count('data:image/png;base64,'), 2)
            self.assertNotIn('grid-template-columns', body)
            self.assertIn('working set', body)

    def test_leaf_containers_and_prometheus_get_individual_cpu_and_ram(self):
        with tempfile.TemporaryDirectory() as directory:
            out = Path(directory)
            (out/'series').mkdir()
            for metric in ('cpu', 'ram'):
                rows = [{'labels':{'namespace':'chimera','pod':pod,'container':'postgres'},
                         'points':[[100,1],[101,2]]} for pod in ('postgres-0','postgres-replica-0')]
                (out/f'series/{metric}_detailed.json').write_text(json.dumps(rows))
            (out/'docker-resources.json').write_text(json.dumps({'samples':[
                {'time':100,'container':'prometheus','cpu_cores':.1,'memory_bytes':1024**3}]}))
            body = report.resources(out, 100, [])
            self.assertIn('chimera/postgres-0/postgres — CPU', body)
            self.assertIn('chimera/postgres-replica-0/postgres — RAM', body)
            self.assertIn('Docker/prometheus — CPU', body)
            self.assertIn('Docker/prometheus — RAM', body)

    def test_restart_delta_does_not_treat_historic_restarts_as_new(self):
        old = {'namespace':'chimera','pod':'rustfs-0','uid':'old','container':'rustfs',
               'restart_count':14,'ready':True,'last_termination':{'reason':'OOMKilled'}}
        rows = report.restarts([old], [old])
        self.assertEqual(rows[0][1], '0')
        new = {**old,'uid':'new','restart_count':0,'last_termination':None}
        self.assertIn('новый pod', report.restarts([old],[new])[0][1])

    def test_cpu_count_uses_actual_topology_including_single_thread_cores(self):
        hardware = {'cpus':[{'cpu':0,'core':0,'socket':0},{'cpu':1,'core':0,'socket':0},
                            {'cpu':2,'core':1,'socket':0}]}
        text = report.cpu_description('0-2', hardware)
        self.assertIn('3 логических', text)
        self.assertIn('физических ядер: 2', text)
