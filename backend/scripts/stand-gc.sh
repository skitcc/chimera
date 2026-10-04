#!/bin/sh
set -eu
. "$(dirname "$0")/stand.sh"
load_env
stand_require

list=$(docker exec chimera-stand-postgres \
	psql -v ON_ERROR_STOP=1 -U "$POSTGRES_TEST_USER" -d postgres -tAc \
	"SELECT datname FROM pg_database WHERE datname ~ '^t_[0-9a-f]+$' AND NOT EXISTS (SELECT 1 FROM pg_stat_activity a WHERE a.datname = pg_database.datname)")

dropped=0
for db in $list; do
	case "$db" in
	t_[0-9a-f]*) ;;
	*)
		printf '%s\n' "refusing to drop unexpected database $db" >&2
		exit 1
		;;
	esac
	stand_psql -c "DROP DATABASE \"$db\" WITH (FORCE)"
	printf '%s\n' "dropped $db"
	dropped=$((dropped + 1))
done
printf '%s\n' "dropped $dropped orphan databases"
