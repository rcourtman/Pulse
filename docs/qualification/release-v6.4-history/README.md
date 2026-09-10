# Release-line canonical History repair

Base: a97e67e0a19382cdcff3eca0b3dd2d50063e9b01.
Selected next-preview scope: identity-only portions of reviewed main
3a189f31d44789a642b68a915d3d4a683ab81104 plus PBS Backups repair
605643b0172280c131783b3ed81dc455e28ebc71 and its landed surface formatting.

The candidate incorrectly supplies agent coordinates for Proxmox node history,
drops canonical coordinates during Node adaptation, and treats discovery routing
as explicit agent identity. Backups also excludes standalone PBS telemetry and
agent-bearing guest correlation. These are repairs of existing History surfaces,
not new product scope. Release Train candidate-fix rules apply.

Before runtime changes, adapted regressions fail: Go Proxmox family expected
node but received agent; five frontend suites report 9 failed / 92 passed.
After runtime changes the focused Go race suite passes (1.028s) and all 101
frontend tests pass (12.76s). TypeScript no-emit check passes.

Commands:
- go test -race ./internal/unifiedresources -run TestBuildMetricsTarget -count=1
- cd frontend-modern && npx vitest run src/components/Workloads/NodeDrawer.test.tsx src/components/Workloads/__tests__/nodeDrawerModel.branchcov0713.test.ts src/utils/__tests__/resourceStateAdapters.test.ts src/features/proxmox/__tests__/ProxmoxBackupServersTable.drawer.test.tsx src/features/proxmox/__tests__/ProxmoxPageSurface.contract.test.tsx
- cd frontend-modern && npx tsc --noEmit

Existing explicit-agent helper, canonical MetricsTarget type, PBS identity
correlation, and deduplicated resource model are present on this line; no other
runtime dependencies were imported. Missing disk and ambiguous identity guards
remain tested. Beta.2 contents and 9eafd31f6 remain ancestors. VERSION, release
notes, AI and metrics-store/storage-tier implementation remain unchanged.

Browser command:
pulse-heavy-run -- env PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MOCK_MODE=true PULSE_E2E_LOCAL_BACKEND_PORT=18769 npm --prefix tests/integration test -- tests/63-pbs-history-targets.spec.ts --project=chromium

Browser result: 2/2 Chromium checks passed (6.9s), at 1280×844 and 390×844, using the source-built frontend and isolated local backend. Exact vm/history-vm and agent/history-agent requests were observed, incorrect PBS targets were absent, and both CPU/memory paths plotted without disk data. Desktop VM and phone standalone-agent screenshots were visually inspected; absent network/disk series remain visibly collecting. The fixture deliberately disables WebSocket, so its reconnect indicator is expected. This fixture uses synthetic
inventory and history responses, not a reporter installation or retained-store
correctness proof. Independent review, exact beta qualification, publication
and installed History acceptance remain separate.
