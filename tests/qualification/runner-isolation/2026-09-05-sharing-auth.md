# Sharing diagnostic authentication — 5 September 2026

Source: f5c6520b1c40184b6f2d08f35864ce4c4c9c71f3 plus the accompanying
scenario-6 cookie-session change. Main-based isolated checkout; no application
code or release image changed. All browser/container runs used pulse-heavy-run.

Fresh external search found continuing community trust concerns at
https://www.reddit.com/r/Proxmox/comments/1vnztw1/enterprise_observability_solutions/;
this is perception evidence, not a verified security defect or feature mandate.
Official https://playwright.dev/docs/auth#session-storage documents the distinction
between saved storage state and sessionStorage. Neither source substitutes for
runtime evidence.

## Results

1. Unchanged source: scenarios 1, 2, 3, 5, 7 passed; 4 skipped (disabled-feature
   scenario in enabled environment); 6 failed after switchOrg reload. Viewed
   screenshot: login screen. Server logs recorded token access denied for the
   synthetic share-source organisation. The primary-token authentication shortcut
   is inappropriate for this cross-org user-session flow; token restrictions must
   not be weakened.
2. Cookie-session change: same per-scenario verdicts, but scenario 6 passed reload,
   selected the intended organisation, rendered both pending shares, then failed
   waiting for Accept. Viewed screenshot: sharing page with admin/owner-required
   warning. This is a narrower authentication repair, NOT a passing sharing test.
3. Temporary fresh-login experiment: cleared this test context's cookies, logged
   in through the UI and asserted the sessionStorage username plus absence of API
   token. Same missing-Accept failure and warning; screenshot viewed. The missing
   sessionStorage identity-hint hypothesis did not explain the controls failure.
   That experiment was removed; the retained patch uses cookie-session auth and
   asserts no browser token and a session cookie, preserving all sharing checks.

All three runs exited 1. The two session runs each reported 5 passed, 1 failed,
1 skipped. No quarantine promotion or installed-customer/release qualification.

## Exact runtime identities

All bindings below used 127.0.0.1; ports are HTTP / agent / mock respectively.
Images were source-built e2e_runtime (not release) and mock fixtures.

| Run | Compose project | Ports |
| --- | --- | --- |
| Baseline | pulse-e2e-68b520733af5c259fef517bc4e6c82ed | 32793 / 32794 / 32792 |
| Cookie session | pulse-e2e-ce0f1ff5af240f4a2cf293ac221cc295 | 32796 / 32797 / 32795 |
| Fresh login experiment | pulse-e2e-8b1a1cf8592e465edaf0a89c593dc7ac | 32799 / 32800 / 32798 |

Baseline server: sha256:12da185a68a98fc8119c416385289676fce5af950a8b6f35b25e83ae82fcfe91

Baseline mock: sha256:6eeb7117e104c562c554356dd018a85fa60b188f708744a44a1abdbe61b556cf

Cookie server: sha256:73df3df57423896f81e49a2cc05c27e9014857cf6b564e0f255249184babfa67

Cookie mock: sha256:37395ad843b093f57ae499c3681a37c78f2a2fa63fbc8d7fe12ea2f54662ab54

Fresh login server: sha256:6d32bedb48a905c88a6466e071bc836962cb922ab54753118aa7f29e870d7843

Fresh login mock: sha256:1e6a704e28b395c206d5eca76bbfe28fa899fd0d1fcd6a6b2e6c0cea0350fd03

## Remaining

Investigate the authenticated sharing management identity/role presentation and
compare actual security-status identity with the organisation owner/membership
before changing capability checks. Do not assume granting admin is a fix.
Delayed outgoing admission, failed/superseded incoming admission and reconnect
recovery remain unqualified. Direct npm/setup/helper isolation is not established.
Worker fixture cleanup does not encompass helpers.ts's checkout-shared
`tmp/playwright-auth/shared-cookie-session.json`; it must not be described as
complete credential cleanup. Browser artifacts remain sensitive even for mocks.

## Interruption and cleanup

Sent process-group SIGTERM only to this invocation after its worker state.json
existed and the source-built stack was running. Runner exited 143.

Project: pulse-e2e-75fbc23d30c49cbb488d7ad1ec80b2bd

Loopback ports HTTP / agent / mock: 32802 / 32803 / 32801

Server: sha256:6abf38035a3a9e14015a9a7473f4e21d74a44eaab57482767c48583701daa2dc

Mock: sha256:3add9a33dd71cd4a2cb712b3e47b2cf0fb0012471995a5b5671b1229d9b24955

Label-scoped container, network and volume inventories returned empty after
teardown for all four projects. No worker auth files remained after interruption.
However, an invocation video and checkout-shared cookie-session cache remained.
Automatic complete credential/artifact cleanup is therefore NOT proved. Removed
only these four invocation report/result roots and this isolated checkout's
shared cookie cache after inspection; no other worktree or invocation touched.
This proves one process-group TERM case, not shell-only TERM, INT, HUP or SIGKILL.

Focused validation: 40 Node checks passed across managed-local-backend,
run-tests-images and run-tests-cleanup; canonical completion guard and
`git diff --check` passed. Raw browser artifacts/authentication state are not
committed. No frontend surface changed.
