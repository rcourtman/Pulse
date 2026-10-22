# Sign-in report intake — browser evidence, 19 October 2026

Containing source `905ccc5e156267c592f5087af51437242361c701`, parent `ecaabecb740f6d0c43216b132e9eab5f11680bb7`. This receipt is recorded in a non-merge commit whose own parent is the containing source. No verified form or guide text changed after the pass.

## Verified scope

Both bug forms now keep sign-in context optional, preserve an existing administrator session and distinguish discovery, authentication and permission failures without requesting another save, login or secret. The mirrored triage guide retains the same boundaries and links to the existing diagnosis and recovery paths.

- 27 focused intake, OIDC-session and public-report-containment tests pass. The new check rejects both exact parent forms and the parent guide; malformed or detached expandable sections are rejected.
- The real Docs fetch, router, Markdown renderer, PageHeader and production styles render the changed shipped triage guide. Its entire 12-file local consumer/configuration closure and dependency lock match the retained compiled viewer at `a631a34db2189f118ba675a4db12bbb6384be7fb`. Existing compiled assets are reused, not claimed as a new full application build.
- The exact YAML Markdown block preceding each optional evidence field is also rendered through that consumer as a local fixture. This checks the supplied wording and disclosure interactions, not GitHub’s remote form controls or layout. Both forms contain identical optional context.
- Playwright 1.56.1 / Chromium 141.0.7390.37 passes six journeys at 1280 × 900 light desktop and 320 × 900 dark narrow: collapsed/open/close disclosure, keyboard/touch, unrelated sections remaining collapsed, both diagnosis/recovery links, fragment focus, Back and reload.
- All requests are local and GET-only; no backend request is admitted. Browser client and protocol exit 0, no recorded page/console/request errors, browser and server closed.
- All eight retained capture files were opened natively (seven unique image hashes). Text and links wrap without horizontal clipping at both widths. Narrow content uses normal vertical scrolling; the privacy and access-preservation precautions are readable.

## Final content

| File | SHA-256 |
| --- | --- |
| `.github/ISSUE_TEMPLATE/bug_report.yml` | `a37a21ac55c151b1a6606b6f4d21d781dae7d769775260ec6e89663decc009e4` |
| `.github/ISSUE_TEMPLATE/v6_rc_feedback.yml` | `1666980c9df3ea6091905e955d97f4983eecbd3882e4001f2e0fbae277152f8a` |
| `docs/ISSUE_TRIAGE.md` | `779dd5f8aebdafc35da0067f881521d80670453093cc2ce1464d9489c615d99b` |
| `frontend-modern/public/docs/ISSUE_TRIAGE.md` | `779dd5f8aebdafc35da0067f881521d80670453093cc2ce1464d9489c615d99b` |
| `scripts/release_control/release_promotion_policy_test.py` | `b66221ab166687001eec8c3ee49aecba926057d91aceac2262f4818e89957b24` |

## Captures

| File | SHA-256 |
| --- | --- |
| `desktop-bug-context.jpg` | `6a67bf5f96d20663c20f6d7c71556f02effb9338267e269d8abbd608e3d23b75` |
| `desktop-prerelease-context.jpg` | `1a17d2f9120717f6abf88828b075b90ae09f7236af1fde4bcf991aa55fe5363f` |
| `desktop-triage-context.jpg` | `8872f919bef5e6019a7fc327f282dec66d3b18007689ec56c42aac28343213b7` |
| `narrow-bug-context.jpg` | `f8f0413c92f707cb4e25d4452c9c54e89536a76d9d9119fcb7e934fed0d2d1a2` |
| `narrow-bug-private-evidence.jpg` | `f8f0413c92f707cb4e25d4452c9c54e89536a76d9d9119fcb7e934fed0d2d1a2` |
| `narrow-prerelease-context.jpg` | `3389a85a84d0524f500dd842ecab2a8fadee6a514539ff439abf04f679b1fa1b` |
| `narrow-triage-context.jpg` | `5ce423e2cefc03354a04f975aebdf69c05705b7763c620f2e4a1680f2d389ddb` |
| `narrow-triage-safe-paths.jpg` | `d290f11b0d188927547a9fae3e8b0817dc1c3c317e81dc0455dea1d7d20a365d` |

## Limits

No identity provider, credential, save, login or encrypted persistence was exercised. This is reporting and documentation safety, not a repair for the reported OIDC save defect, a new workaround or evidence of installed/released availability. No fresh full-suite/build result is claimed. Complete protocol, binding, command limits and captures are retained with the lane outcome.
