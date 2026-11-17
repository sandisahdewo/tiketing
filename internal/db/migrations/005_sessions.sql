-- Logins that are still valid. They are kept here so that closing and
-- opening the app on the server PC does not log every cashier out.
-- token_hash is the SHA-256 of the session token: a copy of the database
-- (a backup) cannot be used to log in. expires_at is in Unix seconds.
CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id),
    expires_at INTEGER NOT NULL
);

CREATE INDEX sessions_user ON sessions (user_id);
