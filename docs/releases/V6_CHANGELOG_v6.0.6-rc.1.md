# Pulse v6.0.6-rc.1

_This changelog describes the `v6.0.6-rc.1` release candidate compared with
stable `v6.0.5`._

## Added

- Pulse Intelligence now exposes explicit detection and investigation
  profiles over the canonical typed action lifecycle.
- Patrol can capture typed action proposals and route approved work through
  the shared lifecycle instead of command-side execution paths.
- Desktop and Pulse Mobile clients now use the canonical pending-action queue,
  decision endpoint, and action identity instead of retired command-shaped
  approval endpoints.
- Docker and Kubernetes actions verify supported scale and update outcomes
  after execution.
- Policy-scoped Patrol autonomy can authorize low-risk Docker and Podman
  restarts only for explicitly allowed resources and optional recurring
  maintenance windows.
- Local provider setup includes a guided Ollama `qwen3:8b` quickstart.
- Cluster members can override discovered connection addresses.
- The Unified Agent accepts a rotating JSON log path for native service use.

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
- Docker, Kubernetes, TrueNAS, vSphere, and Proxmox node tables share one
  sortable platform-table model and retain user-controlled column ordering.
- Windows native CI exercises installer parsing, install, version replacement,
  logged readiness, forced-process recovery, restart persistence, and cleanup.
- Release automation builds one signed exact-SHA candidate and promotes that
  candidate without rebuilding.

## Fixed

- Replacement binaries are self-tested before the in-app updater swaps them
  into place.
- Edition-aware update checks block silent Pro-to-community downgrades and
  preserve an explicit rollback route.
- Docker update guidance recreates the container so the selected image is
  actually applied.
- Docker and Kubernetes agent liveness handles clock skew correctly.
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

## Release Metadata

- Version: `v6.0.6-rc.1`
- Rollback target: `v6.0.5`
- Promotion path: release candidate from `main`
