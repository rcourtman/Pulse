# Feature Idea: PBS Datastore Verification/GC Schedule

**Goal:** Display the configured schedules for maintenance jobs (Verification, Garbage Collection) on Proxmox Backup Server (PBS) datastores.

**Potential Data:**

*   Garbage collection schedule (e.g., "daily at 02:00").
*   Verification schedule.
*   Pruning schedule associated with the datastore.

**Relevant API Endpoints:**

*   `/config/datastore`: Lists configured datastores.
*   `/config/datastore/{store}`: Gets the detailed configuration for a specific datastore (`{store}`), which includes schedule properties for `gc-schedule` and `verify-schedule`.

**Implementation Notes:**

*   Requires fetching the configuration for each datastore listed by `/config/datastore`.
*   The schedule format returned by the API needs to be parsed into a human-readable string.
*   Could be displayed alongside the datastore usage information already fetched.
*   Provides useful context about the maintenance routines configured on the PBS. 