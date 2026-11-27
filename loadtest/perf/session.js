// Open arrival-rate model. k6 starts iterations at a fixed rate, so the offered
// HTTP rate stays flat when responses slow down. Think-time pauses are not
// used: identical pauses lock every user to the same phase and draw a saw.
// Roles stay 60% listeners, 25% subscribers, 15% authors.
import http from 'k6/http'
import exec from 'k6/execution'
import { Counter, Trend, Rate } from 'k6/metrics'

const base = (__ENV.BASE_URL || 'http://chimera.192.168.49.2.nip.io').replace(/\/$/, '')
const api = `${base}/api/v1`
const profile = __ENV.PROFILE || 'quick'
const scenario = __ENV.SCENARIO || 'ramp'
const faultRate = Number(__ENV.FAULT_RATE || '0')
const warmupMs = (Number(__ENV.WARMUP_SEC || (profile === 'quick' ? '10' : '30'))) * 1000
const stable = Number(__ENV.STABLE_RPS || '50')
// Happy-path HTTP calls in one measured iteration, without login.
// Login is once per seed user in setup: repeating bcrypt on every iteration
// filled the node until requests and kubectl both timed out.
// k6 arrival-rate executors accept only integer rates, so each role gets its
// own time unit. An integer iteration count per that unit equals
// targetRps * share / calls per second.
const callsPerIteration = { listeners: 6, subscribers: 7, authors: 9 }
const shareNum = { listeners: 3, subscribers: 1, authors: 3 }
const shareDen = { listeners: 5, subscribers: 4, authors: 20 }
const password = 'password1'
const trackBytes = Number(__ENV.TRACK_BYTES || '65536')
const missingID = '00000000-0000-0000-0000-000000000000'

const sessions = new Counter('sessions')
const unexpected = new Rate('unexpected_fail')
const measuredMs = new Trend('measured_ms', true)
const measuredRequests = new Counter('measured_requests')
const expectedFault = new Counter('expected_fault')
const streamBytes = new Counter('stream_bytes')
const latencyBucket = new Counter('latency_bucket')
const stepMs = new Trend('step_ms', true)

const buckets = [5, 10, 20, 50, 100, 200, 400, 800, 1500, 3000, 5000]

function rampRates() {
  const raw = (__ENV.RAMP_RPS || '').trim()
  const levels = raw
    ? raw.split(',').map((item) => Number(item.trim())).filter((item) => item > 0)
    : [25, 50, 100, 200, 350, 600]
  if (!levels.length) throw new Error('RAMP_RPS must list positive HTTP rates')
  return levels
}

function iterationsPerSecond(role, httpRps) {
  return httpRps * shareNum[role] / (shareDen[role] * callsPerIteration[role])
}

function formatTimeUnit(ms) {
  return ms % 1000 === 0 ? `${ms / 1000}s` : `${ms}ms`
}

// Smallest time unit that makes every requested HTTP rate an integer iteration count.
function pace(role, levels) {
  const denom = shareDen[role] * callsPerIteration[role] * 1000
  const factor = shareNum[role]
  const whole = levels.every((rps) => Number.isInteger(rps))
  for (let ms = 1; ms <= 180000; ms += 1) {
    const counts = []
    let exact = true
    for (let i = 0; i < levels.length; i += 1) {
      const rps = levels[i]
      if (whole) {
        const product = rps * factor * ms
        if (product % denom !== 0) {
          exact = false
          break
        }
        const count = product / denom
        if (count < 1) {
          exact = false
          break
        }
        counts.push(count)
      } else {
        const count = iterationsPerSecond(role, rps) * ms / 1000
        if (count < 1 || Math.abs(count - Math.round(count)) >= 1e-6) {
          exact = false
          break
        }
        counts.push(Math.round(count))
      }
    }
    if (exact) return { timeUnit: formatTimeUnit(ms), counts }
  }
  throw new Error(`cannot express ${role} rates ${levels.join(',')} as integer k6 arrival rates`)
}

function phases() {
  if (scenario === 'soak') return []
  const sec = Number(__ENV.RAMP_STAGE_SEC || (profile === 'quick' ? 20 : 100))
  if (scenario === 'stress') {
    const hold = profile === 'quick' ? [30, 30, 60] : [180, 180, 240]
    const raw = (__ENV.STRESS_RPS || '').trim()
    const levels = raw
      ? raw.split(',').map((item) => Number(item.trim())).filter((item) => item > 0)
      : [stable, Math.ceil(stable * 1.6), stable]
    if (!levels.length) throw new Error('STRESS_RPS must list positive HTTP rates')
    return levels.map((rps, i) => ({ sec: hold[Math.min(i, hold.length - 1)], rps }))
  }
  return rampRates().map((rps) => ({ sec, rps }))
}

function stagesFor(steps, counts) {
  const stages = []
  let previous = 0
  for (let i = 0; i < steps.length; i += 1) {
    const rate = counts[i]
    if (previous !== rate && steps[i].sec > 1) {
      stages.push({ duration: '1s', target: rate })
      stages.push({ duration: `${steps[i].sec - 1}s`, target: rate })
    } else {
      stages.push({ duration: `${steps[i].sec}s`, target: rate })
    }
    previous = rate
  }
  return stages
}

function vuPool(iterRate) {
  const pre = Math.min(150, Math.max(8, Math.ceil(iterRate * 2)))
  const max = Math.min(1500, Math.max(pre, Math.ceil(iterRate * 25)))
  return { preAllocatedVUs: pre, maxVUs: max }
}

