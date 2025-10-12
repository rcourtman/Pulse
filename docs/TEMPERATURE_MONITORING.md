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

## How It Works

Temperature monitoring uses standard SSH key authentication (just like Ansible, Saltstack, and other automation tools) to securely collect sensor data from your nodes. Pulse connects via SSH and runs the `sensors` command to read hardware temperatures - that's it!

> **Important:** Run every setup command as the same user account that executes the Pulse service (typically `pulse`). The backend reads the SSH key from that user’s home directory; keys under `root` or other accounts will be ignored.

## Requirements

1. **SSH Key Authentication**: Your Pulse server needs SSH key access to nodes (no passwords)
2. **lm-sensors Package**: Installed on nodes to read hardware sensors

## Setup (Automatic)

The auto-setup script (Settings → Nodes → Setup Script) will prompt you to configure SSH access for temperature monitoring:

1. Run the auto-setup script on your Proxmox node
2. When prompted for SSH setup, choose "y"
3. Get your Pulse server's public key:
   ```bash
   # On your Pulse server (run as the user running Pulse)
   cat ~/.ssh/id_rsa.pub
   ```
4. Paste the public key when prompted
5. The script will:
   - Add the key to `/root/.ssh/authorized_keys`
   - Install `lm-sensors`
   - Run `sensors-detect --auto`

If the node is part of a Proxmox cluster, the script will now detect the other members and offer to configure the same SSH/lm-sensors setup on each of them automatically—confirm when prompted to roll it out cluster-wide.

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

⚠️ **Important for Docker/LXC deployments**

If you run Pulse in a container (Docker or LXC), SSH private keys are stored inside the container filesystem. This creates additional security considerations:

### Risk

A compromised Pulse container could expose SSH keys that access your Proxmox hosts, even with forced command restrictions.

### Opt-In Required (v4.23.1+)

Temperature monitoring now requires explicit confirmation during setup. The setup script displays a security notice and asks for your consent before enabling SSH access.

### Runtime Warning

Pulse logs a security warning at startup if it detects:
- Running in a container (Docker/LXC/containerd)
- SSH keys present for temperature monitoring

This reminds you to review security hardening recommendations.

### Hardening Recommendations

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

### Future: Agent-Based Architecture

**Status:** Planned for future release

The current SSH approach is a legacy implementation. Future versions will use agent-based monitoring where:

- Lightweight temperature agents run on each Proxmox node
- Agents **push** metrics to Pulse over authenticated HTTPS
- No SSH keys stored in Pulse
- Better security boundary between monitoring and infrastructure

This is the recommended architecture for production deployments. The SSH method will be maintained as a fallback for simple setups.

### When to Use SSH vs Waiting for Agents

**SSH Method (Current) - Acceptable for:**
- Home labs and trusted networks
- Non-containerized Pulse deployments
- Environments where you trust the container host

**Wait for Agents - Better for:**
- Production infrastructure
- Multi-tenant environments
- High-security requirements
- Containerized Pulse with untrusted container hosts

### Disabling Temperature Monitoring

To remove SSH access:

```bash
# On each Proxmox node
sed -i '/pulse@/d' /root/.ssh/authorized_keys

# Or remove just the forced command entry
sed -i '/command="sensors -j"/d' /root/.ssh/authorized_keys
```

Temperature data will stop appearing in the dashboard after the next polling cycle.
