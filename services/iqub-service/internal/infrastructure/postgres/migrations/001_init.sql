CREATE TABLE IF NOT EXISTS iqubs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    contribution_amount BIGINT NOT NULL CHECK (contribution_amount > 0),
    frequency TEXT NOT NULL CHECK (frequency IN ('WEEKLY', 'BIWEEKLY', 'MONTHLY')),
    max_members INTEGER NOT NULL CHECK (max_members BETWEEN 2 AND 50),
    status TEXT NOT NULL CHECK (status IN ('OPEN', 'ACTIVE', 'COMPLETED')),
    admin_id TEXT NOT NULL,
    invite_code TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS iqub_members (
    iqub_id TEXT NOT NULL REFERENCES iqubs(id) ON DELETE RESTRICT,
    member_id TEXT NOT NULL,
    join_order INTEGER NOT NULL CHECK (join_order >= 1),
    PRIMARY KEY (iqub_id, member_id),
    UNIQUE (iqub_id, join_order)
);

CREATE INDEX IF NOT EXISTS idx_iqub_members_member
    ON iqub_members(member_id);

CREATE TABLE IF NOT EXISTS rounds (
    id TEXT PRIMARY KEY,
    iqub_id TEXT NOT NULL REFERENCES iqubs(id) ON DELETE RESTRICT,
    number INTEGER NOT NULL CHECK (number >= 1),
    due_date TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'CURRENT', 'COMPLETE')),
    recipient_id TEXT NOT NULL,
    UNIQUE (iqub_id, number)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_rounds_one_current_per_iqub
    ON rounds(iqub_id) WHERE status = 'CURRENT';

CREATE TABLE IF NOT EXISTS contributions (
    id TEXT PRIMARY KEY,
    iqub_id TEXT NOT NULL REFERENCES iqubs(id) ON DELETE RESTRICT,
    round_id TEXT NOT NULL REFERENCES rounds(id) ON DELETE RESTRICT,
    member_id TEXT NOT NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    status TEXT NOT NULL CHECK (status IN ('RECORDED', 'CONFIRMED')),
    recorded_at TIMESTAMPTZ NOT NULL,
    confirmed_at TIMESTAMPTZ,
    UNIQUE (round_id, member_id)
);

CREATE INDEX IF NOT EXISTS idx_contributions_round
    ON contributions(round_id);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id TEXT PRIMARY KEY,
    iqub_id TEXT NOT NULL REFERENCES iqubs(id) ON DELETE RESTRICT,
    round_id TEXT NOT NULL REFERENCES rounds(id) ON DELETE RESTRICT,
    member_id TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('CONTRIBUTION', 'PAYOUT', 'REVERSAL')),
    amount BIGINT NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ledger_iqub_created
    ON ledger_entries(iqub_id, created_at, id);

CREATE OR REPLACE FUNCTION prevent_ledger_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'ledger entries are append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS ledger_no_update ON ledger_entries;
CREATE TRIGGER ledger_no_update
BEFORE UPDATE OR DELETE ON ledger_entries
FOR EACH ROW EXECUTE FUNCTION prevent_ledger_mutation();
