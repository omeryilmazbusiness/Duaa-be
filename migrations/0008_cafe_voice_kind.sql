-- Guest QR intents: review, complaint, request. Rating may be omitted on asks.
ALTER TABLE cafe_voices
    ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT 'review';

ALTER TABLE cafe_voices
    DROP CONSTRAINT IF EXISTS cafe_voices_rating_check;

ALTER TABLE cafe_voices
    ADD CONSTRAINT cafe_voices_rating_check CHECK (rating = 0 OR rating BETWEEN 1 AND 5);

ALTER TABLE cafe_voices
    DROP CONSTRAINT IF EXISTS cafe_voices_kind_check;

ALTER TABLE cafe_voices
    ADD CONSTRAINT cafe_voices_kind_check CHECK (kind IN ('review', 'complaint', 'request'));
