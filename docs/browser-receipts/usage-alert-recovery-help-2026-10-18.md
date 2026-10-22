# Usage-alert recovery help — browser verification, 18 October 2026

## Containing source

The verified producer is `41f7c5caed45d6ea1aba6e529d1698bec86cc801`,
parent `767fc5b73d5650f4bf92662f9dd46d3ea990794a`. This receipt is recorded
in a non-merge commit whose own parent is that producer. No served content
changed after the pass.

| Content | SHA-256 |
| --- | --- |
| `docs/TROUBLESHOOTING.md` | `fb2bd5ac4831f6d995a641ad5cf20eadd154ab71a2c3523f4f17bc1933c838e0` |
| `frontend-modern/public/docs/TROUBLESHOOTING.md` | `fb2bd5ac4831f6d995a641ad5cf20eadd154ab71a2c3523f4f17bc1933c838e0` |
| `scripts/check-alert-recovery-help.cjs` | `d3ec28fab93831014126ca5550f5911c74780e504bde4d3ddb1a3e61238f67ab` |
| `scripts/tests/test_alert_recovery_help.py` | `c98f235d98f487d8305aa71c59f4ba98a03e43fd31ea43b6b908ce1d858f0739` |

## Completed checks

- Five help regression checks and 121 existing troubleshooting checks pass.
  The five new checks reject the original guide, which lacks this symptom path.
  All 62 shipped document mirrors and 259 public Markdown documents pass checks.
- The real Docs page, router, Markdown renderer, PageHeader and production CSS
  were compiled at the producer: 2,996 source-input hashes and 29 artifact hashes
  match. The rendered Markdown and linked Configuration/guest-disk assets match
  their source bytes. The surrounding shell alone is synthetic.
- Playwright 1.56.1, Chromium 141.0.7390.37: 1440 × 1000 light desktop and
  320 × 900 dark phone. Keyboard/touch symptom and cross-document links,
  fragment focus, Back and reload pass without failed HTTP or page errors.
  No backend request, guest probe, backup or notification operation was used.
- All eight captures were opened with native `view_image`. The changed steps,
  filesystem examples, clear-level example, uncertainty/reporting paragraph and
  safety warning are readable across the desktop and narrow captures. The linked
  table wraps within the narrow viewport. No horizontal clipping was observed.
  Browser and local server cleanup completed.

| Capture | SHA-256 |
| --- | --- |
| `desktop-light-identity.png` | `509c31fb9038df52095473030d53807c73d8a98cd5e624eed5857d6693c28b73` |
| `desktop-light-clear-level.png` | `9a76f2aaf70887f679889d026a72e36e46e646a6056f20b1566a2cdbf6016359` |
| `desktop-light-safety.png` | `016712353bab99e80603bba42cba90425e8458cabfe130c14619bfa202c080df` |
| `desktop-light-linked-thresholds.png` | `10453812479f857bc3c4d7619f6123c1476e93ada2a2ec65d06635678e6613e3` |
| `phone-dark-identity.png` | `d00472ab14065d76699578305c68933057c2a75d588f975e23fa79b862c764e7` |
| `phone-dark-clear-level.png` | `2266b3d27ebd256ea774ee27eb1d6202294d76fb65d2012f8ec911a479243571` |
| `phone-dark-safety.png` | `cda414de79c905f461622add0ea317996b6a71a9497538fc8ddcfecdd6cb24da` |
| `phone-dark-linked-thresholds.png` | `b67e32db6df5f09e16a1f726e41826d08c085dcc5c11d3a503c9281ac4b9d87d` |

## Limits

This verifies help content and navigation, not the reported defect's cause,
threshold-engine recovery, an installation, delivery to a recipient or release
availability. It does not establish resolution of issue #2068. Chromium narrow
proof is not WebKit proof. No runtime or alert policy changed.

Complete source/build/browser receipts, raw output, preview and captures are
retained with the lane outcome in `RECEIPTS.md`, `SOURCE-PROOFS.tar.xz`,
`BUILD-PREVIEW.tgz`, `BROWSER-EXECUTION.tar.xz`, `BROWSER-VERIFICATION.json`
and `PIXEL-REVIEW.json`.
