-- Stage 2 spend packs: versioned evidence, no pending duplicate. Approve still does not pay.
ALTER TABLE spend_approvals
    ADD COLUMN IF NOT EXISTS fingerprint text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS version int NOT NULL DEFAULT 1;

UPDATE spend_approvals
SET fingerprint = lower(btrim(regexp_replace(title, '\s+', ' ', 'g')))
    || '|' || lower(btrim(regexp_replace(supplier, '\s+', ' ', 'g')))
    || '|' || amount::text
    || '|' || upper(btrim(currency))
WHERE fingerprint = '';

CREATE UNIQUE INDEX IF NOT EXISTS spend_approvals_pending_fingerprint_idx
    ON spend_approvals (workspace_id, fingerprint)
    WHERE status IN ('open', 'needs_evidence');
