-- The PC that sends a sale gives it a number of its own (request_id). When
-- the answer of the server got lost and the same sale is sent again, the
-- server recognises the number and reports the sale it already has, so the
-- cashier is asked before it is recorded a second time. Sales from before
-- this have none.
ALTER TABLE sales ADD COLUMN request_id TEXT;

CREATE UNIQUE INDEX sales_request ON sales (request_id) WHERE request_id IS NOT NULL;
