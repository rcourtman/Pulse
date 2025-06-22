// Backup-specific utility functions
PulseApp.utils = PulseApp.utils || {};

PulseApp.utils.backup = (() => {
    
    // Format backup age from days to human-readable string
    function formatAge(ageInDays) {
        if (ageInDays === Infinity || ageInDays === null || ageInDays === undefined) return 'Never';
        
        // Handle edge cases
        if (ageInDays < 0) return 'Just now'; // Future dates or clock sync issues
        if (ageInDays < 0.042) return 'Just now'; // Less than 1 hour
        if (ageInDays < 1) return `${Math.floor(ageInDays * 24)}h`;
        if (ageInDays < 7) return `${Math.floor(ageInDays)}d`;
        if (ageInDays < 30) return `${Math.floor(ageInDays / 7)}w`;
        if (ageInDays < 365) return `${Math.floor(ageInDays / 30)}mo`;
        return `${Math.floor(ageInDays / 365)}y`;
    }
    
    // Format time ago from timestamp (in seconds) to human-readable string
    function formatTimeAgo(timestamp) {
        if (!timestamp) return 'Never';
        
        const now = Math.floor(Date.now() / 1000);
        const diff = now - timestamp;
        
        if (diff < 0) return 'Just now'; // Future timestamp
        if (diff < 60) return 'Just now';
        if (diff < 3600) return `${Math.floor(diff / 60)} min ago`;
        if (diff < 86400) return `${Math.floor(diff / 3600)} hours ago`;
        if (diff < 604800) return `${Math.floor(diff / 86400)} days ago`;
        if (diff < 2592000) return `${Math.floor(diff / 604800)} weeks ago`;
        if (diff < 31536000) return `${Math.floor(diff / 2592000)} months ago`;
        return `${Math.floor(diff / 31536000)} years ago`;
    }
    
    // Get color class based on backup age
    function getAgeColor(ageInDays) {
        if (ageInDays <= 1) return 'text-green-600 dark:text-green-400';
        if (ageInDays <= 3) return 'text-blue-600 dark:text-blue-400';
        if (ageInDays <= 7) return 'text-yellow-600 dark:text-yellow-400';
        if (ageInDays <= 14) return 'text-orange-600 dark:text-orange-400';
        return 'text-red-600 dark:text-red-400';
    }
    
    // Format date to compact string (e.g., "Jan 15")
    function formatCompactDate(dateStr) {
        const date = new Date(dateStr);
        const month = date.toLocaleDateString('en-US', { month: 'short' });
        const day = date.getDate();
        return `${month} ${day}`;
    }
    
    // Get human-readable backup type label
    function getBackupTypeLabel(type) {
        switch(type) {
            case 'pbs': return 'PBS backups';
            case 'pve': return 'PVE backups';
            case 'snapshots': return 'snapshots';
            case 'pbsSnapshots': return 'PBS backups';
            case 'pveBackups': return 'PVE backups';
            case 'vmSnapshots': return 'snapshots';
            default: return 'backups';
        }
    }
    
    // Get backup type badge HTML
    function getBackupTypeBadge(type, size = 'normal') {
        const sizeClasses = {
            small: 'px-1 py-0.5 text-[8px]',
            normal: 'px-2 py-1 text-xs',
            large: 'px-3 py-1.5 text-sm'
        };
        
        const sizeClass = sizeClasses[size] || sizeClasses.normal;
        
        switch(type) {
            case 'pbs':
            case 'pbsSnapshots':
                return `<span class="${sizeClass} bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 rounded font-medium">PBS</span>`;
            case 'pve':
            case 'pveBackups':
                return `<span class="${sizeClass} bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300 rounded font-medium">PVE</span>`;
            case 'snapshots':
            case 'vmSnapshots':
                return `<span class="${sizeClass} bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300 rounded font-medium">SNAP</span>`;
            default:
                return '';
        }
    }
    
    // Calculate backup age in days from timestamp
    function calculateAgeInDays(timestamp, referenceTime = null) {
        if (!timestamp) return Infinity;
        
        const ref = referenceTime || (Date.now() / 1000);
        const age = (ref - timestamp) / 86400; // Convert to days
        
        return Math.max(0, age); // Ensure non-negative
    }
    
    // Check if backup is considered "recent" based on schedule
    function isRecentBackup(ageInDays, schedule = 'daily') {
        const thresholds = {
            'daily': 1.5,
            'weekly': 8,
            'monthly': 35,
            'unknown': 7
        };
        
        const threshold = thresholds[schedule] || thresholds.unknown;
        return ageInDays <= threshold;
    }
    
    // Public API
    return {
        formatAge,
        formatTimeAgo,
        getAgeColor,
        formatCompactDate,
        getBackupTypeLabel,
        getBackupTypeBadge,
        calculateAgeInDays,
        isRecentBackup
    };
})();