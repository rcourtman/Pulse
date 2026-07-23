# Relay Runtime Contract

## Contract Metadata

```json
{
  "subsystem_id": "relay-runtime",
  "lane": "L7",
  "contract_file": "docs/release-control/v6/internal/subsystems/relay-runtime.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "api-contracts"
  ]
}
```

## Purpose

Own the desktop and mobile relay runtimes, their persisted relay state
boundaries, the server-owned mobile relay capability boundary, and the
canonical reconnect, encryption, protocol, proxy, and relay-trust behavior
for Pulse instance bridging.

## Canonical Files

1. `internal/relay/client.go`
2. `internal/relay/protocol.go`
3. `internal/config/persistence_relay.go`
4. `internal/api/relay_mobile_capability.go`
5. `internal/api/relay_mobile_capability_generated.go`
6. `internal/relay/mobile_compatibility_generated.go`
7. `docs/release-control/v6/internal/MOBILE_COMPATIBILITY_MANIFEST.json`
8. `scripts/release_control/generate_mobile_compatibility.py`
9. `scripts/release_control/mobile_compatibility.py`
10. `pulse-mobile:config/mobile-api-surface.json`
11. `pulse-mobile:src/generated/coreCompatibility.ts`
12. `pulse-pro:relay-server/bridge.go`
13. `pulse-pro:relay-server/device_store.go`
14. `pulse-pro:relay-server/main.go`
15. `pulse-pro:relay-server/metrics.go`
16. `pulse-pro:relay-server/registry.go`
17. `pulse-pro:relay-server/revocation_feed.go`

## Shared Boundaries

1. `internal/api/relay_mobile_capability.go` shared with `api-contracts`: the backend-owned Pulse Mobile relay capability inventory is both a relay runtime boundary and a canonical API payload contract surface.
2. `internal/api/relay_mobile_capability_generated.go` shared with `api-contracts`: the generated Pulse Mobile route inventory is both a relay runtime allowlist and the backend projection of the canonical mobile API contract.
3. `pulse-mobile:config/mobile-api-surface.json` shared with `api-contracts`: the Pulse Mobile consumer minimum and released-line probe inventory are both API compatibility and relay runtime boundaries.
4. `pulse-mobile:src/generated/coreCompatibility.ts` shared with `api-contracts`: the generated mobile route, pairing, and push projection is both an API consumer contract and relay runtime boundary.
5. `pulse-pro:relay-server/main.go` shared with `cloud-paid`: the Relay server startup and readiness path is both a relay-runtime server boundary and a cloud-paid entitlement invalidation boundary.
6. `pulse-pro:relay-server/registry.go` shared with `cloud-paid`: the Relay server active-session registry is both a relay-runtime connection boundary and a cloud-paid entitlement invalidation boundary.
7. `pulse-pro:relay-server/revocation_feed.go` shared with `cloud-paid`: the Relay server revocation feed is both a relay-runtime server boundary and a cloud-paid entitlement invalidation boundary.

## Extension Points

1. Add or change desktop relay reconnect, registration, drain, proxy-stream, or encrypted channel behavior through `internal/relay/`
2. Add or change relay control payload schemas, including mobile-visible push notification metadata, through `internal/relay/protocol.go`
3. Add or change persisted relay enablement, server URL, or reconnect-safe default loading through `internal/config/persistence_relay.go`
   `LoadRelayConfig` must apply environment-variable overrides
   (`PULSE_RELAY_ENABLED`, `PULSE_RELAY_SERVER`) on top of the file-loaded
   or default-fallback `relay.Config` via `relay.ApplyEnvOverrides`. The
   env overrides must distinguish unset / empty / unparseable values
   (file or default wins) from explicit `true`/`false`/valid-URL values
   (env wins), and must reject invalid `PULSE_RELAY_SERVER` URLs through
   the canonical `validateRelayServerURL` check rather than silently
   accepting a malformed override. Saving the resulting `relay.Config`
   from the UI is allowed to persist the env-effective state to disk;
   the override is not stripped before save.
4. Add or change the backend-owned mobile relay capability inventory, compatibility scope mapping, pairing contract, or push routing through `docs/release-control/v6/internal/MOBILE_COMPATIBILITY_MANIFEST.json`, then regenerate both repositories. Generated files are not extension points.
5. Keep desktop and mobile relay changes aligned with the governed server relay surfaces represented by the L7 lane evidence
6. Add or change server-side grant revocation ingestion, readiness, active-session teardown, or reconnect-token invalidation through `pulse-pro:relay-server/main.go`, `pulse-pro:relay-server/revocation_feed.go`, and `pulse-pro:relay-server/registry.go`.
7. Add or change privacy-safe server-side mobile operational observability through `pulse-pro:relay-server/bridge.go`, `pulse-pro:relay-server/device_store.go`, and `pulse-pro:relay-server/metrics.go`. Platform labels must remain bounded to `ios`, `android`, and `unknown`; user, installation, instance, and device-token identifiers must never become metric labels.

