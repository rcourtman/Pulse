PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.unifiedBackups = (() => {

let unified = {
    snapshots: [],
    localBackups: [],
    pbsBackups: [],
    filteredData: [],
    instances: [],
    activeInstanceIndex: 0,
    sortKey: 'backupTime',
    sortDirection: 'desc',
    mounted: false
};

const CHART_MODE_KEY = 'unified-backups-chart-mode';
const FILTER_STATE_KEY = 'unified-backups-filters';
const TIMESTAMP_DISPLAY_KEY = 'unified-backups-timestamp-display';

function initializeUnifiedBackups() {
    if (!document.getElementById('tab-content-unified')) return;
    
    unified.mounted = true;
    
    // Restore timestamp display preference
    const savedTimestampDisplay = localStorage.getItem(TIMESTAMP_DISPLAY_KEY) || 'relative';
    const timestampRadio = document.querySelector(`input[name="timestamp-display"][value="${savedTimestampDisplay}"]`);
    if (timestampRadio) {
        timestampRadio.checked = true;
    }
    
    setupEventListeners();
    initializeBackupFrequencyChart();
    
    // Fetch and display data when initialized
    updateUnifiedBackupsInfo();
    
    // Keyboard handlers
    const searchInput = document.getElementById('unified-search');
    if (searchInput) {
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                resetFilters();
            } else if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
                const activeElement = document.activeElement;
                const isTyping = activeElement && (
                    activeElement.tagName === 'INPUT' ||
                    activeElement.tagName === 'TEXTAREA' ||
                    activeElement.isContentEditable
                );
                
                if (!isTyping && document.getElementById('unified').style.display !== 'none') {
                    searchInput.focus();
                    searchInput.value = e.key; // Set the typed character immediately
                    e.preventDefault(); // Prevent the character from being typed twice
                }
            }
        });
    }
    
    const filterControls = document.querySelector('#tab-content-unified .filter-controls');
    if (filterControls) {
        // Restore filter state if needed
    }
}

function normalizeBackupData() {
    const normalized = [];
    
    // Normalize snapshots
    unified.snapshots.forEach(snapshot => {
        normalized.push({
            backupType: 'snapshot',
            vmid: snapshot.vmid,
            name: snapshot.name,
            type: snapshot.type === 'qemu' ? 'VM' : 'LXC',
            node: snapshot.node,
            backupTime: snapshot.snaptime,
            backupName: snapshot.snapname,
            description: snapshot.description,
            status: 'ok',
            size: null,
            storage: null,
            datastore: null,
            namespace: null,
            verified: null,
            protected: false
        });
    });
    
    // Normalize local backups
    unified.localBackups.forEach(backup => {
        // Extract name from notes or use a better fallback
        let guestName = backup.notes || '';
        
        // If notes is empty, try to get name from guestName field if available
        if (!guestName && backup.guestName) {
            guestName = backup.guestName;
        }
        
        normalized.push({
            backupType: 'local',
            vmid: backup.vmid,
            name: guestName,
            type: backup.guestType,
            node: backup.node,
            backupTime: backup.ctime,
            backupName: backup.volid?.split('/').pop() || '',
            description: '',
            status: 'ok',
            size: backup.size,
            storage: backup.storage,
            datastore: null,
            namespace: null,
            verified: null,
            protected: false
        });
    });
    
    // Normalize PBS backups
    unified.pbsBackups.forEach(backup => {
        // PBS API returns 'notes' field, and 'type' instead of 'backupType'
        // Extract just the guest name from notes/comment
        let guestName = '';
        let fullNotes = backup.notes || backup.comment || '';
        
        // Try to extract name from "Name: xyz" pattern
        const nameMatch = fullNotes.match(/Name:\s*([^,\n]+)/);
        if (nameMatch) {
            guestName = nameMatch[1].trim();
        } else {
            // If no pattern, use the whole thing as name if it's short
            guestName = fullNotes.length < 30 ? fullNotes : '';
        }
        
        normalized.push({
            backupType: 'remote',
            vmid: backup.vmid,
            name: guestName,
            type: (backup.type === 'vm' || backup.type === 'qemu') ? 'VM' : 'LXC',
            node: backup.server || 'PBS',
            backupTime: backup.ctime || backup.backupTime,
            backupName: `${backup.vmid}/${new Date((backup.ctime || backup.backupTime) * 1000).toISOString().split('T')[0]}`,
            description: fullNotes,
            status: backup.verified ? 'verified' : 'unverified',
            size: backup.size,
            storage: null,
            datastore: backup.datastore,
            namespace: backup.namespace || 'root',
            verified: backup.verified,
            protected: backup.protected || false,
            instanceIndex: backup.instanceIndex
        });
    });
    
    return normalized;
}

function applyFilters() {
    const searchInput = document.getElementById('unified-search');
    const typeFilter = document.querySelector('input[name="unified-type-filter"]:checked');
    const backupTypeFilter = document.querySelector('input[name="unified-backup-type-filter"]:checked');
    
    const searchTerm = searchInput?.value.toLowerCase() || '';
    const selectedType = typeFilter?.value || 'all';
    const selectedBackupType = backupTypeFilter?.value || 'all';
    
    const allData = normalizeBackupData();
    
    unified.filteredData = allData.filter(item => {
        // Date range filter (from chart selection)
        if (selectedDateRange) {
            if (item.backupTime < selectedDateRange.start || item.backupTime > selectedDateRange.end) {
                return false;
            }
        }
        
        // Search filter
        if (searchTerm) {
            const searchFields = [
                item.vmid,
                item.name,
                item.node,
                item.backupName,
                item.description,
                item.storage,
                item.datastore,
                item.namespace
            ].filter(Boolean).map(field => field.toString().toLowerCase());
            
            const searchTerms = searchTerm.split(',').map(term => term.trim());
            const matchesSearch = searchTerms.some(term => 
                searchFields.some(field => field.includes(term))
            );
            
            if (!matchesSearch) return false;
        }
        
        // Type filter (VM/LXC)
        if (selectedType !== 'all' && item.type !== selectedType) {
            return false;
        }
        
        // Backup type filter
        if (selectedBackupType !== 'all' && item.backupType !== selectedBackupType) {
            return false;
        }
        
        return true;
    });
    
    // Save filter state
    const filterControls = document.querySelector('#tab-content-unified .filter-controls');
    if (filterControls) {
        // Save filter state if needed
    }
}

