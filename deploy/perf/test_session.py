"""Exercise author error branches without k6, network requests or workload."""
import json
from pathlib import Path
import shutil
import subprocess
import unittest


@unittest.skipUnless(shutil.which('node'), 'node is needed for isolated JS checks')
class AuthorFlowTests(unittest.TestCase):
    def check_flow(self, upload_status, publish_status, expected_track):
        source = Path(__file__).resolve().parents[2] / 'loadtest/perf/session.js'
        harness = r'''
const fs = require('fs'), vm = require('vm'), assert = require('assert');
const source = fs.readFileSync(process.argv[1], 'utf8')
  .replace(/^import .*$/gm, '').replace(/export /g, '');
class Metric { add() {} }
const context = { __ENV: {}, __VU: 1, __ITER: 1,
  Counter: Metric, Trend: Metric, Rate: Metric, sleep() {}, console };
vm.createContext(context);
vm.runInContext(source, context);
context.phase = () => 'measure';
const calls = [];
context.login = () => ({token:'token', id:'author'});
context.call = (step, method, path, body) => {
  calls.push({step, path, body});
  const status = step === 'upload' ? UPLOAD : step === 'publish' ? PUBLISH
    : step === 'create' || step === 'playlist' ? 201 : 200;
  return {status, json: key => key === 'track.id' ? 'new-track'
    : key === 'upload_url' ? 'http://s3/upload' : 'new-playlist'};
};
context.author({authors:[{}], tracks:['seed-ready']});
assert.equal(calls.find(x => x.step === 'playlist-add').body.track_id, EXPECTED);
assert.equal(calls.find(x => x.step === 'card').path, '/tracks/' + EXPECTED);
assert.equal(calls.find(x => x.step === 'track-delete').path, '/tracks/new-track');
if (UPLOAD !== 200 && UPLOAD !== 204)
  assert.equal(calls.filter(x => x.step === 'publish').length, 0);
'''.replace('UPLOAD', str(upload_status)).replace('PUBLISH', str(publish_status)).replace('EXPECTED', json.dumps(expected_track))
        result = subprocess.run(['node', '-e', harness, str(source)],
                                capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_failed_upload_uses_ready_track_without_cascading_conflict(self):
        self.check_flow(503, 200, 'seed-ready')

    def test_failed_publish_uses_ready_track(self):
        self.check_flow(200, 503, 'seed-ready')

    def test_successful_publish_uses_new_track(self):
        self.check_flow(200, 200, 'new-track')
