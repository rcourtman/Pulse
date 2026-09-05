# Release-line reconnect admission regression (#1899)

Backport of runtime repair and tests from mainline `672a497076fe2c2ef216d5c2f62176bcdd28e6a7`
onto release/v6.4 base `2c4b27fc4b229bece4585c33eb3b11e74f1bdeef`.
Issue #1899 reports headings disappearing at sync reconnect on v6.4.1 (LXC);
the full issue and empty comments list were read on 5 September 2026. No attachments.

On this candidate, the backported focused tests before the runtime repair
reported 84 passed / 3 failed. The behavioural failure reproduced alert-only
recovery incorrectly resolving resource admission; the other two failures
required the new store signal/architecture. After repair all 87 passed:

```sh
npm --prefix frontend-modern test -- src/__tests__/useAppRuntimeState.test.ts src/stores/__tests__/websocket-unified.test.ts src/__tests__/App.architecture.test.ts
```

Resource snapshot receipt is now distinct from alert hydration, survives
transport reconnect and resets on organisation change. Explicit empty resource
snapshots remain authoritative. This is a regression backport, not new navigation.

The isolated browser qualification command is:

```sh
pulse-heavy-run -- env PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MOCK_MODE=true PULSE_E2E_NAVIGATION_RECOVERY=1 PULSE_E2E_LOCAL_BACKEND_PORT=18765 npm --prefix tests/integration test -- tests/96-navigation-socket-recovery.spec.ts --project=chromium
```

No mainline browser receipt is copied as candidate evidence. Published artifact,
reporter installation, long-outage and mobile More-menu interaction remain
separate qualification boundaries. The opt-in spec is not an automatic release
gate. A changed patch candidate requires the full restarted 72-hour soak under
the Release Train policy and steward qualification before promotion.

Fresh release-line browser run on 5 September: all three Chromium checks passed
(1440, 1100 and 390px), using the source-built embedded frontend and isolated Go
backend. Desktop and narrow reconnect screenshots were inspected. Navigation
survived interruption and recovered without reload; desktop incident access
retained an enabled acknowledgement control. This is not acknowledgement
persistence or provider notification receipt. The fresh browser-verification.json
binds this run to the release-line base and runtime source hashes.

Existing critical-transition and stable-identity recovery tests also passed
three race repetitions on this base with the backport applied.

Follow-up hook coverage on 5 September separates the mocked resource-snapshot
flag from general hydration. Alert-only hydration now explicitly reports
initialDataReceived=true with resourceSnapshotReceived=false; the inverse case
retains an authoritative empty resource snapshot during reconnect. All 88 tests
in the three focused files above pass. Temporarily substituting
initialDataReceived for resourceSnapshotReceived in runtimeStateResolved makes
both boundary tests fail (two failures, 21 skipped); the mutation was removed.
This verifies regression sensitivity, not installed browser or soak readiness.
