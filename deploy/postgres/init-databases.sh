#!/bin/sh
# Creates one role and one database per service, so no service can read another one's tables.
# Runs once from docker-entrypoint-initdb.d (or the Postgres pod) on an empty data directory.
set -eu

for service in identity catalog social; do
	var=$(printf '%s_DB_PASSWORD' "$service" | tr '[:lower:]' '[:upper:]')
	eval "password=\${$var:?$var is required}"
	psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
		-v name="$service" -v password="$password" <<'SQL'
CREATE ROLE :"name" LOGIN PASSWORD :'password';
CREATE DATABASE :"name" OWNER :"name";
REVOKE ALL ON DATABASE :"name" FROM PUBLIC;
SQL
done
