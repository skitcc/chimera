#!/bin/sh
# Compatibility entrypoint: preparation is explicit and never erases data.
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
case ${1:-} in
  prepare) exec python3 "$root/deploy/perf/prepare.py" ;;
  restore) PERF_RESTORE=1 exec python3 "$root/deploy/perf/prepare.py" ;;
  *) echo 'Automatic infrastructure reset was removed. Use make -C backend perf-prepare; no PVC/data is deleted.' >&2; exit 2 ;;
esac
