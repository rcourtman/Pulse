# Proxmox guest lifecycle Patrol loop — 2026-07-13

## Outcome

Pulse now has a production Patrol detector-to-verified-fix path for Proxmox VM
and LXC lifecycle incidents. A fresh running-to-stopped transition creates a
bounded reliability finding only when the stopped resource advertises its exact
governed `start` capability. The normal shared action lifecycle then plans,
asks for approval, dispatches through the owning node agent, records the audit,
and reconciles the finding from canonical verification truth.

## Safety floor

- First observations seed a baseline; guests already stopped at startup do not
  become incidents.
- Stale, missing, repeated-clock, unknown, template, locked, and capability-less
  observations do not emit or clear findings.
- VM and LXC routes require their exact resource-owned lifecycle handlers.
- Finding evidence contains only bounded Proxmox identity/state facts and no
  command, path, credential, token, or raw provider output.
- Execution success stays separate from postcondition truth. `fix_verified`
  requires a confirmed independent Proxmox control-plane observation.

## Proof

- `internal/ai/findings_proxmox_lifecycle_test.go` proves VM/LXC transitions,
  baseline behavior, freshness/capability/guest guardrails, recovery/removal,
  and production Patrol wiring before the provider gate.
- `internal/unifiedresources/views_test.go` proves detached lifecycle
  capabilities and canonical per-source freshness on VM/LXC typed views.
- `internal/api/proxmox_patrol_integration_test.go` proves the full finding,
  exact `start {}` proposal, human decision, `qm start` dispatch, independent
  Proxmox API verification, durable action audit, terminal notification, and
  originating finding reconciliation to `fix_verified`.

## Residual

This closes the missing Proxmox lifecycle detector floor within the broader
Monitor-First Patrol Workbench candidate. It does not claim that whole candidate
lane complete. Mobile approval presentation and decision/execution parity
remain governed separately by `lane-followup:mobile-post-rc-hardening`.
