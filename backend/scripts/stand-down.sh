#!/bin/sh
set -eu
. "$(dirname "$0")/stand.sh"
load_env
stand_compose down --remove-orphans
