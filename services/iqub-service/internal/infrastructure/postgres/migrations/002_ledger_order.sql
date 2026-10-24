-- 002_ledger_order.sql: give ledger entries a stable "oldest first" order.
--
-- A contribution and the payout it triggers are saved with the same
-- timestamp, so ordering by created_at cannot tell them apart. seq is a
-- counter the database fills in, in insert order. Adding the column does
-- not fire the append-only trigger, so this is safe on a database that
-- already holds ledger entries.

ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS seq BIGSERIAL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ledger_seq
    ON ledger_entries(seq);

CREATE INDEX IF NOT EXISTS idx_ledger_iqub_seq
    ON ledger_entries(iqub_id, seq);
