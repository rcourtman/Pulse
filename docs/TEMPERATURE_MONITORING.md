# Temperature Monitoring

Pulse can collect and display CPU and NVMe temperature data from your Proxmox nodes via SSH.

## Features

- **CPU Package Temperature**: Shows the overall CPU temperature
- **Individual Core Temperatures**: Tracks each CPU core
- **NVMe Drive Temperatures**: Monitors NVMe SSD temperatures
- **Color-Coded Display**: 
  - Green: < 60°C (normal)
  - Yellow: 60-80°C (warm)
  - Red: > 80°C (hot)

## Requirements

1. **SSH Access**: Pulse needs SSH key authentication to the Proxmox nodes
2. **lm-sensors**: Must be installed on each node you want to monitor

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

## Setup (Manual)

If you skipped SSH setup during auto-setup, you can configure it manually:

### 1. Generate SSH Key (on Pulse server)

```bash
# Run as the user running Pulse (usually root or pulse)
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
5. Displays the data in node cards with color coding

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

## Security Notes

### What Access Is Granted

Temperature monitoring requires SSH key authentication to run `sensors -j` on your Proxmox nodes. Here's exactly what this means:

**Access Level**:
- SSH access to `root@your-proxmox-node` from your Pulse server
- Uses SSH key authentication (no passwords)
- Pulse executes only: `sensors -j 2>/dev/null`
- 5 second timeout per connection
- No interactive shell, no other commands

**Security Model**:
- ✅ **SSH key stored on Pulse server only** - Private key never leaves your Pulse server
- ✅ **Public key on Proxmox nodes** - Only the public key is added to authorized_keys
- ✅ **Read-only operation** - sensors command only reads hardware data
- ✅ **No write access** - Cannot modify any Proxmox configuration
- ✅ **Revocable** - Remove the key from authorized_keys to revoke access instantly
- ✅ **Auditable** - All SSH connections are logged in Proxmox auth logs

**Best Practices**:
1. Only enable if you trust your Pulse server's security
2. Use a dedicated SSH key for Pulse (not your personal key)
3. Protect the Pulse server with proper firewalls/security
4. Regularly review `/var/log/auth.log` on Proxmox nodes
5. Consider using firewall rules to restrict SSH to your Pulse server's IP

**Risk Assessment**:
- **If Pulse server is compromised**: Attacker gains SSH access to your Proxmox nodes as root
- **Mitigation**: Secure your Pulse server, use dedicated SSH keys, monitor logs
- **Alternative**: Skip SSH setup and manually check temperatures via Proxmox web UI

### Restricting SSH Access (Advanced)

If you want to limit SSH access to only the sensors command, you can use `authorized_keys` restrictions:

```bash
# In /root/.ssh/authorized_keys on Proxmox node:
command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty ssh-rsa AAAAB3NzaC1yc2E...
```

This forces the key to only run `sensors -j` and nothing else.

## Performance Impact

- Minimal: SSH connection is made once per polling cycle
- Timeout: 5 seconds (non-blocking)
- Falls back gracefully if SSH fails
- No impact if SSH is not configured
