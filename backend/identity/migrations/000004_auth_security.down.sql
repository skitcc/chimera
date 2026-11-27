DROP TABLE auth_challenges;
ALTER TABLE users DROP COLUMN auth_version, DROP COLUMN auth_failures, DROP COLUMN locked_until;
