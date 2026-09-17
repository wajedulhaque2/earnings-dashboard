# Methodology

## Source and missing-value policy

Provider records are normalized before persistence. Ordinary pages read PostgreSQL. Explicit remote search and workers are the only provider callers. Financial fields are nullable: an unavailable value is not zero and is never replaced with an inferred figure. Provider upserts preserve non-null observations when a response omits a field. Derived reactions can become NULL when a new source record invalidates an earlier calculation.

Yahoo supplies search/profile data, earnings dates, EPS estimates/actuals/surprises, daily OHLC and recent pre/post-market minute observations where available. Its endpoints are unofficial. SEC supplies structured submissions and company-facts data, with per-field priority over Yahoo for reported quarterly fundamentals. Separate revenue/EPS provenance avoids mislabelling a Yahoo EPS value as SEC data.

SEC extraction accepts directly reported USD revenue and diluted EPS observations with 75–105 day periods. It excludes annual and year-to-date totals rather than subtracting them to manufacture quarters. It chooses the latest filed observation for the same period and prioritizes recognized consolidated revenue tags. Company-specific tags, unusual short/long quarters, IFRS facts and unsupported currencies may be absent. Restated facts may replace earlier values; this is not a historical point-in-time dataset. Period-end dates are retained, while fiscal labels stay NULL unless explicitly supplied. Do not interpret SEC filing fiscal-year metadata as the fiscal year of every comparative fact.

Yahoo quarterly charts show up to twelve available observations, possibly fewer. Fiscal labels from explicit earnings event names are retained. Historical revenue estimates and forward quarterly estimates are not reliably sourced by the current adapters, so revenue-surprise/conditional tables may have zero samples and forward chart estimates are omitted. No past premarket price is reconstructed from daily OHLC. Logos are optional stored fields and render only when supplied; no logo service or inferred URL is called.

## Session assignment

All U.S. event timestamps use America/New_York, including daylight-saving transitions. Normal regular hours are 09:30–16:00. A source BMO/AMC designation takes precedence. An explicitly supplied timed timestamp can establish BMO/AMC/during-market timing; unspecified times and midnight date placeholders stay UNKNOWN. Supported early-close dates use 13:00 for timestamp classification. Unknown and during-market events do not receive BMO/AMC return calculations.

- Tuesday AMC: previous close Tuesday, event session Wednesday.
- Wednesday BMO: previous close Tuesday, event session Wednesday.
- Friday AMC: next exchange session, skipping weekends and holidays.
- A BMO/AMC event on a closed date uses the next valid session.

The centralized U.S. cash-equity calendar supports 1990–2035, standard observed holidays, historical commencement of MLK/Juneteenth holidays and listed exceptional full-day closures. Early-close days still count as trading sessions. The calendar is a maintained rule set, not an exchange feed: annual review and newly announced exceptional closures require updates. Reactions are computed only for companies whose provider metadata explicitly identifies America/New_York; other/unknown market timezones remain N/A.

## Prices and returns

All stored returns are fractions: 0.052 displays as 5.2%. Provider surprise percentages remain percentage points, separately from fractional price returns.

The engine looks up the exact expected trading date. It never compresses a missing price bar into the next available date. Missing, non-finite or non-positive price inputs yield NULL. Daily closes use the Yahoo close series (split-adjusted, without dividend total-return adjustment); adjusted close is stored separately and is not silently substituted. Results are price returns, not dividend-inclusive total returns. Current-day daily bars are withheld until the next New York date, conservatively avoiding incomplete sessions and early-close assumptions.

- Premarket return = selected premarket price / previous session close − 1.
- Event-day return = event close / previous session close − 1.
- Forward return = target session close / event close − 1.

Forward horizons are 1D=1, 2D=2, 1W=5, 2W=10, 1M=21 and 3M=63 trading sessions after event close. Missing or future horizons stay NULL.

Premarket selects the latest available stored PRE observation from 09:25 through 09:29 New York time on the event session, preferring Yahoo on an equal timestamp. Only completed minute bars are ingested. This is an observation close to the open, not a guaranteed executable price. If a later stock split makes an old unadjusted snapshot incompatible with the refreshed split-adjusted historical close, the premarket return is withheld. No synthetic split-adjusted intraday reconstruction is made.

Methodology identifier: `us-sessions-v1-fractional-close`. Stored event dates and calculation timestamps support inspection and repeatable recalculation.

## Statistics

Every horizon has its own sample set. NULL and non-finite values are excluded from sample counts, arithmetic means, medians, extrema, win rates and standard deviation. A win means return > 0; zero is a valid observation but not a win. Median is the middle sorted value or the average of the two middle values. Standard deviation uses the sample denominator n−1 and is NULL below two observations. Empty sets show N/A, with n=0.

Earnings conditions require both nonzero EPS and revenue surprise percentages. Positive is a beat; negative is a miss. In-line (zero) and missing results are excluded rather than assigned a fabricated direction. Group Events counts qualifying events; each return statistic separately excludes missing horizons.

Gap buckets use the observed premarket return g:

| Bucket | Interval |
| --- | --- |
| Above +5% | g > 0.05 |
| +2% to +5% | 0.02 ≤ g ≤ 0.05 |
| −2% to +2% | −0.02 < g < 0.02 |
| −5% to −2% | −0.05 ≤ g ≤ −0.02 |
| Below −5% | g < −0.05 |

Continuation = sign(g) × ((1 + event-day return) / (1 + g) − 1). Positive means movement from the observed premarket price to the close continued in the initial gap direction. A zero initial gap has no direction and is excluded from continuation statistics, while remaining eligible for its bucket's forward-return statistics. Gap win rate is the fraction of valid continuation observations above zero. Weekly/monthly averages still use ordinary signed forward returns.

## Interpretation limits

Sources can revise earnings dates, estimates, actuals and prices. Records are keyed by company/report date; a changed date without a reliable common source identifier can leave an older scheduled record. Review suspect duplicates on `/status` and against the source; do not silently merge different reports. Upcoming dates are not guaranteed announcements. Suspensions/delistings and missing bars remain missing. Historical samples are descriptive, not proof of predictive performance or a complete survivorship-free universe.
