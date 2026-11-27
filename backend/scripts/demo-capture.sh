#!/bin/sh
# Brings up the system, replays the demo with curl, and writes traffic captures.
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
backend=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
out=$backend/e2e-out
image=${NETSHOOT_IMAGE:-nicolaka/netshoot:v0.13}
dump=chimera-e2e-dump

if [ ! -f "$root/.env" ]; then
	echo "missing $root/.env; copy .env.example to .env" >&2
	exit 1
fi

set -a
# shellcheck disable=SC1091
. "$root/.env"
set +a
. "$backend/scripts/e2e-secrets.sh"
sh "$backend/scripts/check-e2e-credentials.sh"

api_port=${API_PORT:-8080}
project=${PROJECT_NAME:-chimera}
compose="docker compose --project-directory $root --project-name $project -f $root/docker-compose.yml"

if [ -d "$out" ]; then
	docker run --rm -v "$backend:/work" "$image" rm -rf /work/e2e-out
fi
mkdir -p "$out"

cleanup() {
	docker stop "$dump" >/dev/null 2>&1 || true
	docker rm "$dump" >/dev/null 2>&1 || true
	if [ "${E2E_KEEP_STACK:-}" != 1 ]; then
		$compose down --volumes --remove-orphans >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT INT TERM

# gRPC compresses headers with per-connection HPACK state and keeps connections open,
# so tshark can name the methods only if it sees each connection from its first byte.
# The dump therefore listens on the compose bridge from the host and starts before
# any service: it sees client HTTP, the Gateway's gRPC fan-out, SQL of all three
# services, Kafka, and the direct upload to RustFS.
$compose build
$compose create
network=$(docker network inspect --format '{{.Id}}' "${project}_default" | cut -c1-12)
docker run -d --name "$dump" --network host \
	--cap-add NET_RAW --cap-add NET_ADMIN \
	-v "$out:/out" "$image" \
	tcpdump -i "br-$network" -n -U -w /out/demo.pcap \
	'tcp port 8080 or tcp port 9090 or tcp port 5432 or tcp port 9000 or tcp port 9092' >/dev/null

i=0
until docker top "$dump" 2>/dev/null | grep -q tcpdump; do
	i=$((i + 1))
	if [ "$i" -ge 50 ]; then
		echo "$dump: tcpdump is not running" >&2
		docker logs "$dump" >&2 || true
		exit 1
	fi
	sleep 0.1
done

$compose up --detach --wait --wait-timeout 300

docker run --rm --network host \
	-e GITHUB_ACTIONS -e E2E_USER_PASSWORD -e E2E_NEW_PASSWORD \
	-e E2E_MAILPIT_URL="http://127.0.0.1:${MAILPIT_PORT:-8025}" \
	-e E2E_API_URL="http://127.0.0.1:${api_port}" \
	-e E2E_OUT=/out \
	-v "$out:/out" \
	-v "$backend/scripts/demo-requests.sh:/demo-requests.sh:ro" \
	"$image" sh /demo-requests.sh

# Let the outbox relays and consumers finish the delete cascade before stopping the dump.
sleep 3
docker stop "$dump" >/dev/null
printf '%s\n' "--- $dump ---"
docker logs "$dump" 2>&1 || true

docker run --rm -v "$out:/out" "$image" \
	tshark -r /out/demo.pcap \
	-d tcp.port==8080,http -d tcp.port==9000,http \
	-Y http -T fields -E header=y -E separator='|' \
	-e frame.number -e ip.src -e tcp.dstport \
	-e http.request.method -e http.request.uri -e http.response.code \
	> "$out/http.txt"

docker run --rm -v "$out:/out" "$image" \
	tshark -r /out/demo.pcap \
	-d tcp.port==9000,http -d tcp.port==9090,http2 \
	-Y 'http or pgsql or kafka or (grpc and http2.headers.path and !(http2.headers.path contains "grpc.health"))' \
	> "$out/demo.txt"
printf '\n# gRPC\n' >> "$out/demo.txt"
docker run --rm -v "$out:/out" "$image" \
	tshark -r /out/demo.pcap -d tcp.port==9090,http2 \
	-Y 'http2.headers.path and !(http2.headers.path contains "grpc.health")' \
	-T fields -e ip.src -e ip.dst -e http2.headers.path \
	>> "$out/demo.txt"
printf '\n# SQL\n' >> "$out/demo.txt"
docker run --rm -v "$out:/out" "$image" \
	tshark -r /out/demo.pcap \
	-Y 'pgsql.query' -T fields -e pgsql.query \
	>> "$out/demo.txt"

docker run --rm -v "$out:/out" "$image" \
	sh -c "chown -R $(id -u):$(id -g) /out"

printf '\n--- gRPC ---\n'
awk '/^# SQL$/ { exit } seen { print } /^# gRPC$/ { seen=1 }' "$out/demo.txt"
printf '\n--- SQL ---\n'
awk 'seen { print } /^# SQL$/ { seen=1 }' "$out/demo.txt"
if [ "${GITHUB_ACTIONS:-}" = true ]; then
 python3 "$backend/scripts/sanitize-auth-artifacts.py" "$out"
fi
printf 'capture written to %s\n' "$out"
