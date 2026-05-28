# Known RC Issue Closure For GA Late Issue Integration Record

- Date: `2026-05-28`
- Gate: `known-rc-issue-closure-for-ga`
- Issues: `#1341`, `#1429`, `#1469`, `#1472`, `#1476`, `#1481`
- Result: `fixed-local-proof`

## Context

The May 28 open-issue pass found several late reports where the right v6
outcome was not a v5-only patch or public issue state change:

- `#1476` exposed a PBS setup-script auto-registration 405 after token
  creation.
- `#1472` exposed an OIDC refresh cadence defect where five-minute access
  tokens were treated as immediately refreshable.
- `#1481` requested a configurable Ollama `keep_alive` runtime option.
- `#1429` remained open after v6 navigation clarification and agent reinstall
  feedback; the remaining v6-hardening gap was host-agent continuity across
  short/FQDN hostname drift after upgrade or reload.
- `#1469` was checked against the current branch. TrueNAS decode fixes and
  unified connection runtime summaries were already present; a regression test
  now locks the expected connection-ledger presentation.
- `#1341` received fresh reporter evidence showing a saved Ceph pool override
  at 50 percent while the agent-reported pool was above threshold and no active
  alert fired. Screenshots were inspected before treating the new evidence as
  actionable.

## Disposition

The integrated v6 fixes are:

- PBS setup scripts now post auto-registration to the Pulse base URL plus
  `/api/auto-register`, not the setup-script download URL plus that path.
- OIDC session refresh now uses a relative refresh lead based on the access
  token lifetime, capped at five minutes, with a one-minute fallback for
  legacy sessions that do not yet carry issued-at metadata.
- OIDC sessions persist access-token issued-at metadata and update it after a
  successful refresh.
- Ollama provider settings now expose and persist `ollama_keep_alive`; the
  provider sends that value as Ollama `keep_alive`, preserves numeric values
  such as `0` and `-1`, and omits the field when the operator clears it to use
  the Ollama server default.
- Host-agent continuity now treats short and fully-qualified hostnames as
  equivalent where one side is a short name and the other side is its FQDN, and
  host-token bindings alias the current hostname form after reload.
- Agent-sourced Ceph pool reports now run pool alert evaluation, reconcile
  with Proxmox API reports by FSID, and preserve old `agent:<host>` pool IDs
  as threshold aliases while emitting alerts under the normalized storage ID.
- TrueNAS connection-ledger regression coverage confirms successful runtime
  summaries do not present as `Credentials unknown` or `No activity yet`.

This record does not close any public GitHub issue by itself. Public issue
closure should still wait for normal maintainer review, release publication,
or reporter retest where the issue thread needs environment confirmation.

## Proof

- `go test ./internal/api -run 'TestPBSSetupScript_FailsClosedOnAutoRegisterSuccessDetection|TestHandleSetupScript_PBSWithAuthToken|TestHandleSetupScript_PBSTypeGeneratesScript|TestHandleCanonicalAutoRegister_PBS' -count=1`
- `go test ./internal/api -run 'Test(ShouldRefreshOIDCSessionToken|RefreshOIDCSessionTokens|CreateOIDCSession|UpdateOIDCTokens|OIDCSessionPersistence|CreateOIDCSession_NilTokenInfo|InvalidateSession|SessionStore)' -count=1`
- `go test ./internal/api -run 'TestAISettingsHandler_(UpdateSettings_OllamaKeepAlive|UpdateSettingsRejectsInvalidOllamaKeepAlive|GetAndUpdateSettings_RoundTrip)|TestContract_.*AI' -count=1`
- `go test ./internal/ai/providers ./internal/config`
- `go test ./internal/alerts -run 'Test(StorageThresholdResolutionUsesAliasIDs|ReevaluateActiveAlertsUsesSharedStorageOverrideResolution|CheckStorageOfflineUsesSharedThresholdResolution)' -count=1`
- `go test ./internal/models -run 'Test(StateUpsertCephCluster|UpdateCephClustersForInstance)' -count=1`
- `go test ./internal/monitoring -run 'TestApplyHostReport(FiresCephPoolAlertsForAgentSourcedCluster|MergesAgentCephWithProxmoxAPICluster|ReusesTokenBindingAcrossShortFQDNAfterReload)|TestRemoveHostAgentUnbindsToken|TestPollCephClusterChecksPoolStorageThresholds' -count=1`
- `npm --prefix frontend-modern test -- src/components/Settings/__tests__/AISettings.test.tsx src/components/Settings/__tests__/useConnectionsLedger.test.ts src/features/patrol/__tests__/usePatrolIntelligenceState.test.ts`
- `npm --prefix frontend-modern run type-check`
- Browser inspection of Settings > Assistant & Patrol provider configuration for the Ollama Keep Alive field.

## Remaining Boundaries

- `#1482` is treated as a v5/configuration-specific Docker-agent-as-container
  report, not as evidence that Docker-only installs should produce a host row.
- The storage-offline follow-up in `#1429` remains a separate storage
  connection-state lane and was not resolved by the host continuity hardening.
- The OneDrive video linked from `#1341` was not inspected during this batch;
  the attached screenshots and pasted `/data/alerts.json` override were enough
  to validate the Ceph alert-evaluation gap addressed here.
