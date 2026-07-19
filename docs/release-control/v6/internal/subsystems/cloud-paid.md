# Cloud Paid Contract

## Contract Metadata

```json
{
  "subsystem_id": "cloud-paid",
  "lane": "L3",
  "contract_file": "docs/release-control/v6/internal/subsystems/cloud-paid.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own cloud plan/version semantics, entitlement limits, hosted billing/runtime
agreement, the Pulse Cloud control plane, provider-hosted MSP account
bootstrap/licensing, hosted tenant lifecycle, and cloud-specific enforcement
rules.

The subsystem also owns the canonical commercial offer and transition
boundary across Community, Relay, Pro, Cloud, and MSP. One versioned offer
contract must project into public pricing, Pulse Account, in-product plan
presentation, checkout, the read-only Stripe catalog audit, support policy,
license-server billing state, and runtime entitlements. Stripe remains billing
truth, while one Pulse-owned idempotent transition authority must atomically
project authoritative Stripe subscription snapshots into the local billing
contract, continuity epoch, entitlement state, license version, transition
history, and grant-revocation outbox. Checkout, webhook, reconciliation,
refund, support/admin, and future account transition paths must not implement
independent entitlement mutation rules.

Pulse Pro commercial Pulse Intelligence value reporting is a cloud-paid proof
surface. It must compare full-loop, approved-execution-loop, Assistant-loop,
Assistant-resolved-loop, external-agent-loop, external-agent-resolved-loop,
Patrol-resolution, resolved-operations-loop, Patrol-guided resolved-loop,
Patrol-control completed-loop, Patrol-control resolved-loop, paid Patrol-control
completed-loop, paid Patrol-control resolved-loop, legacy Pro activation
compatibility mirrors, MCP adapter
operations-loop, MCP adapter approved-execution-loop, MCP adapter
resolved-operations-loop, and
MCP/external-agent capability-class cohorts against the generic Assistant chat
baseline with activation, retention, paid-rate, free-start conversion, and
signal-to-paid deltas.
The Patrol-guided resolved-loop cohort is the first-party completion signal:
it requires the `pulse_patrol` operations-loop starter marker plus resolved
operations-loop proof, so Patrol starter interest is not mistaken for a
completed activation loop.
The Patrol-control completed-loop cohort is a first-class telemetry signal for
historical conversion and retention analysis. It requires the Patrol control
starter or its legacy Pro activation entry-point alias, Patrol issue evidence,
contextual Assistant or external-agent collaboration, and either a rejected
governed decision or an approved governed decision with verified outcome proof,
so a deliberate operator rejection can complete the decision loop without being
misreported as an approved-and-resolved outcome. Local self-hosted Pro
operations-status presentation must apply the same distinction through the
shared operations-loop `patrolControlValueState`, falling back to
`patrolAutonomyValueState` and then `proActivationValueProofState` only for
compatibility: `governed_decision_recorded` remains partial decision status
with a continue path, legacy `verified_needs_mcp` is treated as verified
first-party Patrol value with MCP readiness left as optional external-agent
context, and `verified` may be presented as a verified Patrol operations
outcome or suppress a new starter marker. Paid-plan status rows must describe
missing or partial Pro action entitlements as Patrol investigation/remediation
capability, not as generic `Pro operations capability`, so the commercial
surface tells users which governed Patrol behavior is affected.
The Patrol-control resolved-loop cohort is a first-class telemetry signal for
paid Patrol control value, not a report-time intersection of generic starter
interest and generic resolved-loop proof. Pulse emits it only when a Patrol
control or legacy Pro activation starter, Patrol issue evidence, contextual
Assistant or external-agent collaboration, an approved governed decision, and
verified outcome proof are present, and the license-server value report must
consume that explicit primary flag.
The paid Patrol-control completed-loop and resolved-loop cohorts are also
first-class telemetry signals. Pulse emits them only when the current coarse
`paid_license` posture coexists with the corresponding Patrol-control
completed-loop or resolved-loop proof in the same telemetry payload. Legacy Pro
activation completed/resolved and paid cohort fields may mirror those primary
Patrol-control values for continuity. The value report must use those explicit
booleans for paid-rate, retention, and free-to-paid comparisons, and must not
infer paid Patrol-control value from exact tiers, account links, checkout
sessions, license IDs, or a report-time join against customer identity.
Approved-execution-loop
metrics mean Patrol issue evidence plus Assistant governed-context, direct
external-agent, or MCP collaboration plus at least one approved governed-action
attempt; complete-loop metrics mean Patrol issue evidence plus contextual collaboration plus an
approved or rejected governed-action decision signal. Those metrics must remain
separate from broader governed-action activity so action plans and approval
requests are not mistaken for completed operations-loop proof. Capability-class signal-to-paid
metrics must use the first time that specific capability was observed while
the install was still free, then count conversion only when paid activation
happened after that capability signal; paid installs that first use a
capability after purchase must not be counted as capability-driven conversion
proof.

Provider-hosted MSP bootstrap output is part of the cloud-paid operator
contract. It must carry the resolved MSP plan version, plan source, workspace
limit, and validated provider MSP license id/email so installability proof can
distinguish signed license-backed activation from local environment fallback
configuration.
Provider-hosted MSP status uses the same tenant registry, health summary, and
stuck-provisioning threshold as the control-plane cleanup loop, so operator
readiness reports and automated failure handling cannot drift into separate
definitions of a failed provider workspace.
Provider-hosted MSP recovery uses the same license-backed provider identity,
workspace registry, tenant data, and canonical tenant-runtime rollout path as
the rest of the control plane so failed, stuck, or unhealthy client workspaces
are repaired without introducing a second provisioning model.
Provider-hosted MSP backup, verification, and restore are part of the cloud-paid
operator contract because the recovery artifact must preserve the provider's
license-backed plan identity, control-plane account/workspace registry, and
tenant-local runtime state without depending on Stripe billing surfaces.
Provider-hosted MSP tenant runtime isolation is part of the cloud-paid runtime
contract. `CP_DOCKER_NETWORK` names the provider ingress and support network,
not the client runtime boundary: each client workspace runtime must be created
on a derived per-tenant Docker bridge, and support containers must be explicitly
attached to that bridge before the tenant runtime is started.
Provider-hosted MSP control-plane host access is part of the same runtime
contract. The internet-facing control plane must not receive broad host-root or
Docker-data read mounts for storage admission checks; it may stat only narrow
operator-owned marker directories on the target filesystems. Docker daemon
access must be routed through the packaged socket proxy rather than a raw
read-write socket mount, and audit/rate-limit client IP recovery must honor
forwarded headers only from configured trusted provider proxy CIDRs.
Provider-hosted MSP report branding is a tenant runtime configuration
contract, not control-plane report generation. The control plane may accept
provider-default report brand environment values and pass them into each tenant
container as generic `PULSE_REPORT_PROVIDER_BRAND_*` runtime configuration, but
the tenant Pulse runtime owns report rendering, per-workspace override loading,
scheduled report cadence/delivery, generated output retention, and the
`white_label` entitlement gate. This keeps provider-hosted MSP Stripe-free and
avoids a cloud-control-plane report data path across clients.

## Canonical Files

1. `pkg/licensing/features.go`
2. `pkg/licensing/billing_state_normalization.go`
3. `pkg/licensing/database_source.go`
4. `pkg/licensing/evaluator.go`
5. `pkg/licensing/models.go`
6. `pkg/licensing/activation_types.go`
7. `pkg/licensing/token_source.go`
8. `pkg/licensing/entitlement_payload.go`
9. `pkg/licensing/hosted_subscription.go`
10. `pkg/licensing/dev_mode_features.go`
11. `pkg/licensing/service.go`
12. `pkg/licensing/grant_refresh.go`
13. `pkg/licensing/installation_status_poll.go`
14. `pkg/licensing/license_server_client.go`
15. `pkg/licensing/persistence.go`
16. `pkg/licensing/activation_store.go`
17. `pkg/licensing/trial_activation.go`
18. `pkg/licensing/purchase_return.go`
19. `pkg/licensing/stripe_subscription.go`
20. `pkg/licensing/monitored_system_limit.go`
21. `internal/cloudcp/account/audit.go`
22. `internal/cloudcp/account/handlers.go`
23. `internal/cloudcp/account/tenant_handlers.go`
24. `internal/cloudcp/auth/handlers.go`
25. `internal/cloudcp/auth/session.go`
26. `internal/cloudcp/config.go`
27. `internal/cloudcp/entitlements/service.go`
28. `internal/cloudcp/portal/handlers.go`
29. `internal/cloudcp/portal/page.go`
    29a. `internal/cloudcp/portal/setup_facts.go`
30. `internal/cloudcp/public_cloud_signup_handlers.go`
31. `internal/cloudcp/registry/models.go`
32. `internal/cloudcp/registry/registry.go`
33. `internal/cloudcp/routes.go`
34. `internal/cloudcp/stripe/provisioner.go`
35. `internal/hosted/provisioner.go`
36. `frontend-modern/src/App.tsx`
37. `frontend-modern/src/AppLayout.tsx`
    37a. `frontend-modern/src/components/CommercialMigrationBanner.tsx`
38. `frontend-modern/src/useAppRuntimeState.ts`
39. `frontend-modern/src/components/Settings/BillingAdminPanel.tsx`
40. `frontend-modern/src/components/Settings/BillingAdminOrganizationsTable.tsx`
41. `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx`
42. `frontend-modern/src/components/Settings/OrganizationBillingLoadingState.tsx`
43. `frontend-modern/src/components/Settings/MonitoredSystemImpactPreview.tsx`
44. `frontend-modern/src/components/Settings/ProLicensePanel.tsx`
45. `frontend-modern/src/components/Settings/ProLicensePlanSection.tsx`
46. `frontend-modern/src/components/Settings/CommercialBillingSections.tsx`
47. `frontend-modern/src/components/Settings/SelfHostedCommercialRecoverySection.tsx`
48. `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`
49. `frontend-modern/src/components/Settings/RelayPairingSection.tsx`
50. `frontend-modern/src/components/Settings/useBillingAdminPanelState.ts`
51. `frontend-modern/src/components/Settings/useOrganizationBillingPanelState.ts`
52. `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
53. `frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts`
54. `frontend-modern/src/pages/PricingHandoff.tsx`
55. `frontend-modern/src/utils/apiClient.ts`
56. `frontend-modern/src/utils/cloudPlans.ts`
57. `frontend-modern/src/utils/commercialBillingModel.ts`
58. `frontend-modern/src/utils/licensePresentation.ts`
59. `frontend-modern/src/utils/monitoredSystemPresentation.ts`
60. `frontend-modern/src/utils/pricingHandoff.ts`
61. `frontend-modern/src/utils/selfHostedPlans.ts`
62. `frontend-modern/src/utils/upgradePresentation.ts`
63. `frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`
64. `pkg/licensing/trial_activation_public_key_override_dev.go`
65. `pkg/licensing/trial_activation_public_key_override_release.go`
66. `pkg/licensing/testing_helpers.go`
67. `pkg/licensing/self_hosted_feature_catalog.go`
68. `frontend-modern/src/utils/selfHostedFeatureCatalog.generated.ts`
69. `internal/cloudcp/server.go`, `internal/cloudcp/authz.go`, `internal/cloudcp/commercial_identity.go`, `internal/cloudcp/security.go`
70. `internal/cloudcp/health_monitor.go`, `internal/cloudcp/health_stuck_provisioning.go`, `internal/cloudcp/tenant_state_metrics.go`, `internal/cloudcp/ratelimit.go`
    81a. `internal/cloudcp/proxytrust/client_ip.go`
71. `internal/cloudcp/hosted_entitlement_handlers.go`, `internal/cloudcp/url_helpers.go`
72. `internal/cloudcp/admin/handlers.go`, `internal/cloudcp/admin/status.go`, `internal/cloudcp/auditlog/auditlog.go`
73. `internal/cloudcp/cpmetrics/metrics.go`, `internal/cloudcp/cpsec/nonce.go`, `internal/cloudcp/static_assets.go`, `internal/cloudcp/favicon.svg`
74. `internal/cloudcp/email/sender.go`, `internal/cloudcp/email/templates.go`
75. `internal/cloudcp/handoff/handler.go`, `internal/cloudcp/handoff/handoff.go`
76. `internal/cloudcp/stripe/grace_enforcer.go`, `internal/cloudcp/stripe/helpers.go`, `internal/cloudcp/stripe/reconciler.go`, `internal/cloudcp/stripe/webhook.go`
77. `internal/hosted/hosted_metrics.go`, `internal/hosted/reaper.go`
78. `internal/cloudcp/public_msp_signup_handlers.go`
79. `internal/cloudcp/provider_msp_bootstrap.go`
80. `internal/cloudcp/provider_msp_backup.go`
81. `internal/cloudcp/provider_msp_recovery.go`
82. `pulse-pro:relay-server/main.go`
83. `pulse-pro:relay-server/registry.go`
84. `pulse-pro:relay-server/revocation_feed.go`
85. `pulse-pro:landing-page/index.html`
86. `pulse-pro:scripts/validate_stripe_catalog.py`
87. `pulse-pro:scripts/remediate_stripe_commercial_state.py`
88. `pulse-pro:license-server/README.md`
89. `pulse-pro:license-server/entrypoint.sh.template`
90. `pulse-pro:license-server/secrets.env.template`

## Shared Boundaries

1. `frontend-modern/src/components/CommercialMigrationBanner.tsx` shared with `frontend-primitives`: the global commercial migration notice is both a cloud-paid entitlement recovery surface and a shared app-shell notice primitive consumer.
   The banner's commercial migration predicates, copy, retry polling, dismissal
   state, and billing-plan destination remain cloud-paid owned, while the
   visible banner shell, icon, action slot, and dismiss chrome must compose the
   shared `InlineNotice` primitive instead of carrying local paid-state tone
   classes, raw SVGs, or page-local buttons.
   Pulse Mobile pairing inside `frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts`
   is also a commercial relay surface only for entitlement and operator
   presentation. When the backend returns `409 onboarding_not_ready`, the
   settings surface must preserve API-owned diagnostics and clean up the
   temporary server-minted mobile token; it must not downgrade that response
   into a generic paid-state prompt or create browser-owned relay credentials.
   Relay presentation on the settings surface is value-first: Remote Access
   panel and pairing copy must state the user job (alert push notifications
   and remote monitoring through the Pulse Mobile app with no inbound ports or
   VPN), name iOS and Android early access, and state that download-page
   install links appear only after activation-key entry, rather than
   describing relay transport mechanics alone.
2. `frontend-modern/src/components/Settings/MonitoredSystemImpactPreview.tsx` shared with `agent-lifecycle`: the monitored-system impact preview is both a platform-connections lifecycle surface and a canonical cloud-paid monitored-system presentation boundary.
3. `frontend-modern/src/useAppRuntimeState.ts` shared with `performance-and-scalability`: the authenticated app runtime bootstrap is both a hosted commercial org-context boundary and a protected app-shell performance boundary.
   The app runtime may use `ssoSessionDisplayName` from security status for
   visible signed-in chrome, but hosted/commercial organization context must
   remain bound to the stable authenticated principal carried separately in
   `ssoSessionUsername` and backend request context. Display labels, contact
   emails, and SSO name claims must not become hosted owner/member identity,
   billing authority, entitlement state, or organization bootstrap source of
   truth.
4. `internal/api/licensing_bridge.go` shared with `api-contracts`: commercial licensing bridge handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
5. `internal/api/licensing_handlers.go` shared with `api-contracts`: commercial licensing handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
   That same shared licensing boundary also owns installation-version and
   runtime-build continuity for authenticated v6 installs: `internal/api/router.go`
   and `internal/api/licensing_handlers.go` must hand the canonical process
   version and runtime identity into `pkg/licensing/service.go` and
   `pkg/licensing/grant_refresh.go`, and cloud-paid transport must send those
   values on activate, legacy exchange, installation status, and grant refresh
   instead of inferring install version or paid-runtime status from browser
   state, dev build metadata, public image tags, or outbound usage telemetry.
   Active self-hosted paid entitlements must also treat runtime identity as a
   first-class product state. A paid Pro, Pro Annual, Pro+, lifetime, or
   enterprise entitlement on a non-Pro or missing runtime identity must render
   the private Pulse Pro runtime handoff in the local Plans surface rather than
   presenting as fully healthy. License-server support/admin payloads must
   preserve the raw runtime build and expose a normalized `pro`, `community`,
   or `unknown` status so paid-runtime support triage does not depend on
   interpreting Docker tags, public release names, or customer screenshots.
   The same activation state is also the credential source for the compiled
   Pro binary's in-app self-update through the license-gated download broker:
   that consumer is read-only over the activation snapshot (installation
   token, instance fingerprint, license server URL) and must not mutate
   licensing state, extend entitlements, or act as an alternate activation
   path. Keeping the Pro runtime updatable in place is part of the
   paid-runtime posture; the community self-update flow must never be the
   default answer for an installed Pro runtime.
   That same shared licensing boundary also owns paid-migration degradation
   visibility and recovery. A persisted v5 license that exists but cannot be
   read or decrypted must publish a terminal `commercial_migration` state
   (`persisted_license_unreadable`) instead of downgrading to Community
   behind a log line, and startup legacy-exchange failures classified as
   pending must self-retry in the background with backoff for the life of
   the process so a transient license-server or DNS failure at first boot
   never strands a paying upgrader on Community until a manual restart.
   Signature-valid v5 JWTs that are expired beyond the v5 grace window but
   correspond to a newer retrievable server-side key or live entitlement must
   classify as terminal stale-key recovery (`exchange_stale_key` with
   `retrieve_current_key`) rather than generic invalid-key failure; malformed,
   signature-invalid, and truly lapsed keys must keep the generic terminal
   rejection path. Transport-level legacy-exchange failures must preserve the
   first continuous failure timestamp on `commercial_migration.first_failed_at`
   and, after 24 hours, keep retrying while surfacing
   `exchange_connectivity_required` with the outbound
   `license.pulserelay.pro` connectivity policy instead of rendering ordinary
   pending copy forever.
6. `internal/api/licensing_legacy_retry.go` shared with `api-contracts`: the background legacy-exchange retry loop carries both API payload contract and cloud-paid entitlement boundary ownership.
7. `internal/api/payments_webhook_handlers.go` shared with `api-contracts`: commercial payment webhook handlers carry both API payload contract and cloud-paid billing boundary ownership.
8. `internal/api/public_signup_handlers.go` shared with `api-contracts`: hosted signup handlers carry both API payload contract and cloud-paid hosted provisioning boundary ownership.
   That shared monitored-system presentation boundary also owns disabled
   provider-connection copy. Commercial entitlement surfaces must treat canonical
   zero-delta and removal-only TrueNAS or VMware previews as non-consuming or
   capacity-freeing changes rather than warning users that a disabled connection
   still grows monitored-system usage.
   VMware vSphere network inventory is product navigation and resource context,
   not a separate commercial unit. The `/vmware/networks` route may display
   API-native network rows and those rows may contribute to connection
   previews, but cloud-paid surfaces must continue to meter only the governed
   monitored-system grouping result. Network child-resource volume must not
   become a hosted usage cap, upgrade prompt, or billing-admission condition.
   That same shared signup boundary also owns the public privacy floor:
   syntactically valid `/api/public/signup` requests resolve to one uniform
   `202 Accepted` Pulse Account response whether provisioning/email side
   effects ran or were suppressed by owner-email throttling.
9. `internal/cloudcp/auth/magiclink.go` shared with `security-privacy`: control-plane magic-link HMAC handling is both a Pulse Cloud account-access boundary and a security/privacy token-secrecy boundary.
10. `internal/cloudcp/auth/magiclink_store.go` shared with `security-privacy`: control-plane magic-link persistence is both a Pulse Cloud account-access boundary and a security/privacy storage-hardening boundary.
11. `internal/cloudcp/docker/labels.go` shared with `deployment-installability`: hosted tenant Docker labels are both a Pulse Cloud runtime contract boundary and a deployment-installability rollout boundary.
12. `internal/cloudcp/docker/manager.go` shared with `deployment-installability`: hosted tenant container management is both a Pulse Cloud runtime contract boundary and a deployment-installability rollout boundary.
    Hosted tenant container creation must also bound Docker `json-file` logs
    through the control-plane Docker manager so tenant runtime logging cannot
    fill the live Pulse Cloud host independently of tenant data quotas.
    Hosted tenant container creation must also stamp tenant identity into the
    runtime environment: the Docker manager resolves the workspace display name
    from the tenant registry at container-create time and injects
    `PULSE_TENANT_NAME` alongside `PULSE_TENANT_ID`, so alert webhook payloads
    from hosted client runtimes carry a human-readable workspace label instead
    of falling back to the tenant ID. Display-name changes after creation apply
    on the next tenant runtime rollout, which recreates the container with
    freshly resolved environment.
    Provider-hosted MSP and Pulse-hosted tenant creation must also prepare or
    fail closed on the configured tenant runtime image before mutating the
    workspace into a state that expects a live runtime container; readiness
    checks must surface missing Docker daemon, network, image, and storage
    prerequisites as operator-facing failures.
    Hosted checkout and MSP workspace provisioning must also pass the
    control-plane storage admission guard before tenant/account mutation: root
    filesystem, tenant data, Docker runtime store, and Docker build-cache
    thresholds are part of the Cloud paid readiness contract rather than an
    operator-only cleanup script.
    The same cloud audit contract must fail on stale proof/canary account rows
    and paid hosted entitlements whose tenant rows are missing, because either
    residue can recreate or mask hosted runtime state after a cleanup.
    Disposable proof-account cleanup must use the control-plane registry as the
    source of truth rather than ad hoc SQL: account deletion must refuse accounts
    that still own tenant rows, remove account-owned membership, invitation, and
    Stripe account metadata in one registry transaction, and leave user identity
    rows intact unless a separately governed identity-retention rule says
    otherwise.
    The live production host must run that cloud audit through the private
    Pulse Pro operations bundle on a recurring systemd timer, write durable
    status/log output, and emit Prometheus textfile metrics so a clean GA
    baseline is continuously monitored rather than manually rediscovered.
    Provider-hosted MSP runtime isolation must keep client tenant runtimes off
    the shared provider ingress/support network. The Docker manager must derive
    a per-client tenant bridge, label it with tenant identity, pin Traefik
    routing to that tenant bridge, attach only provider support containers
    selected by provider MSP labels, start the tenant runtime only after those
    support attachments succeed, prefer the tenant bridge for health checks, and
    remove tenant bridges when the owning runtime is removed.
    Tenant runtime containers must also start with the provider-hosted escape
    hardening defaults owned by the Docker manager: no-new-privileges, dropped
    Linux capabilities unless explicitly reintroduced by a governed need,
    Docker's default seccomp profile still active, a read-only root filesystem,
    and bounded writable tmpfs mounts only for runtime scratch paths. These
    defaults are part of the client isolation contract, not optional compose
    decoration.
    Tenant runtime containers must also start as the rootless Pulse runtime user
    instead of entering the image's root-owned chown and `su-exec` branch. The
    Docker manager owns host-side preparation of the tenant data directory,
    including recursive ownership alignment to the configured tenant UID/GID and
    strict permissions on tenant key files, so `CapDrop: ["ALL"]` and
    read-only root filesystems remain compatible with first workspace startup.
    Client-bound proof must cover the boundary itself: workspace limits must not
    be raceable past the licensed cap, handoff tokens must not be replayed or
    retargeted across workspaces, org-bound agent install/report tokens must not
    write into another client runtime, and rotated-out install tokens must fail
    immediately.
    Provider-default report branding is part of the same tenant environment
    contract: `internal/cloudcp/docker/manager.go` may inject
    `PULSE_REPORT_PROVIDER_BRAND_DISPLAY_NAME`, logo path/data, and logo format
    into each tenant container, but it must not collect report data or render
    PDFs in the control plane. Tenant-local reporting, tenant-local schedules,
    and tenant-local licensing decide whether that configured brand appears and
    how recurring report delivery runs.
    `pulse_hosted_msp` is the Pulse-operated form of the same Stripe-free MSP
    control-plane family, not the public Pulse-hosted SaaS checkout path. It
    must share the license-backed MSP plan source, workspace limit policy,
    disabled public signup and Stripe billing routes, isolated tenant runtime
    networks, tenant-local report branding, and provider MSP operator commands
    with `provider_hosted_msp` while preserving its own control-plane mode value
    for status, backup manifests, and operational audit.
