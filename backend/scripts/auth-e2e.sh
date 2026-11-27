#!/bin/sh
set -eu
backend=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
sh "$backend/scripts/check-e2e-credentials.sh"
exec 9>"$backend/../.perf-run.lock"
flock -n 9 || { printf '%s\n' 'perf run is active; retry after it finishes' >&2; exit 1; }
export E2E_PERF_LOCK_HELD=1
make -C "$backend" test-image
exec sh "$backend/scripts/stand-e2e.sh"
