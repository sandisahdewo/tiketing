-- Report filters by counter and cashier, and listing the counters quickly.
CREATE INDEX sales_terminal ON sales (terminal);
CREATE INDEX sales_user_created ON sales (user_id, created_at);
