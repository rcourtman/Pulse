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

function initializeUnifiedBackups() {
    if (!document.getElementById('tab-content-unified')) return;
    
    unified.mounted = true;
    
    setupEventListeners();
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
    const storageFilter = document.getElementById('unified-storage-filter');
    
    const searchTerm = searchInput?.value.toLowerCase() || '';
    const selectedType = typeFilter?.value || 'all';
    const selectedBackupType = backupTypeFilter?.value || 'all';
    const selectedStorage = storageFilter?.value || 'all';
    
    const allData = normalizeBackupData();
    
    unified.filteredData = allData.filter(item => {
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
        
        // Storage/Datastore filter
        if (selectedStorage !== 'all') {
            const itemStorage = item.storage || item.datastore;
            if (itemStorage !== selectedStorage) return false;
        }
        
        return true;
    });
    
    // Save filter state
    const filterControls = document.querySelector('#tab-content-unified .filter-controls');
    if (filterControls) {
        // Save filter state if needed
    }
}

function updateStorageFilter() {
    const backupTypeFilter = document.querySelector('input[name="unified-backup-type-filter"]:checked');
    const selectedBackupType = backupTypeFilter?.value || 'all';
    
    const allData = normalizeBackupData();
    const storageMap = new Map(); // storage name -> type
    
    allData.forEach(item => {
        // Only include storages relevant to the selected backup type
        if (selectedBackupType === 'all' || item.backupType === selectedBackupType) {
            if (item.storage) {
                storageMap.set(item.storage, 'local');
            }
            if (item.datastore) {
                storageMap.set(item.datastore, 'pbs');
            }
        }
    });
    
    const filterContainer = document.getElementById('unified-storage-filter-container');
    const selectElement = document.getElementById('unified-storage-filter');
    
    if (!filterContainer || !selectElement) return;
    
    if (storageMap.size <= 1) {
        filterContainer.style.display = 'none';
        return;
    }
    
    filterContainer.style.display = 'flex';
    const currentValue = selectElement.value;
    
    selectElement.innerHTML = '<option value="all">All Locations</option>';
    
    // Group by type
    const localStorages = [];
    const pbsStorages = [];
    
    storageMap.forEach((type, storage) => {
        if (type === 'local') {
            localStorages.push(storage);
        } else {
            pbsStorages.push(storage);
        }
    });
    
    // Add local storages
    if (localStorages.length > 0 && selectedBackupType !== 'remote') {
        if (pbsStorages.length > 0) {
            const optgroup = document.createElement('optgroup');
            optgroup.label = 'PVE Storage';
            localStorages.sort().forEach(storage => {
                const option = document.createElement('option');
                option.value = storage;
                option.textContent = storage;
                optgroup.appendChild(option);
            });
            selectElement.appendChild(optgroup);
        } else {
            // If only local storages, don't use optgroup
            localStorages.sort().forEach(storage => {
                const option = document.createElement('option');
                option.value = storage;
                option.textContent = storage;
                selectElement.appendChild(option);
            });
        }
    }
    
    // Add PBS datastores
    if (pbsStorages.length > 0 && selectedBackupType !== 'local') {
        if (localStorages.length > 0) {
            const optgroup = document.createElement('optgroup');
            optgroup.label = 'PBS Datastores';
            pbsStorages.sort().forEach(storage => {
                const option = document.createElement('option');
                option.value = storage;
                option.textContent = storage;
                optgroup.appendChild(option);
            });
            selectElement.appendChild(optgroup);
        } else {
            // If only PBS storages, don't use optgroup
            pbsStorages.sort().forEach(storage => {
                const option = document.createElement('option');
                option.value = storage;
                option.textContent = storage;
                selectElement.appendChild(option);
            });
        }
    }
    
    if (currentValue && Array.from(selectElement.options).some(opt => opt.value === currentValue)) {
        selectElement.value = currentValue;
    }
}

