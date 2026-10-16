# AI safety help — consuming verification, 16 October 2026

## Containing source

- Producer: `3c9a1d27584b351ca00b3b74386066625ff1f2f9`; parent: `061b2ae3df52a905fa9f3657f54530be1e740de0`.
- This receipt is recorded in a separate non-merge commit whose own parent is
  the producer above. No frontend source is changed by the receipt commit.
- The existing Docs route, router, Markdown renderer, styles and static guide
  bytes are consumed; only the surrounding application shell is synthetic.

| Final source | SHA-256 |
| --- | --- |
| `docs/AI_AUTONOMY.md` | `bccf613c2df456bd4601786c6dfe5ca32d01e7268b8c5d94ecc9fce4845706a4` |
| `frontend-modern/public/docs/AI_AUTONOMY.md` | `bccf613c2df456bd4601786c6dfe5ca32d01e7268b8c5d94ecc9fce4845706a4` |
| `docs/AI.md` | `568d229ec2f3ccd8095a81dfbba45a4aca04f68b8e1c42dd8688c5c1fc1b3d40` |
| `frontend-modern/public/docs/AI.md` | `568d229ec2f3ccd8095a81dfbba45a4aca04f68b8e1c42dd8688c5c1fc1b3d40` |
| `frontend-modern/browser-tests/build-doc-command-help.mjs` | `39214bc1be1bcae5b615402fefdcfe9ff71c214adb7539281ef89007ddce8d63` |
| `frontend-modern/browser-tests/ai-safety-help.cjs` | `7d7b7b4b71c596ed11ec60b338ab809653559b3f1146c6abcd2e917b34d9b6b8` |
| `scripts/tests/test_ai_safety_help.py` | `0bfe9157aa3ae357ac5ae01d2be61f5a93c17a9937eae1176e6e875a1bf15b08` |

## Completed observations

- Five new local documentation methods fail on the original guide and pass on
  the correction. Four retained privacy methods, ten AI documentation policy
  methods and twelve mirror-checker methods pass. The actual mirror checker
  covers 62 shipped documents; public links/claims cover 251 Markdown files.
- Source proof `source-h4400fce` builds the real Docs consumer from the
  exact producer with the launch-selected offline frontend snapshot. Client and
  command exit zero; guest readiness and completed stop are recorded. All
  2,963 bound source files and 28 artifact hashes match the worker copy;
  complete 553,283-byte command output matches the executor digest.
- Browser client exits zero. Playwright 1.56.1, Chromium
  141.0.7390.37: 1440 × 1000 light desktop and 320 × 900 dark phone.
  Keyboard/touch index and safety links, fragment focus, back/reload, outcome
  text and exact fetched guide bytes pass. No API requests or page errors.
- All eight original captures were opened with native `view_image` in the
  source turn. Warnings, all six outcome rows and uncertainty guidance remain
  readable. Both table and viewport widths are 896 px desktop / 288 px phone;
  no document overflow. Capture hashes follow.

| Capture | SHA-256 |
| --- | --- |
| `desktop-light-planning.png` | `a2c615f3ce849ad7b5d47f3a23145e6a650053b6168abb087bd7479517dfc1fd` |
| `desktop-light-guardrails.png` | `b51c4ab75118fb660f1785003fc03006ce3a6e786ea1ca6852d99de6f281f171` |
| `desktop-light-verification.png` | `ccb86bea189a978e2e91cdb56ce0eef3641cd01939a424a21de77ef6f9d97d95` |
| `desktop-light-outcome-table.png` | `82985407f343599612f2a0c2d1444b8f639f0364b0033ba0a1b553f26ab47f4e` |
| `phone-dark-planning.png` | `5de46670bdee803dfc015e9e4f392102380ea8f5ce589feee865fc5b8661638a` |
| `phone-dark-guardrails.png` | `81e051015ecfe8d35806c2245f157302fc9652117e0a44e73380d2ccdd12b7af` |
| `phone-dark-verification.png` | `db2b355a72ffd11eb70bf3a86c15a6342a182945def53e7551f363c7025ae19f` |
| `phone-dark-outcome-table.png` | `1782ab48aa71182710fbd87e7eafc1488c9dbe0b2c3fc03736ed4683c1069f0f` |

## Limits

This verifies help content and navigation, not action execution, a full Pulse
build/suite, runtime safety, installation, release availability or customer
recovery. No runtime, mode, approval or verification policy changed. Chromium
phone proof is not a WebKit pass: the earlier privacy-guide WebKit reload
failure remains uncleared with its existing owner. The earlier optional source
executor readback remains separate. No prior failed attempt was replayed.

Raw executor/client output, the exact build, browser recipe/result/captures and
per-image human-readable observations are retained with the lane outcome in
`RECEIPTS.md`, `SOURCE-LOGS.tar.xz`, `DOCS-BUILD.tar.xz`,
`BROWSER-EXECUTION.tar.xz`, `BUILD-VERIFICATION.json`,
`BROWSER-VERIFICATION.json` and `PIXEL-REVIEW.json`.
