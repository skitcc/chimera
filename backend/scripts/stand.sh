#!/bin/sh
if [ "${0##*/}" = "stand.sh" ]; then
	printf '%s\n' "source this file from another stand script" >&2
	exit 1
fi

stand_script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
stand_backend_dir=$(CDPATH= cd -- "$stand_script_dir/.." && pwd)
stand_repo_root=$(CDPATH= cd -- "$stand_script_dir/../.." && pwd)

load_env() {
	if [ ! -f "$stand_repo_root/.env" ]; then
		printf '%s\n' "missing $stand_repo_root/.env; copy .env.example to .env" >&2
		exit 1
	fi
	set -a
	# shellcheck disable=SC1091
	. "$stand_repo_root/.env"
	set +a
	if [ -z "${POSTGRES_TEST_USER:-}" ] || [ -z "${POSTGRES_TEST_PASSWORD:-}" ]; then
		printf '%s\n' "add POSTGRES_TEST_USER and POSTGRES_TEST_PASSWORD from .env.example to .env" >&2
		exit 1
	fi
	case "$POSTGRES_TEST_PASSWORD" in
	*[!A-Za-z0-9._~-]*)
		printf '%s\n' "POSTGRES_TEST_PASSWORD must contain only URL-safe characters" >&2
		exit 1
		;;
	esac
	POSTGRES_TEST_PORT=${POSTGRES_TEST_PORT:-54329}
	STAND_MINIO_PORT=${STAND_MINIO_PORT:-9002}
	export POSTGRES_TEST_USER POSTGRES_TEST_PASSWORD POSTGRES_TEST_PORT STAND_MINIO_PORT
}

stand_compose() {
	docker compose --project-directory "$stand_repo_root" -f "$stand_repo_root/docker-compose.test.yml" "$@"
}

stand_healthy() {
	container=$1
	docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container" 2>/dev/null || true
}

stand_require() {
	if [ "$(stand_healthy chimera-stand-postgres)" != "healthy" ]; then
		stand_fail "test stand is not running; from backend/ run: make stand-up"
	fi
}

stand_require_rustfs() {
	if [ "$(stand_healthy chimera-stand-rustfs)" != "healthy" ]; then
		stand_fail "test stand RustFS is not running; from backend/ run: make stand-up"
	fi
}

stand_psql() {
	docker exec -i chimera-stand-postgres \
		psql -v ON_ERROR_STOP=1 -U "$POSTGRES_TEST_USER" -d postgres "$@"
}

stand_require_template() {
	state=$(docker exec chimera-stand-postgres \
		psql -v ON_ERROR_STOP=1 -U "$POSTGRES_TEST_USER" -d postgres -tAc \
		"SELECT CASE WHEN datallowconn THEN 'open' ELSE 'closed' END FROM pg_database WHERE datname = 'chimera_template'")
	state=$(printf '%s' "$state" | tr -d '[:space:]')
	if [ "$state" != "closed" ]; then
		stand_fail "chimera_template is missing or still accepts connections; from backend/ run: make stand-refresh"
	fi
}

# stand_fail prints the reason and, for a test launch, records it in Allure.
# STAND_ALLURE_SUITE is integration or e2e. stand-up leaves it unset.
stand_fail() {
	printf '%s\n' "$1" >&2
	stand_allure_broken "$1" || true
	exit 1
}

stand_allure_broken() {
	reason=$1
	case "${STAND_ALLURE_SUITE:-}" in
	e2e)
		layer="End to end"
		title="e2e tests did not start"
		tag=e2e
		;;
	integration)
		layer="Integration"
		title="integration tests did not start"
		tag=integration
		;;
	*)
		return 0
		;;
	esac

	dir=${ARTIFACTS_DIR:-$stand_backend_dir}/allure-results
	mkdir -p "$dir" || return 0
	# A previous successful run leaves its own result file. allure-open
	# renders every file in the directory, so the old pass would stay green.
	for f in "$dir"/*-result.json; do
		[ -f "$f" ] || continue
		if awk -v tag="$tag" 'BEGIN { RS = "}" }
			/"name"[[:space:]]*:[[:space:]]*"tag"/ && $0 ~ "\"value\"[[:space:]]*:[[:space:]]*\"" tag "\"" { found = 1 }
			END { exit !found }' "$f"; then
			rm -f "$f"
		fi
	done
	id=$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')
	now=$(date +%s%3N 2>/dev/null || true)
	case "$now" in
	''|*[!0-9]*) now=$(date +%s)000 ;;
	esac
	esc=$(printf '%s' "$reason" | awk 'BEGIN { ORS = "" }
		{
			gsub(/\\/, "\\\\")
			gsub(/"/, "\\\"")
			if (NR > 1) printf "\\n"
			printf "%s", $0
		}')
	cat > "$dir/${id}-result.json" <<EOF
{
  "uuid": "$id",
  "historyId": "stand-${tag}-not-started",
  "name": "$title",
  "fullName": "stand.${tag}",
  "description": "The stand check failed before go test started.",
  "status": "broken",
  "statusDetails": {"message": "$esc", "trace": "$esc"},
  "stage": "finished",
  "start": $now,
  "stop": $now,
  "labels": [
    {"name": "parentSuite", "value": "$layer"},
    {"name": "suite", "value": "Stand"},
    {"name": "subSuite", "value": "startup"},
    {"name": "epic", "value": "$layer"},
    {"name": "feature", "value": "Stand"},
    {"name": "story", "value": "startup"},
    {"name": "severity", "value": "blocker"},
    {"name": "tag", "value": "$tag"},
    {"name": "package", "value": "stand"},
    {"name": "framework", "value": "shell"},
    {"name": "language", "value": "sh"}
  ],
  "links": [],
  "parameters": [],
  "attachments": [],
  "steps": [
    {
      "name": "stand check",
      "status": "broken",
      "stage": "finished",
      "start": $now,
      "stop": $now,
      "statusDetails": {"message": "$esc"},
      "steps": [],
      "attachments": [],
      "parameters": []
    }
  ]
}
EOF
}
