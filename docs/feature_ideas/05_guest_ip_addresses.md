# Feature Idea: Guest IP Addresses (PVE)

**Goal:** Display the IP addresses assigned *within* the guest operating system for VMs.

**Potential Data:**

*   IPv4 addresses.
*   IPv6 addresses.
*   Associated MAC address (to link IP to a NIC).

**Relevant API Endpoints:**

*   `/nodes/{node}/qemu/{vmid}/agent/get-network-ifs`: Retrieves network interface details, including IP addresses, directly from the guest OS via the QEMU Guest Agent.

**Implementation Notes:**

*   **Crucially depends on the QEMU Guest Agent being installed and running within the VM.** If the agent is not present or running, the API call will fail or return no data.
*   Requires an additional API call per running VM.
*   Need robust error handling for when the agent is unavailable.
*   UI should clearly indicate if IPs are missing due to the agent not running.
*   Consider how to display multiple IPs/interfaces (e.g., tooltip, comma-separated list in a column).
*   This is likely **high-value** information for users. 