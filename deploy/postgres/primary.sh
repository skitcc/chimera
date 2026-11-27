#!/bin/sh
# Let the official entrypoint own initialization and signals. Replication setup
# belongs to init-databases.sh, before the real TCP server starts.
set -eu
: "${POSTGRES_REPLICATION_USER:?POSTGRES_REPLICATION_USER is required}"
: "${POSTGRES_REPLICATION_PASSWORD:?POSTGRES_REPLICATION_PASSWORD is required}"
exec docker-entrypoint.sh postgres \
  -c wal_level=replica \
  -c hot_standby=on \
  -c max_wal_senders=10 \
  -c max_replication_slots=10 \
  -c max_slot_wal_keep_size=1GB \
  -c hot_standby_feedback=on