function renderUnifiedTable(skipTransition = false) {
    const container = document.getElementById('unified-table');
    if (!container) return;
    
    const sortState = PulseApp.state.getSortState('unified') || { key: 'backupTime', direction: 'desc' };
    unified.sortKey = sortState.key;
    unified.sortDirection = sortState.direction;
    const sortedData = PulseApp.utils.sortData(unified.filteredData, unified.sortKey, unified.sortDirection, 'unified');
    
    // Add fade effect unless skipping
    if (!skipTransition) {
        container.style.opacity = '0.7';
    }
    
    if (sortedData.length === 0) {
        setTimeout(() => {
            container.innerHTML = `
                <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                    <p class="text-lg">No backups found</p>
                    <p class="text-sm mt-2">No backups, snapshots, or remote backups match your filters</p>
                </div>
            `;
            container.style.opacity = '1';
        }, skipTransition ? 0 : 50);
        return;
    }
    
    // Group by date
    const grouped = groupByDate(sortedData);
    
    let html = `
        <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded overflow-hidden scrollbar">
            <style>
                /* Date header rows need more specific rule to override global CSS */
                #unified-table-inner tr.bg-gray-50 td.sticky.left-0 {
                    background-color: rgb(249, 250, 251) !important; /* gray-50 - solid */
                }
                .dark #unified-table-inner tr.dark\\:bg-gray-700\\/50 td.sticky.left-0 {
                    /* Use a color that visually matches gray-700/50 over gray-800 background */
                    background-color: rgb(43, 53, 68) !important; /* Between gray-800 and gray-700 */
                }
                
                /* Ensure sticky columns are opaque to prevent background bleed-through */
                #unified-table-inner td.sticky.left-0 {
                    background-clip: padding-box;
                }
            </style>
            <table class="w-full text-xs sm:text-sm" id="unified-table-inner">
                <thead class="bg-gray-100 dark:bg-gray-800">
                    <tr class="text-[10px] sm:text-xs font-medium tracking-wider text-left text-gray-600 uppercase bg-gray-100 dark:bg-gray-700 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                    <th class="sticky left-0 z-10 p-1 px-2 whitespace-nowrap w-[150px]">
                        Name
                    </th>
                    <th class="sortable p-1 px-2 whitespace-nowrap w-16" onclick="PulseApp.ui.unifiedBackups.sortTable('type')">
                        Type ${getSortIndicator('type')}
                    </th>
                    <th class="sortable p-1 px-2 whitespace-nowrap w-16" onclick="PulseApp.ui.unifiedBackups.sortTable('vmid')">
                        VMID ${getSortIndicator('vmid')}
                    </th>
                    <th class="sortable p-1 px-2 whitespace-nowrap w-24" onclick="PulseApp.ui.unifiedBackups.sortTable('node')">
                        Node ${getSortIndicator('node')}
                    </th>
                    <th class="sortable p-1 px-2 whitespace-nowrap w-32" onclick="PulseApp.ui.unifiedBackups.sortTable('backupTime')">
                        Time ${getSortIndicator('backupTime')}
                    </th>
                    <th class="sortable p-1 px-2 whitespace-nowrap w-24" onclick="PulseApp.ui.unifiedBackups.sortTable('size')">
                        Size ${getSortIndicator('size')}
                    </th>
                    <th class="sortable p-1 px-2 whitespace-nowrap w-20" onclick="PulseApp.ui.unifiedBackups.sortTable('backupType')">
                        Backup ${getSortIndicator('backupType')}
                    </th>
                    <th class="p-1 px-2 whitespace-nowrap text-center w-12">
                        Verified
                    </th>
                    <th class="p-1 px-2 whitespace-nowrap">
                        Location
                    </th>
                    <th class="p-1 px-2 whitespace-nowrap w-[250px]">
                        Details
                    </th>
                </tr>
            </thead>
            <tbody>
    `;
    
    // Sort date groups chronologically (most recent first)
    const sortedGroups = Object.entries(grouped).sort((a, b) => {
        const [labelA] = a;
        const [labelB] = b;
        
        // Handle special cases
        if (labelA === 'Today') return -1;
        if (labelB === 'Today') return 1;
        if (labelA === 'Yesterday') return labelB === 'Today' ? 1 : -1;
        if (labelB === 'Yesterday') return labelA === 'Today' ? -1 : 1;
        
        // For other dates, parse and compare
        // Get the first item from each group to extract the actual date
        const dateA = a[1][0].backupTime;
        const dateB = b[1][0].backupTime;
        
        // Sort descending (most recent first)
        return dateB - dateA;
    });
    
    sortedGroups.forEach(([dateLabel, items]) => {
        html += `
            <tr class="bg-gray-50 dark:bg-gray-700/50">
                <td class="sticky left-0 z-10 px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-700/50 whitespace-nowrap">
                    ${dateLabel} (${items.length})
                </td>
                <td colspan="9" class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400"></td>
            </tr>
        `;
        
        // Sort items within the group by time (most recent first)
        items.sort((a, b) => b.backupTime - a.backupTime);
        
        items.forEach(item => {
            const ageColor = getTimeAgoColorClass(item.backupTime);
            const typeIcon = getTypeIcon(item.type);
            
            html += `
                <tr class="border-t border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700">
                    <td class="sticky left-0 z-10 p-1 px-2 w-[150px] max-w-[150px] truncate" title="${escapeHtml(item.name)}">${escapeHtml(item.name) || '-'}</td>
                    <td class="p-1 px-2 whitespace-nowrap">${typeIcon}</td>
                    <td class="p-1 px-2 whitespace-nowrap font-medium">${item.vmid}</td>
                    <td class="p-1 px-2 whitespace-nowrap cursor-pointer hover:text-blue-600 dark:hover:text-blue-400" onclick="handleNodeClick('${item.node}')">${item.node}</td>
                    <td class="p-1 px-2 whitespace-nowrap text-xs ${ageColor}" title="${getTimestampDisplay() === 'relative' ? formatFullTime(item.backupTime) : formatTime(item.backupTime)}">${getTimestampDisplay() === 'relative' ? formatTime(item.backupTime) : formatFullTime(item.backupTime)}</td>
                    <td class="p-1 px-2 whitespace-nowrap ${getSizeColor(item.size)}">${item.size ? formatBytes(item.size) : '-'}</td>
                    <td class="p-1 px-2 whitespace-nowrap">${getBackupTypeIcon(item.backupType)}</td>
                    <td class="p-1 px-2 whitespace-nowrap text-center">${getStatusIcon(item)}</td>
                    <td class="p-1 px-2 whitespace-nowrap">
                        ${getLocationDisplay(item)}
                    </td>
                    <td class="p-1 px-2 w-[250px] max-w-[250px] truncate" title="${escapeHtml(getDetails(item))}">
                        ${escapeHtml(getDetails(item))}
                    </td>
                </tr>
            `;
        });
    });
    
    html += '</tbody></table></div>';
    
    // Apply content with fade effect
    setTimeout(() => {
        container.innerHTML = html;
        container.style.opacity = '1';
    }, skipTransition ? 0 : 50);
}

