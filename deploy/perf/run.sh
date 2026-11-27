#!/bin/sh
# The runner measures an existing stand. Deploying or resetting it is a separate action.
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
exec python3 "$root/deploy/perf/runner.py"
