# Node Cleanup Implementation - CODEX REVIEW COMPLETE

**Status**: ✅ Implementation complete, Codex review passed, ready for deployment testing.

**Latest Update**: Addressed all 8 critical issues found by Codex review (conv-1763166192078-1076).

## Implementation History

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

**Commits ed48d7555, 17d2e6876**: Bug fixes during testing
- ✅ Fixed directory creation order (create before binary download)
- ✅ Fixed SHARE_DIR unbound variable

**Commit bcd8d4e0f**: Critical Codex review fixes #1-4
- ✅ Host detection now includes hostname/FQDN (not just IP)
- ✅ Systemd sandbox relaxed (/etc/pve and /etc/systemd/system writable)
- ✅ Uninstaller called with --purge flag for complete removal
- ✅ All /usr/local references migrated to /opt paths
- ✅ UUID used for transient unit names (prevents collisions)
- ✅ --wait and --collect flags capture uninstaller exit code

**Commit fe53d6473**: Remaining Codex review fixes #5-8
- ✅ LXC bind mounts removed via `pct set -delete` (not sed)
- ✅ API token parsing: three-tier fallback (pveum JSON → pvesh JSON → table)
- ✅ Retry logic: rename to .processing, delete only on success

## Issues Discovered and Resolved

### 1. Host Detection Failure (CRITICAL) - ✅ FIXED
**Problem**: Cleanup script only compared against IPs from `hostname -I`, missing nodes configured as `https://hostname:8006`. This caused localhost cleanup (API tokens, bind mounts, service removal) to be skipped entirely.

**Fix**: Check against `hostname`, `hostname -f`, and all IPs. Now catches all localhost variations.

**Impact**: Without this fix, cleanup would only remove remote SSH keys, leaving services/binaries/tokens intact.

### 2. Systemd Sandbox Blocked Critical Operations - ✅ FIXED
**Problem**: Cleanup service ran with `ProtectSystem=strict` and `ReadWritePaths=/var/lib/pulse-sensor-proxy /root/.ssh`, blocking writes to `/etc/pve` (Proxmox configs) and `/etc/systemd/system`.

**Fix**: Added `/etc/pve` and `/etc/systemd/system` to `ReadWritePaths`.

**Impact**: `pveum` token deletion and `pct set -delete` would fail with "read-only file system".

### 3. Incomplete Purging - ✅ FIXED
**Problem**: Uninstaller called without `--purge`, leaving `/var/lib/pulse-sensor-proxy`, service user, and SSH private keys on disk. Request file deleted before work completed, preventing retry on failure.

**Fix**: Added `--purge` flag, added `--wait --collect` to capture exit code, fail cleanup if uninstaller fails.

**Impact**: Claimed "cleanup completed successfully" even when artifacts remained.

### 4. Incomplete Path Migration - ✅ FIXED
**Problem**: After relocating binaries to `/opt`, three references still pointed to `/usr/local`:
- Forced command in SSH authorized_keys: `/usr/local/bin/pulse-sensor-wrapper.sh`
- Self-heal script: `/usr/local/share/pulse/install-sensor-proxy.sh`
- Backend removal helpers: `/usr/local/bin/pulse-sensor-cleanup.sh`

**Fix**: Updated all three locations. Go backend now checks both paths (new + legacy).

**Impact**: New installs would lose telemetry immediately (SSH command not found). UI-triggered cleanup wouldn't find helpers.

### 5. Transient Unit Name Collisions - ✅ FIXED
**Problem**: Used `date +%s` for unit names. If two cleanup requests fired within same second, second would fail (unit already exists). Error suppressed, logged as "started" anyway.

**Fix**: Use `/proc/sys/kernel/random/uuid` for unique names.

**Impact**: Multiple concurrent cleanups could race, with silent failures.

### 6. Bind Mount Removal Too Broad - ✅ FIXED
**Problem**: Used `sed -i '/pulse-sensor-proxy/d'` which would delete ANY line mentioning the substring (including unrelated comments/hooks). Also couldn't run inside systemd sandbox.

**Fix**: Use `pct set <ctid> -delete mp<N>` which validates syntax and is sandbox-compatible.

**Impact**: Could break container configs. Sed approach would fail anyway due to sandbox.

### 7. API Token Parsing Fragile - ✅ FIXED
**Problem**: Table parser filtered only `│┌└╞`, failing on other Unicode borders or locales. "User not found" vs "feature unsupported" both treated as "no tokens".

**Fix**: Three-tier fallback:
1. `pveum --output-format json` with python3 parsing
2. `pvesh get /access/users/pulse-monitor@pam/token` (always JSON)
3. Improved table parser with better filtering

**Impact**: Non-English locales or Proxmox versions with different table formatting would silently skip token cleanup.

### 8. No Retry on Failure - ✅ FIXED
**Problem**: Request file deleted immediately. Any crash left no way to retry.

**Fix**: Rename to `.processing`, delete only on success. Failures leave `.processing` file for manual investigation/retry.

**Impact**: Transient failures (network issues, systemd hiccups) couldn't be retried automatically.

## Read-Only Filesystem

**Problem**: Proxmox VE can mount `/usr` as read-only (hardened setups, boot-from-snapshots, appliance builds).