function getStatusIcon(item) {
    if (item.backupType === 'remote') {
        if (item.verified) {
            return '<span title="PBS backup verified" class="text-green-500 dark:text-green-400">✓</span>';
        } else {
            return '<span title="PBS backup not yet verified" class="text-yellow-500 dark:text-yellow-400">⏱</span>';
        }
    }
    // Snapshots and local backups don't have verification status
    return '<span class="text-gray-400 dark:text-gray-500" title="Verification only available for PBS backups">-</span>';
}

function getBackupTypeIcon(type) {
    const badges = {
        snapshot: '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-300" title="Point-in-time snapshot stored on the VM\'s host">Snapshot</span>',
        local: '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-orange-100 text-orange-700 dark:bg-orange-900/50 dark:text-orange-300" title="Local backup stored on Proxmox VE storage">PVE</span>',
        remote: '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-700 dark:bg-purple-900/50 dark:text-purple-300" title="Remote backup stored on Proxmox Backup Server">PBS</span>'
    };
    return badges[type] || '';
}

function getLocationDisplay(item) {
    if (item.storage) {
        return item.storage;
    } else if (item.datastore) {
        const parts = [item.datastore];
        if (item.namespace && item.namespace !== 'root') {
            parts.push(item.namespace);
        }
        return parts.join('/');
    }
    return '-';
}

function getDetails(item) {
    const details = [];
    
    if (item.backupType === 'snapshot') {
        details.push(item.backupName);
        if (item.description) {
            details.push(item.description);
        }
    } else if (item.backupType === 'local') {
        details.push(truncateMiddle(item.backupName, 30));
    } else if (item.backupType === 'remote') {
        if (item.protected) details.push('Protected');
        if (item.description && !item.description.includes('Name:')) {
            details.push(item.description);
        }
    }
    
    return details.join(' • ') || '-';
}

function getSizeColor(size) {
    if (!size) return '';
    const gb = size / (1024 * 1024 * 1024);
    if (gb < 5) return 'text-green-600 dark:text-green-400';
    if (gb < 20) return 'text-yellow-600 dark:text-yellow-400';
    if (gb < 50) return 'text-orange-600 dark:text-orange-400';
    return 'text-red-600 dark:text-red-400';
}

function groupByDate(data) {
    const grouped = {};
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const yesterday = new Date(today);
    yesterday.setDate(yesterday.getDate() - 1);
    
    const months = ['January', 'February', 'March', 'April', 'May', 'June',
                    'July', 'August', 'September', 'October', 'November', 'December'];
    
    data.forEach(item => {
        const date = new Date(item.backupTime * 1000);
        const dateOnly = new Date(date.getFullYear(), date.getMonth(), date.getDate());
        
        let label;
        const month = months[date.getMonth()];
        const day = date.getDate();
        const suffix = getDaySuffix(day);
        const absoluteDate = `${month} ${day}${suffix}`;
        
        if (dateOnly.getTime() === today.getTime()) {
            label = getTimestampDisplay() === 'absolute' ? `Today (${absoluteDate})` : 'Today';
        } else if (dateOnly.getTime() === yesterday.getTime()) {
            label = getTimestampDisplay() === 'absolute' ? `Yesterday (${absoluteDate})` : 'Yesterday';
        } else {
            label = absoluteDate;
        }
        
        if (!grouped[label]) {
            grouped[label] = [];
        }
        grouped[label].push(item);
    });
    
    return grouped;
}

function getSortIndicator(key) {
    if (unified.sortKey !== key) return '';
    return unified.sortDirection === 'asc' ? ' ↑' : ' ↓';
}

function sortTable(field) {
    if (unified.sortKey === field) {
        unified.sortDirection = unified.sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
        unified.sortKey = field;
        unified.sortDirection = 'asc';
    }
    
    PulseApp.state.setSortState('unified', field, unified.sortDirection);
    renderUnifiedTable();
}

