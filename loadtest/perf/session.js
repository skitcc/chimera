// One virtual user is one person for the whole iteration: sign in, do the
// work of that role, wait. Roles stay at 60% listeners, 25% subscribers and
// 15% authors. Pauses are fixed so two runs at the same VU count offer the
// same load.
import http from 'k6/http'
import exec from 'k6/execution'
import { sleep, check } from 'k6'
import { Counter, Trend, Rate } from 'k6/metrics'

const base = (__ENV.BASE_URL || 'http://chimera.192.168.49.2.nip.io').replace(/\/$/, '')
const api = `${base}/api/v1`
const profile = __ENV.PROFILE || 'quick'
const scenario = __ENV.SCENARIO || 'ramp'
const faultRate = Number(__ENV.FAULT_RATE || '0')
const warmupMs = (Number(__ENV.WARMUP_SEC || (profile === 'quick' ? '10' : '30'))) * 1000
const stable = Number(__ENV.STABLE_VUS || '10')
const password = 'password1'
const trackBytes = Number(__ENV.TRACK_BYTES || '65536')
const missingID = '00000000-0000-0000-0000-000000000000'

const sessions = new Counter('sessions')
const unexpected = new Rate('unexpected_fail')
const measuredMs = new Trend('measured_ms', true)
const measuredRequests = new Counter('measured_requests')
const failedSessions = new Counter('failed_sessions')
const expectedFault = new Counter('expected_fault')
const streamBytes = new Counter('stream_bytes')
const latencyBucket = new Counter('latency_bucket')
const stepMs = new Trend('step_ms', true)

const buckets = [5, 10, 20, 50, 100, 200, 400, 800, 1500, 3000, 5000]

function split(total) {
  let listeners = Math.floor(total * 0.6)
  let subscribers = Math.floor(total * 0.25)
  let authors = total - listeners - subscribers
  if (listeners < 1) listeners = 1
  if (subscribers < 1) subscribers = 1
  if (authors < 1) authors = 1
  while (listeners + subscribers + authors > total && total >= 3) {
    if (listeners >= subscribers && listeners > 1) listeners -= 1
    else if (subscribers > 1) subscribers -= 1
    else if (authors > 1) authors -= 1
    else break
  }
  return { listeners, subscribers, authors }
}

function phases() {
  if (scenario === 'soak') return []
  if (scenario === 'stress') {
    const hold = profile === 'quick' ? [30, 30, 60] : [180, 180, 240]
    const levels = [stable, Math.ceil(stable * 1.6), stable]
    return hold.map((sec, i) => ({ sec, vus: levels[i] }))
  }

  const levels = [100, 200, 300, 400, 500, 650]
  const sec = Number(__ENV.RAMP_STAGE_SEC || (profile === 'quick' ? 20 : 100))
  return levels.map((vus) => ({ sec, vus }))
}

function stagesFor(role) {
  const stages = []
  for (const phase of phases()) {
    const n = split(phase.vus)[role]
    stages.push({ duration: '1s', target: n })
    stages.push({ duration: `${Math.max(phase.sec - 1, 1)}s`, target: n })
  }
  return stages
}

function soakOptions() {
  const n = split(stable)
  const duration = __ENV.SOAK_DURATION || (profile === 'quick' ? '60s' : '10m')
  return {
    listeners: { executor: 'constant-vus', vus: n.listeners, duration, exec: 'listener' },
    subscribers: { executor: 'constant-vus', vus: n.subscribers, duration, exec: 'subscriber' },
    authors: { executor: 'constant-vus', vus: n.authors, duration, exec: 'author' },
  }
}

function rampOptions() {
  return {
    listeners: { executor: 'ramping-vus', startVUs: 0, stages: stagesFor('listeners'), gracefulRampDown: '0s', exec: 'listener' },
    subscribers: { executor: 'ramping-vus', startVUs: 0, stages: stagesFor('subscribers'), gracefulRampDown: '0s', exec: 'subscriber' },
    authors: { executor: 'ramping-vus', startVUs: 0, stages: stagesFor('authors'), gracefulRampDown: '0s', exec: 'author' },
  }
}

export const options = {
  scenarios: scenario === 'soak' ? soakOptions() : rampOptions(),
  summaryTrendStats: ['min', 'avg', 'med', 'max', 'p(50)', 'p(75)', 'p(90)', 'p(95)', 'p(99)'],
  noConnectionReuse: false,
  systemTags: ['status', 'method', 'scenario', 'expected_response'],
  setupTimeout: '5m',
  thresholds: {
    measured_requests: ['count>0'],
    sessions: ['count>0'],
    ...(scenario === 'soak' ? { unexpected_fail: ['rate<0.01'] } : {}),
  },
}

function phase() {
  const elapsed = Date.now() - exec.scenario.startTime
  return elapsed < warmupMs ? 'warmup' : 'measure'
}

