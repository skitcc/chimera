#!/bin/sh
set -eu
. "$(dirname "$0")/stand.sh"
load_env
export STAND_ALLURE_SUITE=integration
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
	stand_fail "failed to generate a database id"
	;;
esac
db=t_$id
run_id=it-${RUN:-$(printf '%s' "$id" | cut -c1-8)}
mkdir -p "$artifacts/test-logs"
run_log=$artifacts/test-logs/$run_id.log

run_event() {
	printf 'time=%s level=INFO msg="%s" run_id=%s%s\n' \
		"$(date +%Y-%m-%dT%H:%M:%S.%3N%:z)" "$1" "$run_id" "${2:+ result=$2}" | tee -a "$run_log"
}

cleanup() {
	code=$?
	if ! stand_psql -c "DROP DATABASE IF EXISTS \"$db\" WITH (FORCE)" >/dev/null; then
		printf '%s\n' "failed to drop $db" >&2
		if [ "$code" -eq 0 ]; then
			code=1
		fi
	fi
	result=passed
	if [ "$code" -ne 0 ]; then
		result=failed
	fi
	run_event "test run finished" "$result"
	printf '%s\n' "integration log: $run_log"
	exit "$code"
}
trap cleanup EXIT

run_event "test run started"
stand_psql -c "CREATE DATABASE \"$db\" TEMPLATE chimera_template"
url="postgres://${POSTGRES_TEST_USER}:${POSTGRES_TEST_PASSWORD}@postgres:5432/${db}?sslmode=disable&application_name=${run_id}"
docker run --rm \
	--network chimera-stand \
	--user "${TEST_UID:?TEST_UID is required}:${TEST_GID:?TEST_GID is required}" \
	-e HOME=/tmp \
	-e ALLURE_RESULTS_DIR=/artifacts/allure-results \
	-e TEST_DATABASE_URL="$url" \
	-e TEST_RUN_ID="$run_id" \
	-e TEST_LOG_FILE="/artifacts/test-logs/$run_id.log" \
	-v "$artifacts:/artifacts" \
	"${TEST_IMAGE:?TEST_IMAGE is required}" \
	go test -count=1 -p=1 \
	-tags="${INTEGRATION_TAGS:-integration}" \
	-run "${INTEGRATION_RUN:-Integration}" \
	"$@"
