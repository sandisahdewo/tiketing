CREATE TABLE sale_prints (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    sale_id    INTEGER NOT NULL REFERENCES sales (id),
    user_id    INTEGER NOT NULL REFERENCES users (id),
    terminal   TEXT NOT NULL,
    printed_at TEXT NOT NULL
);

CREATE INDEX sale_prints_sale_id ON sale_prints (sale_id);
