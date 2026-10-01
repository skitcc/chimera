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
project=${PROJECT_NAME:-chimera}

if [ -d "$out" ]; then
	docker run --rm -v "$backend:/work" "$image" rm -rf /work/e2e-out
fi
mkdir -p "$out"

cleanup() {
	docker stop chimera-e2e-dump-api-edge chimera-e2e-dump-objects chimera-e2e-dump-api >/dev/null 2>&1 || true
	docker rm chimera-e2e-dump-api-edge chimera-e2e-dump-objects chimera-e2e-dump-api >/dev/null 2>&1 || true
	if [ "${E2E_KEEP_STACK:-}" != 1 ]; then
		$compose down --volumes --remove-orphans >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT INT TERM

$compose up --build --detach --wait --wait-timeout 300

# tcpdump copies frames from one interface. `any` on a current kernel is a
# cooked socket, and a port filter there often matches nothing. eth0 is the
# container's real Ethernet NIC, where the bridge actually delivers packets.
start_dump() {
	name=$1
	network=$2
	file=$3
	filter=$4
	docker run -d --name "$name" --network "$network" \
		--cap-add NET_RAW --cap-add NET_ADMIN \
		-v "$out:/out" "$image" \
		tcpdump -i eth0 -n -U -w "/out/$file" "$filter" >/dev/null
}

wait_dump() {
	name=$1
	i=0
	while [ "$i" -lt 50 ]; do
		if docker top "$name" 2>/dev/null | grep -q tcpdump; then
			return 0
		fi
		i=$((i + 1))
		sleep 0.1
	done
	echo "$name: tcpdump is not running" >&2
	docker logs "$name" >&2 || true
	exit 1
}

start_dump chimera-e2e-dump-api-edge "container:${project}-api" api-edge.pcap 'tcp port 8080'
start_dump chimera-e2e-dump-objects "container:${project}-minio" objects.pcap 'tcp port 9000'
start_dump chimera-e2e-dump-api "container:${project}-api" api-internal.pcap 'tcp port 5432 or tcp port 9000'
wait_dump chimera-e2e-dump-api-edge
wait_dump chimera-e2e-dump-objects
wait_dump chimera-e2e-dump-api
sleep 1

docker run --rm --network host \
	-e E2E_API_URL="http://127.0.0.1:${api_port}" \
	-e E2E_OUT=/out \
	-v "$out:/out" \
	-v "$backend/scripts/demo-requests.sh:/demo-requests.sh:ro" \
	"$image" sh /demo-requests.sh

docker stop chimera-e2e-dump-api-edge chimera-e2e-dump-objects chimera-e2e-dump-api >/dev/null

for dump in chimera-e2e-dump-api-edge chimera-e2e-dump-objects chimera-e2e-dump-api; do
	printf '%s\n' "--- $dump ---"
	docker logs "$dump" 2>&1 || true
done

docker run --rm -v "$out:/out" "$image" \
	mergecap -w /out/client.pcap /out/api-edge.pcap /out/objects.pcap

docker run --rm -v "$out:/out" "$image" \
	tshark -r /out/client.pcap \
	-d tcp.port==8080,http -d tcp.port==9000,http \
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

docker run --rm -v "$out:/out" "$image" chown -R "$(id -u):$(id -g)" /out

printf '\n--- SQL ---\n'
awk 'seen { print } /^# SQL$/ { seen=1 }' "$out/api-internal.txt"
printf 'capture written to %s\n' "$out"
