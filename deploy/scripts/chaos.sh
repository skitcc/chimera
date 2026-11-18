#!/bin/sh
# Shows what happens when one dependency disappears.
#
# 1. k6 reads the feed while one Catalog pod is deleted. Retries land on the
#    other pods, so almost every request stays 200.
# 2. Catalog is scaled to zero. The feed answers 503 inside the client timeout,
#    while login still works. A user deleted during the outage leaves UserDeleted
#    in Kafka; the draft disappears after Catalog returns.
# 3. The read replicas are scaled to zero. Feed reads fall back to the primary
#    and stay 200.
#
# Prints one line per sample and restores both deployments before exiting.
set -eu

namespace=${K8S_NAMESPACE:-chimera}
host=${K8S_HOST:?K8S_HOST is required}
k6_image=${K6_IMAGE:-grafana/k6:0.57.0}
out=${ARTIFACTS_DIR:-/tmp}/chaos
api="http://${host}/api"
log="$out/chaos.log"

mkdir -p "$out"
: >"$log"
say() { printf '%s\n' "$*" | tee -a "$log"; }

kube() { $KUBECTL -n "$namespace" "$@"; }

code() {
  curl -sS -o /dev/null -w '%{http_code}' --max-time 8 "$@" || printf '000'
}

restore() {
  say "restore catalog=3 postgres-replica=2"
  kube scale deployment/catalog --replicas=3 >/dev/null || true
  kube scale statefulset/postgres-replica --replicas=2 >/dev/null || true
}
trap restore EXIT

email="chaos-$(date +%s)@example.com"
say "register $email"
reg=$(curl -sS --max-time 8 -X POST "$api/v1/users" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$email\",\"name\":\"Chaos\",\"password\":\"password1\"}")
user_id=$(printf '%s' "$reg" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
login=$(curl -sS --max-time 8 -X POST "$api/v1/sessions" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$email\",\"password\":\"password1\"}")
token=$(printf '%s' "$login" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
draft=$(curl -sS --max-time 8 -X POST "$api/v1/tracks" \
  -H "Authorization: Bearer $token" -H 'Content-Type: application/json' \
  -d '{"title":"Chaos draft","artist":"Chaos","size":1}')
draft_id=$(printf '%s' "$draft" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
say "user=$user_id draft=$draft_id"
if [ -z "$user_id" ] || [ -z "$token" ] || [ -z "$draft_id" ]; then
  say "setup failed: $reg $login $draft"
  exit 1
fi

say "phase pod-loss: k6 for 40s, one catalog pod deleted after 12s"
docker rm -f chimera-k6 >/dev/null 2>&1 || true
docker run --rm -d --name chimera-k6 --network host --user "$(id -u):$(id -g)" \
  -e BASE_URL="http://$host" -e DURATION=40s -e SUMMARY_DIR=/out \
  -v "$(cd "$(dirname "$0")/../.." && pwd)/loadtest:/loadtest:ro" \
  -v "$out:/out" \
  "$k6_image" run /loadtest/resilience.js >/dev/null
sleep 12
pod=$(kube get pods -l app.kubernetes.io/name=catalog -o jsonpath='{.items[0].metadata.name}')
say "delete $pod"
kube delete pod "$pod" --wait=false >/dev/null
docker wait chimera-k6 >/dev/null || say "k6 exit $?"
say "k6 summary:"
cat "$out/summary-resilience.json" | tee -a "$log"

say "phase catalog-down"
kube scale deployment/catalog --replicas=0 >/dev/null
kube rollout status deployment/catalog --timeout=60s >/dev/null
feed=$(code "$api/v1/tracks")
again=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 8 -X POST "$api/v1/sessions" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$email\",\"password\":\"password1\"}" || printf '000')
deleted=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 8 -X DELETE "$api/v1/users/$user_id" \
  -H "Authorization: Bearer $token" || printf '000')
say "feed=$feed login=$again delete=$deleted"

say "phase catalog-back"
kube scale deployment/catalog --replicas=3 >/dev/null
kube rollout status deployment/catalog --timeout=180s
draft_status=000
i=0
while [ "$i" -lt 30 ]; do
  draft_status=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 8 \
    -H "Authorization: Bearer $token" "$api/v1/tracks/$draft_id" || printf '000')
  [ "$draft_status" = "404" ] && break
  i=$((i + 1))
  sleep 2
done
say "draft_after_event=$draft_status"

say "phase replica-down"
kube scale statefulset/postgres-replica --replicas=0 >/dev/null
kube rollout status statefulset/postgres-replica --timeout=120s >/dev/null
sleep 12
replica_feed=$(code "$api/v1/tracks")
say "feed_without_replicas=$replica_feed"
