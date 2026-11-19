import importlib.util
import tempfile
import unittest
import gzip
import json
from pathlib import Path

spec = importlib.util.spec_from_file_location('perf_plot', Path(__file__).with_name('plot.py'))
plot = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plot)


class MetricsTests(unittest.TestCase):
    def test_cumulative_buckets_use_inf_total(self):
        series = [{'labels': {'le': edge}, 'points': [(10, count)]}
                  for edge, count in [('5', 50), ('10', 90), ('20', 100), ('+Inf', 100)]]
        self.assertEqual(plot.percentile_from_buckets(series, .95), [(10, 15)])

    def test_raw_windows_exact_percentile(self):
        samples = [(.1, 10, {}), (.2, 20, {}), (.3, 30, {}), (.4, 40, {})]
        self.assertEqual(plot.raw_windows(samples, 0, 1, .95), [(1, 38.5)])
        self.assertEqual(plot.raw_windows(samples, 0, 1), [(1, 100)])

    def test_first_stage_is_not_lost_at_window_boundary(self):
        raw = {'http_req_duration': [(11, 10, {}), (12, 20, {})],
               'http_req_failed': [(11, 0, {}), (12, 0, {})]}
        report = plot.stage_report({'warmup_sec': 10, 'phases': [{'sec': 20, 'vus': 10}]},
                                   0, [(20, 19.5)], [(20, 0)], [(20, .2)], raw)
        self.assertEqual(report['stable_vus'], 10)
        self.assertAlmostEqual(report['stages'][0]['p95_ms'], 19.5)

    def test_latency_jump_within_slo_is_not_a_capacity_limit(self):
        summary = {'phases': [{'sec': 20, 'vus': n} for n in [10, 25]]}
        result = plot.stage_report(summary, 0, [(10, 50), (30, 200)], [(10, 0), (30, 0)], [(10, 5), (30, 12)])
        self.assertEqual(result['stable_vus'], 25)
        self.assertFalse(result['capacity_found'])
        self.assertTrue(result['stages'][1]['latency_growth_warning'])

    def test_latency_slo_violation_is_reported(self):
        summary = {'phases': [{'sec': 20, 'vus': n} for n in [10, 25]]}
        result = plot.stage_report(summary, 0, [(10, 50), (30, 600)], [(10, 0), (30, 0)], [(10, 5), (30, 12)])
        self.assertEqual(result['stable_vus'], 10)
        self.assertTrue(result['capacity_found'])

    def test_operation_report_separates_api_failures_from_expected_statuses(self):
        raw = {'http_req_duration': [(1, 10, {'step': 'feed', 'status': '503'}), (2, 20, {'step': 'fault', 'status': '404'})],
               'http_req_failed': [(1, 1, {'step': 'feed'}), (2, 0, {'step': 'fault'})]}
        report = plot.operation_report(raw)
        self.assertEqual(report['feed']['error_rate'], 1)
        self.assertEqual(report['fault']['error_rate'], 0)
        self.assertEqual(report['feed']['statuses'], {'503': 1})

    def test_recovered_does_not_imply_an_actual_overload_failure(self):
        summary = {'phases': [{'sec': 20, 'vus': n} for n in [10, 40, 10]]}
        result = plot.recovery_report(summary, 0, [(10, 100), (30, 100), (40, 100), (50, 100)], [(30, 0), (40, 0), (50, 0)])
        self.assertTrue(result['recovered'])
        self.assertFalse(result['overload_degraded'])
        self.assertEqual(result['threshold_p95_ms'], 120)

    def test_missing_error_window_is_not_assumed_healthy(self):
        summary = {'phases': [{'sec': 20, 'vus': n} for n in [10, 40, 10]]}
        result = plot.recovery_report(summary, 0, [(10, 100), (50, 100)], [])
        self.assertFalse(result['recovered'])

    def test_recovery_records_later_relapse(self):
        summary = {'phases': [{'sec': 20, 'vus': n} for n in [10, 40, 10]]}
        result = plot.recovery_report(summary, 0, [(10, 100), (30, 200), (50, 100), (60, 180)], [(30, 0), (50, 0), (60, 0)])
        self.assertTrue(result['recovered'])
        self.assertEqual(result['recovery_sec'], 10)
        self.assertFalse(result['no_observed_relapse'])
        self.assertEqual(result['relapse_windows'][0]['end_sec_since_return'], 20)

    def test_duplicate_event_times_have_one_annotation(self):
        events = plot.grouped_events([(60, 'phase end'), (60, 'load back'), (70, 'recovered'), (60, 'load back')])
        self.assertEqual(events, [(60., 'phase end; load back'), (70., 'recovered')])

    def test_empty_request_window_has_zero_rps(self):
        samples = [(.1, 1, {})]
        self.assertEqual(plot.raw_windows(samples, 0, 2, include_empty=True), [(1, 1), (2, 0)])
        self.assertEqual(plot.raw_windows(samples, 0, 2, .95), [(1, 1)])

    def test_setup_only_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            with gzip.open(Path(directory) / 'samples.json.gz', 'wt') as f:
                f.write(json.dumps({'type': 'Point', 'metric': 'http_req_duration', 'data': {
                    'time': '2026-01-01T00:00:00Z', 'value': 10, 'tags': {}}}) + '\n')
            with self.assertRaisesRegex(ValueError, 'no measured'):
                plot.raw_metrics(Path(directory))

    def test_recovery_requires_full_post_overload_window(self):
        summary = {'phases': [{'sec': 20, 'vus': n} for n in [10, 40, 10]]}
        result = plot.recovery_report(summary, 0, [(10, 100), (40, 100), (40.5, 100), (41, 100)], [(40, 0), (40.5, 0), (41, 0)])
        self.assertEqual(result['recovery_sec'], 1)
        self.assertEqual(result['window_sec'], 1)


if __name__ == '__main__':
    unittest.main()
