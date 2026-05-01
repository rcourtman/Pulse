# Agent Lifecycle Agent Privilege Documentation Record

- Date: `2026-05-01`
- Lane: `L16`
- Related discussion: `#1445`

## Context

Discussion `#1445` asked whether Pulse agents should run as root, whether a
lower-privilege read-only mode exists, and what happens if a release source is
compromised. The runtime already defaults command execution off and verifies
self-update payloads, but the public agent security guide did not state the
Linux/systemd privilege model directly.

## Outcome

`docs/AGENT_SECURITY.md` now explains the RC3 agent security posture:

- Linux/systemd agents run as `root` by default for full host telemetry;
- a lower-privilege profile is not currently a supported full-telemetry mode;
- command execution remains disabled by default and should stay disabled for
  read-only monitoring;
- self-updates require checksum validation and release signatures where trusted
  update keys are embedded;
- initial root shell installers are a separate trust boundary, so release-pinned
  and signature-verified server installer flows are preferred.

This addresses the user-facing security question without changing the governed
agent runtime contract for RC3.
