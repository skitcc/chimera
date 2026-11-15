import http from 'k6/http'
import { check, sleep } from 'k6'
import { Counter } from 'k6/metrics'

const base = __ENV.BASE_URL || 'http://127.0.0.1:5173'
const read = __ENV.READ_URL || 'http://127.0.0.1:8081'

const getsApi = new Counter('gets_api')
const getsRead1 = new Counter('gets_read1')
const getsRead2 = new Counter('gets_read2')
const getsOther = new Counter('gets_other')
const postsNotPrimary = new Counter('posts_not_primary')

export const options = {
  scenarios: {
    reads: {
      executor: 'constant-vus',
      vus: 15,
      duration: '30s',
      exec: 'reads',
    },
    writes: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 4,
      maxDuration: '30s',
      exec: 'writes',
    },
    lag: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 1,
      startTime: '32s',
      maxDuration: '20s',
      exec: 'lag',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<500'],
    'checks{kind:get}': ['rate>0.99'],
    'checks{kind:post}': ['rate>0.99'],
    posts_not_primary: ['count==0'],
  },
}

function instanceOf(res) {
  return res.headers['X-Chimera-Instance'] || ''
}

function countGet(res) {
  const name = instanceOf(res)
  if (name === 'api') {
    getsApi.add(1)
    return
  }
  if (name === 'api-read-1') {
    getsRead1.add(1)
    return
  }
  if (name === 'api-read-2') {
    getsRead2.add(1)
    return
  }
  getsOther.add(1)
}

function register(path, email) {
  return http.post(
    `${base}${path}`,
    JSON.stringify({ email, name: 'k6', password: 'password1' }),
    { headers: { 'Content-Type': 'application/json' } },
  )
}

export function reads() {
  const res = http.get(`${base}/api/v1/tracks`)
  check(res, { 'get 200': (r) => r.status === 200 }, { kind: 'get' })
  countGet(res)
  sleep(0.2)
}

export function writes() {
  const path = __ITER % 2 === 0 ? '/api/v1/users' : '/mirror/api/v1/users'
  const res = register(path, `k6-${__ITER}-${Date.now()}@example.com`)
  check(
    res,
    {
      'post 201': (r) => r.status === 201,
      'post primary': (r) => instanceOf(r) === 'api',
    },
    { kind: 'post' },
  )
  postsNotPrimary.add(instanceOf(res) === 'api' ? 0 : 1)
  sleep(0.2)
}

export function lag() {
  const created = register('/api/v1/users', `k6-lag-${Date.now()}@example.com`)
  const id = created.json('id')
  const immediate = http.get(`${read}/v1/users/${id}`)
  sleep(2)
  const delayed = http.get(`${read}/v1/users/${id}`)
  console.log(`lag post=${created.status} immediate=${immediate.status} delayed=${delayed.status} id=${id}`)
}

function count(data, name) {
  const metric = data.metrics[name]
  if (!metric || !metric.values) {
    return 0
  }
  return metric.values.count || 0
}

export function handleSummary(data) {
  const api = count(data, 'gets_api')
  const read1 = count(data, 'gets_read1')
  const read2 = count(data, 'gets_read2')
  const other = count(data, 'gets_other')
  const total = api + read1 + read2 + other
  const duration = data.metrics.http_req_duration.values
  const body = {
    gets: { api, read1, read2, other, total, primary_share: total ? api / total : 0 },
    http_reqs: data.metrics.http_reqs.values.count,
    http_req_failed: data.metrics.http_req_failed.values.rate,
    http_req_duration_p95_ms: duration['p(95)'],
    http_req_duration_avg_ms: duration.avg,
    posts_not_primary: count(data, 'posts_not_primary'),
  }
  const text = JSON.stringify(body, null, 2)
  return {
    stdout: `${text}\n`,
    '/loadtest/summary-balance.json': text,
  }
}
