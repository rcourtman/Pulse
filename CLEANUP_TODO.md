# Node Cleanup Implementation TODO

**Status**: Partial implementation committed (ed65fda74). Requires architectural changes for full functionality.

## Current State

Commit `ed65fda74` extends the cleanup script to perform full uninstallation when a node is removed:
- ✅ SSH key removal (working)
- ⚠️ Service uninstallation (attempted but fails)
- ⚠️ API token deletion (working but has parsing issues)
- ⚠️ Bind mount removal (via uninstaller, not tested)

## Issues Discovered During Testing

### 1. Read-Only Filesystem
**Problem**: Proxmox VE can mount `/usr` as read-only (hardened setups, boot-from-snapshots, appliance builds). The binary at `/usr/local/bin/pulse-sensor-proxy` cannot be removed.

**Error**: `rm: cannot remove '/usr/local/bin/pulse-sensor-proxy': Read-only file system`

**Solution**: Relocate all Pulse artifacts to `/opt/pulse/sensor-proxy/` where we control permissions.

### 2. Process Isolation During Uninstall
**Problem**: Cleanup script runs as systemd service. When it calls the uninstaller which stops the proxy service, systemd kills the cleanup service with SIGTERM.

**Attempted fixes** (all failed):
- `systemd-run --scope`
- `at now` scheduling
- `setsid` with double-fork

**Root cause**: Cleanup service and proxy service share dependency tree.

**Solution**: Use transient systemd unit that's independent:
```bash
systemd-run --unit=pulse-sensor-proxy-uninstall@$(uuidgen) \
    /opt/pulse/scripts/uninstall.sh
```

The transient unit should:
- Not use `--scope`
- Have `Conflicts=pulse-sensor-proxy.service`
- Run in its own cgroup

### 3. Cleanup Loop
**Problem**: Cleanup script ran multiple times because cleanup-request file persisted.

**Solution**:
- Delete request file BEFORE starting long-running work
- Use `flock` for serialization
- Ensure `.path` unit triggers on file creation, not existence

### 4. API Token Parsing
**Problem**: Token list includes table formatting characters (│, ┌, └, ╞)

**Current workaround**: Filter with grep, but brittle.

**Better solution**: Use `pveum user token list --output-format json-pretty pulse-monitor@pam` if available.

## Required Changes

### Phase 1: Relocate Binaries
- [ ] Update installer to use `/opt/pulse/sensor-proxy/bin/`
- [ ] Create convenience symlink in `/usr/local/bin` only if writable
- [ ] Update systemd unit ExecStart path
- [ ] Update cleanup script to remove from `/opt` location
- [ ] Test on both writable and read-only `/usr` systems

### Phase 2: Fix Uninstall Orchestration
- [ ] Create dedicated uninstall script at `/opt/pulse/scripts/sensor-proxy-uninstall.sh`
- [ ] Modify cleanup script to spawn transient systemd unit instead of direct call
- [ ] Add `Conflicts=pulse-sensor-proxy.service` to transient unit
- [ ] Ensure cleanup service exits immediately after spawning uninstaller
- [ ] Test that proxy service can be removed without killing cleanup service

### Phase 3: Prevent Cleanup Loops
- [ ] Add `flock` serialization to cleanup script
- [ ] Delete cleanup-request file at start of script (not end)
- [ ] Review `.path` unit configuration for proper trigger semantics
- [ ] Add logging to track cleanup invocations

### Phase 4: Improve API Token Handling
- [ ] Use JSON output from pveum if available
- [ ] Add error handling for token deletion failures
- [ ] Log which tokens were removed successfully

### Phase 5: Testing & Validation
- [ ] Test on fresh Proxmox VE install
- [ ] Test on hardened PVE with read-only `/usr`
- [ ] Test cluster vs standalone scenarios
- [ ] Test LXC bind mount removal
- [ ] Verify no `pulse-*` artifacts remain after cleanup:
  - [ ] No systemd units
  - [ ] No binaries
  - [ ] No bind mounts in LXC configs
  - [ ] No API tokens or pulse-monitor user
  - [ ] No SSH keys in authorized_keys

## Alternative Approaches Considered

**Option A**: Remove only SSH keys and API tokens, skip service/binary removal
- ❌ Rejected: Leaves privileged services on unmanaged hosts

**Option B**: Make cleanup manual with documented commands
- ❌ Rejected: Shifts security responsibility to users

**Option C**: Run cleanup from Pulse controller via SSH instead of path unit
- ✅ Viable alternative: Controller has no dependency on proxy service
- Consider for future iteration

## References

- Commit: ed65fda74 "Extend node cleanup to fully remove Pulse footprint"
- Codex conversation: conv-1763161052746-956
- Related files:
  - `scripts/pulse-sensor-cleanup.sh`
  - `scripts/install-sensor-proxy.sh`
  - `internal/api/config_handlers.go` (triggerPVEHostCleanup)
  - `cmd/pulse-sensor-proxy/cleanup.go` (handleRequestCleanup)

## Priority

**Medium-High**: Current implementation removes SSH keys (most critical security piece). Full cleanup would be nice-to-have for operational cleanliness and aligns with "remove node = sever trust" principle, but the additional complexity requires careful implementation and testing.
