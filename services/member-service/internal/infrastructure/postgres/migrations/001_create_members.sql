CREATE TABLE members (
    id            UUID        PRIMARY KEY,
    full_name     TEXT        NOT NULL,
    phone         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL,
    CONSTRAINT members_phone_key UNIQUE (phone)
);
