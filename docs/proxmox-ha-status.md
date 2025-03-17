# Proxmox HA Status and Primary Node Indication

This document describes how the Proxmox VE API indicates whether a guest VM or container is "primary" or not through its cluster/resources endpoint.

## Overview

In Proxmox VE clusters, virtual machines and containers can be managed by the High Availability (HA) system. When querying the API to determine the status of resources across a cluster, understanding how to interpret the response is critical for proper cluster management.

## The `/cluster/resources` Endpoint

The `/cluster/resources` endpoint is a key API endpoint that provides information about all resources in a Proxmox cluster, including VMs, containers, storage, and nodes.

### Example Response

```json
{
  "data": [
    {
      "cpu": 0.01,
      "disk": 0,
      "diskread": 0,
      "diskwrite": 0,
      "hastate": "started",  // HA state indicator
      "id": "qemu/100",
      "maxcpu": 2,
      "maxdisk": 32212254720,
      "maxmem": 2147483648,
      "mem": 315621376,
      "name": "VM-100",
      "netin": 524,
      "netout": 4112,
      "node": "node1",       // Which node is hosting the resource
      "status": "running",   // Current operational status
      "template": 0,
      "type": "qemu",
      "uptime": 3612,
      "vmid": 100
    }
  ]
}
```

## HA States and Primary Node Status

In Proxmox terminology, the concept of a "primary" node for a VM relates to which node is actively hosting and running that VM. This is determined by a combination of fields in the API response:

### Critical Fields:

1. **`hastate`**: Indicates the High Availability state of the resource
2. **`node`**: Indicates which node is currently hosting the resource
3. **`status`**: Indicates the operational status of the resource

### Possible `hastate` Values:

- **`started`**: The resource is running and managed by HA
- **`stopped`**: The resource is stopped but still managed by HA for failover
- **`disabled`**: The resource is not currently managed by HA for failover
- **`ignored`**: The resource is explicitly removed from HA management
- **`error`**: The resource is in an error state
- **`migrate`**: The resource is being migrated between nodes
- **`relocate`**: The resource is being relocated (cold migration)
- **`fence`**: The node needs to be fenced to recover the resource
- **`recovery`**: The resource is in the recovery process

### Determining Primary Status

To determine if a VM is "primary" on a specific node, you would check:

1. The VM's `node` field shows the node you're interested in
2. The VM's `hastate` field is `started` (or present, depending on HA configuration)
3. The VM's `status` field is `running`

A VM would be considered "primary" when it is actively running on the specified node, as indicated by these fields.

## Additional HA-Related Endpoints

Proxmox provides additional endpoints specifically for HA management:

### 1. `/cluster/ha/resources`

This endpoint returns information about resources managed by the HA system:

```json
{
  "data": [
    {
      "sid": "vm:100",
      "type": "vm",
      "status": "started",
      "state": "started",
      "digest": "abc123...",
      "max_restart": 1,
      "max_relocate": 1,
      "group": "default"
    }
  ]
}
```

### 2. `/cluster/ha/status`

This endpoint returns the current status of the HA cluster:

```json
{
  "data": [
    {
      "type": "cluster",
      "name": "Proxmox HA Cluster",
      "quorate": true,
      "enabled": true
    },
    {
      "id": "node1",
      "name": "node1",
      "type": "node",
      "status": "online",
      "quorum": true
    },
    {
      "state": "started",
      "sid": "vm:100",
      "node": "node1",
      "crm_state": "started",
      "request_state": "started"
    }
  ]
}
```

## Proxmox HA Implementation Details

Proxmox VE implements high availability through the `ha-manager`, which consists of two main components:

1. **Cluster Resource Manager (CRM)**: Makes cluster-wide decisions about resource placement
2. **Local Resource Manager (LRM)**: Controls services on each individual node

When a VM is configured for HA:

- It appears in the `/etc/pve/ha/resources.cfg` file
- The HA system monitors its status and handles failovers
- It's assigned an HA state that is exposed through the API via the `hastate` field

During VM migrations in an HA-managed cluster:

1. The VM's `hastate` changes to `migrate` on the source node
2. After migration, the source node marks the VM as `stopped`
3. The destination node changes the VM's `hastate` to `started` once migration completes
4. This ensures only one node runs the VM at a time, preventing split-brain situations

## Mock Implementation

Our mock implementation now accurately simulates Proxmox's HA behavior:

1. **HA State Tracking**: Each VM has a `hastate` field that corresponds to its HA management status
2. **Migration Simulation**: When VMs migrate between nodes, their `hastate` is updated through realistic state transitions
3. **Shared Guests**: VMs that exist across multiple nodes (shared guests) maintain consistent `hastate` values:
   - Only the primary node shows the guest with `hastate: "started"`
   - Other nodes show it with `hastate: "stopped"`
4. **API Consistency**: Multiple endpoints are supported to match a real Proxmox environment:
   - `/cluster/resources` - Shows all resources with their HA states
   - `/cluster/ha/resources` - Shows just the HA-managed resources
   - `/cluster/ha/status` - Shows detailed HA status
5. **Realistic Migrations**: The migration process shows the actual state transitions a VM would go through in a real environment
6. **Error States**: Occasional error states are generated to test application resilience

This implementation ensures that your application can properly handle HA status information whether it's connected to a real Proxmox cluster or using the mock data for development.

## Implementation Considerations

When implementing cluster management tools that interact with Proxmox:

1. Always check the `hastate` field to understand the HA management status
2. Don't rely solely on the `status` field, as a VM might be in a transitional state
3. Consider that VMs may migrate between nodes in an HA cluster
4. HA-managed resources will follow the configured HA policies
5. Handle all possible `hastate` values, including error states and transitional states like `migrate`

## Caveats

- HA status may change during cluster maintenance or failures
- The API response might reflect a transitional state during migrations
- Not all VMs/containers will have an `hastate` if they're not managed by HA
- Different Proxmox versions might have slight variations in the `hastate` values and behaviors

## References

- [Proxmox VE API Documentation](https://pve.proxmox.com/pve-docs/api-viewer/index.html)
- [Proxmox HA Manager Documentation](https://pve.proxmox.com/pve-docs/ha-manager.1.html)
- [Proxmox HA Resources Configuration](https://pve.proxmox.com/wiki/High_Availability) 