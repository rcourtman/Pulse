# Pulse Sensor Proxy – Phase 1 Summary

## Executive Summary
Phase 1 delivered a complete hardening and observability overhaul for the Pulse sensor proxy. The service now runs under least privilege, exposes tamper-evident audit trails, forwards logs off-host, enforces adaptive rate caps, and ships with comprehensive validation tests plus documentation for ongoing operations and security posture. These improvements dramatically reduce the proxy's attack surface while giving operators clear visibility and controls.

## Security Improvements
- **Host hardening**
  - SSH daemon locked down (no passwords, no forwarding, `ForceCommand` wrapper).
  - Dedicated user `pulse-sensor` with minimal home/project directories.
  - File permissions tightened (0750 binaries, 0600 private keys, 0640 append-only logs).
  - Privilege drop via `Setuid/Setgid` post-bind; service confirms running as unprivileged UID 995.
- **Command execution guardrails**
  - Whitelist-based command validator for `sensors`/`ipmitool`; rejects shell metacharacters, subshells, dangerous ipmitool subcommands, null bytes, and path traversal.
  - Enhanced node-name validation covering unicode, length, and absolute path abuse.
- **Logging & audit**
  - Structured audit logger with hash chain + HMAC-style tamper detection.
  - Remote forwarding via `rsyslog` (RELP/TLS) and local queue for resilience.
- **Sandboxing**
  - Network segmentation documentation with firewall ACLs.
  - AppArmor profile restricting filesystem/networking; seccomp profile (classic + OCI JSON).
- **Rate limiting**
  - Per-UID token bucket (0.2 QPS burst 2) + global concurrency cap (8) + penalty sleeps.
  - Audit + metrics instrumentation for limiter decisions and penalties.

## Key Metrics & Tests
- `go test ./cmd/pulse-sensor-proxy/...` passes (including new unit suites for command validation, sanitizer, limiter penalties, audit logging).
- 10 hostile-command attack cases covered (metacharacters, subshells, redirects, homoglyphs, null bytes, long args, path traversal, dangerous ipmitool ops, env prefixes, absolute paths).
- Fuzz harness (`FuzzValidateCommand`) executing for 24 h (Task #24).
- Prometheus metrics validated:
  - `pulse_proxy_limiter_rejections_total{reason="rate"}` increments under load.
  - `pulse_proxy_limiter_penalties_total{reason="invalid_json"}` increments on validation failure.
  - `pulse_proxy_limiter_active_peers` accurate (UID grouping).
- Audit log entries verified: connection acceptance, limiter rejections, validation failures, command start/finish with event hash chaining.

## Deployment Checklist
1. **Scripts**
   - Run `scripts/create-sensor-user.sh`
   - Run `scripts/harden-sensor-proxy.sh`
   - Run `scripts/secure-sensor-files.sh`
   - Run `scripts/setup-log-forwarding.sh`
2. **Binaries**
   - Build/install `/opt/pulse/sensor-proxy/bin/pulse-sensor-proxy` (0750, root:pulse-sensor).
3. **Configuration**
   - `/etc/pulse-sensor-proxy/config.yaml` updated (allowed subnets, UID/GID list, metrics addr).
   - Systemd unit exports `PULSE_SENSOR_PROXY_USER`, `PULSE_SENSOR_PROXY_SSH_DIR`, `PULSE_SENSOR_PROXY_AUDIT_LOG`.
4. **Profiles**
   - Deploy AppArmor profile (`security/apparmor/pulse-sensor-proxy.apparmor`).
   - Apply seccomp (systemd `SystemCallFilter` overrides or container JSON profile).
5. **Networking**
   - Implement firewall ACLs per `docs/security/pulse-sensor-proxy-network.md`.
6. **Log Forwarding**
   - Place TLS certs in `/etc/pulse/log-forwarding`.
   - Verify rsyslog forwarding to remote collector.
7. **Restart & Validate**
   - `systemctl restart pulse-sensor-proxy`.
   - Confirm metrics endpoint, audit log creation, limiter behaviour.

## Verification Steps
1. **Privilege Drop**: `ps -o user= -p $(pgrep -f pulse-sensor-proxy)` → `pulse-sensor`.
2. **Audit Trail**: Trigger RPC (`get_status`) → verify `audit.log` entries with valid `event_hash`.
3. **Rate Limiter**: Fire >10 concurrent requests → confirm `pulse_proxy_limiter_rejections_total{reason="rate"}` and audit `limiter.rejection`.
4. **Remote Logging**: `logger` or manual append to proxy log → confirm arrival at remote collector.
5. **Security Profiles**: `aa-status | grep pulse-sensor-proxy` (enforced), `systemctl show pulse-sensor-proxy -p SystemCallFilter`.
6. **App Functionality**: Run `ensure_cluster_keys`, `get_temperature` RPCs, ensure success and no audit warnings.

## Known Limitations / Deferred to Phase 2
- **Adaptive Polling**: still fixed intervals (Phase 2 focuses on controller, backpressure, staleness SLOs).
- **Queue Backpressure**: groundwork in rate limiter; full queue-based collector scheduling to be built next.
- **External Sentinels**: cross-check monitoring and metric ingestion planned in Phase 3.
- **AppArmor/Seccomp Tuning**: profiles may need refinement after real-world observation.
- **Long-run Fuzz Results**: Task #24 fuzz campaign active; incorporate findings post-run.
