CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL,
    full_name     TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('admin', 'kasir')),
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    deleted_at    TEXT
);

CREATE UNIQUE INDEX users_username_active ON users (username) WHERE deleted_at IS NULL;

CREATE TABLE tickets (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    price      INTEGER NOT NULL CHECK (price >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT
);

CREATE TABLE sales (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_no    TEXT NOT NULL UNIQUE,
    user_id       INTEGER NOT NULL REFERENCES users (id),
    terminal      TEXT NOT NULL,
    total         INTEGER NOT NULL,
    paid          INTEGER NOT NULL,
    change_amount INTEGER NOT NULL,
    created_at    TEXT NOT NULL
);

CREATE INDEX sales_created_at ON sales (created_at);

CREATE TABLE sale_items (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    sale_id   INTEGER NOT NULL REFERENCES sales (id),
    ticket_id INTEGER NOT NULL REFERENCES tickets (id),
    price     INTEGER NOT NULL,
    qty       INTEGER NOT NULL CHECK (qty > 0),
    subtotal  INTEGER NOT NULL
);

CREATE INDEX sale_items_sale_id ON sale_items (sale_id);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
