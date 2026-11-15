// Steady read load while chaos.sh removes a Catalog pod. Failures here are the
// requests that did not survive a retry onto another pod. The later phases
// (Catalog scaled to zero, replica stopped) are measured by chaos.sh itself,
// after this script has finished, so those 503s do not mix into this rate.
import http from 'k6/http'
import { check, sleep } from 'k6'
import { Counter } from 'k6/metrics'

const base = __ENV.BASE_URL || 'http://127.0.0.1:5173'

const status200 = new Counter('status_200')
const status5xx = new Counter('status_5xx')
const statusOther = new Counter('status_other')

export const options = {
  scenarios: {
    feed: {
      executor: 'constant-vus',
      vus: 10,
      duration: __ENV.DURATION || '40s',
      exec: 'feed',
    },
  },
  thresholds: {
    status_5xx: ['count<5'],
    http_req_duration: ['p(95)<500'],
  },
}

export function feed() {
  const res = http.get(`${base}/api/v1/tracks`, { timeout: '5s' })
  check(res, { 'feed 200': (r) => r.status === 200 })
  if (res.status === 200) {
    status200.add(1)
  } else if (res.status >= 500 || res.status === 0) {
    status5xx.add(1)
  } else {
    statusOther.add(1)
  }
  sleep(0.2)
}

function count(data, name) {
  const metric = data.metrics[name]
  if (!metric || !metric.values) {
    return 0
  }
  return metric.values.count || 0
}

export function handleSummary(data) {
  const duration = data.metrics.http_req_duration.values
  const ok = count(data, 'status_200')
  const failed = count(data, 'status_5xx')
  const body = {
    ok,
    failed,
    other: count(data, 'status_other'),
    failed_share: ok + failed ? failed / (ok + failed) : 0,
    http_req_duration_p95_ms: duration['p(95)'],
    http_req_duration_avg_ms: duration.avg,
  }
  const text = JSON.stringify(body, null, 2)
  const out = __ENV.SUMMARY_DIR ? `${__ENV.SUMMARY_DIR}/summary-resilience.json` : 'summary-resilience.json'
  return { stdout: `${text}\n`, [out]: text }
}
