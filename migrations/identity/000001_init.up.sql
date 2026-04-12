CREATE EXTENSION IF NOT EXISTS "citext";

-- Identities
-- Owns: credentials, active status, roles.
-- Does NOT own: firstName, lastName — those live in profile_db.
CREATE TABLE identities (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT      NOT NULL,
    password_hash TEXT        NOT NULL,
    roles         TEXT[]      NOT NULL DEFAULT '{user}',
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_identities_email ON identities(email);
CREATE INDEX idx_identities_is_active ON identities(is_active) WHERE is_active = TRUE;
