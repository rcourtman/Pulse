/**
 * Centralized Backup Filter Manager
 * Handles all backup filtering logic in one place with clear separation of concerns
 */
PulseApp.utils = PulseApp.utils || {};

PulseApp.utils.BackupFilterManager = class BackupFilterManager {
    constructor() {
        // Define which backup types support which filters
        this.backupTypeConfig = {
            pbsSnapshots: {
                supportsNamespaces: true,
                displayName: 'PBS Snapshots',
                filterKey: 'pbs',
                color: 'purple'
            },
            pveBackups: {
                supportsNamespaces: false,
                displayName: 'PVE Backups',
                filterKey: 'pve',
                color: 'orange'
            },
            vmSnapshots: {
                supportsNamespaces: false,
                displayName: 'VM Snapshots',
                filterKey: 'snapshots',
                color: 'yellow'
            }
        };
    }

    /**
     * Check if a backup type supports namespace filtering
     * @param {string} backupType - The type of backup (pbsSnapshots, pveBackups, vmSnapshots)
     * @returns {boolean}
     */
    supportsNamespaces(backupType) {
        return this.backupTypeConfig[backupType]?.supportsNamespaces || false;
    }

    /**
     * Get display configuration for a backup type
     * @param {string} backupType 
     * @returns {Object}
     */
    getBackupTypeConfig(backupType) {
        return this.backupTypeConfig[backupType] || null;
    }

    /**
     * Check if a backup should be included based on active filters
     * @param {Object} backup - The backup item
     * @param {string} backupType - The type of backup
     * @param {Object} filters - Active filters
     * @returns {boolean}
     */
    shouldIncludeBackup(backup, backupType, filters = {}) {
        // Check backup type filter
        if (filters.backupType && filters.backupType !== 'all') {
            const config = this.getBackupTypeConfig(backupType);
            if (!config || config.filterKey !== filters.backupType) {
                return false;
            }
        }

        // Check namespace filter (only for backup types that support it)
        if (filters.namespace && filters.namespace !== 'all' && this.supportsNamespaces(backupType)) {
            const backupNamespace = backup.namespace || 'root';
            if (backupNamespace !== filters.namespace) {
                return false;
            }
        }

        // Check guest type filter
        if (filters.guestType && filters.guestType !== 'all') {
            const guestType = backup.type || backup.guestType;
            if (guestType !== filters.guestType) {
                return false;
            }
        }

        // Check search filter
        if (filters.search) {
            const searchLower = filters.search.toLowerCase();
            const searchableFields = [
                backup.name,
                backup.guestName,
                backup.vmid?.toString(),
                backup.volid,
                backup.comment
            ].filter(Boolean).join(' ').toLowerCase();
            
            if (!searchableFields.includes(searchLower)) {
                return false;
            }
        }

        return true;
    }

    /**
     * Filter a collection of backups based on active filters
     * @param {Array} backups - Array of backup items
     * @param {string} backupType - The type of backups
     * @param {Object} filters - Active filters
     * @returns {Array}
     */
    filterBackups(backups, backupType, filters = {}) {
        if (!Array.isArray(backups)) return [];
        return backups.filter(backup => this.shouldIncludeBackup(backup, backupType, filters));
    }

    /**
     * Get filtered guest IDs based on backup data and filters
     * @param {Object} backupData - All backup data by type
     * @param {Object} filters - Active filters
     * @returns {Array} Array of guest IDs that have backups matching the filters
     */
    getFilteredGuestIds(backupData, filters = {}) {
        const guestIds = new Set();

        Object.entries(backupData).forEach(([backupType, backups]) => {
            if (!Array.isArray(backups)) return;

            // For each backup type, only filter by namespace if it supports it
            const filteredBackups = this.filterBackups(backups, backupType, {
                ...filters,
                // Override namespace filter for types that don't support it
                namespace: this.supportsNamespaces(backupType) ? filters.namespace : 'all'
            });

            filteredBackups.forEach(backup => {
                const vmid = backup.vmid || backup['backup-id'] || backup.backupVMID;
                if (vmid) {
                    const nodeIdentifier = backup.node || backup.endpointId || '';
                    const uniqueKey = nodeIdentifier ? `${vmid}-${nodeIdentifier}` : vmid.toString();
                    guestIds.add(uniqueKey);
                }
            });
        });

        return Array.from(guestIds);
    }

    /**
     * Check if filters would exclude all backups of a certain type
     * @param {string} backupType 
     * @param {Object} filters 
     * @returns {boolean}
     */
    wouldExcludeBackupType(backupType, filters = {}) {
        // If backup type filter is set and doesn't match this type
        if (filters.backupType && filters.backupType !== 'all') {
            const config = this.getBackupTypeConfig(backupType);
            return !config || config.filterKey !== filters.backupType;
        }
        return false;
    }

    /**
     * Get active filter summary for display
     * @param {Object} filters 
     * @returns {Object}
     */
    getFilterSummary(filters = {}) {
        const summary = {
            activeCount: 0,
            filters: []
        };

        if (filters.backupType && filters.backupType !== 'all') {
            summary.activeCount++;
            const config = Object.values(this.backupTypeConfig).find(c => c.filterKey === filters.backupType);
            summary.filters.push({
                type: 'backupType',
                label: config ? config.displayName : filters.backupType
            });
        }

        if (filters.namespace && filters.namespace !== 'all') {
            summary.activeCount++;
            summary.filters.push({
                type: 'namespace',
                label: `Namespace: ${filters.namespace}`
            });
        }

        if (filters.guestType && filters.guestType !== 'all') {
            summary.activeCount++;
            summary.filters.push({
                type: 'guestType',
                label: filters.guestType === 'qemu' ? 'VMs' : 'Containers'
            });
        }

        if (filters.search) {
            summary.activeCount++;
            summary.filters.push({
                type: 'search',
                label: `Search: "${filters.search}"`
            });
        }

        return summary;
    }
};

// Create singleton instance
PulseApp.utils.backupFilterManager = new PulseApp.utils.BackupFilterManager();