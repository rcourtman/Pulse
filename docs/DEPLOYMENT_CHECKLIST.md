# Deployment Checklist - Security Hardening Update
## Version: 2025-11-07 Security Hardening Release

---

## Pre-Deployment Checklist

### 1. Review Changes
- [ ] Read `docs/SECURITY_AUDIT_2025-11-07.md` for full context
- [ ] Review `docs/SECURITY_CHANGELOG.md` for breaking changes
- [ ] Understand the `:ro` socket mount requirement

### 2. Backup Current State
```bash
# Backup configuration
cp /etc/pulse-sensor-proxy/config.yaml /etc/pulse-sensor-proxy/config.yaml.backup

# Backup systemd unit
cp /etc/systemd/system/pulse-sensor-proxy.service \
   /etc/systemd/system/pulse-sensor-proxy.service.backup

# Note current service status
systemctl status pulse-sensor-proxy > /tmp/proxy-status-before.txt

# Backup docker-compose (if using Docker)
cp docker-compose.yml docker-compose.yml.backup
```

### 3. Verify Test Results
- [ ] All tests pass: `go test ./cmd/pulse-sensor-proxy ./internal/ssh/knownhosts`
- [ ] Binary builds: `go build ./cmd/pulse-sensor-proxy`
- [ ] No compilation errors or warnings

---

## Deployment Steps

### For Docker Deployments

#### Step 1: Update docker-compose.yml
```bash
# Change socket mount from :rw to :ro
sed -i 's|/run/pulse-sensor-proxy:rw|/run/pulse-sensor-proxy:ro|g' docker-compose.yml

# Verify change
grep pulse-sensor-proxy docker-compose.yml
# Should show: - /run/pulse-sensor-proxy:/run/pulse-sensor-proxy:ro
```

#### Step 2: Optional Configuration
Create `/etc/pulse-sensor-proxy/config.yaml` (optional but recommended):
```yaml
# Node allowlist (prevents SSRF)
allowed_nodes:
  - "10.0.0.0/24"  # Replace with your cluster network
strict_node_validation: true

# Timeouts (prevents DoS)
read_timeout: 5s
write_timeout: 10s

# SSH output limits
max_ssh_output_bytes: 1048576  # 1MB

# Host key management
require_proxmox_hostkeys: false  # Set true for strict mode

# Capability-based authorization (optional)
allowed_peers:
  - uid: 0
    capabilities: [read, write, admin]
  - uid: 1000
    capabilities: [read]  # Docker containers: read-only
```

#### Step 3: Restart Services
```bash
# Restart proxy on host
sudo systemctl restart pulse-sensor-proxy

# Check proxy status
sudo systemctl status pulse-sensor-proxy

# Restart Pulse container
docker compose down
docker compose up -d

# Check container logs
docker logs pulse -f
# Look for: "Temperature proxy detected - using secure host-side bridge"
```

---

### For LXC Deployments

#### Step 1: Verify Mount Configuration
```bash
# Check mount is already correct (install script uses correct format)
pct config <VMID> | grep mp

# Should see: mp0: /run/pulse-sensor-proxy,mp=/mnt/pulse-proxy
# If shows old format with explicit :rw, update manually
```

#### Step 2: Update Systemd Unit
```bash
# Copy new hardened unit
sudo cp scripts/pulse-sensor-proxy.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Verify new directives loaded
systemctl cat pulse-sensor-proxy | grep -E "(MemoryDeny|RestrictRealtime)"
```

#### Step 3: Optional Configuration
Create `/etc/pulse-sensor-proxy/config.yaml` (same as Docker above)

#### Step 4: Restart Proxy
```bash
sudo systemctl restart pulse-sensor-proxy
sudo systemctl status pulse-sensor-proxy
```

---

## Verification Steps

### 1. Proxy Health Check
```bash
# Check proxy is running
sudo systemctl status pulse-sensor-proxy

# Check socket exists and is accessible
ls -la /run/pulse-sensor-proxy/pulse-sensor-proxy.sock

# Test socket from host
echo '{"method":"get_status"}' | \
  sudo socat - UNIX-CONNECT:/run/pulse-sensor-proxy/pulse-sensor-proxy.sock | jq .
# Should return: {"success":true,"data":{"version":"dev","public_key":"...","ssh_dir":"..."}}
```

