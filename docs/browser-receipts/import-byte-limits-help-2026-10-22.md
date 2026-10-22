# Unsent-import and cleanup help — browser evidence, 22 October 2026

## Containing source

Verified containing source `5ca2724565597fc94e9c9d675e838f48a0105356`. This receipt is recorded in a
non-merge commit whose own parent is that source. The migration help is from
`3bdccd8a601d268efb88c1bba61b5648eafc9d84`; the shipped contributor-guide
synchronisation is from `5ca2724565597fc94e9c9d675e838f48a0105356`.
No served document changed after this pass.

| Content | SHA-256 |
| --- | --- |
| `docs/MIGRATION.md` | `5d7c6badd51d582ab1a398318a58bb2f0b2471d7b2a40037751772716fed6b70` |
| `frontend-modern/public/docs/MIGRATION.md` | `5d7c6badd51d582ab1a398318a58bb2f0b2471d7b2a40037751772716fed6b70` |
| `CONTRIBUTING.md` | `96128ffc90b5f7f4312ebd3bf9507f0ddff9e3fa3214809068caa50fdf5b3f8a` |
| `frontend-modern/public/docs/CONTRIBUTING.md` | `96128ffc90b5f7f4312ebd3bf9507f0ddff9e3fa3214809068caa50fdf5b3f8a` |

## Completed evidence

- Exact-source offline proof `source-b6vv0mg4` passes 342 Docs/link/fragment
  tests, ten recovery-guidance checks, all 62 shipped-document mirrors and
  frontend type-checking. The production frontend build and the separate
  production-Docs consumer build both pass. Complete 641,680-byte command
  output matches the receipt; client/command exits are zero and the guest stopped.
- All 3,227 consumer source inputs and 32 preview artifacts hash-match.
  The real Docs page, router, Markdown renderer, PageHeader and production
  styles consume the changed shipped Markdown; only the outer shell is a fixture.
- Playwright 1.56.1 matches the integration lock. Chromium 141.0.7390.37 at
  1365 × 1000 light desktop and WebKit 26.0 at 320 × 960 dark narrow pass
  the transfer-step → unsent checks → unconfirmed checks journey, using
  keyboard Enter and native touch respectively. Both headings receive focus,
  remain visible and retain the same shell. Exact fetched Markdown matches.
- The synchronised cleanup section also renders through Docs at both widths.
  No page/console errors or horizontal page overflow; only local GET requests.
  Browser client exit zero; browser and server closed.
- All seven captures were opened through native `view_image`. Both size budgets,
  archive/passphrase preservation, stop-cutover instructions and the distinction
  from an earlier uncertain import are readable at both widths. The contributor
  guide's preservation and no-privilege-escalation boundaries wrap readably.

| Capture | SHA-256 |
| --- | --- |
| `desktop-chromium-unsent-import.png` | `f772e6ffad987b3ada3c3b75ccc9f0ee5db7b2ee021040f036dbcb4fb939c7f1` |
| `desktop-chromium-uncertain-import.png` | `f245adee8e84f4313a6106ff2943cb72e724f120b6f526fa98b989496bf71f26` |
| `desktop-chromium-cleanup-guide.png` | `31e22455daeb3a2a3eebebd9fc8c7926b0c78e2d576d447db3aa2e2ff7ef45d0` |
| `narrow-webkit-unsent-import.png` | `fc50426f0bbe80481788b0f94b5f7a3e2c248250a1deb00d807a3cf3e94001c9` |
| `narrow-webkit-preserve-state.png` | `1712e4d13938a09ed7587d9d288ded6eddb6228cd9cb060a471a197991a60a73` |
| `narrow-webkit-uncertain-import.png` | `1b6950b51574d8f44599df7af812f0b8e760bb828beec5898bb1cb6aefe8728f` |
| `narrow-webkit-cleanup-guide.png` | `c35479cb2f04673f101bdb593e30de55871bd9b1794b2d3ede32df51553dc915` |

## Limits

This verifies existing help, its consuming renderer and links, not encryption,
actual import or rollback, SSO, native cleanup, installed recovery, reporter
resolution or release availability. The separate adverse mobile-WebKit
history/Back/reload control is unchanged and was not rerun. No API, credential,
backup, guest, installation or recovery operation was performed.

Complete command, raw output, source binding, preview, browser script/results,
captures and pixel observations are retained with the lane outcome in
`RECEIPTS.md`, `SOURCE-PROOF.json`, `BROWSER-PROOF.json`, `PIXEL-REVIEW.json`
and `CURRENT-EVIDENCE.tar.xz`.