function renderUnifiedTable() {
    const container = document.getElementById('unified-table');
    if (!container) return;
    
    const sortState = PulseApp.state.getSortState('unified') || { key: 'backupTime', direction: 'desc' };
    unified.sortKey = sortState.key;
    unified.sortDirection = sortState.direction;
    const sortedData = PulseApp.utils.sortData(unified.filteredData, unified.sortKey, unified.sortDirection, 'unified');
    
    if (sortedData.length === 0) {
        container.innerHTML = `
            <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                <p class="text-lg">No backups found</p>
                <p class="text-sm mt-2">No backups, snapshots, or remote backups match your filters</p>
            </div>
        `;
        return;
    }
    
    // Group by date
    const grouped = groupByDate(sortedData);
    
    let html = `
        <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded overflow-hidden scrollbar">
            <table class="w-full text-xs sm:text-sm">
                <thead class="bg-gray-100 dark:bg-gray-800">
                    <tr class="text-[10px] sm:text-xs font-medium tracking-wider text-left text-gray-600 uppercase bg-gray-100 dark:bg-gray-700 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                    <th class="p-1 px-2 whitespace-nowrap text-center w-12">
                        Status
                    </th>
                    <th class="sortable p-1 px-2 whitespace-nowrap" onclick="PulseApp.ui.unifiedBackups.sortTable('backupType')">
                        Backup ${getSortIndicator('backupType')}
                    </th>
                    <th class="p-1 px-2 whitespace-nowrap w-[150px]">
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
                    <th class="sortable p-1 px-2 whitespace-nowrap w-24" onclick="PulseApp.ui.unifiedBackups.sortTable('size')">
                        Size ${getSortIndicator('size')}
                    </th>
                    <th class="p-1 px-2 whitespace-nowrap">
                        Location
                    </th>
                    <th class="p-1 px-2 whitespace-nowrap w-[150px]">
                        Details
                    </th>
                    <th class="sortable p-1 px-2 whitespace-nowrap w-20" onclick="PulseApp.ui.unifiedBackups.sortTable('backupTime')">
                        Time ${getSortIndicator('backupTime')}
                    </th>
                </tr>
            </thead>
            <tbody>
    `;
    
    Object.entries(grouped).forEach(([dateLabel, items]) => {
        html += `
            <tr>
                <td colspan="10" class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-700/50">
                    ${dateLabel} (${items.length})
                </td>
            </tr>
        `;
        
        items.forEach(item => {
            const ageColor = getTimeAgoColorClass(item.backupTime);
            const typeIcon = getTypeIcon(item.type);
            
            html += `
                <tr class="border-t border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/30">
                    <td class="p-1 px-2 whitespace-nowrap text-center">${getStatusIcon(item)}</td>
                    <td class="p-1 px-2 whitespace-nowrap">${getBackupTypeIcon(item.backupType)}</td>
                    <td class="p-1 px-2 w-[150px] max-w-[150px] truncate" title="${escapeHtml(item.name)}">${escapeHtml(item.name) || '-'}</td>
                    <td class="p-1 px-2 whitespace-nowrap">${typeIcon}</td>
                    <td class="p-1 px-2 whitespace-nowrap font-medium">${item.vmid}</td>
                    <td class="p-1 px-2 whitespace-nowrap cursor-pointer hover:text-blue-600 dark:hover:text-blue-400" onclick="handleNodeClick('${item.node}')">${item.node}</td>
                    <td class="p-1 px-2 whitespace-nowrap ${getSizeColor(item.size)}">${item.size ? formatBytes(item.size) : '-'}</td>
                    <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-400">
                        ${getLocationDisplay(item)}
                    </td>
                    <td class="p-1 px-2 text-gray-600 dark:text-gray-400 w-[150px] max-w-[150px] truncate" title="${escapeHtml(getDetails(item))}">
                        ${escapeHtml(getDetails(item))}
                    </td>
                    <td class="p-1 px-2 whitespace-nowrap text-xs ${ageColor}" title="${formatFullTime(item.backupTime)}">${formatTime(item.backupTime)}</td>
                </tr>
            `;
        });
    });
    
    html += '</tbody></table></div>';
    container.innerHTML = html;
}

