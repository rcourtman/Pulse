# Release/v6.4 delivery-health backport qualification

Parent: `89c8e61463d134408caef430b580b53e154e17f4`, freshly confirmed public
release/v6.4 head and fast-forwarded from supplied `3f3d7d6df3`.
Backport source: `9d1b726da9a08797c6adb6330a1d1bac6d763ac7`.
Only the existing health read's latest-started ownership changes at runtime.
No new product surface; this repairs a reproduced candidate regression under
RELEASE_PROMOTION_POLICY's Release Train rules.

## Reproduction and qualification

- Added the source hook/caller regressions first: 9 failed, 7 passed against
  unmodified parent runtime. After runtime backport: all 16 passed.
  Command: `cd frontend-modern && npm test -- src/features/alerts/__tests__/useNotificationDeliveryHealth.test.tsx src/features/alerts/__tests__/useAlertDestinationsTabState.test.tsx`.
- `pulse-heavy-run -- node scripts/check-delivery-health-ordering.mjs` failed
  before repair: stale healthy completion removed degraded attention. After:
  12 cases passed at 1440x900 and 390x900. Each run used this release checkout.
- Real Chromium, real caller/hook/card, scripted API and queue-action results.
  Preserves latest success/error and loading ownership across configuration
  Retry and retained-delivery retry/dismiss refreshes. No installed server,
  provider acceptance, recipient receipt, full-shell or accessibility claim.
- The two retained screenshots were visually inspected. Narrow controls wrap
  within the card; newer degraded/unavailable warnings remain visible.
- Before/after outputs (trailing whitespace normalised) are adjacent; `source-sha256.json` binds exercised
  runtime and fixture, and frontend browser receipt names this release parent.
  Fixture screenshots now stay inside this checkout rather than shared /tmp.

## CI continuation already satisfied in source

`3f3d7d6df3` already backported `3413b37940`; workflow/script patch IDs match
(`6ccb854541f04c7c5e208fad18b497133c110fd9`). Do not duplicate it.
Re-running `python3 scripts/tests/test_release_train_ci_contract.py` with the
pre-repair workflow blobs from `35770da` fails all four workflow/event cases;
restoring current workflows passes. Both outputs retained. No gates changed.

Fresh read-only GitHub evidence: PR #1922 merged as this parent. Checks on
its exact proposal head `3f3d7d6df3254867403668bfbf5a6950a136ae59` include
backend/frontend and eight Playwright Core E2E shards. Admission is verified,
not build success: Benchmarks failed (run 34000982948, job 101399753658),
other jobs were queued/running at inspection. The new backport still needs
normal publication/review and exact-proposal checks.

External primary evidence independently checked on 6 September 2026:
https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax
confirms pull_request branch filters select target branches. Existing path and
job filters continue to apply. The admission test is not a replacement for CI.

No publication or installed user benefit established here. Delivery still needs
an exact-candidate assessment, installed recovery/receipt evidence and the
required candidate soak before stable approval; this local proof changes none
of those authorities.