## Forbidden Paths

1. Reintroducing relay client trust or TLS handling through ad hoc dialer configuration outside the canonical relay client
2. Letting encrypted DATA frames race ahead of nonce-order guarantees by deferring inbound decrypt work to background goroutines
3. Treating a missing persisted relay config file as a hard failure instead of falling back to the canonical disabled default
4. Editing generated mobile compatibility projections directly or landing a mobile-required route/push/pairing change with a stale Pulse Mobile consumer contract

## Completion Obligations

1. Update this contract when new desktop relay runtime or persistence entry points become canonical
2. Keep relay desktop runtime changes tied to explicit proof in
   `internal/relay/client_test.go` and `internal/relay/encryption_test.go`
3. Keep persisted relay config loading tied to explicit proof in `internal/config/persistence_relay_test.go`
4. Keep backend-owned mobile capability inventory changes tied to explicit proof in `internal/api/relay_mobile_capability_test.go`
5. Keep mobile relay runtime changes tied to explicit proof in `pulse-mobile:src/relay/__tests__/`
6. Keep the operator Relay incapable of serving v6 grants until it has synchronously drained the authenticated revocation feed, and expose stale feed state through readiness.
7. Keep feed-applied restrictive events tied to proof that already-connected stale grants are disconnected and their persisted reconnect credentials are invalidated.
8. Keep exact-revision Pulse/Pulse Mobile compatibility evidence green in Canonical Governance and keep released-line compatibility green in the Pulse Mobile OTA gate.
9. Keep server-side mobile operational metric changes tied to Relay bridge, device-store, and metric contract tests. These aggregate service signals must not be presented as unique-user, retention, app-open, screen-view, or feature-usage analytics.

## Current State

The relay lane had been carrying real desktop and mobile runtime behavior
without fully explicit subsystem ownership even though L7 already treats relay
readiness as a governed release lane. This contract makes both relay-runtime
boundaries explicit.
The canonical relay runtime now includes local CA-bundle trust via
`SSL_CERT_FILE`, so self-hosted relay deployments can use a private CA without
forking the dial path or disabling TLS verification globally.
Inbound DATA handling is also part of this boundary: encrypted relay frames
must be decrypted before they are handed to background stream handlers so the
channel nonce guard stays monotonic and relay arrival order cannot trip false
replay failures under concurrency.
Relay channel key derivation is part of that same encryption boundary. The
X25519 shared secret may not feed the generic nil-salt HKDF form: desktop
relay runtime must domain-separate the channel KDF with the relay-owned salt
`pulse-relay-e2e-channel-v1` together with the existing directional info
strings before deriving AES-GCM keys.
Relay status truthfulness is part of the same contract. If the client is in a
governed reconnect or license-pause window, `ClientStatus.reconnect_in` must
reflect that delay instead of silently presenting a disconnected state with no
retry timing.
Relay push notification identity is part of this same owned contract. The
canonical `PUSH_NOTIFICATION` payload must carry the authoritative
`instance_id` when the desktop relay client serializes it so multi-pairing
mobile routing and repair flows cannot reinterpret a queued approval or
finding against whichever instance happens to be active later.
The relay HTTP proxy boundary is also part of this owned surface. Relay DATA
requests must normalize outer method/path whitespace before building the local
Pulse API request so trivial spacing drift does not cause a valid proxied
operation to fail at the desktop boundary. The same normalization rule applies
to allowlisted proxy header names, so safe headers like `Content-Type` are not
silently stripped just because the upstream frame wrapped them in outer
whitespace. That same proxy boundary also owns per-channel abuse control: one
relay channel may not consume an unbounded share of the local HTTP proxy, so
desktop relay runtime must enforce a channel-local request budget and fail
excess DATA frames with owned proxy backpressure instead of letting one
channel starve the rest of the connection.
Relay mobile observability is part of the server runtime boundary. The Relay
may expose aggregate connection outcomes, authenticated session counts,
registered push-token counts, and push delivery health by a bounded platform
label. It must not export user, installation, instance, or device-token
identifiers as metric labels, and these operational aggregates must not be
described as unique-user or behavioural analytics. The mobile client remains
free of analytics and usage-event collection.
Persisted relay config loading is part of the same owned surface. A missing
`relay.enc` file must fall back cleanly to the canonical disabled default so a
fresh v6 install or a partially migrated instance does not fail closed just
because relay settings have never been saved.
That same persistence boundary also owns relay client secrets at rest:
`relay.enc` may only accept plaintext relay config as migration input. Once
the runtime can read legacy plaintext relay settings, it must rewrite
canonical encrypted storage immediately on load instead of leaving
`instance_secret` or the relay identity private key on disk as a normal
runtime path.
The mobile relay runtime is part of the same owned surface: reconnect drain
failover hints are one-shot recovery instructions, not permanent relay URL
overrides, so a successful failover reconnect must return future reconnects to
the instance's canonical relay URL unless the server sends a fresh drain hint.
The server-owned mobile relay capability boundary is part of that same owned
surface too. The dedicated `relay:mobile:access` credential may only reach the
explicit runtime route inventory generated from
`MOBILE_COMPATIBILITY_MANIFEST.json`, and expanding that inventory is governed
L7 work rather than a router-local compatibility tweak. The same manifest now
generates mobile-visible push constants and the pairing schema projection, and
Pulse Mobile consumes its TypeScript route/push/pairing projection. Canonical
Governance compares the provider manifest with the mobile consumer declaration
at exact repository revisions and publishes that evidence; the mobile OTA gate
continues to probe stable and RC server lines with a real
`relay:mobile:access` token.
Assistant session rename is part of that explicit mobile relay runtime
inventory: `PATCH /api/ai/sessions/{session_id}` may pass through
`relay:mobile:access` with `ai:chat` scope because it mutates only the
browser-safe session title projection. The relay inventory must not widen that
route into transcript, provider-context, approval, action-execution, or raw
tool-evidence mutation authority.
The Patrol attention workbench routes are part of that same explicit
inventory: `GET /api/ai/patrol/attention`, `GET
/api/ai/patrol/attention/{item_id}`, and `POST
/api/ai/patrol/attention/{item_id}/{mutation}` supersede the legacy patrol
findings/acknowledge routes that mobile devices already consumed, so they map
`relay:mobile:access` alongside `monitoring:read` (reads) and
`monitoring:write` (lifecycle mutations), with `ai:execute` as the enumerated
legacy compatibility scope. Gating any attention route on a single
non-mobile scope severs registered phones' alert sync on server upgrade
(v6.1.0-rc.4 regression) and is a forbidden regression, not a hardening
opportunity. This grants no new mutation authority: attention mutations stay
inside the alert lifecycle acknowledge/suppress surface the legacy routes
already exposed to the mobile credential.
The route that mints that dedicated credential is also part of the paid Relay
boundary. `POST /api/security/tokens/relay-mobile` lives in the shared
auth/security router, but it must require the paid `relay` entitlement before
creating a `relay:mobile:access` token so Community installs cannot bypass
Relay/mobile gating through direct API calls.

