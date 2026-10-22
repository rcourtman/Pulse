# Provider-save recovery entry routes — browser evidence, 18 October 2026

## Containing source

Verified producer `1982fbff91c273b77c82590ae1d677373eba2ae6`, parent
`c35788950b47e7a09fc78612c497b83a28bd235a`. This receipt is recorded in a
non-merge commit whose own parent is that producer. No served content changed
after the pass. Existing OIDC recovery content is unchanged.

| Content | SHA-256 |
| --- | --- |
| `docs/TROUBLESHOOTING.md` | `383e370154a8174fe1144ce0c3a80b1378cf7f0434582f2170a85151326d10b7` |
| `frontend-modern/public/docs/TROUBLESHOOTING.md` | `383e370154a8174fe1144ce0c3a80b1378cf7f0434582f2170a85151326d10b7` |
| `scripts/tests/test_oidc_session_docs.py` | `dabbf62e73ca73bd2b4f6e9235538e5720b5901b7ffc0965a2386500a24e978c` |
| `docs/OIDC.md` | `224e42731390517d44354604002eaa867b237f1e0a026f3c139d9201957e81e4` |

## Completed evidence

- `source-nwka81kq` passes 35 OIDC/password/authentication-help tests, all
  62 shipped-document mirrors and 259 public-document checks. Both new
  entry-route assertions reject the exact parent guide. No assertion was removed.
- The same exact-source offline proof builds the real Docs page, router,
  Markdown renderer, PageHeader and production styles. All 2,997 source inputs
  and 29 artifacts hash-match. Only the outer shell is synthetic. Complete
  raw output, client/command exits 0 and completed guest stop are verified.
- The first run, `source-ffef4u6p`, exited 1 on a sibling test-import error.
  Its complete output and guest stop are retained. The corrected invocation
  supplies the normal test-directory search path on unchanged source.
- Playwright 1.56.1 / Chromium 141.0.7390.37 at 1440 × 1000 (light desktop) passes the named symptom and 401 entry routes, keyboard/touch links, heading focus, same-shell navigation, Back and reload.
- Playwright 1.56.1 / Chromium 141.0.7390.37 at 320 × 1000 (dark narrow) passes the named symptom and 401 entry routes, keyboard/touch links, heading focus, same-shell navigation, Back and reload.
- Browser client exit 0, no page/console/HTTP errors, browser and server closed.
  Every request is local and GET-only; there are no backend calls.
- All six captures were opened with native `view_image`. New entry text and
  underlined links wrap readably at both widths without horizontal clipping.
  The existing recovery heading and independent-administrator instructions
  are visible; its longer narrow list uses ordinary vertical scrolling.

| Capture | SHA-256 |
| --- | --- |
| `desktop-light-symptom.png` | `79e92d186a0a0afdc1032d33e2223a781d04a9aedfd127974b03b1a7f3b8c1c7` |
| `desktop-light-recovery.png` | `a5ad836fa7ba9d07a224782890edca5aa20a6891e9a57e70a8188f48977c6ccb` |
| `desktop-light-401.png` | `60662568fdb4a0bdde3e27c0fd25a6408819866185362d190e2a8c32025516c4` |
| `phone-dark-symptom.png` | `ccba8c0ff7c96e089779532f8e62d50f76da314175fd9325ab6084b3fb8a4b8d` |
| `phone-dark-recovery.png` | `61d4495f1aa50b45f5e5153ec711fa7184ac1f449a90d9e1ef8379c6a6231e1b` |
| `phone-dark-401.png` | `7793a6ca6d487da8712785b42c99ded621b157df0e15b78bd1f4c36fad3dd177` |

## Limits

This verifies changed help reachability, not the reported OIDC blank-save defect,
encrypted persistence, authenticated login, Settings recovery, installed source
or release availability. The reporter has already supplied/rejected a workaround;
these links do not justify repeating it publicly as a fix. No credential, IdP,
access rule or encrypted file was used or changed.

Chromium narrow proof does not disposition the separately retained source-free
WebKit history/Back/reload crash, or establish native Safari behaviour.

Complete command/raw-output/build/browser/cleanup and pixel-review receipts are
retained with the lane outcome in `RECEIPTS.md`, `VALIDATION.json`,
`SOURCE-PROOFS.tar.xz`, `BUILD-PREVIEW.tgz`, `SOURCE-BINDING.json`,
`BROWSER-EXECUTION.tar.xz`, `BROWSER-RESULT.json` and `PIXEL-REVIEW.json`.
