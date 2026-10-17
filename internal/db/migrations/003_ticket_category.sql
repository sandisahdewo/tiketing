ALTER TABLE tickets ADD COLUMN category TEXT NOT NULL DEFAULT 'ticket' CHECK (category IN ('ticket', 'topup'));
ALTER TABLE tickets ADD COLUMN color TEXT NOT NULL DEFAULT 'orange';
