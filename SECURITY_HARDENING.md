# Temperature Monitoring Security Hardening Roadmap

This document outlines post-launch security improvements for the Pulse temperature monitoring system with pulse-sensor-proxy.

## Completed Security Fixes ✅

### 1. SSH Command Injection (CRITICAL) - Fixed in commit 124ab7826
**Issue**: Container could inject SSH options via malicious node hostnames
- Example: `node="-oProxyCommand=sh -c 'evil code'"`
- Impact: Remote code execution on Proxmox host

**Fix**:
- Strengthened hostname validation regex: `^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`
- Added validation to all RPC handlers (V1 and V2)
- Hostnames must start with alphanumeric character

### 2. Unauthorized Key Distribution (HIGH) - Fixed in commit d55112ac4
**Issue**: Compromised containers could call privileged RPC methods
- `ensureClusterKeys`: Trigger SSH key distribution
- `registerNodes`: Learn cluster topology, DOS cluster nodes
- Impact: Host-level operations accessible from containers

**Fix**:
- Added method-level authorization
- Privileged methods restricted to host processes only
- ID-mapped root (containers) blocked from privileged methods
- Containers can still call `get_temperature` and `get_status`

## Post-Launch Hardening Tasks 📋

### 3. Socket ACL Multi-Tenancy Improvements (MEDIUM)
**Current State**:
- Authentication uses UID-based ACL via SO_PEERCRED
- Allows: root (UID 0), proxy's UID, configured UIDs, ID-mapped root ranges
- Authorization now includes method-level restrictions (commit d55112ac4)

**Problem**:
- Privilege escalation inside container grants proxy access
- Single compromised container compromises entire proxy
- No per-client authentication tokens

**Proposed Solutions**:

**Option A: Per-Client Tokens** (Recommended)
- Generate unique token for each LXC container during setup
- Store token in container's environment or systemd override
- Client sends token with each RPC request
- Proxy validates token before processing
- Revocation: Remove token from proxy's allowlist

**Option B: Mutual TLS**
- Generate client certificate for each container
- Mount certificate into container (read-only)
- TLS socket authentication
- Certificate revocation via CRL

**Option C: Tighter ACL + Audit Logging**
- Keep UID-based ACL but add comprehensive audit logging
- Log all RPC calls with caller credentials, method, parameters
- Alert on suspicious patterns (excessive calls, failures)
- Easier to implement but doesn't solve root cause

**Decision Required**:
- Which approach fits Pulse's deployment model?
- Balance between security and operational complexity
- Consider backup/restore implications

**Target**: v4.24.0 (2-3 weeks post-launch)

---

### 4. Direct SSH Fallback Policy (MEDIUM)
**Current State**:
`internal/tempproxy/client.go` silently falls back to direct SSH if proxy unavailable

**Problem**:
- Fallback requires SSH keys inside container
- Undermines primary security objective (no secrets in containers)
- Silent fallback hides configuration issues

**Proposed Solutions**:

**Option A: Remove Fallback Entirely** (Strictest)
- Fail fast if proxy unavailable
- Force operators to fix proxy issues
- Temperature monitoring becomes hard dependency

**Option B: Opt-In Fallback with Warnings** (Recommended)
- Environment variable: `PULSE_ALLOW_DIRECT_SSH_FALLBACK=true`
- Log prominent warning when falling back
- Dashboard alert: "Temperature monitoring using fallback mode"
- Document security trade-offs clearly

**Option C: Read-Only Key Fallback**
- If fallback needed, use separate read-only SSH key
- Key can ONLY run `sensors -j` (forced command)
- Limit blast radius of key compromise

**Decision Required**:
- Is temperature monitoring critical enough to require fallback?
- Can we trust operators to fix proxy issues quickly?
- What's the UX for "temperature unavailable"?

**Target**: v4.24.0 (2-3 weeks post-launch)

---

### 5. Client Resilience & Observability (MEDIUM)
**Current State**:
- Client makes synchronous RPC calls without deadlines
- No exponential backoff on failures
- No distinction between transport errors vs command errors

**Problem**:
- Network hiccups can cause goroutine pileup
- Sensor command timeouts block request handling
- Difficult to debug client-side issues

**Proposed Improvements**:

**5.1 Add Context Deadlines**
```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
tempData, err := proxyClient.GetTemperatureWithContext(ctx, nodeHost)
```

**5.2 Exponential Backoff**
```go
type backoffConfig struct {
    InitialDelay time.Duration // 100ms
    MaxDelay     time.Duration // 30s
    Multiplier   float64       // 2.0
    Jitter       float64       // 0.1
}
```

**5.3 Error Classification**
```go
type ProxyError struct {
    Type    ErrorType // Transport, Auth, SSH, Sensor, Unknown
    Message string
    Retry   bool
}
```

**5.4 Circuit Breaker Pattern**
- Track failure rate per node
- Open circuit after N consecutive failures
- Half-open state for testing recovery
- Full open after sustained failures

**5.5 Structured Metrics**
- `pulse_tempproxy_requests_total{node, result}` - counter
- `pulse_tempproxy_request_duration_seconds{node}` - histogram
- `pulse_tempproxy_circuit_state{node}` - gauge (0=closed, 1=open, 2=half-open)

**Target**: v4.25.0 (4-6 weeks post-launch)

---

## Testing Plan

### Security Testing
- [ ] Pen test: Try SSH injection from container
- [ ] Pen test: Try calling privileged methods from container
- [ ] Verify method-level authorization logs properly
- [ ] Test with multiple simultaneous containers

### Resilience Testing
- [ ] Network partition between container and proxy
- [ ] Proxy crash/restart scenarios
- [ ] Sensor command timeouts (e.g., `sensors` hangs)
- [ ] High request volume (stress test)

---

## Documentation Improvements

### For Operators
- [ ] Document proxy security model in main README
- [ ] Add "Security Architecture" section to docs
- [ ] Explain what data proxy has access to
- [ ] Document how to audit proxy logs
- [ ] Explain bind mount security implications

### For Developers
- [ ] Add security considerations to API docs
- [ ] Document RPC authorization model
- [ ] Add client retry logic examples
- [ ] Create troubleshooting guide for proxy issues

---

## Open Questions

1. **Token Distribution**: If we implement per-client tokens, where should tokens be stored?
   - Environment variables?
   - Systemd service override files?
   - Dedicated secrets directory?

2. **Audit Retention**: How long should we retain proxy audit logs?
   - systemd journal rotation?
   - Separate log file with rotation policy?
   - Forward to central logging?

3. **Monitoring**: What alerts do operators need?
   - Proxy service down?
   - High failure rate?
   - Unauthorized access attempts?
   - Circuit breaker open?

4. **Backwards Compatibility**: How do we roll out these changes?
   - Feature flags during transition?
   - Parallel deployment of old and new?
   - Hard cut-over with upgrade script?

---

## References

- Security Audit Discussion: [Session 0199fd12]
- SSH Injection Fix: commit 124ab7826
- Method Authorization Fix: commit d55112ac4
- Installer Improvements: commits f9c0927c1, bc2f643b0

---

## Contact

For security concerns, contact:
- File issue: https://github.com/rcourtman/Pulse/issues
- Security email: [TBD]
- Private disclosure: [TBD]

**Last Updated**: 2025-10-19
