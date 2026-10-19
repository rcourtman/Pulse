# Issue reporting navigation — browser evidence, 19 October 2026

Containing source `8ca12c4740d3a6779f0ab1072f3d00e2a1b18633`, parent `d9b3f0c932948fb81f1969d8966041214ba419ad`. This receipt is recorded in a non-merge commit whose own parent is the containing source; verified Markdown did not change after the pass.

## Verified scope

The existing triage page has an early topic index and semantic headings. Reporting rules, form fields, original advice and existing fragments are unchanged. Eight previously unaddressable topics now have direct destinations; the existing sign-in section also gains an index link.

- Ten focused Python checks pass, including existing intake safety checks. The new navigation check rejects the exact parent and 27 deliberately missing-link, wrong-fragment or missing-heading variants. Removing only the added index and headings gives the exact parent Markdown.
- The actual Docs fetch, router, Markdown renderer, PageHeader and styles consume the current shipped Markdown. The complete 12-file repository-local consumer/configuration closure and dependency lock match the retained compiled viewer at `a631a34db2189f118ba675a4db12bbb6384be7fb`. This reuses that unchanged viewer, not a new full application build.
- Playwright 1.56.1 / Chromium 141.0.7390.37: 22 recorded cases at 1280 × 900 light desktop and 320 × 900 dark narrow, including two exact-parent baselines, 18 keyboard/touch topic traversals and two compatibility cases. All nine links resolve to their relevant headings and move focus. Back, performance/sign-in reload, same-fragment re-activation and old fragments pass.
- All requests are local GETs; no API operation is admitted. Client/protocol exit 0, no recorded page/console/request errors, browser and server closed.
- All ten retained captures were opened with the native image viewer. The parent shows uninterrupted prose; the new index and topic headings are readable at both widths, without horizontal clipping. Narrow content uses normal vertical scrolling.
- Public documentation links and all 62 shipped mirrors pass their static checks.

## Final content

| File | SHA-256 |
| --- | --- |
| `docs/ISSUE_TRIAGE.md` | `c2a07e95e29372d94ba6374701753f0ed466593e85fc955b88c79cdd46889536` |
| `frontend-modern/public/docs/ISSUE_TRIAGE.md` | `c2a07e95e29372d94ba6374701753f0ed466593e85fc955b88c79cdd46889536` |
| `scripts/tests/test_issue_triage_docs.py` | `e55d54c72b403960ffa8f369892a4421dd184700d4321e4d5af58d0aa1dd941b` |

## Captures

| File | SHA-256 |
| --- | --- |
| `desktop-baseline.jpg` | `36c5ec707cb86cb893a5436b47e989ee5a56803906176c72f2217bd735e08d65` |
| `desktop-index.jpg` | `d49975ca083d641c7fe09148f68f38324607c8dfa6c896d45749df0d1089c5e6` |
| `desktop-versions-and-installation-failures.jpg` | `6040997dfd865c8890815022ec2b876b78a061750ca59782192c29eb25f7fc01` |
| `desktop-missing-readings-and-unresponsive-workloads.jpg` | `5ca3fcab41c3d212c05b64b0f0db179b27c208f5099a2ec13d22f49c2407e960` |
| `desktop-pulse-performance-and-memory.jpg` | `8ed5aabcc70d90e452a3294520bbb491a6fac53d91f5ccfb6168a4a34e6c855c` |
| `narrow-baseline.jpg` | `197cb93564799eeada6d6f18ad47b9b6779e1af90cd3264d737feb51e5c57742` |
| `narrow-index.jpg` | `df0d3a3adf35c3ff5fa56555c385a13757310686c7d2190d0335fc3cb1199154` |
| `narrow-several-hosts-or-clusters.jpg` | `6f77cda4ee9ee7a6be13d87cbc25d66b3e7bef6421401ff24f0bcec22759613f` |
| `narrow-notification-delivery-and-identity.jpg` | `4d1e466580d5364b33b7e929d957a953169b2250191dbfdc4feecfc6f6bbbbb3` |
| `narrow-sign-in-and-access-reports.jpg` | `fd56a4233c6c03841c90800e805896d16c923649f52965d3fbdf55e7595f5e99` |

## Limits

The first browser protocol incorrectly required a bare-fragment link to store a full path in its `href`; the renderer correctly retains the fragment. That assertion failed and cleanup completed. The corrected protocol checks its resolved destination and then exercises actual navigation. No renderer or product assertion was weakened.

This verifies the existing Docs consuming surface in a fixture shell, not GitHub’s remote rendering, a screen reader session, a full-app build, installed/released availability, or any repair to SSO, backups or paid recovery. Protocol, binding, adverse result and captures are retained with the source review evidence.