The operator Relay revocation feed is now part of the canonical server runtime,
not optional commercial decoration. Startup requires the license-server URL and
operator feed credential, completes an authenticated feed drain before serving,
and reports stale feed state as unhealthy. Each applied license revocation,
installation revocation, or license-version floor is enforced against both new
registrations and already-connected v6 sessions. A forced disconnect also
clears the persisted reconnect credential so a Relay restart cannot restore an
invalidated session before full grant registration.

The mobile relay capability inventory and router allowlist include the
canonical pending-action read, global pending/settled action list, authoritative
action detail, action decision, and action execution routes. These core reads
expose the durable attempt/receipt correlation without moving inbox or push
deduplication into Pulse core; Pulse Mobile and pulse-pro relay consumption
remain Task 07 Phase B2 after Task 10, and no relay-local retry may compensate
for an absent or receipt-pending core action result.
Patrol approval pushes use `decide_action` with a canonical action id; terminal
pushes distinguish verified, unverified, verification-failed, and
execution-failed outcomes. Relay must not revive `/api/ai/approvals` as a live
mutation route or claim that executor completion alone verified the change.

The action routes in the relay-mobile inventory now name the granular approval
and execution scopes. `relay:mobile:access` and `ai:execute` remain explicitly
enumerated compatibility scopes only for this bounded migration window; the
central lifecycle authority still requires a current durable token owner,
matching organization and credential, current RBAC capability, and action/plan
binding. Detached relay/service tokens fail closed. A mobile-local biometric
label is not server-verifiable MFA evidence, so device-key enrollment and
cryptographic mobile step-up remain a later governed slice and relay must not
claim that local biometric success satisfied the core MFA floor.

The canonical relay notification projection accepts `ActionResultV2` and uses
wording that names execution separately from verification: not run, execution
failed, execution inconclusive, executed and confirmed, executed but
contradicted, verification not attempted, or verification inconclusive.
Every canonical notification states both execution and verification even when
one axis is failed, not run, or inconclusive, and confirmed/contradicted copy
names agent-attested versus independent evidence.
Legacy status-string notification input remains a compatibility boundary; new
workflow producers must not use it as a competing truth model. Task 11 owns
client presentation and Task 12 owns final-SHA certification.
