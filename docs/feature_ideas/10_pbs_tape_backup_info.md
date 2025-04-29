# Feature Idea: PBS Tape Backup Information

**Goal:** Display status and information related to tape backup operations within Proxmox Backup Server (PBS), if used.

**Potential Data:**

*   List of configured tape drives and their status.
*   Status of ongoing or recent tape backup/restore jobs.
*   Information about media pools and tapes.

**Relevant API Endpoints:**

*   `/nodes/{node}/tape/drive`: Lists tape drives and their status.
*   `/nodes/{node}/tape/job`: Lists tape backup/restore jobs.
*   `/nodes/{node}/tape/pool`: Lists media pools.
*   `/nodes/{node}/tape/media`: Lists tapes (media).
*   Various endpoints under these for specific details (e.g., job logs, drive status).

**Implementation Notes:**

*   This is a niche feature, only relevant if the user has configured tape backups in PBS.
*   Requires multiple additional API calls.
*   Consider adding a dedicated section or view for Tape Backup status, only visible if tape drives are detected or configured.
*   `Audit` role might be sufficient, but needs verification. 