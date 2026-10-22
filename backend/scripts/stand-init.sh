#!/bin/sh
set -eu

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
	-c "CREATE DATABASE chimera_template"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname chimera_template \
	-f /schema.sql
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
	-c "UPDATE pg_database SET datallowconn = false WHERE datname = 'chimera_template'"
