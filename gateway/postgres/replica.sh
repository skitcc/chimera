#!/bin/sh
set -eu

if [ -z "${PGDATA:-}" ]; then
	echo "PGDATA is required" >&2
	exit 1
fi
if [ -z "${POSTGRES_REPLICATION_USER:-}" ] || [ -z "${POSTGRES_REPLICATION_PASSWORD:-}" ]; then
	echo "POSTGRES_REPLICATION_USER and POSTGRES_REPLICATION_PASSWORD are required" >&2
	exit 1
fi

if [ ! -s "$PGDATA/PG_VERSION" ]; then
	mkdir -p "$PGDATA"
	find "$PGDATA" -mindepth 1 -delete
	echo "waiting for primary and taking a base backup" >&2
	export PGPASSWORD="${POSTGRES_REPLICATION_PASSWORD}"
	until pg_basebackup -h postgres -p 5432 -U "$POSTGRES_REPLICATION_USER" -D "$PGDATA" -Fp -Xs -R; do
		echo "base backup failed, retrying" >&2
		find "$PGDATA" -mindepth 1 -delete
		sleep 2
	done
	unset PGPASSWORD
	chown -R postgres:postgres "$PGDATA"
fi

exec docker-entrypoint.sh postgres -c hot_standby=on
