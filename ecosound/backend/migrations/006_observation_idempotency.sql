ALTER TABLE observations ADD COLUMN idempotency_key TEXT;
CREATE UNIQUE INDEX observations_idempotency_key ON observations(idempotency_key) WHERE idempotency_key IS NOT NULL;
