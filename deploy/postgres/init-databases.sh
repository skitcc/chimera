#!/bin/sh
# Creates one role and one database per service, so no service can read another one's tables.
# With <SVC>_DB_READ_PASSWORD set it also creates <svc>_ro, which may only read that database;
# physical replication copies the role to every replica.
# Runs once from docker-entrypoint-initdb.d (or the Postgres pod) on an empty data directory.
set -eu

for service in identity catalog social; do
	upper=$(printf '%s' "$service" | tr '[:lower:]' '[:upper:]')
	eval "password=\${${upper}_DB_PASSWORD:?${upper}_DB_PASSWORD is required}"
	psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
		-v name="$service" -v password="$password" <<'SQL'
CREATE ROLE :"name" LOGIN PASSWORD :'password';
CREATE DATABASE :"name" OWNER :"name";
REVOKE ALL ON DATABASE :"name" FROM PUBLIC;
SQL

	eval "read_password=\${${upper}_DB_READ_PASSWORD:-}"
	[ -n "$read_password" ] || continue
	psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$service" \
		-v name="$service" -v reader="${service}_ro" -v password="$read_password" <<'SQL'
CREATE ROLE :"reader" LOGIN PASSWORD :'password' NOINHERIT;
GRANT CONNECT ON DATABASE :"name" TO :"reader";
GRANT USAGE ON SCHEMA public TO :"reader";
ALTER DEFAULT PRIVILEGES FOR ROLE :"name" IN SCHEMA public GRANT SELECT ON TABLES TO :"reader";
SQL
done

# Runs on a new primary only, under the official entrypoint's temporary server.
# Compose without replication keeps using this same database initializer.
if [ -n "${POSTGRES_REPLICATION_USER:-}" ]; then
  : "${POSTGRES_REPLICATION_PASSWORD:?POSTGRES_REPLICATION_PASSWORD is required}"
  : "${PGDATA:?PGDATA is required}"
  case "$POSTGRES_REPLICATION_USER" in
    *[!a-zA-Z0-9_]*) echo 'replication role must be an identifier' >&2; exit 1 ;;
  esac
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
    -v role="$POSTGRES_REPLICATION_USER" -v password="$POSTGRES_REPLICATION_PASSWORD" <<'SQL'
CREATE ROLE :"role" WITH REPLICATION LOGIN PASSWORD :'password';
SQL
  printf 'host replication %s all scram-sha-256\n' "$POSTGRES_REPLICATION_USER" >> "$PGDATA/pg_hba.conf"
fi
