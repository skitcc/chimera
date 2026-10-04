#!/bin/sh
set -eu
. "$(dirname "$0")/stand.sh"
load_env
stand_require
stand_require_template

artifacts=${ARTIFACTS_DIR:-$stand_backend_dir}
mkdir -p "$artifacts/allure-results"
if [ "$#" -eq 0 ]; then
	set -- ./internal/infra/adapters/postgres ./internal/usecase
fi

id=$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')
case "$id" in
[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]) ;;
*)
	printf '%s\n' "failed to generate a database id" >&2
	exit 1
	;;
esac
db=t_$id

cleanup() {
	code=$?
	if ! stand_psql -c "DROP DATABASE IF EXISTS \"$db\" WITH (FORCE)" >/dev/null; then
		printf '%s\n' "failed to drop $db" >&2
		if [ "$code" -eq 0 ]; then
			code=1
		fi
	fi
	exit "$code"
}
trap cleanup EXIT

stand_psql -c "CREATE DATABASE \"$db\" TEMPLATE chimera_template"
url="postgres://${POSTGRES_TEST_USER}:${POSTGRES_TEST_PASSWORD}@postgres:5432/${db}?sslmode=disable"
docker run --rm \
	--network chimera-stand \
	--user "${TEST_UID:?TEST_UID is required}:${TEST_GID:?TEST_GID is required}" \
	-e HOME=/tmp \
	-e ALLURE_RESULTS_DIR=/artifacts/allure-results \
	-e TEST_DATABASE_URL="$url" \
	-v "$artifacts:/artifacts" \
	"${TEST_IMAGE:?TEST_IMAGE is required}" \
	go test -count=1 -p=1 \
	-tags="${INTEGRATION_TAGS:-integration}" \
	-run "${INTEGRATION_RUN:-Integration}" \
	"$@"
