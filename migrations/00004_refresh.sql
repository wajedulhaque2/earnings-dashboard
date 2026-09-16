-- +goose Up
ALTER TABLE provider_sync_state ADD COLUMN attempted_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX sync_requests_due_idx ON sync_requests(next_attempt) WHERE attempts<5;
-- +goose Down
DROP INDEX sync_requests_due_idx;
ALTER TABLE provider_sync_state DROP COLUMN attempted_at;
