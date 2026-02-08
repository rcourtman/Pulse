# Changelog Draft - Delete After Release

## PVE Backup Visibility Fix (Issue #1139)

### Fixed
- **PVE Backups not showing (agent setup)**: Fixed an issue where local PVE backups weren't visible when nodes were set up via the unified agent (`--enable-proxmox`). The agent now grants the required `PVEDatastoreAdmin` permission automatically.

### Improved
- **Backup permission warnings**: The Backups page now detects and displays a warning banner when backup permission issues are detected, with instructions on how to fix them. Users no longer have to guess why backups aren't appearing.

### Action Required for Existing Users
If you set up PVE nodes via the unified agent before this release, backups may not appear. The Backups page will now show a warning banner with the fix command, but you can also run this manually on each Proxmox host:

```bash
pveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin
```

Alternatively, delete the node in Pulse Settings and re-run the agent setup - it will now grant the correct permissions.

---

**Commits:**
- 316a5629 fix(agent): grant PVEDatastoreAdmin for backup visibility
- 3237a4d7 docs: clarify PVE backup permission requirements
- 1733bea1 feat(ui): show backup permission warnings on Backups page
- 896b5bfc Fix: enable backup monitoring for PVE instances via config migration (already in main)

---

## Program Closeout Tracks

- Alerts: unified resource model hardening complete.
- Control Plane: router decomposition and contract hardening complete.
- Settings: control plane decomposition complete.
- Multi-Tenant: productization complete; final certification pending import cycle resolution.
- Storage + Backups V2: deferred to next milestone.