function observe(step, res, now) {
  const ms = res.timings.duration
  stepMs.add(ms, { step, phase: now })
  if (now === 'measure') { measuredMs.add(ms); measuredRequests.add(1) }
  for (let i = 0; i < buckets.length; i += 1) {
    if (ms <= buckets[i]) {
      latencyBucket.add(1, { le: String(buckets[i]), step, phase: now })
    }
  }
  latencyBucket.add(1, { le: '+Inf', step, phase: now })
}

function call(step, method, path, body, token, expected) {
  const headers = {}
  if (body !== null && body !== undefined && typeof body !== 'string') headers['Content-Type'] = 'application/json'
  if (typeof body === 'string') headers['Content-Type'] = 'application/octet-stream'
  if (token) headers.Authorization = `Bearer ${token}`
  const requestPhase = phase()
  const params = {
    headers,
    redirects: 0,
    timeout: '5s',
    tags: { step, phase: requestPhase },
    responseCallback: http.expectedStatuses(...expected),
  }
  const url = path.startsWith('http') ? path : `${api}${path}`
  const payload = body === null || body === undefined ? null : (typeof body === 'string' ? body : JSON.stringify(body))
  let res
  if (method === 'GET') res = http.get(url, params)
  else if (method === 'POST') res = http.post(url, payload, params)
  else if (method === 'PUT') res = http.put(url, payload, params)
  else if (method === 'DELETE') res = http.del(url, payload, params)
  else if (method === 'PATCH') res = http.patch(url, payload, params)
  else throw new Error(`unsupported method ${method}`)
  observe(step, res, requestPhase)
  const ok = expected.indexOf(res.status) !== -1
  if (requestPhase === 'measure') unexpected.add(!ok, { step })
  return res
}

// A wrong call stands in for the real step, except sign-in: a bad password is
// attempted and the real sign-in still happens, otherwise the session ends.
function mistaken(step, token, me) {
  if (!(faultRate > 0) || Math.random() >= faultRate) return false
  if (step === 'login') {
    call('fault', 'POST', '/sessions', { email: 'nobody-perf@example.com', password: 'not-the-password' }, null, [400, 401])
    expectedFault.add(1, { step })
    return false
  }
  if (step === 'feed' || step === 'card' || step === 'stream' || step === 'likes') {
    call('fault', 'GET', `/tracks/${missingID}`, null, token, [404])
  } else if (step === 'like') {
    call('fault', 'PUT', `/users/${me}/likes/${missingID}`, null, null, [401])
  } else if (step === 'users' || step === 'profile') {
    call('fault', 'GET', `/users/${missingID}`, null, token, [404])
  } else if (step === 'follow') {
    call('fault', 'PUT', `/follows/${me}/${me}`, null, token, [400])
  } else if (step === 'create') {
    call('fault', 'POST', '/tracks', { title: '', artist: 'nobody', size: trackBytes }, token, [400])
  } else {
    return false
  }
  expectedFault.add(1, { step })
  return true
}

function login(email) {
  if (mistaken('login', null, null)) return { token: '', id: '' }
  const res = call('login', 'POST', '/sessions', { email, password }, null, [201])
  check(res, { 'login 201': (r) => r.status === 201 })
  if (res.status !== 201) return { token: '', id: '' }
  return { token: res.json('token'), id: res.json('user_id') }
}

function pick(list) {
  return list[(__VU - 1) % list.length]
}

function markStart() {
  if (__ITER !== 0) return
  console.log(`SCENARIO_START ${exec.scenario.startTime}`)
}

// Seed data is prepared once by runner.py, outside measurement and reused by
// every scenario. The load generator never deploys or resets infrastructure.
const dataset = __ENV.DATASET_FILE ? JSON.parse(open(__ENV.DATASET_FILE)) : null

export function setup() {
  if (!dataset) throw new Error('DATASET_FILE is required; use make -C backend perf-run')
  return dataset
}

function stream(trackID) {
  const res = call('stream', 'GET', `/tracks/${trackID}/stream`, null, null, [302])
  if (res.status !== 302) return
  const loc = res.headers.Location || res.headers.location
  if (!loc) return
  const file = call('stream-body', 'GET', loc, null, null, [200])
  if (file.status === 200 && file.body) streamBytes.add(file.body.length, { phase: phase() })
}

export function listener(data) {
  markStart()
  const account = pick(data.listeners)
  const trackID = pick(data.tracks)
  const session = login(account.email)
  sleep(2)
  if (!session.token) {
    failedSessions.add(1, { phase: phase() })
    return
  }
  if (!mistaken('feed', session.token, session.id)) {
    call('feed', 'GET', '/tracks?limit=20', null, session.token, [200])
  }
  sleep(3)
  if (!mistaken('card', session.token, session.id)) {
    call('card', 'GET', `/tracks/${trackID}`, null, session.token, [200])
  }
  sleep(1)
  if (!mistaken('stream', session.token, session.id)) stream(trackID)
  sleep(4)
  if (!mistaken('like', session.token, session.id)) {
    call('like', 'PUT', `/users/${session.id}/likes/${trackID}`, null, session.token, [204])
  }
  sleep(1)
  if (!mistaken('likes', session.token, session.id)) {
    call('likes', 'GET', `/tracks?liked_by=${session.id}`, null, session.token, [200])
  }
  sleep(3)
  sessions.add(1, { phase: phase() })
}

