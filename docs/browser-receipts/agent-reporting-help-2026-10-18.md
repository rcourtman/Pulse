# Agent reporting help — browser evidence, 18 October 2026

## Containing source

Verified producer `30eb57a6a16821459c2f7886306db111a8c99ecc`, parent
`126cd04bf7d9d07d962fdfff778ab5d9bd4fb9de`. This receipt is recorded in a
non-merge commit whose own parent is the producer. No served content changed
after these observations.

| Content | SHA-256 |
| --- | --- |
| `docs/TROUBLESHOOTING.md` | `d3e99dc46e384ceaad50b4672ebce5216e812c0eeecb0eb43a074d2467ce8989` |
| `docs/UNIFIED_AGENT.md` | `ab54679a5f1ea26068d40d48c1a99f757c2a9d3d14cb913a0b9e14bd6325314b` |
| `frontend-modern/public/docs/TROUBLESHOOTING.md` | `d3e99dc46e384ceaad50b4672ebce5216e812c0eeecb0eb43a074d2467ce8989` |
| `frontend-modern/public/docs/UNIFIED_AGENT.md` | `ab54679a5f1ea26068d40d48c1a99f757c2a9d3d14cb913a0b9e14bd6325314b` |

## Completed evidence and adverse result

- All five new documentation checks pass; four reject the original missing
  help and the unchanged runtime-order consistency check still passes.
- Exact source-proof build `source-jqhl0jw9` compiles the real Docs page,
  router, Markdown renderer, PageHeader and production styles. All 2,997
  source-input hashes and 29 artifact hashes match. The outer shell alone is
  synthetic; no backend, agent, mount or recovery operation runs.
- Playwright 1.56.1, Chromium 141.0.7390.37, 1440 × 1000 light desktop:
  keyboard symptom/filter/authentication links, fragment focus, Back, reload,
  fetched Markdown byte equality and no document overflow pass.
- WebKit 26.0, 320 × 900 dark phone: touch symptom/filter/authentication
  links, focus and Back reach the intended content. Four narrow captures
  complete without document overflow or an earlier page/HTTP error.
- **The final WebKit reload crashes the page. Browser client exit is 1.**
  Narrow reload and the following narrow fetched-byte assertions are not
  completed. This result is retained, not labelled a passing browser run or
  replayed for green. It establishes neither a product nor executor cause;
  ordinary Web ownership retains the bounded next assessment.
- All eight original captures were opened using native `view_image`.
  Desktop text, focus outline and links are readable. Narrow dark headings,
  inline code, safety boundaries and coverage advice wrap within the viewport;
  no horizontal clipping is visible. Browser and preview-server close calls
  complete. Those pixels do not clear the subsequent crash.

| Capture | SHA-256 |
| --- | --- |
| `desktop-light-reporting.png` | `6870ece1462f83453b5866b26cf9f51343fbe5eff00c058f33d3161b83637000` |
| `desktop-light-storage.png` | `0a4e82f0514069dd3707f1f9808dfda95838f32628afd1b80a7dc458a0731332` |
| `desktop-light-safety.png` | `b7c1c13bb5b245453fb7231fa4ff82feb8f187c8d8838d2b4bae2e0a5a0b3ebe` |
| `desktop-light-filters.png` | `25d36381f57395c973a4b69a277ab4d778a0c8f762cbb33c8fe4ba5e4706fdf3` |
| `phone-dark-reporting.png` | `e96542855b73d1319381c393946ec121d796c2316e192a9375137a05bd9ab524` |
| `phone-dark-storage.png` | `f5eab47697ea00a3faeb0cd951440d68d53f9c453595b8fb0e1926bf97d69702` |
| `phone-dark-safety.png` | `96fe5abee189258380ed6c21dc450c6a3adf730eeb7e6f8a39619f25de215512` |
| `phone-dark-filters.png` | `acbe8cb10f3fb77291f0e79d1a8c801848e6ec76216c564f9a007d88f913e111` |

Detailed source, command, complete raw output, screenshots and the original
adverse browser result are retained in the lane's `RECEIPTS.md`,
`BUILD-VERIFICATION.json`, `BROWSER-RESULT.json` and `PIXEL-REVIEW.json`.
This is passive existing-help verification, not an installed NFS/kernel
recovery, accepted host report, reporter retest or runtime repair claim.
