# Fixed release-cadence guidance — browser evidence, 22 October 2026

## Containing source

Verified source `17d08393d0ecb2b4ffd6115307b42fce8c317f4f`. This receipt is recorded in a non-merge commit whose own parent is that exact source. Served content and consumer inputs did not change after the browser pass.

| Content/check | SHA-256 |
| --- | --- |
| `docs/AI_TRANSPARENCY.md` | `8e19299b7b0942ce42a2a6c17dc9e8818940783dd4609e14655233c9d7da51bb` |
| `docs/RELEASE_PROCESS.md` | `9dde89db7e4943fb6f3990d6380c1e2b3afe4dca0691f09de6984612bd525b31` |
| `frontend-modern/public/docs/AI_TRANSPARENCY.md` | `8e19299b7b0942ce42a2a6c17dc9e8818940783dd4609e14655233c9d7da51bb` |
| `frontend-modern/public/docs/RELEASE_PROCESS.md` | `9dde89db7e4943fb6f3990d6380c1e2b3afe4dca0691f09de6984612bd525b31` |
| `frontend-modern/src/features/docs/__tests__/issueFollowThroughGuide.test.ts` | `7fccf8ad37b4a236079cef12c01336a7878ffa44a362dd0ee6c0269d5099b258` |
| `frontend-modern/browser-tests/release-channel-help.cjs` | `cf59dfe75b0ea9892d961c253618e5fe8e58bffccc30def82a9fb025fd8b73ba` |
| `scripts/tests/test_transparency_docs.py` | `b95edd489e09d4a972401911d850389be8d4b6598f256bece7f41cf57a9dcba8` |

## Completed proof

- Offline root proof `source-wkm4kriy` passes 41 documentation tests, all 265 public Markdown files and all 62 shipped mirrors. Two exact assigned-parent guides are rejected by the new schedule oracle. Both client and command exit zero, output collection is complete and the guest stopped.
- Offline frontend proof `source-yjo5ksbt` passes 356 Docs/link regressions in 38 files, type-check, all lint stages, formatting, production build, bundle budgets and the real Docs consuming build. Both exits are zero with complete output and stopped guest.
- The consuming build binds all 3,244 source inputs and 35 artifacts. Changed Markdown is read by the actual Docs page, Markdown renderer, router, PageHeader and styles; only the outer shell is a fixture. Exact input/artifact hashes were independently checked before the browser pass.
- Playwright 1.56.1 matches the integration lock. Chromium 141.0.7390.37 at 1365 × 1000 light desktop and WebKit 26.0 at 320 × 1000 dark touch pass direct fragment focus and keyboard/touch disclosure-to-release navigation in the current shell. Text assertions preserve weekly target, beta inclusion, stable separation, failed-release distinction and unchanged candidate qualification/soak. No page overflow, console/page errors, external requests or backend/mutation requests; browsers and local servers closed.
- All eight captures below were opened natively with `view_image`. Cadence and beta access are readable in both profiles. First-pass heading captures missed the final changed soak paragraph; the two supplementary same-source captures show it fully. The two original desktop release captures are identical, not different states.

| Capture | SHA-256 |
| --- | --- |
| `desktop-triage-cycle.png` | `84c01f57f6da196ec7b756d892c8055482386d01de7c0ff146483c24e71a70e3` |
| `desktop-candidate-cut.png` | `3db97635e29e8404ed97425b4fcf7d04ddf5f80f3f73e9c8593a596e9d64acae` |
| `desktop-soak-boundary.png` | `3db97635e29e8404ed97425b4fcf7d04ddf5f80f3f73e9c8593a596e9d64acae` |
| `narrow-triage-cycle.png` | `00451b9aff87eadfb949bdc2f8476fea24279646d3fc74458eebefa8a0d8d3d6` |
| `narrow-candidate-cut.png` | `8c9d7a37bd4a52d3b994f289dfe0bfc9e8e71718626ab960e2351c9c7054072a` |
| `narrow-soak-boundary.png` | `a91ba445f6be0f52ce6b0737b26ca76a35195ae583294c74290d3e223c0758ba` |
| `desktop-soak-exception.png` | `c0eb52e7efc51e66dd103edb5d16a38f2ca587c8c63b093b6ec97c1fe1e74536` |
| `narrow-soak-exception.png` | `e6850db52bf9e28c325153acfd8907e13049a06de2dda44c1b982efb8069ceaa` |

## Limits and retained corrections

This verifies release guidance, not operation of the release train, release availability, installed updates or any reporter's recovery. No release action was performed. The separate stopped Docs Back/reload investigation was neither replayed nor cleared; the older general browser script retains that coverage but was not executed here.

The canonical-tree formatter file was unreadable locally (`EACCES`); there was no alternate read. A single assertion was manually wrapped in a separate commit and exact offline formatting passes. Root stdout initially normalised Git progress CR bytes to LF in the worker's text capture; a fixed progress-line reconciliation matches the executor's byte count and SHA-256 exactly. Both forms and the original receipt remain retained; no test was rerun for output collection. Frontend stdout matches the executor's raw output directly.

Detailed commands, output/cleanup receipts, original and supplementary browser scripts/results, bindings, source archive, captures and pixel observations are retained in the lane outcome's RECEIPTS.md, SOURCE-PROOF.json and VALIDATION.tar.xz. Protected review, containing CI and publication remain separate.
