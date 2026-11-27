#!/bin/sh
# Runs the e2e suite against a throwaway copy of the whole system: every service,
# Postgres, RustFS and Kafka in their own compose project, network and volumes.
set -eu
. "$(dirname "$0")/stand.sh"
load_env
# Shared perf lock: never start builds or a full stack while a perf run holds it.
if [ "${E2E_PERF_LOCK_HELD:-}" != 1 ]; then
exec 9>"$stand_repo_root/.perf-run.lock"
flock -n 9 || { printf '%s\n' 'perf run is active; retry after it finishes' >&2; exit 1; }
fi
. "$stand_script_dir/e2e-secrets.sh"
sh "$stand_script_dir/check-e2e-credentials.sh"
export STAND_ALLURE_SUITE=e2e

artifacts=${ARTIFACTS_DIR:-$stand_backend_dir}
mkdir -p "$artifacts/allure-results" "$artifacts/e2e-out"
id=$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')
case "$id" in
[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]) ;;
*)
	stand_fail "failed to generate a run id"
	;;
esac
project=chimera-e2e-$id
export PROJECT_NAME="$project"

e2e_compose() {
 if [ "${E2E_TEST_TAGS:-}" = perfauth ]; then
  docker compose --project-directory "$stand_repo_root" --project-name "$project" \
   -f "$stand_repo_root/docker-compose.yml" -f "$stand_repo_root/docker-compose.e2e.yml" \
   -f "$stand_repo_root/deploy/perf/auth-compat.compose.yml" "$@"
  return
 fi
	docker compose --project-directory "$stand_repo_root" --project-name "$project" \
		-f "$stand_repo_root/docker-compose.yml" -f "$stand_repo_root/docker-compose.e2e.yml" "$@"
}

cleanup() {
	code=$?
	if [ "${E2E_KEEP_STACK:-}" = 1 ]; then
		printf '%s\n' "keeping compose project $project" >&2
		exit "$code"
	fi
	if e2e_compose logs --no-color > "$artifacts/e2e-out/$project.log" 2>&1; then
		printf '%s\n' "system log: $artifacts/e2e-out/$project.log" >&2
	fi
 python3 "$stand_script_dir/sanitize-auth-artifacts.py" "$artifacts/e2e-out" "$artifacts/auth-bdd" "$artifacts/allure-results" || code=1
 if ! e2e_compose down --volumes --remove-orphans >/dev/null 2>&1; then
		printf '%s\n' "failed to remove compose project $project" >&2
		if [ "$code" -eq 0 ]; then
			code=1
		fi
	fi
	exit "$code"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if ! e2e_compose up --build --detach --wait --wait-timeout 300 gateway web; then
	stand_fail "system under test $project did not become healthy"
fi

if [ "${E2E_IDENTITY_INTEGRATION:-}" = 1 ]; then
 e2e_compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres \
  -c 'CREATE DATABASE identity_auth_integration OWNER identity' >/dev/null
 docker run --rm --network "${project}_default" \
  --user "${TEST_UID:?}:${TEST_GID:?}" -e HOME=/tmp \
  -e ALLURE_RESULTS_DIR=/artifacts/allure-results \
  -e TEST_DATABASE_URL="postgres://identity:${IDENTITY_DB_PASSWORD}@postgres:5432/identity_auth_integration?sslmode=disable" \
  -v "$artifacts:/artifacts" "${TEST_IMAGE:?}" \
  go test -count=1 -p=1 -tags=integration chimera/identity/...
fi

auth_mode=secured
[ "${E2E_TEST_TAGS:-}" != perfauth ] || auth_mode=legacy
docker run --rm --network "${project}_default" \
	--user "${TEST_UID:?TEST_UID is required}:${TEST_GID:?TEST_GID is required}" \
	-e HOME=/tmp \
	-e GITHUB_ACTIONS -e E2E_USER_PASSWORD -e E2E_NEW_PASSWORD \
	-e E2E_AUTH_MODE="$auth_mode" \
	-e ALLURE_RESULTS_DIR=/artifacts/allure-results \
 -e E2E_WEB_URL=http://web \
 -e E2E_MAILPIT_URL=http://mailpit:8025 \
 -e E2E_IDENTITY_DATABASE_URL="postgres://identity:${IDENTITY_DB_PASSWORD}@postgres:5432/identity?sslmode=disable" \
 -e AUTH_BDD_RESULTS_DIR=/artifacts/auth-bdd \
 -e E2E_API_URL=http://gateway:8080 \
	-e E2E_RUN="${RUN:-$id}" \
	-v "$artifacts:/artifacts" \
	"${TEST_IMAGE:?TEST_IMAGE is required}" \
	go test -count=1 -p=2 -tags="${E2E_TEST_TAGS:-e2e}" chimera/e2e/...
