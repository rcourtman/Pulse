# Backup Monitoring Architecture Analysis & Recommendations

## Executive Summary

**Problem**: Pulse, a Proxmox monitoring tool, is double-counting backup files when Proxmox Backup Server (PBS) is configured as both a direct PBS instance and as storage within Proxmox VE (PVE).

**Impact**: Inaccurate backup counts, misleading dashboards, potential operational confusion.

**Root Cause**: Architectural design that treats PBS and PVE storage as completely separate sources without cross-source deduplication.

---

## Background & Context

### What is Pulse?
Pulse is a web-based monitoring dashboard for Proxmox environments that provides:
- VM/Container monitoring
- Resource usage tracking
- **Backup status monitoring** (the focus of this analysis)
- Performance metrics and alerting

### Proxmox Backup Architecture
In a typical Proxmox environment, there are multiple ways to access backup data:

1. **Direct PBS API Access**: Connect directly to PBS instances to get backup snapshots, verification status, namespace information
2. **PVE Storage Interface**: PBS can be configured as storage within PVE, making backups accessible via PVE's storage API
3. **Traditional PVE Storage**: NFS, local storage, etc. for backup files
4. **VM/CT Snapshots**: Point-in-time snapshots (not backups) stored locally

### Current Token Configuration
Users typically configure:
- **PVE tokens**: For accessing VMs, containers, storage, and backups via PVE API
- **PBS tokens**: For accessing PBS-specific features like verification, namespaces, detailed backup metadata

---

## Problem Discovery

### Initial Symptom
User ran diagnostic tool and saw:
```
[INFO] Backup Status: Proxmox "homelab": Successfully accessing 9 backup storage(s) with 556 backup files.
[INFO] Backup Status: Proxmox "pimox": Successfully accessing 2 backup storage(s) with 0 backup files.
```

This raised questions about whether PBS backups were being counted twice when PBS is configured as both:
1. A direct PBS instance in Pulse configuration
2. A storage backend in PVE that Pulse accesses via PVE tokens

### Investigation Methodology
We performed a comprehensive code analysis tracing:
1. **PBS data flow**: How direct PBS API calls fetch snapshots
2. **PVE storage data flow**: How PVE storage API calls fetch backup files  
3. **Frontend aggregation**: How different sources are combined
4. **Deduplication logic**: What prevents double-counting

---

## Technical Analysis Results

### Data Flow Architecture

#### PBS Direct Access (`/opt/pulse/server/dataFetcher.js:869`)
```javascript
// Fetches from PBS API: /admin/datastore/{datastore}/snapshots
const snapshotResponse = await client.get(`/admin/datastore/${storeName}/snapshots`, { params });
// Tagged with source: 'pbs'
```

#### PVE Storage Access (`/opt/pulse/server/dataFetcher.js:1780`) 
```javascript
// Fetches from PVE API: /nodes/{node}/storage/{storage}/content?content=backup
const response = await apiClient.get(`/nodes/${nodeName}/storage/${storage}/content`, {
    params: { content: 'backup' }
});
// Tagged with source: 'pve'
```

### Key Generation Logic

#### PBS Backup Keys (`/opt/pulse/src/public/js/ui/backups.js:523`)
```javascript
const key = `${snap.backupVMID}-${snap.backupType}-${snap.pbsInstanceName}-${namespace}`;
// Example: "101-vm-MyPBS-root"
```

#### PVE Backup Keys (`/opt/pulse/src/public/js/ui/backups.js:529-531`)
```javascript
const endpointGenericKey = `${snap.backupVMID}-${snap.backupType}${endpointKey}`;
const fullNodeSpecificKey = `${snap.backupVMID}-${snap.backupType}${endpointKey}${nodeKey}`;
// Example: "101-vm-endpoint1" and "101-vm-endpoint1-node1"
```

### **Critical Finding: No Cross-Source Deduplication**

The same backup file accessed via both methods gets **completely different keys**:
- PBS Direct: `101-vm-MyPBS-root`
- PVE Storage: `101-vm-endpoint1`

Since these keys are different, the same backup gets counted multiple times.

### Existing "Deduplication" Logic
There is partial deduplication logic in `fetchStorageBackups()`:
```javascript
if (storage.toLowerCase().includes('pbs')) {
    console.log(`Skipping PBS storage '${storage}' for PVE backup collection`);
    return [];
}
```

**Problems with this approach:**
1. Only works if storage name contains "pbs"
2. PBS storage could be named anything ("backup-server", "storage1", etc.)
3. Doesn't handle existing dual configurations
4. No systematic solution for other potential duplications

### Backup Counting Logic (`/opt/pulse/src/public/js/ui/backups.js:784-786`)
```javascript
const pbsSnapshots = guestSnapshots.filter(s => s.source === 'pbs');
const pveSnapshots = guestSnapshots.filter(s => s.source === 'pve');
// These are counted separately - same backup counted twice!
```

---

## Impact Assessment

