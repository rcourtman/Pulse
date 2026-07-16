# Pulse v6.1.0-rc.2

_This changelog describes the cumulative `v6.1.0-rc.2` release candidate
compared with stable `v6.0.5`. It supersedes `v6.1.0-rc.1`._

## Added

- Pulse Intelligence now exposes explicit detection and investigation
  profiles over the canonical typed action lifecycle.
- Patrol can capture typed action proposals and route approved work through
  the shared lifecycle instead of command-side execution paths.
- Desktop and Pulse Mobile clients now use the canonical pending-action queue,
  decision endpoint, and action identity instead of retired command-shaped
  approval endpoints.
- The desktop Actions inbox now provides one review surface for proposed work,
  policy provenance, approval state, execution progress, verification detail,
  and direct Patrol handoffs.
- Docker and Kubernetes actions verify supported scale and update outcomes
  after execution.
- Policy-scoped Patrol autonomy can authorize low-risk Docker and Podman
  restarts only for explicitly allowed resources and optional recurring
  maintenance windows.
- Host update, Debian and Ubuntu package, storage-pressure cleanup, Docker
  restart, and supported Proxmox guest lifecycle plans now share the governed
  action, durable receipt, and independent-verification path.
- Local provider setup includes a guided Ollama `qwen3:8b` quickstart.
- Cluster members can override discovered connection addresses.
- The Unified Agent accepts a rotating JSON log path for native service use.
- Live Patrol qualification now exercises model-led investigations, typed tool
  use, finding quality, remediation planning, and negative controls.
- Claude subscription-backed models now support schema-bound streaming turns,
  native tools, bounded preflight, and retry-safe durable outcomes.
- Docker inventory identifies conflicting shared agent identities, and registry
  clients can negotiate bearer tokens from authentication challenges.

## Changed

- Connected systems, platform attention states, and responsive layouts are
  task-first and monitor-first across the main product surfaces.
- Assistant and Patrol settings now use the Pulse Intelligence product name
  consistently while retaining Patrol as the detection and investigation
  engine.
- The provider MSP portal now uses product design tokens and dark mode, and
  its no-email-provider behavior is explicit and self-sufficient.
- Native-agent update application now runs through one store action and one
  transport-independent lifecycle service.
- Action transitions reconcile investigation outcomes from the authoritative
  audit at write time and hydrate missed transitions when investigations are
  read later.
- Investigation prompts include the validated capability catalog with approval
  floors, parameter schemas, and sensitive/operator-only constraints.
- Assistant supports in-place retry, response regeneration, edit-and-resend,
  mid-turn steering, collapsed long-paste attachments, and estimated
  last-turn cost summaries.
- Patrol action handoffs open the matching Actions review, background Patrol
  sessions stay out of Assistant quick resume, and pending approvals are
  visible on the Actions navigation tab.
- Docker, Kubernetes, TrueNAS, vSphere, and Proxmox node tables share one
  sortable platform-table model and retain user-controlled column ordering.
- Windows native CI exercises installer parsing, install, version replacement,
  logged readiness, forced-process recovery, restart persistence, and cleanup.
- Release automation builds one signed exact-SHA candidate and promotes that
  candidate without rebuilding.
- Action dispatch now binds server-authored policy provenance and reviewed plan
  identity through durable admission, transport, result, and audit records.
- Patrol separates Watch detection from investigation, bounds evidence and model
  turns, preserves multiple accepted findings, and keeps canonical resource
  identities through typed tool calls.
- Docker update and restart work remains on reviewed typed plans with durable
  receipts and reconnect recovery.
- Commercial plan, cadence, entitlement, revocation, and downgrade handling now
  shares an installation-scoped lifecycle that preserves customer data.

## Fixed

- Replacement binaries are self-tested before the in-app updater swaps them
  into place.
- Edition-aware update checks block silent Pro-to-community downgrades and
  preserve an explicit rollback route.
- Docker update guidance recreates the container so the selected image is
  actually applied.
- Docker and Kubernetes agent liveness handles clock skew correctly.
- Docker updates preserve containers that share another container's network
  namespace.
- Guest suppression is honored for posture alerts.
- Legacy OIDC callback handling recovers the initiating provider.
- Simultaneous provider/runtime and proposal-channel failures preserve both
  failure causes across the core and enterprise boundary.
- Terminal action verification now drives honest desktop and mobile completion
  or failure state, including canonical push action identities.
- FreeBSD update recovery preserves the native agent lifecycle.
- Windows service installation fails closed when required recovery actions or
  non-crash recovery cannot be configured.
- Windows install success now requires both local readiness and a non-empty
  durable service log.
- Discovery backfills quiesce during shutdown.
- Cluster members retain an operator-selected connection address when they
  re-register through another member.
- Guest alerts move with the guest when its owning node changes, and missing
  guest-agent disk data uses an unavailable sentinel instead of a fabricated
  measurement.
- Physical disk tables keep wide-node disks visible and avoid reporting
  standby SSDs as active failures.
- SSO-backed administrators retain their effective settings privileges.
- Patrol rejects untrusted instructions, invalid reconfirmation shortcuts, and
  ungrounded health claims while using authoritative restart and OOM evidence.
- OIDC sessions without refresh tokens remain valid where allowed, mixed-auth
  startup avoids deadlock, and Basic-auth identity reaches action authorization.
- Deleted hosts can re-enroll with fresh credentials, continuity state serves
  agent configuration during reload windows, and Windows version checks
  normalize a leading `v`.
- Constrained NAS installs no longer require `od`; recovery-point, TrueNAS, and
  `nvme-eui` disk reconciliation preserve authoritative identity.
- Availability polling cadence, alert timezones, Docker change history, and
  stable web-interface links now reflect authoritative values.

## Security

- First-run security boundaries now fail closed.
- Autonomous dispatches fail closed when remediation lock state is unknown.
- Request, storage, integer-conversion, allocation, and cookie boundaries have
  additional CodeQL-driven hardening.
- macOS native release signing and notarization are required for this RC;
  Windows Authenticode remains required for stable promotion and is disclosed
  explicitly while the public signing application is pending.
- Demo SSH setup no longer weakens host-key handling for private deploy hosts
  or IP targets.
- Update verification documentation and release output use a guarded OpenSSH
  `allowed_signers` line, and hardened update services keep configuration
  backups on a writable path.

## Release Metadata

- Version: `v6.1.0-rc.2`
- Rollback target: `v6.0.5`
- Rollback command: `./scripts/install.sh --version v6.0.5`
- Promotion path: release candidate from `main`
- Mobile companion candidates: iOS build 10 and Android versionCode 8 on the
  TestFlight and Google Play internal-testing tracks
