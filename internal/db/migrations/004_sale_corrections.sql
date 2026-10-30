-- A sale made to correct another one points to it; the corrected sale is
-- deleted, with the reason, in the same step.
ALTER TABLE sales ADD COLUMN replaces_sale_id INTEGER REFERENCES sales (id);

CREATE INDEX sales_replaces ON sales (replaces_sale_id);
