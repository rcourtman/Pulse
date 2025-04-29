# Feature Idea: Detailed Disk Information (PVE)

**Goal:** Display more granular disk information for VMs and Containers beyond just the current total I/O.

**Potential Data:**

*   Individual virtual disk usage/size.
*   Disk throughput per virtual disk (if available separately from total I/O).
*   Physical disk S.M.A.R.T. health status for the underlying host nodes.

**Relevant API Endpoints:**

*   `/nodes/{node}/{type}/{vmid}/config`: Contains configuration details for virtual disks attached to a VM/CT (size, storage, format).
*   `/nodes/{node}/disks/list`: Lists physical disks on the node.
*   `/nodes/{node}/disks/smart?disk={diskname}`: Retrieves S.M.A.R.T. data for a specific physical disk on the node (requires `Disk.Audit` or higher permissions).
*   `/nodes/{node}/{type}/{vmid}/status/current`: Already used, provides aggregate disk I/O (`diskread`, `diskwrite`). Check if this contains per-disk details already.

**Implementation Notes:**

*   Requires parsing the `config` endpoint to identify virtual disks.
*   Fetching S.M.A.R.T. data requires an additional API call per physical disk and potentially higher permissions than `PVEAuditor`.
*   Consider how to display this: added columns, a modal/popup, or a separate "details" view. 