#!/bin/sh
set -eu
. "$(dirname "$0")/stand.sh"
load_env
stand_compose up --detach --wait --wait-timeout 180 --remove-orphans
stand_require
stand_require_rustfs
stand_require_template
printf '%s\n' "test stand is up: postgres 127.0.0.1:${POSTGRES_TEST_PORT}/postgres, rustfs 127.0.0.1:${STAND_MINIO_PORT}"
