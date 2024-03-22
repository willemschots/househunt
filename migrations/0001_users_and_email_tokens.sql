CREATE TABLE users(
    id            INTEGER PRIMARY KEY,
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_active     INTEGER NOT NULL,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE email_tokens (
    token_hash  TEXT NOT NULL PRIMARY KEY,
    user_id     INTEGER NOT NULL,
    email       TEXT NOT NULL,
    purpose     TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    consumed_at TEXT,
    FOREIGN KEY(user_id) REFERENCES users(id)
);
