#!/bin/sh
# Runs the e2e suite against a throwaway copy of the whole system: every service,
# Postgres, RustFS and Kafka in their own compose project, network and volumes.
set -eu
. "$(dirname "$0")/stand.sh"
load_env
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
	if ! e2e_compose down --volumes --remove-orphans >/dev/null 2>&1; then
		printf '%s\n' "failed to remove compose project $project" >&2
		if [ "$code" -eq 0 ]; then
			code=1
		fi
	fi
	exit "$code"
}
trap cleanup EXIT

if ! e2e_compose up --build --detach --wait --wait-timeout 300 gateway; then
	stand_fail "system under test $project did not become healthy"
fi

docker run --rm --network "${project}_default" \
	--user "${TEST_UID:?TEST_UID is required}:${TEST_GID:?TEST_GID is required}" \
	-e HOME=/tmp \
	-e ALLURE_RESULTS_DIR=/artifacts/allure-results \
	-e E2E_API_URL=http://gateway:8080 \
	-e E2E_RUN="${RUN:-$id}" \
	-v "$artifacts:/artifacts" \
	"${TEST_IMAGE:?TEST_IMAGE is required}" \
	go test -count=1 -tags=e2e chimera/e2e/...