function scenarioFor(role, exec) {
  const steps = phases()
  const levels = scenario === 'soak' ? [stable] : steps.map((step) => step.rps)
  const maxHttp = Math.max(stable, ...levels)
  const pool = vuPool(iterationsPerSecond(role, maxHttp))
  const paced = pace(role, levels)
  if (scenario === 'soak') {
    const duration = __ENV.SOAK_DURATION || (profile === 'quick' ? '60s' : '10m')
    return {
      executor: 'constant-arrival-rate',
      rate: paced.counts[0],
      timeUnit: paced.timeUnit,
      duration,
      gracefulStop: '10s',
      exec,
      ...pool,
    }
  }
  return {
    executor: 'ramping-arrival-rate',
    startRate: 0,
    timeUnit: paced.timeUnit,
    stages: stagesFor(steps, paced.counts),
    gracefulStop: '10s',
    exec,
    ...pool,
  }
}

export const options = {
  scenarios: {
    listeners: scenarioFor('listeners', 'listener'),
    subscribers: scenarioFor('subscribers', 'subscriber'),
    authors: scenarioFor('authors', 'author'),
  },
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

function loginQuiet(email) {
  const res = http.post(`${api}/sessions`, JSON.stringify({ email, password }), {
    headers: { 'Content-Type': 'application/json' },
    redirects: 0,
    timeout: '15s',
    tags: { step: 'setup-login' },
  })
  if (res.status !== 201) throw new Error(`setup login failed for ${email}: HTTP ${res.status || 'no response'}`)
  return res.json('token')
}

export function setup() {
  if (!dataset) throw new Error('DATASET_FILE is required; use make -C backend perf-run')
  for (const role of ['listeners', 'subscribers', 'authors']) {
    for (const account of dataset[role]) account.token = loginQuiet(account.email)
  }
  return dataset
}

function accountOf(list) {
  const account = pick(list)
  if (!account.token || !account.id) throw new Error('setup did not attach a session token')
  return account
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
  const account = accountOf(data.listeners)
  const trackID = pick(data.tracks)
  if (!mistaken('feed', account.token, account.id)) {
    call('feed', 'GET', '/tracks?limit=20', null, account.token, [200])
  }
  if (!mistaken('card', account.token, account.id)) {
    call('card', 'GET', `/tracks/${trackID}`, null, account.token, [200])
  }
  if (!mistaken('stream', account.token, account.id)) stream(trackID)
  if (!mistaken('like', account.token, account.id)) {
    call('like', 'PUT', `/users/${account.id}/likes/${trackID}`, null, account.token, [204])
  }
  if (!mistaken('likes', account.token, account.id)) {
    call('likes', 'GET', `/tracks?liked_by=${account.id}`, null, account.token, [200])
  }
  sessions.add(1, { phase: phase() })
}

export function subscriber(data) {
  markStart()
  const account = accountOf(data.subscribers)
  const author = pick(data.authors)
  const playlist = pick(data.playlists)
  if (!mistaken('users', account.token, account.id)) {
    call('users', 'GET', '/users?limit=20', null, account.token, [200])
  }
  if (!mistaken('profile', account.token, account.id)) {
    call('profile', 'GET', `/users/${author.id}`, null, account.token, [200])
    call('author-tracks', 'GET', `/tracks?user_id=${author.id}`, null, account.token, [200])
  }
  if (!mistaken('follow', account.token, account.id)) {
    call('follow', 'PUT', `/follows/${account.id}/${author.id}`, null, account.token, [204])
  }
  call('following', 'GET', `/follows?follower_id=${account.id}`, null, account.token, [200])
  call('playlists', 'GET', `/playlists?owner_id=${author.id}`, null, account.token, [200])
  call('playlist-tracks', 'GET', `/playlists/${playlist.id}/tracks`, null, account.token, [200])
  sessions.add(1, { phase: phase() })
}

export function author(data) {
  markStart()
  const account = accountOf(data.authors)
  call('mine', 'GET', `/tracks?user_id=${account.id}`, null, account.token, [200])
  let ownTrackID = null
  // Playlist entries require a ready track. Keep the seeded ready track as
  // fallback until both upload and publication of our new track succeed.
  let trackID = pick(data.tracks)
  if (!mistaken('create', account.token, account.id)) {
    const created = call('create', 'POST', '/tracks', {
      title: `Live ${__VU}-${__ITER}`,
      artist: 'Live',
      size: trackBytes,
    }, account.token, [201])
    if (created.status === 201) {
      ownTrackID = created.json('track.id')
      const uploadURL = created.json('upload_url')
      const file = call('upload', 'PUT', uploadURL, 'x'.repeat(trackBytes), null, [200, 204])
      if (file.status === 200 || file.status === 204) {
        const published = call('publish', 'PATCH', `/tracks/${ownTrackID}`, { status: 'ready' }, account.token, [200])
        if (published.status === 200) trackID = ownTrackID
      }
    }
  }
  call('card', 'GET', `/tracks/${trackID}`, null, account.token, [200])
  const playlist = call('playlist', 'POST', '/playlists', {
    title: `Live ${__VU}-${__ITER}`,
    is_public: true,
  }, account.token, [201])
  if (playlist.status === 201) {
    call('playlist-add', 'POST', `/playlists/${playlist.json('id')}/tracks`, {
      track_id: trackID,
      position: 0,
    }, account.token, [200, 201, 204])
  }
  if (playlist.status === 201) call('playlist-delete', 'DELETE', `/playlists/${playlist.json('id')}`, null, account.token, [204])
  if (ownTrackID) call('track-delete', 'DELETE', `/tracks/${ownTrackID}`, null, account.token, [204])
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
    stable_rps: stable,
    model: 'open-arrival-rate',
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
