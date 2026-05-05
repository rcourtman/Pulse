# Agent Lifecycle Root Agent Hardening Record

- Date: `2026-05-05`
- Lane: `L16`
- Trigger: Proxmox community concern about root Pulse agent deployments

## Context

Operators asked whether deploying Pulse agents as `root` on Proxmox, VMs, and
containers creates avoidable risk for read-only monitoring. The canonical
full-telemetry Linux agent still needs root-equivalent local access for SMART,
temperature, Docker/Podman socket, Proxmox-local, and NAS/platform data, but
the shipped defaults did not need to expose the agent health/metrics HTTP
surface on every interface.

## Outcome

The root-agent boundary now has tighter defaults without claiming a supported
non-root full-telemetry profile:

- `pulse-agent` binds health and Prometheus endpoints to `127.0.0.1:9191` by
  default instead of all interfaces;
- explicit network scraping remains opt-in through `--health-addr :9191`;
- the agent can disable that listener with `--health-addr ""` or
  `PULSE_HEALTH_ADDR=off`;
- `scripts/install.sh` passes explicit health-address choices into the
  installed service and verifies custom or disabled health listeners without
  falsely reporting an unhealthy install;
- generated Linux/systemd agent units include conservative sandboxing
  directives (`NoNewPrivileges`, `PrivateTmp`, kernel/control-group write
  protections, private umask, setuid/personality restrictions, and native
  syscall architecture restriction) that reduce blast radius while preserving
  the filesystem/device access required by supported full host telemetry.

The customer-facing security documentation now distinguishes API-only Proxmox
monitoring from full host-agent telemetry and documents the new listener and
systemd hardening defaults.
