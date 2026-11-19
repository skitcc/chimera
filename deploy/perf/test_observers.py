import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('perf_observers', Path(__file__).with_name('observers.py'))
observers = importlib.util.module_from_spec(spec)
spec.loader.exec_module(observers)


class ObserverTests(unittest.TestCase):
    def test_memory_units(self):
        self.assertEqual(observers.memory_bytes('512MiB'), 512*1024**2)
        self.assertEqual(observers.memory_bytes('1.5 GB'), 1.5*1000**3)
        with self.assertRaises(ValueError):
            observers.memory_bytes('unknown')

    def test_sampler_filters_unrelated_containers_and_counts_cpu_cores(self):
        calls = []
        def command(args, **kwargs):
            calls.append(args)
            if args[1] == 'ps':
                return 'minikube\nperf-test-prometheus\nperf-test-k6\nunrelated\n'
            return '\n'.join(json.dumps({'Name':name,'CPUPerc':'150.0%',
                                         'MemUsage':'512MiB / 2GiB'})
                             for name in args[5:])
        sampler = observers.DockerSampler('perf-test', command)
        sampler.sample_once()
        self.assertNotIn('unrelated', calls[1])
        self.assertEqual(len(sampler.snapshot()['samples']), 3)
        for sample in sampler.snapshot()['samples']:
            self.assertEqual(sample['cpu_cores'], 1.5)
            self.assertEqual(sample['memory_bytes'], 512*1024**2)
