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

## SSH-Based Collection (Fallback)

Pulse can also collect temperatures by SSHing into each host and running `sensors -j`, with a fallback to `/sys/class/thermal/thermal_zone0/temp` when available (for example, on Raspberry Pi).

### Requirements

- SSH connectivity from the Pulse server to each host
- `lm-sensors` installed and `sensors -j` returning JSON on the host
- A restricted SSH key entry that only allows `sensors -j`

### Setup

1. Generate the node setup command from the UI:
   **Settings -> Infrastructure -> Add Node**
2. Run the command on each Proxmox host. The setup script can:
   - Create the required API user and permissions
   - Add a restricted SSH key entry for temperature collection
   - Install `lm-sensors` (optional)

The SSH entry added to `authorized_keys` is restricted to `sensors -j`, for example:

```text
command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty <public-key> # pulse-sensors
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
- Ensure the SSH key entry is present and restricted to `sensors -j`.

## Legacy Cleanup (If Upgrading)

If you still have the old sensor proxy installed from prior releases, remove it from each **Proxmox host** (not the Pulse container):

```bash
# Stop and disable all sensor-proxy systemd units
sudo systemctl disable --now pulse-sensor-proxy pulse-sensor-proxy-selfheal.timer pulse-sensor-proxy-selfheal.service pulse-sensor-cleanup.path pulse-sensor-cleanup.service 2>/dev/null

# Remove systemd unit files
sudo rm -f /etc/systemd/system/pulse-sensor-proxy.service
sudo rm -f /etc/systemd/system/pulse-sensor-proxy-selfheal.timer
sudo rm -f /etc/systemd/system/pulse-sensor-proxy-selfheal.service
sudo rm -f /etc/systemd/system/pulse-sensor-cleanup.service
sudo rm -f /etc/systemd/system/pulse-sensor-cleanup.path
sudo systemctl daemon-reload

# Remove sensor-proxy files
sudo rm -rf /opt/pulse/sensor-proxy
sudo rm -rf /etc/pulse-sensor-proxy
sudo rm -rf /var/lib/pulse-sensor-proxy
sudo rm -rf /var/log/pulse/sensor-proxy
sudo rm -rf /run/pulse-sensor-proxy

# Optional: remove sensor-proxy SSH keys from authorized_keys
sudo sed -i '/# pulse-managed-key$/d;/# pulse-proxy-key$/d' /root/.ssh/authorized_keys
```

Reinstalling or upgrading the Pulse container does **not** remove the sensor proxy from the host — they are separate installations. If you skip this cleanup, the selfheal timer will keep running and may generate recurring `TASK ERROR` entries in the Proxmox task log.
