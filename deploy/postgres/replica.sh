#!/bin/sh
# Clones the primary into an empty data directory and runs it as a hot standby.
# Each replica streams through its own slot, so the primary keeps the WAL it still needs.
set -eu

if [ -z "${PGDATA:-}" ]; then
	echo "PGDATA is required" >&2
	exit 1
fi
if [ -z "${POSTGRES_REPLICATION_USER:-}" ] || [ -z "${POSTGRES_REPLICATION_PASSWORD:-}" ]; then
	echo "POSTGRES_REPLICATION_USER and POSTGRES_REPLICATION_PASSWORD are required" >&2
	exit 1
fi
primary="${PRIMARY_HOST:-postgres}"
slot=$(printf '%s' "${REPLICATION_SLOT:-$(hostname)}" | tr -c 'a-z0-9_\n' '_')

if [ ! -s "$PGDATA/PG_VERSION" ]; then
	mkdir -p "$PGDATA"
	chmod 0700 "$PGDATA"
	find "$PGDATA" -mindepth 1 -delete
	echo "waiting for $primary and taking a base backup into slot $slot" >&2
	export PGPASSWORD="${POSTGRES_REPLICATION_PASSWORD}"
	# -C fails when the slot survived a lost volume; the second form reuses it.
	until pg_basebackup -h "$primary" -p 5432 -U "$POSTGRES_REPLICATION_USER" -D "$PGDATA" -Fp -Xs -R -C -S "$slot" ||
		pg_basebackup -h "$primary" -p 5432 -U "$POSTGRES_REPLICATION_USER" -D "$PGDATA" -Fp -Xs -R -S "$slot"; do
		echo "base backup failed, retrying" >&2
		find "$PGDATA" -mindepth 1 -delete
		sleep 2
	done
	unset PGPASSWORD
fi

# cluster_name becomes application_name in pg_stat_replication on the primary.
exec docker-entrypoint.sh postgres -c hot_standby=on -c hot_standby_feedback=on -c cluster_name="$slot"