### 2. Container Access Verification

**Docker:**
```bash
# Check socket is visible in container
docker exec pulse ls -la /run/pulse-sensor-proxy/

# Verify container CANNOT write to directory (security fix)
docker exec pulse touch /run/pulse-sensor-proxy/test 2>&1
# Should fail with: "Read-only file system" (GOOD)

# Check Pulse logs for proxy detection
docker logs pulse 2>&1 | grep -i "temperature.*proxy"
# Should see: "Temperature proxy detected - using secure host-side bridge"
```

**LXC:**
```bash
# Check socket is visible in container
pct exec <VMID> -- ls -la /mnt/pulse-proxy/

# Verify container cannot write
pct exec <VMID> -- touch /mnt/pulse-proxy/test 2>&1
# Should fail (GOOD)

# Check Pulse logs
pct exec <VMID> -- journalctl -u pulse -n 50 | grep -i temperature
```

### 3. Temperature Data Collection
```bash
# Access Pulse UI: http://your-server:7655
# Navigate to any Proxmox node
# Verify temperature data is shown (if hardware supports it)

# Or check via API
curl http://localhost:7655/api/nodes | jq '.[].temperature'
```

### 4. Security Feature Verification

**Node Validation (if configured):**
```bash
# Check logs for validation (if you added allowed_nodes)
sudo journalctl -u pulse-sensor-proxy -n 50 | grep -i validation
```

**Metrics Check:**
```bash
# Check new security metrics exist
curl -s http://localhost:7655/metrics | grep pulse_proxy_node_validation
curl -s http://localhost:7655/metrics | grep pulse_proxy_read_timeouts
curl -s http://localhost:7655/metrics | grep pulse_proxy_limiter

# Or check proxy metrics directly (if exposed)
curl -s http://127.0.0.1:9127/metrics | grep pulse_proxy
```

### 5. Rate Limiting Verification
```bash
# Check limiter is using range-based identification for containers
sudo journalctl -u pulse-sensor-proxy -n 100 | grep -i "range:"
# Should see logs like: peer="range:100000-165535" if container is accessing

# Check metrics
curl -s http://127.0.0.1:9127/metrics | grep pulse_proxy_limiter_rejections_total
```

---

## Post-Deployment Monitoring

### Immediate (First Hour)
- [ ] Watch proxy logs: `journalctl -u pulse-sensor-proxy -f`
- [ ] Watch Pulse logs: `docker logs pulse -f` or `journalctl -u pulse -f`
- [ ] Check for errors or warnings
- [ ] Verify temperature data is updating in UI

### First Day
- [ ] Monitor for security events in logs
- [ ] Check metrics for anomalies
- [ ] Verify no legitimate requests being blocked

### First Week
- [ ] Review security metrics trends
- [ ] Check for any false positives in node validation
- [ ] Adjust configuration if needed

---

## Rollback Procedure

If issues occur:

### Quick Rollback (Docker)
```bash
# Restore old docker-compose
cp docker-compose.yml.backup docker-compose.yml

# Restart
docker compose down && docker compose up -d
```

### Quick Rollback (LXC)
```bash
# Restore old systemd unit
sudo cp /etc/systemd/system/pulse-sensor-proxy.service.backup \
        /etc/systemd/system/pulse-sensor-proxy.service

# Restore old config (if needed)
sudo cp /etc/pulse-sensor-proxy/config.yaml.backup \
        /etc/pulse-sensor-proxy/config.yaml

# Reload and restart
sudo systemctl daemon-reload
sudo systemctl restart pulse-sensor-proxy
```

### Git Rollback
```bash
# Revert to previous commit
git revert HEAD
git push

# Rebuild and redeploy
go build ./cmd/pulse-sensor-proxy
sudo systemctl restart pulse-sensor-proxy
```

---

## Troubleshooting