export function subscriber(data) {
  markStart()
  const account = pick(data.subscribers)
  const author = pick(data.authors)
  const playlist = pick(data.playlists)
  const session = login(account.email)
  sleep(2)
  if (!session.token) {
    failedSessions.add(1, { phase: phase() })
    return
  }
  if (!mistaken('users', session.token, session.id)) {
    call('users', 'GET', '/users?limit=20', null, session.token, [200])
  }
  sleep(2)
  if (!mistaken('profile', session.token, session.id)) {
    call('profile', 'GET', `/users/${author.id}`, null, session.token, [200])
    call('author-tracks', 'GET', `/tracks?user_id=${author.id}`, null, session.token, [200])
  }
  sleep(3)
  if (!mistaken('follow', session.token, session.id)) {
    call('follow', 'PUT', `/follows/${session.id}/${author.id}`, null, session.token, [204])
  }
  sleep(1)
  call('following', 'GET', `/follows?follower_id=${session.id}`, null, session.token, [200])
  call('playlists', 'GET', `/playlists?owner_id=${author.id}`, null, session.token, [200])
  call('playlist-tracks', 'GET', `/playlists/${playlist.id}/tracks`, null, session.token, [200])
  sleep(3)
  sessions.add(1, { phase: phase() })
}

export function author(data) {
  markStart()
  const account = pick(data.authors)
  const session = login(account.email)
  sleep(2)
  if (!session.token) {
    failedSessions.add(1, { phase: phase() })
    return
  }
  call('mine', 'GET', `/tracks?user_id=${session.id}`, null, session.token, [200])
  sleep(2)
  let ownTrackID = null
  // Playlist entries require a ready track. Keep the seeded ready track as
  // fallback until both upload and publication of our new track succeed.
  let trackID = pick(data.tracks)
  if (!mistaken('create', session.token, session.id)) {
    const created = call('create', 'POST', '/tracks', {
      title: `Live ${__VU}-${__ITER}`,
      artist: 'Live',
      size: trackBytes,
    }, session.token, [201])
    if (created.status === 201) {
      ownTrackID = created.json('track.id')
      const uploadURL = created.json('upload_url')
      const file = call('upload', 'PUT', uploadURL, 'x'.repeat(trackBytes), null, [200, 204])
      if (file.status === 200 || file.status === 204) {
        const published = call('publish', 'PATCH', `/tracks/${ownTrackID}`, { status: 'ready' }, session.token, [200])
        if (published.status === 200) trackID = ownTrackID
      }
    }
  }
  sleep(2)
  call('card', 'GET', `/tracks/${trackID}`, null, session.token, [200])
  const playlist = call('playlist', 'POST', '/playlists', {
    title: `Live ${__VU}-${__ITER}`,
    is_public: true,
  }, session.token, [201])
  if (playlist.status === 201) {
    call('playlist-add', 'POST', `/playlists/${playlist.json('id')}/tracks`, {
      track_id: trackID,
      position: 0,
    }, session.token, [200, 201, 204])
  }
  sleep(3)
  if (playlist.status === 201) call('playlist-delete', 'DELETE', `/playlists/${playlist.json('id')}`, null, session.token, [204])
  if (ownTrackID) call('track-delete', 'DELETE', `/tracks/${ownTrackID}`, null, session.token, [204])
  sessions.add(1, { phase: phase() })
}

function values(data, name) {
  const metric = data.metrics[name]
  if (!metric || !metric.values) return null
  return metric.values
}

export function handleSummary(data) {
  const duration = values(data, 'measured_ms')
  const body = {
    profile,
    scenario,
    fault_rate: faultRate,
    stable_vus: stable,
    warmup_sec: warmupMs / 1000,
    test_run_ms: data.state && data.state.testRunDurationMs,
    http_req_duration: duration,
    http_reqs: values(data, 'http_reqs'),
    sessions: values(data, 'sessions'),
    unexpected_fail: values(data, 'unexpected_fail'),
    expected_fault: values(data, 'expected_fault'),
    failed_sessions: values(data, 'failed_sessions'),
    measured_requests: values(data, 'measured_requests'),
    track_bytes: trackBytes,
    stream_bytes: values(data, 'stream_bytes'),
    phases: phases(),
  }
  const text = JSON.stringify(body, null, 2)
  const dir = __ENV.SUMMARY_DIR || '.'
  return { stdout: `${text}\n`, [`${dir}/summary.json`]: text }
}
