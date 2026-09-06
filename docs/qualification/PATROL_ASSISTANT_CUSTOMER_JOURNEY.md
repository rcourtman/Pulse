
## Release-line identity regression verification — 6 September 2026

Scoped backport of main fix `33b852f66b` onto `a5caca0d63`, with portable
synthetic fixture from `2d36bb7d4d`. No private capture or main browser receipt
is used. `scripts/check-assistant-identity.mjs` exercises this checkout's real
reducer and renderer at 1440, 900 and 390 pixels; browser service workers are
blocked and API/external HTTP requests are aborted and fail the check.

All 167 affected unit tests pass. The browser check passes at all three widths:
concurrent same-name IDs stay distinct, completion retains the sibling approval,
and removal of a workflow row preserves shared input/output evidence. Each
original runtime file substituted independently from the release parent fails
the browser check; restored fixed files pass. Desktop approval and narrow
expanded-evidence screenshots were visually inspected. Transient running rows
remain visible in the synthetic streaming approval screenshot; this proof does
not assert complete workflow-status correctness.

Evidence is retained with lane run `20260906T114016Z-release-line`, under
`evidence/final/receipt.json` (source and screenshot hashes), sensitivity logs,
and `evidence/units.log`. Reproduce using the fixture README. This is component
browser regression evidence only, not production routing/build, provider
reasoning, authorised action execution, installed receipt or release readiness.