### Issue: "Temperature proxy not detected"

**Symptom:** Pulse logs don't show proxy detection message

**Check:**
```bash
# 1. Verify proxy is running
sudo systemctl status pulse-sensor-proxy

# 2. Verify socket exists
ls -la /run/pulse-sensor-proxy/pulse-sensor-proxy.sock

# 3. Verify container can see socket
docker exec pulse ls -la /run/pulse-sensor-proxy/  # Docker
pct exec <VMID> -- ls -la /mnt/pulse-proxy/        # LXC

# 4. Check mount is correct
docker inspect pulse | grep -A 5 Mounts  # Docker
pct config <VMID> | grep mp               # LXC
```

**Fix:**
- Ensure socket directory is mounted into container
- Verify mount path matches Pulse expectation
- Restart both proxy and Pulse

---

### Issue: "Read-only file system" errors in container

**Symptom:** Container logs show read-only errors for `/run/pulse-sensor-proxy/`

**This is EXPECTED and CORRECT!** The mount is intentionally read-only for security.

**If Pulse itself has issues:**
- Pulse should only READ from the socket, not write
- Check Pulse isn't trying to write to the socket directory
- This should not happen in normal operation

---

### Issue: "Node validation failed" or "potential SSRF attempt"

**Symptom:** Temperature requests failing, logs show validation errors

**Check:**
```bash
# Check your allowed_nodes configuration
cat /etc/pulse-sensor-proxy/config.yaml | grep -A 5 allowed_nodes

# Check what nodes Pulse is trying to connect to
sudo journalctl -u pulse-sensor-proxy | grep "potential SSRF"
```

**Fix:**
- Add legitimate nodes to `allowed_nodes` list
- Or set `strict_node_validation: false` temporarily for debugging
- Verify node hostnames/IPs are correct

---

### Issue: High rate limit rejections

**Symptom:** Metrics show many `pulse_proxy_limiter_rejections_total`

**Check:**
```bash
# Check which peers are being limited
curl -s http://127.0.0.1:9127/metrics | grep limiter_rejections_total

# Check logs for details
sudo journalctl -u pulse-sensor-proxy | grep "Rate limit"
```

**Fix:**
- This may be normal if you have many nodes
- Rate limits are per-container-range (not per-UID) now
- Adjust `rate_limit` config if needed:
  ```yaml
  rate_limit:
    per_peer_interval_ms: 500  # Increase to 2 req/sec
    per_peer_burst: 10         # Allow larger bursts
  ```

---

## Metrics to Monitor

### Security Metrics
- `pulse_proxy_node_validation_failures_total` - Should be 0 (any value = potential attack)
- `pulse_proxy_read_timeouts_total` - Low is good (high = possible attack)
- `pulse_proxy_limiter_rejections_total` - Some is normal (very high = issue)
- `pulse_proxy_hostkey_changes_total` - Should be 0 (any value = investigate)
- `pulse_proxy_ssh_output_oversized_total` - Should be 0 (any value = investigate)

### Operational Metrics
- `pulse_proxy_rpc_requests_total{method="get_temperature",result="success"}` - Should increase steadily
- `pulse_proxy_queue_depth` - Should be low (< 5)
- `pulse_proxy_global_concurrency_inflight` - Should be low (< 8)

---

## Success Criteria

✅ Deployment is successful when:

- [ ] Proxy service is running and stable
- [ ] Socket is mounted read-only in container
- [ ] Temperature data appears in Pulse UI
- [ ] No errors in proxy or Pulse logs
- [ ] Security metrics show no anomalies
- [ ] All temperature pollers report healthy in scheduler health endpoint

---

## Support

**If issues persist:**

1. Check full logs: `journalctl -u pulse-sensor-proxy -n 500 > /tmp/proxy-logs.txt`
2. Check configuration: `cat /etc/pulse-sensor-proxy/config.yaml`
3. Review audit report: `docs/SECURITY_AUDIT_2025-11-07.md`
4. File issue: https://github.com/rcourtman/Pulse/issues

---

**Deployment checklist last updated:** 2025-11-07