13. `internal/cloudcp/provider_msp_backup.go` shared with `deployment-installability`: provider-hosted MSP backup is both a cloud-paid license/account/runtime continuity boundary and a deployment-installability recovery artifact boundary.
    License-backed provider MSP backups must include the signed MSP license
    file as a recovery artifact while exposing only license metadata in command
    output, restore must recover that license as an explicit operator artifact,
    and the archive must stay Stripe-free so provider-hosted MSP recovery does
    not inherit Pulse-hosted SaaS billing assumptions.
14. `internal/cloudcp/provider_msp_recovery.go` shared with `deployment-installability`: provider-hosted MSP failed-workspace recovery is both a cloud-paid license/account/runtime continuity boundary and a deployment-installability recovery artifact boundary.
    Provider-hosted MSP recovery must require the signed provider MSP license
    source by default, preserve the client workspace boundary, refuse to start
    from empty tenant data, and mark a workspace active only after the canonical
    tenant-runtime rollout path has produced a healthy runtime.
15. `internal/cloudcp/tenant_runtime_rollout.go` shared with `deployment-installability`: hosted tenant runtime rollout is both a Pulse Cloud runtime contract boundary and a deployment-installability release-rollout boundary.
    Hosted tenant runtime reconciliation must treat a registered tenant with
    preserved tenant data but no live Docker runtime as a recoverable managed
    state, not as a terminal skip. The control-plane-owned reconcile path must
    recreate the canonical tenant container, health-check it, and persist the
    new runtime identity before hosted billing/auth surfaces are considered
    coherent.
    Tenant runtime rollout and missing-runtime restore must fail closed on the
    same storage admission guard before snapshotting or swapping containers.
    Provider-hosted MSP rollout and recovery must preserve the same isolated
    tenant network contract when recreating, reconciling, or rolling tenant
    containers; a recovered client workspace must not fall back to the shared
    provider ingress/support network as its runtime network.
    `tenant-runtime rollout --all --image <target>` is the canonical hosted
    fleet image upgrade path for active tenant runtimes. Its `--dry-run` mode
    must print the target-image rollout plan before mutation, `--all` must
    select active tenants only, and apply must still use the canonical
    per-tenant snapshot, health-check, and rollback path. `tenant-runtime
reconcile --all` remains contract/routing drift repair for each tenant's
    current image line, not an image-line upgrade path.
16. `pulse-pro:relay-server/main.go` shared with `relay-runtime`: the Relay server startup and readiness path is both a relay-runtime server boundary and a cloud-paid entitlement invalidation boundary.
17. `pulse-pro:relay-server/registry.go` shared with `relay-runtime`: the Relay server active-session registry is both a relay-runtime connection boundary and a cloud-paid entitlement invalidation boundary.
18. `pulse-pro:relay-server/revocation_feed.go` shared with `relay-runtime`: the Relay server revocation feed is both a relay-runtime server boundary and a cloud-paid entitlement invalidation boundary.

The real `pulse-pro` license-server legacy checkout issuance, recurring
renewals, manual issue, and legacy exchange flows are part of that same
cloud-paid continuity boundary as well. Those flows must resolve grandfathered
recurring continuity from explicit active pre-cutover state, not from bare
legacy plan IDs, stale historical tokens, or issue timestamps alone.
The canonical operator preview for that cutover now lives in
`pulse-pro/scripts/grandfathered_recurring_cutover_preview.py`, and it must use
the same active-at-snapshot rule as the live server before anyone sets
`PULSE_LICENSE_GRANDFATHERED_RECURRING_SNAPSHOT_AT`.
The retired server-side checkout funnel analytics, alerting, digest
generation, and admin summary payloads are no longer part of the active v6
proof floor. `pulse-pro:license-server/v6_checkout.go` may accept legacy
funnel-telemetry fields only as ignored compatibility input for older static
site deploys; active paid-feature proof must stay on checkout creation,
portal-handoff state, pricing/catalog truth, download delivery, and license or
relay entitlement behavior rather than requiring the removed funnel monitoring
files.
Pulse Intelligence paid-value evidence is a separate private admin aggregate,
not a revival of those retired funnel stores. The `pulse-pro` license server
may expose `/v1/admin/pulse-intelligence/value` and the matching admin-console
strip only as coarse install cohorts over outbound usage telemetry pings: paid
install rate, free-to-paid conversion, returning-install retention, and whether
the full Patrol -> contextual collaboration -> governed action loop, the
stricter Patrol -> contextual collaboration -> approved execution loop, and
the approved-action-success and adapter-specific Pulse MCP loop variants are
active, including the stricter resolved operations loop split by first-party
Assistant collaboration, external-agent collaboration, and Pulse MCP adapter
use.
The same aggregate may expose an `operations_funnel` object that keeps the
loop proof readable as a staged anonymous cohort sequence: loop configured,
Patrol activity, Assistant or external-agent collaboration, governed action,
approved execution, approved action success, Patrol resolution, resolved
operations loop, Assistant resolved loop, external-agent resolved loop, active
30-day use, retained installs, Patrol-control completed-loop and resolved-loop
proof, paid Patrol-control cohorts, and full-loop free-to-paid conversion compared with generic
Assistant-only chat. That funnel is a projection over the same coarse telemetry
counters and value cohorts; it must not introduce per-install drilldowns or raw
commercial account links. Source-specific guided starter cohorts may split the
same anonymous operations-loop starter count into Assistant, Patrol, primary
Patrol-control, legacy Pro activation entry-point, and Pulse MCP sources so
conversion reporting can tell which entry point led to loop entry, but those cohorts
remain starter evidence only and must not be counted as completed
operations-loop, approved execution, or resolved-loop proof.
Patrol-control completed-loop and resolved-loop funnel stages must come from
the explicit content-free Patrol-control booleans; resolved proof may satisfy
completed proof, but a rejected terminal completed loop must never be counted as
resolved proof. Legacy Pro activation completed/resolved booleans may be
retained only as mirrors for historical cohort continuity.
The self-hosted Pro plan surface may consume the authenticated, content-free
`GET /api/agent/operations-loop/status` projection only to choose the primary
Patrol handoff in the current-plan summary: set Patrol control when no work is
active, continue unfinished Patrol work, or review a pending governed decision.
The plan capability details disclosure must stay entitlement/runtime
diagnostics only. It must not add a `Patrol work`, started,
decision-recorded, verified, external-agent readiness, or compatibility
operations-loop status row from Patrol starter/completed/resolved counts,
legacy `proActivation*` aliases, legacy `patrolAutonomy*` mirrors, shared
status `nextAction`, or shared status `progressLabel`; those fields are
Patrol-page context and telemetry, not paid-plan detail copy. The plan surface
must not expose prompts, accounts, resource names, finding IDs, action IDs,
commands, token metadata, or any per-install drilldown, and lack of that
projection must not block entitlement or plan rendering. The visible Pro plan
card must keep the paid user's first-party task clear: choose Patrol mode and
let Patrol handle infrastructure work. The first-party commercial copy owns
the product framing: stale activation-loop, operations-loop, or MCP setup
progress text from the projection must not become the visible paid CTA when
the correct next step is Patrol control or Patrol work.
Commercial, plan, and activation-success copy must use `Choose Patrol mode`
for the paid operator boundary and frame it as setting how autonomous Patrol
should be. Self-hosted Community copy should state the features available on
that install, such as watch-only Patrol, instead of explaining unavailable
fix classes. Pro plan cards may describe hands-on Patrol modes and verified
fixes, but they must not revive activation-loop, MCP readiness, or internal
proof vocabulary as the paid user's default setup story; route-backed anchors,
telemetry, and compatibility wire fields may retain stable names such as
`patrol_control` and `patrolControl*`.
Routine self-hosted Plans and capability-status copy must describe what is
available on the instance and route recovery as `refresh the plan` or `open
recovery`; it must not describe normal setup as activation or expose raw
entitlement-payload wording to the operator.
That external-agent readiness boolean means the manifest-owned Pulse MCP
operations-loop contract is available and one non-expired API token covers the
full published operations-loop scope set; a partial MCP token is not paid value
proof, missing MCP readiness must not downgrade verified Patrol value, and it
must not become an activation/setup step on the default plan surface.
The completed-loop count consumed here is the API-owned terminal-loop proof: it
requires Patrol control starter evidence, Patrol issue evidence, contextual
Assistant or external-agent collaboration, and either a rejected governed
decision or an approved governed decision with verified outcome proof.
The resolved-loop count consumed here is already the API-owned full-loop proof:
it requires Patrol control starter evidence, Patrol issue evidence, contextual
Assistant or external-agent collaboration, an approved governed decision, and a
verified outcome in the same evidence window.
The steady-state current-plan action may use that same first-party Patrol
control projection to label the Patrol handoff as start, continue, or review,
but it must still route to the Patrol-owned Patrol control anchor rather than
creating a separate commercial activation state machine. Legacy Pro activation
evidence alone must leave the action as a fresh Patrol control handoff.
Start and continue paid Patrol control handoffs from
`frontend-modern/src/utils/licensePresentation.ts` must compose the
route-backed Patrol control URL from
`frontend-modern/src/routing/resourceLinks.ts`; the settings panel must not
write the starter marker from a local click handler before navigation, and its
presentation model must not carry a `recordActivity` switch for that handoff.
Patrol owns consuming that route flag and writing the content-free
`patrol_control` marker; `patrol_autonomy` and `pulse_pro_activation` remain
accepted only as legacy route/activity aliases. Once the action has become verified review,
`licensePresentation.ts` must route to the plain Patrol control anchor and
keep marker recording disabled.
When that action has become a completed-loop review, it must not write another
Patrol control starter marker; starter telemetry remains for start/continue
entry into the guided journey rather than for post-completion review clicks.
The value report may also expose a `value_driver_evidence` summary derived only
from those same anonymous cohort deltas. That summary may state whether the
available evidence is insufficient, positive on paid conversion, positive on
activation or retention, or has no positive delta against the generic
Assistant-only baseline; it must not claim causality, store per-install
drilldowns, or introduce a second analytics data source outside telemetry
pings.
That report must not expose install IDs, customer emails, prompts, finding IDs,
resource IDs, or action payloads, and generic Assistant-only chat must remain a
separate cohort so it cannot be counted as proof that the proactive operations
loop is driving paid value.
That same public-pricing boundary also owns the machine-readable v6 pricing
validator and launch checklist in `pulse-pro:scripts/validate_public_pricing_model.py`
and `pulse-pro:V6_LAUNCH_CHECKLIST.md`. Those operator surfaces must describe
self-hosted Pulse as core monitoring included by default, must not reintroduce
monitored-system upsell language, and must verify the no-cap release posture
rather than legacy Community limit enforcement.
Self-hosted paid-feature claims must also remain covered by the executable
cross-repo proof bundle in `scripts/release_control/paid_feature_claims_proof.py`.
When Community, Relay, or Pro claims change, that proof must continue to verify
the public customer-facing copy, canonical licensing matrix, runtime history
entitlement enforcement, frontend plan presentation, public
pricing/checkout/download behavior, Pro admin-extra runtime behavior, and
Relay entitlement acceptance together rather than treating copy, checkout, and
runtime gating as separate unlinked claims.
Uncapped self-hosted plans must also remain uncapped before and after legacy
activation continuity is applied. A captured monitored-system continuity floor
may preserve capacity for capped grants, but it must not be written back as a
raw monitored-system cap for Community, Relay, Pro, Pro+, annual Pro, lifetime,
or other self-hosted uncapped continuity plans.

## Extension Points

The authenticated app shell exposes `/actions` as a global utility route on
desktop and responsive navigation. `App.tsx` and `AppLayout.tsx` only provide
route/navigation ownership; action authority and result truth remain in the
unified-resources and api-contracts projections. Navigation proof is owned by
the App/AppLayout, routing, and desktop Actions journey tests.

1. Add or change limits through `pkg/licensing/`
2. Add or change hosted entitlement issuance through `internal/cloudcp/entitlements/service.go`
   Hosted entitlement refresh is scoped to active workspace rows only. A tenant
   in any non-`active` lifecycle state must return an inactive hosted entitlement
   result even when the owning account's Stripe subscription is still active, so
   soft-deleted or suspended workspaces cannot keep refreshing runtime leases.
   Hosted entitlement refresh must parse `stripe_accounts.subscription_state`
   as the normalized stored vocabulary (`active`, `trial`, `past_due`,
   `canceled`, and migration-tolerated terminal states), not as raw Stripe API
   statuses such as `trialing`. Raw Stripe status mapping remains limited to
   webhook/provisioner ingress; stored-row reads must use the licensing-owned
   stored-state parser so active trials do not fail closed during lease refresh.
3. Add or change control-plane plan storage through `internal/cloudcp/registry/models.go` and `internal/cloudcp/registry/registry.go`
4. Add or change MSP account-scoped workspace provisioning entry handlers through `internal/cloudcp/account/tenant_handlers.go`
5. Add or change public cloud self-serve signup price configuration,
   tenant-runtime capacity/log retention configuration, or checkout gating
   through `internal/cloudcp/config.go` and
   `internal/cloudcp/public_cloud_signup_handlers.go`
   Public MSP signup is the Pulse-operated account front door for the same
   boundary and lives in `internal/cloudcp/public_msp_signup_handlers.go`.
   Starter is the only tier that may expose checkout when the MSP rollout gate
   is deliberately opened; Growth, Scale, and Enterprise remain request-based
   even when their Stripe prices are configured for assisted/operator-controlled
   workflows. MSP price configuration
   (`CP_MSP_STARTER_PRICE_ID`, `CP_MSP_GROWTH_PRICE_ID`,
   `CP_MSP_SCALE_PRICE_ID`) is still validated in `internal/cloudcp/config.go`
   against the canonical `msp_starter`, `msp_growth`, and `msp_scale` plan
   versions. The canonical MSP product model is one provider account managing
   many isolated client workspaces. A client workspace is the hard monitoring
   boundary: agent credentials, ingest, dashboards, alert routes, reports,
   users, audit state, and tenant-local runtime data belong to the client
   workspace, not to a shared provider monitoring namespace. The MSP account
   portal is the provider control plane and handoff surface; it must not become
   a cross-client monitoring console or mint tenant-local agent credentials.
   Public MSP pricing covers the isolated-client MSP product model, not a
   promise that a provider-hosted stack is already self-serve. Provider-hosted
   MSP is the intended default launch direction because the MSP runs the
   control-plane/runtime stack in their own cloud or infrastructure, but it is
   gated on a packaged provider artifact that a technical MSP can actually
   operate. That artifact must deliberately remove the SaaS billing/public
   signup plane and keep only the provider essentials: client-workspace roster,
   per-client runtime provisioning, account-to-workspace handoff, provider
   access management, and workspace-bound install tokens. Pulse-hosted MSP is
   the same isolated-client workspace model operated by Pulse, but it is not
   part of the default public launch motion and must remain request-only until
   tenant isolation, ingest, reporting, operations, and support load are proven.
   The provider-hosted implementation mode is
   `CP_CONTROL_PLANE_MODE=provider_hosted_msp`. In that mode the control plane
   is Stripe-free by construction: it must not register Stripe webhook or portal
   billing routes, must reject Stripe API keys, Stripe webhook secrets, MSP
   Stripe price IDs, and public signup enablement, and must enforce the MSP
   client-workspace cap from `CP_PROVIDER_MSP_PLAN_VERSION` using the canonical
   `msp_starter`, `msp_growth`, and `msp_scale` plan versions. Provider-hosted
   portal bootstrap/accounts may have no hosted billing record, and that absence
   must remove Pulse-hosted Billing navigation, support actions, and role copy
   rather than exposing dead Billing controls.
   Shared-process multi-tenant mode (`PULSE_MULTI_TENANT_ENABLED` plus
   `FeatureMultiTenant`) remains an Enterprise/internal-organization capability;
   it is not the canonical MSP route for separate customer businesses and must
   not leak MSP terminology, controls, prompts, or setup requirements into
   ordinary self-hosted Pulse.
   The canonical provider workspace limits are 5 client workspaces for Starter,
   15 for Growth, and 40 for Scale; the canonical public monthly prices are
   $149, $249, and $399 respectively. Public copy may publish all three prices,
   but must keep Starter as gated checkout rather than a general self-serve
   provider-hosted purchase path, and describe Growth and Scale as
   request-assisted access rather than implying immediate self-serve
   provisioning. Larger providers belong to Enterprise/custom terms.
   Design-partner discounts do not create a public tier or change runtime
   limits: a qualified MSP lead may receive assisted Growth access at the
   Starter price for a limited 6-12 month field-feedback window, with up to 15
   client workspaces and no self-serve checkout exposure. The MSP signup routes
   stay gated behind the same `PublicCloudSignupEnabled` flag as the individual
   cloud signup front door.
   Checkout metadata produced by the MSP front door must mark
   `account_kind=msp`, `signup_source=public_msp_signup`, and
   `checkout_billing_mode=immediate` so provisioning seeds an operator
   workspace as an immediately active paid account rather than an individual
   trial tenant. Individual public Cloud signup may keep using the hosted trial
   checkout helper; MSP public signup must not set Stripe `trial_period_days`.
   Hosted Cloud/MSP tenant runtime may be hibernated for cost control while the
   default launch motion stays provider-hosted. Hibernation may scale down or
   pause tenant containers, demo/proof tenants, tenant-only workers, and
   nonessential hosted observability, but it must preserve snapshots/backups,
   DNS/config/secrets references, and a documented wake path. It must not
   disable the commercial/account backbone: public site, license server,
   checkout/webhook fulfillment, license retrieval, refund/data-request
   fallbacks, Relay/mobile infrastructure for active users, or Pulse Account
   account-recovery surfaces. This policy records the allowed cost-control
   posture only; any live infrastructure shutdown still requires an explicit
   target inventory, rollback/wake runbook, and operator approval.
6. Add or change the hosted account portal API, Pulse Account access/auth/session handling, task-first browser shell, maintained portal frontend/bundle, or account-scoped workspace/access/billing handoff through `internal/cloudcp/account/audit.go`, `internal/cloudcp/account/handlers.go`, `internal/cloudcp/auth/handlers.go`, `internal/cloudcp/auth/session.go`, `internal/cloudcp/portal/`, and `internal/cloudcp/routes.go`
   That same customer-entry boundary owns the canonical hosted Cloud handoff:
   public Cloud entry, secure checkout return, and returning-customer sign-in
   must converge on Pulse Account. Public Cloud signup stays canonical at
   `/cloud/signup`, returning commercial magic-link requests from hosted signup
   must target `portal`, and hosted checkout provisioning must issue a
   portal-targeted magic link carrying tenant identity so the first
   authenticated landing is Pulse Account rather than a tenant-runtime-only
   redirect.
   Hosted tenant identity created from checkout, Pulse Account, or handoff
   must use the control-plane registry's stable `User.ID` as the tenant org
   owner/member principal. Stripe customer email and SSO email are contact and
   delivery metadata only; they may be used for idempotent lookup or legacy
   fallback, but they must not become the primary hosted tenant user key when a
   stable account user ID exists.
   Hosted workspace provisioning with an owner email must create or resolve
   the account owner registry user before tenant org seeding. The Stripe
   provisioner must fail closed instead of synthesizing `OwnerUserID` or member
   `UserID` from an email when no stable registry user exists.
   Public hosted signup must also repair older duplicate-signup rows at the
   provisioning boundary: if the existing org owner principal is blank or
   email-shaped, the hosted provisioner must generate a stable `u_...` owner,
   rewrite owner/member metadata, and grant the tenant admin role before
   returning the existing org for magic-link delivery.
   Hosted magic links are a delivery mechanism inside that same boundary:
   verification must re-read the tenant organization and session as the stored
   stable owner/member principal, so an old token cannot keep email as the
   runtime identity or survive removal from tenant membership.
   Stripe checkout webhook magic-link delivery must use that same organization
   resolver before sending the best-effort sign-in link; a webhook may activate
   billing for a server-linked org, but it must not send a sign-in link merely
   because Stripe `customer_email` matches contact metadata with no stored
   principal.
   Tenant-targeted hosted magic-link handoff must fail closed when the
   control-plane registry cannot resolve a stable account `User.ID`; it must
   not sign a tenant handoff with a blank subject and let the tenant runtime
   promote contact email into identity.
   That same portal boundary also owns the signed-in shell shape: hosted
   arrivals default to `Workspaces`, self-hosted-only arrivals default to
   `Billing`, and the shell destinations are limited to `Workspaces`,
   `Access`, `Billing`, and `Support` rather than a separate `Overview` or
   `Summary` destination. The top of `Workspaces` may surface one quiet inline
   facts line plus one next-action row before the list, but it must not turn
   that summary into a second overview deck, metric grid, or competing shell.
   Pulse Account workspace handoff is the provider control-plane path into a
   client workspace, not a parallel monitoring surface. Workspace rows and
   selected-workspace panels may offer task handoffs such as `Open workspace`,
   `Install agents`, and reporting, but those actions must stay POST handoffs
   through the account tenant route. Any task target must be sanitized and
   signed into the control-plane handoff token as a tenant-local path, then
   sanitized again by the tenant exchange before redirect. Agent install
   commands, alerts, and performance reports remain owned by the target tenant
   runtime; Pulse Account may deep-link to those tenant surfaces but must not
   mint workspace agent credentials or render cross-client monitoring state in
   the account portal.
   Pulse Account may surface tenant-local active alert rollups as counts and
   age labels from read-only setup facts so providers can prioritize the
   workspace list, but it must not become an alert console or expose alert
   bodies, remediation state, acknowledgements, or cross-client alert streams.
   Those setup facts must read report schedule counts from the client runtime's
   org-scoped `report_schedules.json` store and active-alert counts from the
   org-scoped `alerts/active-alerts.json` runtime file before falling back to a
   legacy tenant-root active-alert file, because tenant monitors own
   org-scoped runtime persistence.
   Pulse Account also owns the provider-facing setup progression for client
   workspaces: after workspace creation the portal should select the created
   workspace, reveal the setup job, and preserve workspace/target context in
   install or reporting handoffs so an MSP can continue the first-run flow
   without guessing where to go next. That setup progression is control-plane
   guidance, not a second monitoring plane. Workspace readiness must be derived
   from read-only tenant-local setup facts without creating tenant config,
   encryption keys, migrations, agent credentials, alert routes, or report
   schedules as a side effect. The provider shell may show an unfinished-setup
   queue, disabled-output diagnostics, workspace-scoped primary actions, and
   provider setup templates for MSP accounts, but those templates are guidance
   rather than configuration. `Ready` requires at least one reporting agent, one
   enabled alert route, and one enabled report schedule; a failed latest health
   check remains `Review` ahead of setup counts, and critical alert rollups
   outrank generic health/setup review in provider attention ordering. Local MSP
   onboarding previews should be scenario-backed portal bootstrap data, not
   static screenshots, so they stay grounded in the real portal shape as the
   bundle changes.
   Hosted provider workspaces may store agent install tokens in the tenant
   runtime root token store rather than the org-specific config directory.
   Portal setup facts must count only root tokens whose `OrgID` or `OrgIDs`
   matches the workspace tenant ID, combine them with org-local setup facts,
   and never let one client's root token make another client look configured.
   MSP account surfaces may call account workspaces `clients`, but that is a
   customer-facing portal vocabulary choice over the same tenant/workspace
   lifecycle. Mixed-account Pulse Account sessions must label account surfaces
   clearly enough that MSP client wording, Cloud workspace wording, access role
   explanations, and lifecycle controls do not bleed across accounts. Read-only
   account members must not see destructive client lifecycle controls. Repeated
   customer hostnames are acceptable only because agent ingest and monitors stay
   tenant-isolated; an MSP flow that lets two clients report `pve1` must keep
   those reports in separate tenant workspaces rather than centralizing host
   identity in the provider account.
   Portal workspace payloads are live-workspace surfaces, not registry history
   dumps: tenants in `deleting` or `deleted` state must stay hidden from the
   browser bootstrap, `/api/portal/dashboard`, and workspace-detail API so
   soft-deleted runtime cleanup preserves billing/account continuity without
   presenting retired hosted workspaces as actionable customer state.
   Pulse Account self-hosted upgrade and pricing-preview copy must describe
   Community as included core monitoring plus paid Relay/Pro extras. It must
   not frame self-hosted checkout as buying unlimited monitoring or monitored-
   system volume.
   Relay pricing presentation should be annual-first at the existing public
   $39/year price, with $4.99/month as the secondary option, so Relay reads as
   an optional support/convenience tier rather than a pressure upgrade. Relay
   copy must describe secure remote access to the Pulse web UI, Pulse Mobile
   pairing for handoff, push notifications, and 14-day history; it must not imply that the
   native mobile app is a full monitoring dashboard until that product surface
   exists.
   Portal sessions and sign-in delivery are part of this boundary. Session
   lifetime is configured through `CP_SESSION_TTL` and flows through the
   control-plane auth service (`SetSessionTTL`/`SessionTTLOrDefault`); portal
   session issuance sites must use the service value rather than the package
   constant so provider-hosted MSP control planes can default to 7 days while
   Pulse-hosted control planes stay at 12 hours.
   The portal bootstrap payload carries `email_sign_in_available` (false when
   no transactional email provider is configured) and `provider_hosted_mode`
   (true for `provider_hosted_msp` control planes). The signed-out portal must
   not promise an emailed sign-in link when `email_sign_in_available` is
   false; it must instead present the operator host command
   (`provider-msp portal-link --email ...`), and the Access invite panel must
   disclose that invitation emails are not sent in that state. The
   `provider-msp portal-link` CLI mints one-time portal links only for an
   existing account member or a pending invitee, never for arbitrary
   addresses.
   Workspace-limit rejections from tenant creation must be JSON payloads
   (`error=workspace_limit_reached` with `message`, `current`, and `limit`)
   so the portal can present the licence reason instead of a generic failure;
   the portal API client must not drop non-JSON error bodies.
