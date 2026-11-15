// Distribution of requests over the three pods of Gateway and Catalog, and the
// lag of a read replica: a user created on the primary can be missing from the
// first read and present after a short pause.
import http from 'k6/http'
import { check, sleep } from 'k6'
import { Counter } from 'k6/metrics'

const base = __ENV.BASE_URL || 'http://127.0.0.1:5173'

const gatewayHits = new Counter('gateway_pod')
const catalogHits = new Counter('catalog_pod')

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
  },
}

function catalogOf(res) {
  const header = res.headers['X-Chimera-Upstreams'] || ''
  const names = header.split(',').map((part) => part.trim()).filter((part) => part.startsWith('catalog'))
  return names[0] || ''
}

export function reads() {
  const res = http.get(`${base}/api/v1/tracks`)
  check(res, { 'get 200': (r) => r.status === 200 }, { kind: 'get' })
  const gateway = res.headers['X-Chimera-Instance'] || ''
  const catalog = catalogOf(res)
  if (gateway) {
    gatewayHits.add(1, { pod: gateway })
  }
  if (catalog) {
    catalogHits.add(1, { pod: catalog })
  }
  sleep(0.2)
}

function register(email) {
  return http.post(
    `${base}/api/v1/users`,
    JSON.stringify({ email, name: 'k6', password: 'password1' }),
    { headers: { 'Content-Type': 'application/json' } },
  )
}

export function writes() {
  const res = register(`k6-${__ITER}-${Date.now()}@example.com`)
  check(res, { 'post 201': (r) => r.status === 201 }, { kind: 'post' })
  sleep(0.2)
}

export function lag() {
  const created = register(`k6-lag-${Date.now()}@example.com`)
  const id = created.json('id')
  const immediate = http.get(`${base}/api/v1/users/${id}`, {
    responseCallback: http.expectedStatuses(200, 404),
  })
  sleep(2)
  const delayed = http.get(`${base}/api/v1/users/${id}`)
  console.log(`lag post=${created.status} immediate=${immediate.status} delayed=${delayed.status} id=${id}`)
}

function byPod(data, prefix) {
  const pods = {}
  for (const key of Object.keys(data.metrics)) {
    if (!key.startsWith(prefix + '{')) {
      continue
    }
    const name = key.slice(prefix.length + 1, -1).replace(/^pod:/, '')
    pods[name] = data.metrics[key].values.count || 0
  }
  return pods
}

export function handleSummary(data) {
  const duration = data.metrics.http_req_duration.values
  const body = {
    gateway_pods: byPod(data, 'gateway_pod'),
    catalog_pods: byPod(data, 'catalog_pod'),
    http_reqs: data.metrics.http_reqs.values.count,
    http_req_failed: data.metrics.http_req_failed.values.rate,
    http_req_duration_p95_ms: duration['p(95)'],
    http_req_duration_avg_ms: duration.avg,
  }
  const text = JSON.stringify(body, null, 2)
  const out = __ENV.SUMMARY_DIR ? `${__ENV.SUMMARY_DIR}/summary-balance.json` : 'summary-balance.json'
  return { stdout: `${text}\n`, [out]: text }
}
