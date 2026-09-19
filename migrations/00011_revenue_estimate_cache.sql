-- +goose Up
CREATE TABLE revenue_provider_cache (
 symbol text NOT NULL, operation text NOT NULL,
 payload jsonb NOT NULL, fetched_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(symbol,operation)
);
CREATE TABLE revenue_provider_budget (
 provider text PRIMARY KEY, request_times timestamptz[] NOT NULL,
 last_request timestamptz NOT NULL
);
-- +goose Down
DROP TABLE revenue_provider_cache;
DROP TABLE revenue_provider_budget;
