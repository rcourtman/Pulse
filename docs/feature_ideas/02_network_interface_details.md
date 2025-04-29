# Feature Idea: Network Interface Details (PVE)

**Goal:** Display configuration details for network interfaces attached to VMs and Containers.

**Potential Data:**

*   MAC Address per interface.
*   Connected Bridge/Network.
*   VLAN Tag (if applicable).
*   Model/Type of virtual NIC.
*   Potentially live throughput per interface (if API provides this separately from total).

**Relevant API Endpoints:**

*   `/nodes/{node}/{type}/{vmid}/config`: Contains configuration details for network interfaces (e.g., `net0`, `net1`) including MAC, bridge, tag, model.
*   `/nodes/{node}/{type}/{vmid}/status/current`: Already used, provides aggregate network I/O (`netin`, `netout`). Check if this contains per-interface details already.

**Implementation Notes:**

*   Requires parsing the `config` endpoint to identify and extract details for each network interface (e.g., `net0`, `net1`, ...).
*   Displaying live per-interface stats might not be feasible or may already be aggregated in the `/status/current` data.
*   Focus on displaying the static configuration first (MAC, bridge, VLAN).
*   Consider how to display: added columns (might get wide), a modal/popup, or a details view. 