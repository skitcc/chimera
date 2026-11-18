#!/bin/sh
set -eu
. "$(dirname "$0")/stand.sh"
load_env
stand_require
if [ "${1:-}" = "rustfs" ]; then
	stand_require_rustfs
fi
