# Release-line Overview diagnosis ordering regression

Exact runtime parent: `101bae33963ee65a5a83bc229d081b43825de889`.
Backport source: `495562ef663e473889490a774a80cf8d04405732` (main).
Regression provenance: `32e620a6c36b4ce08729de747cb232dfabf5c1ba`
introduced the asynchronous delivery diagnosis projection on 26 August 2026;
it is an ancestor of this parent and v6.4.1. The unversioned completion path
is also present in v6.4.3-rc.1. This repairs an existing candidate surface,
not new product scope (RELEASE_PROMOTION_POLICY, Release Train rules 2/4).

The main runtime patch and its two regression tests apply without adaptation.
With only those tests and the browser fixture added to the exact parent,
2 tests fail and 11 pass: overlapping reads replace the current disabled
warning, and an obsolete response repopulates an empty active set. Chromium
also fails the retained-warning assertion (0 versus 1) at its first viewport.
These are local release-line reproductions, not inherited main receipts.

With the six-line runtime patch, all 13 tests pass. Another 25 adjacent
presentation and delivery-action tests pass. TypeScript (`tsc --noEmit`) exits
0. No full repository suite was run for this bounded UI backport.

Commands:

- `npm --prefix frontend-modern test -- src/features/alerts/__tests__/OverviewTab.deliverystatus.test.tsx src/features/alerts/__tests__/useAlertOverviewState.test.tsx`
- `npm --prefix frontend-modern test -- src/features/alerts/__tests__/OverviewTab.deliveryactions.test.tsx src/features/alerts/__tests__/deliveryDiagnosisPresentation.test.ts`
- `npm --prefix frontend-modern run type-check`
- `pulse-heavy-run -- node scripts/check-alert-diagnosis-ordering.mjs`

The fixture uses the real Overview component and synthetic API responses on
loopback only. It does not exercise an installed backend, actual delivery,
recipient receipt, or production routing. Latest-started ownership prevents
superseded successful reads from updating state, including empty-set
invalidation. Failed refreshes still retain the previous snapshot; no new
freshness claim or indicator is added.

This does not resolve warning-projection ordering, SQLite crashes, paid
convergence, recipient qualification or the release HOLD. No public release
or issue-resolution claim follows from this component evidence.

Patched Chromium passes at 1440, 900 and 390 pixels: warning retained, stale
dispatch absent, new alert present, text within viewport and no page errors.
Desktop and phone screenshots were visually inspected: warning and controls
remain visible. Receipt binds the runtime file hash to the exact parent.