7. Add or change Stripe provisioning plan resolution through
   `internal/cloudcp/stripe/provisioner.go`, `internal/cloudcp/stripe/webhook.go`,
   and `pkg/licensing/stripe_subscription.go`. Checkout-session provisioning
   must be explicitly classified as hosted before tenant creation: `cloud_*`
   and `msp_*` plan versions may provision hosted runtime, public Cloud signup
   source may provision only when the plan is otherwise unresolved for staging
   compatibility, and self-hosted v5/v6 checkout metadata must be ignored by
   the Cloud control-plane webhook even if Stripe delivers the event there.
8. Add or change activation/grant lifecycle, release build helper gating, or dev-mode capability widening through `pkg/licensing/dev_mode_features.go`, `pkg/licensing/service.go`, `pkg/licensing/testing_helpers.go`, `pkg/licensing/grant_refresh.go`, and `pkg/licensing/installation_status_poll.go`
9. Add or change license-server transport through `pkg/licensing/license_server_client.go`
   That transport boundary must use HTTPS for non-loopback commercial endpoints,
   may allow plaintext only on direct loopback development targets, and must
   route outbound license-server fetches through the restricted HTTP client so
   DNS rebinding or private-network drift cannot silently reopen the boundary.
10. Add or change encrypted activation persistence through `pkg/licensing/persistence.go` and `pkg/licensing/activation_store.go`
    That persistence boundary must encrypt new state only with the persistent
    random key file and the current HKDF-based derivation. Machine ID may
    remain only as a compatibility loader for previously saved state, and
    legacy derivations must never become the write path again.
    The compatibility loader must keep every key material a real v5.0.x
    install could have sealed `license.enc` with: the raw machine-id file
    content exactly as read (untrimmed, trailing newline included), the
    hostname fallback, and the fixed v5 dev fallback, each tried only through
    the legacy sha256 derivation on decrypt. Licenses activated on v5.0.0
    predate `.license-key` and are never re-saved, so dropping or trimming
    any of those materials silently downgrades paying upgraders to Community.
11. Add or change hosted trial or self-hosted purchase-return token semantics
    through `pkg/licensing/trial_activation.go` and
    `pkg/licensing/purchase_return.go`
12. Add or change hosted signup provisioning through `internal/hosted/provisioner.go`
    That provisioning boundary owns owner-email idempotency for public hosted
    signup. Repeated signup requests for the same owner email must resolve to
    the existing tenant record instead of creating parallel orgs for one
    account identity.
    The idempotency key remains the contact email only at the provisioning
    edge. New public hosted signup orgs must generate an opaque stable owner
    user ID, and registry-backed hosted paths must resolve a registry user ID,
    before writing hosted org `OwnerUserID` or member `UserID`; owner/member
    email fields preserve delivery and display contact metadata only.
13. Add or change hosted billing-admin presentation through `frontend-modern/src/components/Settings/BillingAdminPanel.tsx`, `frontend-modern/src/components/Settings/BillingAdminOrganizationsTable.tsx`, and `frontend-modern/src/components/Settings/useBillingAdminPanelState.ts`
    Hosted billing-admin refresh semantics, availability gating, and billing
    table state stay cloud-paid owned, while the refresh button chrome must
    compose the frontend-primitives `Button` family instead of carrying a
    hosted-billing-local compact action shell.
    Hosted billing admin organization row actions follow the same ownership
    split: cloud-paid owns Suspend, Activate, Reload, tenant billing state, and
    mutation semantics, while frontend-primitives owns the shared `Button`
    chrome used by those row actions.
14. Add or change shared commercial plan/usage presentation through `frontend-modern/src/components/Settings/CommercialBillingSections.tsx` and `frontend-modern/src/utils/commercialBillingModel.ts`
15. Add or change organization billing and usage presentation through `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx`, `frontend-modern/src/components/Settings/OrganizationBillingLoadingState.tsx`, and `frontend-modern/src/components/Settings/useOrganizationBillingPanelState.ts`
16. Add or change self-hosted Pro plan, recovery, and entitlement actions through `frontend-modern/src/components/Settings/ProLicensePanel.tsx`, `frontend-modern/src/components/Settings/ProLicensePlanSection.tsx`, `frontend-modern/src/components/Settings/SelfHostedCommercialRecoverySection.tsx`, and `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
    Ordinary self-hosted trial acquisition is retired: these plan surfaces may
    show active historical `subscription_state=trial` entitlement state, but
    they must not turn `trial_eligible`, `trial_eligibility_reason`, or an
    expired trial marker into a default Pro CTA or banner.
    Plan action links with commercial upgrade destinations must compose
    frontend-primitives' `UpgradeButtonLink`; non-upgrade external commercial
    URLs must compose `ButtonLink`. `ProLicensePlanSection.tsx` must not
    recreate local anchor button shells for purchase returns, private-runtime
    actions, or plan-comparison CTAs.
    Self-hosted plan refresh, entitlement retry, activation, and clear-license
    actions are cloud-paid behaviors, but their button chrome must compose the
    frontend-primitives `Button` family instead of carrying Pro-license-local
    primary, secondary, or warning action class strings.
    The self-hosted Pro plan and activation-success surfaces must keep Patrol
    actions tied to current operator work: set Patrol control when no work is
    active, open Patrol for current work, or review a pending governed
    decision. Self-hosted Pro plan status and readiness payloads are support
    diagnostics: the default Plan surface must keep the current plan and Patrol
    mode action primary, while capability details render behind a disclosure
    for troubleshooting unavailable controls or missing capabilities. That
    disclosure must not duplicate Patrol-page work progress, decisions,
    verified outcomes, or external-agent setup readiness.
    Completed verified Patrol outcomes are history/telemetry and must not
    appear as a default `Patrol history` CTA or a `Patrol work` verified
    status row in the Plan surface. Frontend presentation models must call
    this a Patrol control action/status boundary; `operations-loop` wording is
    only acceptable at backend API or telemetry compatibility edges.
    The Plan surface may fetch Patrol operator status from the compatibility
    `/api/agent/operations-loop/status` endpoint only after the current
    self-hosted plan resolves to a Pro-family tier through
    `getSelfHostedPlanDefinitionForBillingTier`. Community and Relay plans
    must render their included capabilities without loading paid Patrol
    operator-status plumbing.
17. Add or change paid relay settings and pairing presentation through `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`, `frontend-modern/src/components/Settings/RelayPairingSection.tsx`, and `frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts`. The retired Dashboard shell must not be restored to carry a Relay onboarding card or equivalent blanket upsell — relay discovery stays inside its owning settings surface.
    Public demo and other read-only presentation policy states must suppress
    relay setup and upsell onboarding instead of inviting pairing or commercial
    action from a governed non-manageable surface.
    Relay settings must also translate internal registration-token failures
    into an operator-actionable activation-required state. Customer-facing
    Relay settings must not render raw `register:` or license-token-provider
    diagnostics from the relay client as the primary status message.
    Relay settings and public commercial docs must also keep Remote Access
    positioned as a Relay-and-higher capability, not a Pro-only feature, so the
    Relay tier remains a tangible standalone paid product.
18. Add contract tests where runtime and pricing need to stay aligned
19. Add or change hosted browser org-context bootstrap through `frontend-modern/src/App.tsx`, `frontend-modern/src/AppLayout.tsx`, `frontend-modern/src/useAppRuntimeState.ts`, and `frontend-modern/src/utils/apiClient.ts`
    The shared app shell may mount the deployment-installability-owned
    post-update release highlights card next to existing global banners, but
    that card must remain independent of plan, entitlement, organization, and
    hosted bootstrap state. It must not turn release communication into a
    commercial prompt or add another capability probe to the shell.
    That same hosted bootstrap boundary also owns the runtime-capability JSON
    shape that the app shell consumes before it decides whether organization
    chrome and multi-tenant routes exist. `pkg/licensing/entitlement_payload.go`
    must preserve empty `capabilities` and `limits` as JSON arrays, and
    `frontend-modern/src/useAppRuntimeState.ts` plus the shared license API
    adapter must treat those collections as canonical arrays rather than
    letting `null` collapse hosted browser bootstrap into a free-tier fallback
    that hides organization settings or discards a valid org context.
    Organization membership changes that arrive through the self-hosted
    invitation flow must therefore refresh org bootstrap through the shared
    `organizations_changed` app-shell path instead of forking a second hosted
    org bootstrap or pricing-aware shell reload.
    SSO display labels consumed during that same security-status bootstrap must
    reuse the existing `/api/security/status` response and must not add another
    hosted organization or commercial posture probe before the protected state
    bootstrap.
    App-shell navigation rendered by `frontend-modern/src/AppLayout.tsx` must
    also keep decorative icon titles out of tab accessible names, so hosted and
    self-hosted chrome announce the canonical tab label plus meaningful badge
    counts rather than duplicating branded SVG titles. The Patrol-owned utility
    destination remains `Patrol` over the same `/patrol` route; it must not
    alter hosted org bootstrap, entitlement loading, or commercial posture
    loading. A Patrol open-work badge in that shell may consume only the
    existing Patrol findings and live-approval read model after authentication;
    it must not read hosted billing state, trigger commercial-posture loading,
    affect organization visibility, or become an upgrade/acquisition cue.
    The same primary platform navigation must remember the last in-tab route
    state per platform, including query and hash, so route-owned filters such
    as Proxmox workload status survive switching to another platform tab and
    back. That memory is chrome route state only: it must validate the route
    still belongs to the selected platform before reuse and must not become
    hosted org bootstrap, entitlement, billing, acquisition, or cross-platform
    query state.
    The same AppLayout shell may contextualize the closed Pulse Assistant
    launcher around the current monitoring, Patrol, Alerts, or Settings route,
    but that launcher must remain a local product affordance. It must not read
    hosted billing state, alter org bootstrap, or become a commercial
    activation prompt while attaching current-view Assistant context.
    Retired operator top-level route compatibility in
    `frontend-modern/src/App.tsx` must stay inside authenticated app-owned
    surfaces such as Infrastructure, Workloads, Storage, and Recovery. Those
    compatibility routes must not revive hosted signup, public Cloud trial,
    pricing, or commercial acquisition surfaces inside the local product
    runtime.
20. Keep the hosted account portal shell task-first and compact. Section
    headers, billing action rows, and the maintained portal bundle under
    `internal/cloudcp/portal/` may surface the facts an operator needs, but the
    shell should not spend vertical space on duplicate context-chip strips when
    the page header and section body already communicate that scope. `Workspaces`
    may own one inline facts line and one inline next-action row at the top of
    the section, but that summary treatment must stay inside the same task
    surface instead of reviving a separate overview panel, summary strip, or
    metric deck ahead of the real workspace list.
21. Keep monitored-system volume out of the commercial product model.
    Recognized self-hosted and hosted v6 tiers must treat legacy
    monitored-system limit, continuity, or `max_monitored_systems` metadata as
    scrub-only compatibility input rather than as support/audit policy or a
    customer-facing plan model. Community, Relay, Pro, Pro+, lifetime,
    grandfathered recurring, Cloud, and MSP plan labels render through their
    normal plan surfaces with no Usage subtab, finite policy banner,
    monitored-system limit row, grandfathered-floor summary, or
    pause-new-admissions copy from stale volume metadata. Retired
    `max_monitored_systems` pricing handoffs must land on the neutral
    self-hosted plan surface and be normalized to plan selection rather than a
    monitored-system usage ledger.
    That same self-hosted commercial boundary also owns the operator-value
    narrative across the in-app billing shell, Pulse Account handoff, public
    pricing contract, and owned upgrade reasons. Customer-facing self-hosted
    copy must keep the ladder explicit as `Community = monitor`, `Relay = reach
Pulse from anywhere plus pair Pulse Mobile for handoff`, and `Pro =
hands-on Patrol modes, issue investigation, verified fixes, and 90-day history`.
    Team/admin extras such as
    RBAC, SSO, audit logging, reporting, and agent profiles may remain present,
    but they are secondary included value and must not displace that operator
    outcome framing on owned commercial surfaces.
22. Keep public-demo route bootstrap owned on the adjacent
    commercial/runtime boundary. `frontend-modern/src/useAppRuntimeState.ts`
    may prewarm shared infrastructure summary caches for route-owned surfaces,
    but public-demo arrival must not front-run a broader infrastructure-summary
    fetch than the route actually renders. Commercial
    posture on the stable public demo therefore stays governed by the route-owned
    presentation policy and summary scope rather than by app-shell-wide
    bootstrap heuristics. That same app-shell boundary must also keep the
    public login entrypoints (`/` and `/login`) quiet before a session exists:
    when auth is configured but no local auth hint has been established yet,
    `frontend-modern/src/useAppRuntimeState.ts` must stop at the shared
    login-needed state and skip `/api/state`, while protected-route arrivals
    and post-login reloads still keep the canonical state probe for runtime
    detection. Protected Patrol entrypoints follow that same route-owned rule:
    the canonical Patrol surface may live at `/patrol`, and any legacy `/ai`
    browser entrypoint must remain only a thin authenticated redirect rather
    than a second public handoff or a route that invites separate commercial
    posture logic.
23. Keep self-hosted commercial plan copy aligned with the v6 operator-value
    model instead of mirroring raw entitlement keys. `frontend-modern/src/
utils/selfHostedPlans.ts` may still map onto internal capabilities such as
    `ai_patrol`, `ai_alerts`, `ai_autofix`, and `kubernetes_ai`, but the
    customer-facing plan cards, top-summary highlights, and comparison rows
    must lead with the canonical plan story: `Community = monitor`, `Relay =
