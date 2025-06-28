# PBS Node Information Extraction Report

## Executive Summary

We investigated whether Proxmox Backup Server (PBS) can reliably identify which node a backup originated from when multiple nodes have VMs/CTs with identical VMIDs. Our findings show that **node information is reliably available for all container (CT) backups and diskless VM backups, but NOT for VM backups that include disk data**. Additionally, we discovered that **without proper namespace configuration, VMID collisions result in silent data loss** as backups overwrite each other.

## Background

When multiple Proxmox VE nodes back up to the same PBS instance, VMID collisions can occur (e.g., both node1 and node2 have a VM with ID 100). PBS identifies backups using the structure `<datastore>/<type>/<id>/<timestamp>/`, which does not include node information. **Without namespaces, this causes newer backups to silently overwrite older ones from different nodes**, making previous backups permanently inaccessible.

## Critical Discovery: Silent Data Loss

Our investigation revealed a critical issue: **PBS silently overwrites backups when VMIDs collide**. When node2 backs up VM 100 to the same datastore where node1 already has VM 100 backups, PBS overwrites the backup group ownership, making all previous snapshots from node1 unreachable through the PBS interface. No warnings or errors are generated during this process.

### Storage Structure Without Namespaces
```
/datastore/
├── vm/
│   ├── 100/  ← All nodes' VM 100 backups collide here
│   └── 101/
└── ct/
    └── 100/
```

## Investigation Methodology

1. Analyzed PBS API endpoints and backup metadata structure
2. Examined backup files including:
   - `client.log.blob` (backup logs)
   - `index.json.blob` (backup manifest)
   - `qemu-server.conf.blob` / `pct.conf.blob` (configuration files)
3. Tested extraction methods across multiple backup types
4. Created automated testing scripts to verify findings
5. Researched official documentation and community reports on VMID collision handling

## Findings

### Node Information Extraction

#### Container (CT) Backups - ✅ Reliable

All container backups contain node information in the `client.log.blob` file:

```
2025-06-27 23:06:17 INFO: Client name: delly
```

**Extraction method:**
```bash
curl -sk -H "Authorization: PBSAPIToken=<token>" \
  "https://<pbs-host>/api2/json/admin/datastore/<store>/download-decoded?\
  backup-type=ct&backup-id=<id>&backup-time=<timestamp>&file-name=client.log.blob" \
  | grep "Client name:"
```

#### Diskless VM Backups - ✅ Reliable

Virtual machine backups that contain no disk data (configuration-only backups) also include the node information:

```
2025-06-28 04:00:50 INFO: backup contains no disks
2025-06-28 04:00:50 INFO: Client name: desktop
```

#### VM Backups with Disks - ❌ Not Reliable

Virtual machine backups that include disk images do NOT contain the "Client name:" field in their logs. The node information is not stored in any standardized location within the backup metadata.

**Why the comment field is not reliable:**
- The backup comment field sometimes contains node information (e.g., "ubuntu-gpu-vm, desktop, 400")
- However, this is user-configurable and depends on how the backup job is configured
- Cannot be relied upon for programmatic extraction

### Test Results

| Backup Type | VMID | Node Extraction | Source |
|------------|------|-----------------|---------|
| CT | 106 | ✅ delly | client.log.blob |
| CT | 112 | ✅ desktop | client.log.blob |
| CT | 120 | ✅ delly | client.log.blob |
| VM (diskless) | 200 | ✅ desktop | client.log.blob |
| VM (with disks) | 400 | ❌ Not available | - |
| VM (with disks) | 102 | ❌ Not available | - |

## VMID Collision Behavior

### Without Namespaces: Data Loss

1. **Silent Overwrite**: When multiple nodes backup VMs with the same VMID, newer backups overwrite the backup group metadata
2. **Inaccessible Backups**: Previous backups become unreachable through PBS interface (though chunk data remains due to deduplication)
3. **Retention Policy Chaos**: The most recent backup source controls retention for the entire backup group, potentially causing unexpected data pruning

### Deduplication Still Works

PBS implements chunk-based deduplication using SHA-256 checksums globally across the datastore. Even with VMID collisions, identical data chunks are stored only once in the shared `.chunks` directory. However, this efficiency is meaningless if backups become inaccessible due to metadata overwrites.

## Official Solution: Namespaces (PBS 2.2+)

Proxmox introduced namespaces in PBS version 2.2 specifically to address this issue. With namespaces, the storage structure becomes:

```
/datastore/
├── ns/
│   ├── node1/
│   │   └── vm/
│   │       └── 100/  ← Node1's VM 100 isolated here
│   ├── node2/
│   │   └── vm/
│   │       └── 100/  ← Node2's VM 100 - no collision
└── .chunks/            ← Shared deduplication across all namespaces
```

This maintains complete backup group separation while preserving deduplication benefits.

## Implications

1. **Critical for Multi-Node**: Without namespaces, multi-node PBS deployments risk silent data loss
2. **Node Tracking Limited**: Even with namespaces, VM backups with disks don't store source node in metadata
3. **Manual Correlation Required**: For VM disaster recovery, external documentation linking VMIDs to nodes is essential

## Recommendations

### Immediate Actions Required

1. **Enable Namespaces** (PBS 2.2+):
   - Create separate namespace per PVE node/cluster
   - Configure namespace in PVE storage settings
   - Prevents all collision-related data loss

2. **For PBS < 2.2**:
   - Use separate datastores per node (loses deduplication)
   - OR enforce strict VMID uniqueness across all nodes

### Best Practices

1. **VMID Range Allocation**:
   - Cluster A: 1000-1999
   - Cluster B: 2000-2999
   - Configure via Datacenter → Options → "Next free VMID range"

2. **Namespace Naming Convention**:
   - Use descriptive names: "pve-prod-node1", "pve-dev-cluster"
   - Document namespace-to-node mappings

3. **For Pulse Integration**:
   - Implement namespace support in PBS connections
   - Extract node info from CT backups using reliable method
   - For VMs, require either:
     - VMID naming conventions
     - Manual node-to-VM mapping configuration
     - Namespace-based organization

4. **Retention Policy Management**:
   - Configure retention at namespace level in PBS
   - Avoid relying on per-backup retention when not using namespaces

## Technical Details

The provided test scripts demonstrate:
- `test-pbs-node-extraction-v2.sh`: Node information extraction methods
- Silent overwrite behavior can be verified by backing up identical VMIDs from different nodes

## Conclusion

PBS has two critical limitations for multi-node environments:

1. **Without namespaces**: VMID collisions cause silent data loss through backup overwrites
2. **Metadata limitations**: VM backups with disks don't store source node information

The namespace feature (PBS 2.2+) is **mandatory** for safe multi-node deployments, not optional. While it solves the collision problem, the lack of node metadata in VM backups still requires external tracking solutions. Organizations must implement both namespaces and VMID management strategies to ensure reliable backup and recovery operations.