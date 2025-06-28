# PBS Namespace Grouping Feature

## Overview
Added a new "Backup Summary by Namespace" section to the PBS UI that groups guests by their namespace, providing a clear overview of backup organization across different namespaces.

## Features

### 1. Namespace-Grouped Summary Card
- Displays all namespaces with backup data
- Shows root namespace first, then others alphabetically
- Each namespace card shows:
  - Total number of snapshots
  - Total backup size
  - Number of unique VMs and containers
  - Top 5 most recently backed up guests per type
  - Date range of backups (oldest to newest)

### 2. Visual Organization
- Each namespace gets its own card with subtle background color
- Two-column layout separating VMs and containers
- Shows backup count per guest (e.g., "VM 100: 3 backups")
- Truncates list if more than 5 guests, showing "...and X more"

### 3. Integration
- Appears before the Datastores section in the PBS tab
- Works with existing namespace filtering
- Automatically updates when switching between PBS instances

## Implementation Details

### New Function: `createNamespaceGroupedSummary()`
Located in `/opt/pulse/src/public/js/ui/pbs.js`

The function:
1. Iterates through all datastores and their snapshots
2. Groups snapshots by namespace
3. Tracks unique guests (VMs/CTs) per namespace
4. Calculates totals (size, count, date ranges)
5. Renders a summary card for each namespace

### Data Structure
```javascript
{
  namespace: {
    vms: Map<guestId, { count, size, latest }>,
    cts: Map<guestId, { count, size, latest }>,
    totalSnapshots: number,
    totalSize: number,
    oldestBackup: timestamp,
    newestBackup: timestamp
  }
}
```

## Benefits

1. **Multi-Node Visibility**: Clearly shows which guests are in which namespace
2. **Quick Overview**: See backup distribution without scrolling through individual snapshots
3. **Namespace Adoption**: Encourages proper namespace usage by showing organization
4. **Collision Detection**: Makes it easier to spot when multiple nodes might be using the same namespace

## Example Output

```
Backup Summary by Namespace

┌─ Root Namespace ─────────────────────────── 187 snapshots │
│                                             1.2 TB        │
│ Virtual Machines (3)      Containers (8)                 │
│ VM 400: 12 backups       CT 106: 45 backups            │
│ VM 102: 8 backups        CT 112: 38 backups            │
│ VM 200: 5 backups        CT 120: 22 backups            │
│                          CT 101: 18 backups            │
│                          CT 105: 15 backups            │
│                          ...and 3 more                  │
│ Backups from 1/1/2025 to 6/28/2025                     │
└──────────────────────────────────────────────────────────┘

┌─ Namespace: pimox ──────────────────────── 44 snapshots │
│                                            256 GB       │
│ Virtual Machines (2)      Containers (4)               │
│ VM 501: 8 backups        CT 201: 12 backups          │
│ VM 502: 6 backups        CT 202: 10 backups          │
│                          CT 203: 5 backups           │
│                          CT 204: 3 backups           │
│ Backups from 3/15/2025 to 6/28/2025                  │
└─────────────────────────────────────────────────────────┘
```

This feature enhances the PBS integration by providing better visibility into backup organization, especially important for multi-node environments using namespaces to prevent VMID collisions.