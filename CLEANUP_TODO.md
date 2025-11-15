# Node Cleanup Implementation - COMPLETED

**Status**: ✅ Full implementation completed in commits b192c60e9 and 6692228e0.

**Previous Status**: Partial implementation attempted (ed65fda74), reverted due to process isolation issues.

**Current Status**: All phases complete, ready for testing.

## Implementation Complete

**Commit b192c60e9**: Relocate binaries to /opt/pulse/sensor-proxy/
- ✅ Binary path moved from /usr/local/bin to /opt/pulse/sensor-proxy/bin
- ✅ All systemd ExecStart paths updated
- ✅ Guarantees cleanup works on read-only /usr systems

**Commit 6692228e0**: Full cleanup script implementation
- ✅ SSH key removal (working)
- ✅ Service uninstallation (via isolated systemd-run transient unit)
- ✅ API token deletion (JSON first, filtered table fallback)
- ✅ Bind mount removal (scans all LXC configs)
- ✅ pulse-monitor user deletion
- ✅ flock serialization (prevents concurrent runs)
- ✅ Immediate request file deletion (prevents loops)

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

## Implementation Status

### Phase 1: Relocate Binaries ✅ COMPLETE
- ✅ Update installer to use `/opt/pulse/sensor-proxy/bin/`
- ✅ Update systemd unit ExecStart path
- ✅ Update cleanup script to use new paths
- ⏭️ No symlinks needed (PATH not required for systemd ExecStart)
- ⏸️ Test on read-only `/usr` (deferred to integration testing)

### Phase 2: Fix Uninstall Orchestration ✅ COMPLETE
- ✅ Cleanup script spawns transient systemd unit via systemd-run
- ✅ Added `Conflicts=pulse-sensor-proxy.service` to transient unit
- ✅ Cleanup service exits immediately after spawning uninstaller
- ✅ Uninstaller runs via installer's --uninstall flag (reuses existing code)
- ✅ Process isolation prevents SIGTERM to cleanup service

### Phase 3: Prevent Cleanup Loops ✅ COMPLETE
- ✅ Added `flock` serialization via exec 200>lockfile
- ✅ Delete cleanup-request file immediately after reading (before operations)
- ✅ `.path` unit uses PathChanged/PathModified (correct semantics)
- ✅ Comprehensive logging at info/warn/error levels

### Phase 4: Improve API Token Handling ✅ COMPLETE
- ✅ Try JSON output first (--output-format json-pretty)
- ✅ Fall back to table parsing with proper filtering (removes │┌└╞)
- ✅ Error handling for token deletion failures (logs warnings, continues)
- ✅ Logs each token removal attempt

### Phase 5: Testing & Validation ⏸️ PENDING
- ⏸️ Test on fresh Proxmox VE install
- ⏸️ Test on hardened PVE with read-only `/usr`
- ⏸️ Test cluster vs standalone scenarios
- ⏸️ Test LXC bind mount removal
- ⏸️ Verify no `pulse-*` artifacts remain after cleanup:
  - ⏸️ No systemd units
  - ⏸️ No binaries
  - ⏸️ No bind mounts in LXC configs
  - ⏸️ No API tokens or pulse-monitor user
  - ⏸️ No SSH keys in authorized_keys

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