function getTimeAgoColorClass(timestamp) {
    if (!timestamp) return 'text-gray-500 dark:text-gray-400';
    
    const now = Date.now() / 1000;
    const diff = now - timestamp;
    const days = diff / 86400;
    
    if (days < 1) {
        // Less than 1 day - green (fresh)
        return 'text-green-600 dark:text-green-400';
    } else if (days < 3) {
        // 1-3 days - still good
        return 'text-green-600 dark:text-green-400';
    } else if (days < 7) {
        // 3-7 days - yellow (caution)
        return 'text-yellow-600 dark:text-yellow-400';
    } else if (days < 30) {
        // 7-30 days - orange (warning)
        return 'text-orange-500 dark:text-orange-400';
    } else {
        // Over 30 days - red (old)
        return 'text-red-600 dark:text-red-400';
    }
}

function getTypeIcon(type) {
    if (type === 'VM') {
        return `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300">VM</span>`;
    } else if (type === 'LXC') {
        return `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300">LXC</span>`;
    }
    return `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-700 dark:bg-gray-900/50 dark:text-gray-300">${type}</span>`;
}

function formatTime(timestamp) {
    if (!timestamp) return '-';
    
    const now = Date.now() / 1000;
    const diff = now - timestamp;
    
    // Convert to relative time
    if (diff < 60) {
        return 'just now';
    } else if (diff < 3600) {
        const mins = Math.floor(diff / 60);
        return `${mins}m ago`;
    } else if (diff < 86400) {
        const hours = Math.floor(diff / 3600);
        return `${hours}h ago`;
    } else if (diff < 604800) {
        const days = Math.floor(diff / 86400);
        return `${days}d ago`;
    } else if (diff < 2592000) {
        const weeks = Math.floor(diff / 604800);
        return `${weeks}w ago`;
    } else if (diff < 31536000) {
        const months = Math.floor(diff / 2592000);
        return `${months}mo ago`;
    } else {
        const years = Math.floor(diff / 31536000);
        return `${years}y ago`;
    }
}

function formatFullTime(timestamp) {
    if (!timestamp) return '';
    const date = new Date(timestamp * 1000);
    
    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 
                    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    
    const month = months[date.getMonth()];
    const day = date.getDate();
    const hours = date.getHours().toString().padStart(2, '0');
    const minutes = date.getMinutes().toString().padStart(2, '0');
    
    return `${day} ${month} ${hours}:${minutes}`;
}

function getDaySuffix(day) {
    if (day >= 11 && day <= 13) return 'th';
    switch (day % 10) {
        case 1: return 'st';
        case 2: return 'nd';
        case 3: return 'rd';
        default: return 'th';
    }
}

function getTimestampDisplay() {
    const radio = document.querySelector('input[name="timestamp-display"]:checked');
    return radio ? radio.value : 'relative';
}

function updateTimestampToggleVisualState() {
    // Since we're using CSS adjacent sibling selector, the visual state should update automatically
    // This function is here for any additional visual updates if needed in the future
    const checkedRadio = document.querySelector('input[name="timestamp-display"]:checked');
    if (checkedRadio) {
        // Force reflow to ensure CSS updates are applied
        checkedRadio.offsetHeight;
    }
}