function getStatusIcon(item) {
    if (item.backupType === 'remote') {
        if (item.verified) {
            return '<span title="Verified" class="text-green-600 dark:text-green-400">✓</span>';
        } else {
            return '<span title="Unverified" class="text-yellow-600 dark:text-yellow-400">⏱</span>';
        }
    }
    return '<span class="text-green-600 dark:text-green-400">✓</span>';
}

function getBackupTypeIcon(type) {
    const badges = {
        snapshot: '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-400" title="Point-in-time snapshot stored on the VM\'s host">Snapshot</span>',
        local: '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-orange-100 text-orange-800 dark:bg-orange-900/50 dark:text-orange-400" title="Local backup stored on Proxmox VE storage">PVE</span>',
        remote: '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900/50 dark:text-purple-400" title="Remote backup stored on Proxmox Backup Server">PBS</span>'
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
        if (dateOnly.getTime() === today.getTime()) {
            label = 'Today';
        } else if (dateOnly.getTime() === yesterday.getTime()) {
            label = 'Yesterday';
        } else {
            const month = months[date.getMonth()];
            const day = date.getDate();
            const suffix = getDaySuffix(day);
            label = `${month} ${day}${suffix}`;
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
    if (!timestamp) return 'text-gray-600 dark:text-gray-400';
    
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
        return 'text-orange-600 dark:text-orange-400';
    } else {
        // Over 30 days - red (old)
        return 'text-red-600 dark:text-red-400';
    }
}

function getTypeIcon(type) {
    if (type === 'VM') {
        return `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-400">VM</span>`;
    } else if (type === 'LXC') {
        return `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-400">LXC</span>`;
    }
    return `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-900/50 dark:text-gray-400">${type}</span>`;
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
    
    const months = ['January', 'February', 'March', 'April', 'May', 'June', 
                    'July', 'August', 'September', 'October', 'November', 'December'];
    
    const month = months[date.getMonth()];
    const day = date.getDate();
    const suffix = getDaySuffix(day);
    
    const hours = date.getHours();
    const minutes = date.getMinutes().toString().padStart(2, '0');
    const ampm = hours >= 12 ? 'PM' : 'AM';
    const hour12 = hours % 12 || 12;
    
    return `${month} ${day}${suffix}, ${hour12}:${minutes} ${ampm}`;
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
        });
    }
    
    // Type filters
    document.querySelectorAll('input[name="unified-type-filter"]').forEach(radio => {
        radio.addEventListener('change', () => {
            applyFilters();
            renderUnifiedTable();
        });
    });
    
    // Backup type filters
    document.querySelectorAll('input[name="unified-backup-type-filter"]').forEach(radio => {
        radio.addEventListener('change', () => {
            updateStorageFilter(); // Update storage filter when backup type changes
            applyFilters();
            renderUnifiedTable();
        });
    });
    
    // Storage filter
    const storageFilter = document.getElementById('unified-storage-filter');
    if (storageFilter) {
        storageFilter.addEventListener('change', () => {
            applyFilters();
            renderUnifiedTable();
        });
    }
    
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
    
    const storageFilter = document.getElementById('unified-storage-filter');
    if (storageFilter) storageFilter.value = 'all';
    
    unified.sortKey = 'backupTime';
    unified.sortDirection = 'desc';
    
    // Update sort state
    PulseApp.state.setSortState('unified', 'backupTime', 'desc');
    
    applyFilters();
    renderUnifiedTable();
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
    
    updateStorageFilter();
    applyFilters();
    renderUnifiedTable();
}

async function updateUnifiedBackupsInfo() {
    await fetchAllBackupData();
}

function cleanupUnifiedBackups() {
    unified.mounted = false;
}

    return {
        init: initializeUnifiedBackups,
        cleanup: cleanupUnifiedBackups,
        updateUnifiedBackupsInfo: updateUnifiedBackupsInfo,
        sortTable: sortTable
    };
})();