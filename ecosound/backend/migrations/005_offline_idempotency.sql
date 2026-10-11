ALTER TABLE recordings ADD COLUMN idempotency_key TEXT;
ALTER TABLE recordings ADD COLUMN content_sha256 TEXT;
CREATE UNIQUE INDEX recordings_idempotency_key ON recordings(idempotency_key) WHERE idempotency_key IS NOT NULL;
ALTER TABLE discoveries ADD COLUMN idempotency_key TEXT;
CREATE UNIQUE INDEX discoveries_idempotency_key ON discoveries(idempotency_key) WHERE idempotency_key IS NOT NULL;
