# Dashboard state-delivery help — browser evidence, 22 October 2026

## Containing source

Verified source `580b058efadec7dcd80a789593ba7c9102739d0a`. This receipt is recorded in a non-merge commit with that exact source as its own parent. No served content changed after the pass.

| Content | SHA-256 |
| --- | --- |
| `docs/REVERSE_PROXY.md` | `3ff226a3221c9e01083151b11eceff02a65aa599c977c5a0021a86083d9e2eb6` |
| `docs/TROUBLESHOOTING.md` | `5fb3f1923c192cf7d815077d5ea5f1d9589725307e933e2de6d75c45a93af0f1` |
| `frontend-modern/browser-tests/build-doc-command-help.mjs` | `29cac323776b24b388c1a4254a75f0a6c62d0e89cae1a81539e4fc23c0958cfb` |
| `frontend-modern/public/docs/REVERSE_PROXY.md` | `3ff226a3221c9e01083151b11eceff02a65aa599c977c5a0021a86083d9e2eb6` |
| `frontend-modern/public/docs/TROUBLESHOOTING.md` | `5fb3f1923c192cf7d815077d5ea5f1d9589725307e933e2de6d75c45a93af0f1` |
| `frontend-modern/src/features/docs/__tests__/dashboardStateGuide.test.ts` | `852f57d96b80b9739ed3990652efe442c99bd077a229b7c34f09e692b31beb68` |
| `frontend-modern/src/features/docs/__tests__/troubleshootingEntryGuide.test.ts` | `80c3603c9d9911b3b7caee32cfc037f62980bcd23b581027d5be7c638f68bca2` |

## Completed proof

- On `660f30563d`, offline source proof `source-f7vahb1p` passed all 310 Docs checks, type-check and the full lint pipeline, then exited 1 at formatting in the new test. The only subsequent source change is that test's Prettier line wrapping; production content and consuming code are unchanged.
- On the final containing source, `source-1erzvnev` passes the seven-check file and formatting, 265 public Markdown checks, all 62 shipped mirrors, production build, bundle budgets and the real Docs consuming build. Client/command exits are zero, complete output matches its receipt and the guest stopped. Parent-document controls expose six missing-guidance failures; these are not an outage reproduction.
- All 3,244 consuming source inputs and 35 preview artifacts match their recorded hashes. The production Docs page, router, Markdown renderer, PageHeader and styles consume the changed served Markdown; only the outer application shell is a fixture.
- Playwright 1.56.1 matches the integration lock. Chromium 141.0.7390.37 at 1365 × 1000 light desktop and WebKit 26.0 at 320 × 1000 dark touch pass the symptom link, same-shell fragment focus, Getting Help and Backup safety destinations, and exact served-document readback. Before-controls show that the own-parent guides lack the entry and section in both profiles.
- All five captures below were opened with native `view_image`; the distinction, existing links and privacy/session/backup-safety warnings are legible without horizontal page overflow. No console/page errors or external/backend/mutation requests; browser and local server closed.

| Capture | SHA-256 |
| --- | --- |
| `desktop-state-phases.png` | `daed6d8cb31be99f6e9a7228fc982a71874afaf1d8a890929d0d35f57bdf7ccd` |
| `narrow-entry.png` | `a0abcea057bd8a9addab6426ae862a348b6034bd1cf6d029fb4216da3c6a13b6` |
| `narrow-state-phases.png` | `be7d556247e32f3fb12088e14c11db29c92a8e624afcf6d51c85d02eafedface` |
| `narrow-comparisons.png` | `0cceb89809db7ec14c78722e204f74d32d85f81459386bd04d3f7f485ce0b2a0` |
| `narrow-privacy-and-safety.png` | `c979ec8439d641884c949fea274c39d5d65b1b598b1add6f793afca974e22024` |

## Adverse result and limits

The first run caught an incorrect expectation that an in-document link would be rewritten to a route; the renderer deliberately keeps `#fragment`. That test correction and the later formatting failure are retained, not hidden or claimed as overall passes. Source and browser results, exact protocols/bindings, raw outputs and pixel observations are retained with the lane outcome in RECEIPTS.md, VALIDATION-CORRECTIONS.json, PIXEL-REVIEW.json and CURRENT-EVIDENCE.tar.xz.

This verifies help content and its consuming navigation, not the cause or repair of the dashboard stall, truthful runtime health, reporter resolution, encrypted SSO, guest/filesystem recovery, installation, protected landing or published availability. The separate stopped Back/reload/inert-control investigation was not rerun or cleared. No credentials, provider, guest, backup or diagnostic operation was used.
