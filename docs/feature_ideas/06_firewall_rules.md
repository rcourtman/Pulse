# Feature Idea: Firewall Rules (PVE)

**Goal:** Provide visibility into the Proxmox VE firewall rules.

**Potential Data:**

*   Display firewall rules configured at the Datacenter level.
*   Display firewall rules configured at the Node level.
*   Display firewall rules configured per VM/CT.

**Relevant API Endpoints:**

*   `/cluster/firewall/rules`: Gets firewall rules defined at the datacenter level.
*   `/nodes/{node}/firewall/rules`: Gets firewall rules defined for a specific node.
*   `/nodes/{node}/{type}/{vmid}/firewall/rules`: Gets firewall rules defined for a specific VM or Container.
*   Other related endpoints under `/cluster/firewall/`, `/nodes/{node}/firewall/`, `/nodes/{node}/{type}/{vmid}/firewall/` for options, groups, aliases etc.

**Implementation Notes:**

*   Firewall rules can be numerous and complex.
*   Displaying this effectively in the main dashboard is likely impractical.
*   Might be better suited for a dedicated "Firewall" view or section within the application.
*   Requires careful consideration of which rules (datacenter, node, guest) are most relevant to display and how.
*   `PVEAuditor` role should be sufficient for reading rules. 