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
		printf '%s\n' "test stand is not running; from backend/ run: make stand-up" >&2
		exit 1
	fi
}

stand_require_rustfs() {
	if [ "$(stand_healthy chimera-stand-rustfs)" != "healthy" ]; then
		printf '%s\n' "test stand RustFS is not running; from backend/ run: make stand-up" >&2
		exit 1
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
		printf '%s\n' "chimera_template is missing or still accepts connections; from backend/ run: make stand-refresh" >&2
		exit 1
	fi
}
