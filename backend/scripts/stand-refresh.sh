#!/bin/sh
set -eu
. "$(dirname "$0")/stand.sh"
load_env
stand_require

stand_psql <<'SQL'
UPDATE pg_database SET datallowconn = true WHERE datname = 'chimera_template';
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = 'chimera_template' AND pid <> pg_backend_pid();
DROP DATABASE IF EXISTS chimera_template;
CREATE DATABASE chimera_template;
SQL
docker exec -i chimera-stand-postgres \
	psql -v ON_ERROR_STOP=1 -U "$POSTGRES_TEST_USER" -d chimera_template -f /schema.sql
stand_psql -c "UPDATE pg_database SET datallowconn = false WHERE datname = 'chimera_template'"
stand_require_template
printf '%s\n' "rebuilt chimera_template; existing t_* copies were left in place"
