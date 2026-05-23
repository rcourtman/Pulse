# Provider-First Platform Landing Surface

Date: 2026-05-23
Lane: L8 frontend primitives
Assertion: `provider-first-platform-landing-surface`

## Outcome

Pulse v6 authenticated root and login handoff now resolves to the first visible
provider or runtime platform in canonical shell order: Proxmox, Containers,
Kubernetes, TrueNAS, vSphere, then Agents only when the estate is agent-only.

Agents is a standalone Pulse-agent-primary machine surface. It must not become
the primary estate landing when provider evidence exists, and legacy
Infrastructure remains route-compatible rather than the default operational
surface.

The app shell also no longer prewarms retired Infrastructure or Workloads chart
caches as a generic authenticated side effect. Platform-first startup stays
route-module warm and data-light until the selected platform page owns its
normal resource/table query.

## Proof

- `selectFirstVisiblePrimaryInfrastructureNavigationId` uses the canonical
  provider-first order and falls back to Agents only for agent-only estates.
- Root/login redirect uses that selector through `getDefaultWorkspaceRoute`.
- Desktop and mobile primary nav, command palette, keyboard shortcuts, route
  preloading, and active-tab helpers now keep provider platforms ahead of
  Agents.
- `useAppRuntimeState` does not import or prewarm Infrastructure or Workloads
  chart caches.
