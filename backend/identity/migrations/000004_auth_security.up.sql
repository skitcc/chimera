ALTER TABLE users ADD COLUMN auth_version bigint NOT NULL DEFAULT 1,
 ADD COLUMN auth_failures integer NOT NULL DEFAULT 0,
 ADD COLUMN locked_until timestamptz;
CREATE TABLE auth_challenges (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 purpose text NOT NULL CHECK (purpose IN ('login','recovery','change')),
 id text NOT NULL UNIQUE,
 digest bytea NOT NULL,
 version bigint NOT NULL,
 expires_at timestamptz NOT NULL,
 sent_at timestamptz NOT NULL,
 PRIMARY KEY (user_id, purpose)
);
