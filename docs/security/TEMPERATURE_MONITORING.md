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

SSH-based temperature monitoring uses a restricted key entry that only allows `sensors -j` to run. This limits the blast radius if a key leaks.

Recommended restrictions:

```text
command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty <public-key> # pulse-sensors
```

Additional notes:
- Use a dedicated key for temperature collection only.
- Avoid running Pulse in a container for SSH-based collection. If you must for dev/test, set `PULSE_DEV_ALLOW_CONTAINER_SSH=true` and keep access tightly scoped.

See [docs/TEMPERATURE_MONITORING.md](../TEMPERATURE_MONITORING.md) for the full setup flow.

## Related Docs

- Unified Agent Security: [docs/AGENT_SECURITY.md](../AGENT_SECURITY.md)
- Repository Security Policy: [SECURITY.md](../../SECURITY.md)
