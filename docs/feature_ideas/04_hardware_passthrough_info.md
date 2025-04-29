# Feature Idea: Hardware/PCI Passthrough Info (PVE)

**Goal:** Display information about hardware devices (PCI, USB) passed through to specific VMs.

**Potential Data:**

*   List of PCI devices passed through (Vendor/Device ID, Name).
*   List of USB devices passed through.

**Relevant API Endpoints:**

*   `/nodes/{node}/qemu/{vmid}/config`: Contains configuration details for VMs, including `hostpci` entries for PCI passthrough and `usb` entries for USB passthrough.
*   `/nodes/{node}/hardware/pci`: Lists available PCI devices on the host.
*   `/nodes/{node}/hardware/usb`: Lists available USB devices on the host.

**Implementation Notes:**

*   Requires parsing the VM's `config` endpoint.
*   This information is static configuration.
*   Likely only relevant for a subset of users.
*   Display could be in a details view or a modal, as it's not primary monitoring data. 