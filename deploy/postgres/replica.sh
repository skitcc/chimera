#!/bin/sh
# Publish a clone only after pg_basebackup succeeds. PG_VERSION is written early
# and cannot be used as a completion marker for an interrupted backup.
set -eu
: "${PGDATA:?PGDATA is required}"
: "${POSTGRES_REPLICATION_USER:?POSTGRES_REPLICATION_USER is required}"
: "${POSTGRES_REPLICATION_PASSWORD:?POSTGRES_REPLICATION_PASSWORD is required}"
[ "$PGDATA" != / ] || { echo 'PGDATA must not be /' >&2; exit 1; }
primary=${PRIMARY_HOST:-postgres}
slot=$(printf '%s' "${REPLICATION_SLOT:-$(hostname)}" | tr -c 'a-z0-9_\n' '_')
stage=${PGDATA}.bootstrap
export PGPASSWORD="$POSTGRES_REPLICATION_PASSWORD"
export PGCONNECT_TIMEOUT=5

if [ ! -s "$PGDATA/PG_VERSION" ]; then
  mkdir -p "$stage"
  chmod 0700 "$stage"
  attempt=0
  while :; do
    attempt=$((attempt + 1))
    [ "$attempt" -le "${REPLICA_BOOTSTRAP_ATTEMPTS:-30}" ] || {
      echo 'replica bootstrap failed; inspect primary and replication credentials' >&2
      exit 1
    }
    find "$stage" -mindepth 1 -delete
    # A fast checkpoint avoids waiting for the primary's paced checkpoint.
    if pg_basebackup -h "$primary" -U "$POSTGRES_REPLICATION_USER" -D "$stage" \
      -Fp -Xs -R -C -S "$slot" --checkpoint=fast --no-password; then
      break
    fi
    find "$stage" -mindepth 1 -delete
    # Reuse a slot left by a previous attempt. Never treat a partial clone as ready.
    if pg_basebackup -h "$primary" -U "$POSTGRES_REPLICATION_USER" -D "$stage" \
      -Fp -Xs -R -S "$slot" --checkpoint=fast --no-password; then
      break
    fi
    echo "replica bootstrap attempt $attempt failed" >&2
    sleep 2
  done
  touch "$stage/.chimera-clone-complete"
  if [ -d "$PGDATA" ]; then rmdir "$PGDATA"; fi
  mv "$stage" "$PGDATA"
fi
# Older successful clones have no custom marker, but do have standby.signal and
# backup_manifest. An incomplete old PGDATA requires an explicit operator repair.
[ -f "$PGDATA/standby.signal" ] && [ -f "$PGDATA/backup_manifest" ] || {
  echo 'incomplete replica PGDATA; refusing to boot a partial clone' >&2
  exit 1
}
unset PGPASSWORD
exec postgres -D "$PGDATA" -c hot_standby=on -c hot_standby_feedback=on -c cluster_name="$slot"
