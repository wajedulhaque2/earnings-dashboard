-- +goose Up
ALTER TABLE provider_sync_state DROP CONSTRAINT provider_sync_state_status_check;
ALTER TABLE provider_sync_state RENAME COLUMN attempted_at TO latest_attempt_at;
ALTER TABLE provider_sync_state RENAME COLUMN status TO latest_attempt_status;
ALTER TABLE provider_sync_state RENAME COLUMN error TO latest_error_category;
ALTER TABLE provider_sync_state RENAME COLUMN last_sync TO last_success_at;
ALTER TABLE provider_sync_state ALTER COLUMN latest_attempt_at DROP NOT NULL;
ALTER TABLE provider_sync_state ALTER COLUMN latest_attempt_at DROP DEFAULT;
UPDATE provider_sync_state SET latest_attempt_status='not_attempted', latest_attempt_at=NULL
 WHERE latest_attempt_status IN ('pending','running');
ALTER TABLE provider_sync_state ADD CONSTRAINT provider_sync_state_latest_status_check
 CHECK (latest_attempt_status IN ('success','error','unsupported','not_attempted'));
COMMENT ON COLUMN provider_sync_state.last_success_at IS 'Last successful refresh, preserved after errors or unsupported attempts.';
COMMENT ON COLUMN provider_sync_state.latest_attempt_at IS 'Latest completed attempt; NULL when provider operation was not attempted.';

-- +goose Down
ALTER TABLE provider_sync_state DROP CONSTRAINT provider_sync_state_latest_status_check;
UPDATE provider_sync_state SET latest_attempt_status='pending' WHERE latest_attempt_status IN ('unsupported','not_attempted');
UPDATE provider_sync_state SET latest_attempt_at=COALESCE(last_success_at,now()) WHERE latest_attempt_at IS NULL;
ALTER TABLE provider_sync_state ALTER COLUMN latest_attempt_at SET NOT NULL;
ALTER TABLE provider_sync_state ALTER COLUMN latest_attempt_at SET DEFAULT now();
ALTER TABLE provider_sync_state RENAME COLUMN latest_attempt_at TO attempted_at;
ALTER TABLE provider_sync_state RENAME COLUMN latest_attempt_status TO status;
ALTER TABLE provider_sync_state RENAME COLUMN latest_error_category TO error;
ALTER TABLE provider_sync_state RENAME COLUMN last_success_at TO last_sync;
ALTER TABLE provider_sync_state ADD CONSTRAINT provider_sync_state_status_check CHECK (status IN ('pending','running','success','error'));