reach Pulse from anywhere plus pair Pulse Mobile for handoff`, and `Pro =
hands-on Patrol modes, issue investigation, verified fixes, and longer history`.
    Team/admin capabilities such as
    RBAC, audit logging, reporting, and agent profiles may appear as
    secondary included extras. Their customer-facing plan summary label is
    `admin controls`, not `admin tools`, because Pro is sold as Patrol handling
    infrastructure within a chosen mode rather than as a bag of operator
    utilities. Platform-specific compatibility keys such
    as `kubernetes_ai` must not be elevated into a marquee marketed Pro line
    item. Pro comparison and plan-selection copy must frame Patrol around what
    it may handle automatically, issue investigation, policy-bounded action,
    verified fixes, and history rather than as an abstract governed operator or
    a choice about how much control Patrol has.
    When free-plan copy mentions Patrol, it must describe watch-only/background
    health checks; investigation, approval-backed fixes, autonomous action,
    outcome verification, and extended history are Pro value.
    Legacy packaging nouns such as
    `incident memory`, `scheduled remediations`, and `execution audit trail`
    must likewise stay out of current v6 commercial copy unless Pulse ships a
    first-class product surface that makes those names truthful again. The raw
    entitlement key `ai_autofix` may remain the internal capability identifier,
    but owned plan cards, comparison rows, and upgrade reasons must present it
    as Patrol fixing safe issues within the selected Patrol mode rather than
    as `Patrol Auto-Fix` or an abstract remediation-workflow label. Pulse Account preview/handoff pricing
    copy, public plan docs, and the in-app self-hosted billing shell must share
    that same free-first ladder so release-candidate and public purchase paths
    do not drift into a separate commercial story.
    That same self-hosted pricing boundary now also owns canonical feature
    classification metadata. `pkg/licensing/self_hosted_feature_catalog.go`
    is the source of truth for customer-facing self-hosted feature labels,
    comparison visibility, plan roles (`primary_pillar`, `included_extra`,
    `compatibility_only`, and hidden/included states), and generic upgrade
    reasons. `frontend-modern/src/utils/selfHostedFeatureCatalog.generated.ts`
    and `pulse-pro:license-server/self_hosted_feature_catalog.generated.go`
    are generated projections of that source for app and public-pricing
    consumers; those projections must not drift through hand-maintained
    parallel lists or page-local renaming. `pkg/licensing/upgrade.go` must
    also treat `compatibility_only` entries as non-marketed compatibility
    capabilities by returning only the generic pricing destination rather than
    a feature-specific paid upgrade URL.
24. Keep hosted trial-activation verifier source selection compile-time owned.
    `pkg/licensing/trial_activation.go`,
    `pkg/licensing/trial_activation_public_key_override_dev.go`, and
    `pkg/licensing/trial_activation_public_key_override_release.go` together
    define whether runtime environment may override the hosted verifier. Dev
    builds may keep the local override path, but release builds must resolve
    the verifier from the embedded build-time source of truth instead of
    honoring `PULSE_HOSTED_MODE` or other runtime wiring.
25. Add or change the Cloud control-plane runtime, public signup lifecycle,
    tenant health/reconciliation, control-plane metrics, email delivery,
    hosted handoff, hosted entitlement refresh, or hosted tenant reaping through
    `internal/cloudcp/` and `internal/hosted/`. Those paths must stay governed
    by the Cloud paid subsystem even when a narrower support subsystem also
    owns a deployment, security, or relay-specific file in the same package
    tree.
    Control-plane transactional email may use a no-reply sender for delivery,
    but Resend delivery must set an explicit support reply path from
    `PULSE_EMAIL_REPLY_TO`, defaulting to `support@pulserelay.pro`, so magic
    links, checkout handoffs, and other hosted-account messages remain
    directly replyable by customers.
    Paid and hosted app shells must inherit the frontend-primitives-owned
    platform/runtime primary navigation contract and canonical path constants
    from `frontend-modern/src/routing/resourceLinks.ts`; cloud-paid must not
    carry a second copy of the platform list, shell order, or Machines-surface
    eligibility rules. Product navigation remains support-and-evidence gated:
    `enabled`, `live`, and inclusion in the rendered primary navigation derive
    from the governed support manifest plus canonical resource evidence in
    `state.resources`. Admitted-only, presentation-only, unsupported, or
    absent platform families must not render as disabled placeholders in the
    paid or hosted app shell. Infrastructure is not an equal primary tab in
    that list. Workloads, Storage, and Recovery are not standalone aggregate
    workspace tabs; their component surfaces remain reusable inside platform
    pages via embedded `tableOnly` surfaces. Platform primary-tab settings
    handoffs in `AppLayout.tsx` must target the canonical
    `/settings/infrastructure` workspace and must not retain retired settings
    aliases such as `/settings/workloads/docker` or nested
    `/settings/infrastructure/platforms/*` paths. Each platform page must
    remain chrome-only: routing plus sub-tab navigation that embeds the
    canonical `WorkloadsSurface`, `StorageSurface`, `RecoverySurface`, or
    `UnifiedResourceTable` in `embedded tableOnly` mode with a forced
    platform/source filter. The shell must not introduce dashboard cards,
    bespoke per-family tables, synthetic placeholder data, or reintroduce
    Infrastructure as a primary navigation entry without a governed contract
    decision recorded by the frontend-primitives owner.
    The presence of the user-facing `Machines` tab, backed by the internal
    `standalone` route/id, is not a new commercial usage unit: hosted and paid
    surfaces continue to meter the governed monitored-system grouping result
    rather than counting primary navigation destinations.
    The user-facing `Patrol` utility tab is likewise not a commercial plan or
    usage unit; it remains the Patrol-owned daily work entrypoint over current
    findings, approvals, and verification state.
    Landing behavior for paid and hosted shells must also defer to the
    frontend-primitives-owned provider-first landing contract instead of
    defining a cloud-paid-specific order.
26. Introduce the self-hosted `business` tier only through this shape:
    `TierBusiness` in `pkg/licensing/features.go` carries exactly the Pro
    feature set (no feature-level differentiation), 365-day history
    retention in `TierHistoryDays`, membership in the self-hosted
    core-monitoring-uncapped tier and plan-version sets (`business`,
    `business_annual`), the `Business` display name, and a slot between Pro
    and MSP in the min-tier ordering. Business differentiates commercially
    by the `max_users` license limit (unlimited for Business; newly issued
    Pro licenses may carry a finite `max_users` while previously issued
    licenses keep their unlimited posture), retention, and support, never
    by gating features away from Pro. The tier stays dormant until the
    license server issues business plan versions through a governed
    rollout; no checkout, pricing-model payload, public pricing page, or
    in-product plan surface may reference Business before that rollout, and
    monitored-system volume stays out of the Business plan model per the
    self-hosted commercial boundary above.
    The `max_users` seat limit travels the licensing chain as one named
    shape mirroring `MaxGuests`: `Plan.MaxUsers` on the license server,
    `max_users` on the stored v6 license and in relay grant claims (all
    three copies of the grant wire struct), `GrantClaims.MaxUsers` copied
    into `Claims.MaxUsers` in `pkg/licensing`, and surfaced through
    `EffectiveLimits()["max_users"]` into the shipped user-limit
    enforcement (`MaxUsersLimitFromLicense`). It mirrors `MaxGuests` in
    shape only, not in scrub behavior: `max_users` is a seat limit, not a
    monitored-volume cap, so `stripSelfHostedCommercialVolumeCaps` and the
    license-server entitlement normalization must never strip or zero it.
    Absent claim = 0 = unlimited at every hop, which keeps all existing
    licenses, grants, and plans inert until the governed rollout sets
    `max_users` on a plan.
27. Add or change the canonical commercial offer, self-hosted subscription
    transition matrix, cancellation/payment-failure grace, downgrade
    preservation, or Cloud/MSP availability only through the approved contract
    in
    `docs/release-control/v6/internal/records/commercial-offer-lifecycle-contract-2026-07-14.md`.
    Stripe Customer Portal `subscription_update` remains disabled. Pulse-owned
    plan changes must use the canonical transition authority, present an
    explicit amount/effective-date quote, and converge from an authoritative
    Stripe snapshot rather than trusting one webhook payload or a browser-
    supplied plan identity. Unknown prices, catalog-version disagreement, and
    incomplete entitlement projections fail closed. Every material tier,
    cadence, capability, or restrictive-state change increments
    `license_version` and emits the version-floor/outbox event in the same
    local transaction.
28. Add or change Relay-side commercial invalidation only through the shared
    `pulse-pro:relay-server/main.go`, `pulse-pro:relay-server/revocation_feed.go`,
    and `pulse-pro:relay-server/registry.go` boundary. Operator Relay startup
    must fail closed without the authenticated feed, drain it before serving,
    report stale feed state through readiness, and disconnect already-connected
    v6 sessions whose grant falls below an applied version floor.
29. Add or change customer Pulse commercial invalidation only through
    `pkg/licensing/installation_status_poll.go`,
    `pkg/licensing/license_server_client.go`, and the installation-authenticated
    `pulse-pro:license-server/admin_ui.go`,
    `pulse-pro:license-server/v6_handlers.go`,
    `pulse-pro:license-server/v6_grants.go`, and
    `pulse-pro:license-server/v6_store.go` boundary. The status response may
    expose only the caller's authoritative license version and bounded cadence
    hints; it must never expose a grant, the global Relay feed credential, or
    another commercial subject's event data.

## Forbidden Paths

1. New ad hoc plan names in runtime or UI
2. Silent aliases between old and new limit keys in live runtime paths
3. Pricing/UI claims that are not enforced by runtime entitlements
4. Direct Stripe subscription price changes that update billing identifiers
   without atomically updating tier, cadence, features, continuity, and license
   version
5. Customer Portal configuration as the authority for plan transitions
6. Separate offer definitions in landing, account, app, support, or Stripe
   provisioning code
7. Distributing the global Relay revocation-feed bearer token to customer Pulse
   installations; customer-runtime invalidation requires installation-scoped
   authentication or an equivalently bounded authority.

## Completion Obligations

1. Update this contract when cloud plan semantics change
2. Update runtime and frontend tests together when plan/limit rules move
3. Add or tighten drift tests when a pricing/runtime mismatch is fixed
4. Keep self-hosted pricing and public docs on runtime-backed commercial truth:
   self-hosted Patrol remains provider/local-model based in Community, paid
   discovery stays out of ordinary monitoring flows, and retired hosted-model
   credits or trial acquisition must not be presented as a default self-hosted
   benefit. Relay and Pro remain the canonical self-hosted commercial story.
5. Treat grandfathered `lifetime` licenses as uncapped commercial entitlements:
   they keep the Pro feature set, but they must not inherit monitored-system or
   guest caps from recurring Pro contracts anywhere in runtime, issuance, or
   migrated-license storage.
6. Treat active recurring v5 Pulse Pro customers as grandfathered commercial
   entitlements until cancellation: migrated and renewing v5/v1 recurring
   plans keep their existing recurring price while the subscription remains
   continuous. The current Community / Relay / Pro self-hosted packaging also
   stays no-cap for monitored systems, guests, and child-resource volume, with
   Pro+ remaining legacy continuity only.
7. Keep Pro+ app presentation continuity-only: customer-facing tier, plan, and
   plan-version labels for `pro_plus` must include legacy framing while still
   mapping the entitlement to the Pro runtime feature set.
8. Keep persisted billing baselines and live recurring continuity scrubbed:
   `pkg/licensing/billing_state_normalization.go`,
   `pkg/licensing/database_source.go`, `pkg/licensing/models.go`, and
   downstream runtime entitlement evaluation must delete retired
   monitored-system volume keys before runtime status, grants, leases, or
   frontend billing payloads are built.
9. Keep legacy migration fallback continuity compatibility-only on v6:
   `legacy_migration_fallback` may be parsed so old activation files and
   billing records load, but `plan_limit`, `grandfathered_floor`, and related
   volume telemetry must not become runtime status, entitlement enforcement,
   customer-facing support context, or self-hosted Plans presentation.
10. Keep organization billing loading placeholders on the shared
    `SettingsLoadingSkeleton` primitive. Cloud-paid owns billing and usage
    semantics; frontend-primitives owns the loading skeleton animation, fill
    tokens, and metric/progress/card placeholder shells.
11. Keep Stripe webhook idempotency state bounded in the control-plane
    registry: `internal/cloudcp/registry/registry.go` may retain `stripe_events`
    rows long enough to suppress duplicate deliveries and reclaim stale
    in-flight work, but it must prune expired processed or abandoned rows so
    webhook dedupe does not grow without bound on disk.
12. Keep the maintained Pulse Account portal bundle source-synced with
    `internal/cloudcp/portal/frontend/`: any slice that changes the portal
    frontend hash or emitted manifest must rebuild
    `internal/cloudcp/portal/dist/build_manifest.json` and keep
    `internal/cloudcp/portal/frontend_sync_test.go` green in the same change.
13. Before GA, treat infrastructure monitoring volume as unmetered:
    monitored systems remain the canonical inventory grouping unit, but paid
    value must come from optional extras, hosted convenience, business
    workflow, support, or similar non-core surfaces rather than using
    monitored-system volume itself as a paid gate.
    Child-resource volume, including guest capacity, must follow the same
    self-hosted rule instead of becoming a replacement paid gate for core
    monitoring.
    Monitored-system impact-preview copy must follow the same rule: ordinary
    connection previews may describe count impact and grouping changes, but
    they must not use capacity-style titles or slash-style quota summaries that
    imply self-hosted monitoring volume is the product being sold.
14. Keep v6 billing state uncapped even when persisted state still carries
    legacy commercial volume-limit keys:
    `pkg/licensing/models.go`,
    `pkg/licensing/service.go`,
    `pkg/licensing/entitlement_payload.go`,
    `pkg/licensing/billing_state_normalization.go`,
    `pkg/licensing/database_source.go`,
    `pulse-pro:license-server/v6_store.go`, and
    `pulse-pro:license-server/v6_schema.go` must scrub stale
    `max_monitored_systems` values for Community/free, Relay, Pro, Pro+, Pro
    Annual, lifetime, eligible grandfathered recurring, Cloud, and MSP plan
    labels before runtime-capability, entitlement, grant, lease, billing, or
    browser-facing payloads are built.
    The same boundary must merge sparse legacy, configured-plan, or manually
    supplied feature lists with canonical recognized-tier defaults before
    storage, API response, or grant signing so Lifetime, Pro, Pro+, and
    grandfathered recurring customers cannot be downgraded to the partial
    feature list carried by an old JWT or legacy plan row.
15. Keep self-hosted commercial funnel stage ownership out of the customer
    product runtime. The customer frontend must not contain `upgradeMetrics`,
    `conversionEvents`, or infrastructure onboarding metrics wrappers or call
    sites at all. In-app `Plans & Billing`, pricing, checkout, paywall, or
    onboarding surfaces must not emit browser product events, and the normal
    self-hosted API must not register local upgrade-metrics ingestion, health,
    config, stats, or funnel routes. `pulse-pro:license-server/v6_checkout.go`
    owns the Pulse Account handoff equivalents bound to `portal_handoff_id`.
    Pulse must not infer those portal stages from referrer state, and the
    commercial service must keep those self-hosted handoffs on release track
    `v6` even while the public site remains on `v5` before GA.
16. Keep retired local commercial funnel reporting out of customer diagnostics,
    settings, and product startup: `internal/api/diagnostics.go`,
    `frontend-modern/src/components/Settings/DiagnosticsResultsPanel.tsx`,
    `frontend-modern/src/components/Settings/diagnosticsModel.ts`,
    `internal/api/router_routes_licensing.go`, `internal/api/system_settings.go`,
    `internal/config/config.go`, `internal/config/persistence.go`,
    `pkg/server/server.go`, `frontend-modern/src/stores/systemSettings.ts`, and
    `frontend-modern/src/types/config.ts` must not expose maintainer funnel,
    pricing/checkout, upgrade-metrics, or infrastructure-onboarding analytics
    fields, controls, routes, stores, or startup DB artifacts to normal product
    users.
17. Keep ordinary self-hosted v6 commercial prompts opt-in. Cloud-paid runtime
    may keep checkout, activation, recovery, and support-only trial plumbing
    available for explicit handoffs and entitled installs, but default
    self-hosted browser surfaces must honor `presentationPolicy.hideUpgrade`
    and suppress Relay/Pro plan comparison, Pro trial CTAs, monitored-system
    limit pressure, paid-only settings navigation, and feature upsells unless
    hosted mode, direct intent, activation/recovery state, or active entitlement
    makes them relevant. The authenticated app shell must also skip background
    `/api/license/commercial-posture` bootstrap while `presentationPolicy.hideUpgrade`
    is true; explicit self-hosted plan, activation/recovery, and hosted or
    prompt-allowed flows may still refresh the shared posture store.
18. Keep hosted and trial billing construction separate from retired hosted-AI
    quickstart inventory: `pkg/licensing/trial_start.go` and hosted
    entitlement refresh paths may preserve historical billing fields for old
    state, but new trial or hosted workspaces must not mint quickstart credits
    or imply a managed-model allowance as part of the commercial contract.
19. Keep Cloud signup acquisition owned by the Cloud control plane and public
    account surfaces, not by the ordinary self-hosted app shell. The default
    self-hosted frontend must not register unauthenticated `/cloud` or
    `/cloud/signup` routes, and self-hosted pricing fallbacks must route Cloud
    interest to Pulse Account/public Cloud ownership rather than rendering an
    in-product trial or checkout page.
20. Keep the `unlimited` feature key neutral in shared metadata. It is a
    hosted/MSP capacity-policy marker, not a self-hosted monitoring-volume
    product promise. Runtime and generated feature catalogs must label it as
    hosted capacity policy, keep it hidden from self-hosted plan cards, and
    avoid customer-facing "Unlimited Instances" copy that sounds like the old
    capped self-hosted packaging.
21. Keep the approved self-hosted scope coherent: one owner-operated
    environment, three primary/migration/recovery activations, and unmetered
    monitored systems and child resources. Terms, public copy, activation
    enforcement, support transfer tooling, and MSP boundaries must use the
    same definition.
22. Keep Pro's Relay bundle explicit in every offer and fulfillment
    projection, including Stripe product-description audits. Relay and Pro are
    distinct jobs rather than sequential ladder steps, and a Pro buyer must
    never be told to buy Relay separately for the same environment.
23. Do not expose Relay/Pro or cadence self-service transitions until the
    `self-hosted-commercial-transition-coherence` gate proves proration,
    scheduled reductions, cancellation/recovery grace, payment-failure grace,
    refund/dispute handling, replay/order convergence, downgrade preservation,
    and grant-version invalidation.
24. Keep Cloud unavailable and MSP request-assisted until their respective
    availability contracts are explicitly reopened. Historical Cloud prices,
    caps, trial, and support labels are dormant planning data, while provider-
    hosted MSP remains the default assisted-preview delivery boundary.
25. Keep operator Relay revocation enforcement mandatory and readiness-backed:
    startup must synchronously establish the monotonic version-floor cache,
    feed staleness must fail readiness, and feed-applied restrictive events
    must disconnect stale active v6 sessions and invalidate their reconnect
    credentials.
26. Keep customer Pulse invalidation separate from the operator-wide feed
    credential. Before self-service transitions are released, prove a scoped
    customer-safe authority that refreshes or clears paid state without giving
    any installation visibility into another customer's revocation events.
27. Keep terminal Stripe event failure recoverable without bypassing ordering
    safeguards. The authoritative 30-day Stripe backfill may reopen a `failed`
    inbox event for one fresh retry cycle, but it must claim the event before
    dispatch, retain the original payload, use the cursor-aware retry path, and
    return to bounded backoff when the replay still fails.
28. Keep production Stripe catalog remediation bounded, reviewable, and
    separate from runtime deployment. The canonical tool must default to
    GET-only planning, derive public price identity from the pricing model,
    refuse catalog or Stripe-mode drift before writes, require an explicit
    apply confirmation, update only the two public self-hosted product
    descriptions, and create or reuse an exact Pulse-owned invoice/payment-
    method-only portal configuration. It must not mutate an unknown portal
    configuration, price, customer, subscription, webhook, license, or
    deployed environment. The returned portal identifier is a separate
    approved configuration/deployment input, and a passing GET-only production
    re-audit remains the postcondition.

## Current State

The approved commercial contract is recorded in
`docs/release-control/v6/internal/records/commercial-offer-lifecycle-contract-2026-07-14.md`.
The local self-hosted license server now has an atomic commercial projection
and a Pulse-owned quoted transition saga for Relay/Pro and cadence changes.
Cancellation recovery is non-entitling, payment failure has a distinct
functional grace, unknown or multi-price snapshots fail closed, and runtime
downgrade state delays physical history/artifact cleanup while blocking paid
background work immediately. Renewal retries now re-project authoritative
Stripe state even when the requested flag already matches, newly created
subscription schedules are durably attached to their quote before later Stripe
or local completion steps, schedule-release retries repair the local quote, and
the authoritative backfill can reopen a terminal inbox event through the
cursor-aware retry path. The implementation is not yet a released self-
service capability: the governed external Stripe transition matrix, event-
order/reconciliation exercise and Relay version-floor proof remain required by
`self-hosted-commercial-transition-coherence`. Cloud remains unavailable and
MSP remains an assisted preview.

The 2026-07-14 GET-only production catalog/portal audit was exercised and
failed expected state: all 25 governed prices resolved, but the runtime did not
provide a governed billing-portal configuration identifier and the public
Relay/Pro Stripe products retained pre-contract descriptions. No Stripe object
or customer record was mutated or inspected. The dated blocked record is
`docs/release-control/v6/internal/records/self-hosted-commercial-transition-coherence-production-audit-blocked-2026-07-14.md`;
at that point the gate stayed blocked until separately approved remediation
was deployed and the same read-only audit passed.

Repository-side remediation preparation now exists in
`pulse-pro:scripts/remediate_stripe_commercial_state.py`. Offline proof covers
shared-product deduplication, strict price and mode preconditions, exact portal
reuse, safe creation instead of mutation of a nonconforming portal, bounded
write endpoints, and apply confirmation. The dormant new-environment price
creation helper also uses the same canonical descriptions. This preparation
did not change the production finding at that point: no live apply or runtime
deployment had yet been authorized or executed.

On 2026-07-15 the separately approved bounded production remediation converged
the two public Relay/Pro product descriptions, created and deployed dedicated
invoice/payment-method-only portal configuration
`bpc_1TtNsxBrHBocJIGHTjRvG4Qf`, and passed the same GET-only production audit:
all 25 governed prices resolved, the portal contract passed, and the only
warnings were the already-governed inactive v1 legacy recurring prices. A
second remediation plan returned no product updates and reused the dedicated
portal, proving idempotent convergence. The production catalog/portal residual
is closed, but `self-hosted-commercial-transition-coherence` remains blocked
until the external Stripe lifecycle transition/event-reconciliation matrix and
real Relay license-version-floor exercise pass. The evidence record is
`docs/release-control/v6/internal/records/self-hosted-commercial-transition-coherence-production-remediation-2026-07-15.md`.

The Relay side now fails closed on missing feed authority, drains the feed
before serving, exposes feed staleness through readiness, and tears down stale
already-connected v6 sessions while clearing their reconnect tokens. Customer
Pulse now uses an immediate, installation-authenticated `POST
/v1/grants/status` check followed by the existing signed-grant refresh path
only when the authoritative license version advances. Explicit token,
installation, or license revocation clears encrypted activation state. An
authoritative suspension removes paid entitlements immediately while retaining
the scoped installation credential for automatic recovery; other authorization
or transport failures retain the last signed grant and surface non-secret
synchronization health through `/api/license/status`. The global operator feed
remains Relay-only. Real external Stripe-to-runtime proof remains release-gated.

Hosted handoff target paths are normalized by the shared host-local redirect
validator at token minting, exchange, and provider-proof boundaries. Absolute,
scheme-relative, backslash-authority, encoded-separator, and control-character
targets are rejected before they can reach a browser redirect.

Primary nav moved to platform-first on 2026-05-16 through
`frontend-modern/src/App.tsx`, `frontend-modern/src/AppLayout.tsx`, and
`frontend-modern/src/pages/RuntimeHome.tsx`. The authenticated-runtime landing
redirect now resolves to `/proxmox/overview` via `buildProxmoxPath()` rather
than the retired `/infrastructure` shell; kiosk-mode blocked-prefix redirects
follow the same canonical home. The hosted bootstrap / activation / recovery
routes (`/pricing`, `/preview/setup-complete`, `/login`, settings panels
under `/settings/...`) are unchanged. Pricing-handoff and hosted-signup
upgrade paths must use platform routes as their canonical "back to product"
destination instead of the retired top-level Infrastructure route; aggregate
Workloads / Storage / Recovery pages are product workspaces, not hosted
acquisition destinations. The Docker / Podman runtime route remains `/docker`
for search familiarity, but the shell may label the
destination as Docker so it reads as the recognizable container runtime lens
rather than an owning infrastructure platform.
Localized runtime-home loading copy may route through the shared frontend i18n
catalog, but this does not move route ownership or commercial posture into
localization: authenticated `/`, hosted workspaces, public-demo suppression, and
entitlement-read prohibitions remain governed by this cloud-paid boundary.

The Proxmox platform tab in the authenticated app shell is ordinary product
navigation. It must stay separate from hosted acquisition, monitored-system
pressure, billing prompts, and Pulse Account handoffs: self-hosted operators
may open the Proxmox page without seeing commercial upgrade copy, while hosted
commercial policy continues to render through the existing cloud-paid banners
and settings surfaces.

`frontend-modern/src/utils/pricingHandoff.ts` now routes displayable paid
self-hosted feature keys (`relay`, `mobile_app`, `push_notifications`,
`ai_alerts`, `ai_autofix`, `long_term_metrics`, `rbac`,
`audit_logging`, `advanced_reporting`, `agent_profiles`) to the
self-hosted billing plan page instead of the Pulse Account purchase-start
handoff. The purchase-start handoff requires a `PublicURL` and fails on local
instances; routing these keys to the in-product billing plan keeps upgrades
accessible from self-hosted environments.
Retired trial-acquisition intents such as `trial_expired` must not be owned
pricing handoff destinations; no ordinary self-hosted runtime path should emit
them as an upgrade reason or plan-page CTA.
Legacy browser arrivals carrying that stale feature may land on the neutral
self-hosted plan surface, but must not preserve `trial_expired` into a Pulse
Account purchase-start handoff or trial-shaped checkout feature. The shared
upgrade-action fallback must therefore return no action for retired trial keys,
while the legacy `/pricing?feature=trial_expired` browser route may strip the
stale feature and route to `/settings/system/billing/plan`.
The monitored-system app-shell warning CTA now follows that same commercial
boundary by rendering only in hosted mode. Ordinary self-hosted installs must
not see finite monitored-system pressure in the global app shell; when hosted
capacity policy is active, the banner reviews finite-policy usage on the usage
ledger rather than sending operators to the plan-selection surface with
capacity-shaped copy.
Stripe checkout metadata now treats `plan_id` and `price_id` as Stripe price
identifiers for plan-version derivation, including grandfathered v5 recurring
prices. The Cloud control-plane webhook uses that derivation as an admission
boundary: hosted Cloud/MSP checkout events may provision tenants, while
self-hosted landing purchases are acknowledged and ignored rather than
creating hosted containers or Stripe account mappings.

The Pulse-operated MSP account path has a gated public front door alongside the
individual cloud signup page. `internal/cloudcp/public_msp_signup_handlers.go`
serves `/cloud/msp/signup`, `/cloud/msp/signup/complete`, and
`/api/public/msp/signup`, all gated behind the same `PublicCloudSignupEnabled`
flag as `/cloud/signup`, so the MSP front door stays dark until an operator
explicitly enables public signup. This path is not the provider-hosted MSP
artifact; it is the Pulse-operated checkout/account entry point that proves the
provider-account/client-workspace model. MSP Starter is the only tier that may
be served through gated checkout, and it is offered only when
`CP_MSP_STARTER_PRICE_ID` is configured. Growth, Scale, Enterprise, manual
pilots, and discounts belong in request-based or operator-controlled
Stripe/customer-support workflows rather than the public front door. The signup
page renders an explicit "Starter self-serve signup is not open yet" notice when
the Starter price is not configured rather than offering an unbacked checkout.
Checkout sessions started from this front door carry `account_kind=msp`,
`signup_source=public_msp_signup`, and `checkout_billing_mode=immediate`
metadata and do not set Stripe `trial_period_days`; any opened checkout is
paid-first. The provisioner reads that billing-mode metadata so the initial
Stripe account mapping is stored as active instead of trial before subscription
webhooks catch up.

Cloud paid readiness is materially behind architecture work. The main concern is
contract coherence between pricing, entitlements, and runtime enforcement.
Pulse Account portal workspace copy is part of that same readiness contract:
hosted view-only users may be invited to review workspace health and open ready
workspaces, but workspace section lead copy must route lifecycle handling to an
owner or admin instead of presenting Lifecycle as a live task for the current
signed-in role.
That public-demo commercial boundary also owns monitored-system preview
unavailability wording. Browser presentation may keep the unavailable reason
nullable until the formatting edge, but it must normalize the message through
the shared monitored-system presentation helper instead of branching on
demo/billing state inside settings panels or inventing a second mock-only
license explanation path.

The app-layout Patrol attention count is a license-neutral operational read.
It may show canonical current work after authenticated bootstrap, but it must
not infer a paid entitlement, expose an upgrade prompt, or convert missing
commercial state into feature access.

Hosted organization and Billing Admin browser clients must prefer an active
`pulse_session` cookie when the same browser also retains an API token from
first-run setup. Platform-admin and session-required organization operations
must not let that ambient token override the authenticated administrator
session. Token-only browser clients retain their existing scoped fallback when
no session cookie is present.
That same cloud-paid/browser boundary now also governs public demo posture.
`DEMO_MODE` may run against a real internal entitlement, but public demo
surfaces must not reveal self-hosted license metadata, hosted billing state,
monitored-system ledgers, upgrade nudges, or activation controls just because
the underlying runtime is commercially enabled.
That same cloud-paid boundary now also owns release-demo fixture secrecy.
Release builds may enforce the internal `demo_fixtures` entitlement before
enabling mock fixture runtime state, but browser-visible licensing and
commercial payloads must still filter that capability back out and must not
let cloud-paid surfaces treat dev/test fixture toggles as plan, billing, or
upgrade truth.
`PULSE_DEV` and `PULSE_MOCK_MODE` may remain backend-only development or
fixture gate bypasses for local/runtime operations, but
`pkg/licensing.Service.Status()` must not use either flag to synthesize
customer-facing capabilities, limits, plan state, upgrade reasons, or paid
settings navigation in `/api/license/status`,
`/api/license/runtime-capabilities`, or `/api/license/entitlements`.
That same resolved presentation policy also governs hosted organization chrome
inside the demo/browser shell: app bootstrap may retain an internal default
org context for hosted API routing, but `frontend-modern/src/useAppRuntimeState.ts`,
`frontend-modern/src/App.tsx`, and `frontend-modern/src/AppLayout.tsx` must
not render visible org switchers, `Default Organization` labels, or
organization-scoped navigation just because a seeded hosted org still exists
behind the session.
`internal/api/router_routes_auth_security.go`,
`internal/api/security_status_capabilities.go`,
`frontend-modern/src/useAppRuntimeState.ts`, and
`frontend-modern/src/stores/sessionPresentationPolicy.ts` must treat
`/api/security/status` as the canonical browser bootstrap contract. The
backend capability fact `sessionCapabilities.demoMode` still seeds that
contract, but shared billing and upgrade surfaces now hide or suppress
themselves from the resolved `presentationPolicy` payload instead of teaching
mock mode, response-header inference, or frontend-only feature flags to
bypass the real licensing model. That includes settings billing tabs,
public-demo banner and monitored-system/trial nudges, app-wide relay
paywalls, Patrol upgrade CTAs, and history-lock upsells. Demo readiness
therefore means presentation isolation, not a license exemption.
That same public-demo boundary now also owns route-level commercial
classification. `internal/api/demo_middleware.go` and
`internal/api/demo_mode_commercial.go` must decide centrally which commercial
endpoints are fully hidden (`404`) and which remain available only as a
non-commercial public contract. `/api/license/runtime-capabilities` is the
canonical public exception for feature truth and history retention.
`/api/license/commercial-posture` is the canonical non-billing commercial
contract for upgrade posture in real customer workspaces, while
`/api/license/entitlements` remains billing-only. In public demo mode both
commercial routes, plus `/auth/license-purchase-start`, must stay hidden and
public browsers must not see licensed identity, plan labels, upgrade reasons,
trial urgency, observed usage counts, or checkout handoff state. The
commercial posture store and billing-entitlements store must also fail closed
locally until the shared presentation policy resolves, then stay fail-closed
in demo mode so hidden routes are not probed from the browser shell.
Paid self-hosted Pro licenses also have a runtime-distribution boundary:
billing entitlement payloads may confirm the paid feature grant, but the
runtime-capabilities payload must mark private-runtime-only capabilities as
blocked when the process is the public community build. Customer-facing paid
runtime copy must send those users to the private Pulse Pro download rather
than implying that the public GitHub release or public Docker image can unlock
the same runtime hooks.
The owned self-hosted Plans surface must also show a direct Pulse Pro downloads
action when an active paid entitlement is running on the community runtime, and
the existing-key recovery panel must warn that paid Docker and Linux installs
need the private Pulse Pro image or archive before a customer pastes a key.
That same browser-shell boundary also owns utility-nav suppression:
`frontend-modern/src/AppLayout.tsx` must not expose a separate top-level
Operations destination. Diagnostics, reports, and logs now live under the
canonical Settings support navigation, and public demo mode must keep those
support-only entries hidden instead of leaving the retired operations surface
discoverable after commercial surfaces are hidden.
Deep-linkable commercial panels must consume the same resolved presentation
policy directly, not rely only on settings navigation to keep public-demo
browsers away from commercial routes. `ProLicensePanel` may instantiate its
license/trial/recovery state only after the policy resolves and only when
commercial surfaces are visible. `MonitoredSystemLedgerPanel` must also defer
ledger reads until that policy resolves and suppress them while commercial
surfaces are hidden. Public demo mode therefore renders a redacted
presentation-policy state instead of creating a fake entitlement, probing
hidden commercial endpoints, or showing monitored-system usage pressure.
That same commercial/public browser boundary also owns pricing-handoff
framing. `frontend-modern/src/pages/PricingHandoff.tsx` may keep the
operator-visible handoff on `/pricing`, but it must render the shared
`PageHeader` shell while `frontend-modern/src/utils/pricingHandoff.ts`
continues to own destination resolution, self-hosted Pulse Account handoff,
and public-pricing fallback truth. The route must not fork a raw top-level
heading, duplicate destination logic in the page shell, or let commercial
handoff framing drift away from the same shared browser chrome used by the
rest of the product.
The visible redirect title and manual-link copy on that page may use the
shared frontend i18n catalog for first-wave locale coverage, but the
destination selection, self-hosted `GET /auth/license-purchase-start` handoff,
public pricing URL fallback, feature query keys, and purchase-return states
remain cloud-paid-owned and machine-stable.
The governed browser proof for that posture lives in
`tests/integration/tests/53-demo-mode-commercial-boundary.spec.ts` and is
expected to stay runnable through
`tests/integration/scripts/run-tests.sh demo-contract`.
That same public-demo boundary now treats the old `v6-demo.pulserelay.pro`
preview route and its `pulse-v6-preview` service/bootstrap tooling as retired
after v6 GA. The stable public demo is the canonical customer-facing v6 demo
surface. Any future prerelease or canary demo needs a newly governed target,
hostname, service identity, and audit path; it must not silently revive the
retired v6-preview route or split the public demo posture back into parallel
stable and v6 surfaces.
That same licensing/browser boundary now also owns authenticated commercial
posture bootstrap and the prohibition on non-billing entitlement reads.
`frontend-modern/src/useAppRuntimeState.ts` is the canonical authenticated
owner for the first `/api/license/commercial-posture` read when upgrade prompts
are allowed, while
`frontend-modern/src/AppLayout.tsx`,
`frontend-modern/src/components/Settings/Settings.tsx`,
`frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts`, and
other non-billing feature hooks may consume the resolved posture store but
must not each trigger their own mount-time posture fetch. Ordinary self-hosted
sessions where `presentationPolicy.hideUpgrade` is true must not perform that
background posture read from the app shell at all. Billing-owned
surfaces such as `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
may still force refresh through the same shared store when plan, activation, or
recovery actions mutate commercial truth.
Non-billing browser journeys must also stay off
`/api/license/entitlements` entirely. Infrastructure, Workloads, alerts,
first-session gated routes, and relay settings must take feature truth from
`/api/license/runtime-capabilities` or already loaded commercial posture, and
the governed browser proof in
`tests/integration/tests/11-first-session.spec.ts`,
`tests/integration/tests/journeys/01-smoke-bootstrap-login-infrastructure.spec.ts`,
and `tests/integration/tests/journeys/03-relay-pairing.spec.ts` must continue
to assert zero browser-shell entitlement requests outside owned billing flows.
That same authenticated-shell commercial boundary may surface prerelease
guidance only as a thin release-metadata consumer. `frontend-modern/src/AppLayout.tsx`
may mount a shared release-candidate callout when the resolved version channel
is `rc`, but that shell copy must stay non-billing: it may link to release
notes and feedback, yet it must not probe billing endpoints, expose licensed
identity, or infer upgrade pressure from plan state just to explain the
current prerelease.
That same entitlement boundary also owns internal demo-fixture grants.
`FeatureDemoFixtures` may be issued only for governed internal demo runtimes,
must never join public tier defaults or pricing contracts, and must be
filtered back out of browser-facing capability and entitlement payloads even
when the runtime uses it to authorize release-build mock fixtures.
That same browser/store boundary also owns typed non-billing commercial
selectors. `frontend-modern/src/stores/licenseCommercial.ts` may interpret the
commercial-posture payload once for browser consumers, but relay, Patrol,
trial banners, monitored-system warnings, and settings paywall state must not
branch directly on raw `subscription_state`, `trial_eligible`,
`trial_days_remaining`, or `overflow_days_remaining` fields in each leaf
surface.
Legacy Cloud plan aliases are now expected to canonicalize to the `cloud_*`
contract not only when Stripe metadata is parsed, but also when persisted plan
versions are consumed at hosted entitlement and workspace-limit enforcement
boundaries.
That same hosted browser bootstrap boundary now owns browser-only lifecycle
attachment too. `frontend-modern/src/useAppRuntimeState.ts` must register its
auth bootstrap, theme sync, and post-reconnect hosted refresh work through
Solid `onMount(...)` inside the runtime owner instead of letting `App.tsx`,
`AppLayout.tsx`, or module-evaluation side effects reach directly into browser
APIs before the hosted shell has mounted.
That same hosted browser shell boundary also owns same-path query-state
transitions that refresh hosted/org context without changing the active page.
`frontend-modern/src/App.tsx` must preserve the mounted `.app-scroll-shell`
position across those remount-like transitions so inline row focus, hosted
drawer state, and org-context route updates do not present as a full-page
refresh.
Persisted billing state is now also part of that canonical boundary: when a
recognized Cloud/MSP plan version is loaded or saved, the stored `plan_version`
must canonicalize and retired monitored-system volume limit keys must be
scrubbed rather than reconciled back into runtime state.
That same persisted billing boundary now also applies to hosted entitlement
lease secrets: `internal/config/billing_state.go` may keep `EntitlementJWT`
and `EntitlementRefreshToken` in runtime billing state, but `billing.json` may
not persist either value raw. Canonical persistence must keep the hosted lease
JWT and refresh token encrypted at rest, rewrite legacy plaintext billing files
on load, and drop those secrets instead of preserving plaintext-at-rest billing
state if encryption cannot be established. Empty/no-secret billing state may
not auto-create new crypto state just to add integrity metadata; no-key
graceful degradation remains canonical until a real billing secret or real
encryption key exists.
That same hosted billing boundary also owns base-path resolution:
`internal/config/billing_state.go` and
`internal/api/payments_webhook_handlers.go` must derive their base data
directory from the shared runtime data-dir helper in
`internal/config/config.go` instead of each carrying a private `/etc/pulse`
fallback. Hosted billing leases, webhook dedupe state, and customer indexes
must therefore follow the same configured runtime data-dir authority as the
rest of the product.
The same secret-at-rest rule also applies to activation state persistence:
`pkg/licensing/activation_store.go` may keep `InstallationToken` and
`GrantJWT` in runtime activation state, but `activation.enc` may accept
plaintext only as migration input. Once a legacy plaintext activation file can
be read, canonical persistence must rewrite encrypted storage immediately on
load instead of treating plaintext as a steady-state runtime path.
That same rule also applies to the canonical local license state in
`pkg/licensing/persistence.go`: `license.enc` may carry the commercial license
key and grace-period metadata in runtime state, but a legacy plaintext
`license.enc` may only serve as migration input. Once it can be read,
canonical persistence must rewrite encrypted storage immediately on load
instead of treating plaintext licensing state as a valid steady-state path.
That same local-persistence boundary also owns the filesystem path contract for
commercial secrets at rest. `pkg/licensing/persistence.go` and
`pkg/licensing/activation_store.go` must normalize the owned config directory
once and resolve only the fixed `.license-key`, `license.enc`, and
`activation.enc`, and `instance-fingerprint` leaves through the shared
storage-path helper before any filesystem read, write, rename, stat, or delete.
Future licensing persistence changes must not bypass that resolver with raw
`filepath.Join(configDir, ...)` joins or introduce caller-controlled
persistence filenames. The `instance-fingerprint` file is not an activation
credential and must survive clear-license flows: native activation and legacy
exchange must reuse it as the stable local installation identity so one machine
does not consume another paid installation slot after a local activation clear,
container recreation, or retry.
That same local-persistence boundary also owns writable-but-not-owned runtime
storage semantics for commercial state. `pkg/licensing/persistence.go` may
harden directories it owns to `0700`, but it must not assume it can chmod the
root of a writable Kubernetes or container-mounted runtime data directory. When
the mount root is writable but not owned by the Pulse process, canonical
persistence must keep file-level secrets hardened at `0600`, validate that the
resolved storage root is still the expected real directory rather than a symlink
or other filesystem object, and continue operating instead of crashing just
because the mount root itself cannot be chmod-ed.
Hosted entitlement-source loading follows the same rule: `DatabaseSource` must
normalize persisted Cloud/MSP plan aliases and legacy limit keys before runtime
evaluation, but it must not fabricate a canonical `plan_version` from bare
subscription lifecycle state when the stored plan label is absent.
Stripe control-plane fallback paths are also part of the boundary: when
subscription or workspace provisioning logic reuses an already stored
`plan_version`, it must canonicalize that value before persisting tenant,
Stripe-account, or billing-state updates.
Public cloud self-serve signup follows the same rule: `internal/cloudcp/config.go`
and `internal/cloudcp/public_cloud_signup_handlers.go` must accept only
canonical `cloud_*` Stripe prices for public hosted signup tiers and fail
closed when misconfigured so self-hosted or prelaunch-only prices cannot leak
into live public checkout flows. Until the governed v6 cutover explicitly
opens that path, `internal/cloudcp/routes.go` and the hosted portal bootstrap
must also keep public cloud signup links and route registration disabled even
if canonical Cloud price IDs are already present for release-day readiness.
That prelaunch gate must not disable `/api/public/magic-link/request`, because
existing hosted commercial accounts still depend on the public portal sign-in
flow before v6 public Cloud checkout is opened.
Signed hosted entitlement leases are part of the same boundary: lease signing
and verification must canonicalize recognized Cloud plan aliases and delete
retired monitored-system volume keys instead of trusting stale embedded values.
They also must not fabricate `plan_version` from bare `subscription_state`
when the signed lease claim label is absent.
The control-plane registry is also canonical: tenant and Stripe-account
`plan_version` rows must canonicalize recognized Cloud aliases on read and
write so stored legacy values cannot re-enter provisioning, entitlement, or
limit-enforcement fallbacks.
JWT-backed entitlement claims are also canonical: when runtime evaluation uses
claim `plan_version` and `limits`, recognized Cloud plan aliases must
canonicalize and retired monitored-system volume keys must be removed instead
of trusting stale embedded claim values. When a Cloud/MSP claim arrives without
a recognized plan label, runtime must preserve the missing/unknown
`plan_version` metadata while still treating monitored-system volume as
unmetered.
Activation-grant translation is part of the same boundary: when relay/license
server grants enter the local claims model, Cloud plan keys and lifecycle state
must still resolve through the canonical entitlement claim accessors rather
than becoming a parallel truth path. The activation grant `exp` remains the
short-lived enforcement and refresh lease; recurring self-hosted license period
display must come from the optional grant `current_period_end` mapped into the
local claims model and exposed through `/api/license/status` `expires_at` and
`days_remaining`. Older grants that omit `current_period_end` may fall back to
the grant/JWT expiry for compatibility, but new recurring grants must not make
the UI present the 72-hour relay lease as the customer's subscription end date.
The shared grant wire shape must stay aligned across the Pulse client, the
`pulse-pro` license server issuer, and the relay-server grant validator.
The legacy-license exchange transport is part of that same activation boundary:
`pkg/licensing/activation_types.go` and `pkg/licensing/license_server_client.go`
must treat `legacy_license_token` as the canonical v6 request field for
`POST /v1/licenses/exchange`, while accepting `legacy_license_key` only as a
backward-compatible decode alias for older local stubs and historical test
fixtures. Future exchange-path changes must not reintroduce a split contract
where the shared Pulse runtime and the real `pulse-pro/license-server` disagree
on the activation payload shape.
That same license-server transport boundary now treats Patrol quickstart
bootstrap as retired, not as a mixed-version runtime extension point.
`pulse-pro/license-server` must not register `/v1/quickstart/*` routes, parse
model-provider quickstart env, or create new quickstart ledger tables. The
Pulse runtime must not call `POST /v1/quickstart/bootstrap`, must not persist
quickstart-backed AI config, and must not mint hosted-model tokens from hosted
or self-hosted billing state. Historical quickstart credit fields may remain
parseable in billing state only so old HMAC-protected files can still load.
The self-hosted commercial counted-unit contract is retired. Monitored systems
remain a canonical inventory grouping and support-debugging concept, but
`max_monitored_systems`, legacy `max_agents`, and legacy `max_nodes` are
decode-only compatibility inputs at the storage, grant, or purchase boundary.
Runtime enforcement, entitlement payload current usage, checkout/activation
flows, and upgrade messaging must not treat monitored-system volume as a cap.
Portal shell copy, pricing explainers, and upgrade helper text must focus on
paid feature value rather than monitored-system expansion.
That same retired counted-unit contract still owns prospective grouping
preview. Proxmox/PBS/PMG config adds, TrueNAS adds, VMware inventory previews,
and equivalent updates may ask the canonical monitored-system projection how a
connection groups into top-level systems before persistence, including
replacement-aware projections for source swaps on an existing grouped host,
but those previews are informational and cannot block a save for commercial
volume reasons.
The monitored-system ledger settings surface now also explains those grouping
decisions. Support UI may show grouped monitored systems, but it must render
the canonical backend explanation for why one or more top-level views group as
a single monitored system instead of inventing support copy or merge heuristics
in the frontend.
That billing support surface must also remain readable while mixed-version
clients and servers roll forward: missing explanation payloads may degrade to a
safe generic explanation, but the monitored-system ledger must never fail the
page or hide counted systems because the nested support details are absent.
That same monitored-system presentation boundary now also owns the customer-
facing ledger loading and retry copy through
`frontend-modern/src/utils/monitoredSystemPresentation.ts`, so commercial
usage states do not leak back into lifecycle inventory helpers.
That same cloud-paid boundary also owns the shared entitlement and upgrade
presentation helpers through `frontend-modern/src/utils/licensePresentation.ts`
and `frontend-modern/src/utils/upgradePresentation.ts`. Commercial tier labels,
feature minimum-tier messaging, migration/trial notices, billing-admin status
labels, and generic upgrade CTA labels must extend those helpers instead of
being forked into per-surface status-code branches that drift from backend
error truth. Button-styled upgrade CTA chrome is owned by
frontend-primitives' `UpgradeButtonLink`; commercial plan surfaces may choose
the destination, label, and emphasis but must not recreate local Tailwind
button anchors. The old self-hosted trial-start acquisition path is not a
current v6 commercial surface; denial and retry copy for ordinary self-hosted
runtimes must be limited to owned purchase, recovery, portal, Cloud signup, or
support handoff paths.
That same helper boundary also owns generic settings-paywall CTA labels. Under
the v6 free-first self-hosted policy, non-billing feature gates must stay
factual and route deliberate commercial intent to Plans with neutral "View
plans" copy; they must not start a Pro trial directly or embed local
`Upgrade to Pro` / `Start free trial` strings. Direct trial-start actions stay
retired for ordinary self-hosted v6; explicit commercial intent routes through
plan comparison, purchase activation, recovery, Cloud, or support handoff
surfaces where the operator has already chosen that path.
The same shared self-hosted commercial presentation boundary also owns the
recovery-surface copy for `SelfHostedCommercialRecoverySection.tsx`,
including the existing-key field label/help text and legacy-key exchange
notice, while `selfHostedBillingPresentation.ts` owns the first-class plan and
license-state copy that belongs on the primary billing surface. That split
prevents manual key redemption from drifting back into the main purchase path.
The recovery key input is also a frontend-primitives consumer:
`SelfHostedCommercialRecoverySection.tsx` must compose `FormTextarea` for the
manual license-key field so label/id/help wiring and textarea chrome remain
shared while cloud-paid continues to own the commercial copy and recovery
actions.
That same recovery boundary also owns the linked legal surface:
`SelfHostedCommercialRecoverySection.tsx` must route its Terms-of-Service link
through the shipped `TERMS.md` docs asset instead of sending operators to
GitHub `main`, so the recovery trust surface stays version-matched and
available on restricted installs. The anchor shell, rel policy, and compact
link styling for that legal/docs link belong to frontend-primitives'
`ExternalTextLink`; cloud-paid owns the commercial recovery copy and
destination semantics.
Already-issued legacy hosted trial leases are also part of that same
compatibility contract. If such a lease is refreshed, it must carry the
canonical Pro capability set while scrubbing retired monitored-system volume
limits rather than relying on downstream runtime fallback to infer commercial
state from trial metadata.
The top-level authenticated shell is part of that same customer-facing
boundary: cloud-paid trial prompts may appear in owned commercial surfaces, but
the app shell must not force a global, persistent Pro trial nudge that
overrides the primary runtime chrome for every signed-in user.
The maintained Pulse Account portal frontend is part of that same boundary for
local development as well as runtime delivery. Styling or interaction work on
`internal/cloudcp/portal/frontend/` must have a local preview path that uses
the same frontend build pipeline as the embedded production bundle, while
serving scenario-backed local portal APIs without requiring a live cloud deploy
for every iteration. Final verification still belongs on a real control-plane
instance, but local portal design work must not depend on redeploying
`cloud.pulserelay.pro` just to see each frontend change.
That same preview/runtime boundary also owns shared browser chrome such as the
portal favicon: the local preview must serve the same `/favicon.svg` asset as
the real control-plane route so icon changes can be reviewed locally before
deployment instead of appearing only after a live push. The portal page itself
must also reference that shared favicon through a versioned href so updated
icon revisions bypass browser cache on deploy instead of waiting for asset
expiry.
That same frontend delivery boundary must keep the account portal visual language sharp and restrained, avoiding gradients, heavy shadows, decorative SaaS styling, and shell chrome that dominates the active task. The canonical target is a calm, flat account-operations surface closer to GitHub Settings or Android Settings than to a dashboard: a compact identity bar (account name, role, kind), a horizontal tab bar for section navigation, and direct content panels for Workspaces, Access, Billing, and Support. Shell framing uses white backgrounds, 1px borders, minimal border-radius, and hierarchy driven by spacing and typography rather than cards, pills, or ornamental headers. Section block headers render inline actions only inside the data container toolbar row rather than floating orphaned above content; fact chips, summary strips, decorative kicker labels, and redundant section headings do not appear in the primary task surface. Action buttons (Create workspace, Invite people, Change roles, Remove access) are integrated into toolbar rows within their respective bordered data cards rather than existing as free-floating elements above the content.
That same portal delivery boundary also owns the checked-in embedded bundle in
`internal/cloudcp/portal/dist/`. Visual or interaction changes are not
complete when they exist only in the local preview runtime or in hand-edited
generated output. The canonical edit path remains the frontend source under
`internal/cloudcp/portal/frontend/`, and any ship-ready portal styling change
must regenerate the embedded bundle plus the source-hash manifest so the live
control-plane, embedded Go template, and local preview all run the same
frontend revision.
That same shell framing also owns user-facing prerelease labeling for
rc-channel builds. `frontend-modern/src/AppLayout.tsx` may still key off the
canonical `rc` channel metadata internally, but the visible badge must frame
those builds as a preview/prerelease experience rather than implying a
near-ready release candidate. That authenticated shell must not pair the
preview label with a second top-of-shell release-candidate warning banner or
release-feedback CTA in `frontend-modern/src/AppLayout.tsx`; prerelease cloud
posture stays a subtle shell label, not a public-RC callout inside the paid
runtime chrome.
The self-hosted trial-start frontend runtime is retired for v6 GA. Commercial
relay, onboarding, setup, Pro settings, and shared paywall surfaces may still
offer explicit plan, activation, recovery, support, or hosted handoff paths, but
they must not revive local `startProTrial()` branches, shared trial-start
helpers, or global trial acquisition banners in the normal self-hosted app.
The same commercial handoff rule also covers the legacy `/pricing` route in
`frontend-modern/src/pages/PricingHandoff.tsx` and
`frontend-modern/src/utils/pricingHandoff.ts`: compatibility handoff may keep
product-owned destinations such as monitored-system billing or Cloud plans
inside the app, but self-hosted commercial upgrade intents now hand off to
`Pulse Account` first instead of treating the public pricing page as the app's
canonical destination. `Pulse Account` now owns self-hosted plan comparison,
checkout-session creation, and the purchase return handoff back into Pulse via
`/auth/license-purchase-activate`; Pulse itself must not render a second
public pricing surface inside the runtime, and the authenticated portal shell
must not collapse back into a public-site-only waystation for new-purchase
depth. That handoff now starts through Pulse-owned `GET /auth/license-purchase-start`,
which mints a signed `purchase_return_token`, creates a commercial-owned
`portal_handoff_id` that resolves to the bound checkout intent plus Pulse
success/cancel targets, and sends only that opaque `portal_handoff_id` to
`Pulse Account` before the browser leaves for the portal. `Pulse Account`
must resolve that server-owned handoff through the shared commercial API
before it starts checkout, so the portal never trusts browser referrer state,
loose `feature` / `return_url` query parameters, or a raw browser-supplied
`checkout_intent_id` for self-hosted purchase completion. The portal proxy
must expose `GET /v1/checkout/portal-handoff` as the canonical browser
bootstrap, and the retired `GET /v1/checkout/intent` bootstrap must stay
removed once `portal_handoff_id` is the owned contract. The browser-facing
portal handoff response must reveal only portal-owned bootstrap metadata such
as `portal_handoff_id`, feature, handoff lifecycle state, resolve timestamp,
and expiry; it must not disclose the bound `checkout_intent_id`, and browser
checkout creation must post only `portal_handoff_id` so Pulse Account
resolves the private checkout intent server-side before contacting Stripe.
For self-hosted upgrades, that browser-facing feature metadata is now
canonically `self_hosted_plan`. Pulse Account may continue normalizing the
legacy `max_monitored_systems` alias only for already-created checkout records,
but in-product browser links and fallback helpers must route that alias to the
neutral self-hosted plan surface instead of the monitored-system
usage/counting-rules surface. The portal proxy contract, checked-in embedded
bundle, and rendered upgrade copy must treat self-hosted commerce as plan
selection and paid extras rather than monitored-system-cap expansion. Shared
helpers and route-owned browser symbols should name that commercial state as
plan selection as well; the monitored-system alias belongs only to
backward-compatible handoff normalization and non-commercial usage review.
If Pulse cannot create or resolve that portal handoff locally, the Pulse-owned
start route must still return the operator to the owned billing plan surface
with an explicit `purchase=unavailable` arrival instead of leaving the browser
on a raw error page, so the runtime can explain that Pulse Account is
temporarily unavailable and offer a retry from the same instance.
That portal handoff is now intentionally narrowly stateful rather than a bare
lookup row: lookup must stamp a first `resolved_at`, and the reported handoff
lifecycle must stay derived from the owned portal handoff plus the bound
checkout intent (`created`, `resolved`, `checkout_started`, `completed`) so
Pulse Account can distinguish a fresh upgrade bootstrap from a resumed or
already-completed checkout without reviving browser-owned commercial state.
That same owned handoff row is now also the canonical self-hosted purchase
binding record: it must persist the signed `purchase_return_jti`, the bound
Stripe `session_id`, and the lifecycle timestamps that prove when checkout was
resolved, started, and completed. Stripe success must carry that same
`portal_handoff_id` back into Pulse's activation callback, and Pulse must
cross-check both `portal_handoff_id` and `purchase_return_jti` against the
commercial session result before local activation so the browser no longer
trusts Stripe metadata or local form state alone for return integrity.
Once that commercial binding verifies, Pulse must not fall back to a generic
JTI replay tombstone. The local activation callback must persist a dedicated
purchase-return redemption record keyed by `portal_handoff_id` plus
`purchase_return_jti`, stamp explicit local redemption state
(`started`, `activated`, `failed`), and use that owned record to make
completed returns idempotent while still allowing retry after transient local
activation failures.
Stripe success now lands on Pulse's public
`frontend-modern/src/utils/pricingHandoff.ts` and
`frontend-modern/src/pages/PricingHandoff.tsx` may only hand operators into
Pulse-owned `GET /auth/license-purchase-start`; they must not construct
Pulse Account portal URLs, `return_url` parameters, or other portal-entry
contract state in the browser once the server owns that handoff boundary.
`GET /auth/license-purchase-activate` bridge, which auto-submits into the
owned POST activation path; the portal must not render a second manual
`Activate in Plans & Billing` step after checkout. Stripe cancel must return
directly to the owned Pulse billing plan route rather than back into the
portal. The owned activation callback must accept the signed state, redeem the
completed checkout, and return the operator to the canonical billing plan
route with an explicit owned purchase arrival state (`purchase=activated`,
`purchase=expired`, `purchase=failed`, or `purchase=unavailable`) whether
checkout completed in a secondary tab or in the current-tab fallback path.
When billing preserves `intent=self_hosted_plan` across that return, the
runtime may keep the operator on the compare-plans prompt, but any visible
`Compare plans` action must still route through owned
`GET /auth/license-purchase-start?feature=self_hosted_plan` instead of
constructing a direct Pulse Account URL in the browser.
That same purchase-return contract also narrows the insecure local-development
carve-out: signed purchase-return callbacks may use plain HTTP only for
loopback hosts such as `localhost` or `127.0.0.1`. RFC1918, `.local`, and
other LAN-visible hosts must use HTTPS so the token-bearing return state is
not exposed to same-subnet sniffing during commercial checkout return.
That destination split is canonical commercial truth, but navigation semantics
are not owned here. `frontend-modern/src/utils/pricingHandoff.ts` and
`frontend-modern/src/stores/license.ts` decide which href each commercial
feature resolves to; `frontend-primitives` owns the typed navigation contract
that decides whether that href stays in-app or opens externally. Commercial
surfaces must not re-infer that behavior locally with per-component
`target="_blank"` or `window.open(...)` branches once a feature can resolve to
either product-owned billing/cloud routes or the public pricing site.
The self-hosted trial-start boundary is now retired for ordinary v6 GA
runtimes. Local `POST /api/license/trial/start` must not be registered as an
in-app acquisition route, and it must not return the old
hosted-signup or trial-rate-limit payloads from a normal self-hosted instance.
The retired `/auth/trial-activate` return path must also stay out of the
ordinary self-hosted router and Pro settings UI. Hosted/cloud entitlement lease
refresh may still validate signed leases for already-approved hosted state, but
ordinary self-hosted Pulse must not create a local trial acquisition callback.
The remaining `TrialActivation*` verifier names and
`PULSE_TRIAL_ACTIVATION_PUBLIC_KEY` environment literal are boundary-only
compatibility for hosted entitlement lease signing and already-deployed tenant
configuration. New entitlement runtime call sites must use the
`HostedEntitlement*` aliases unless they are explicitly proving the retired
callback boundary, and changing the environment literal requires a separately
governed credential rollout.
The matching control-plane acquisition routes are retired too:
`/start-pro-trial`, `/trial-signup/*`, and `/api/trial-signup/*` must remain
unregistered. `/api/entitlements/refresh` remains the only hosted entitlement
refresh endpoint for already-issued hosted leases.
Browser and shell coverage now guard that retired boundary:
`tests/integration/tests/07-retired-trial-acquisition.spec.ts` and
`tests/integration/scripts/retired-trial-acquisition-contract.sh` must expect `404` from the
retired route and prove entitlements remain unchanged. The paid-prompt browser
proof in `tests/integration/tests/58-self-hosted-trial-rate-limit-ui.spec.ts`
must keep trial CTAs and paid-only navigation out of the default self-hosted UI.
`scripts/tests/test-retired-trial-acquisition-docs.sh` guards the same documentation posture
so active operator docs and eval metadata describe the retired route instead of
the old hosted-signup acquisition contract.
Hosted tenant organization seeding and hosted handoff role mapping now belong
to the same cloud-paid truth too. `internal/cloudcp/stripe/provisioner.go`
must seed tenant org members from the shared account-role-to-organization-role
mapping, and hosted handoff/runtime consumers must preserve that same mapping
when older workspaces need membership continuity repaired at open time. Cloud,
MSP, and commercial account roles therefore have one canonical translation into
tenant org permissions instead of drifting between provisioning-time seeds and
handoff-time access repair.
The old self-hosted trial activation return notice and `/settings/system-pro`
route are retired with that callback: the `trial` query result must not produce
owned activation UI, a success banner, retry copy, or a settings compatibility
redirect in v6 GA. Purchase activation continues through the Pulse Account
return contract instead.
That same retirement applies to the old hosted control-plane completion and
redeem routes: `/trial-signup/complete` and `/api/trial-signup/redeem` must
not return customer-facing trial UX or raw acquisition errors in v6 GA.
Commercial plan disclosure copy must not be redefined inline in settings
feature gates, Pro license panels, or upgrade nudges.
That same disclosure copy must stay professional and customer-facing. The
settings ledger may expose counting details and included collection paths, but
it must avoid informal debug-style labels or ad hoc wording that makes the
commercial usage surface feel provisional.
That same governed ledger support surface now also owns backend-authored status
explanation copy beside the monitored-system status label. The frontend details
view may normalize a safe default when that field is absent during mixed-version
rollouts, but it must render the canonical backend explanation when present
instead of inventing page-local wording for what warning, offline, or unknown
means on a counted monitored system.
That same cloud-paid surface must now also render the canonical status reason
list when present, so customers can see exactly which grouped source or
top-level surface degraded and its canonical `reported_at` timestamp rather
than only reading generic status copy beside a fresh aggregate `Last Seen`
value.
That same settings surface must also label the monitored-system signal by its
real meaning. The canonical API shape is now the structured
`latest_included_signal` object, and the normalized frontend client contract
should expose only that canonical object. Retired flat alias fields must not
re-enter the live backend payload or client contract. It represents the freshest
included grouped observation, not a guarantee that every grouped source is
healthy, so the UI must not present it with single-source `Last Seen` wording.
When the canonical object is present, the surface should use its
source/name/type attribution instead of showing an unqualified aggregate
timestamp, and that attribution should stay customer-facing rather than
exposing raw monitored-system type/source slugs in the settings table or
included-surface details. When the row expands to show status reasoning, it
should also restate the freshest included surface and timestamp there so a
degraded reason and a fresher grouped signal remain readable together. When
mixed-version payloads omit that canonical freshest signal entirely, the
settings surface should degrade to a safe customer-facing
fallback instead of an unexplained placeholder glyph.
That same billing support boundary now also owns the shared monitored-system
presentation helper. `frontend-modern/src/utils/monitoredSystemPresentation.ts`
is the canonical owner for monitored-system brief/disclosure copy, ledger
labels, safe fallback summaries, source/type attribution wording, and the
customer-facing monitored-system usage/migration strings reused by the shared
settings and support ledger surfaces, so those surfaces must consume that
helper instead of redefining customer-facing monitored-system copy inline or
keeping a parallel copy in generic self-hosted plan utilities.
That same helper also owns preview-impact copy and current/projected
source-label wording for optional monitored-system impact previews, so the
TrueNAS and VMware settings panels do not drift into provider-local billing
phrasing when they surface canonical preview results. Impact preview summaries
must also come from that helper and describe current/projected count impact
without raw `current / limit` quota math.
That same helper-owned impact-preview contract now also owns unavailable
verification messages. Provider settings
panels must map `monitored_system_usage_unavailable` plus backend
`details.reason` through `frontend-modern/src/utils/monitoredSystemPresentation.ts`,
but those preview failures are explanatory only and must not disable or block
connection saves.
That same disclosure surface must not accept arbitrary caller-supplied
monitored-system summary strings as its primary API. When the disclosure needs
to show brief summary copy, it should render the canonical helper-owned brief
summary rather than giving callers a free-form summary prop that can drift from
the governed monitored-system language.
That same helper now also owns the row-status fallback summaries used when
mixed-version payloads omit `status_explanation.summary`. The API client and
settings panel must derive online/warning/offline/unknown fallback text from
that shared owner instead of keeping separate local defaults that can drift.
That same customer/support-facing ledger surface must consume the canonical
`/api/license/monitored-system-ledger/explain` model when showing counted
systems. Rows must expose backend-authored `explanation.summary`, reasons, and
grouped source surfaces through helper-owned labels so customers can see why a
system counts once without the panel inventing a second counting model.
When canonical usage is not safe to read yet and the backend returns
`monitored_system_usage_unavailable`, the support ledger must render
helper-owned verification copy instead of a synthetic count. The canonical
unavailable reason mapping belongs to
`frontend-modern/src/utils/monitoredSystemPresentation.ts`; Pro license panels
and ledger panels must consume that helper instead of rechecking availability
or hard-coding `Verifying...` / `Unavailable` formatting locally.
The entitlement payload builder must only mark monitored-system usage available
from an explicit canonical `MonitoredSystemsAvailable` signal supplied by the
runtime usage boundary. Deprecated compatibility aliases such as `Nodes`, or a
non-zero raw monitored-system count without that availability signal, must not
drive current usage, limit state, or customer-facing cap warnings.
The same usage view must not render monitored-system continuity context from
the entitlement payload as a live commercial cap. Base plan limits, effective
limits, grandfathered floors, and capture state are retired compatibility
metadata, not customer-facing self-hosted capacity controls.
beside the canonical count explanation.
That same settings owner split now also requires
`frontend-modern/src/components/Settings/MonitoredSystemLedgerPanel.tsx` to
consume the normalized ledger client contract directly. Panel-local fallback
reconstruction for `status_explanation`, `explanation`, or
`latest_included_signal` belongs in
`frontend-modern/src/api/monitoredSystemLedger.ts` instead, so the billing
surface stays a pure render shell over one canonical monitored-system ledger
shape.
Frontend billing/admin surfaces must not synthesize `plan_version` from
subscription lifecycle state. When a hosted billing record lacks a plan label,
the UI must preserve that absence instead of fabricating values like `active`
or `suspended` into the canonical plan field.
The hosted billing-admin settings surface is now part of the explicit
cloud-paid ownership model as well. Changes to
`frontend-modern/src/components/Settings/BillingAdminPanel.tsx` must carry this
contract and the dedicated billing-admin proof file instead of remaining an
unowned consumer of hosted billing state.
That hosted billing-admin owner is now intentionally split by responsibility:
`frontend-modern/src/components/Settings/BillingAdminPanel.tsx` is the
settings shell, `frontend-modern/src/components/Settings/useBillingAdminPanelState.ts`
owns the hosted organization list, billing-state cache, preload, and
subscription update runtime, and
`frontend-modern/src/components/Settings/BillingAdminOrganizationsTable.tsx`
owns the tenant grid plus expanded JSON presentation. Future hosted billing
admin changes must extend that split instead of pulling hosted state and
mutation flow back into the panel render shell.
Hosted billing admin organization row actions follow the shared frontend
primitive contract as well: cloud-paid owns Suspend, Activate, Reload, tenant
billing state, and mutation semantics, while frontend-primitives owns the
`Button` chrome for those row actions.
The organization billing settings surface now follows the same rule. Changes
to `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx` must
carry this contract and the dedicated organization-billing proof file instead
of remaining an unowned consumer of plan tier, entitlement limits, and
usage-versus-cap presentation.
That same billing surface now also normalizes org scope through
`frontend-modern/src/utils/orgScope.ts` before it builds per-tenant state, so
cache keys and tenant lookups do not keep a local `getOrgID() || 'default'`
fallback in the hosted billing UI.
That hosted organization billing owner is now intentionally split by role:
`frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx` is the
settings shell, `frontend-modern/src/components/Settings/useOrganizationBillingPanelState.ts`
owns org-scoped billing/runtime lifecycle and plan/usage model wiring, and
`frontend-modern/src/components/Settings/OrganizationBillingLoadingState.tsx`
owns the loading skeleton. Future billing work must extend that split instead
of pulling org-switch lifecycle and hosted entitlement reads back into the
panel render shell.
The self-hosted and hosted billing surfaces now also share a canonical
commercial page shell and plan/usage model. `ProLicensePanel.tsx` and
`OrganizationBillingPanel.tsx` may still differ in deployment-specific actions
and context, but `CommercialBillingSections.tsx` and
`frontend-modern/src/utils/commercialBillingModel.ts` now own the shared
commercial information architecture. Future billing work must extend that
shared shell/model first instead of letting self-hosted Pulse Pro and hosted
organization billing drift back into parallel local layouts or vocabularies.
For self-hosted Pulse Pro specifically, the plan/usage split is now also a
router contract: `/settings/system/billing/plan` is the canonical plan state,
and `/settings/system/billing` remains a compatibility handoff rather than the
primary owned destination. `/settings/system/billing/usage` is only a
compatibility support state for a displayable bounded monitored-system context
that remains after recognized self-hosted v6 plan labels have been normalized.
Community, Relay, Pro, Pro+, lifetime, and eligible grandfathered recurring
plans must not keep a standing Usage subtab alive just to restate counted-unit
totals or stale migration volume metadata; direct `/usage` arrivals on those
no-cap plans should canonicalize back to `/plan`. Arrival-specific UI
affordances belong to that same owned billing surface: legacy usage arrivals
may open counting rules by default only when a bounded support context is still
displayable, and plan arrivals may surface an upgrade callout that hands off
into `Pulse Account` for self-hosted plan comparison and checkout before
returning to the owned plan state through the runtime-owned activation
callback. Pulse product routes
keep ownership of license status, usage, and activation state; `Pulse Account`
owns the commerce flow itself. That same self-hosted settings surface is
plan-owned rather than tier-owned: the navigation label and surface title are
`Plans & Billing`, and the canonical plan state must make
the current tier plus capabilities immediately obvious so existing paid
upgrades can confirm what their new key enabled without hunting through
generic billing details. That capability-first summary must stay tier-specific
and continuity-aware: Relay and Pulse Pro should describe the actual paid
capabilities available on this instance, while grandfathered pricing must make
existing-price protection explicit without reviving finite monitored-system
continuity copy for normalized no-cap self-hosted plans.
That top card is plan-owned, not raw subscription-state-owned: Community copy
should describe what is included on this instance in plain language, and the
current-plan badge must not present the fallback Community state as `Expired`
just because a prior paid subscription or trial ended.
Upsell on the self-hosted Plans surface must stay contextual and subordinate
to current-plan clarity. The top card confirms the current tier first; only
below that may Pulse show a factual comparison section for the next higher
tiers. Community may compare Relay and Pulse Pro, Relay may compare Pulse Pro,
and active Pulse Pro should not show a promotional higher-tier block on this
surface.
Fresh activation is part of that same governed plan state. After a checkout
return, trial handoff, or pasted-key activation succeeds, the owned plan
surface must show an explicit success summary that names the active tier and
the marquee capabilities now available on this instance, rather than
falling back to a generic "activated" banner that leaves the user to infer
what changed from the steady-state billing card alone.
For active Pulse Pro/Lifetime tiers with the private Pro runtime available,
both the current-plan summary and the activation-success body must make the
next setup decision explicit: set Patrol mode, then let Patrol investigate,
act within that policy, verify outcomes, and keep context/history. Completed
verified Patrol outcomes are telemetry on this surface, not a reason to replace
the plan action with a default `Patrol history` CTA; when no active Patrol work
or pending governed decision exists, the paid-plan action returns to
`Choose Patrol mode`.
When the activated entitlement includes the governed Pulse Intelligence
operations-loop capability, that success summary may hand the user directly to
Patrol with a "Choose Patrol mode" action. The cloud-paid presentation contract
must name the action URL and action intent as Patrol-control concepts
(`PATROL_CONTROL_STARTER_URL` and `patrol_control`) rather than exposing
operations-loop names in the Plan surface. That action is still
cloud-paid-owned activation UX, but its destination must use the route-owned
Patrol control target
(`/patrol?patrolControlStarter=patrol_control#patrol-control`) so Patrol
owns starter recording through the shared Pulse Intelligence starter contract.
The legacy `patrol_autonomy` and `pulse_pro_activation` route and activity
markers remain compatibility input only, and the shared
`pulse_operations_loop` telemetry workflow marker may remain behind the Patrol
control surface. The Plans surface must not create a commercial-only analytics
counter, must not record prompt text or resource context, and must not treat the
click as proof that Patrol found an issue, Assistant explained it, a governed
action was approved, or the outcome was verified.
That same router-owned billing contract now also includes recovery as a plan
detail state instead of a fragment alias. The canonical recovery arrival is
`/settings/system/billing/plan?details=recovery`, while
`#pulse-pro-recovery` remains a compatibility deep link that must normalize
into that owned query state. `purchase=failed` arrival handling on the
self-hosted billing surface must consume the one-shot purchase result, preserve
the owned plan intent when present, and replace the URL with the canonical
recovery detail route so `Open recovery` lands on a first-class billing state
instead of a hash that settings-shell normalization strips away.
That same ownership split is explicit in the governed registry as well:
`CommercialBillingSections.tsx` is part of the shared commercial shell/model
surface, while `SelfHostedCommercialRecoverySection.tsx` stays on the
self-hosted Pro recovery surface with `ProLicensePanel.tsx` rather than
floating as an unowned settings fragment.
Hosted tenant browser bootstrap is part of that same cloud-paid boundary as
well. After control-plane or magic-link handoff, the browser client must
preserve the tenant-scoped `pulse_org_id` context that the server issued
instead of clearing it on first page load or collapsing back to `default`
simply because hosted tenants do not expose self-hosted multi-tenant admin
capabilities. Hosted runtime entry must therefore treat tenant org context as
infrastructure state, not a paid UI toggle that can be discarded during app
bootstrap.
That hosted browser bootstrap owner is now intentionally split by role:
`frontend-modern/src/App.tsx` is the route/provider entry shell,
`frontend-modern/src/useAppRuntimeState.ts` owns authentication, org bootstrap,
theme synchronization, and authenticated runtime startup, and
`frontend-modern/src/AppLayout.tsx` owns authenticated cloud-aware chrome such
as org switching and kiosk-safe navigation. Future hosted browser bootstrap
work must extend that split rather than pulling org bootstrap and app chrome
back into one monolithic route component.
The global update progress watcher inside that entry shell is self-hosted
server-update chrome: its stage vocabulary tracks the backend updater
pipeline (downloading through restarting, including the `restoring` stage
emitted by update rollback) and it carries no hosted entitlement, org, or
paid-gating semantics; hosted and paid surfaces must not key tenant or
billing behavior off update progress stages.
That same authenticated shell split must also respect shared blocking dialogs:
hosted chrome may not leave the Pulse Assistant launcher or an already-open
assistant drawer interactive behind a modal that currently owns the viewport.
That same hosted chrome rule also covers responsive launcher placement. While
the authenticated shell is using mobile navigation, `frontend-modern/src/AppLayout.tsx`
must keep the closed Pulse Assistant launcher as a bottom floating affordance
that clears the mobile nav; it must not reintroduce the desktop right-edge
launcher until the desktop shell breakpoint, otherwise hosted and self-hosted
operators lose usable dashboard and billing-page edge space at tablet widths.
That same authenticated shell contract now also has to distinguish backend
availability from websocket-stream liveness. When hosted runtime health stays
available during a stream reconnect or renegotiation window,
`frontend-modern/src/useAppRuntimeState.ts` must preserve a canonical
backend-healthy signal and `frontend-modern/src/AppLayout.tsx` must not
advertise the whole paid shell as disconnected or "Reconnecting..." solely
because the live stream is transiently recovering. Future hosted shell work
must keep the top-right connection badge aligned to overall authenticated
runtime availability first, with websocket churn treated as a narrower live
stream status instead of the shell-level truth.
That same hosted shell badge must also stay explicit about degraded-but-usable
paid runtime states. When the backend remains healthy but the live stream is
recovering, the authenticated shell should surface a degraded sync state
instead of collapsing back to the same disconnected wording used for total
runtime loss, so hosted operators can tell the difference between browser/API
usability and live-stream freshness at a glance.
That same authenticated shell contract also applies to every chrome consumer of
runtime health, not only the badge label. `frontend-modern/src/AppLayout.tsx`
must consume the canonical `connectionStatus` contract end-to-end for shell
behavior such as the Pulse logo activity animation rather than retaining
parallel `connected`/`reconnecting` prop assumptions after the runtime model
moves. Future hosted shell changes must update all chrome affordances together
so a status-model refactor cannot leave stale prop calls behind that crash the
authenticated cloud shell at render time.
That same route/provider shell must stay page-oriented as well: where a
browser route exists, `App.tsx` should lazy-load the route shell instead of
wiring product-surface components such as
`frontend-modern/src/components/Storage/Storage.tsx` directly into the router.
Retired aggregate or utility aliases must stay unregistered rather than being
kept as route-shell compatibility handoffs, so hosted bootstrap ownership stays
at the app boundary rather than leaking route concerns back into feature
components.
That same authenticated route shell also defers to the
frontend-primitives-owned post-auth landing path. `frontend-modern/src/App.tsx`
and `frontend-modern/src/pages/RuntimeHome.tsx` must send authenticated `/`
through the runtime-home landing contract first; hosted workspaces with no
connected infrastructure may still fall through to the single
`/settings/infrastructure` ownership path, but the shell must not regress into
a root-only settings redirect, legacy Infrastructure default, or
dashboard-only shortcut that strands first-time hosted tenants.
That same landing contract also owns authenticated `/login`: once the browser
has a valid session, `frontend-modern/src/App.tsx` must route `/login`
through that same runtime-home landing boundary instead of leaving the
authenticated app shell on a not-found compatibility path.
Authenticated app-shell route and chart prewarming may run from
`frontend-modern/src/App.tsx` and `frontend-modern/src/useAppRuntimeState.ts`
only as auth-neutral performance work: it must not change hosted organization
bootstrap, presentation-policy loading, commercial posture loading, or upgrade
prompt visibility. The runtime bootstrap must not prewarm retired
Infrastructure or Workloads summary-chart caches as a generic authenticated
side effect; route-module warming stays in `App.tsx`, and chart payloads belong
to the route or interaction that renders them.
The Pro license settings surface now follows the same rule as well. Changes to
`frontend-modern/src/components/Settings/ProLicensePanel.tsx` must carry this
contract and the dedicated Pro-license proof file instead of remaining an
unowned consumer of activation, trial eligibility, entitlement capability, and
plan-term presentation.
That owned Pro-license boundary is now intentionally split by role:
`frontend-modern/src/components/Settings/ProLicensePanel.tsx` is the settings
shell, `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
owns activation, trial, route-scoped notices, and commercial plan runtime
state, and `frontend-modern/src/components/Settings/ProLicensePlanSection.tsx`
owns the plan/read-model presentation. Future Pro-license work must extend
that split instead of pulling route state, entitlement derivation, and retry
lifecycle back into the shell component.
That owned presentation boundary includes the settings shell itself: the
top-level self-hosted billing surface must keep its page-shell title and
leading SettingsPanel title aligned on the canonical `Plans & Billing` label
so commercial activation, trial, and pricing state do not present as one
surface in navigation and a differently named surface in the actual paid
settings UI.
That same boundary also owns the shell and section framing copy for the
self-hosted Pro billing surface. `ProLicensePanel.tsx` must consume shared
presentation for its shell title/description, refresh CTA label, and section
headings rather than carrying those commercial labels inline.
The same title should also feed the `system-billing` settings navigation item,
and the same title and description should also feed the `system-billing` route
header metadata so the nav, shell, and route header do not narrate the same
commercial surface differently.
That same shared presentation owner also carries the canonical cross-surface
referral copy used outside the billing shell itself. When infrastructure or
other adjacent settings surfaces need to point operators toward Plans & Billing
for billing, license status, or paid feature access, they must consume the
settings-owned referral strings from
`frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`
instead of drafting route-local commercial guidance or reaching directly into
generic commercial helpers from hosted settings routes.
That same hosted-settings presentation boundary is explicit about bundle
ownership. `frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`
is the canonical settings-shell adapter for self-hosted Plans & Billing shell
framing and referral copy, while
`frontend-modern/src/utils/licensePresentation.ts` remains the shared
commercial notice/label owner for activation, trial, purchase, and recovery
language such as `License key`. Hosted settings surfaces must
not import self-hosted billing framing straight from the generic helper module
when doing so would reintroduce top-level bundle-init cycles into hosted
tenant settings routes.
Paid Pulse Pro v5 grandfathering is now part of that same canonical boundary:
when a recurring v5 customer migrates into v6, billing persistence,
entitlement evaluation, renewal handling, and Pro-license presentation must
preserve the customer's existing recurring price identity and uncapped
commercial capacity instead of silently rewriting them onto current v6 retail
pricing or Pro-era caps. Activation persistence and grant refresh may still
carry local-only legacy-migration continuity metadata as a defensive fallback
for any bounded legacy grant that survives outside the canonical v5 recurring
contracts, but active recurring v5/v1 customers must not rely on a captured
floor to stay admissible in v6.
That fallback continuity path is scrub-owned rather than read-owned. Ordinary
status or entitlement reads may tolerate old continuity fields while loading
legacy records, but they must not persist grandfather floors, start capacity
reconciliation, or expose monitored-system continuity as live commercial state.
Save-time monitored-system commercial denials are retired. TrueNAS and VMware
settings may render explicit preview results for grouping impact, but rejected
saves must flow through ordinary API error handling rather than a preserved
monitored-system cap preview.
That continuity rule cannot depend on webhook metadata being perfect. The
canonical Stripe price-to-plan lookup in `pkg/licensing/features.go` and
`pkg/licensing/stripe_subscription.go` must recognize the still-renewing
grandfathered recurring v5 and legacy v1 Stripe price IDs directly, so a real
`customer.subscription.updated` event that omits `metadata.plan_version` still
resolves to `v5_pro_monthly_grandfathered` or
`v5_pro_annual_grandfathered` instead of falling back to an opaque
`stripe_price:*` marker.
That Pro-license presentation rule is explicit UX, not only hidden metadata:
when a migrated recurring v5 plan is active or in grace, the settings surface
must render plan terms and a continuity notice that makes it clear the
existing recurring price remains in force until cancellation, while
self-hosted monitoring and child-resource volume are not metered in current v6
self-hosted packaging. The same plan summary must not render a separate
`Guest Capacity`, child-resource allowance, or equivalent volume-cap row for
uncapped/grandfathered self-hosted plans; the customer-facing continuity story
is existing-price protection plus unmetered self-hosted monitoring and
child-resource volume in current v6 self-hosted packaging, not a new paid
guest-capacity benefit.
The self-hosted commercial presentation on that same surface is now locked to
the no-cap monitored-system model as well. `ProLicensePanel.tsx`,
`CommercialBillingSections.tsx`, and
`frontend-modern/src/utils/commercialBillingModel.ts` must present current v6
self-hosted packages as core monitoring included in every self-hosted tier plus
plan-specific extras: Community stays free for core monitoring, Relay adds
remote access/Pulse Mobile handoff/push convenience and 14-day history, Pro adds Relay plus
hands-on Patrol modes, issue investigation, verified fixes, advanced administration,
and 90-day history, while Pro+ remains legacy continuity only.
Cloud/MSP pricing semantics stay separate, and grandfathered v5 continuity copy
must describe legacy continuity and recorded baselines rather than current
self-hosted monitored-system caps, capacity, or policy boundaries.
Active Relay and Pro installs may also render a factual value-proof panel on
the owned Settings -> Plan surface, but that proof must derive from the live
self-hosted entitlement payload rather than from public pricing copy. It may
render only for active/grace/trial paid self-hosted states with a numeric
`max_history_days` payload, must suppress itself for Community, inactive, and
missing-payload states, and must describe partial or missing rows as
activation/recovery attention rather than as marketing pressure. Relay proof is
limited to remote access, Pulse Mobile pairing for handoff, push notifications, and the
reported 14-day history entitlement. Pro proof adds the reported 90-day history
entitlement plus root-cause/remediation and team/admin capability groups.
Frontend presentation and component tests must cover active, partial, and
Community-suppressed proof states whenever this panel changes.
That same settings-owned presentation must distinguish active commercial
features from stale bounded legacy fallback metadata. Active grandfathered
recurring v5 plans must render the existing recurring price continuity
directly and must not show a pending or captured floor banner or any finite
monitored-system volume cap. Recognized v6 plan labels must follow the same
customer-facing no-cap rule even if a legacy entitlement still carries
`plan_limit`, `effective_limit`, `grandfathered_floor`, or capture-pending
telemetry: `useProLicensePanelState.ts` must suppress Usage tabs,
monitored-system policy sections, continuity notices, plan-limit detail rows,
and pause-new-admissions copy, while `licensePresentation.ts` must keep current
plan summaries focused on core monitoring plus the actual tier extras.
When billing or support presentation still needs a capacity-free self-hosted
summary value, the customer-facing label is `Not metered`, not `Unlimited`;
lifetime commercial continuity duration may use `Permanent`, but no normal
self-hosted plan surface should imply monitored-system volume is a paid tier.
That same pricing boundary now also owns the shared frontend plan-definition
models. `frontend-modern/src/utils/cloudPlans.ts` and
`frontend-modern/src/utils/selfHostedPlans.ts` are the canonical frontend
sources for cloud and self-hosted plan copy, limits, and comparison metadata.
`frontend-modern/src/pages/CloudPricing.tsx`,
`frontend-modern/src/pages/HostedSignup.tsx`, and the self-hosted billing
settings surfaces must consume those shared plan-definition sources instead of
redefining retail plan facts or counted-unit policy locally.
That shared ownership also includes display-ready price semantics. Monthly
headline price, founding-rate override, compare-at strike-through copy,
campaign badge copy, and annual summary text must come from the shared
plan-definition owners rather than page-local string parsing or hardcoded
retail amounts inside hosted pricing/signup screens.
The same owner also holds shared hosted commercial copy such as page title,
introductory description, common Cloud inclusions, setup-step guidance,
hosted-signup section headings, and primary hosted-signup CTA labels when
those facts describe the canonical offer rather than one page's local layout.
The same rule applies to self-hosted commercial framing inside product-owned
billing and activation surfaces: plan names, counted-unit copy, and upgrade
adjacency must come from the shared self-hosted plan-definition owner rather
than page-local strings. That owner must also keep Community, Relay, and Pro
framed as core monitoring included in every self-hosted tier plus their
bundle-specific extras rather than drifting back to bounded monitored-system
sales copy, vague `systems`, or device-style language. The same shared owner
must also provide the billing surface bundle summary and metric-history facts
for current unmetered self-hosted tiers, so `commercialBillingModel.ts` and
`useProLicensePanelState.ts` do not regress back to legacy guest-capacity or
meter-style entitlement fields on retail Community, Relay, or Pro plans. Pro+
may appear only as a legacy continuity tier, and
`frontend-modern/src/utils/licensePresentation.ts` must label `pro_plus`
accordingly in tier, plan, and plan-version presentation instead of rendering
it as a current public Pulse Pro+ package. When direct plan-selection intent
opens the explicit self-hosted comparison surface, the shared presentation
helpers must show Pro's operations, admin, and reporting extras together while
still framing Community, Relay, and Pro as core monitoring included in every
self-hosted tier rather than monitored-system allowance tiers. Relay copy in
that shared owner must describe the current v6 GA product as standard Relay
remote access, supported Pulse Mobile pairing for handoff, push notifications, and 14-day
history, and it must not market a customer-specific Relay URL until that
routing surface exists as a backed product capability. Cloud common-inclusion
copy follows the same mobile-language rule: describe supported Pulse Mobile
pairing for handoff and push notifications rather than generic mobile app access. The same factual
plan surface must keep inactive-state copy neutral: default Community installs
must describe the instance as ready to use rather than as missing an activation
key, and they must not foreground paid self-hosted activation, v5-era "No Pro
license" language, or `unlocked` entitlement phrasing before the operator opens
an explicit comparison, checkout, activation, or recovery path.
The shared license presentation owner also holds self-hosted Pro settings
trial-ended notice copy for `ProLicensePlanSection.tsx`; that surface must
consume canonical helper notices instead of carrying inline upgrade copy or
local status-tone branches.
That same `frontend-modern/src/utils/licensePresentation.ts` owner must treat
compatibility-only capability keys such as `kubernetes_ai` as non-marketed
technical facts: API/docs surfaces may still name the compatibility route, but
customer-facing self-hosted current-plan summaries and unlocked-capability
lists must not surface those keys as marquee Pro value. MSP-only contract keys
such as `multi_tenant` follow the same rule inside the self-hosted Pro plan
surface: they may remain entitlement inputs for hosted/MSP/account surfaces,
but they are not self-hosted Pro value copy. The same rule applies to legacy
claims such as `incident memory`: current v6 upgrade notices and commercial
copy must use the canonical 90-day history framing until a distinct
incident-memory product exists.
That same plan-section boundary must also defer notice resolution to component
runtime. `frontend-modern/src/components/Settings/ProLicensePlanSection.tsx`
may not compute trial-ended notices at module scope, because hosted settings
bundles must survive top-level import ordering without throwing
initialization-time `ReferenceError` crashes before the workspace UI mounts.
That same plan-section boundary also owns the rule that Community (non-paid)
users must not be funneled through upgrade CTAs inside Settings -> Plan.
`ProLicensePlanSection.tsx` must not render the monitored-system upgrade
arrival banner, the trial-start CTA, or the inactive-Pro upsell notice to
users without paid features, `licensePresentation.ts` must not retain a dead
inactive-Pro upsell helper for that surface, and the capacity section must not
render an upgrade-plan button for monitored-system pressure. Self-hosted Pro marketing lives at
`pulserelay.pro/pricing`; the Settings plan surface must show factual
license state for Community users and leave discovery of paid tiers to
surfaces outside the plan page.
That same no-funnel rule extends beyond Settings -> Plan. The self-hosted
frontend must not render blanket trial/upgrade marketing to Community users
from the main app shell, the setup wizard completion screen, or any
app-wide surface that fires without the user having explicitly engaged with
a paid feature. In particular the retired Dashboard overview must not be
restored to carry a `RelayOnboardingCard` paywall or equivalent `Start trial`
prompt, the
`SetupCompletionPanel` may not carry a `Monitor from Anywhere` Relay trial
block, and no time-triggered "active use" nudge such as `ActiveUseTrialNudge`
may auto-appear for Community users. Feature-gated discovery that fires only
when a Community user clicks a locked feature (for example alert
investigation, 30/90-day history ranges, Patrol AI autonomy modes, or
Settings panels whose feature the user opened themselves) remains in scope
for that feature's owning subsystem — those are user-initiated discovery
paths, not blanket funnels, and are not required to be removed.
Public AI and entitlement docs must use the same boundary: Community/Relay may
describe Patrol background findings with BYOK, while investigation, proposed
fixes, governed fix execution, and higher autonomy remain paid AI-operations
features.
Those docs should describe moving between available modes, not tell readers to
"upgrade" as part of an ordinary safety progression.
That same retired counted-unit boundary also owns the disclosure rule for
retail copy: default billing and pricing surfaces should not use
monitored-system allowance copy. Monitored-system grouping definitions belong
only behind explicit support/ledger disclosures rather than persistent
plan-tab chrome.
The same boundary also owns where monitored-system continuity truth lives. A
dedicated self-hosted Pro plan-surface continuity section is only canonical when
Pulse is reconciling explicit carry-forward support context outside normal
self-hosted monitoring. Normal self-hosted plans must not keep a `Monitoring
capacity` section alive just to restate that monitoring is included without a
monitored-system volume gate; counted-unit explanation plus current usage
inspection belongs to the ledger/disclosure path, not to plan-capacity UI.
The app shell must not expose a monitored-system warning entry point for
self-hosted plan volume. Support review states can link to the usage-owned
ledger, but the CTA must not revive "View capacity" copy as an upsell-shaped
monitored-system prompt.
Community overflow/setup-slot messaging must still explain the current
top-level monitored systems plus any temporary setup slot in customer terms
rather than compressing the contract into slash-style quota strings that imply
Pulse is counting every child device.
That same billing-facing boundary also owns why an installation can require
legacy review at all. When the monitored-system posture is `over_limit_frozen`,
customer copy must explain whether a carried-forward baseline needs review or
whether migrated v5 continuity capture is still pending. Billing surfaces must
not render an unexplained `15 monitored, plan includes 5` state that looks like
Pulse is enforcing a current self-hosted cap model.
Those same billing-facing surfaces must also describe the commercial contract in
customer terms: monitored systems, legacy continuity baselines, subscription
status, and license status. They must not revive legacy `installed-agent`
wording or vague internal nouns like `allocation` once the monitored-system
billing model is the canonical product truth.
Loading, empty, and temporary-unavailable states on monitored-system usage
surfaces follow that same rule: they should use calm customer-facing status
language such as usage loading or usage unavailable with a clear retry action,
not raw transport phrasing like `failed to load`.
The self-hosted pricing page should therefore stay focused on plan cards,
upgrade paths, and the comparison table rather than rendering a separate
counted-unit explainer card beneath the tier grid.
Hosted commercial pages follow the same presentation rule: `CloudPricing.tsx`
and `HostedSignup.tsx` should describe plan scope and setup in concise product
language, not in implementation-detail terms such as internal workspace
provisioning guarantees, control-plane routing mechanics, or narrated
deployment internals.
Those same hosted surfaces should also prefer neutral plan-selection and
sign-in language such as `Choose Starter` or `sign-in link` rather than
marketing-heavy CTA copy or infrastructure-specific transport jargon.
They should also avoid duplicating shared commercial facts across multiple
adjacent blocks on the same screen. Common Cloud inclusions belong in one
shared area rather than being restated inside every plan card, and hosted
signup should not repeat the selected plan facts across the page header, form
card, and plan summary at the same time.
That contract applies to both plan summary labels and upgrade/paywall copy:
current v6 self-hosted pricing may not drift back to the older `$49/yr Relay`,
`$99/yr Pro`, monitored-system-count marketing drift, or public Pro+ sales
language that contradicts the locked Community / Relay / Pro no-cap packaging.
Cancellation is the explicit boundary for that policy. Once a grandfathered v5
recurring subscription is canceled, any later return must resolve through the
current v6 pricing contract rather than reviving the legacy recurring rate.
The canonical cross-repo manual drill for that boundary is
`docs/release-control/v6/COMMERCIAL_CANCELLATION_REACTIVATION_E2E_TEST_PLAN.md`.
The paid relay settings surface is part of that same ownership model. Changes
to `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx` must carry
this contract and the dedicated relay frontend proof files instead of
remaining unowned consumers of relay licensing state.
Its paywall and activation copy must say Relay and higher plans rather than
Pro-only wording; Pro may include Relay, but Relay is still its own public
self-hosted tier.
That relay settings owner is intentionally split by role:
`frontend-modern/src/components/Settings/RelaySettingsPanel.tsx` is the
settings shell, `frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts`
owns relay config/status polling and pairing runtime, and
`frontend-modern/src/components/Settings/RelayPairingSection.tsx` owns the QR
pairing surface. Future relay settings work must extend that split instead of
pulling polling and QR-generation lifecycle back into the shell component.
The retired Dashboard shell must not be restored to host a Relay onboarding
card or equivalent blanket Relay upsell. Relay discovery belongs to the owning
Settings surface above; Infrastructure and Workloads stay monitoring-first
views with no app-wide paywall onboarding composed into them.
That relay pairing boundary now also includes backend-owned mobile credential
lifecycle: when the settings surface generates a mobile pairing QR, it must ask
the server for a fresh scoped Pulse Mobile relay access token, fetch the
onboarding payload through that token context, and tear down superseded or
failed unused tokens instead of accumulating hidden credentials.
That same pairing fetch path must stay token-bound instead of ambient-session
bound: the onboarding QR request must carry the freshly minted relay pairing
token explicitly so the returned payload reflects the exact credential that the
mobile device will bootstrap with, not whichever browser session happened to
open the settings page.
The same flow must fail closed if the pairing payload omits the authenticated
relay token state needed by the mobile deep link; pairing UI cannot silently
render a QR that bypasses governed auth ownership.
Relay pairing token presentation is part of that same contract as well: the
settings surface must label those Pulse Mobile relay access credentials
distinctly from long-lived automation tokens so operators can identify and
revoke stale mobile devices or abandoned pairing attempts without guessing
which credential was created for mobile runtime access.
That same runtime contract now uses a dedicated backend-owned
`relay:mobile:access` scope, with the server preserving backward-compatible
gates for older mobile tokens that still carry the legacy AI scopes during the
post-RC migration window.
Relay pairing refresh behavior is part of that lifecycle contract too: a
successful QR refresh must revoke the superseded token once the new token-backed
payload is ready only if the superseded token still shows no device use, while
a failed refresh must revoke only the new failed token and keep the previously
valid QR visible instead of collapsing the operator back to an empty pairing
state.
Hosted signup provisioning now follows the same rule. Changes to
`internal/api/public_signup_handlers.go` and `internal/hosted/provisioner.go`
must carry this contract and the dedicated hosted-signup provisioning proof
files instead of remaining a split boundary between API handlers and an
unowned hosted runtime helper.
That hosted signup boundary is now also canonical in shape: the public signup
handler owns request validation, trial billing initialization, and magic-link
issuance, while `internal/hosted/provisioner.go` owns the shared org
bootstrap/admin-role assignment, duplicate-owner-email idempotency, and
rollback path for hosted signup failures.
The same boundary is identity-stable: `internal/hosted/provisioner.go` must
generate a stable opaque owner user ID for new hosted signup organizations,
store the signup email only as `OwnerEmail`/member contact metadata, assign
RBAC against the stable owner ID, and return contact email separately so the
public signup handler can issue magic links without turning email into the
tenant principal.
That duplicate-owner-email rule is fail-closed and server-owned. Repeated
hosted signup attempts for an email that already owns a tenant must resolve to
the existing org identity instead of minting a second tenant and relying on
later auth or billing surfaces to untangle the collision.
That same public signup boundary must also stay privacy-safe at the browser
edge: syntactically valid signup requests return one uniform `202 Accepted`
Pulse Account message whether the backend provisioned/sent email or suppressed
side effects due to the owner-email rate limit, so the route does not reveal
prior email usage through `201` versus `429` drift.
Hosted billing-state normalization now follows the same rule: a missing
`plan_version` must remain missing instead of being synthesized from
`subscription_state`, while explicit trial defaults remain explicit.
Hosted trial bootstrap and hosted entitlement refresh must not mint new
quickstart inventory. The `pulse-pro` license server route/config/proxy layer
for hosted quickstart is retired for v6 GA; historical quickstart fields may
remain parseable as persisted billing compatibility while those fields exist,
but new user-facing hosted or self-hosted acquisition work must not depend on
that inventory as a product promise, and lease-refresh rewrites must avoid
turning legacy fields into a renewed customer-facing hosted-model offer.
Hosted AI runtime defaults are part of the same boundary as well: when a cloud
tenant falls back to provider defaults, the persisted model identifier must
remain canonical `provider:model` data rather than a bare provider-local alias,
so hosted enterprise runtime startup does not fail before chat or approvals can
initialize.
Hosted release builds must not reopen the trial-activation public key through
runtime environment just because `PULSE_HOSTED_MODE=true`. Managed tenants may
still receive a hosted-specific verification key, but the release binary must
consume that key through the build-time embedded source of truth rather than a
runtime env override that can silently replace the trusted verifier after
deployment.
Legacy MSP plan aliases are input-only compatibility shims. Live runtime
defaults, fallback provisioning, entitlement issuance, and limit/workspace
lookups must resolve to canonical `msp_starter` rather than preserving
`msp_hosted_v1` as an active first-class plan name.
Hosted control-plane plan resolution is now part of the enforced ownership
model: changes to hosted entitlement issuance, control-plane registry
canonicalization, or Stripe provisioning plan resolution must carry this
contract and the path-specific proof files that verify those boundaries.
Hosted tenant container bootstrap is part of that same boundary as well: the
control plane may bind-mount billing and handoff files into `/etc/pulse` as
read-only inputs, but runtime startup ownership repair must treat those paths
as immutable and skip `chown` attempts against them instead of aborting tenant
provisioning.
That startup repair boundary also treats image-owned runtime paths as
immutable. The container entrypoint may repair writable tenant data paths, but
it must not recursively `chown` built image paths such as `/app` or
`/opt/pulse`, because doing so copy-ups the runtime image into every hosted
tenant writable layer and turns fleet reconciliation into host-disk pressure.
That same immutable-file boundary now also owns write-time runtime ownership:
control-plane provisioning and later billing-state rewrites must leave
`billing.json`, `secrets/handoff.key`, and `.cloud_handoff_key` readable by
the tenant Pulse runtime user at the moment they are written. Fixing startup
`chown` behavior alone is not enough if the mounted files stay `root:root`
after provisioning, because hosted auth handoff and hosted lease reads will
then fail closed inside an otherwise healthy tenant.
Hosted tenant org bootstrap is part of that same runtime boundary. Cloud and
MSP tenant provisioning must seed a canonical tenant-scoped `org.json` under
`orgs/<tenant-id>/` and leave both the directory tree and file readable by the
hosted runtime user; otherwise hosted magic-link handoff can preserve the
correct tenant org cookie while the tenant API still fails closed with
`invalid_org` because no tenant organization metadata exists on disk.
That same shared browser client boundary also owns structured error extraction
for hosted tenant requests. Cloud and MSP surfaces may show failures from
`frontend-modern/src/utils/apiClient.ts`, but they must resolve canonical JSON
`error` / `message` fields before showing UI feedback instead of leaking raw
response payloads while tenant-scoped org headers are still in flight.
That same boundary now also owns stable structured error metadata on the shared
browser client. When backend routes return canonical JSON `code` plus
string-valued `details`, `frontend-modern/src/utils/apiClient.ts` must preserve
that metadata on the thrown error object instead of collapsing everything to one
display string. Hosted and platform-onboarding surfaces may then classify
supported failure modes from the shared error object, but they must not invent
route-local parsing rules or provider-local transport shims to recover metadata
the shared client dropped.
Hosted tenant runtime env is part of the same contract too: provisioned
containers must carry hosted-safe tenant context such as
`PULSE_TENANT_ID=<tenant-id>`, `PULSE_MULTI_TENANT_ENABLED=true`, and an explicit
`PULSE_PUBLIC_URL=https://<tenant-id>.<base-domain>` so the tenant-scoped org
surface is actually enabled after handoff instead of failing closed under a
paid hosted session.
Hosted MSP workspace org seeding is part of that same boundary too:
`internal/cloudcp/account/tenant_handlers.go` owns the authenticated
account-scoped workspace-create entry path, and
`internal/cloudcp/stripe/provisioner.go` owns the underlying workspace/org
bootstrap. When the control plane provisions a new workspace under an existing
account, that boundary must seed `org.OwnerUserID` from the authenticated
creator email when that actor is known on the request path instead of
inferring a canonical owner from membership query order. If the creator is not
available, fallback owner selection must still be deterministic rather than
depending on newest-row ordering in the registry.
Hosted tenant entitlement evaluation is part of that same boundary too: when a
hosted tenant lands in a tenant-scoped org like `t-...`, the runtime must
still honor the instance-level hosted billing lease in `default` until or
unless an org-local billing state exists, rather than collapsing a freshly
provisioned paid tenant into `subscription_required` on first entry.
JWT-backed entitlement claim evaluation and activation-grant translation now
follow the same explicit proof model instead of relying only on the broad cloud
runtime catch-all policy.
Persisted billing-state normalization, hosted database-source loading, Stripe
plan derivation, and the cloud plan/limit tables now follow the same ratchet:
they are expected to move behind path-specific proof routes rather than staying
indistinguishable inside the generic cloud runtime policy.
The runtime entitlement surface now follows the same rule: evaluator/token
source accessors, hosted-subscription validity rules, and frontend entitlement
payload construction should move behind explicit proof routes rather than being
implicitly trusted as part of the catch-all cloud runtime layer.
That same runtime surface must stay internally coherent in local dev mode. If
`pkg/licensing/service.go` widens backend `HasFeature()` checks under
`PULSE_DEV=true` or demo/mock mode, the entitlement payload and any other
frontend-facing capability contract derived from that service must advertise
the same capability set instead of leaving paid surfaces artificially locked
behind stale free-tier capabilities.
That dev widening is still bounded by runtime readiness rather than marketing
intent: `multi_tenant` is not a valid advertised dev capability unless the
process also has `PULSE_MULTI_TENANT_ENABLED=true`, because the org API surface
is intentionally disabled otherwise.
The same runtime-readiness rule also excludes placeholder and plan-marker
entries like `white_label`, `multi_user`, and `unlimited` from dev/demo
entitlement payloads when there is no corresponding operable runtime surface;
those belong in tier metadata and plan semantics, not in live capability
advertising for a free dev shell. The `unlimited` feature key is a hosted/MSP
capacity-policy marker and must use neutral hosted-capacity wording in shared
metadata rather than customer-facing "Unlimited Instances" copy that implies a
self-hosted monitoring-volume tier.
The same distinction applies to customer-facing self-hosted billing
presentation. `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
and `frontend-modern/src/utils/licensePresentation.ts` may label operable
capabilities, but they must not render placeholder or plan-marker entries like
`white_label`, `multi_user`, `multi_tenant`, or `unlimited` in the Pro plan
feature list just because those keys exist in tier metadata.
Cloud/MSP live price IDs are no longer an open fill-in task either. The audit
record `docs/release-control/v6/records/cloud-msp-price-audit-2026-03-13.md`
verified that the 13 canonical Cloud/MSP v6 `price_*` IDs are present in the
governed `pulse-pro` operational docs and license-server env template, and that
each ID resolves to an active live recurring Stripe price object.
Activation service runtime, license-server transport, encrypted activation
persistence, and hosted entitlement lease signing now follow the same ratchet. Changes
to `pkg/licensing/service.go`, `pkg/licensing/grant_refresh.go`,
`pkg/licensing/installation_status_poll.go`, `pkg/licensing/license_server_client.go`,
`pkg/licensing/persistence.go`, `pkg/licensing/activation_store.go`, and
`pkg/licensing/trial_activation.go` should carry their dedicated proof files
instead of relying only on the generic cloud runtime policy.
That same activation-service boundary also owns test-only helper exclusion from
release builds: legacy JWT fixtures and grant JWT generators used by tests must
live in non-release build-tagged helpers instead of shipping inside the primary
runtime service file.
The remaining cloud-paid runtime families now follow the same rule as well:
feature/limit primitives, billing and entitlement type shapes, commercial
migration and trial flow, host lifecycle tracking, and public-key/build-mode
boundaries should all resolve through explicit proof routes rather than a
package-wide `pkg/licensing/` fallback.
The retired conversion-telemetry boundary now keeps self-hosted commercial
progression out of customer-side browser analytics and out of the compiled
normal licensing package. Customer frontend source must not retain
`upgradeMetrics`, `conversionEvents`, infrastructure onboarding metrics
wrappers, or their call sites. The browser app must not emit local
`pricing_viewed`, `checkout_clicked`, paywall, commercial funnel, or onboarding
events from the in-app `Plans & Billing` plan surface.
`pulse-pro:license-server/v6_checkout.go` owns the Pulse Account handoff
equivalents bound to `portal_handoff_id` and the canonical checkout intent.
The browser app must not try to recreate those Pulse Account stages from
referrer state, and the commercial service must not collapse self-hosted v6
handoffs back onto the public-site release track when production public GA is
still on v5.
That same retired local conversion store is no longer a normal product read
model. `/api/diagnostics` and the Settings support diagnostics panel must not
expose pricing, checkout, conversion, or infrastructure-onboarding analytics,
and the normal API router must not register local commercial metric stats,
health, config, ingestion, or funnel reads. Maintainer commercial reporting
must live outside the self-hosted customer product runtime instead of reusing
the Pulse settings or diagnostics surface.
Stripe checkout and subscription webhook persistence now also follows the
retired monitored-system-volume rule: when paid state is granted, billing-state
writes must persist canonical plan and feature state while scrubbing retired
`max_monitored_systems` limits, and when paid state is revoked they must not
preserve stale paid capacity.
Grandfathered recurring v5/v1 continuity is the explicit stored-state
exception inside that same boundary. `pkg/licensing/billing_state_normalization.go`
must persist the canonical billing baseline for recognized grandfathered
recurring Stripe plans even though the live entitlement contract is uncapped,
so webhook persistence, hosted billing state, and admin inspection stay
deterministic instead of leaking an internal `0 == unlimited` convention into
saved billing records. `pkg/licensing/database_source.go`,
`pkg/licensing/models.go`, and downstream entitlement evaluation must then
strip that stored monitored-system cap back out before any runtime entitlement
payload, so continuous grandfathered recurring customers stay uncapped until
cancellation.
That same monitored-system entitlement boundary also owns the retired-warning
copy rule: customer-facing surfaces must not revive the old limit banner,
migration freeze guidance, or agent-install quota language while describing
non-counted legacy/API-connected resources.
That same webhook boundary now also owns request-lifetime decoupling for
checkout provisioning: long-running `checkout.session.completed` tenant
provisioning must complete under an explicit background timeout instead of
depending on the inbound Stripe request context surviving long enough for first
boot and health polling.
That same Stripe webhook boundary also owns bounded idempotency retention in
the control-plane registry. `internal/cloudcp/registry/registry.go` may keep
dedupe rows long enough to suppress duplicate Stripe deliveries and reclaim
stale in-flight attempts, but it must prune expired processed or abandoned
`stripe_events` rows instead of letting webhook idempotency state grow without
bound on disk.
That same paid relay onboarding boundary now also owns QR-token lifecycle on
the supported Pulse Mobile pairing surface. `RelaySettingsPanel.tsx`,
`RelayPairingSection.tsx`, and `useRelaySettingsPanelState.ts` may revoke a
displayed pairing token only when canonical token metadata still shows no
`lastUsedAt`; once a token has been used by a paired device, refreshing or
hiding the QR must preserve that credential instead of treating it as
disposable UI state.
That same paid relay onboarding boundary also owns the operator-facing rollout
posture on the licensed settings surface: relay paywall and pairing copy must
describe supported Pulse Mobile pairing as a normal-use Relay capability, and
must not fall back to staged-beta or coming-soon messaging once the owned
pairing/runtime path is live.
The customer-account surface is now also an explicit cloud-paid ownership
boundary. Pulse now has a coherent authenticated Pulse Account shell in
`internal/cloudcp/portal/`, account/workspace mutation APIs in
`internal/cloudcp/account/tenant_handlers.go`, and transitional self-hosted
commercial utility pages in `pulse-pro/landing-page/`. The canonical product
shape is `docs/release-control/v6/internal/PULSE_ACCOUNT_PORTAL_SPEC.md`: one
authenticated Pulse account shell that unifies Cloud tenants, self-hosted
licenses, billing, recovery, and MSP admin surfaces without creating a
standalone Relay portal. That shell now also owns two product rules
explicitly: signed-in hosted arrivals land on `Workspaces`, not on a separate
`Overview` or `Summary` destination, and the top of `Workspaces` may render
only one quiet inline facts line plus one next-action row before the workspace
list instead of a second summary deck, metric grid, or duplicated overview
panel. The top-level task row must stay honest to account shape by removing
irrelevant hosted-only tasks instead of pretending they are live. Any shared
fallback that still lands on a non-live task must render an explicit
unavailable state instead of blank space. The same shell also owns
action-first task surfaces for `Access`, `Billing`, and `Support`: access
mutations must be permission-honest and roster-led, billing must reduce to one
obvious job at a time with hosted billing first when relevant, support must
stay a failed-path handoff rather than a peer workflow, and phone-width
layouts must collapse the desktop shell into a compact task strip so the
active job remains primary and visibly in-frame when the strip scrolls. The
same narrow-screen shell must also compress account identity into one compact
context strip instead of repeating a desktop-sized intro block or second
summary box ahead of every task, and lower
workspace job surfaces such as lifecycle review or create-workspace forms must
be revealed when the user opens them. `Workspaces` must also default to the
workspace list plus the real task entry points rather than an idle lifecycle
essay; the lifecycle rail should appear only when a lifecycle or
create-workspace job is active. `Access` follows the same rule: the hosted
roster must be the default state, while invite, role-change, and remove
controls appear only when that exact access job is active, and a hosted
view-only roster must stay a review surface instead of a row-by-row action
table with fake disabled action state. Managed idle `Access` follows that same
review-first rule: the roster should stay two-column until a remove job is
active, with the third action column reserved for real remove access work.
That same task-first `Access` contract
must also start from bootstrap-owned roster truth: the first hosted roster
render must come from the portal bootstrap payload itself, with later member
API reads reserved for refresh and mutation follow-through instead of
placeholder-first rendering. That same hosted access boundary also owns
invitation consent semantics: `internal/cloudcp/account/handlers.go` plus
`internal/cloudcp/registry/models.go` and `internal/cloudcp/registry/registry.go`
may persist pending invitation subjects for unknown emails, but they must not
auto-create users or memberships from invite input alone. Pending access rows
must remain explicit `pending` invitation subjects in the bootstrap roster and
member API until the invited email authenticates through the portal-owned
magic-link/session path in `internal/cloudcp/auth/handlers.go` and
`internal/cloudcp/auth/session.go`, and role or removal mutations on those
rows must target the invitation record rather than guessing a future user
identity. `Billing` follows
the same task-first rule: hosted billing leads when present, the self-hosted
billing, license, refund, and privacy paths reduce to explicit job pickers,
and the active self-hosted billing panel must stay hidden until that exact job
is opened and then be revealed in-frame on narrow layouts. When no hosted
account exists, the billing shell must skip any empty hosted-billing block and
lead directly with the self-hosted job picker. `Support` follows
the same account-shape rule: self-hosted-only accounts reduce to the billing
escalation path and billing-specific handoff packet only, and hosted
workspace/access escalation routes must not render without hosted accounts.
That same owned shell also owns the signed-in visual posture: the portal
should read like a serious settings/account tool rather than a dashboard.
Navigation chrome, account framing, and inline workspace summary treatment
must stay visually quieter than the active task surface, with flatter light
treatment and list-first task presentation instead of dark ornamental rails,
sidebars, or nested explanatory cards. The signed-in shell should orient the
user with one quiet account-context header and one flat top task row; the
`Workspaces` surface may add one factual inline summary line and one next-step
row inside the section itself, but it must not bring back a second summary
deck competing with the task.
That same owned shell must also open on the first live job instead of a
summary layer: hosted accounts should land in `Workspaces`, and self-hosted-
only accounts should land in `Billing`. The signed-in shell must not expose a
separate `Overview` or `Summary` tab ahead of the real task surfaces.
That same owned shell also owns the signed-out visual posture: the unauthenticated
`/portal` page must read like the same product boundary, not a leftover
marketing block plus a generic login card. The auth surface should present one
obvious sign-in action, precise account-scope rows, and the same restrained
flat system as the signed-in account shell.
The same shell/runtime boundary must also stay honest to hosted-account
permissions: when the current role is view-only, `Workspaces`, `Access`, and
hosted `Billing` copy must not advertise create, roster-mutation, or hosted
billing actions that the runtime will not allow. Those surfaces must say that
an owner or admin is required instead of implying blocked jobs are live.
`Support` follows that same permission rule: a hosted view-only user may be
sent back to `Workspaces`, `Access`, or `Billing` only as review and
owner/admin handoff paths, not as though the user can perform hosted
lifecycle, access-mutation, or hosted-billing changes directly before
escalation.
Inline workspace counts and shell copy follow the same account-shape and
permission rule: hosted-only accounts must not mention self-hosted billing
utilities by default, and hosted view-only roles must say when hosted billing
still needs owner/admin authority.
That same portal runtime contract now also owns explicit self-hosted upgrade
arrivals from the product: when Pulse hands a self-hosted commercial upgrade
intent into `/portal`, the billing shell may expose an `Upgrade` job even when
the signed-in account has no existing self-hosted commercial history, but that
arrival must stay narrower than the full self-hosted utility set. Hosted-only
accounts may not suddenly render retrieve/refund/privacy tools by default just
because they arrived from an upgrade CTA; only the specific portal-owned
upgrade path may appear until the account actually has relevant self-hosted
history.
The same rule applies to the compact account-context strip: it must describe
the current user's effective hosted tasks, not restate full access-control and
billing capability when those actions are blocked behind owner/admin roles.
That same owned shell must also keep role labels on product vocabulary:
customer-facing copy may say `Owner`, `Admin`, `Tech`, or `Read-only`, but it
must not leak internal identifiers such as `read_only` or legacy aliases such
as `member`.
That same owned signed-in shell must also keep the first available action
permission-honest for hosted view-only users: when no workspace is ready, the
primary route must stay on reviewable `Workspaces` or `Access` surfaces before
any blocked hosted billing or owner/admin-only mutation path.
That same owned task surface must also keep failure copy on the user job
instead of leaking raw transport wording: `Access`, `Workspaces`, and
`Billing` failures must render the task-specific action that could not
complete, not generic copy such as `Network error.`.
That same owned inline workspace counts and workspace-state copy must also
keep `Ready` honest when no hosted workspace exists yet: hosted accounts with
zero workspaces may not tell the user to review current workspace state, and
must instead say that nothing is ready yet until the first hosted workspace
exists. The same counts and copy must keep suspended-only states honest, and
must stay fact-first rather than inventing urgency or health verdicts such as
`Nothing urgent` or `Healthy now`.
That same portal shell/runtime boundary must also keep task and status copy
literal across the account surface: customer-facing wording may not lean on
commentary such as `obvious`, `actual work`, `trustworthy`, or `settled` when
the runtime already knows the concrete state, action, or failure being shown.
That includes shell badges, section labels, context chips, route labels, and
error headings: they must use the exact action or state (`Manage access`,
`Hosted billing attached`, `Email support`, `Failed to load roster`) instead
of shorthand such as `Manage`, `Hosted`, or generic alert labels. Support
copy follows the same contract: escalation guidance must stay short and typed
to the exact task path, account/email, and failed step instead of drifting
into longer procedural prose.
That same canonical shell/runtime boundary now also owns the bootstrap truth
for when self-hosted commercial history is relevant. Hosted-only accounts must
not render self-hosted license, refund, privacy, or support-escalation copy
unless the portal bootstrap explicitly marks self-hosted commercial history as
relevant to the signed-in account.
Until that
candidate lane lands, new
commercial account work must extend the governed Pulse account shape rather
than spawning additional one-off recovery or billing pages.
The same owned shell/runtime boundary also keeps signed-in bootstrap free of
feature-local telemetry collectors. `frontend-modern/src/useAppRuntimeState.ts`
may start shared app-shell services, auth refresh, and websocket recovery, but
it must not boot a storage-only disk history collector now that storage
drawers read the canonical backend metrics-history contract through shared
chart primitives.
That same hosted browser shell boundary now also treats
`frontend-modern/src/App.tsx` as provider placement only. Shared runtime
consumers must import websocket and dark-mode hooks from
`frontend-modern/src/contexts/appRuntime.ts` instead of importing `@/App`, so
lazy hosted and settings chunks cannot create a reverse dependency into the
app shell and blank the mounted browser surface before auth/bootstrap
completes.
That same cloud-paid/browser boundary now also governs public demo posture.
`DEMO_MODE` may run against a real internal entitlement, but public demo
surfaces must not reveal self-hosted license metadata, hosted billing state,
monitored-system ledgers, upgrade nudges, or activation controls just because
the underlying runtime is commercially enabled.
`internal/api/router_routes_auth_security.go`,
`internal/api/security_status_capabilities.go`,
`frontend-modern/src/useAppRuntimeState.ts`, and the shared session-capability
stores must treat `/api/security/status.sessionCapabilities.demoMode` as the
canonical browser bootstrap signal, and shared billing or upgrade surfaces
must hide or suppress themselves from that contract rather than teaching mock
mode, response-header inference, or frontend-only feature flags to bypass the
real licensing model. That includes settings billing tabs, public-demo banner
and monitored-system/trial nudges, app-wide relay paywalls, Patrol upgrade
CTAs, and history-lock upsells. Demo readiness therefore means presentation
isolation, not a license exemption.
That same bootstrap owner must also avoid protected pre-auth noise. When auth
is configured but the browser is still on a public login entrypoint (`/` or
`/login`) with no local auth hint yet, `frontend-modern/src/useAppRuntimeState.ts`
must stop at the shared login-needed state and skip the `/api/state` probe, so
public demos and other browser-first login surfaces do not emit avoidable
`401` traffic before a session exists. Once the local login flow has written
its bootstrap hint or the operator is landing on a protected route, the
canonical state probe still owns authenticated runtime detection.
That same presentation-policy contract now owns the default self-hosted v6
commercial posture. Outside hosted mode, ordinary self-hosted installs must
fail closed on in-app upgrade prompts: Relay/Pro plan comparison, Pro trial
CTAs, paid-only settings navigation, and history/feature upsells may render
only for explicit handoff/direct routes, activation or recovery state, hosted
mode, or an already entitled install. Trial checkout plumbing may remain only
as legacy/support or externally initiated compatibility, but it is not a normal
GA self-hosted app journey.
Shared self-hosted plan presentation helpers must carry that same policy:
Community copy may mention provider/local-model Patrol, but must not present
hosted-model credits, account-backed AI access, or trial acquisition as a
default self-hosted benefit or a reason to put a paid prompt in front of
ordinary users.
The public offer, Pulse settings presentation, and Pulse Account handoff must
also describe Community, Relay, and Pro as distinct jobs rather than an
artificial feature ladder. Relay is the secure-access product. Pro is the
Patrol-powered operations product and explicitly includes Relay connectivity,
Pulse Mobile pairing, and push notifications. Retired Pro trial acquisition
copy and dormant Cloud checkout catalogs must not reappear in the ordinary
self-hosted journey; historical Cloud plan identifiers may remain only as
customer-state labels.
The Stripe billing portal is limited to invoices and payment methods. Every
portal-session request must name an explicit governed
`STRIPE_BILLING_PORTAL_CONFIGURATION_ID` beginning with `bpc_`; a missing or
invalid configuration fails closed. That Stripe configuration must keep
subscription plan updates disabled because Relay/Pro product and cadence
transitions stay in the Pulse-owned verified transition authority. Production
readiness therefore requires a read-only external proof that the configured
portal exposes only that intended scope before this commercial gate can pass.
The repo-owned license-server entrypoint must require that configuration before
the production binary starts, and the runtime configuration template and
operator documentation must describe the same restricted portal scope. This
turns a missing production value into a pre-deploy failure instead of a broken
customer manage-subscription path.
The public Stripe catalog audit must also enforce product descriptions that
name the one owner-operated environment boundary and state that Pro includes
Relay, so checkout cannot contradict the canonical package model.
That public-demo commercial boundary also owns monitored-system preview
unavailability wording. Browser presentation may keep the unavailable reason
nullable until the formatting edge, but it must normalize the message through
the shared monitored-system presentation helper instead of branching on
demo/billing state inside settings panels or inventing a second mock-only
license explanation path.
