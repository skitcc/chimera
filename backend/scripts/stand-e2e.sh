#!/bin/sh
set -eu
. "$(dirname "$0")/stand.sh"
load_env
export STAND_ALLURE_SUITE=e2e
stand_require
stand_require_rustfs
stand_require_template

if [ -z "${S3_ACCESS_KEY:-}" ] || [ -z "${S3_SECRET_KEY:-}" ]; then
	stand_fail "S3_ACCESS_KEY and S3_SECRET_KEY are required in .env"
fi

artifacts=${ARTIFACTS_DIR:-$stand_backend_dir}
mkdir -p "$artifacts/allure-results"
id=$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')
case "$id" in
[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]) ;;
*)
	stand_fail "failed to generate a database id"
	;;
esac
db=t_$id
bucket=t-$id
api_name=chimera-e2e-$id
api_image=${API_IMAGE:-chimera-stand-api:local}

cleanup() {
	code=$?
	if [ "${E2E_KEEP_STACK:-}" = 1 ]; then
		printf '%s\n' "keeping api $api_name database $db bucket $bucket" >&2
		exit "$code"
	fi
	mkdir -p "$artifacts/e2e-out"
	if docker logs "$api_name" > "$artifacts/e2e-out/$api_name.log" 2>&1; then
		printf '%s\n' "api log: $artifacts/e2e-out/$api_name.log" >&2
	fi
	docker rm -f "$api_name" >/dev/null 2>&1 || true
	if ! stand_psql -c "DROP DATABASE IF EXISTS \"$db\" WITH (FORCE)" >/dev/null; then
		printf '%s\n' "failed to drop $db" >&2
		if [ "$code" -eq 0 ]; then
			code=1
		fi
	fi
	if ! docker run --rm --network chimera-stand \
		-e S3_ENDPOINT=http://rustfs:9000 \
		-e S3_ACCESS_KEY="$S3_ACCESS_KEY" \
		-e S3_SECRET_KEY="$S3_SECRET_KEY" \
		"${TEST_IMAGE:?TEST_IMAGE is required}" \
		go run ./scripts/standbucket -bucket "$bucket"; then
		printf '%s\n' "failed to delete bucket $bucket" >&2
		if [ "$code" -eq 0 ]; then
			code=1
		fi
	fi
	exit "$code"
}
trap cleanup EXIT

stand_psql -c "CREATE DATABASE \"$db\" TEMPLATE chimera_template"
database_url="postgres://${POSTGRES_TEST_USER}:${POSTGRES_TEST_PASSWORD}@postgres:5432/${db}?sslmode=disable"

docker run -d --name "$api_name" --network chimera-stand \
	--env-file "$stand_repo_root/.env" \
	-e DATABASE_URL="$database_url" \
	-e HTTP_ADDR=:8080 \
	-e S3_ENDPOINT=http://rustfs:9000 \
	-e S3_PRESIGN_ENDPOINT="http://127.0.0.1:${STAND_MINIO_PORT}" \
	-e S3_BUCKET="$bucket" \
	-p "127.0.0.1::8080" \
	"$api_image" >/dev/null

i=0
while [ "$i" -lt 90 ]; do
	status=$(stand_healthy "$api_name")
	if [ "$status" = "healthy" ]; then
		break
	fi
	if [ "$status" = "unhealthy" ] || [ "$status" = "none" ]; then
		docker logs "$api_name" >&2 || true
		stand_fail "api container $api_name did not become healthy"
	fi
	i=$((i + 1))
	sleep 1
done
if [ "$(stand_healthy "$api_name")" != "healthy" ]; then
	docker logs "$api_name" >&2 || true
	stand_fail "api container $api_name did not become healthy"
fi

port=$(docker port "$api_name" 8080/tcp | head -n 1 | sed 's/.*://')
if [ -z "$port" ]; then
	stand_fail "api container $api_name has no published port"
fi

docker run --rm --network host \
	--user "${TEST_UID:?TEST_UID is required}:${TEST_GID:?TEST_GID is required}" \
	-e HOME=/tmp \
	-e ALLURE_RESULTS_DIR=/artifacts/allure-results \
	-e E2E_API_URL="http://127.0.0.1:${port}" \
	-e E2E_RUN="${RUN:-$(printf '%s' "$id" | cut -c1-8)}" \
	-v "$artifacts:/artifacts" \
	"${TEST_IMAGE:?TEST_IMAGE is required}" \
	go test -count=1 -tags=e2e ./e2e/...
