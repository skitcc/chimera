import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location('capacity_tested', Path(__file__).with_name('capacity.py'))
capacity = importlib.util.module_from_spec(spec)
spec.loader.exec_module(capacity)


class CapacityTests(unittest.TestCase):
    def classify(self, p95=100, dropped=0, cpu=.3, completed=100):
        return capacity.classify({'p95_ms': p95, 'error_rate': 0},
            {'target_rps': 100, 'measurement_sec': 60, 'started_requests': {'count': 6000},
             'dropped_iterations': {'count': dropped}}, completed,
            [] if cpu is None else [{'cpu_cores': cpu}], 2)

    def test_passing_rate_requires_slo_and_delivery(self):
        self.assertTrue(self.classify()['passed'])
        self.assertFalse(self.classify(completed=95)['passed'])

    def test_latency_failure_is_not_a_generator_failure(self):
        result = self.classify(p95=700)
        self.assertFalse(result['passed'])
        self.assertFalse(result['inconclusive'])
        self.assertIn('p95_above_slo', result['reasons'])

    def test_saturated_generator_cannot_define_application_maximum(self):
        self.assertTrue(self.classify(cpu=1.95)['inconclusive'])
        self.assertTrue(self.classify(cpu=None)['inconclusive'])

    def test_drops_with_good_responses_are_inconclusive(self):
        self.assertTrue(self.classify(dropped=1)['inconclusive'])
        self.assertFalse(self.classify(dropped=1, p95=700)['inconclusive'])

    def test_cpu_range_count_is_correct(self):
        self.assertEqual(capacity.cpu_count('4-5'), 2)
        self.assertEqual(capacity.cpu_count('1,3-5,4'), 4)

    def test_adaptive_search_brackets_and_confirms(self):
        with patch.dict(os.environ, {'CAP_START_RPS':'50', 'CAP_MAX_RPS':'800', 'CAP_RESOLUTION_RPS':'25'}):
            search = capacity.Search(Mock())
        search.prepare, search.save = Mock(), Mock()
        def trial(rate, confirm=False):
            result = {'target_rps': rate, 'passed': rate <= 230, 'inconclusive': False, 'confirmation': confirm}
            search.trials.append(result)
            return result
        search.trial = Mock(side_effect=trial)
        search.run()
        final = search.save.call_args.kwargs
        self.assertEqual(final['confirmed'], 225)
        self.assertEqual(final['upper'], 250)
        search.trial.assert_called_with(225, confirm=True)

    def test_failed_confirmation_falls_back_instead_of_claiming_a_maximum(self):
        with patch.dict(os.environ, {'CAP_START_RPS':'50', 'CAP_MAX_RPS':'400', 'CAP_RESOLUTION_RPS':'25'}):
            search = capacity.Search(Mock())
        search.prepare, search.save = Mock(), Mock()
        def trial(rate, confirm=False):
            result = {'target_rps': rate, 'passed': rate <= (200 if confirm else 230),
                      'inconclusive': False, 'confirmation': confirm}
            search.trials.append(result)
            return result
        search.trial = Mock(side_effect=trial)
        search.run()
        self.assertEqual(search.save.call_args.kwargs['confirmed'], 200)
        self.assertEqual(search.save.call_args.kwargs['upper'], 225)


if __name__ == '__main__':
    unittest.main()