### Confirmed Issues
1. **Double-counting**: Same backup appears in both PBS and PVE counts
2. **Misleading dashboards**: Users see inflated backup numbers
3. **Resource waste**: Unnecessary API calls to both systems for same data
4. **User confusion**: Why do I need both PBS and PVE tokens?

### Diagnostic Evidence
The 556 backup files reported likely include:
- Traditional PVE backups (NFS, local storage)
- PBS backups accessed via PVE storage interface
- Same PBS backups counted again via direct PBS API access

---

## Proposed Solutions

### Option A: Source-Authoritative Architecture

**Core Principle**: Each backup source becomes authoritative for specific storage types.

```
┌─────────────────────────────────────────────────────────────┐
│                    UNIFIED BACKUP SERVICE                  │
├─────────────────────────────────────────────────────────────┤
│  Discovery Layer: Auto-detect backup sources               │
│  Normalization Layer: Convert to unified format            │
│  Deduplication Layer: Eliminate cross-source duplicates    │
│  Aggregation Layer: Combine data intelligently             │
└─────────────────────────────────────────────────────────────┘
```

**Authority Mapping:**
- **PBS Service**: Authoritative for all PBS-managed storage
- **PVE Storage**: Authoritative for traditional storage (NFS, local, etc.)
- **VM Snapshots**: Always via PVE (not backups)

**Configuration Example:**
```javascript
const backupSourceConfig = {
  pbsInstances: [
    { id: 'pbs1', name: 'MyPBS', authority: 'pbs-service' }
  ],
  pveStorages: [
    { id: 'nfs1', type: 'nfs', authority: 'pve-storage' },
    { id: 'pbs-storage', type: 'pbs', pbsInstanceId: 'pbs1', authority: 'delegate-to-pbs' }
  ]
}
```

### Option B: Quick Fix - Enhanced Storage Name Detection
Improve existing logic to better detect PBS storages:
```javascript
// Check storage type, not just name
if (storage.type === 'pbs' || storage.toLowerCase().includes('pbs')) {
    return [];
}
```

### Option C: Unified Data Model with Deduplication
Create backup fingerprinting:
```javascript
function generateBackupFingerprint(backup) {
    return `${backup.guest.vmid}-${backup.guest.node}-${backup.timestamp}-${backup.size}`;
}
```

---

## Implementation Roadmap

### Phase 1: Foundation (Weeks 1-2)
- [ ] Create unified data model (`BackupRecord` structure)
- [ ] Implement source authority mapping
- [ ] Add basic cross-source deduplication

### Phase 2: Source Authority (Weeks 3-4)
- [ ] PBS Service Authority implementation
- [ ] PVE Storage Authority implementation  
- [ ] Conflict resolution logic

### Phase 3: Enhanced Monitoring (Weeks 5-6)
- [ ] Recovery point tracking
- [ ] Backup job integration
- [ ] Cross-source relationship mapping

### Phase 4: User Experience (Weeks 7-8)
- [ ] Unified UI implementation
- [ ] Recovery-focused interface
- [ ] Advanced features (namespaces, job management)

---

## Decision Points

### 1. **Architecture Choice**
- **Option A**: Complete redesign with source authority
- **Option B**: Quick fix for immediate problem
- **Option C**: Hybrid approach with incremental improvements

### 2. **Configuration Strategy**
- **Auto-detection**: Automatically discover PBS-as-storage relationships
- **Explicit config**: Require manual configuration of source authorities
- **Hybrid**: Auto-detect with manual override capability

### 3. **Migration Approach**
- **Side-by-side**: Implement new system alongside current
- **Incremental**: Replace components one by one
- **Big bang**: Replace entire backup monitoring system

### 4. **Token Strategy**
- **Keep both**: Maintain PBS and PVE tokens with intelligent routing
- **Minimize**: Reduce to essential tokens only
- **Optimize**: Smart token usage based on source authority

---

## Questions for Consideration

1. **Which architectural approach would you recommend and why?**

2. **How should we handle the migration from the current system?**

3. **Are there other architectural patterns from similar monitoring tools we should consider?**

4. **What's the appropriate balance between auto-detection and explicit configuration?**

5. **Should we prioritize fixing the immediate duplication issue or building the ideal long-term architecture?**

---

## Technical Artifacts

### Key Files Analyzed
- `/opt/pulse/server/dataFetcher.js` - Main data collection logic
- `/opt/pulse/server/pbsUtils.js` - PBS task processing
- `/opt/pulse/server/pbsNamespaceDiscovery.js` - Namespace handling
- `/opt/pulse/src/public/js/ui/backups.js` - Frontend backup processing
- `/opt/pulse/server/diagnostics.js` - System diagnostics

### API Endpoints Involved
- PBS: `/admin/datastore/{datastore}/snapshots`
- PVE: `/nodes/{node}/storage/{storage}/content?content=backup`
- PVE: `/nodes/{node}/qemu/{vmid}/snapshot` (VM snapshots)
- PBS: `/status/datastore-usage` (PBS status)

---

*This analysis was conducted through comprehensive code review and API flow tracing. All findings are based on actual implementation details in the Pulse codebase.*