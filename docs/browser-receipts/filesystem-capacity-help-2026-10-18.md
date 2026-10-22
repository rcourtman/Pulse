# Filesystem-capacity recovery help — browser evidence, 18 October 2026

## Containing source

Verified producer `a32eb1b4578e44b1228dc4f9b16ad20704f19310`, parent
`2db672fddf4400ea524fc44f82192a3c8eaa4c37`. This receipt is recorded in a
non-merge commit whose own parent is the producer. No served content changed
after the pass.

| Content | SHA-256 |
| --- | --- |
| `docs/TROUBLESHOOTING.md` | `4387826ccfbc2cad8908b4c0eda580dedec58d8c86cf4151315e020618239331` |
| `docs/CONFIGURATION.md` | `a5ab3b7df92f515f1210fabbb568e2765253a4883f842b0c52a0f4377d560930` |
| `frontend-modern/public/docs/TROUBLESHOOTING.md` | `4387826ccfbc2cad8908b4c0eda580dedec58d8c86cf4151315e020618239331` |
| `frontend-modern/public/docs/CONFIGURATION.md` | `a5ab3b7df92f515f1210fabbb568e2765253a4883f842b0c52a0f4377d560930` |
| `scripts/tests/test_alert_recovery_help.py` | `0db47320d9a8bf06068b552f9ee7c4aff6e1f01a52bf9f70d33e9bbaa3a8f312` |

## Completed evidence

- `source-lv29_6qz` passes 19 help/configuration checks, 62 shipped mirrors
  and 259 public-document checks. The new capacity-timing check rejects both
  original guides (nine expected assertions, no fixture error). These are help
  checks, not a reproduction of the open report.
- `source-or6a66ea` compiles the real Docs page, router, Markdown renderer,
  PageHeader and production styles. All 2,997 source inputs and 29 artifacts
  hash-match; the outer shell alone is synthetic. Both complete proof outputs,
  command/client exits 0 and completed guest stops are retained.
- Playwright 1.56.1 matches the integration lock. Chromium 141.0.7390.37 at
  1440 × 1000 light desktop and 320 × 1000 dark phone passes keyboard/touch
  symptom and cross-document links, fragment focus, Back, reload, exact fetched
  Markdown and overflow checks. Browser client exit 0; browser/server closed.
- All six captures were opened with native `view_image`. The capacity paragraph
  and linked Configuration explanation wrap readably at both widths. The narrow
  capacity capture also shows the unresolved-symptom and safety paragraphs;
  desktop fragment focus is visible. No horizontal clipping is visible.

| Capture | SHA-256 |
| --- | --- |
| `desktop-light-incident.png` | `a48f7bf2697326b60817d5b9e02532d4f2e1bfa600c49794c89b8b1e61fad006` |
| `desktop-light-capacity.png` | `cd2782a77f0074a13907685162f2f7575fa42075deb0c43d69ca615ef6f8e966` |
| `desktop-light-configuration.png` | `f7c7e41272983cf9b81e64d6a4be1305a31970786dd1dd30ab44e70cd106413a` |
| `phone-dark-incident.png` | `7c17fd2216bf449ec5fdcf1aac927141c597c3ff6a29914a80c1308716e506f7` |
| `phone-dark-capacity.png` | `85a3595a143d534683102bf532bdb1889293d7a5c783442a182f7491027beda4` |
| `phone-dark-configuration.png` | `d649a11571c5680c3943ea2c1146c1ced1e7559e7028603e926c78a3182e44da` |

## Limits

This proves the changed existing help and navigation, not a runtime repair,
installed or recipient recovery, reporter resolution or release availability.
Chromium narrow proof does not disposition the separately retained WebKit
reload crash in `agent-reporting-help-2026-10-18.md`. No alert policy, backend,
agent, guest probe, backup, credential or recovery operation changed or ran.

Complete commands, raw outputs, preview, script, captures and source binding
are retained with the lane outcome in `RECEIPTS.md`, `VALIDATION.json`,
`SOURCE-PROOFS.tar.xz`, `BUILD-PREVIEW.tgz`, `BROWSER-EXECUTION.tar.xz`,
`BROWSER-RESULT.json` and `PIXEL-REVIEW.json`.
