# Proxmox setup output ownership — 15 October 2026

Scope: existing assisted PVE/PBS setup, not new platform demand or installed
recovery. The previous editor-lifetime check admitted old downloads after
endpoint/name edits and old agent output after installer TLS/mode changes.
A matching cache key could revive an earlier request after A → B → A, and
copy/download/agent preparation could overlap. The consuming state now binds
all those completions to a monotonically revised editor scope, retains the
pending action until actual completion, honours import review, and reveals
tokens only after a current copy succeeds. Issued server tokens and already
sent clipboard writes are not retractable through this UI change.

Executable proof: `frontend-modern/src/components/Settings/__tests__/useNodeModalState.setupLifecycle.test.ts`
and `NodeModalSetupGuideSection.test.tsx`. Chosen original-source comparisons
cover stale mint ABA, late downloaded script, TLS ABA and overlapping issuance
for both PVE and PBS. Additional controls cover fresh cache/download/copies,
health polling, expiry, actionable current errors, late clipboard completion,
review changes, close/reopen, resets, target changes and disposal.

Browser proof: `frontend-modern/browser-tests/setup-output-lifecycle.cjs` uses
compiled actual setup/basic-information/authentication sections, hook, API
clients, token dialog, notifications and production CSS. Synthetic responses
exercise both platforms at desktop and narrow touch viewports; the final
source-bound receipt and worker outcome contain results and actual pixel
inspection. This fixture does not install source, verify live credentials or
assert whole-appliance recovery, full-app authentication or release placement.
