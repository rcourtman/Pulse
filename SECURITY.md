# Pulse Security Documentation

## Critical Security Notice for Production Deployments

### Container SSH Key Policy (BREAKING CHANGE)

**Effective immediately, SSH-based temperature monitoring is BLOCKED in containerized Pulse deployments.**

#### Why This Change?

Storing SSH private keys inside Docker containers creates an unacceptable security risk in production environments:

- **Container compromise = Infrastructure compromise**: If an attacker gains access to your Pulse container, they immediately obtain SSH private keys with root access to your Proxmox infrastructure.
- **Keys persist in images**: SSH keys can be extracted from container layers and images if pushed to registries.
- **No key rotation**: Long-lived keys in containers are difficult to rotate.
- **Violates principle of least privilege**: Containers should not hold credentials for the infrastructure they monitor.

#### Affected Deployments

✅ **Not Affected** (SSH temperature monitoring still allowed):
- Pulse installed directly on a VM or bare metal (non-containerized)
- Home lab deployments where you understand and accept the risk

❌ **BLOCKED** (SSH temperature monitoring disabled):
- Pulse running in Docker containers
- Pulse running in LXC containers
- Any deployment where `PULSE_DOCKER=true` or `/.dockerenv` exists

#### Migration Path

**For Production Container Deployments:**

1. **Deploy pulse-sensor-proxy on each Proxmox host:**
   ```bash
   # On each Proxmox host
   curl -o /usr/local/bin/pulse-sensor-proxy \
     https://github.com/rcourtman/pulse/releases/latest/download/pulse-sensor-proxy

   chmod +x /usr/local/bin/pulse-sensor-proxy
   ```

2. **Create systemd service** (`/etc/systemd/system/pulse-sensor-proxy.service`):
   ```ini
   [Unit]
   Description=Pulse Temperature Sensor Proxy
   After=network.target

   [Service]
   Type=simple
   User=root
   ExecStart=/usr/local/bin/pulse-sensor-proxy
   Restart=on-failure

   [Install]
   WantedBy=multi-user.target
   ```

3. **Enable and start:**
   ```bash
   systemctl daemon-reload
   systemctl enable --now pulse-sensor-proxy
   ```

4. **Restart Pulse container** - it will automatically detect and use the proxy

**Removing Existing SSH Keys:**

If you previously used SSH-based temperature monitoring in containers:

```bash
# On each Proxmox host, remove Pulse SSH keys
sed -i '/# pulse-/d' /root/.ssh/authorized_keys

# Inside the Pulse container (or destroy and recreate)
docker exec pulse rm -rf /home/pulse/.ssh/id_ed25519*
```

#### Technical Details

**How pulse-sensor-proxy Works:**

- Runs as a lightweight daemon on the Proxmox host
- Exposes a Unix socket at `/run/pulse-sensor-proxy.sock`
- Pulse container connects via bind-mounted socket
- Only exposes `sensors -j` output - no SSH access
- Keys never leave the Proxmox host

**Security Boundaries:**

```
┌─────────────────────────────────────┐
│  Proxmox Host                       │
│  ┌───────────────────────────────┐  │
│  │  pulse-sensor-proxy (root)    │  │
│  │  - Runs sensors -j            │  │
│  │  - Unix socket only           │  │
│  └───────────────────────────────┘  │
│            │                         │
│            │ /run/pulse-sensor-proxy.sock
│            │                         │
│  ┌─────────▼─────────────────────┐  │
│  │  Container (bind mount)       │  │
│  │  - No SSH keys                │  │
│  │  - No root access to host     │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

#### For Home Lab Users

If you understand and accept the risk, you can still use non-containerized Pulse with SSH keys:

1. Install Pulse directly on a VM (not in Docker)
2. Setup script will offer SSH temperature monitoring
3. Follow standard security practices:
   - Use dedicated monitoring user (not root)
   - Restrict key with `command="sensors -j"`
   - Add `from="<pulse-ip>"` restrictions
   - Rotate keys periodically

#### Audit Your Deployment

**Check if you're affected:**
```bash
# Inside Pulse container
ls /home/pulse/.ssh/id_ed25519* 2>/dev/null && echo "⚠️  VULNERABLE"

# On Proxmox host
grep "# pulse-" /root/.ssh/authorized_keys && echo "⚠️  SSH keys present"
```

**Check if proxy is working:**
```bash
# On Proxmox host
systemctl status pulse-sensor-proxy

# Inside Pulse container
docker logs pulse | grep -i "temperature proxy detected"
```

#### Timeline

- **Now**: SSH key generation blocked in containers (code-level enforcement)
- **Next Release**: Setup script updated with clear warnings
- **Future**: pulse-sensor-proxy bundled in official releases

#### Questions?

- Documentation: https://docs.pulseapp.io/security/containerized-deployments
- GitHub Issues: https://github.com/rcourtman/pulse/issues
- Security Issues: security@pulseapp.io (private disclosure)

---

## General Security Best Practices

### Authentication

- Use API tokens with minimal required permissions
- Rotate tokens regularly
- Never commit tokens to version control
- Use read-only tokens where possible

### Network Security

- Run Pulse in a dedicated monitoring VLAN
- Restrict Pulse's network access to only monitored systems
- Use firewall rules to limit inbound connections
- Enable TLS for all Proxmox API connections

### Monitoring

- Enable audit logging on Proxmox hosts
- Monitor Pulse container logs for suspicious activity
- Set up alerts for failed authentication attempts
- Review access logs regularly

### Updates

- Keep Pulse updated to latest stable version
- Subscribe to security announcements
- Test updates in staging before production
- Have rollback plan ready

---

Last updated: 2025-10-19
