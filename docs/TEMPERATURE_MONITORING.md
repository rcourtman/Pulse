# Temperature Monitoring

Pulse can collect host temperatures in two supported ways:

- Pulse agent on Proxmox hosts (recommended)
- SSH-based collection from the Pulse server (fallback or for non-agent hosts)

If you are upgrading from older releases that used `pulse-sensor-proxy`, see the legacy cleanup section below. The sensor proxy is no longer supported in Pulse.

## Recommended: Pulse Agent (Proxmox)

The unified agent runs on each Proxmox host and reports temperatures locally with no SSH keys needed.

```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <api-token> --enable-proxmox
```

Notes:
- Install `lm-sensors` on each host (`apt install lm-sensors && sensors-detect --auto`).
- Temperatures appear automatically once the agent reports.
- When a Proxmox host has recent usable agent temperature data, Pulse treats the agent as the source of truth and does not also try SSH temperature collection for that host.

## SSH-Based Collection (Fallback)

Pulse can also collect temperatures by SSHing into each host that does not have usable agent temperature data. The SSH path runs the Pulse sensor wrapper when present, falls back to `sensors -j`, and can fall back again to `/sys/class/thermal/thermal_zone0/temp` when available (for example, on Raspberry Pi).

### Requirements

- SSH connectivity from the Pulse server to each host
- `lm-sensors` installed and `sensors -j` returning JSON on the host
- A restricted SSH key entry that only allows the Pulse sensor wrapper

### Setup

1. Generate the node setup command from the UI:
   **Settings -> Infrastructure -> Add Node**
2. Run the command on each Proxmox host. The setup script can:
   - Create the required API user and permissions
   - Add a restricted SSH key entry for temperature collection
   - Install `lm-sensors` (optional)

The SSH entry added to `authorized_keys` is restricted to the Pulse sensor wrapper, for example:

```text
command="/usr/local/sbin/pulse-sensors",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty <public-key> # pulse-sensors
```

If you use a non-standard SSH port, set `SSH_PORT` (system-wide) or configure it in **Settings -> System**.

### Containerized Pulse

SSH-based collection from inside a container is not recommended for production. Prefer the agent method or run Pulse on the host. For dev/test, you can allow SSH from the container with:

```bash
PULSE_DEV_ALLOW_CONTAINER_SSH=true
```

### Verification

From the Pulse server, verify that SSH and sensors output work:

```bash
ssh -i /path/to/key root@node "sensors -j"
```

For platforms that expose a thermal zone file:

```bash
ssh -i /path/to/key root@node "cat /sys/class/thermal/thermal_zone0/temp"
```

### Troubleshooting

- If `sensors -j` returns empty output, run `sensors-detect --auto` and retry.
- If temperatures show as unavailable, confirm the host actually exposes sensor data.
- If the unified agent is already reporting temperatures for a Proxmox host, SSH collection is not required for that host.
- Ensure the SSH key entry is present and restricted to `/usr/local/sbin/pulse-sensors`.

## Legacy Cleanup (If Upgrading)

If you still have the old sensor proxy installed from prior releases, remove it from each **Proxmox host** (not the Pulse container) with the supported cleanup helper:

```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/uninstall-sensor-proxy.sh | \
  sudo bash -s -- --uninstall --purge
```

If you also want to remove the old `pulse-monitor@pam` API user and tokens before re-adding the node, include `--remove-proxmox-access`:

```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/uninstall-sensor-proxy.sh | \
  sudo bash -s -- --uninstall --purge --remove-proxmox-access
```

Reinstalling or upgrading the Pulse container does **not** remove the sensor proxy from the host — they are separate installations. If you skip this cleanup, the selfheal timer will keep running and may generate recurring `TASK ERROR` entries in the Proxmox task log.
