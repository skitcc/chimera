#!/bin/sh
# Brings up the system, replays the demo with curl, and writes traffic captures.
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
backend=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
out=$backend/e2e-out
compose="docker compose --project-directory $root -f $root/docker-compose.yml"
image=${NETSHOOT_IMAGE:-nicolaka/netshoot:v0.13}

if [ ! -f "$root/.env" ]; then
	echo "missing $root/.env; copy .env.example to .env" >&2
	exit 1
fi

set -a
# shellcheck disable=SC1091
. "$root/.env"
set +a

api_port=${API_PORT:-8080}
minio_port=${MINIO_API_PORT:-9000}
project=${PROJECT_NAME:-chimera}

rm -rf "$out"
mkdir -p "$out"

cleanup() {
	docker stop chimera-e2e-dump-client chimera-e2e-dump-api >/dev/null 2>&1 || true
	docker rm chimera-e2e-dump-client chimera-e2e-dump-api >/dev/null 2>&1 || true
	if [ "${E2E_KEEP_STACK:-}" != 1 ]; then
		$compose down --volumes --remove-orphans >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT INT TERM

$compose up --build --detach --wait --wait-timeout 300

docker run -d --name chimera-e2e-dump-client --network host \
	--cap-add NET_RAW --cap-add NET_ADMIN \
	-v "$out:/out" "$image" \
	tcpdump -i lo -U -w /out/client.pcap "tcp port ${api_port} or tcp port ${minio_port}"

docker run -d --name chimera-e2e-dump-api \
	--network "container:${project}-api" \
	--cap-add NET_RAW --cap-add NET_ADMIN \
	-v "$out:/out" "$image" \
	tcpdump -i any -U -w /out/api-internal.pcap 'tcp port 5432 or tcp port 9000'

sleep 1

docker run --rm --network host \
	-e E2E_API_URL="http://127.0.0.1:${api_port}" \
	-e E2E_OUT=/out \
	-v "$out:/out" \
	-v "$backend/scripts/demo-requests.sh:/demo-requests.sh:ro" \
	"$image" sh /demo-requests.sh

docker stop chimera-e2e-dump-client chimera-e2e-dump-api >/dev/null

docker run --rm -v "$out:/out" "$image" \
	tshark -r /out/client.pcap \
	-d "tcp.port==${api_port},http" -d "tcp.port==${minio_port},http" \
	-Y http -T fields -E header=y -E separator='|' \
	-e frame.number -e ip.src -e tcp.dstport \
	-e http.request.method -e http.request.uri -e http.response.code \
	> "$out/client-http.txt"

docker run --rm -v "$out:/out" "$image" \
	tshark -r /out/api-internal.pcap \
	-d tcp.port==9000,http \
	-Y 'http or pgsql' \
	> "$out/api-internal.txt"
printf '\n# SQL\n' >> "$out/api-internal.txt"
docker run --rm -v "$out:/out" "$image" \
	tshark -r /out/api-internal.pcap \
	-Y 'pgsql.query' -T fields -e pgsql.query \
	>> "$out/api-internal.txt"

docker run --rm -v "$out:/out" "$image" chmod -R a+rX /out

if ! grep -q '|POST|/v1/auth/register|' "$out/client-http.txt"; then
	echo 'client capture is missing the register request' >&2
	exit 1
fi
if ! grep -qi 'pgsql' "$out/api-internal.txt"; then
	echo 'internal capture has no PostgreSQL traffic' >&2
	exit 1
fi

printf 'capture written to %s\n' "$out"
