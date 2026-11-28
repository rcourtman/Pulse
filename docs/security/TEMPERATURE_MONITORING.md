# Temperature Monitoring Security

Pulse supports two temperature collection paths: the unified agent (recommended) and SSH-based collection from the Pulse server. This page summarizes the security tradeoffs.

## Recommended: Pulse Agent

The unified agent (`pulse-agent --enable-proxmox`) runs locally on each Proxmox host and reports temperature metrics directly to Pulse. No SSH keys are stored on the server, and access is scoped to the agent token.

Benefits:
- Local sensor access only
- No inbound SSH requirement
- Standard agent auth and transport

See [docs/TEMPERATURE_MONITORING.md](../TEMPERATURE_MONITORING.md) for setup.

## SSH-Based Collection

SSH-based temperature monitoring uses a restricted key entry that only allows the Pulse sensor wrapper to run. This limits the blast radius if a key leaks.

Recommended restrictions:

```text
from="<pulse-ip>",command="/usr/local/sbin/pulse-sensors",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty <public-key> # pulse-sensors
```

Additional notes:
- Use a dedicated key for temperature collection only, and rotate it regularly.
- Keep host SSH private keys out of Pulse containers. Automatic SSH-key generation is blocked in containers unless the development override `PULSE_DEV_ALLOW_CONTAINER_SSH=true` is set, while an already mounted key can still be used by the legacy collector with a security warning. That override does not make container-held keys safe. Prefer the host-local Unified Agent instead.
- With multi-tenant organizations, the sensors SSH key is instance-scoped, so SSH temperature collection runs only in the **default** organization and is disabled on non-default tenants. Use the Unified Agent for non-default tenant hosts.

See [docs/TEMPERATURE_MONITORING.md](../TEMPERATURE_MONITORING.md) for the full setup flow.

## Related Docs

- Unified Agent Security: [docs/AGENT_SECURITY.md](../AGENT_SECURITY.md)
- Repository Security Policy: [SECURITY.md](../../SECURITY.md)
