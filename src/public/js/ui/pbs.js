PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.pbs = (() => {
    // State
    let isInitialized = false;
    let pbsData = [];
    let pbsInstances = [];
    let activeInstance = 0;
    let taskStats = {};
    let filters = {
        searchTerm: '',
        datastore: 'all',
        backupType: 'all', // 'all', 'vm', 'ct'
        verificationStatus: 'all' // 'all', 'verified', 'unverified'
    };
    let currentSort = {
        field: 'backupTime',
        ascending: false
    };

    // Initialize
    function init() {
        if (isInitialized) return;
        isInitialized = true;
        updatePBSInfo();
    }

    // Fetch and update PBS data
    function updatePBSInfo() {
        const container = document.getElementById('pbs-content');
        if (!container) return;

        // Only show loading state on initial load
        if (!isInitialized || pbsData.length === 0) {
            container.innerHTML = `
                <div class="p-4 text-center text-gray-500 dark:text-gray-400">
                    Loading remote backups...
                </div>
            `;
        }

        // Get PBS data from state
        pbsInstances = PulseApp.state?.get?.('pbsDataArray') || [];
        
        if (pbsInstances.length === 0) {
            container.innerHTML = `
                <div class="p-8 text-center">
                    <div class="text-gray-500 dark:text-gray-400">
                        Remote backup server integration is not configured
                    </div>
                </div>
            `;
            return;
        }

        // Process PBS data into flat list of backups and collect task stats
        const newBackups = [];
        let newTaskStats = {
            lastBackupTime: 0,
            failedTasks24h: 0,
            runningTasks: 0,
            syncStatus: 'Unknown',
            totalDatastores: 0,
            taskDetails: {
                backup: { ok: 0, failed: 0, running: 0 },
                verify: { ok: 0, failed: 0, running: 0, lastRun: 0 },
                sync: { ok: 0, failed: 0, running: 0 },
                prune: { ok: 0, failed: 0, running: 0, lastRun: 0 }
            }
        };
        
        const now = Date.now() / 1000;
        const last24h = now - (24 * 60 * 60);
        
        pbsInstances.forEach((instance, instanceIndex) => {
            const serverName = instance.nodeName || instance.pbsInstanceName || 'PBS';
            
            // Count datastores
            if (instance.datastores && Array.isArray(instance.datastores)) {
                newTaskStats.totalDatastores += instance.datastores.length;
                
                instance.datastores.forEach(datastore => {
                    // Collect datastore stats
                    if (datastore.total && datastore.used) {
                        if (!newTaskStats.datastoreStats) {
                            newTaskStats.datastoreStats = { total: 0, used: 0 };
                        }
                        newTaskStats.datastoreStats.total += datastore.total || 0;
                        newTaskStats.datastoreStats.used += datastore.used || 0;
                    }
                    
                    if (datastore.snapshots && Array.isArray(datastore.snapshots)) {
                        datastore.snapshots.forEach(snapshot => {
                            // Calculate total size from files
                            let totalSize = 0;
                            if (snapshot.files && Array.isArray(snapshot.files)) {
                                totalSize = snapshot.files.reduce((sum, file) => sum + (file.size || 0), 0);
                            }
                            
                            // Track most recent backup
                            if (snapshot['backup-time'] > newTaskStats.lastBackupTime) {
                                newTaskStats.lastBackupTime = snapshot['backup-time'];
                            }
                            
                            const backup = {
                                instanceIndex: instanceIndex,
                                server: serverName,
                                datastore: datastore.name,
                                namespace: snapshot.namespace || 'root',
                                vmid: snapshot['backup-id'],
                                backupTime: snapshot['backup-time'],
                                backupType: snapshot['backup-type'], // 'vm' or 'ct'
                                size: totalSize,
                                files: snapshot.files || [],
                                verified: snapshot.verification && snapshot.verification.state === 'ok',
                                verificationTime: snapshot.verification && snapshot.verification.upid ? snapshot['backup-time'] : null,
                                protected: snapshot.protected || false,
                                owner: snapshot.owner || '',
                                comment: snapshot.comment || '',
                                deduplicationFactor: datastore.deduplicationFactor || null
                            };
                            newBackups.push(backup);
                        });
                    }
                });
            }
            
            // Process task data
            const taskTypeMap = {
                'backupTasks': 'backup',
                'verificationTasks': 'verify',
                'syncTasks': 'sync',
                'pruneTasks': 'prune'
            };
            
            Object.entries(taskTypeMap).forEach(([taskKey, taskName]) => {
                if (instance[taskKey] && instance[taskKey].recentTasks) {
                    instance[taskKey].recentTasks.forEach(task => {
                        const taskTime = task.startTime || task.starttime || 0;
                        
                        // Only count tasks from last 24h for the summary
                        if (taskTime >= last24h) {
                            const taskDetail = newTaskStats.taskDetails[taskName];
                            
                            if (!task.status || task.status.toLowerCase().includes('running')) {
                                taskDetail.running++;
                                newTaskStats.runningTasks++;
                            } else if (task.status === 'OK') {
                                taskDetail.ok++;
                            } else {
                                taskDetail.failed++;
                                newTaskStats.failedTasks24h++;
                            }
                            
                            // Track last run time for verify and prune
                            if ((taskName === 'verify' || taskName === 'prune') && taskTime > taskDetail.lastRun) {
                                taskDetail.lastRun = taskTime;
                            }
                        }
                        
                        // Special handling for sync status
                        if (taskName === 'sync' && taskTime >= last24h) {
                            if (!task.status || task.status.toLowerCase().includes('running')) {
                                newTaskStats.syncStatus = 'Running';
                            } else if (task.status === 'OK') {
                                newTaskStats.syncStatus = 'OK';
                            } else {
                                newTaskStats.syncStatus = 'Failed';
                            }
                        }
                    });
                }
            });
        });

        // Check if data has actually changed
        const backupsChanged = JSON.stringify(newBackups) !== JSON.stringify(pbsData);
        const statsChanged = JSON.stringify(newTaskStats) !== JSON.stringify(taskStats);
        
        if (backupsChanged || statsChanged || !isInitialized) {
            pbsData = newBackups;
            taskStats = newTaskStats;
            renderPBSUI();
        }
    }

    // Main render function
    function renderPBSUI() {
        const container = document.getElementById('pbs-content');
        if (!container) return;

        // Save scroll position before update
        const scrollContainer = container.querySelector('.overflow-x-auto');
        const savedScrollLeft = scrollContainer ? scrollContainer.scrollLeft : 0;
        const savedScrollTop = scrollContainer ? scrollContainer.scrollTop : 0;

        let html = '';
        
        // If multiple instances, show tabs
        if (pbsInstances.length > 1) {
            html += `
                <div class="border-b border-gray-200 dark:border-gray-700 mb-3">
                    <nav class="flex space-x-1 overflow-x-auto scrollbar" role="tablist">
                        ${pbsInstances.map((instance, index) => {
                            const instanceName = instance.nodeName || instance.pbsInstanceName || 'PBS Server ' + (index + 1);
                            const isActive = index === activeInstance;
                            return `
                                <button class="pbs-instance-tab whitespace-nowrap px-3 py-2 font-medium text-sm ${isActive ? 'text-blue-600 dark:text-blue-400 border-b-2 border-blue-600 dark:border-blue-400' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600 border-b-2 border-transparent'} transition-colors" 
                                    data-instance="${index}"
                                    role="tab"
                                    aria-selected="${isActive}"
                                    title="${instanceName}">
                                    ${instanceName}
                                </button>
                            `;
                        }).join('')}
                    </nav>
                </div>
            `;
        }
        
        // Show single instance summary card
        if (pbsInstances.length >= 1) {
            html += `
                <div class="mb-3">
                    <div class="flex flex-wrap gap-3">
                        ${createPBSInstanceCard(pbsInstances[activeInstance], activeInstance)}
                    </div>
                </div>
            `;
        }
        
        html += renderPBSContent();
        
        container.innerHTML = html;

        // Restore scroll position after update
        const newScrollContainer = container.querySelector('.overflow-x-auto');
        if (newScrollContainer && (savedScrollLeft > 0 || savedScrollTop > 0)) {
            newScrollContainer.scrollLeft = savedScrollLeft;
            newScrollContainer.scrollTop = savedScrollTop;
        }

        // Setup event listeners
        setupEventListeners();
        updateResetButtonState();
    }


    // Switch to a different PBS instance
    function switchInstance(index) {
        if (index >= 0 && index < pbsInstances.length) {
            activeInstance = index;
            renderPBSUI();
        }
    }

    // Get data for the active instance only
    function getActiveInstanceData() {
        if (!pbsInstances[activeInstance]) {
            return [];
        }
        // Filter by instance index instead of server name
        return pbsData.filter(backup => backup.instanceIndex === activeInstance);
    }

    // Get task stats for the active instance only
    function getActiveInstanceStats() {
        if (!pbsInstances[activeInstance]) return {};
        
        const instance = pbsInstances[activeInstance];
        const now = Date.now() / 1000;
        const last24h = now - (24 * 60 * 60);
        
        let stats = {
            lastBackupTime: 0,
            failedTasks24h: 0,
            runningTasks: 0,
            syncStatus: 'Unknown',
            totalDatastores: 0,
            taskDetails: {
                backup: { ok: 0, failed: 0, running: 0 },
                verify: { ok: 0, failed: 0, running: 0, lastRun: 0 },
                sync: { ok: 0, failed: 0, running: 0 },
                prune: { ok: 0, failed: 0, running: 0, lastRun: 0 }
            }
        };
        
        // Count datastores and collect storage stats
        if (instance.datastores && Array.isArray(instance.datastores)) {
            stats.totalDatastores = instance.datastores.length;
            
            // If multiple datastores, find the most critical one (highest usage)
            let mostCriticalDatastore = null;
            let highestUsagePercent = 0;
            
            instance.datastores.forEach(datastore => {
                if (datastore.total && datastore.used && datastore.total > 0) {
                    const usagePercent = (datastore.used / datastore.total) * 100;
                    if (usagePercent > highestUsagePercent) {
                        highestUsagePercent = usagePercent;
                        mostCriticalDatastore = {
                            name: datastore.name,
                            total: datastore.total,
                            used: datastore.used,
                            usagePercent: usagePercent
                        };
                    }
                }
                
                if (datastore.snapshots && Array.isArray(datastore.snapshots)) {
                    datastore.snapshots.forEach(snapshot => {
                        if (snapshot['backup-time'] > stats.lastBackupTime) {
                            stats.lastBackupTime = snapshot['backup-time'];
                        }
                    });
                }
            });
            
            // Use the most critical datastore for display, or sum if only one
            if (stats.totalDatastores === 1 && instance.datastores[0]) {
                const ds = instance.datastores[0];
                stats.datastoreStats = {
                    total: ds.total || 0,
                    used: ds.used || 0,
                    name: ds.name
                };
            } else if (mostCriticalDatastore) {
                stats.datastoreStats = mostCriticalDatastore;
            }
        }
        
        // Process task data for this instance
        const taskTypeMap = {
            'backupTasks': 'backup',
            'verificationTasks': 'verify',
            'syncTasks': 'sync',
            'pruneTasks': 'prune'
        };
        
        Object.entries(taskTypeMap).forEach(([taskKey, taskName]) => {
            if (instance[taskKey] && instance[taskKey].recentTasks) {
                instance[taskKey].recentTasks.forEach(task => {
                    const taskTime = task.startTime || task.starttime || 0;
                    
                    // Only count tasks from last 24h for the summary
                    if (taskTime >= last24h) {
                        const taskDetail = stats.taskDetails[taskName];
                        
                        if (!task.status || task.status.toLowerCase().includes('running')) {
                            taskDetail.running++;
                            stats.runningTasks++;
                        } else if (task.status === 'OK') {
                            taskDetail.ok++;
                        } else {
                            taskDetail.failed++;
                            stats.failedTasks24h++;
                        }
                        
                        // Track last run time for verify and prune
                        if ((taskName === 'verify' || taskName === 'prune') && taskTime > taskDetail.lastRun) {
                            taskDetail.lastRun = taskTime;
                        }
                    }
                    
                    // Special handling for sync status
                    if (taskName === 'sync' && taskTime >= last24h) {
                        if (!task.status || task.status.toLowerCase().includes('running')) {
                            stats.syncStatus = 'Running';
                        } else if (task.status === 'OK') {
                            stats.syncStatus = 'OK';
                        } else {
                            stats.syncStatus = 'Failed';
                        }
                    }
                });
            }
        });
        
        return stats;
    }

    // Render PBS content
    function renderPBSContent() {
        // Filter data for active instance only
        const activeInstanceData = getActiveInstanceData();
        const uniqueDatastores = getUniqueValues('datastore', activeInstanceData);
        
        return `

            <!-- PBS Filters -->
            <div class="mb-3 p-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded shadow-sm">
                <div class="flex flex-row flex-wrap items-center gap-2 sm:gap-3">
                    <div class="filter-controls-wrapper flex items-center gap-2 flex-1 min-w-[180px] sm:min-w-[240px]">
                        <input type="search" id="pbs-search" placeholder="Search by VMID, comment, or namespace..." 
                            value="${filters.searchTerm}"
                            class="flex-1 p-1 px-2 h-7 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 focus:ring-1 focus:ring-blue-500 focus:border-blue-500 outline-none">
                        <button id="reset-pbs-button" title="Reset Filters & Sort (Esc)" class="flex items-center justify-center p-1 h-7 w-7 text-xs border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-500 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 focus:outline-none transition-colors flex-shrink-0">
                            <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg>
                        </button>
                    </div>
                    
                    <!-- Datastore Filter -->
                    ${uniqueDatastores.length > 1 ? `
                        <div class="flex items-center gap-2">
                            <span class="text-xs text-gray-500 dark:text-gray-400 font-medium">Datastore:</span>
                            <div class="segmented-control inline-flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                                <input type="radio" id="pbs-datastore-all" name="pbs-datastore" value="all" class="hidden peer/all" ${filters.datastore === 'all' ? 'checked' : ''}>
                                <label for="pbs-datastore-all" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 peer-checked/all:bg-gray-100 dark:peer-checked/all:bg-gray-700 peer-checked/all:text-blue-600 dark:peer-checked/all:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">All</label>
                                
                                ${uniqueDatastores.map((datastore, idx) => `
                                    <input type="radio" id="pbs-datastore-${idx}" name="pbs-datastore" value="${datastore}" class="hidden peer/datastore${idx}" ${filters.datastore === datastore ? 'checked' : ''}>
                                    <label for="pbs-datastore-${idx}" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/datastore${idx}:bg-gray-100 dark:peer-checked/datastore${idx}:bg-gray-700 peer-checked/datastore${idx}:text-blue-600 dark:peer-checked/datastore${idx}:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">${datastore}</label>
                                `).join('')}
                            </div>
                        </div>
                    ` : ''}
                    
                    <!-- Type Filter -->
                    <div class="flex flex-wrap items-center gap-2">
                        <span class="text-xs text-gray-500 dark:text-gray-400 font-medium">Type:</span>
                        <div class="segmented-control inline-flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                            <input type="radio" id="pbs-type-all" name="pbs-type" value="all" class="hidden peer/all" ${filters.backupType === 'all' ? 'checked' : ''}>
                            <label for="pbs-type-all" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 peer-checked/all:bg-gray-100 dark:peer-checked/all:bg-gray-700 peer-checked/all:text-blue-600 dark:peer-checked/all:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">All</label>
                            
                            <input type="radio" id="pbs-type-vm" name="pbs-type" value="vm" class="hidden peer/vm" ${filters.backupType === 'vm' ? 'checked' : ''}>
                            <label for="pbs-type-vm" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/vm:bg-gray-100 dark:peer-checked/vm:bg-gray-700 peer-checked/vm:text-blue-600 dark:peer-checked/vm:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">VMs</label>
                            
                            <input type="radio" id="pbs-type-ct" name="pbs-type" value="ct" class="hidden peer/ct" ${filters.backupType === 'ct' ? 'checked' : ''}>
                            <label for="pbs-type-ct" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/ct:bg-gray-100 dark:peer-checked/ct:bg-gray-700 peer-checked/ct:text-blue-600 dark:peer-checked/ct:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">LXCs</label>
                        </div>
                    </div>
                    
                    <!-- Verification Filter -->
                    <div class="flex flex-wrap items-center gap-2">
                        <span class="text-xs text-gray-500 dark:text-gray-400 font-medium">Status:</span>
                        <div class="segmented-control inline-flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                            <input type="radio" id="pbs-verified-all" name="pbs-verified" value="all" class="hidden peer/all" ${filters.verificationStatus === 'all' ? 'checked' : ''}>
                            <label for="pbs-verified-all" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 peer-checked/all:bg-gray-100 dark:peer-checked/all:bg-gray-700 peer-checked/all:text-blue-600 dark:peer-checked/all:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">All</label>
                            
                            <input type="radio" id="pbs-verified-yes" name="pbs-verified" value="verified" class="hidden peer/verified" ${filters.verificationStatus === 'verified' ? 'checked' : ''}>
                            <label for="pbs-verified-yes" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/verified:bg-gray-100 dark:peer-checked/verified:bg-gray-700 peer-checked/verified:text-blue-600 dark:peer-checked/verified:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">Verified</label>
                            
                            <input type="radio" id="pbs-verified-no" name="pbs-verified" value="unverified" class="hidden peer/unverified" ${filters.verificationStatus === 'unverified' ? 'checked' : ''}>
                            <label for="pbs-verified-no" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer bg-white dark:bg-gray-800 border-l border-gray-300 dark:border-gray-600 peer-checked/unverified:bg-gray-100 dark:peer-checked/unverified:bg-gray-700 peer-checked/unverified:text-blue-600 dark:peer-checked/unverified:text-blue-400 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">Unverified</label>
                        </div>
                    </div>
                    
                    
                </div>
            </div>

            <!-- Backups Table -->
            <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded overflow-hidden scrollbar">
                <table class="w-full text-xs sm:text-sm">
                    <thead class="bg-gray-100 dark:bg-gray-800">
                        <tr class="text-[10px] sm:text-xs font-medium tracking-wider text-left text-gray-600 uppercase bg-gray-100 dark:bg-gray-700 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                            ${renderTableHeader()}
                        </tr>
                    </thead>
                    <tbody>
                        ${renderTableRows(filterAndSortBackups())}
                    </tbody>
                </table>
            </div>
        `;
    }

    // Create PBS instance summary card
    function createPBSInstanceCard(instance, instanceIndex) {
        const instanceData = pbsData.filter(backup => backup.instanceIndex === instanceIndex);
        const instanceName = instance.nodeName || instance.pbsInstanceName || `PBS ${instanceIndex + 1}`;
        
        // Calculate statistics for this instance
        let totalSize = 0;
        let verifiedCount = 0;
        let failedTasks = 0;
        let runningTasks = 0;
        const uniqueGuests = new Set();
        
        instanceData.forEach(backup => {
            totalSize += backup.size || 0;
            if (backup.verified) verifiedCount++;
            uniqueGuests.add(backup.vmid);
        });
        
        // Get task stats for this instance
        const now = Date.now() / 1000;
        const last24h = now - (24 * 60 * 60);
        
        if (instance.tasks) {
            instance.tasks.forEach(task => {
                const taskTime = task.starttime || 0;
                if (taskTime >= last24h) {
                    if (!task.status || task.status.toLowerCase().includes('running')) {
                        runningTasks++;
                    } else if (task.status !== 'OK') {
                        failedTasks++;
                    }
                }
            });
        }
        
        const verificationRate = instanceData.length > 0 ? (verifiedCount / instanceData.length) * 100 : 0;
        
        // Get storage info - show all datastores with sizes
        let storageInfo = '';
        let avgDedupFactor = null;
        let totalLogicalData = 0;
        let totalPhysicalData = 0;
        
        if (instance.datastores && instance.datastores.length > 0) {
            // Sort datastores by usage percentage (highest first)
            const sortedDatastores = [...instance.datastores]
                .filter(ds => ds.total && ds.used)
                .sort((a, b) => {
                    const percentA = (a.used / a.total) * 100;
                    const percentB = (b.used / b.total) * 100;
                    return percentB - percentA;
                });
            
            // Calculate average deduplication factor
            sortedDatastores.forEach(ds => {
                if (ds.deduplicationFactor && ds.gcDetails) {
                    const physicalBytes = ds.gcDetails['disk-bytes'] || ds.used;
                    const logicalBytes = ds.gcDetails['index-data-bytes'] || (physicalBytes * ds.deduplicationFactor);
                    totalPhysicalData += physicalBytes;
                    totalLogicalData += logicalBytes;
                }
            });
            
            if (totalPhysicalData > 0 && totalLogicalData > 0) {
                avgDedupFactor = totalLogicalData / totalPhysicalData;
            }
            
            if (sortedDatastores.length > 0) {
                storageInfo = sortedDatastores.map(ds => {
                    const usagePercent = (ds.used / ds.total) * 100;
                    const color = usagePercent >= 90 ? 'red' : usagePercent >= 80 ? 'yellow' : 'green';
                    const progressColorClass = {
                        red: 'bg-red-500/60 dark:bg-red-500/50',
                        yellow: 'bg-yellow-500/60 dark:bg-yellow-500/50',
                        green: 'bg-green-500/60 dark:bg-green-500/50'
                    }[color];
                    
                    const usedFormatted = PulseApp.utils.formatBytesCompact(ds.used);
                    const totalFormatted = PulseApp.utils.formatBytesCompact(ds.total);
                    const displayText = `${usedFormatted}/${totalFormatted}`;
                    
                    const dedupTooltip = ds.deduplicationFactor && ds.deduplicationFactor > 1 
                        ? `Deduplication: ${ds.deduplicationFactor.toFixed(1)}x (${((1 - (1/ds.deduplicationFactor)) * 100).toFixed(0)}% saved)` 
                        : '';
                    
                    return `
                        <div class="text-sm">
                            <div class="flex items-center justify-between mb-0.5">
                                <span class="text-gray-500 dark:text-gray-500 truncate">${ds.name || 'Storage'}:</span>
                                <div class="flex items-center gap-2">
                                    <span class="text-xs font-medium text-gray-800 dark:text-gray-200 whitespace-nowrap">${displayText} (${usagePercent.toFixed(0)}%)</span>
                                    ${ds.deduplicationFactor && ds.deduplicationFactor > 1 ? `<span class="text-xs text-green-600 dark:text-green-400" ${dedupTooltip ? `title="${dedupTooltip}"` : ''}>${ds.deduplicationFactor.toFixed(1)}x</span>` : ''}
                                </div>
                            </div>
                            <div class="relative w-full h-2 rounded-full overflow-hidden bg-gray-200 dark:bg-gray-600">
                                <div class="absolute top-0 left-0 h-full ${progressColorClass} rounded-full" style="width: ${usagePercent}%;"></div>
                            </div>
                        </div>
                    `;
                }).join('');
            }
        }
        
        // Get last prune and verify times from task data
        let lastPruneTime = 0;
        let lastVerifyTime = 0;
        
        if (instance.pruneTasks && instance.pruneTasks.recentTasks) {
            instance.pruneTasks.recentTasks.forEach(task => {
                const taskTime = task.startTime || task.starttime || 0;
                if (task.status === 'OK' && taskTime > lastPruneTime) {
                    lastPruneTime = taskTime;
                }
            });
        }
        
        if (instance.verificationTasks && instance.verificationTasks.recentTasks) {
            instance.verificationTasks.recentTasks.forEach(task => {
                const taskTime = task.startTime || task.starttime || 0;
                if (task.status === 'OK' && taskTime > lastVerifyTime) {
                    lastVerifyTime = taskTime;
                }
            });
        }
        
        // Format times
        const formatTaskTime = (timestamp) => {
            if (!timestamp) return 'Never';
            const diff = now - timestamp;
            if (diff < 3600) return Math.floor(diff / 60) + 'm ago';
            if (diff < 86400) return Math.floor(diff / 3600) + 'h ago';
            return Math.floor(diff / 86400) + 'd ago';
        };
        
        const getTaskColorClass = (timestamp, warningDays = 7) => {
            if (!timestamp) return 'text-gray-500 dark:text-gray-400';
            const daysSince = (now - timestamp) / 86400;
            if (daysSince <= 1) return 'text-green-600 dark:text-green-400';
            if (daysSince <= 3) return 'text-green-600 dark:text-green-400';
            if (daysSince <= warningDays) return 'text-yellow-600 dark:text-yellow-400';
            if (daysSince <= 30) return 'text-orange-600 dark:text-orange-400';
            return 'text-red-600 dark:text-red-400';
        };
        
        const isActive = instanceIndex === activeInstance;
        
        // Get CPU and memory usage if available
        let cpuUsage = null;
        let memUsage = null;
        if (instance.nodeStatus && instance.nodeStatus.cpu !== null && instance.nodeStatus.cpu !== undefined) {
            // CPU might already be a percentage (0-100) or a decimal (0-1)
            cpuUsage = instance.nodeStatus.cpu > 1 ? Math.round(instance.nodeStatus.cpu) : Math.round(instance.nodeStatus.cpu * 100);
        }
        if (instance.nodeStatus && instance.nodeStatus.memory) {
            const mem = instance.nodeStatus.memory;
            if (mem.total && mem.used) {
                memUsage = Math.round((mem.used / mem.total) * 100);
            }
        }
        
        return `
            <div class="bg-white dark:bg-gray-800 shadow-md rounded-lg p-3 border border-gray-200 dark:border-gray-700 flex-1 min-w-0 sm:min-w-[280px]">
                <div class="flex justify-between items-center mb-2">
                    <h3 class="text-base font-semibold text-gray-800 dark:text-gray-200">${instanceName}</h3>
                    <div class="flex items-center gap-3">
                        ${failedTasks > 0 ? `<span class="text-xs font-medium text-red-600 dark:text-red-400" title="${failedTasks} failed tasks in last 24h">● ${failedTasks}</span>` : ''}
                        ${runningTasks > 0 ? `<span class="text-xs font-medium text-blue-600 dark:text-blue-400 animate-pulse" title="${runningTasks} running tasks">▶ ${runningTasks}</span>` : ''}
                    </div>
                </div>
                <div class="space-y-1 text-sm">
                    <div class="flex justify-between">
                        <div class="flex gap-2">
                            <span class="text-gray-500 dark:text-gray-500">Backups:</span>
                            <span class="font-semibold text-gray-800 dark:text-gray-200">${instanceData.length}</span>
                        </div>
                        <div class="flex gap-2">
                            <span class="text-gray-500 dark:text-gray-500">Verified:</span>
                            <span class="font-semibold ${verificationRate === 100 ? 'text-green-600 dark:text-green-400' : verificationRate >= 80 ? 'text-blue-600 dark:text-blue-400' : 'text-yellow-600 dark:text-yellow-400'}">${verificationRate.toFixed(0)}%</span>
                        </div>
                    </div>
                    <div class="flex justify-between">
                        <div class="flex gap-2">
                            <span class="text-gray-500 dark:text-gray-500">Pruned:</span>
                            <span class="font-semibold ${getTaskColorClass(lastPruneTime, 30)}">${formatTaskTime(lastPruneTime)}</span>
                        </div>
                        <div class="flex gap-2">
                            <span class="text-gray-500 dark:text-gray-500">Verify:</span>
                            <span class="font-semibold ${getTaskColorClass(lastVerifyTime, 7)}">${formatTaskTime(lastVerifyTime)}</span>
                        </div>
                    </div>
                </div>
                ${storageInfo ? `<div class="mt-2 pt-2 border-t border-gray-200 dark:border-gray-700 space-y-1">${storageInfo}</div>` : ''}
            </div>
        `;
    }

    // Render deduplication explanation
    function renderDedupExplanation() {
        // Removed - no longer showing per-backup deduplication estimates
        return '';
    }

    // Render backup summary
    function renderBackupSummary() {
        const backups = getActiveInstanceData();
        const stats = getActiveInstanceStats();
        
        if (backups.length === 0) {
            return '';
        }
        
        // Calculate statistics
        let totalSize = 0;
        let verifiedCount = 0;
        let protectedCount = 0;
        const uniqueGuests = new Set();
        
        backups.forEach(backup => {
            totalSize += backup.size || 0;
            if (backup.verified) verifiedCount++;
            if (backup.protected) protectedCount++;
            uniqueGuests.add(backup.vmid);
        });
        
        const verificationRate = backups.length > 0 ? Math.round((verifiedCount / backups.length) * 100) : 0;
        
        // Format last backup time
        let lastBackupText = 'Never';
        if (stats.lastBackupTime) {
            const now = Date.now() / 1000;
            const diff = now - stats.lastBackupTime;
            if (diff < 3600) {
                lastBackupText = Math.floor(diff / 60) + 'm ago';
            } else if (diff < 86400) {
                lastBackupText = Math.floor(diff / 3600) + 'h ago';
            } else {
                lastBackupText = Math.floor(diff / 86400) + 'd ago';
            }
        }
        
        // Calculate storage usage percentage
        let storageUsedPercent = 0;
        if (stats.datastoreStats && stats.datastoreStats.total > 0) {
            storageUsedPercent = Math.round((stats.datastoreStats.used / stats.datastoreStats.total) * 100);
        }
        
        // Format storage display
        let storageDisplay = '';
        if (stats.totalDatastores > 0) {
            if (stats.totalDatastores > 1) {
                // Multiple datastores - just show count
                storageDisplay = `<span><span class="font-semibold text-gray-800 dark:text-gray-100">${stats.totalDatastores}</span> stores</span>`;
            } else if (stats.datastoreStats && stats.datastoreStats.total > 0) {
                // Single datastore - show progress bar
                const usedFormatted = formatBytes(stats.datastoreStats.used);
                const totalFormatted = formatBytes(stats.datastoreStats.total);
                const displayText = `${usedFormatted.text} / ${totalFormatted.text}`;
                const usageColorClass = storageUsedPercent <= 50 ? 'green' : storageUsedPercent <= 80 ? 'yellow' : 'red';
                
                // Create a custom progress bar that fits the content
                const progressColorClass = {
                    'red': 'bg-red-500/60 dark:bg-red-500/50',
                    'yellow': 'bg-yellow-500/60 dark:bg-yellow-500/50',
                    'green': 'bg-green-500/60 dark:bg-green-500/50'
                }[usageColorClass] || 'bg-gray-500/60 dark:bg-gray-500/50';
                
                storageDisplay = `
                    <div class="inline-flex items-center" style="white-space: nowrap;">
                        <div class="relative h-3.5 rounded overflow-hidden bg-gray-200 dark:bg-gray-600" style="min-width: fit-content;">
                            <div class="absolute top-0 left-0 h-full ${progressColorClass}" style="width: ${storageUsedPercent}%;"></div>
                            <span class="relative flex items-center justify-center h-full px-2 text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
                                ${displayText} (${storageUsedPercent}%)
                            </span>
                        </div>
                    </div>
                `;
            }
        }
        
        // Don't show inline summary since we're using cards
        return '';
    }

    // Render table header
    function renderTableHeader() {
        const headers = [
            { field: 'status', label: 'Status', width: 'w-12', center: true },
            { field: 'vmid', label: 'VMID', width: 'w-16' },
            { field: 'comment', label: 'Name/Comment', width: '' },
            { field: 'backupType', label: 'Type', width: 'w-16' },
            { field: 'backupTime', label: 'Backup Time', width: 'w-24' },
            { field: 'datastore', label: 'Datastore', width: 'w-20' },
            { field: 'namespace', label: 'Namespace', width: 'w-20' },
            { field: 'size', label: 'Size', width: 'w-28' }
        ];
        
        return headers.map(header => {
            const isActive = currentSort.field === header.field;
            const sortIcon = isActive ? (currentSort.ascending ? '↑' : '↓') : '';
            const sortable = header.field !== 'status' && header.field !== 'comment';
            
            return `
                <th class="${sortable ? 'sortable' : ''} p-1 px-2 whitespace-nowrap ${header.center ? 'text-center' : ''}" 
                    ${sortable ? `onclick="PulseApp.ui.pbs.sortTable('${header.field}')"` : ''}>
                    ${header.label} ${sortIcon}
                </th>
            `;
        }).join('');
    }

    // Render table rows grouped by date
    function renderTableRows(filteredBackups) {
        if (filteredBackups.length === 0) {
            return `
                <tr>
                    <td colspan="8" class="p-4 text-center text-gray-500 dark:text-gray-400">
                        No backups found
                    </td>
                </tr>
            `;
        }
        
        // Group backups by date
        const groupedBackups = {};
        filteredBackups.forEach(backup => {
            const dateKey = formatDateKey(backup.backupTime);
            if (!groupedBackups[dateKey]) {
                groupedBackups[dateKey] = [];
            }
            groupedBackups[dateKey].push(backup);
        });
        
        // Sort dates (newest first)
        const sortedDates = Object.keys(groupedBackups).sort().reverse();
        
        let html = '';
        sortedDates.forEach(dateKey => {
            const backups = groupedBackups[dateKey];
            const displayDate = formatDateDisplay(dateKey);
            
            // Add date header
            html += `
                <tr class="bg-gray-50 dark:bg-gray-700/50">
                    <td colspan="8" class="p-1 px-2 text-xs font-medium text-gray-600 dark:text-gray-400">
                        ${displayDate} (${backups.length} backup${backups.length > 1 ? 's' : ''})
                    </td>
                </tr>
            `;
            
            // Add backups for this date
            backups.forEach(backup => {
                const size = formatBytes(backup.size || 0);
                const typeLabel = backup.backupType === 'vm' ? 'VM' : 'LXC';
                const guestName = getGuestName(backup.vmid, backup.backupType);
                
                // Show logical size only
                const sizeHtml = `<span class="${getSizeColorClass(backup.size)}">${size.text}</span>`;
                
                html += `
                    <tr class="border-t border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/30">
                        <td class="p-1 px-2 whitespace-nowrap text-center">
                            <span class="inline-flex items-center justify-center w-4 h-4" title="${backup.verified ? 'Backup verified' : 'Not verified'}">
                                ${backup.verified ? `
                                    <svg class="w-3 h-3 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"></path>
                                    </svg>
                                ` : `
                                    <svg class="w-3 h-3 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z" clip-rule="evenodd"></path>
                                    </svg>
                                `}
                            </span>
                        </td>
                        <td class="p-1 px-2 whitespace-nowrap font-medium">${backup.vmid}</td>
                        <td class="p-1 px-2 max-w-[200px] truncate" title="${guestName}${backup.comment ? ' - ' + backup.comment : ''}">
                            ${guestName}${backup.comment ? ' <span class="text-gray-500 dark:text-gray-400">- ' + backup.comment + '</span>' : ''}
                        </td>
                        <td class="p-1 px-2 whitespace-nowrap">
                            <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${typeLabel === 'VM' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400' : 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'}">
                                ${typeLabel}
                            </span>
                        </td>
                        <td class="p-1 px-2 whitespace-nowrap text-xs ${getTimeAgoColorClass(backup.backupTime)}" title="${formatBackupTime(backup.backupTime)}">${formatTimeAgo(backup.backupTime)}</td>
                        <td class="p-1 px-2 whitespace-nowrap">${backup.datastore}</td>
                        <td class="p-1 px-2 whitespace-nowrap">${backup.namespace}</td>
                        <td class="p-1 px-2 whitespace-nowrap" style="width: 112px; text-align: right;">
                            ${sizeHtml}
                        </td>
                    </tr>
                `;
            });
        });
        
        return html;
    }

    // Get guest name from VM/CT data
    function getGuestName(vmid, type) {
        const vms = PulseApp.state?.get?.('vmsData') || [];
        const containers = PulseApp.state?.get?.('containersData') || [];
        
        const vmidStr = String(vmid);
        
        if (type === 'vm') {
            const vm = vms.find(v => String(v.vmid) === vmidStr);
            return vm ? vm.name : vmidStr;
        } else if (type === 'ct') {
            const ct = containers.find(c => String(c.vmid) === vmidStr);
            return ct ? ct.name : vmidStr;
        }
        
        return vmidStr;
    }

    // Filter and sort backups
    function filterAndSortBackups() {
        // Start with only active instance data
        let filtered = getActiveInstanceData();
        
        // Apply other filters
        if (filters.searchTerm) {
            const search = filters.searchTerm.toLowerCase();
            filtered = filtered.filter(backup => 
                backup.vmid.toString().includes(search) ||
                (backup.comment && backup.comment.toLowerCase().includes(search)) ||
                (backup.namespace && backup.namespace.toLowerCase().includes(search)) ||
                getGuestName(backup.vmid, backup.backupType).toLowerCase().includes(search)
            );
        }
        
        if (filters.datastore !== 'all') {
            filtered = filtered.filter(backup => backup.datastore === filters.datastore);
        }
        
        if (filters.backupType !== 'all') {
            filtered = filtered.filter(backup => backup.backupType === filters.backupType);
        }
        
        if (filters.verificationStatus !== 'all') {
            filtered = filtered.filter(backup => {
                if (filters.verificationStatus === 'verified') return backup.verified;
                if (filters.verificationStatus === 'unverified') return !backup.verified;
                return true;
            });
        }
        
        // Apply sorting
        filtered.sort((a, b) => {
            let aVal = a[currentSort.field];
            let bVal = b[currentSort.field];
            
            // Handle numeric fields
            if (currentSort.field === 'vmid' || currentSort.field === 'size' || currentSort.field === 'backupTime') {
                aVal = parseInt(aVal) || 0;
                bVal = parseInt(bVal) || 0;
            }
            
            if (aVal < bVal) return currentSort.ascending ? -1 : 1;
            if (aVal > bVal) return currentSort.ascending ? 1 : -1;
            return 0;
        });
        
        return filtered;
    }

    // Event listeners
    function setupEventListeners() {
        // Instance tabs
        document.querySelectorAll('.pbs-instance-tab').forEach(tab => {
            tab.addEventListener('click', (e) => {
                const instanceIndex = parseInt(e.target.getAttribute('data-instance'));
                switchInstance(instanceIndex);
            });
        });
        
        const searchInput = document.getElementById('pbs-search');
        const resetButton = document.getElementById('reset-pbs-button');
        
        // Search input
        if (searchInput) {
            searchInput.addEventListener('input', (e) => {
                filters.searchTerm = e.target.value;
                updateTable();
                updateResetButtonState();
            });
            
            // ESC key handler
            searchInput.addEventListener('keydown', (e) => {
                if (e.key === 'Escape') {
                    resetFiltersAndSort();
                }
            });
        }
        
        // Reset button
        if (resetButton) {
            resetButton.addEventListener('click', resetFiltersAndSort);
        }
        
        // Radio button filters
        document.querySelectorAll('input[name="pbs-server"], input[name="pbs-datastore"], input[name="pbs-type"], input[name="pbs-verified"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                const filterName = e.target.name.replace('pbs-', '');
                if (filterName === 'server') {
                    filters.server = e.target.value;
                } else if (filterName === 'datastore') {
                    filters.datastore = e.target.value;
                } else if (filterName === 'type') {
                    filters.backupType = e.target.value;
                } else if (filterName === 'verified') {
                    filters.verificationStatus = e.target.value;
                }
                
                // Always re-render the full UI for any filter change to ensure consistency
                renderPBSUI();
            });
        });
        
        // Set up keyboard navigation for auto-focus search
        setupKeyboardNavigation();
    }

    // Setup keyboard navigation to auto-focus search
    function setupKeyboardNavigation() {
        // Remove any existing listener to avoid duplicates
        if (window.pbsKeyboardHandler) {
            document.removeEventListener('keydown', window.pbsKeyboardHandler);
        }
        
        // Define the handler
        window.pbsKeyboardHandler = (event) => {
            // Only handle if PBS-temp tab is active
            const activeTab = document.querySelector('.tab.active');
            if (!activeTab || activeTab.getAttribute('data-tab') !== 'pbs') {
                return;
            }
            
            const searchInput = document.getElementById('pbs-search');
            if (!searchInput) return;
            
            // Handle Escape for resetting filters
            if (event.key === 'Escape') {
                const resetButton = document.getElementById('reset-pbs-button');
                if (resetButton) {
                    resetButton.click();
                }
                return;
            }
            
            // Ignore if already typing in an input, textarea, or select
            const targetElement = event.target;
            const targetTagName = targetElement.tagName;
            if (targetTagName === 'INPUT' || targetTagName === 'TEXTAREA' || targetTagName === 'SELECT') {
                return;
            }
            
            // Ignore if any modal is open
            const modals = document.querySelectorAll('.modal:not(.hidden)');
            if (modals.length > 0) {
                return;
            }
            
            // For single character keys (letters, numbers, etc.)
            if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
                if (document.activeElement !== searchInput) {
                    searchInput.focus();
                    event.preventDefault();
                    searchInput.value += event.key;
                    searchInput.dispatchEvent(new Event('input', { bubbles: true, cancelable: true }));
                }
            } 
            // For Backspace
            else if (event.key === 'Backspace' && !event.ctrlKey && !event.metaKey && !event.altKey) {
                if (document.activeElement !== searchInput) {
                    searchInput.focus();
                    event.preventDefault();
                }
            }
        };
        
        // Add the listener
        document.addEventListener('keydown', window.pbsKeyboardHandler);
    }

    // Update table without full re-render
    function updateTable() {
        const tbody = document.querySelector('#pbs-content tbody');
        if (tbody) {
            tbody.innerHTML = renderTableRows(filterAndSortBackups());
        }
    }

    // Sort table
    function sortTable(field) {
        if (currentSort.field === field) {
            currentSort.ascending = !currentSort.ascending;
        } else {
            currentSort.field = field;
            currentSort.ascending = false;
        }
        renderPBSUI();
    }

    // Reset filters and sort
    function resetFiltersAndSort() {
        const searchInput = document.getElementById('pbs-search');
        if (searchInput) {
            searchInput.value = '';
        }
        
        // Reset filters
        filters = {
            searchTerm: '',
            datastore: 'all',
            backupType: 'all',
            verificationStatus: 'all'
        };
        
        // Reset sort
        currentSort.field = 'backupTime';
        currentSort.ascending = false;
        
        // Re-render the UI to reset all radio buttons
        renderPBSUI();
    }

    // Update reset button state
    function updateResetButtonState() {
        const hasFilters = hasActiveFilters();
        const resetButton = document.getElementById('reset-pbs-button');
        
        if (resetButton) {
            if (hasFilters) {
                resetButton.classList.remove('opacity-50', 'cursor-not-allowed');
                resetButton.classList.add('hover:bg-gray-100', 'dark:hover:bg-gray-700');
                resetButton.disabled = false;
            } else {
                resetButton.classList.add('opacity-50', 'cursor-not-allowed');
                resetButton.classList.remove('hover:bg-gray-100', 'dark:hover:bg-gray-700');
                resetButton.disabled = true;
            }
        }
    }

    // Check if any filters are active
    function hasActiveFilters() {
        const isDefaultSort = currentSort.field === 'backupTime' && !currentSort.ascending;
        
        return filters.searchTerm !== '' ||
               filters.server !== 'all' ||
               filters.datastore !== 'all' ||
               filters.backupType !== 'all' ||
               filters.verificationStatus !== 'all' ||
               !isDefaultSort;
    }

    // Helper functions
    function getUniqueValues(field) {
        const values = new Set();
        pbsData.forEach(backup => {
            if (backup[field]) values.add(backup[field]);
        });
        return Array.from(values).sort();
    }

    // Format date for grouping (YYYY-MM-DD)
    function formatDateKey(timestamp) {
        const date = new Date(timestamp * 1000);
        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const day = String(date.getDate()).padStart(2, '0');
        return `${year}-${month}-${day}`;
    }

    // Format date for display
    function formatDateDisplay(dateKey) {
        const [year, month, day] = dateKey.split('-');
        const date = new Date(year, month - 1, day);
        
        // Check if it's today or yesterday
        const today = new Date();
        const yesterday = new Date(today);
        yesterday.setDate(yesterday.getDate() - 1);
        
        if (dateKey === formatDateKey(today.getTime() / 1000)) {
            return 'Today';
        } else if (dateKey === formatDateKey(yesterday.getTime() / 1000)) {
            return 'Yesterday';
        }
        
        // Otherwise return formatted date
        return date.toLocaleDateString(undefined, { 
            weekday: 'short',
            day: 'numeric',
            month: 'short',
            year: 'numeric'
        });
    }

    function formatBytes(bytes) {
        if (bytes === 0) return { value: 0, unit: 'B', text: '0 B' };
        
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        const value = parseFloat((bytes / Math.pow(k, i)).toFixed(2));
        
        return {
            value: value,
            unit: sizes[i],
            text: `${value} ${sizes[i]}`
        };
    }
    
    function formatTimeAgo(timestamp) {
        if (!timestamp) return 'Never';
        
        const now = Date.now() / 1000;
        const diff = now - timestamp;
        
        if (diff < 3600) {
            return Math.floor(diff / 60) + 'm ago';
        } else if (diff < 86400) {
            return Math.floor(diff / 3600) + 'h ago';
        } else {
            return Math.floor(diff / 86400) + 'd ago';
        }
    }
    
    // Format backup time
    function formatBackupTime(timestamp) {
        const date = new Date(timestamp * 1000);
        return date.toLocaleString(undefined, { 
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });
    }
    
    // Get color class based on time ago
    function getTimeAgoColorClass(timestamp) {
        if (!timestamp) return 'text-gray-600 dark:text-gray-400';
        
        const now = Date.now() / 1000;
        const diff = now - timestamp;
        const days = diff / 86400;
        
        if (days < 1) {
            // Less than 1 day - green (fresh)
            return 'text-green-600 dark:text-green-400';
        } else if (days < 3) {
            // 1-3 days - green (still recent)
            return 'text-green-600 dark:text-green-400';
        } else if (days < 7) {
            // 3-7 days - yellow (getting old)
            return 'text-yellow-600 dark:text-yellow-400';
        } else if (days < 30) {
            // 7-30 days - orange (old)
            return 'text-orange-600 dark:text-orange-400';
        } else {
            // Over 30 days - red (very old)
            return 'text-red-600 dark:text-red-400';
        }
    }
    
    // Get color class based on backup size
    function getSizeColorClass(sizeInBytes) {
        const gb = sizeInBytes / (1024 * 1024 * 1024);
        
        if (gb < 1) {
            // Less than 1 GB - green (small)
            return 'text-green-600 dark:text-green-400';
        } else if (gb < 5) {
            // 1-5 GB - green (still small)
            return 'text-green-600 dark:text-green-400';
        } else if (gb < 20) {
            // 5-20 GB - yellow (medium)
            return 'text-yellow-600 dark:text-yellow-400';
        } else if (gb < 50) {
            // 20-50 GB - orange (large)
            return 'text-orange-600 dark:text-orange-400';
        } else {
            // 50+ GB - red (very large)
            return 'text-red-600 dark:text-red-400';
        }
    }

    // Public API
    return {
        init,
        updatePBSInfo,
        sortTable,
        switchInstance
    };
})();