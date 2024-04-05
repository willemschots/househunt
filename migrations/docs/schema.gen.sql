CREATE TABLE migrations (
	sequence           INTEGER PRIMARY KEY,
	filename           TEXT NOT NULL,
	revision           TEXT NOT NULL,
	revision_timestamp TIMESTAMP NOT NULL
);
CREATE TABLE users(
    id                INTEGER PRIMARY KEY,
    email_encrypted   TEXT NOT NULL,
    email_blind_index TEXT NOT NULL UNIQUE,
    password_hash     TEXT NOT NULL,
    is_active         INTEGER NOT NULL,
    created_at        TIMESTAMP NOT NULL,
    updated_at        TIMESTAMP NOT NULL
);
CREATE TABLE email_tokens (
    id              INTEGER PRIMARY KEY,
    token_hash      TEXT NOT NULL,
    user_id         INTEGER NOT NULL,
    email_encrypted TEXT NOT NULL,
    purpose         TEXT NOT NULL,
    created_at      TIMESTAMP NOT NULL,
    consumed_at     TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES users(id)
);
