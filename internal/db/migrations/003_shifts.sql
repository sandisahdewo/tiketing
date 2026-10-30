-- Cash is counted per shift, against the cash the drawer started with. This
-- replaces the daily cash closing of 002, which was never released.
DROP TABLE IF EXISTS cash_closings;

-- The shifts an admin sets up, e.g. Siang 06:00-14:00. An end at or before
-- the start ends the next day. Types are turned off, never deleted.
CREATE TABLE shift_types (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    start_time TEXT NOT NULL,
    end_time   TEXT NOT NULL,
    active     INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX shift_types_name ON shift_types (name COLLATE NOCASE);

-- One cashier's shift at one counter, from opening (with the cash already in
-- the drawer) until closed_at. Name and hours are copied from the type when it
-- opens. business_date is the day the shift belongs to: a night shift opened
-- after midnight belongs to the day before. A user and a counter each have at
-- most one open shift.
CREATE TABLE shifts (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    type_id          INTEGER NOT NULL REFERENCES shift_types (id),
    name             TEXT NOT NULL,
    start_time       TEXT NOT NULL,
    end_time         TEXT NOT NULL,
    business_date    TEXT NOT NULL,
    scheduled_end_at TEXT NOT NULL,
    user_id          INTEGER NOT NULL REFERENCES users (id),
    counter_id       INTEGER NOT NULL REFERENCES counters (id),
    opening_cash     INTEGER NOT NULL CHECK (opening_cash >= 0),
    opened_at        TEXT NOT NULL,
    closed_at        TEXT
);

CREATE UNIQUE INDEX shifts_user_open ON shifts (user_id) WHERE closed_at IS NULL;
CREATE UNIQUE INDEX shifts_counter_open ON shifts (counter_id) WHERE closed_at IS NULL;
CREATE INDEX shifts_business_date ON shifts (business_date);

-- The cash counted when a shift closed, kept with the shift's sales as they
-- were then. A correction adds the next revision; the highest one counts.
CREATE TABLE shift_closings (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    shift_id      INTEGER NOT NULL REFERENCES shifts (id),
    revision      INTEGER NOT NULL,
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

CREATE UNIQUE INDEX shift_closings_revision ON shift_closings (shift_id, revision);

-- The shift a sale was made in; empty for sales from before shifts.
ALTER TABLE sales ADD COLUMN shift_id INTEGER REFERENCES shifts (id);

CREATE INDEX sales_shift ON sales (shift_id);

-- Starting shifts, so selling goes on right after the update; an admin sets
-- the real hours.
INSERT INTO shift_types (name, start_time, end_time, created_at, updated_at) VALUES
    ('Siang', '06:00', '14:00', datetime('now', 'localtime'), datetime('now', 'localtime')),
    ('Sore', '14:00', '22:00', datetime('now', 'localtime'), datetime('now', 'localtime')),
    ('Malam', '22:00', '06:00', datetime('now', 'localtime'), datetime('now', 'localtime'));
