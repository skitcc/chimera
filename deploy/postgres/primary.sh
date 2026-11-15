#!/bin/sh
# Starts the primary with WAL streaming enabled and keeps the replication role
# and its pg_hba line in place on every start, including over an existing volume.
set -eu

if [ -z "${POSTGRES_REPLICATION_USER:-}" ] || [ -z "${POSTGRES_REPLICATION_PASSWORD:-}" ]; then
	echo "POSTGRES_REPLICATION_USER and POSTGRES_REPLICATION_PASSWORD are required" >&2
	exit 1
fi

child=0
shutdown() {
	if [ "$child" -ne 0 ]; then
		kill -TERM "$child" 2>/dev/null || true
		wait "$child" || true
	fi
}
trap shutdown TERM INT

# max_slot_wal_keep_size stops a replica that is gone for good from filling the disk with WAL.
docker-entrypoint.sh postgres \
	-c wal_level=replica \
	-c hot_standby=on \
	-c max_wal_senders=10 \
	-c max_replication_slots=10 \
	-c max_slot_wal_keep_size=1GB \
	-c hot_standby_feedback=on &
child=$!

# The temporary server that runs the init scripts listens on the socket only,
# so TCP readiness means the real server is up.
export PGPASSWORD="${POSTGRES_PASSWORD}"
i=0
until pg_isready -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; do
	i=$((i + 1))
	if [ "$i" -gt 120 ]; then
		echo "primary did not become ready" >&2
		shutdown
		exit 1
	fi
	if ! kill -0 "$child" 2>/dev/null; then
		wait "$child"
		exit $?
	fi
	sleep 1
done

exists=$(psql -h 127.0.0.1 -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 -v role="$POSTGRES_REPLICATION_USER" -tA <<'SQL'
SELECT 1 FROM pg_roles WHERE rolname = :'role'
SQL
)
if [ "$exists" = "1" ]; then
	verb=ALTER
else
	verb=CREATE
fi
psql -h 127.0.0.1 -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 \
	-v role="$POSTGRES_REPLICATION_USER" \
	-v pass="$POSTGRES_REPLICATION_PASSWORD" \
	-v verb="$verb" <<'SQL'
SELECT format('%s ROLE %I WITH REPLICATION LOGIN PASSWORD %L', :'verb', :'role', :'pass')
\gexec
SQL

pgdata=$(psql -h 127.0.0.1 -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 -tAc 'SHOW data_directory' | tr -d '[:space:]')
line="host replication ${POSTGRES_REPLICATION_USER} all scram-sha-256"
if ! grep -Fqx "$line" "$pgdata/pg_hba.conf"; then
	printf '%s\n' "$line" >> "$pgdata/pg_hba.conf"
	psql -h 127.0.0.1 -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 -c 'SELECT pg_reload_conf()' >/dev/null
fi
unset PGPASSWORD

wait "$child"
