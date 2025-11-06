# Temperature Monitoring

Pulse can display real-time CPU and NVMe temperatures directly in your dashboard, giving you instant visibility into your hardware health.

## Features

- **CPU Package Temperature**: Shows the overall CPU temperature when available
- **Individual Core Temperatures**: Tracks each CPU core
- **NVMe Drive Temperatures**: Monitors NVMe SSD temperatures (visible in the Storage tab's disk list)
- **Color-Coded Display**: 
  - Green: < 60°C (normal)
  - Yellow: 60-80°C (warm)
  - Red: > 80°C (hot)

## Deployment-Specific Setup

> **Important:** Temperature monitoring setup differs by deployment type:
> - **LXC containers:** Fully automatic via the setup script (Settings → Nodes → Setup Script)
> - **Docker containers:** Requires manual proxy installation (see below)
> - **Native installs:** Direct SSH, no proxy needed
>
> **For automation (Ansible/Terraform/etc.):** Jump to [Automation-Friendly Installation](#automation-friendly-installation)

## Quick Start for Docker Deployments

**Running Pulse in Docker?** Follow these steps to enable temperature monitoring:

### 1. Install the proxy on your Proxmox host

SSH to your **Proxmox host** (not the Docker container) and run:

```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-sensor-proxy.sh | \
  bash -s -- --standalone --pulse-server http://YOUR_PULSE_IP:7655
```

Replace `YOUR_PULSE_IP` with your Pulse server's IP address.

### 2. Add bind mount to docker-compose.yml

Add this volume to your Pulse container configuration:

```yaml
volumes:
  - pulse-data:/data
  - /run/pulse-sensor-proxy:/run/pulse-sensor-proxy:rw  # Add this line
```

### 3. Restart Pulse container

```bash
docker-compose down && docker-compose up -d
```

### 4. Verify

Check Pulse UI for temperature data, or verify the setup:

```bash
# Verify proxy is running on host
systemctl status pulse-sensor-proxy

# Verify socket is accessible in container
docker exec pulse ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock

# Check Pulse logs
docker logs pulse | grep -i "temperature.*proxy"
```

You should see: `Temperature proxy detected - using secure host-side bridge`

**Having issues?** See [Troubleshooting](#troubleshooting) below.

---

## Disable Temperature Monitoring

Don't need the sensor data? Open **Settings → Proxmox**, edit any node, and scroll to the **Advanced monitoring** section. The temperature toggle there controls collection for all nodes:

- When disabled, Pulse skips every SSH/proxy request for temperature data.
- CPU and NVMe readings disappear from dashboards and node tables.
- You can re-enable it later without re-running the setup scripts.

For scripted environments, set either:

- `temperatureMonitoringEnabled: false` in `/etc/pulse/system.json`, or
- `ENABLE_TEMPERATURE_MONITORING=false` in the environment (locks the UI toggle until removed).

## How It Works

### Secure Architecture (v4.24.0+)

For **containerized deployments** (LXC/Docker), Pulse uses a secure proxy architecture:

1. **pulse-sensor-proxy** runs on the Proxmox host (outside the container)
2. SSH keys are stored on the host filesystem (`/var/lib/pulse-sensor-proxy/ssh/`)
3. Pulse communicates with the proxy via unix socket
4. The proxy handles all SSH connections to cluster nodes

**Benefits:**
- SSH keys never enter the container
- Container compromise doesn't expose infrastructure credentials
- **LXC:** Automatically configured during installation (fully turnkey)
- **Docker:** Requires manual proxy installation and volume mount (see Quick Start above)

#### Manual installation (host-side)

When you need to provision the proxy yourself (for example via your own automation), run these steps on the host that runs your Pulse container:

1. **Install the binary**
   ```bash
   curl -L https://github.com/rcourtman/Pulse/releases/download/<TAG>/pulse-sensor-proxy-linux-amd64 \
     -o /usr/local/bin/pulse-sensor-proxy
   chmod 0755 /usr/local/bin/pulse-sensor-proxy
   ```
   Use the arm64/armv7 artefact if required.

2. **Create the service account if missing**
   ```bash
   id pulse-sensor-proxy >/dev/null 2>&1 || \
     useradd --system --user-group --no-create-home --shell /usr/sbin/nologin pulse-sensor-proxy
   ```

3. **Provision the data directories**
   ```bash
   install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0750 /var/lib/pulse-sensor-proxy
   install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0700 /var/lib/pulse-sensor-proxy/ssh
   ```

4. **(Optional) Add `/etc/pulse-sensor-proxy/config.yaml`**  
   Only needed if you want explicit subnet/metrics settings; otherwise the proxy auto-detects host CIDRs.
   ```yaml
   allowed_source_subnets:
     - 192.168.1.0/24
   metrics_address: 0.0.0.0:9127   # use "disabled" to switch metrics off
   ```

5. **Install the hardened systemd unit**  
   Copy the unit from `scripts/install-sensor-proxy.sh` or create `/etc/systemd/system/pulse-sensor-proxy.service` with:
   ```ini
   [Unit]
   Description=Pulse Temperature Proxy
   After=network.target

   [Service]
   Type=simple
   User=pulse-sensor-proxy
   Group=pulse-sensor-proxy
   WorkingDirectory=/var/lib/pulse-sensor-proxy
   ExecStart=/usr/local/bin/pulse-sensor-proxy
   Restart=on-failure
   RestartSec=5s
   RuntimeDirectory=pulse-sensor-proxy
   RuntimeDirectoryMode=0775
   UMask=0007
   NoNewPrivileges=true
   ProtectSystem=strict
   ProtectHome=read-only
   ReadWritePaths=/var/lib/pulse-sensor-proxy
   ProtectKernelTunables=true
   ProtectKernelModules=true
   ProtectControlGroups=true
   ProtectClock=true
   PrivateTmp=true
   PrivateDevices=true
   ProtectProc=invisible
   ProcSubset=pid
   LockPersonality=true
   RemoveIPC=true
   RestrictSUIDSGID=true
   RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
   RestrictNamespaces=true
   SystemCallFilter=@system-service
   SystemCallErrorNumber=EPERM
   CapabilityBoundingSet=
   AmbientCapabilities=
   KeyringMode=private
   LimitNOFILE=1024
   StandardOutput=journal
   StandardError=journal
   SyslogIdentifier=pulse-sensor-proxy

   [Install]
   WantedBy=multi-user.target
   ```

6. **Enable the service**
   ```bash
   systemctl daemon-reload
   systemctl enable --now pulse-sensor-proxy.service
   ```
   Confirm the socket appears at `/run/pulse-sensor-proxy/pulse-sensor-proxy.sock`.

7. **Expose the socket to Pulse**
   - **Proxmox LXC:** append `lxc.mount.entry: /run/pulse-sensor-proxy run/pulse-sensor-proxy none bind,create=dir 0 0` to `/etc/pve/lxc/<CTID>.conf` and restart the container.
   - **Docker:** bind mount `/run/pulse-sensor-proxy` into the container (`- /run/pulse-sensor-proxy:/run/pulse-sensor-proxy:rw`).

After the container restarts, the backend will automatically use the proxy. To refresh SSH keys on cluster nodes (e.g., after adding a new node), SSH to your Proxmox host and re-run the setup script: `curl -fsSL https://get.pulsenode.com/install-proxy.sh | bash -s -- --ctid <your-container-id>`

### Post-install Verification (v4.24.0+)

1. **Confirm proxy metrics**
   ```bash
   curl -s http://127.0.0.1:9127/metrics | grep pulse_proxy_build_info
   ```
2. **Ensure adaptive polling sees the proxy**
   ```bash
   curl -s http://localhost:7655/api/monitoring/scheduler/health \
     | jq '.instances[] | select(.key | contains("temperature")) | {key, pollStatus}'
   ```
   - Expect recent `lastSuccess` timestamps, `breaker.state == "closed"`, and `deadLetter.present == false`.
3. **Check update history** – Any future proxy restarts/rollbacks are logged under **Settings → System → Updates**; include the associated `event_id` in post-change notes.
4. **Measure queue depth/staleness** – Grafana panels `pulse_monitor_poll_queue_depth` and `pulse_monitor_poll_staleness_seconds` should return to baseline within a few polling cycles.

### Legacy Architecture (Pre-v4.24.0 / Native Installs)

For native (non-containerized) installations, Pulse connects directly via SSH:

1. Pulse uses SSH key authentication (like Ansible, Terraform, etc.)
2. Runs `sensors -j` command to read hardware temperatures
3. SSH key stored in Pulse's home directory

> **Important for native installs:** Run every setup command as the same user account that executes the Pulse service (typically `pulse`). The backend reads the SSH key from that user's home directory.

## Requirements

1. **SSH Key Authentication**: Your Pulse server needs SSH key access to nodes (no passwords)
2. **lm-sensors Package**: Installed on nodes to read hardware sensors
3. **Passwordless root SSH** (Proxmox clusters only): For proxy architecture, the Proxmox host running Pulse must have passwordless root SSH access to all cluster nodes. This is standard for Proxmox clusters but hardened environments may need to create an alternate service account.

## Setup (Automatic)

The auto-setup script (Settings → Nodes → Setup Script) provides different experiences based on deployment type:

### For LXC Deployments (Fully Automatic)

When run on a Proxmox host with Pulse in an LXC container:

1. Run the auto-setup script on your Proxmox node
2. The script automatically detects your Pulse LXC container
3. Installs `pulse-sensor-proxy` on the host
4. Configures the container bind mount automatically
5. Sets up SSH keys and cluster discovery
6. **Fully turnkey - no manual steps required!**

### For Docker Deployments (Manual Steps Required)

When Pulse runs in Docker, the setup script will show you manual steps:

1. Create the Proxmox API token (manual)
2. Add the node in Pulse UI
3. **For temperature monitoring**: Follow the [Quick Start for Docker](#quick-start-for-docker-deployments) above

### For Node Configuration (All Deployments)

When prompted for SSH setup on Proxmox nodes:

1. Choose "y" when asked about SSH configuration
2. The script will:
   - Install `lm-sensors`
   - Run `sensors-detect --auto`
   - Configure SSH keys (for standalone nodes)

If the node is part of a Proxmox cluster, the script will detect other members and offer to configure the same SSH/lm-sensors setup on each of them automatically.

### Host-side responsibilities (Docker only)

> **Note:** For LXC deployments, the setup script handles all of this automatically. This section applies to **Docker deployments only**.

- Run the host installer (`install-sensor-proxy.sh --standalone`) on the Proxmox machine that hosts Pulse to install and maintain the `pulse-sensor-proxy` service
- Add the bind mount to your docker-compose.yml: `- /run/pulse-sensor-proxy:/run/pulse-sensor-proxy:rw`
- Re-run the host installer if the service or socket disappears after a host upgrade or configuration cleanup; the installer is idempotent
- The installer ships a self-heal timer (`pulse-sensor-proxy-selfheal.timer`) that restarts or reinstalls the proxy if it ever goes missing; leave it enabled for automatic recovery
- Hot dev builds warn when only a container-local proxy socket is present, signaling that the host proxy needs to be reinstalled before temperatures will flow back into Pulse

### Turnkey Setup for Standalone Nodes (v4.25.0+)

**For standalone nodes** (not in a Proxmox cluster) running **containerized Pulse**, the setup script now automatically configures temperature monitoring with zero manual steps:

1. The script detects the node is standalone (not in a cluster)
2. Automatically fetches the temperature proxy's SSH public key from your Pulse server via `/api/system/proxy-public-key`
3. Installs it with forced commands (`command="sensors -j"`) automatically
4. Temperature monitoring "just works" - no manual SSH key management needed!

**Example output:**
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Standalone Node Temperature Setup
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Detected: This is a standalone node (not in a Proxmox cluster)

Fetching temperature proxy public key...
  ✓ Retrieved proxy public key
  ✓ Temperature proxy key installed (restricted to sensors -j)

✓ Standalone node temperature monitoring configured
  The Pulse temperature proxy can now collect temperature data
  from this node using secure SSH with forced commands.
```

**Security:**
- Public keys are safe to expose (it's in the name!)
- Forced commands restrict the key to only `command="sensors -j"`
- All other SSH features disabled (no-port-forwarding, no-pty, etc.)
- Works exactly like cluster setups, but fully automated

**Note:** This only works for containerized Pulse deployments where the temperature proxy is running. For native (non-containerized) installs, you'll still need to provide your Pulse server's public key manually as described in step 3 above.

## Setup (Manual)

If you skipped SSH setup during auto-setup, you can configure it manually:

### 1. Generate SSH Key (on Pulse server)

```bash
# Run as the user running Pulse (usually the pulse service account)
ssh-keygen -t rsa -N "" -f ~/.ssh/id_rsa
```

### 2. Copy Public Key to Proxmox Nodes

```bash
# Get your public key
cat ~/.ssh/id_rsa.pub

# Add it to each Proxmox node
ssh root@your-proxmox-node
mkdir -p /root/.ssh
chmod 700 /root/.ssh
echo "YOUR_PUBLIC_KEY_HERE" >> /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
```

### 3. Install lm-sensors (on each Proxmox node)

```bash
apt-get update
apt-get install -y lm-sensors
sensors-detect --auto
```

### 4. Test SSH Connection

From your Pulse server:
```bash
ssh root@your-proxmox-node "sensors -j"
```

You should see JSON output with temperature data.

## How It Works

1. Pulse uses SSH to connect to each node as root
2. Runs `sensors -j` to get temperature data in JSON format
3. Parses CPU temperatures (coretemp/k10temp)
4. Parses NVMe temperatures (nvme-pci-*)
5. Displays CPU temperatures on the overview dashboard and lists NVMe drive temperatures in the Storage tab's disk table when available

## Troubleshooting

### SSH Connection Attempts from Container ([preauth] Logs)

**Symptom:** Proxmox host logs (`/var/log/auth.log`) show repeated SSH connection attempts from your Pulse container:
```
Connection closed by authenticating user root <container-ip> port <port> [preauth]
```

**This indicates a misconfiguration.** Containerized Pulse should communicate via the sensor proxy, not direct SSH.

**Common causes:**
- Dev mode enabled (`PULSE_DEV_ALLOW_CONTAINER_SSH=true` environment variable)
- Sensor proxy not installed or socket not accessible
- Legacy SSH keys from pre-v4.24.0 installations

**Fix:**
- **Docker:** Follow [Quick Start for Docker Deployments](#quick-start-for-docker-deployments) to install the proxy and add the bind mount
- **LXC:** Run the setup script on your Proxmox host (see [Setup (Automatic)](#setup-automatic))
- **Dev mode:** Remove `PULSE_DEV_ALLOW_CONTAINER_SSH=true` from your environment/docker-compose
- **Verify:** Check Pulse logs for `Temperature proxy detected - using secure host-side bridge`

Once the proxy is properly configured, these log entries will stop immediately. See [Container Security Considerations](#container-security-considerations) for why direct container SSH is blocked.

### No Temperature Data Shown

**Check SSH access**:
```bash
# From Pulse server
ssh root@your-proxmox-node "echo test"
```

**Check lm-sensors**:
```bash
# On Proxmox node
sensors -j
```

**Check Pulse logs**:
```bash
journalctl -u pulse -f | grep -i temp
```

### Temperature Shows as Unavailable

- lm-sensors may not be installed
- Node may not have temperature sensors
- SSH key authentication may not be working

### ARM Devices (Raspberry Pi, etc.)

ARM devices typically don't have the same sensor interfaces. Temperature monitoring may not work or may show different sensors (like `thermal_zone0` instead of `coretemp`).

## Security & Architecture

### How Temperature Collection Works

Temperature monitoring uses **SSH key authentication** - the same trusted method used by automation tools like Ansible, Terraform, and Saltstack for managing infrastructure at scale.

**What Happens**:
1. Pulse connects to your node via SSH using a key (no passwords)
2. Runs `sensors -j` to get temperature readings in JSON format
3. Parses the data and displays it in the dashboard
4. Disconnects (entire operation takes <1 second)

**Security Design**:
- ✅ **Key-based authentication** - More secure than passwords, industry standard
- ✅ **Read-only operation** - `sensors` command only reads hardware data
- ✅ **Private key stays on Pulse server** - Never transmitted or exposed
- ✅ **Public key on nodes** - Safe to store, can't be used to gain access
- ✅ **Instantly revocable** - Remove key from authorized_keys to disable
- ✅ **Logged and auditable** - All connections logged in `/var/log/auth.log`

### What Pulse Uses SSH For

Pulse reuses the SSH access only for the actions already described in [Setup (Automatic)](#setup-automatic) and [How It Works](#how-it-works): adding the public key during setup (if you opt in) and polling `sensors -j` each cycle. It does nothing else—no extra commands, file changes, or config edits—and revoking the key stops temperature collection immediately.

This is the same security model used by thousands of organizations for infrastructure automation.

### Best Practices

1. **Dedicated key**: Generate a separate SSH key just for Pulse (recommended)
2. **Firewall rules**: Optionally restrict SSH to your Pulse server's IP
3. **Regular monitoring**: Review auth logs if you want extra visibility
4. **Secure your Pulse server**: Keep it updated and behind proper access controls

### Command Restrictions (Default)

Pulse now writes the temperature key with a forced command so the connection can only execute `sensors -j`. Port/X11/agent forwarding and PTY allocation are all disabled automatically when you opt in through the setup script. Re-running the script upgrades older installs to the restricted entry without touching any of your other SSH keys.

```bash
# Example entry in /root/.ssh/authorized_keys installed by Pulse
command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty ssh-rsa AAAAB3NzaC1yc2E...
```

You can still manage the entry manually if you prefer, but no extra steps are required for new installations.

## Performance Impact

- Minimal: SSH connection is made once per polling cycle
- Timeout: 5 seconds (non-blocking)
- Falls back gracefully if SSH fails
- No impact if SSH is not configured

## Container Security Considerations

✅ **Resolved in v4.24.0**

### Secure Proxy Architecture (Current)

As of v4.24.0, containerized deployments use **pulse-sensor-proxy** which eliminates the security concerns:

- **SSH keys stored on host** - Not accessible from container
- **Unix socket communication** - Pulse never touches SSH keys
- **Automatic during installation** - No manual configuration needed
- **Container compromise = No credential exposure** - Attacker gains nothing

**For new installations:** The proxy is installed automatically during LXC setup. No action required.

**Installed from inside an existing LXC?** The container-only installer cannot create the host bind mount. Run the host-side script below on your Proxmox node to enable temperature monitoring. When Pulse is running in that container, append the server URL so the proxy script can fall back to downloading the binary from Pulse itself if GitHub isn’t available.

**For existing installations (pre-v4.24.0):** Upgrade your deployment to use the proxy:

```bash
# On your Proxmox host
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-sensor-proxy.sh | \
  bash -s -- --ctid <your-pulse-container-id> --pulse-server http://<pulse-container-ip>:7655
```

> **Heads up for v4.23.x:** Those builds don't ship a standalone `pulse-sensor-proxy` binary yet and the HTTP fallback still requires authentication. Either upgrade to a newer release, install Pulse from source (`install.sh --source main`), or pass a locally built binary with `--local-binary /path/to/pulse-sensor-proxy`.

### Automation-Friendly Installation

For infrastructure-as-code tools (Ansible, Terraform, Salt, Puppet), the installer script is fully scriptable.

#### Installation Script Flags

```bash
install-sensor-proxy.sh [OPTIONS]
```

**Required (choose one):**
- `--ctid <id>` - For LXC containers (auto-configures bind mount)
- `--standalone` - For Docker or standalone deployments

**Optional:**
- `--pulse-server <url>` - Pulse server URL (for binary fallback if GitHub unavailable)
- `--version <tag>` - Specific version to install (default: latest)
- `--local-binary <path>` - Use local binary instead of downloading
- `--quiet` - Non-interactive mode (suppress progress output)
- `--skip-restart` - Don't restart LXC container after installation
- `--uninstall` - Remove the proxy service
- `--purge` - Remove data directories (use with --uninstall)

**Behavior:**
- ✅ **Idempotent** - Safe to re-run, won't break existing installations
- ✅ **Non-interactive** - Use `--quiet` for automated deployments
- ✅ **Verifiable** - Returns exit code 0 on success, non-zero on failure

#### Ansible Playbook Example

**For LXC deployments:**

```yaml
---
- name: Install Pulse sensor proxy for LXC
  hosts: proxmox_hosts
  become: yes
  tasks:
    - name: Download installer script
      get_url:
        url: https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-sensor-proxy.sh
        dest: /tmp/install-sensor-proxy.sh
        mode: '0755'

    - name: Install sensor proxy
      command: >
        /tmp/install-sensor-proxy.sh
        --ctid {{ pulse_container_id }}
        --pulse-server {{ pulse_server_url }}
        --quiet
      register: install_result
      changed_when: "'already exists' not in install_result.stdout"
      failed_when: install_result.rc != 0

    - name: Verify proxy is running
      systemd:
        name: pulse-sensor-proxy
        state: started
        enabled: yes
      register: service_status
```

**For Docker deployments:**

```yaml
---
- name: Install Pulse sensor proxy for Docker
  hosts: proxmox_hosts
  become: yes
  tasks:
    - name: Download installer script
      get_url:
        url: https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-sensor-proxy.sh
        dest: /tmp/install-sensor-proxy.sh
        mode: '0755'

    - name: Install sensor proxy (standalone mode)
      command: >
        /tmp/install-sensor-proxy.sh
        --standalone
        --pulse-server {{ pulse_server_url }}
        --quiet
      register: install_result
      failed_when: install_result.rc != 0

    - name: Verify proxy is running
      systemd:
        name: pulse-sensor-proxy
        state: started
        enabled: yes

    - name: Ensure docker-compose includes sensor proxy bind mount
      blockinfile:
        path: /opt/pulse/docker-compose.yml
        marker: "# {mark} ANSIBLE MANAGED - Sensor Proxy"
        insertafter: "volumes:"
        block: |
          - /run/pulse-sensor-proxy:/run/pulse-sensor-proxy:rw
      notify: restart pulse container

  handlers:
    - name: restart pulse container
      community.docker.docker_compose:
        project_src: /opt/pulse
        state: restarted
```

#### Terraform Example

```hcl
resource "null_resource" "pulse_sensor_proxy" {
  for_each = var.proxmox_hosts

  connection {
    type     = "ssh"
    host     = each.value.host
    user     = "root"
    private_key = file(var.ssh_private_key)
  }

  provisioner "remote-exec" {
    inline = [
      "curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-sensor-proxy.sh -o /tmp/install-sensor-proxy.sh",
      "chmod +x /tmp/install-sensor-proxy.sh",
      "/tmp/install-sensor-proxy.sh --standalone --pulse-server ${var.pulse_server_url} --quiet",
      "systemctl is-active pulse-sensor-proxy || exit 1"
    ]
  }

  triggers = {
    pulse_version = var.pulse_version
  }
}
```

#### Manual Configuration (No Script)

If you can't run the installer script, create the configuration manually:

**1. Download binary:**
```bash
curl -L https://github.com/rcourtman/Pulse/releases/latest/download/pulse-sensor-proxy-linux-amd64 \
  -o /usr/local/bin/pulse-sensor-proxy
chmod 0755 /usr/local/bin/pulse-sensor-proxy
```

**2. Create service user:**
```bash
useradd --system --user-group --no-create-home --shell /usr/sbin/nologin pulse-sensor-proxy
usermod -aG www-data pulse-sensor-proxy  # For pvecm access
```

**3. Create directories:**
```bash
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0750 /var/lib/pulse-sensor-proxy
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0700 /var/lib/pulse-sensor-proxy/ssh
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0755 /etc/pulse-sensor-proxy
```

**4. Create config (optional, for Docker):**
```yaml
# /etc/pulse-sensor-proxy/config.yaml
allowed_peer_uids: [1000]  # Docker container UID
allow_idmapped_root: true
allowed_idmap_users:
  - root
```

**5. Install systemd service:**
```bash
# Download from: https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-sensor-proxy.sh
# Extract the systemd unit from lines 630-730, or see systemd unit in installer script
systemctl daemon-reload
systemctl enable --now pulse-sensor-proxy
```

**6. Verify:**
```bash
systemctl status pulse-sensor-proxy
ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock
```

#### Configuration File Format

The proxy reads `/etc/pulse-sensor-proxy/config.yaml` (optional):

```yaml
# Allowed UIDs that can connect to the socket (default: [0] = root only)
allowed_peer_uids: [0, 1000]  # Allow root and UID 1000 (typical Docker)

# Allowed GIDs that can connect to the socket
allowed_peer_gids: [0]

# Allow ID-mapped root from LXC containers
allow_idmapped_root: true
allowed_idmap_users:
  - root

# Source subnets for SSH key restrictions (auto-detected if not specified)
allowed_source_subnets:
  - 192.168.1.0/24
  - 10.0.0.0/8

# Rate limiting (per calling UID)
rate_limit:
  per_peer_interval_ms: 1000  # 1 request per second
  per_peer_burst: 5           # Allow burst of 5

# Metrics endpoint (default: 127.0.0.1:9127)
metrics_address: 127.0.0.1:9127  # or "disabled"
```

**Environment Variable Overrides:**

Config values can also be set via environment variables (useful for containerized proxy deployments):

```bash
# Add allowed subnets (comma-separated, appends to config file values)
PULSE_SENSOR_PROXY_ALLOWED_SUBNETS=192.168.1.0/24,10.0.0.0/8

# Allow/disallow ID-mapped root (overrides config file)
PULSE_SENSOR_PROXY_ALLOW_IDMAPPED_ROOT=true
```

Example systemd override:
```ini
# /etc/systemd/system/pulse-sensor-proxy.service.d/override.conf
[Service]
Environment="PULSE_SENSOR_PROXY_ALLOWED_SUBNETS=192.168.1.0/24"
```

**Note:** Socket path, SSH key directory, and audit log path are configured via command-line flags (see main.go), not the YAML config file.

#### Re-running After Changes

The installer is idempotent and safe to re-run:

```bash
# After adding a new Proxmox node to cluster
bash install-sensor-proxy.sh --standalone --pulse-server http://pulse:7655 --quiet

# After upgrading Pulse version
bash install-sensor-proxy.sh --standalone --pulse-server http://pulse:7655 --version v4.27.0 --quiet

# Verify installation
systemctl status pulse-sensor-proxy
```

### Legacy Security Concerns (Pre-v4.24.0)

Older versions stored SSH keys inside the container, creating security risks:

- Compromised container = exposed SSH keys
- Even with forced commands, keys could be extracted
- Required manual hardening (key rotation, IP restrictions, etc.)

### Hardening Recommendations (Legacy/Native Installs Only)

#### 1. Key Rotation
Rotate SSH keys periodically (e.g., every 90 days):

```bash
# On Pulse server
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519_new -N ""

# Update all nodes' authorized_keys
# Test connectivity
ssh -i ~/.ssh/id_ed25519_new node "sensors -j"

# Replace old key
mv ~/.ssh/id_ed25519_new ~/.ssh/id_ed25519
```

#### 2. Secret Mounts (Docker)
Mount SSH keys from secure volumes:

```yaml
version: '3'
services:
  pulse:
    image: rcourtman/pulse:latest
    volumes:
      - pulse-ssh-keys:/home/pulse/.ssh:ro  # Read-only
      - pulse-data:/data
volumes:
  pulse-ssh-keys:
    driver: local
    driver_opts:
      type: tmpfs  # Memory-only, not persisted
      device: tmpfs
```

#### 3. Monitoring & Alerts
Enable SSH audit logging on Proxmox nodes:

```bash
# Install auditd
apt-get install auditd

# Watch SSH access
auditctl -w /root/.ssh -p wa -k ssh_access

# Monitor for unexpected commands
tail -f /var/log/audit/audit.log | grep ssh
```

#### 4. IP Restrictions
Limit SSH access to your Pulse server IP in `/etc/ssh/sshd_config`:

```ssh
Match User root Address 192.168.1.100
    ForceCommand sensors -j
    PermitOpen none
    AllowAgentForwarding no
    AllowTcpForwarding no
```

### Verifying Proxy Installation

To check if your deployment is using the secure proxy:

```bash
# On Proxmox host - check proxy service
systemctl status pulse-sensor-proxy

# Check if socket exists
ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock

# View proxy logs
journalctl -u pulse-sensor-proxy -f
```

In the Pulse container, check the logs at startup:
```bash
# Should see: "Temperature proxy detected - using secure host-side bridge"
journalctl -u pulse | grep -i proxy
```

### Disabling Temperature Monitoring

To remove SSH access:

```bash
# On each Proxmox node
sed -i '/pulse@/d' /root/.ssh/authorized_keys

# Or remove just the forced command entry
sed -i '/command="sensors -j"/d' /root/.ssh/authorized_keys
```

Temperature data will stop appearing in the dashboard after the next polling cycle.

## Operations & Troubleshooting

### Managing the Proxy Service

The pulse-sensor-proxy service runs on the Proxmox host (outside the container).

**Service Management:**
```bash
# Check service status
systemctl status pulse-sensor-proxy

# Restart the proxy
systemctl restart pulse-sensor-proxy

# Stop the proxy (disables temperature monitoring)
systemctl stop pulse-sensor-proxy

# Start the proxy
systemctl start pulse-sensor-proxy

# Enable proxy to start on boot
systemctl enable pulse-sensor-proxy

# Disable proxy autostart
systemctl disable pulse-sensor-proxy
```

### Log Locations

**Proxy Logs (on Proxmox host):**
```bash
# Follow proxy logs in real-time
journalctl -u pulse-sensor-proxy -f

# View last 50 lines
journalctl -u pulse-sensor-proxy -n 50

# View logs since last boot
journalctl -u pulse-sensor-proxy -b

# View logs with timestamps
journalctl -u pulse-sensor-proxy --since "1 hour ago"
```

**Pulse Logs (in container):**
```bash
# Check if proxy is being used
journalctl -u pulse | grep -i "proxy\|temperature"

# Should see: "Temperature proxy detected - using secure host-side bridge"
```

### SSH Key Rotation

Rotate SSH keys periodically for security (recommended every 90 days).

**Automated Rotation (Recommended):**

The `/opt/pulse/scripts/pulse-proxy-rotate-keys.sh` script handles rotation safely with staging, verification, and rollback support:

```bash
# 1. Dry-run first (recommended)
sudo /opt/pulse/scripts/pulse-proxy-rotate-keys.sh --dry-run

# 2. Perform rotation
sudo /opt/pulse/scripts/pulse-proxy-rotate-keys.sh
```

**What the script does:**
- Generates new Ed25519 keypair in staging directory
- Pushes new key to all cluster nodes via proxy RPC
- Verifies SSH connectivity with new key on each node
- Atomically swaps keys (current → backup, staging → active)
- Preserves old keys for rollback

**If rotation fails, rollback:**
```bash
sudo /opt/pulse/scripts/pulse-proxy-rotate-keys.sh --rollback
```

**Manual Rotation (Fallback):**

If the automated script fails or is unavailable:

```bash
# 1. On Proxmox host, backup old keys
cd /var/lib/pulse-sensor-proxy/ssh/
cp id_ed25519 id_ed25519.backup
cp id_ed25519.pub id_ed25519.pub.backup

# 2. Generate new keypair
ssh-keygen -t ed25519 -f id_ed25519 -N "" -C "pulse-sensor-proxy-rotated"

# 3. Re-run setup to push keys to cluster
curl -fsSL https://get.pulsenode.com/install-proxy.sh | bash -s -- --ctid <your-container-id>

# 4. Verify temperature data still works in Pulse UI
```

### Automatic Cleanup When Nodes Are Removed (v4.26.0+)

Starting in v4.26.0, SSH keys are **automatically removed** when you delete a node from Pulse:

1. **When you remove a node** in Pulse Settings → Nodes, Pulse signals the temperature proxy
2. **The proxy creates a cleanup request** file at `/var/lib/pulse-sensor-proxy/cleanup-request.json`
3. **A systemd path unit detects the request** and triggers the cleanup service
4. **The cleanup script automatically:**
   - SSHs to the specified node (or localhost if it's local)
   - Removes the SSH key entries (`# pulse-managed-key` and `# pulse-proxy-key`)
   - Logs the cleanup action via syslog

**Automatic cleanup works for:**
- ✅ **Cluster nodes** - Full automatic cleanup (Proxmox clusters have unrestricted passwordless SSH)
- ⚠️ **Standalone nodes** - Cannot auto-cleanup due to forced command security (see below)

**Standalone Node Limitation:**

Standalone nodes use forced commands (`command="sensors -j"`) for security. This same restriction prevents the cleanup script from running `sed` to remove keys. This is a **security feature, not a bug** - adding a workaround would defeat the forced command protection.

For standalone nodes:
- Keys remain after removal (but they're **read-only** - only `sensors -j` access)
- **Low security risk** - no shell access, no write access, no port forwarding
- **Auto-cleanup on re-add** - Setup script removes old keys when node is re-added
- **Manual cleanup if needed:**
  ```bash
  ssh root@standalone-node "sed -i '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys"
  ```

**Monitoring Cleanup:**
```bash
# Watch cleanup operations in real-time
journalctl -u pulse-sensor-cleanup -f

# View cleanup history
journalctl -u pulse-sensor-cleanup --since "1 week ago"

# Check if cleanup system is active
systemctl status pulse-sensor-cleanup.path
```

**Manual Cleanup (if needed):**

If automatic cleanup fails or you need to manually revoke access:

```bash
# On the node being removed, remove all Pulse SSH keys
ssh root@old-node "sed -i -e '/# pulse-managed-key\$/d' -e '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys"

# Or remove them locally
sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys

# No restart needed - proxy will fail gracefully for that node
# Temperature monitoring will continue for remaining nodes
```

### Failure Modes

**Proxy Not Running:**
- Symptom: No temperature data in Pulse UI
- Check: `systemctl status pulse-sensor-proxy` on Proxmox host
- Fix: `systemctl start pulse-sensor-proxy`

**Socket Not Accessible in Container:**
- Symptom: Pulse logs show "Temperature proxy not available - using direct SSH"
- Check: `ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock` in container
- Fix: Verify bind mount in LXC config (`/etc/pve/lxc/<CTID>.conf`)
- Should have: `lxc.mount.entry: /run/pulse-sensor-proxy run/pulse-sensor-proxy none bind,create=dir 0 0`

**pvecm Not Available:**
- Symptom: Proxy fails to discover cluster nodes
- Cause: Pulse runs on non-Proxmox host
- Fallback: Use legacy direct SSH method (native installation)

**Pulse Running Off-Cluster:**
- Symptom: Proxy discovers local host but not remote cluster nodes
- Limitation: Proxy requires passwordless SSH between cluster nodes
- Solution: Ensure Proxmox host running Pulse has SSH access to all cluster nodes

**Unauthorized Connection Attempts:**
- Symptom: Proxy logs show "Unauthorized connection attempt"
- Cause: Process with non-root UID trying to access socket
- Normal: Only root (UID 0) or proxy's own user can access socket
- Check: Look for suspicious processes trying to access the socket

### Monitoring the Proxy

**Manual Monitoring (v1):**

The proxy service includes systemd restart-on-failure, which handles most issues automatically. For additional monitoring:

```bash
# Check proxy health
systemctl is-active pulse-sensor-proxy && echo "Proxy is running" || echo "Proxy is down"

# Monitor logs for errors
journalctl -u pulse-sensor-proxy --since "1 hour ago" | grep -i error

# Verify socket exists and is accessible
test -S /run/pulse-sensor-proxy/pulse-sensor-proxy.sock && echo "Socket OK" || echo "Socket missing"
```

**Alerting:**
- Rely on systemd's automatic restart (`Restart=on-failure`)
- Monitor via journalctl for persistent failures
- Check Pulse UI for missing temperature data

**Future:** Integration with pulse-watchdog is planned for automated health checks and alerting (see #528).

### Known Limitations

**Single Proxy = Single Point of Failure:**
- Each Proxmox host runs one pulse-sensor-proxy instance
- If the proxy service dies, temperature monitoring stops for all containers on that host
- This is acceptable for read-only telemetry, but be aware of the failure mode
- Systemd auto-restart (`Restart=on-failure`) mitigates most outages
- If multiple Pulse containers run on same host, they share the same proxy

**Sensors Output Parsing Brittleness:**
- Pulse depends on `sensors -j` JSON output format from lm-sensors
- Changes to sensor names, structure, or output format could break parsing
- Consider adding schema validation and instrumentation to detect issues early
- Monitor proxy logs for parsing errors: `journalctl -u pulse-sensor-proxy | grep -i "parse\|error"`

**Cluster Discovery Limitations:**
- Proxy uses `pvecm status` to discover cluster nodes (requires Proxmox IPC access)
- If Proxmox hardens IPC access or cluster topology changes unexpectedly, discovery may fail
- Standalone Proxmox nodes work but only monitor that single node
- Fallback: Re-run setup script manually to reconfigure cluster access

**Rate Limiting & Scaling** (updated in commit 46b8b8d):

**What changed:** pulse-sensor-proxy now defaults to 1 request per second with a burst of 5 per calling UID. Earlier builds throttled after two calls every five seconds, which caused temperature tiles to flicker or fall back to `--` as soon as clusters reached three or more nodes.

**Symptoms of saturation:**
- Temperature widgets flicker between values and `--`, or entire node rows disappear after adding new hardware
- `Settings → System → Updates` shows no proxy restarts, yet scheduler health reports breaker openings for temperature pollers
- Proxy logs include `limiter.rejection` or `Rate limit exceeded` entries for the container UID

**Diagnose:**
1. Check scheduler health for temperature pollers:
   ```bash
   curl -s http://localhost:7655/api/monitoring/scheduler/health \
     | jq '.instances[] | select(.key | contains("temperature")) \
       | {key, lastSuccess: .pollStatus.lastSuccess, breaker: .breaker.state, deadLetter: .deadLetter.present}'
   ```
   Breakers that remain `open` or repeated dead letters indicate the proxy is rejecting calls.
2. Inspect limiter metrics on the host:
   ```bash
   curl -s http://127.0.0.1:9127/metrics \
     | grep -E 'pulse_proxy_limiter_(rejects|penalties)_total'
   ```
   A rising counter confirms the limiter is backing off callers.
3. Review logs for throttling:
   ```bash
   journalctl -u pulse-sensor-proxy -n 100 | grep -i "rate limit"
   ```

**Tuning guidance:** Add a `rate_limit` block to `/etc/pulse-sensor-proxy/config.yaml` (see `cmd/pulse-sensor-proxy/config.example.yaml`) when clusters grow beyond the defaults. Use the formula `per_peer_interval_ms = polling_interval_ms / node_count` and set `per_peer_burst ≥ node_count` to allow one full sweep per polling window.

| Deployment size | Nodes | 10 s poll interval → interval_ms | Suggested burst | Notes |
| --- | --- | --- | --- | --- |
| Small | 1–3 | 1000 (default) | 5 | Works for most single Proxmox hosts. |
| Medium | 4–10 | 500 | 10 | Halves wait time; keep burst ≥ node count. |
| Large | 10–20 | 250 | 20 | Monitor CPU on proxy; consider staggering polls. |
| XL | 30+ | 100–150 | 30–50 | Only enable after validating proxy host capacity. |

**Security note:** Lower intervals increase throughput and reduce UI staleness, but they also allow untrusted callers to issue more RPCs per second. Keep `per_peer_interval_ms ≥ 100` in production and continue to rely on UID allow-lists plus audit logs when raising limits.

**SSH latency monitoring:**
- Monitor SSH latency metrics: `curl -s http://127.0.0.1:9127/metrics | grep pulse_proxy_ssh_latency`

**Requires Proxmox Cluster Membership:**
- Proxy requires passwordless root SSH between cluster nodes
- Standard for Proxmox clusters, but hardened environments may differ
- Alternative: Create dedicated service account with sudo access to `sensors`

**No Cross-Cluster Support:**
- Proxy only manages the cluster its host belongs to
- Cannot bridge temperature monitoring across multiple disconnected clusters
- Each cluster needs its own Pulse instance with its own proxy

### Common Issues

**Temperature Data Stops Appearing:**
1. Check proxy service: `systemctl status pulse-sensor-proxy`
2. Check proxy logs: `journalctl -u pulse-sensor-proxy -n 50`
3. Test SSH manually: `ssh root@node "sensors -j"`
4. Verify socket exists: `ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock`

**New Cluster Node Not Showing Temperatures:**
1. Ensure lm-sensors installed: `ssh root@new-node "sensors -j"`
2. Proxy auto-discovers on next poll (may take up to 1 minute)
3. Re-run the setup script to configure SSH keys on the new node: `curl -fsSL https://get.pulsenode.com/install-proxy.sh | bash -s -- --ctid <CTID>`

**Permission Denied Errors:**
1. Verify socket permissions: `ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock`
2. Should be: `srw-rw---- 1 root root`
3. Check Pulse runs as root in container: `pct exec <CTID> -- whoami`

**Proxy Service Won't Start:**
1. Check logs: `journalctl -u pulse-sensor-proxy -n 50`
2. Verify binary exists: `ls -l /usr/local/bin/pulse-sensor-proxy`
3. Test manually: `/usr/local/bin/pulse-sensor-proxy --version`
4. Check socket directory: `ls -ld /var/run`

### Future Improvements

**Potential Enhancements (Roadmap):**

1. **Proxmox API Integration**
   - If future Proxmox versions expose temperature telemetry via API, retire SSH approach
   - Would eliminate SSH key management and improve security posture
   - Monitor Proxmox development for metrics/RRD temperature endpoints

2. **Agent-Based Architecture**
   - Deploy lightweight agents on each node for richer telemetry
   - Reduces SSH fan-out overhead for large clusters
   - Trade-off: Adds deployment/update complexity
   - Consider only if demand for additional metrics grows

3. **SNMP/IPMI Support**
   - Optional integration for baseboard management controllers
   - Better for hardware-level sensors (baseboard temps, fan speeds)
   - Requires hardware/firmware support, so keep as optional add-on

4. **Schema Validation**
   - Add JSON schema validation for `sensors -j` output
   - Detect format changes early with instrumentation
   - Log warnings when unexpected sensor formats appear

5. **Caching & Throttling**
   - Implement result caching for large clusters (10+ nodes)
   - Reduce SSH overhead with configurable TTL
   - Add request throttling to prevent SSH rate limiting

6. **Automated Key Rotation**
   - Systemd timer for automatic 90-day rotation
   - Already supported via `/opt/pulse/scripts/pulse-proxy-rotate-keys.sh`
   - Just needs timer unit configuration (documented in hardening guide)

7. **Health Check Endpoint**
   - Add `/health` endpoint separate from Prometheus metrics
   - Enable external monitoring systems (Nagios, Zabbix, etc.)
   - Return proxy status, socket accessibility, and last successful poll

**Contributions Welcome:** If any of these improvements interest you, open a GitHub issue to discuss implementation!

### Getting Help

If temperature monitoring isn't working:

1. **Collect diagnostic info:**
   ```bash
   # On Proxmox host
   systemctl status pulse-sensor-proxy
   journalctl -u pulse-sensor-proxy -n 100 > /tmp/proxy-logs.txt
   ls -la /run/pulse-sensor-proxy/pulse-sensor-proxy.sock

   # In Pulse container
   journalctl -u pulse -n 100 | grep -i temp > /tmp/pulse-temp-logs.txt
   ```

2. **Test manually:**
   ```bash
   # On Proxmox host - test SSH to a cluster node
   ssh root@cluster-node "sensors -j"
   ```

3. **Check GitHub Issues:** https://github.com/rcourtman/Pulse/issues
4. **Include in bug report:**
   - Pulse version
   - Deployment type (LXC/Docker/native)
   - Proxy logs
   - Pulse logs
   - Output of manual SSH test
