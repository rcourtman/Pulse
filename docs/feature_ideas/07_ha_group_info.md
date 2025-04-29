# Feature Idea: High Availability (HA) Group Information (PVE)

**Goal:** Display more detailed information about the High Availability configuration and status.

**Potential Data:**

*   List of HA groups configured.
*   Members (VMs/CTs) within each HA group.
*   Current state of HA services/resources (e.g., master node, resource states).
*   HA fencing status/configuration.

**Relevant API Endpoints:**

*   `/cluster/ha/resources`: Currently used, lists resources managed by HA.
*   `/cluster/ha/groups`: Lists the configured HA groups and their settings (nodes, priorities).
*   `/cluster/ha/status/current`: Provides the current status of the HA manager, including master node and resource states.
*   `/cluster/ha/status/manager_status`: Detailed status of the HA manager.

**Implementation Notes:**

*   Requires additional API calls to `/groups` and `/status/current`.
*   Could be displayed in a dedicated HA status section or integrated subtly into the existing views (e.g., indicating which guests are HA managed).
*   Useful for users relying on Proxmox HA features. 