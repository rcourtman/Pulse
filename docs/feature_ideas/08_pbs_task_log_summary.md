# Feature Idea: PBS Task Log Summary

**Goal:** Provide a quick view of the end of the task log, especially for failed Proxmox Backup Server (PBS) tasks.

**Potential Data:**

*   Display the last N lines (e.g., 5-10) of a task log.
*   Show the exit status or error message from the log.

**Relevant API Endpoints:**

*   `/nodes/{node}/tasks/{upid}/log`: Fetches the log content for a specific task UPID.
*   `/nodes/{node}/tasks/{upid}/status`: Fetches the final status of a completed task.
*   `/nodes/{node}/tasks`: Currently used to list tasks.

**Implementation Notes:**

*   Requires an additional API call per task for which logs are needed (likely only failed or recent tasks).
*   The `/log` endpoint returns the full log; need to process the response to extract only the last few lines (or use API parameters if available to limit the output, e.g., `?start=X&limit=Y`). Check API docs for log limiting parameters.
*   Could be displayed in a modal when clicking on a task, or inline if kept very brief.
*   Helps quickly diagnose failures without needing to open the PBS UI. 