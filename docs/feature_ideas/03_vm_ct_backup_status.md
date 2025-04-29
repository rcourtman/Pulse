# Feature Idea: VM/CT Backup Status (PVE)

**Goal:** Show the status or timestamp of the last backup taken *by Proxmox VE itself* (snapshot backups, not necessarily PBS) for each VM and Container.

**Potential Data:**

*   Timestamp of the last successful PVE snapshot/backup.
*   Name/description of the last snapshot.
*   Indication if a backup task is currently running for the guest.

**Relevant API Endpoints:**

*   `/nodes/{node}/{type}/{vmid}/snapshot`: Lists all snapshots for the specific VM or Container. This includes backups created via the PVE GUI/API. Need to parse the output to find the most recent one, potentially filtering by name convention if backups have specific names.
*   `/nodes/{node}/tasks`: Could potentially show running backup tasks related to a specific VMID, but filtering might be complex.

**Implementation Notes:**

*   Requires an additional API call per VM/CT (`/snapshot`). Consider performance implications if there are many guests.
*   Parsing the snapshot list is needed to identify the most recent backup.
*   PBS backups are handled separately; this focuses only on backups visible directly within PVE for the guest.
*   Display could be a dedicated column ("Last PVE Backup") or integrated into a status indicator. 