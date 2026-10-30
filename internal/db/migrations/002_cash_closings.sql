-- Daily cash closing of a cashier: a snapshot of that day's sales when the
-- actual cash was counted. A correction adds a row with the next revision;
-- the highest revision of a day and cashier is the one that counts.
CREATE TABLE cash_closings (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    business_date TEXT NOT NULL,
    user_id       INTEGER NOT NULL REFERENCES users (id),
    revision      INTEGER NOT NULL,
    counters      TEXT NOT NULL,
    transactions  INTEGER NOT NULL,
    ticket_qty    INTEGER NOT NULL,
    ticket_total  INTEGER NOT NULL,
    topup_qty     INTEGER NOT NULL,
    topup_total   INTEGER NOT NULL,
    total         INTEGER NOT NULL,
    first_sale_at TEXT,
    last_sale_at  TEXT,
    deleted_count INTEGER NOT NULL,
    deleted_total INTEGER NOT NULL,
    actual_cash   INTEGER NOT NULL,
    note          TEXT NOT NULL DEFAULT '',
    closed_by     INTEGER NOT NULL REFERENCES users (id),
    closed_at     TEXT NOT NULL,
    print_count   INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX cash_closings_revision ON cash_closings (business_date, user_id, revision);