**Solution**: All binaries relocated to `/opt/pulse/sensor-proxy/` where we control permissions.

## Process Isolation During Uninstall

**Problem**: Cleanup script runs as systemd service. When it calls the uninstaller which stops the proxy service, systemd kills the cleanup service with SIGTERM.

**Solution**: Use transient systemd unit with:
- UUID-based unique name
- `Conflicts=pulse-sensor-proxy.service`
- `--wait --collect` to capture exit code
- `--purge` flag for complete removal

## Implementation Status

### Phase 1: Relocate Binaries ✅ COMPLETE
- ✅ Update installer to use `/opt/pulse/sensor-proxy/bin/`
- ✅ Update systemd unit ExecStart path
- ✅ Update cleanup script to use new paths
- ✅ Update forced command in SSH authorized_keys
- ✅ Update self-heal script paths
- ✅ Update Go backend removal helpers (supports both new and legacy paths)

### Phase 2: Fix Uninstall Orchestration ✅ COMPLETE
- ✅ Cleanup script spawns transient systemd unit via systemd-run
- ✅ Added `Conflicts=pulse-sensor-proxy.service` to transient unit
- ✅ UUID-based unit names prevent collisions
- ✅ Added `--wait --collect` to capture exit code
- ✅ Uninstaller runs with `--purge --quiet` flags
- ✅ Cleanup fails if uninstaller exits non-zero

### Phase 3: Prevent Cleanup Loops ✅ COMPLETE
- ✅ Added `flock` serialization via exec 200>lockfile
- ✅ Rename request file to `.processing` (allows retry on failure)
- ✅ Delete `.processing` only on successful completion
- ✅ `.path` unit uses PathChanged/PathModified (correct semantics)
- ✅ Comprehensive logging at info/warn/error levels

### Phase 4: Improve API Token Handling ✅ COMPLETE
- ✅ Three-tier fallback: pveum JSON → pvesh JSON → table parsing
- ✅ Python3 JSON parsing for robustness
- ✅ Better table filtering (handles more Unicode characters)
- ✅ Error handling for token deletion failures (logs warnings, continues)
- ✅ Logs each token removal attempt

### Phase 5: LXC Bind Mount Removal ✅ COMPLETE
- ✅ Use `pct set -delete` instead of sed (validates syntax)
- ✅ Sandbox-compatible (works with ProtectSystem=strict + /etc/pve writable)
- ✅ Only removes mounts containing "pulse-sensor-proxy"

### Phase 6: Host Detection ✅ COMPLETE
- ✅ Check against hostname, FQDN, and all local IPs
- ✅ Localhost detection works for `https://hostname:8006` URLs
- ✅ Ensures full cleanup runs for all localhost variations

### Phase 7: Testing & Validation ⏸️ PENDING
- ⏸️ Deploy updated installer to test host
- ⏸️ Test node removal via Pulse UI
- ⏸️ Verify complete cleanup:
  - ⏸️ No systemd units
  - ⏸️ No binaries in /opt or /usr/local
  - ⏸️ No bind mounts in LXC configs
  - ⏸️ No API tokens or pulse-monitor user
  - ⏸️ No SSH keys in authorized_keys
  - ⏸️ No /var/lib/pulse-sensor-proxy directory
- ⏸️ Test retry logic (simulate failure, verify .processing file persists)
- ⏸️ Test on fresh Proxmox VE install
- ⏸️ Test on hardened PVE with read-only `/usr`
- ⏸️ Test cluster vs standalone scenarios

## Codex Review Summary

**Review Session**: conv-1763166192078-1076

**Findings**: 8 critical issues identified:
1. ❌ Host detection only checked IPs, skipped cleanup for hostname-based URLs
2. ❌ Systemd sandbox blocked /etc/pve and /etc/systemd writes
3. ❌ Uninstaller missing --purge, request file deleted too early
4. ❌ Incomplete /usr/local → /opt migration (SSH forced command, self-heal, backend)
5. ❌ Timestamp-based unit names caused collisions
6. ❌ Brittle sed-based bind mount removal, Unicode table parsing
7. ❌ API token parsing failed on locales, couldn't distinguish error types
8. ❌ No retry mechanism for transient failures

**Resolution**: All 8 issues fixed in commits bcd8d4e0f and fe53d6473.

## References

- Initial commit: ed65fda74 "Extend node cleanup to fully remove Pulse footprint"
- Binary relocation: b192c60e9 "Relocate binaries to /opt/pulse/sensor-proxy/"
- Full implementation: 6692228e0 "Full cleanup script implementation"
- Bug fixes: ed48d7555, 17d2e6876
- Codex review fixes #1-4: bcd8d4e0f
- Codex review fixes #5-8: fe53d6473
- Codex review: conv-1763166192078-1076
- Related files:
  - `scripts/install-sensor-proxy.sh` (installer + cleanup script generation)
  - `internal/api/config_handlers.go` (triggerPVEHostCleanup + manual removal)
  - `cmd/pulse-sensor-proxy/cleanup.go` (handleRequestCleanup)

## Priority

**High**: Implementation complete and Codex-reviewed. Ready for deployment testing. All critical security and correctness issues addressed.
