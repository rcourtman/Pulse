/**
 * Backup Models - Separate data structures for PBS, PVE, and Snapshot backups
 * This module defines the data models and processing logic for each backup type
 */

/**
 * PBS Backup Model
 * Represents a backup stored in Proxmox Backup Server
 */
class PBSBackup {
    constructor(data) {
        // Core identification
        this.type = 'pbs';
        this.backupType = data['backup-type']; // 'vm' or 'ct'
        this.backupId = data['backup-id'];      // VMID
        this.namespace = data.namespace || ''; // Preserve empty string for root namespace
        
        // PBS-specific fields
        this.pbsInstance = data.pbsInstanceName;
        this.datastore = data.datastoreName;
        this.owner = data.owner;                // e.g., "root@pam!backup"
        this.backupTime = data['backup-time'];
        this.size = data.size;
        this.protected = data.protected || false;
        this.comment = data.comment || '';
        this.verification = data.verification;
        
        // Derived fields
        this.ownerToken = this.extractOwnerToken();
        this.guestCompositeKey = null; // Set later when matched to guest
    }
    
    extractOwnerToken() {
        if (!this.owner || !this.owner.includes('!')) return null;
        return this.owner.split('!')[1].toLowerCase();
    }
    
    /**
     * Determine which endpoint/cluster this backup belongs to based on owner token
     */
    getEndpointHint() {
        if (!this.ownerToken) return 'unknown';
        
        // Common patterns:
        // - 'backup' usually means primary cluster
        // - Node names (e.g., 'pimox', 'delly') indicate specific nodes
        // - Custom tokens indicate secondary endpoints
        
        if (this.ownerToken === 'backup') return 'primary';
        return this.ownerToken; // Could be node name or custom endpoint
    }
    
    /**
     * Check if this backup matches a specific guest
     */
    matchesGuest(guest) {
        // Must match VMID
        if (String(this.backupId) !== String(guest.vmid)) return false;
        
        // Must match type (vm/ct)
        const guestType = guest.type === 'qemu' ? 'vm' : 'ct';
        if (this.backupType !== guestType) return false;
        
        // Check endpoint/owner matching
        const endpointHint = this.getEndpointHint();
        const guestEndpoint = guest.endpointId;
        
        // For primary endpoint guests (including when endpointId is not set)
        if (!guestEndpoint || guestEndpoint === 'primary') {
            // Backup must have 'backup' token, 'homelab' (common cluster name), or match the specific node name
            return endpointHint === 'primary' || endpointHint === 'backup' || 
                   endpointHint === 'homelab' || // Common cluster/endpoint name
                   endpointHint === guest.node.toLowerCase();
        }
        
        // For secondary endpoint guests
        if (guestEndpoint === 'secondary') {
            // Check if owner token matches the endpoint ID
            return endpointHint === 'secondary';
        }
        
        // For other endpoints, check if owner token matches endpoint ID
        return endpointHint === guestEndpoint.toLowerCase();
    }
}

/**
 * PVE Backup Model
 * Represents a backup stored on Proxmox VE storage (non-PBS)
 */
class PVEBackup {
    constructor(data) {
        // Core identification
        this.type = 'pve';
        this.vmid = String(data.vmid);
        this.backupType = data.backupType || this.extractTypeFromVolid(data.volid);
        
        // PVE-specific fields
        this.endpointId = data.endpointId || 'primary';
        this.node = data.node;
        this.storage = data.storage;
        this.volid = data.volid;
        this.ctime = data.ctime;
        this.size = data.size;
        this.protected = data.protected || false;
        
        // Derived fields
        this.filename = this.extractFilename();
        this.guestCompositeKey = `${this.endpointId}-${this.node}-${this.vmid}`;
    }
    
    extractTypeFromVolid(volid) {
        if (!volid) return 'unknown';
        if (volid.includes('vzdump-qemu')) return 'vm';
        if (volid.includes('vzdump-lxc')) return 'ct';
        return 'unknown';
    }
    
