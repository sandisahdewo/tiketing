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
    category   TEXT NOT NULL DEFAULT 'ticket' CHECK (category IN ('ticket', 'topup')),
    color      TEXT NOT NULL DEFAULT 'orange',
    active     INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT
);

-- Counters (loket): each PC is assigned one counter by an admin, and a
-- counter is used by at most one PC (device_id).
CREATE TABLE counters (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,
    invoice_code TEXT NOT NULL,
    active       INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    device_id    TEXT,
    device_name  TEXT NOT NULL DEFAULT '',
    last_seen_at TEXT,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

CREATE UNIQUE INDEX counters_name ON counters (name COLLATE NOCASE);
CREATE UNIQUE INDEX counters_invoice_code ON counters (invoice_code);
CREATE UNIQUE INDEX counters_device ON counters (device_id) WHERE device_id IS NOT NULL;

-- terminal is the counter name printed on the receipt; reports use counter_id.
CREATE TABLE sales (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_no    TEXT NOT NULL UNIQUE,
    user_id       INTEGER NOT NULL REFERENCES users (id),
    counter_id    INTEGER NOT NULL REFERENCES counters (id),
    terminal      TEXT NOT NULL,
    total         INTEGER NOT NULL,
    paid          INTEGER NOT NULL,
    change_amount INTEGER NOT NULL,
    created_at    TEXT NOT NULL,
    deleted_at    TEXT,
    deleted_by    INTEGER REFERENCES users (id),
    delete_reason TEXT NOT NULL DEFAULT ''
);

CREATE INDEX sales_created_at ON sales (created_at);
CREATE INDEX sales_user_created ON sales (user_id, created_at);
CREATE INDEX sales_counter_created ON sales (counter_id, created_at);

CREATE TABLE sale_items (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    sale_id   INTEGER NOT NULL REFERENCES sales (id),
    ticket_id INTEGER NOT NULL REFERENCES tickets (id),
    price     INTEGER NOT NULL,
    qty       INTEGER NOT NULL CHECK (qty > 0),
    subtotal  INTEGER NOT NULL
);

CREATE INDEX sale_items_sale_id ON sale_items (sale_id);

CREATE TABLE sale_prints (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    sale_id    INTEGER NOT NULL REFERENCES sales (id),
    user_id    INTEGER NOT NULL REFERENCES users (id),
    terminal   TEXT NOT NULL,
    printed_at TEXT NOT NULL
);

CREATE INDEX sale_prints_sale_id ON sale_prints (sale_id);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
