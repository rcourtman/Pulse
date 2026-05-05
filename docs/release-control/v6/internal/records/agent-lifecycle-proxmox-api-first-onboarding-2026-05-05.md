# Agent Lifecycle Proxmox API-First Onboarding

Date: 2026-05-05

## Decision

New Proxmox VE and Proxmox Backup Server setup defaults to the API Inventory
path. The root Pulse Agent install remains supported, but it is presented as
the Host Telemetry Agent path for full node-local telemetry rather than as the
general Proxmox default.

## Evidence

- `frontend-modern/src/utils/nodeModalPresentation.ts` now initializes new
  PVE/PBS node setup in `auto` mode, so the setup guide opens on the API
  inventory command path.
- `frontend-modern/src/components/Settings/NodeModalSetupGuideSection.tsx`
  labels API Inventory as the recommended least-privilege path and labels the
  agent command as optional full host telemetry for SMART, temperatures, local
  storage details, and agent-driven operations.
- `frontend-modern/src/utils/infrastructureOnboardingPresentation.ts` now
  describes Proxmox-family onboarding as API-first with the agent optional
  where node-local telemetry is needed.
- `docs/AGENT_SECURITY.md` and `docs/UNIFIED_AGENT.md` carry the same operator
  guidance: start with Proxmox API inventory and install agents only where
  the API cannot provide the required data.

## Proof Commands

Run as part of this slice:

- `npm --prefix frontend-modern test -- --run src/components/Settings/__tests__/NodeModalSetupGuideSection.test.tsx src/utils/__tests__/nodeModalPresentation.test.ts src/utils/__tests__/infrastructureOnboardingPresentation.test.ts src/components/Settings/__tests__/InfrastructureWorkspace.test.tsx src/components/Settings/__tests__/settingsArchitecture.test.ts`
- `npm --prefix frontend-modern run type-check`
- Playwright inspection of the Settings Infrastructure Proxmox add flow at
  desktop and mobile viewports after the frontend build updated.

## Follow-Up Guard

Future Proxmox setup copy may still recommend a host agent for temperatures,
SMART, Docker/Podman, local storage detail, and command execution, but it must
not make the root agent install the default route for ordinary Proxmox API
inventory.