    extractFilename() {
        if (!this.volid) return '';
        const parts = this.volid.split('/');
        return parts[parts.length - 1] || '';
    }
    
    /**
     * PVE backups are directly tied to nodes, so matching is simpler
     */
    matchesGuest(guest) {
        return String(this.vmid) === String(guest.vmid) &&
               this.endpointId === guest.endpointId &&
               this.node === guest.node;
    }
}

/**
 * Snapshot Model
 * Represents a VM/CT snapshot (not a backup)
 */
class Snapshot {
    constructor(data) {
        // Core identification
        this.type = 'snapshot';
        this.vmid = String(data.vmid);
        this.name = data.name;
        this.guestType = data.guestType; // 'qemu' or 'lxc'
        
        // Snapshot-specific fields
        this.endpointId = data.endpointId || 'primary';
        this.node = data.node;
        this.snaptime = data.snaptime;
        this.description = data.description || '';
        this.parent = data.parent;
        this.vmstate = data.vmstate || false;
        
        // Derived fields
        this.guestCompositeKey = `${this.endpointId}-${this.node}-${this.vmid}`;
        this.age = this.calculateAge();
    }
    
    calculateAge() {
        if (!this.snaptime) return null;
        const now = Date.now() / 1000;
        return now - this.snaptime;
    }
    
    /**
     * Snapshots are directly tied to specific VMs on specific nodes
     */
    matchesGuest(guest) {
        return String(this.vmid) === String(guest.vmid) &&
               this.endpointId === guest.endpointId &&
               this.node === guest.node;
    }
}

/**
 * Guest Model
 * Represents a VM or Container with a unique composite key
 */
class Guest {
    constructor(data) {
        // Core identification
        this.vmid = String(data.vmid);
        this.name = data.name;
        this.type = data.type; // 'qemu' or 'lxc'
        this.node = data.node;
        this.endpointId = data.endpointId || 'primary';
        
        // Create unique composite key
        this.compositeKey = `${this.endpointId}-${this.node}-${this.vmid}`;
        
        // Guest properties
        this.status = data.status;
        this.tags = data.tags;
        
        // Associated backups (populated later)
        this.pbsBackups = [];
        this.pveBackups = [];
        this.snapshots = [];
    }
    
    /**
     * Add a backup to this guest if it matches
     */
    addBackupIfMatches(backup) {
        if (!backup.matchesGuest(this)) {
            return false;
        }
        
        // Set the guest composite key on the backup
        backup.guestCompositeKey = this.compositeKey;
        
        // Add to appropriate list
        if (backup instanceof PBSBackup) {
            this.pbsBackups.push(backup);
        } else if (backup instanceof PVEBackup) {
            this.pveBackups.push(backup);
        } else if (backup instanceof Snapshot) {
            this.snapshots.push(backup);
        }
        
        return true;
    }
    
    /**
     * Get backup counts by type
     */
    getBackupCounts() {
        return {
            pbs: this.pbsBackups.length,
            pve: this.pveBackups.length,
            snapshots: this.snapshots.length,
            total: this.pbsBackups.length + this.pveBackups.length
        };
    }
    
    /**
     * Get latest backup time across all backup types
     */
    getLatestBackupTime() {
        const allBackupTimes = [
            ...this.pbsBackups.map(b => b.backupTime),
            ...this.pveBackups.map(b => b.ctime)
        ].filter(t => t);
        
        if (allBackupTimes.length === 0) return null;
        return Math.max(...allBackupTimes);
    }
    
    /**
     * Get namespaces this guest has backups in
     */
    getNamespaces() {
        const namespaces = new Set();
        this.pbsBackups.forEach(backup => {
            namespaces.add(backup.namespace);
        });
        return Array.from(namespaces);
    }
}

module.exports = {
    PBSBackup,
    PVEBackup,
    Snapshot,
    Guest
};