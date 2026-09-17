# SEC eligibility and provider status audit

Verified on 2026-09-17 against the configured local PostgreSQL/Compose application and official SEC JSON APIs. No API subscription or fabricated financial data was used.

## Findings and changes

The old adapter already used SEC's official ticker mapping, but returned a generic not-found error for unmapped symbols. Ingestion classified that as a provider failure. The page displayed only the last-success date beside the latest status, omitted the latest-attempt date, combined operation/symbol and sorted NULL success dates first. This made old failures dominate the display.

The ten requested symbols resolved correctly in the live mapping. The old MU/META/NVDA statuses dated from earlier attempts and had no recorded prior SEC success. Their generic legacy error category is insufficient to prove why those requests failed. Fresh financials and filings requests succeeded using the existing configured SEC contact identity.

SEC eligibility now normalizes symbols and validates CIKs against the official mapping before financials/filings calls. Dot/hyphen share-class aliases are supported. An absent CIK yields `unsupported`, without downstream data calls or an optional-provider refresh failure. Mapping outages remain errors; missing SEC configuration is `not_attempted` with `not_configured`. Foreign and OTC-style symbols are not categorically excluded: some have valid SEC identities.

Migration `00005_provider_status.sql` separates `latest_attempt_at`, `latest_attempt_status`, `latest_error_category` and `last_success_at`. Every existing last-success timestamp was compared before/after the live migration and preserved exactly. Later failures or unsupported results preserve prior success. The status page shows separate provider, operation, symbol, latest attempt/status, last success and safe error category, with distinct success/error/unsupported/not-attempted styling. Timestamps include year, seconds and timezone. Results are ordered by latest attempt, limited to 500 rows.

Unsupported operations log at INFO and do not fail scheduler cycles. Unit tests explicitly enforce no downstream SEC calls when eligibility fails, no WARN/ERROR logs for unsupported symbols, and successful scheduler completion with an unsupported optional provider.

## Live symbol audit

| Symbol | SEC CIK | Live financials / filings |
| --- | --- | --- |
| LEN | 0000920760 | success; 62 quarterly rows, 1,002 filing rows |
| NVDA | 0001045810 | success; 67 quarterly rows, 1,000 filing rows |
| META | 0001326801 | success; 46 quarterly rows, 1,001 filing rows |
| MU | 0000723125 | success; 63 quarterly rows, 1,001 filing rows |
| AAPL | 0000320193 | success; 67 quarterly rows, 1,000 filing rows |
| MSFT | 0000789019 | CIK resolution verified |
| AMZN | 0001018724 | CIK resolution verified |
| GOOGL | 0001652044 | CIK resolution verified |
| TSLA | 0001318605 | CIK resolution verified |
| AMD | 0000002488 | CIK resolution verified |

Counts describe persisted observations after this audit, not guaranteed source coverage. Missing financial fields remain NULL. Current-calendar symbols AACTF, AAGFF, ABCCF and ABLGF were confirmed unmapped, recorded as unsupported for financials/filings and returned a non-failing optional refresh. AAVXF resolved to CIK 0001956827 and was correctly not excluded as an OTC-style symbol.

61 existing SEC error rows were reclassified as unsupported only after checking the live mapping. Other historical errors were retained; no unverified success was written. Normal future ingestion updates status using the same eligibility checks.

## Verification and rollout

- Required gofmt, vet, unit tests and build checks.
- PostgreSQL integration tests: repeated migrations/upserts, later-failure timestamp preservation, unsupported/not-attempted/success transitions and error clearing.
- Live Docker images rebuilt, migration applied, application restarted and status page inspected.
- Scheduler was not running at audit start and remains stopped; the updated worker image is ready for the next run.

On another installation, stop app/worker before upgrading because migration 00005 renames status columns:

```sh
docker compose stop app worker
docker compose build app migrate worker
docker compose run --rm --no-deps migrate /app/migrate up
docker compose up -d --no-deps app worker
```

For a fresh verification of a company, run `docker compose run --rm --no-deps worker /app/worker sync-financials NVDA` (replace the symbol as needed). This refreshes SEC plus available Yahoo fallback and updates the status rows. Configure a real descriptive SEC_USER_AGENT in the local environment; do not commit it.