function formatBytes(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function truncateMiddle(str, maxLength) {
    if (!str || str.length <= maxLength) return str;
    const start = Math.ceil(maxLength / 2) - 2;
    const end = Math.floor(maxLength / 2) - 2;
    return str.substring(0, start) + '...' + str.substring(str.length - end);
}

function handleNodeClick(nodeName) {
    // Navigate to node in main tab
    const mainTab = document.querySelector('.tab[data-tab="main"]');
    if (mainTab) {
        mainTab.click();
        
        // Wait for tab to load then scroll to node
        setTimeout(() => {
            const nodeRow = document.querySelector(`tr[data-node="${nodeName}"]`);
            if (!nodeRow) {
                // If not in grouped view, try to find by searching the table
                const searchInput = document.getElementById('dashboard-search');
                if (searchInput) {
                    searchInput.value = nodeName;
                    searchInput.dispatchEvent(new Event('input'));
                }
                return;
            }
            
            nodeRow.scrollIntoView({ behavior: 'smooth', block: 'center' });
            nodeRow.classList.add('bg-blue-100', 'dark:bg-blue-900/50');
            setTimeout(() => {
                nodeRow.classList.remove('bg-blue-100', 'dark:bg-blue-900/50');
            }, 2000);
        }, 100);
    }
}

function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function setupEventListeners() {
    // Search input
    const searchInput = document.getElementById('unified-search');
    if (searchInput) {
        searchInput.addEventListener('input', () => {
            applyFilters();
            renderUnifiedTable();
            updateBackupFrequencyChart();
        });
    }
    
    // Type filters
    document.querySelectorAll('input[name="unified-type-filter"]').forEach(radio => {
        radio.addEventListener('change', () => {
            applyFilters();
            renderUnifiedTable();
            updateBackupFrequencyChart();
        });
    });
    
    // Backup type filters
    document.querySelectorAll('input[name="unified-backup-type-filter"]').forEach(radio => {
        radio.addEventListener('change', () => {
            applyFilters();
            renderUnifiedTable();
            updateBackupFrequencyChart();
        });
    });
    
    // Timestamp display toggle
    document.querySelectorAll('input[name="timestamp-display"]').forEach(radio => {
        radio.addEventListener('change', (e) => {
            localStorage.setItem(TIMESTAMP_DISPLAY_KEY, e.target.value);
            renderUnifiedTable();
            
            // Update visual state for custom toggle styling
            updateTimestampToggleVisualState();
        });
    });
    
    // Initialize visual state on load
    updateTimestampToggleVisualState();
    
    // Reset button
    const resetButton = document.getElementById('unified-reset-button');
    if (resetButton) {
        resetButton.addEventListener('click', resetFilters);
    }
}

function resetFilters() {
    const searchInput = document.getElementById('unified-search');
    if (searchInput) searchInput.value = '';
    
    const allRadio = document.querySelector('input[name="unified-type-filter"][value="all"]');
    if (allRadio) allRadio.checked = true;
    
    const allBackupRadio = document.querySelector('input[name="unified-backup-type-filter"][value="all"]');
    if (allBackupRadio) allBackupRadio.checked = true;
    
    unified.sortKey = 'backupTime';
    unified.sortDirection = 'desc';
    
    // Clear date range filter
    selectedDateRange = null;
    const dateRangeElement = document.getElementById('backup-chart-date-range');
    if (dateRangeElement) {
        dateRangeElement.textContent = 'Last 30 days';
    }
    
    // Hide clear filter button
    const clearButton = document.getElementById('clear-date-filter');
    if (clearButton) {
        clearButton.classList.add('hidden');
    }
    
    // Reset time range to 30 days
    chartTimeRangeDays = 30;
    document.querySelectorAll('.backup-chart-range').forEach(btn => {
        btn.classList.remove('bg-blue-100', 'dark:bg-blue-900/50', 'text-blue-700', 'dark:text-blue-300', 'border-blue-300', 'dark:border-blue-700');
    });
    const thirtyDayButton = document.querySelector('.backup-chart-range[data-days="30"]');
    if (thirtyDayButton) {
        thirtyDayButton.classList.add('bg-blue-100', 'dark:bg-blue-900/50', 'text-blue-700', 'dark:text-blue-300', 'border-blue-300', 'dark:border-blue-700');
    }
    
    // Update sort state
    PulseApp.state.setSortState('unified', 'backupTime', 'desc');
    
    applyFilters();
    renderUnifiedTable();
    updateBackupFrequencyChart();
}

async function fetchAllBackupData() {
    try {
        // Fetch all three types of backup data in parallel
        const [snapshotsRes, localBackupsRes, pbsBackupsRes] = await Promise.all([
            fetch('/api/snapshots').then(r => r.json()),
            fetch('/api/backups/pve').then(r => r.json()),
            fetch('/api/backups/pbs').then(r => r.json())
        ]);
        
        // Update snapshots
        unified.snapshots = snapshotsRes.snapshots || [];
        
        // Update local backups
        unified.localBackups = localBackupsRes.backups || [];
        
        // Update PBS backups
        if (pbsBackupsRes.backups) {
            // Direct backups array from API
            unified.pbsBackups = pbsBackupsRes.backups || [];
        } else if (pbsBackupsRes.instances) {
            // Legacy instances format
            unified.instances = pbsBackupsRes.instances;
            unified.pbsBackups = [];
            
            pbsBackupsRes.instances.forEach((instance, index) => {
                if (instance.backups) {
                    const backupsWithInstance = instance.backups.map(backup => ({
                        ...backup,
                        instanceIndex: index,
                        server: instance.name || instance.host
                    }));
                    unified.pbsBackups.push(...backupsWithInstance);
                }
            });
        }
        
        updateUnifiedBackups();
    } catch (error) {
        console.error('[Unified Backups] Error fetching data:', error);
        const container = document.getElementById('unified-table');
        if (container && unified.snapshots.length === 0 && unified.localBackups.length === 0 && unified.pbsBackups.length === 0) {
            container.innerHTML = `
                <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                    <p class="text-lg">Error loading backup data</p>
                    <p class="text-sm mt-2">${error.message}</p>
                    <button onclick="PulseApp.ui.unifiedBackups.updateUnifiedBackupsInfo()" class="mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
                        Retry
                    </button>
                </div>
            `;
        }
    }
}

function updateUnifiedBackups() {
    if (!unified.mounted) return;
    
    applyFilters();
    renderUnifiedTable();
    updateBackupFrequencyChart();
}

async function updateUnifiedBackupsInfo() {
    await fetchAllBackupData();
}

function cleanupUnifiedBackups() {
    unified.mounted = false;
}

// Backup Frequency Chart Functions
let selectedDateRange = null; // Store selected date range for filtering
let chartTimeRangeDays = 30; // Default to 30 days

function initializeBackupFrequencyChart() {
    const chartContainer = document.getElementById('backup-frequency-chart');
    if (!chartContainer) return;
    
    // Create SVG element for the chart
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('width', '100%');
    svg.setAttribute('height', '100%');
    svg.setAttribute('class', 'backup-frequency-svg');
    chartContainer.innerHTML = '';
    chartContainer.appendChild(svg);
    
    // Setup clear date filter button
    const clearButton = document.getElementById('clear-date-filter');
    if (clearButton) {
        clearButton.addEventListener('click', () => {
            clearDateFilter();
        });
    }
    
    // Setup time range buttons
    document.querySelectorAll('.backup-chart-range').forEach(button => {
        button.addEventListener('click', (e) => {
            const days = parseInt(e.target.dataset.days);
            setChartTimeRange(days);
        });
    });
}

function setChartTimeRange(days) {
    chartTimeRangeDays = days;
    
    // Update button styling
    document.querySelectorAll('.backup-chart-range').forEach(btn => {
        btn.classList.remove('bg-blue-100', 'dark:bg-blue-900/50', 'text-blue-700', 'dark:text-blue-300', 'border-blue-300', 'dark:border-blue-700');
    });
    
    const activeButton = document.querySelector(`.backup-chart-range[data-days="${days}"]`);
    if (activeButton) {
        activeButton.classList.add('bg-blue-100', 'dark:bg-blue-900/50', 'text-blue-700', 'dark:text-blue-300', 'border-blue-300', 'dark:border-blue-700');
    }
    
    // Update date range text
    const dateRangeElement = document.getElementById('backup-chart-date-range');
    if (dateRangeElement && !selectedDateRange) {
        let rangeText;
        if (days === 7) rangeText = 'Last 7 days';
        else if (days === 30) rangeText = 'Last 30 days';
        else if (days === 90) rangeText = 'Last 90 days';
        else if (days === 365) rangeText = 'Last year';
        dateRangeElement.textContent = rangeText;
    }
    
    // Update the chart
    updateBackupFrequencyChart();
}

function updateBackupFrequencyChart() {
    const chartContainer = document.getElementById('backup-frequency-chart');
    if (!chartContainer || !unified.mounted) return;
    
    // Get filters from radio buttons and search input
    const searchInput = document.getElementById('unified-search');
    const typeFilter = document.querySelector('input[name="unified-type-filter"]:checked');
    const backupTypeFilter = document.querySelector('input[name="unified-backup-type-filter"]:checked');
    const searchTerm = searchInput?.value.toLowerCase() || '';
    const selectedType = typeFilter?.value || 'all';
    const selectedBackupType = backupTypeFilter?.value || 'all';
    
    // Get all backup data and apply filters
    let allData = normalizeBackupData();
    
    // Apply search filter
    if (searchTerm) {
        const searchTerms = searchTerm.split(',').map(term => term.trim());
        allData = allData.filter(item => {
            const searchFields = [
                item.vmid,
                item.name,
                item.node,
                item.backupName,
                item.description,
                item.storage,
                item.datastore,
                item.namespace
            ].filter(Boolean).map(field => field.toString().toLowerCase());
            
            return searchTerms.some(term => 
                searchFields.some(field => field.includes(term))
            );
        });
    }
    
    // Apply type filter (VM/LXC)
    if (selectedType !== 'all') {
        allData = allData.filter(item => item.type === selectedType);
    }
    
    // Apply backup type filter
    if (selectedBackupType !== 'all') {
        allData = allData.filter(item => item.backupType === selectedBackupType);
    }
    
    // Calculate backup frequency per day
    const backupsByDate = {};
    const now = new Date();
    const startDate = new Date(now);
    startDate.setDate(startDate.getDate() - chartTimeRangeDays);
    
    // Initialize all days in the range with 0 backups
    for (let d = new Date(startDate); d <= now; d.setDate(d.getDate() + 1)) {
        const dateKey = d.toISOString().split('T')[0];
        backupsByDate[dateKey] = {
            total: 0,
            snapshot: 0,
            local: 0,
            remote: 0,
            date: new Date(d)
        };
    }
    
    // Count backups per day
    allData.forEach(backup => {
        const date = new Date(backup.backupTime * 1000);
        const dateKey = date.toISOString().split('T')[0];
        
        if (backupsByDate[dateKey]) {
            backupsByDate[dateKey].total++;
            backupsByDate[dateKey][backup.backupType]++;
        }
    });
    
    // Convert to array and sort by date
    const chartData = Object.entries(backupsByDate)
        .map(([date, data]) => ({ date, ...data }))
        .sort((a, b) => a.date - b.date);
    
    renderBackupFrequencyChart(chartData);
}

function renderBackupFrequencyChart(data) {
    const svg = document.querySelector('.backup-frequency-svg');
    if (!svg) return;
    
    // Clear existing content
    svg.innerHTML = '';
    
    // Set cursor style
    svg.style.cursor = 'pointer';
    
    // Chart dimensions
    const containerRect = svg.getBoundingClientRect();
    const margin = { top: 10, right: 10, bottom: 30, left: 30 };
    const width = containerRect.width - margin.left - margin.right;
    const height = 128 - margin.top - margin.bottom;
    
    svg.setAttribute('viewBox', `0 0 ${containerRect.width} 128`);
    
    // Create main group
    const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    g.setAttribute('transform', `translate(${margin.left},${margin.top})`);
    svg.appendChild(g);
    
    // Calculate scales
    const maxBackups = Math.max(...data.map(d => d.total), 1);
    const xScale = width / Math.max(data.length, 1);
    const barWidth = Math.max(1, Math.min(xScale - 2, 50)); // Cap at 50px max width
    const yScale = height / maxBackups;
    
    // Add grid lines
    const gridGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    gridGroup.setAttribute('class', 'grid-lines');
    g.appendChild(gridGroup);
    
    // Y-axis grid lines and labels
    const gridCount = 5;
    
    // Always add grid lines for visual consistency
    for (let i = 0; i <= gridCount; i++) {
        const y = height - (i * height / gridCount);
        const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
        line.setAttribute('x1', 0);
        line.setAttribute('y1', y);
        line.setAttribute('x2', width);
        line.setAttribute('y2', y);
        line.setAttribute('stroke', 'currentColor');
        line.setAttribute('stroke-opacity', '0.1');
        line.setAttribute('class', 'text-gray-300 dark:text-gray-600');
        gridGroup.appendChild(line);
    }
    
    // Add labels based on actual values
    if (maxBackups <= 5) {
        // For small values, show each integer
        for (let i = 0; i <= maxBackups; i++) {
            const y = height - (i * height / maxBackups);
            const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
            text.setAttribute('x', -5);
            text.setAttribute('y', y + 3);
            text.setAttribute('text-anchor', 'end');
            text.setAttribute('class', 'text-[10px] fill-gray-500 dark:fill-gray-400');
            text.textContent = i;
            g.appendChild(text);
        }
    } else {
        // For larger values, use the grid positions
        for (let i = 0; i <= gridCount; i++) {
            const value = Math.round(i * maxBackups / gridCount);
            const y = height - (i * height / gridCount);
            
            // Only add label if it's different from the previous one
            if (i === 0 || value !== Math.round((i - 1) * maxBackups / gridCount)) {
                const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
                text.setAttribute('x', -5);
                text.setAttribute('y', y + 3);
                text.setAttribute('text-anchor', 'end');
                text.setAttribute('class', 'text-[10px] fill-gray-500 dark:fill-gray-400');
                text.textContent = value;
                g.appendChild(text);
            }
        }
    }
    
    // Add bars
    const barsGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    barsGroup.setAttribute('class', 'bars');
    g.appendChild(barsGroup);
    
    data.forEach((d, i) => {
        const barHeight = d.total * yScale;
        const x = Math.max(0, i * xScale + (xScale - barWidth) / 2);
        const y = height - barHeight;
        
        // Create bar group
        const barGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
        barGroup.setAttribute('class', 'bar-group');
        barGroup.setAttribute('data-date', d.date.toISOString().split('T')[0]);
        barGroup.setAttribute('data-index', i);
        barGroup.style.cursor = 'pointer';
        
        // Background rect for click area
        const clickRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
        clickRect.setAttribute('x', i * xScale);
        clickRect.setAttribute('y', 0);
        clickRect.setAttribute('width', Math.max(1, xScale));
        clickRect.setAttribute('height', height);
        clickRect.setAttribute('fill', 'transparent');
        clickRect.style.cursor = 'pointer';
        barGroup.appendChild(clickRect);
        
        // Main bar
        const rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
        rect.setAttribute('x', x);
        rect.setAttribute('y', y);
        rect.setAttribute('width', barWidth);
        rect.setAttribute('height', barHeight);
        rect.setAttribute('rx', '2');
        rect.setAttribute('class', 'backup-bar');
        rect.setAttribute('data-date', d.date.toISOString().split('T')[0]);
        
        // Color based on backup count
        let barColor;
        if (d.total === 0) {
            barColor = '#e5e7eb'; // gray-200
        } else if (d.total <= 5) {
            barColor = '#60a5fa'; // blue-400
        } else if (d.total <= 10) {
            barColor = '#34d399'; // emerald-400
        } else {
            barColor = '#a78bfa'; // violet-400
        }
        
        rect.setAttribute('fill', barColor);
        rect.setAttribute('fill-opacity', '0.8');
        rect.style.transition = 'fill-opacity 0.2s ease';
        barGroup.appendChild(rect);
        
        // Add stacked segments for different backup types if there are backups
        if (d.total > 0) {
            let stackY = y;
            
            // PBS backups (bottom)
            if (d.remote > 0) {
                const pbsHeight = (d.remote / d.total) * barHeight;
                const pbsRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
                pbsRect.setAttribute('x', x);
                pbsRect.setAttribute('y', stackY + barHeight - pbsHeight);
                pbsRect.setAttribute('width', barWidth);
                pbsRect.setAttribute('height', pbsHeight);
                pbsRect.setAttribute('rx', '2');
                pbsRect.setAttribute('fill', '#8b5cf6'); // violet-500
                pbsRect.setAttribute('fill-opacity', '0.9');
                barGroup.appendChild(pbsRect);
            }
            
            // PVE backups (middle)
            if (d.local > 0) {
                const pveHeight = (d.local / d.total) * barHeight;
                const pveY = y + (d.snapshot / d.total) * barHeight;
                const pveRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
                pveRect.setAttribute('x', x);
                pveRect.setAttribute('y', pveY);
                pveRect.setAttribute('width', barWidth);
                pveRect.setAttribute('height', pveHeight);
                pveRect.setAttribute('fill', '#f97316'); // orange-500
                pveRect.setAttribute('fill-opacity', '0.9');
                barGroup.appendChild(pveRect);
            }
            
            // Snapshots (top)
            if (d.snapshot > 0) {
                const snapshotHeight = (d.snapshot / d.total) * barHeight;
                const snapshotRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
                snapshotRect.setAttribute('x', x);
                snapshotRect.setAttribute('y', y);
                snapshotRect.setAttribute('width', barWidth);
                snapshotRect.setAttribute('height', snapshotHeight);
                snapshotRect.setAttribute('rx', '2');
                snapshotRect.setAttribute('fill', '#eab308'); // yellow-500
                snapshotRect.setAttribute('fill-opacity', '0.9');
                barGroup.appendChild(snapshotRect);
            }
        }
        
        // Hover effects
        barGroup.addEventListener('mouseenter', (e) => {
            rect.setAttribute('fill-opacity', '1');
            
            // Show tooltip with localized date format
            const date = new Date(d.date);
            const userLocale = navigator.language || 'en-GB';
            const dateStr = date.toLocaleDateString(userLocale, { 
                day: '2-digit', 
                month: '2-digit', 
                year: 'numeric' 
            });
            let tooltipContent = `<strong>${dateStr}</strong><br>`;
            if (d.total > 0) {
                tooltipContent += `Total: ${d.total}<br>`;
                if (d.snapshot > 0) tooltipContent += `Snapshots: ${d.snapshot}<br>`;
                if (d.local > 0) tooltipContent += `PVE: ${d.local}<br>`;
                if (d.remote > 0) tooltipContent += `PBS: ${d.remote}`;
            } else {
                tooltipContent += 'No backups';
            }
            
            if (PulseApp.tooltips && PulseApp.tooltips.showTooltip) {
                PulseApp.tooltips.showTooltip(e, tooltipContent);
            }
            
            // Highlight the bar
            rect.setAttribute('fill-opacity', '1');
            rect.setAttribute('filter', 'brightness(1.2)');
            
        });
        
        barGroup.addEventListener('mouseleave', () => {
            rect.setAttribute('fill-opacity', '0.8');
            rect.removeAttribute('filter');
            if (PulseApp.tooltips && PulseApp.tooltips.hideTooltip) {
                PulseApp.tooltips.hideTooltip();
            }
            
        });
        
        // Click to filter
        barGroup.addEventListener('click', (e) => {
            filterBackupsByDate(d.date);
            e.stopPropagation();
        });
        
        barsGroup.appendChild(barGroup);
        
        // Add date labels based on time range
        let showLabel = false;
        let labelFormat;
        
        if (chartTimeRangeDays <= 7) {
            // Show every day for 7 days
            showLabel = true;
            labelFormat = { month: 'numeric', day: 'numeric' };
        } else if (chartTimeRangeDays <= 30) {
            // Show every few days for 30 days
            showLabel = i % Math.ceil(data.length / 10) === 0 || i === data.length - 1;
            labelFormat = { month: 'numeric', day: 'numeric' };
        } else if (chartTimeRangeDays <= 90) {
            // Show weekly for 90 days
            const dayOfWeek = d.date.getDay();
            showLabel = dayOfWeek === 0 || i === 0 || i === data.length - 1; // Sundays + first/last
            labelFormat = { month: 'short', day: 'numeric' };
        } else {
            // Show monthly for 1 year
            const dayOfMonth = d.date.getDate();
            showLabel = dayOfMonth === 1 || i === 0 || i === data.length - 1; // First of month + first/last
            labelFormat = { month: 'short' };
        }
        
        if (showLabel) {
            const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
            text.setAttribute('x', x + barWidth / 2);
            text.setAttribute('y', height + 15);
            text.setAttribute('text-anchor', 'middle');
            text.setAttribute('class', 'text-[9px] fill-gray-500 dark:fill-gray-400');
            
            // Use browser locale for date formatting
            const date = new Date(d.date);
            let dateStr;
            const userLocale = navigator.language || 'en-GB'; // Default to UK format if no locale
            
            if (chartTimeRangeDays <= 30) {
                // For shorter ranges, show day/month
                dateStr = date.toLocaleDateString(userLocale, { 
                    day: '2-digit', 
                    month: '2-digit' 
                });
            } else if (chartTimeRangeDays <= 90) {
                // For medium ranges, show day/month
                dateStr = date.toLocaleDateString(userLocale, { 
                    day: '2-digit', 
                    month: '2-digit' 
                });
            } else {
                // For year view, show month/year
                dateStr = date.toLocaleDateString(userLocale, { 
                    month: '2-digit', 
                    year: 'numeric' 
                });
            }
            
            text.textContent = dateStr;
            g.appendChild(text);
        }
    });
    
}

function filterBackupsByDate(date, isTemporary = false) {
    // Calculate start and end of the selected day
    const startOfDay = new Date(date);
    startOfDay.setHours(0, 0, 0, 0);
    const endOfDay = new Date(date);
    endOfDay.setHours(23, 59, 59, 999);
    
    if (!isTemporary) {
        // Store the selected date range for permanent filtering
        selectedDateRange = {
            start: startOfDay.getTime() / 1000,
            end: endOfDay.getTime() / 1000
        };
        
        // Update the date range display
        const dateRangeElement = document.getElementById('backup-chart-date-range');
        if (dateRangeElement) {
            const userLocale = navigator.language || 'en-GB';
            const dateStr = startOfDay.toLocaleDateString(userLocale, { 
                month: 'long', 
                day: 'numeric',
                year: 'numeric'
            });
            dateRangeElement.textContent = dateStr;
        }
        
        // Show clear filter button
        const clearButton = document.getElementById('clear-date-filter');
        if (clearButton) {
            clearButton.classList.remove('hidden');
        }
        
        // Highlight the selected bar
        highlightSelectedDate(date);
    } else {
        // For temporary hover filtering, just set a temporary date range
        const tempDateRange = {
            start: startOfDay.getTime() / 1000,
            end: endOfDay.getTime() / 1000
        };
        
        // Apply filters with temporary date range
        const searchInput = document.getElementById('unified-search');
        const typeFilter = document.querySelector('input[name="unified-type-filter"]:checked');
        const backupTypeFilter = document.querySelector('input[name="unified-backup-type-filter"]:checked');
        
        const searchTerm = searchInput?.value.toLowerCase() || '';
        const selectedType = typeFilter?.value || 'all';
        const selectedBackupType = backupTypeFilter?.value || 'all';
        
        const allData = normalizeBackupData();
        
        unified.filteredData = allData.filter(item => {
            // Apply temporary date filter
            if (item.backupTime < tempDateRange.start || item.backupTime > tempDateRange.end) {
                return false;
            }
            
            // Apply other filters
            if (searchTerm) {
                const searchFields = [
                    item.vmid,
                    item.name,
                    item.node,
                    item.backupName,
                    item.description,
                    item.storage,
                    item.datastore,
                    item.namespace
                ].filter(Boolean).map(field => field.toString().toLowerCase());
                
                const searchTerms = searchTerm.split(',').map(term => term.trim());
                const matchesSearch = searchTerms.some(term => 
                    searchFields.some(field => field.includes(term))
                );
                
                if (!matchesSearch) return false;
            }
            
            if (selectedType !== 'all' && item.type !== selectedType) {
                return false;
            }
            
            if (selectedBackupType !== 'all' && item.backupType !== selectedBackupType) {
                return false;
            }
            
            return true;
        });
        
        renderUnifiedTable(true); // Skip transition for hover
        return;
    }
    
    // Apply filters and re-render the table
    applyFilters();
    renderUnifiedTable();
}

function clearDateFilter() {
    // Clear the date range
    selectedDateRange = null;
    
    // Reset date range display to current time range
    const dateRangeElement = document.getElementById('backup-chart-date-range');
    if (dateRangeElement) {
        let rangeText;
        if (chartTimeRangeDays === 7) rangeText = 'Last 7 days';
        else if (chartTimeRangeDays === 30) rangeText = 'Last 30 days';
        else if (chartTimeRangeDays === 90) rangeText = 'Last 90 days';
        else if (chartTimeRangeDays === 365) rangeText = 'Last year';
        dateRangeElement.textContent = rangeText;
    }
    
    // Hide clear filter button
    const clearButton = document.getElementById('clear-date-filter');
    if (clearButton) {
        clearButton.classList.add('hidden');
    }
    
    // Re-apply filters and update display
    applyFilters();
    renderUnifiedTable();
    updateBackupFrequencyChart();
}

function highlightSelectedDate(date) {
    // Remove previous highlights
    document.querySelectorAll('.backup-bar').forEach(bar => {
        bar.classList.remove('ring-2', 'ring-blue-500');
    });
    
    // Add highlight to selected bar
    const dateKey = date.toISOString().split('T')[0];
    const selectedBar = document.querySelector(`.backup-bar[data-date="${dateKey}"]`);
    if (selectedBar) {
        selectedBar.classList.add('ring-2', 'ring-blue-500');
    }
}

    return {
        init: initializeUnifiedBackups,
        cleanup: cleanupUnifiedBackups,
        updateUnifiedBackupsInfo: updateUnifiedBackupsInfo,
        sortTable: sortTable
    };
})();