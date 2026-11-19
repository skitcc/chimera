// Open workload: exactly one HTTP request per scheduled iteration, no sleeps.
import http from 'k6/http'
import exec from 'k6/execution'
import { Counter, Trend, Rate } from 'k6/metrics'
const data = JSON.parse(open(__ENV.DATASET_FILE))
const api = __ENV.BASE_URL.replace(/\/$/, '') + '/api/v1'
const rate = Number(__ENV.CAP_TARGET_RPS)
const duration = Number(__ENV.CAP_STEP_SEC)
const warmup = Number(__ENV.CAP_WARMUP_SEC || 10)
const workload = __ENV.CAP_WORKLOAD || 'mixed'
const started = new Counter('started_requests')
const measured = new Counter('measured_requests')
const ms = new Trend('measured_ms', true)
const failures = new Rate('unexpected_fail')
const streamBytes = new Counter('stream_bytes')
export const options = {
  scenarios: { capacity: {
    executor: 'constant-arrival-rate', rate, timeUnit: '1s',
    duration: `${warmup + duration}s`, gracefulStop: '15s',
    preAllocatedVUs: Number(__ENV.CAP_PRE_VUS), maxVUs: Number(__ENV.CAP_MAX_VUS),
  } },
  systemTags: ['status', 'method', 'scenario', 'expected_response'],
  summaryTrendStats: ['min', 'avg', 'p(50)', 'p(75)', 'p(90)', 'p(95)', 'p(99)', 'max'],
  thresholds: { measured_requests: ['count>0'] },
}

export default function () {
  const elapsed = (Date.now() - exec.scenario.startTime) / 1000
  const phase = elapsed < warmup ? 'warmup' : (elapsed < warmup + duration ? 'measure' : 'drain')
  const i = exec.scenario.iterationInTest
  if (__ITER === 0) console.log(`SCENARIO_START ${exec.scenario.startTime}`)
  const n = i % 100
  const account = data.accounts[Math.floor(i / 7) % data.accounts.length]
  const track = data.tracks[Math.floor(i / 3) % data.tracks.length]
  let step, method = 'GET', path, body = null, expected = [200], token = null
  if (workload === 'catalog' || n < 30) {
    step = 'feed'; path = '/tracks?limit=20'
  } else if (n < 45) {
    step = 'card'; path = `/tracks/${track}`
  } else if (n < 60) {
    step = 'login'; method = 'POST'; path = '/sessions'
    body = { email: account.email, password: 'password1' }; expected = [201]
  } else if (n < 75) {
    step = 'like'; method = 'PUT'; path = `/users/${account.id}/likes/${track}`
    token = account.token; expected = [204]
  } else if (n < 85) {
    step = 'create'; method = 'POST'; path = '/tracks'
    token = account.token; expected = [201]
    body = { title: `Capacity ${i}`, artist: 'Capacity', size: data.track_bytes }
  } else if (n < 90) {
    step = 'playlist'; method = 'POST'; path = '/playlists'
    token = account.token; expected = [201]
    body = { title: `Capacity ${i}`, is_public: true }
  } else {
    step = 'stream-body'; path = data.stream_urls[i % data.stream_urls.length]
  }
  const headers = {}
  if (body) headers['Content-Type'] = 'application/json'
  if (token) headers.Authorization = `Bearer ${token}`
  const params = { headers, tags: { step, phase }, redirects: 0,
    timeout: '5s', responseCallback: http.expectedStatuses(...expected) }
  if (phase === 'measure') started.add(1, { phase })
  const res = http.request(method, path.startsWith('http') ? path : api + path,
    body ? JSON.stringify(body) : null, params)
  if (phase === 'measure') {
    measured.add(1); ms.add(res.timings.duration)
    failures.add(!expected.includes(res.status), { step })
    if (step === 'stream-body' && res.status === 200) streamBytes.add(res.body.length, { phase })
  }
}
function values(summary, name) { return summary.metrics[name]?.values || null }
export function handleSummary(summary) {
  const body = { scenario: 'capacity', profile: __ENV.PROFILE, mode: 'open_http_requests',
    target_rps: rate, workload, measurement_sec: duration, warmup_sec: warmup, phases: [],
    test_run_ms: summary.state.testRunDurationMs, http_req_duration: values(summary, 'measured_ms'),
    started_requests: values(summary, 'started_requests'), measured_requests: values(summary, 'measured_requests'),
    dropped_iterations: values(summary, 'dropped_iterations'), unexpected_fail: values(summary, 'unexpected_fail') }
  return { [`${__ENV.SUMMARY_DIR}/summary.json`]: JSON.stringify(body, null, 2) }
}
