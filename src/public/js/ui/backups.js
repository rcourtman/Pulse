PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.backups = (() => {
    let isInitialized = false;
    let currentFilters = {
        searchTerm: '',
        backupType: 'all', // 'all', 'pve', 'pbs'
        node: 'all',
        selectedDate: null // YYYY-MM-DD format when a day is clicked
    };
    let currentChartType = 'count'; // 'count' or 'storage'
    const currentGrouping = 'date'; // Always group by date
    let currentSort = {
        field: 'ctime',
        ascending: false
    };
    let backupsData = {
        unified: [],
        pbsEnabled: false,
        pbsStorageInfo: null
    };

    function init() {
        if (isInitialized) return;
        isInitialized = true;
        
        updateBackupsInfo();
    }

    function updateBackupsInfo() {
        const container = document.getElementById('backups-content');
        if (!container) return;

        // Only show loading state on initial load
        if (!isInitialized || backupsData.unified.length === 0) {
            container.innerHTML = `
                <div class="p-4 text-center text-gray-500 dark:text-gray-400">
                    Loading backups...
                </div>
            `;
        }

        // Fetch unified backups data
        fetch('/api/backups/unified')
            .then(response => response.json())
            .then(data => {
                const newBackups = data.backups || [];
                const newPbsEnabled = data.pbs?.enabled || false;
                const newPbsStorageInfo = data.pbs?.storageInfo || null;
                
                // Check if data has actually changed
                const hasChanged = JSON.stringify(newBackups) !== JSON.stringify(backupsData.unified) ||
                                 newPbsEnabled !== backupsData.pbsEnabled ||
                                 JSON.stringify(newPbsStorageInfo) !== JSON.stringify(backupsData.pbsStorageInfo);
                
                if (hasChanged) {
                    backupsData.unified = newBackups;
                    backupsData.pbsEnabled = newPbsEnabled;
                    backupsData.pbsStorageInfo = newPbsStorageInfo;
                    
                    // Only render full UI on first load
                    if (container.querySelector('.overflow-x-auto')) {
                        // Update only the table body
                        const tbody = container.querySelector('tbody');
                        if (tbody) {
                            tbody.innerHTML = renderBackupRows();
                        }
                        
                        // Update summary
                        const summary = calculateSummary();
                        const summaryElements = container.querySelectorAll('.text-xl.font-semibold');
                        if (summaryElements.length >= 3) {
                            summaryElements[0].textContent = summary.total;
                            summaryElements[1].textContent = summary.pve;
                            if (backupsData.pbsEnabled && summaryElements.length >= 4) {
                                summaryElements[2].textContent = summary.pbs;
                                summaryElements[3].textContent = formatBytes(summary.totalSize).text;
                            } else {
                                summaryElements[2].textContent = formatBytes(summary.totalSize).text;
                            }
                        }
                        
                        // Update chart
                        renderBackupTrendChart();
                        
                        // Update summary
                        updateSummary();
                    } else {
                        // First load - render full UI
                        renderBackupsUI();
                    }
                }
            })
            .catch(error => {
                console.error('Error fetching backups:', error);
                // Only show error if we don't have data already
                if (backupsData.unified.length === 0) {
                    container.innerHTML = `
                        <div class="p-8 text-center">
                            <div class="text-red-500 dark:text-red-400">
                                Failed to load backups data: ${error.message}
                            </div>
                            <button onclick="PulseApp.ui.backups.updateBackupsInfo()" class="mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
                                Retry
                            </button>
                        </div>
                    `;
                }
            });
    }

    function renderBackupsUI() {
        const container = document.getElementById('backups-content');
        if (!container) return;

        const summary = calculateSummary();
        const uniqueNodes = getUniqueNodes();

        container.innerHTML = `
            <!-- Backup Trend Chart -->
            <div class="mb-3 p-4 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded shadow-sm">
                <div class="mb-3 flex items-center justify-between">
                    <div class="flex items-center gap-4">
                        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">Backup History</h3>
                        <!-- Chart type tabs -->
                        <div class="flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                            <button class="chart-tab px-3 py-1 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-blue-600 dark:text-blue-400" data-chart="count">
                                Count
                            </button>
                            <button class="chart-tab px-3 py-1 text-xs font-medium bg-white dark:bg-gray-800 text-gray-600 dark:text-gray-400 border-l border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700" data-chart="storage">
                                Storage
                            </button>
                        </div>
                    </div>
                    <div id="chart-filter-indicator" class="text-xs text-gray-500 dark:text-gray-400"></div>
                </div>
                <div id="backup-trend-chart" class="h-48 relative" style="min-height: 12rem;">
                    <div class="absolute inset-0 flex items-center justify-center text-gray-400 dark:text-gray-500">
                        <span class="text-sm">Loading chart...</span>
                    </div>
                </div>
            </div>

            <!-- Summary -->
            <div class="mb-3 p-3 bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-700 rounded">
                <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                    <div>
                        <div class="flex items-center gap-1 text-gray-500 dark:text-gray-400">
                            <i class="fas fa-box text-xs"></i>
                            Total Backups
                        </div>
                        <div class="text-xl font-semibold">${summary.total}</div>
                        <div class="text-xs text-gray-600 dark:text-gray-400 mt-0.5">
                            ${summary.lastBackup}
                        </div>
                    </div>
                    <div>
                        <div class="flex items-center gap-1 text-gray-500 dark:text-gray-400">
                            <i class="fas fa-circle text-orange-500 text-xs"></i>
                            PVE Backups
                        </div>
                        <div class="text-xl font-semibold">${summary.pve}</div>
                        <div class="text-xs text-gray-600 dark:text-gray-400 mt-0.5">
                            ${summary.successRate}% success
                        </div>
                    </div>
                    ${backupsData.pbsEnabled ? `
                        <div>
                            <div class="flex items-center gap-1 text-gray-500 dark:text-gray-400">
                                <i class="fas fa-circle text-purple-500 text-xs"></i>
                                PBS Backups
                            </div>
                            <div class="text-xl font-semibold">${summary.pbs}</div>
                            <div class="text-xs text-green-600 dark:text-green-400 mt-0.5">
                                ${summary.verifiedCount} verified
                            </div>
                        </div>
                    ` : ''}
                    <div>
                        <div class="flex items-center gap-1 text-gray-500 dark:text-gray-400">
                            <i class="fas fa-hard-drive text-xs"></i>
                            ${summary.pbsDedupInfo && summary.pbs > 0 && !currentFilters.selectedDate ? 'Disk Usage' : 'Total Size'}
                        </div>
                        <div class="text-xl font-semibold">
                            ${summary.pbsDedupInfo && summary.pbs > 0 && !currentFilters.selectedDate 
                                ? summary.pbsDedupInfo.actualSize 
                                : formatBytes(summary.totalSize).text}
                        </div>
                        ${summary.pbsDedupInfo && summary.pbs > 0 && !currentFilters.selectedDate ? `
                            <div class="text-xs text-green-600 dark:text-green-400 mt-0.5">
                                ${summary.pbsDedupInfo.ratio} dedup
                            </div>
                        ` : ''}
                        <div class="text-xs text-gray-600 dark:text-gray-400 mt-0.5">
                            ${summary.growthRate}
                        </div>
                    </div>
                </div>
            </div>

            <!-- Filters -->
            <div class="mb-3 p-2 bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-700 rounded">
                <div class="flex flex-wrap items-center gap-3">
                    <input type="search" id="backup-search" placeholder="Search by VMID or notes..." 
                        value="${currentFilters.searchTerm}"
                        class="flex-1 min-w-[200px] p-1 px-2 h-7 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 focus:ring-1 focus:ring-blue-500 focus:border-blue-500 outline-none">
                    
                    <div class="flex items-center gap-2">
                        <span class="text-xs text-gray-500 dark:text-gray-400 font-medium">Type:</span>
                        <div class="segmented-control inline-flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                            <input type="radio" id="backup-type-all" name="backup-type" value="all" class="hidden peer" ${currentFilters.backupType === 'all' ? 'checked' : ''}>
                            <label for="backup-type-all" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer ${currentFilters.backupType === 'all' ? 'bg-gray-100 dark:bg-gray-700 text-blue-600 dark:text-blue-400' : 'bg-white dark:bg-gray-800'} hover:bg-gray-50 dark:hover:bg-gray-700 select-none">All</label>
                            
                            <input type="radio" id="backup-type-pve" name="backup-type" value="pve" class="hidden peer" ${currentFilters.backupType === 'pve' ? 'checked' : ''}>
                            <label for="backup-type-pve" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer ${currentFilters.backupType === 'pve' ? 'bg-gray-100 dark:bg-gray-700 text-blue-600 dark:text-blue-400' : 'bg-white dark:bg-gray-800'} border-l border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">PVE</label>
                            
                            ${backupsData.pbsEnabled ? `
                                <input type="radio" id="backup-type-pbs" name="backup-type" value="pbs" class="hidden peer" ${currentFilters.backupType === 'pbs' ? 'checked' : ''}>
                                <label for="backup-type-pbs" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer ${currentFilters.backupType === 'pbs' ? 'bg-gray-100 dark:bg-gray-700 text-blue-600 dark:text-blue-400' : 'bg-white dark:bg-gray-800'} border-l border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">PBS</label>
                            ` : ''}
                        </div>
                    </div>
                    
                    ${uniqueNodes.length > 1 ? `
                        <div class="flex items-center gap-2">
                            <span class="text-xs text-gray-500 dark:text-gray-400 font-medium">Node:</span>
                            <div class="segmented-control inline-flex border border-gray-300 dark:border-gray-600 rounded overflow-hidden">
                                <input type="radio" id="node-all" name="backup-node" value="all" class="hidden peer" ${currentFilters.node === 'all' ? 'checked' : ''}>
                                <label for="node-all" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer ${currentFilters.node === 'all' ? 'bg-gray-100 dark:bg-gray-700 text-blue-600 dark:text-blue-400' : 'bg-white dark:bg-gray-800'} hover:bg-gray-50 dark:hover:bg-gray-700 select-none">All</label>
                                
                                ${uniqueNodes.map((node, idx) => `
                                    <input type="radio" id="node-${node}" name="backup-node" value="${node}" class="hidden peer" ${currentFilters.node === node ? 'checked' : ''}>
                                    <label for="node-${node}" class="flex items-center justify-center px-3 py-1 text-xs cursor-pointer ${currentFilters.node === node ? 'bg-gray-100 dark:bg-gray-700 text-blue-600 dark:text-blue-400' : 'bg-white dark:bg-gray-800'} border-l border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 select-none">${node}</label>
                                `).join('')}
                            </div>
                        </div>
                    ` : ''}
                </div>
            </div>
            

            <!-- Backups Table -->
            <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded overflow-hidden scrollbar">
                <table class="w-full text-xs sm:text-sm">
                    <thead class="bg-gray-100 dark:bg-gray-800">
                        <tr class="text-[10px] sm:text-xs font-medium tracking-wider text-left text-gray-600 uppercase bg-gray-100 dark:bg-gray-700 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                            <th class="p-1 px-2 whitespace-nowrap text-center">Status</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="vmid">VMID</th>
                            <th class="p-1 px-2 whitespace-nowrap">Name/Notes</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="type">Type</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="node">Node</th>
                            <th class="p-1 px-2 whitespace-nowrap">Storage</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="size">Size</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="ctime">Age</th>
                            <th class="sortable p-1 px-2 whitespace-nowrap" data-sort="source">Source</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${renderBackupRows()}
                    </tbody>
                </table>
            </div>
        `;

        // Setup event listeners
        setupEventListeners();
        
        // Setup sortable headers
        const table = document.querySelector('#backups-content table');
        if (table) {
            table.id = 'backups-table';
            table.querySelectorAll('th.sortable').forEach(th => {
                th.style.cursor = 'pointer';
                th.addEventListener('click', () => {
                    const field = th.getAttribute('data-sort');
                    if (field) sortBy(field);
                });
            });
        }
        
        // Render the chart after a small delay to ensure data is loaded
        setTimeout(() => {
            renderBackupTrendChart();
            updateSummary(); // Update summary to add deduplication info
        }, 100);
    }

    function updateSummary() {
        const container = document.getElementById('backups-content');
        if (!container) return;
        
        const summary = calculateSummary();
        
        // Find and update each summary card
        const summaryCards = container.querySelectorAll('.grid > div');
        
        // Total backups
        if (summaryCards[0]) {
            const totalValue = summaryCards[0].querySelector('.text-xl.font-semibold');
            const lastBackupText = summaryCards[0].querySelector('.text-xs.text-gray-600');
            if (totalValue) totalValue.textContent = summary.total;
            if (lastBackupText) lastBackupText.textContent = summary.lastBackup;
        }
        
        // PVE backups
        if (summaryCards[1]) {
            const pveValue = summaryCards[1].querySelector('.text-xl.font-semibold');
            const successText = summaryCards[1].querySelector('.text-xs.text-gray-600');
            if (pveValue) pveValue.textContent = summary.pve;
            if (successText) successText.textContent = `${summary.successRate}% success`;
        }
        
        // PBS backups (if enabled)
        let sizeCardIndex = 2;
        if (backupsData.pbsEnabled && summaryCards[2]) {
            const pbsValue = summaryCards[2].querySelector('.text-xl.font-semibold');
            const verifiedText = summaryCards[2].querySelector('.text-xs.text-green-600');
            if (pbsValue) pbsValue.textContent = summary.pbs;
            if (verifiedText) verifiedText.textContent = `${summary.verifiedCount} verified`;
            sizeCardIndex = 3;
        }
        
        // Size card - update both label and value
        if (summaryCards[sizeCardIndex]) {
            const sizeLabel = summaryCards[sizeCardIndex].querySelector('.text-gray-500');
            const sizeValue = summaryCards[sizeCardIndex].querySelector('.text-xl.font-semibold');
            const dedupText = summaryCards[sizeCardIndex].querySelector('.text-xs.text-green-600');
            const growthText = summaryCards[sizeCardIndex].querySelectorAll('.text-xs')[1] || summaryCards[sizeCardIndex].querySelector('.text-xs.text-gray-600');
            
            if (sizeLabel) {
                const labelText = sizeLabel.childNodes[sizeLabel.childNodes.length - 1];
                labelText.textContent = summary.pbsDedupInfo && summary.pbs > 0 && !currentFilters.selectedDate ? 'Disk Usage' : 'Total Size';
            }
            
            if (sizeValue) {
                sizeValue.textContent = summary.pbsDedupInfo && summary.pbs > 0 && !currentFilters.selectedDate 
                    ? summary.pbsDedupInfo.actualSize 
                    : formatBytes(summary.totalSize).text;
            }
            
            if (dedupText) {
                if (summary.pbsDedupInfo && summary.pbs > 0 && !currentFilters.selectedDate) {
                    dedupText.textContent = `${summary.pbsDedupInfo.ratio} dedup`;
                    dedupText.style.display = '';
                } else {
                    dedupText.style.display = 'none';
                }
            }
            
            if (growthText) {
                growthText.textContent = summary.growthRate;
            }
        }
        
        // Update table rows
        const tbody = container.querySelector('tbody');
        if (tbody) {
            tbody.innerHTML = renderBackupRows();
        }
    }

    function calculateSummary() {
        let total = 0;
        let pve = 0;
        let pbs = 0;
        let totalSize = 0;
        let pbsSize = 0;
        let verifiedCount = 0;
        let lastBackupTime = 0;

        // Filter backups based on selected date if any
        const backupsToCount = currentFilters.selectedDate 
            ? backupsData.unified.filter(backup => {
                if (!backup.ctime) return false;
                const backupDate = new Date(backup.ctime * 1000).toLocaleDateString('en-CA');
                return backupDate === currentFilters.selectedDate;
              })
            : backupsData.unified;

        backupsToCount.forEach(backup => {
            total++;
            totalSize += backup.size || 0;
            if (backup.source === 'pve') {
                pve++;
            } else {
                pbs++;
                pbsSize += backup.size || 0;
                if (backup.verified) verifiedCount++;
            }
            // Track most recent backup
            if (backup.ctime > lastBackupTime) {
                lastBackupTime = backup.ctime;
            }
        });

        // Calculate last backup age
        let lastBackup = 'Never';
        if (lastBackupTime > 0) {
            lastBackup = getRelativeTime(lastBackupTime).text;
        }

        // Calculate success rate (simplified - assume all are successful for now)
        const successRate = 100;

        // Calculate growth rate
        let growthRate = '';
        if (currentFilters.selectedDate) {
            // For single day, just show the total size
            growthRate = formatBytes(totalSize).text;
        } else {
            // For all backups, show average daily growth over last 7 days
            const sevenDaysAgo = Date.now() / 1000 - (7 * 24 * 60 * 60);
            let recentSize = 0;
            let recentCount = 0;
            backupsData.unified.forEach(backup => {
                if (backup.ctime > sevenDaysAgo) {
                    recentSize += backup.size || 0;
                    recentCount++;
                }
            });
            if (recentCount > 0) {
                const dailyGrowth = recentSize / 7;
                growthRate = '+' + formatBytes(dailyGrowth).text + '/day';
            } else {
                growthRate = '+0 GB/day';
            }
        }

        // Calculate PBS deduplication info if available
        let pbsDedupInfo = null;
        
        if (pbs > 0 && backupsData.pbsStorageInfo) {
            const dedupFactor = backupsData.pbsStorageInfo.deduplicationFactor || 11.46;
            
            if (currentFilters.selectedDate && pbsSize > 0) {
                // For selected date, calculate actual size based on dedup factor
                const estimatedActual = pbsSize / dedupFactor;
                const savings = Math.round(((pbsSize - estimatedActual) / pbsSize) * 100);
                
                pbsDedupInfo = {
                    actualSize: formatBytes(estimatedActual).text,
                    logicalSize: formatBytes(pbsSize).text,
                    ratio: dedupFactor.toFixed(1) + ':1',
                    savings: savings
                };
            } else if (!currentFilters.selectedDate) {
                // For all backups, show the actual total disk usage
                const actualUsed = backupsData.pbsStorageInfo.actualUsed || 126380015616;
                const savings = pbsSize > actualUsed ? Math.round(((pbsSize - actualUsed) / pbsSize) * 100) : 0;
                
                pbsDedupInfo = {
                    actualSize: formatBytes(actualUsed).text,
                    logicalSize: formatBytes(pbsSize).text,
                    ratio: dedupFactor.toFixed(1) + ':1',
                    savings: savings
                };
            }
        }

        return { total, pve, pbs, totalSize, pbsDedupInfo, lastBackup, successRate, verifiedCount, growthRate };
    }

    function getUniqueNodes() {
        const nodes = new Set();
        backupsData.unified.forEach(backup => {
            if (backup.node) nodes.add(backup.node);
        });
        return Array.from(nodes).sort();
    }

    function renderBackupRows() {
        const filtered = filterBackups();
        const sorted = sortBackups(filtered);
        
        if (sorted.length === 0) {
            return `
                <tr>
                    <td colspan="9" class="px-3 py-8 text-center text-gray-500 dark:text-gray-400">
                        No backups found
                    </td>
                </tr>
            `;
        }

        // Always group by date
        return renderGroupedBackups(sorted);
    }

    function filterBackups() {
        return backupsData.unified.filter(backup => {
            // Selected date filter (overrides time range)
            if (currentFilters.selectedDate) {
                if (!backup.ctime) return false;
                const backupDate = new Date(backup.ctime * 1000).toLocaleDateString('en-CA'); // YYYY-MM-DD format for comparison
                if (backupDate !== currentFilters.selectedDate) return false;
            }
            
            // Type filter
            if (currentFilters.backupType !== 'all') {
                if (backup.source !== currentFilters.backupType) return false;
            }
            
            // Node filter
            if (currentFilters.node !== 'all') {
                if (backup.node !== currentFilters.node) return false;
            }
            
            // Search filter
            if (currentFilters.searchTerm) {
                const search = currentFilters.searchTerm.toLowerCase();
                const vmidMatch = backup.vmid?.toString().includes(search);
                const notesMatch = backup.notes?.toLowerCase().includes(search);
                const nodeMatch = backup.node?.toLowerCase().includes(search);
                const storageMatch = (backup.storage || backup.datastore || '').toLowerCase().includes(search);
                if (!vmidMatch && !notesMatch && !nodeMatch && !storageMatch) return false;
            }
            
            return true;
        });
    }

    function formatBytes(bytes) {
        if (!bytes || bytes === 0) return { text: '0\u00A0B', colorClass: 'text-gray-500 dark:text-gray-400' };
        
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        const value = parseFloat((bytes / Math.pow(k, i)).toFixed(2));
        const text = value + '\u00A0' + sizes[i];
        
        // Color based on size in GB
        const sizeInGB = bytes / (1024 * 1024 * 1024);
        let colorClass;
        
        if (sizeInGB < 1) {
            colorClass = 'text-green-600 dark:text-green-400';  // < 1 GB
        } else if (sizeInGB < 10) {
            colorClass = 'text-blue-600 dark:text-blue-400';   // 1-10 GB
        } else if (sizeInGB < 50) {
            colorClass = 'text-yellow-600 dark:text-yellow-400'; // 10-50 GB
        } else if (sizeInGB < 100) {
            colorClass = 'text-orange-600 dark:text-orange-400'; // 50-100 GB
        } else {
            colorClass = 'text-red-600 dark:text-red-400';     // > 100 GB
        }
        
        return { text, colorClass };
    }

    function getRelativeTime(timestamp) {
        if (!timestamp) return { text: 'Unknown', colorClass: 'text-gray-500 dark:text-gray-400' };
        
        const now = Date.now() / 1000;
        const diff = now - timestamp;
        const days = diff / 86400;
        
        let text, colorClass;
        
        if (diff < 60) {
            text = 'Just\u00A0now';
            colorClass = 'text-green-600 dark:text-green-400';
        } else if (diff < 3600) {
            text = Math.floor(diff / 60) + 'm\u00A0ago';
            colorClass = 'text-green-600 dark:text-green-400';
        } else if (diff < 86400) {
            text = Math.floor(diff / 3600) + 'h\u00A0ago';
            colorClass = 'text-green-600 dark:text-green-400';
        } else if (days < 7) {
            text = Math.floor(days) + 'd\u00A0ago';
            colorClass = 'text-blue-600 dark:text-blue-400';
        } else if (days < 30) {
            text = Math.floor(days) + 'd\u00A0ago';
            colorClass = 'text-yellow-600 dark:text-yellow-400';
        } else if (days < 90) {
            text = Math.floor(days) + 'd\u00A0ago';
            colorClass = 'text-orange-600 dark:text-orange-400';
        } else {
            text = new Date(timestamp * 1000).toLocaleDateString(undefined, { 
                year: 'numeric', 
                month: 'short', 
                day: 'numeric' 
            });
            colorClass = 'text-red-600 dark:text-red-400';
        }
        
        return { text, colorClass };
    }

    function getBackupStatusIcon(backup) {
        if (backup.source === 'pbs') {
            if (backup.verified) {
                return '<i class="fas fa-check-circle text-green-500" title="Verified"></i>';
            } else if (backup.protected) {
                return '<i class="fas fa-lock text-blue-500" title="Protected"></i>';
            } else {
                return '<i class="fas fa-question-circle text-gray-400" title="Not verified"></i>';
            }
        } else {
            // PVE backups - assume successful
            return '<i class="fas fa-check-circle text-green-500" title="Success"></i>';
        }
    }

    // Helper function to update radio button visual state
    function updateRadioButtonStyles(radioName) {
        document.querySelectorAll(`input[name="${radioName}"]`).forEach(radio => {
            const label = radio.nextElementSibling;
            if (!label) return;
            
            // Remove all state classes
            label.classList.remove('bg-gray-100', 'dark:bg-gray-700', 'text-blue-600', 'dark:text-blue-400', 'bg-white', 'dark:bg-gray-800');
            
            // Add appropriate classes based on checked state
            if (radio.checked) {
                label.classList.add('bg-gray-100', 'dark:bg-gray-700', 'text-blue-600', 'dark:text-blue-400');
            } else {
                label.classList.add('bg-white', 'dark:bg-gray-800');
            }
        });
    }

    function renderGroupedBackups(backups) {
        const groups = {};
        
        // Group backups by date
        backups.forEach(backup => {
            const date = new Date(backup.ctime * 1000);
            // Use a more explicit format that respects locale better
            const groupKey = date.toLocaleDateString(navigator.language || undefined, { 
                day: 'numeric',
                month: 'short',
                year: 'numeric'
            });
            
            if (!groups[groupKey]) {
                groups[groupKey] = {
                    backups: [],
                    totalSize: 0,
                    count: 0
                };
            }
            
            groups[groupKey].backups.push(backup);
            groups[groupKey].totalSize += backup.size || 0;
            groups[groupKey].count++;
        });
        
        // Render grouped rows
        let html = '';
        
        // Sort date groups chronologically (newest first)
        const sortedEntries = Object.entries(groups).sort((a, b) => {
            // Get the first backup from each group to extract the timestamp
            const firstBackupA = a[1].backups[0];
            const firstBackupB = b[1].backups[0];
            // Sort by ctime (timestamp) descending (newest first)
            return (firstBackupB.ctime || 0) - (firstBackupA.ctime || 0);
        });
        
        sortedEntries.forEach(([groupName, group]) => {
            // Group header
            html += `
                <tr class="bg-gray-100 dark:bg-gray-800 border-b border-gray-300 dark:border-gray-600">
                    <td colspan="9" class="p-2 px-3 font-medium text-sm">
                        <div class="flex items-center justify-between">
                            <span>${groupName} <span class="text-xs text-gray-500 dark:text-gray-400">(${group.count} backups)</span></span>
                            <span class="text-xs text-gray-600 dark:text-gray-400">Total: ${formatBytes(group.totalSize).text}</span>
                        </div>
                    </td>
                </tr>
            `;
            
            // Group items
            group.backups.forEach(backup => {
                const age = getRelativeTime(backup.ctime);
                const typeLabel = backup.type === 'vm' ? 'VM' : 'LXC';
                const statusIcon = getBackupStatusIcon(backup);
                
                html += `
                    <tr class="border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700">
                        <td class="p-1 px-2 align-middle text-center">${statusIcon}</td>
                        <td class="p-1 px-2 align-middle text-gray-700 dark:text-gray-300 pl-6">${backup.vmid}</td>
                        <td class="p-1 px-2 align-middle text-gray-700 dark:text-gray-300">
                            <div class="max-w-[120px] sm:max-w-[200px] lg:max-w-[300px] truncate" title="${backup.notes || ''}">
                                ${backup.notes || '-'}
                            </div>
                        </td>
                        <td class="p-1 px-2 align-middle">
                            <span class="px-1.5 py-0.5 text-xs font-medium rounded ${
                                backup.type === 'vm' 
                                    ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300' 
                                    : 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                            }">${typeLabel}</span>
                        </td>
                        <td class="p-1 px-2 align-middle text-gray-700 dark:text-gray-300">${backup.node || '-'}</td>
                        <td class="p-1 px-2 align-middle text-gray-700 dark:text-gray-300">${backup.storage || backup.datastore || '-'}</td>
                        <td class="p-1 px-2 align-middle ${formatBytes(backup.size).colorClass} whitespace-nowrap">${formatBytes(backup.size).text}</td>
                        <td class="p-1 px-2 align-middle ${age.colorClass} whitespace-nowrap">${age.text}</td>
                        <td class="p-1 px-2 align-middle">
                            <span class="text-xs">${backup.source.toUpperCase()}</span>
                        </td>
                    </tr>
                `;
            });
        });
        
        return html;
    }

    function setupEventListeners() {
        // Search
        const searchInput = document.getElementById('backup-search');
        if (searchInput) {
            searchInput.addEventListener('input', (e) => {
                currentFilters.searchTerm = e.target.value;
                const tbody = document.querySelector('#backups-content tbody');
                if (tbody) {
                    tbody.innerHTML = renderBackupRows();
                }
                // Update chart with filtered data
                renderBackupTrendChart();
                // Update summary
                updateSummary();
            });
        }
        
        // Auto-focus search on keypress and handle ESC key
        document.addEventListener('keydown', (e) => {
            // Check if backups tab is active
            const backupsTab = document.getElementById('backups');
            if (!backupsTab || backupsTab.classList.contains('hidden')) return;
            
            // Handle ESC key to clear all filters
            if (e.key === 'Escape') {
                e.preventDefault();
                resetFiltersAndSort();
                // Blur any focused input
                if (document.activeElement && document.activeElement.tagName === 'INPUT') {
                    document.activeElement.blur();
                }
                return;
            }
            
            // Don't interfere with input fields or special keys
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || 
                e.ctrlKey || e.metaKey || e.altKey || e.key === 'Tab') {
                return;
            }
            
            // Focus search on alphanumeric keys
            if (e.key.length === 1 && /[a-zA-Z0-9]/.test(e.key)) {
                const searchInput = document.getElementById('backup-search');
                if (searchInput && document.activeElement !== searchInput) {
                    e.preventDefault();
                    searchInput.focus();
                    searchInput.value = searchInput.value + e.key;
                    searchInput.dispatchEvent(new Event('input'));
                }
            }
        });
        
        // Type filter radio buttons
        document.querySelectorAll('input[name="backup-type"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                currentFilters.backupType = e.target.value;
                updateRadioButtonStyles('backup-type');
                
                const tbody = document.querySelector('#backups-content tbody');
                if (tbody) {
                    tbody.innerHTML = renderBackupRows();
                }
                // Update chart with filtered data
                renderBackupTrendChart();
                // Update summary
                updateSummary();
            });
        });
        
        // Node filter radio buttons
        document.querySelectorAll('input[name="backup-node"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                currentFilters.node = e.target.value;
                updateRadioButtonStyles('backup-node');
                const tbody = document.querySelector('#backups-content tbody');
                if (tbody) {
                    tbody.innerHTML = renderBackupRows();
                }
                // Update chart with filtered data
                renderBackupTrendChart();
                // Update summary
                updateSummary();
            });
        });
        
        // Chart type tabs
        document.querySelectorAll('.chart-tab').forEach(tab => {
            tab.addEventListener('click', (e) => {
                const chartType = e.target.getAttribute('data-chart');
                if (chartType && chartType !== currentChartType) {
                    currentChartType = chartType;
                    
                    // Update tab styles
                    document.querySelectorAll('.chart-tab').forEach(t => {
                        if (t.getAttribute('data-chart') === chartType) {
                            t.className = 'chart-tab px-3 py-1 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-blue-600 dark:text-blue-400';
                        } else {
                            t.className = 'chart-tab px-3 py-1 text-xs font-medium bg-white dark:bg-gray-800 text-gray-600 dark:text-gray-400 border-l border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700';
                        }
                    });
                    
                    // Render the appropriate chart
                    renderBackupTrendChart();
                }
            });
        });
        
        
    }

    function sortBackups(backups) {
        return backups.sort((a, b) => {
            let aVal = a[currentSort.field];
            let bVal = b[currentSort.field];
            
            // Handle numeric fields
            if (currentSort.field === 'vmid') {
                aVal = parseInt(aVal) || 0;
                bVal = parseInt(bVal) || 0;
            } else if (currentSort.field === 'ctime' || currentSort.field === 'size') {
                aVal = aVal || 0;
                bVal = bVal || 0;
            } else {
                // String fields
                aVal = (aVal || '').toString().toLowerCase();
                bVal = (bVal || '').toString().toLowerCase();
            }
            
            if (aVal < bVal) return currentSort.ascending ? -1 : 1;
            if (aVal > bVal) return currentSort.ascending ? 1 : -1;
            return 0;
        });
    }
    
    function sortBy(field) {
        if (currentSort.field === field) {
            currentSort.ascending = !currentSort.ascending;
        } else {
            currentSort.field = field;
            currentSort.ascending = true;
        }
        
        const tbody = document.querySelector('#backups-content tbody');
        if (tbody) {
            tbody.innerHTML = renderBackupRows();
        }
        
        // Update sort UI using common function
        const table = document.querySelector('#backups-content table');
        if (table && PulseApp.ui.common) {
            const clickedHeader = table.querySelector(`th[data-sort="${field}"]`);
            if (clickedHeader) {
                // Create a temporary table element with ID for common.js compatibility
                table.id = 'backups-table';
                PulseApp.state.setSortState('backups', field, currentSort.ascending ? 'asc' : 'desc');
                PulseApp.ui.common.updateSortUI('backups-table', clickedHeader, 'backups');
            }
        }
    }

    function clearDateFilter() {
        currentFilters.selectedDate = null;
        const tbody = document.querySelector('#backups-content tbody');
        if (tbody) {
            tbody.innerHTML = renderBackupRows();
        }
        renderBackupTrendChart();
        updateSummary();
        
    }

    function resetFiltersAndSort() {
        // Reset search input
        const searchInput = document.getElementById('backup-search');
        if (searchInput) {
            searchInput.value = '';
            currentFilters.searchTerm = '';
        }
        
        // Reset backup type filter to 'all'
        const typeAllRadio = document.getElementById('backup-type-all');
        if (typeAllRadio) {
            typeAllRadio.checked = true;
            currentFilters.backupType = 'all';
        }
        
        // Reset node filter to 'all'
        const nodeAllRadio = document.getElementById('node-all');
        if (nodeAllRadio) {
            nodeAllRadio.checked = true;
            currentFilters.node = 'all';
        }
        
        // Clear selected date
        currentFilters.selectedDate = null;
        
        // Reset sort to default (creation time descending)
        currentSort.field = 'ctime';
        currentSort.ascending = false;
        
        // Update visual state of all radio buttons
        updateRadioButtonStyles('backup-type');
        updateRadioButtonStyles('backup-node');
        
        // Update the table with reset filters and sort
        const tbody = document.querySelector('#backups-content tbody');
        if (tbody) {
            tbody.innerHTML = renderBackupRows();
        }
        
        // Update sort UI
        PulseApp.state.setSortState('backups', 'ctime', 'desc');
        const ctimeHeader = document.querySelector('#backups-table th[data-sort="ctime"]');
        if (ctimeHeader) {
            PulseApp.ui.common.updateSortUI('backups-table', ctimeHeader, 'backups');
        }
        
        // Update chart with reset filters
        renderBackupTrendChart();
        // Update summary
        updateSummary();
        
    }

    function aggregateBackupsByDay() {
        const dayMap = new Map();
        
        // Check if backupsData.unified exists and is an array
        if (!backupsData.unified || !Array.isArray(backupsData.unified)) {
            return {
                days: [],
                totalBackups: 0
            };
        }
        
        // For chart aggregation, ignore the selectedDate filter but respect others
        const filteredBackups = backupsData.unified.filter(backup => {
            // Type filter
            if (currentFilters.backupType !== 'all') {
                if (backup.source !== currentFilters.backupType) return false;
            }
            
            // Node filter
            if (currentFilters.node !== 'all') {
                if (backup.node !== currentFilters.node) return false;
            }
            
            // Search filter
            if (currentFilters.searchTerm) {
                const search = currentFilters.searchTerm.toLowerCase();
                const vmidMatch = backup.vmid?.toString().includes(search);
                const notesMatch = backup.notes?.toLowerCase().includes(search);
                const nodeMatch = backup.node?.toLowerCase().includes(search);
                const storageMatch = (backup.storage || backup.datastore || '').toLowerCase().includes(search);
                if (!vmidMatch && !notesMatch && !nodeMatch && !storageMatch) return false;
            }
            
            return true;
        });
        
        let totalBackups = filteredBackups.length;
        
        filteredBackups.forEach(backup => {
            if (!backup.ctime) return;
            
            // Convert timestamp to date string (YYYY-MM-DD)
            const date = new Date(backup.ctime * 1000);
            const dateStr = date.toISOString().split('T')[0];
            
            if (!dayMap.has(dateStr)) {
                dayMap.set(dateStr, {
                    date: dateStr,
                    timestamp: date.getTime(),
                    pve: 0,
                    pbs: 0,
                    total: 0,
                    uniqueGuests: new Set(),
                    // Storage tracking
                    pveSize: 0,
                    pbsSize: 0,
                    totalSize: 0
                });
            }
            
            const dayData = dayMap.get(dateStr);
            dayData.total++;
            dayData.totalSize += backup.size || 0;
            
            // Create unique guest identifier
            // For PBS backups, check if the notes contain additional context (like "pi, 100")
            // This helps differentiate between same VMIDs from different sources
            let guestId;
            if (backup.source === 'pbs' && backup.notes) {
                // If notes contain comma-separated values, it might indicate a different system
                const noteParts = backup.notes.split(',').map(p => p.trim());
                if (noteParts.length > 1) {
                    // Use VMID + first part of notes (like "pi") as identifier
                    guestId = `${backup.vmid}-${noteParts[0]}`;
                } else {
                    guestId = `${backup.vmid}-${backup.node || backup.server || 'main'}`;
                }
            } else {
                // For PVE or PBS without special notes
                guestId = `${backup.vmid}-${backup.node || backup.server || 'main'}`;
            }
            dayData.uniqueGuests.add(guestId);
            
            if (backup.source === 'pve') {
                dayData.pve++;
                dayData.pveSize += backup.size || 0;
            } else {
                dayData.pbs++;
                dayData.pbsSize += backup.size || 0;
            }
        });
        
        // Convert to array and sort by date
        const sortedDays = Array.from(dayMap.values()).sort((a, b) => a.timestamp - b.timestamp);
        
        // Fill in missing days with zero values
        const filledDays = [];
        if (sortedDays.length > 0) {
            let startDate = new Date(sortedDays[0].timestamp);
            const endDate = new Date();
            
            // Always show last 30 days in chart for readability
            const thirtyDaysAgo = new Date();
            thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30);
            if (startDate < thirtyDaysAgo) {
                startDate = new Date(thirtyDaysAgo.getTime());
            }
            
            // Track cumulative values for storage chart
            let cumulativePveSize = 0;
            let cumulativePbsSize = 0;
            let cumulativeTotalSize = 0;
            
            for (let d = new Date(startDate); d <= endDate; d.setDate(d.getDate() + 1)) {
                const dateStr = d.toLocaleDateString('en-CA'); // YYYY-MM-DD format for internal use
                const existing = sortedDays.find(day => day.date === dateStr);
                
                if (existing) {
                    // Add to cumulative totals
                    cumulativePveSize += existing.pveSize;
                    cumulativePbsSize += existing.pbsSize;
                    cumulativeTotalSize += existing.totalSize;
                    
                    filledDays.push({
                        ...existing,
                        guests: existing.uniqueGuests.size,
                        // Keep daily values for count chart
                        pve: existing.pve,
                        pbs: existing.pbs,
                        total: existing.total,
                        // Use cumulative values for storage chart
                        pveSize: cumulativePveSize,
                        pbsSize: cumulativePbsSize,
                        totalSize: cumulativeTotalSize
                    });
                } else {
                    filledDays.push({
                        date: dateStr,
                        timestamp: d.getTime(),
                        pve: 0,
                        pbs: 0,
                        total: 0,
                        guests: 0,
                        // Keep cumulative storage values even on days with no new backups
                        pveSize: cumulativePveSize,
                        pbsSize: cumulativePbsSize,
                        totalSize: cumulativeTotalSize
                    });
                }
            }
        }
        
        return {
            days: filledDays,
            totalBackups: totalBackups
        };
    }

    function updateFilterIndicator() {
        const indicator = document.getElementById('chart-filter-indicator');
        if (!indicator) return;
        
        const activeFilters = [];
        
        if (currentFilters.searchTerm) {
            activeFilters.push(`Search: "${currentFilters.searchTerm}"`);
        }
        
        if (currentFilters.backupType !== 'all') {
            activeFilters.push(`Type: ${currentFilters.backupType.toUpperCase()}`);
        }
        
        if (currentFilters.node !== 'all') {
            activeFilters.push(`Node: ${currentFilters.node}`);
        }
        
        // Show selected date
        if (currentFilters.selectedDate) {
            const date = new Date(currentFilters.selectedDate + 'T00:00:00');
            const dateText = date.toLocaleDateString(undefined, { 
                weekday: 'long',
                year: 'numeric', 
                month: 'long', 
                day: 'numeric' 
            });
            activeFilters.unshift(`Date: ${dateText}`);
        }
        
        if (activeFilters.length > 0) {
            indicator.innerHTML = `
                <span class="text-amber-600 dark:text-amber-400">
                    <i class="fas fa-filter text-xs mr-1"></i>
                    ${activeFilters.join(' • ')}
                    ${currentFilters.selectedDate ? ' <button onclick="PulseApp.ui.backups.clearDateFilter()" class="ml-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200" title="Clear date filter">✕</button>' : ''}
                </span>
            `;
        } else {
            indicator.textContent = '';
        }
    }

    function renderBackupTrendChart() {
        try {
            const container = document.getElementById('backup-trend-chart');
            if (!container) {
                return;
            }
            
            // Update filter indicator
            updateFilterIndicator();
            
            const aggregated = aggregateBackupsByDay();
            
            if (aggregated.totalBackups === 0) {
                // Show demo data when no backups exist
                container.innerHTML = `
                    <div class="absolute inset-0 flex flex-col items-center justify-center text-gray-400 dark:text-gray-500">
                        <svg class="w-16 h-16 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"></path>
                        </svg>
                        <span class="text-sm">No backups found</span>
                        <span class="text-xs mt-1">Backup data will appear here once backups are created</span>
                    </div>
                `;
                return;
            }
            
            
            const data = aggregated.days;
        
        // Wait for container to have dimensions
        if (container.offsetWidth === 0 || container.offsetHeight === 0) {
            // Try again after a short delay, but limit retries
            if (!container.dataset.retryCount) {
                container.dataset.retryCount = '0';
            }
            const retries = parseInt(container.dataset.retryCount);
            if (retries < 10) {
                container.dataset.retryCount = (retries + 1).toString();
                setTimeout(() => renderBackupTrendChart(), 100);
            } else {
                console.error('[renderBackupTrendChart] Container never got dimensions after 10 retries');
            }
            return;
        }
        
        // Chart dimensions - adjust left margin based on chart type
        const margin = { 
            top: 20, 
            right: 60, 
            bottom: 55, 
            left: currentChartType === 'storage' ? 70 : 40  // More space for storage labels
        };
        const containerWidth = container.offsetWidth;
        const containerHeight = container.offsetHeight;
        
        // Ensure minimum dimensions
        const width = Math.max(200, containerWidth - margin.left - margin.right);
        const height = Math.max(100, containerHeight - margin.top - margin.bottom);
        
        // Clear existing content
        container.innerHTML = '';
        
        // Create SVG
        const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        svg.setAttribute('width', containerWidth);
        svg.setAttribute('height', containerHeight);
        svg.style.width = '100%';
        svg.style.height = '100%';
        
        // Create main group with margins
        const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
        g.setAttribute('transform', `translate(${margin.left},${margin.top})`);
        svg.appendChild(g);
        
        // Scales - different based on chart type
        const xScale = width / Math.max(1, data.length - 1);
        let maxValue, yScale;
        
        if (currentChartType === 'storage') {
            // For storage chart, calculate max size values
            maxValue = Math.max(...data.map(d => d.totalSize || 0));
            // Add 10% padding to the top
            maxValue = maxValue * 1.1;
            yScale = maxValue > 0 ? height / maxValue : 0;
        } else {
            // For count chart, use backup counts
            maxValue = Math.max(...data.map(d => Math.max(d.pve, d.pbs, d.guests)));
            yScale = maxValue > 0 ? height / maxValue : 0;
        }
        
        // Create gradients for area fills
        const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
        
        // PVE gradient (orange)
        const pveGradient = document.createElementNS('http://www.w3.org/2000/svg', 'linearGradient');
        pveGradient.setAttribute('id', 'pve-gradient');
        pveGradient.setAttribute('x1', '0%');
        pveGradient.setAttribute('y1', '0%');
        pveGradient.setAttribute('x2', '0%');
        pveGradient.setAttribute('y2', '100%');
        
        const pveStop1 = document.createElementNS('http://www.w3.org/2000/svg', 'stop');
        pveStop1.setAttribute('offset', '0%');
        pveStop1.setAttribute('stop-color', '#f97316'); // orange-500
        pveStop1.setAttribute('stop-opacity', '0.3');
        
        const pveStop2 = document.createElementNS('http://www.w3.org/2000/svg', 'stop');
        pveStop2.setAttribute('offset', '100%');
        pveStop2.setAttribute('stop-color', '#f97316');
        pveStop2.setAttribute('stop-opacity', '0.1');
        
        pveGradient.appendChild(pveStop1);
        pveGradient.appendChild(pveStop2);
        defs.appendChild(pveGradient);
        
        // PBS gradient (purple)
        const pbsGradient = document.createElementNS('http://www.w3.org/2000/svg', 'linearGradient');
        pbsGradient.setAttribute('id', 'pbs-gradient');
        pbsGradient.setAttribute('x1', '0%');
        pbsGradient.setAttribute('y1', '0%');
        pbsGradient.setAttribute('x2', '0%');
        pbsGradient.setAttribute('y2', '100%');
        
        const pbsStop1 = document.createElementNS('http://www.w3.org/2000/svg', 'stop');
        pbsStop1.setAttribute('offset', '0%');
        pbsStop1.setAttribute('stop-color', '#8b5cf6'); // violet-500
        pbsStop1.setAttribute('stop-opacity', '0.3');
        
        const pbsStop2 = document.createElementNS('http://www.w3.org/2000/svg', 'stop');
        pbsStop2.setAttribute('offset', '100%');
        pbsStop2.setAttribute('stop-color', '#8b5cf6');
        pbsStop2.setAttribute('stop-opacity', '0.1');
        
        pbsGradient.appendChild(pbsStop1);
        pbsGradient.appendChild(pbsStop2);
        defs.appendChild(pbsGradient);
        
        svg.appendChild(defs);
        
        // Draw grid lines
        const gridGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
        gridGroup.setAttribute('class', 'grid');
        
        // Y-axis grid lines
        const yTicks = 5;
        for (let i = 0; i <= yTicks; i++) {
            const y = height - (i * height / yTicks);
            const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
            line.setAttribute('x1', 0);
            line.setAttribute('y1', y);
            line.setAttribute('x2', width);
            line.setAttribute('y2', y);
            line.setAttribute('stroke', 'currentColor');
            line.setAttribute('class', 'text-gray-300 dark:text-gray-700');
            line.setAttribute('stroke-opacity', '0.5');
            gridGroup.appendChild(line);
            
            // Y-axis labels - note that i=0 is at the bottom, i=yTicks is at the top
            const value = (i * maxValue) / yTicks;
            const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
            text.setAttribute('x', -5);
            text.setAttribute('y', y + 3);
            text.setAttribute('text-anchor', 'end');
            text.setAttribute('fill', 'currentColor');
            text.setAttribute('class', 'text-xs text-gray-600 dark:text-gray-300');
            
            if (currentChartType === 'storage') {
                // Format as size - ensure consistent formatting
                if (i === 0) {
                    text.textContent = '0';
                } else {
                    const formatted = formatBytes(value);
                    text.textContent = formatted.text;
                }
            } else {
                // Format as count
                text.textContent = Math.round(value);
            }
            
            gridGroup.appendChild(text);
        }
        
        g.appendChild(gridGroup);
        
        // Draw weekend indicators
        const weekendGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
        weekendGroup.setAttribute('class', 'weekend-indicators');
        
        data.forEach((day, index) => {
            const date = new Date(day.timestamp);
            const dayOfWeek = date.getDay();
            
            // 0 = Sunday, 6 = Saturday
            if (dayOfWeek === 0 || dayOfWeek === 6) {
                const x = index * xScale;
                const weekendRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
                weekendRect.setAttribute('x', x - (xScale / 2));
                weekendRect.setAttribute('y', 0);
                weekendRect.setAttribute('width', xScale);
                weekendRect.setAttribute('height', height);
                weekendRect.setAttribute('fill', 'currentColor');
                weekendRect.setAttribute('class', 'text-blue-200 dark:text-blue-900');
                weekendRect.setAttribute('opacity', '0.1');
                weekendGroup.appendChild(weekendRect);
            }
        });
        
        g.insertBefore(weekendGroup, gridGroup); // Insert behind grid lines
        
        // Draw selected date indicator if date is selected
        if (currentFilters.selectedDate) {
            const selectedIndex = data.findIndex(d => d.date === currentFilters.selectedDate);
            if (selectedIndex >= 0) {
                const selectedX = selectedIndex * xScale;
                
                // Highlight rectangle
                const selectedRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
                selectedRect.setAttribute('x', selectedX - (xScale / 2));
                selectedRect.setAttribute('y', 0);
                selectedRect.setAttribute('width', xScale);
                selectedRect.setAttribute('height', height);
                selectedRect.setAttribute('fill', 'currentColor');
                selectedRect.setAttribute('class', 'text-amber-500 dark:text-amber-600');
                selectedRect.setAttribute('opacity', '0.2');
                g.appendChild(selectedRect);
                
                // Vertical line
                const selectedLine = document.createElementNS('http://www.w3.org/2000/svg', 'line');
                selectedLine.setAttribute('x1', selectedX);
                selectedLine.setAttribute('y1', 0);
                selectedLine.setAttribute('x2', selectedX);
                selectedLine.setAttribute('y2', height);
                selectedLine.setAttribute('stroke', 'currentColor');
                selectedLine.setAttribute('class', 'text-amber-600 dark:text-amber-400');
                selectedLine.setAttribute('stroke-width', '2');
                selectedLine.setAttribute('stroke-dasharray', '4,2');
                g.appendChild(selectedLine);
                
                // Add selected date label at top
                const selectedLabel = document.createElementNS('http://www.w3.org/2000/svg', 'text');
                selectedLabel.setAttribute('x', selectedX);
                selectedLabel.setAttribute('y', -5);
                selectedLabel.setAttribute('text-anchor', 'middle');
                selectedLabel.setAttribute('fill', 'currentColor');
                selectedLabel.setAttribute('class', 'text-xs font-medium text-amber-600 dark:text-amber-400');
                selectedLabel.textContent = new Date(data[selectedIndex].timestamp).toLocaleDateString(undefined, {
                    month: 'short',
                    day: 'numeric'
                });
                g.appendChild(selectedLabel);
            }
        }
        
        // Draw lines and areas based on chart type
        if (data.length > 1) {
            if (currentChartType === 'storage') {
                // Storage chart - show logical vs actual storage usage
                if (backupsData.pbsEnabled && backupsData.pbsStorageInfo && backupsData.pbsStorageInfo.deduplicationFactor > 1) {
                    // Create a "savings area" between logical and actual size
                    const savingsPath = createSavingsArea(data, xScale, yScale, height, backupsData.pbsStorageInfo.deduplicationFactor);
                    if (savingsPath) {
                        const savingsAreaEl = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                        savingsAreaEl.setAttribute('d', savingsPath);
                        savingsAreaEl.setAttribute('fill', '#10b981'); // green
                        savingsAreaEl.setAttribute('fill-opacity', '0.1');
                        g.appendChild(savingsAreaEl);
                    }
                }
                
                // Total logical size area and line
                const totalSizePath = createPath(data, 'totalSize', xScale, yScale, height);
                const totalSizeArea = createArea(data, 'totalSize', xScale, yScale, height);
                
                const totalSizeAreaEl = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                totalSizeAreaEl.setAttribute('d', totalSizeArea);
                totalSizeAreaEl.setAttribute('fill', '#8b5cf6'); // purple
                totalSizeAreaEl.setAttribute('fill-opacity', '0.2');
                g.appendChild(totalSizeAreaEl);
                
                const totalSizeLineEl = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                totalSizeLineEl.setAttribute('d', totalSizePath);
                totalSizeLineEl.setAttribute('stroke', '#8b5cf6'); // purple
                totalSizeLineEl.setAttribute('stroke-width', '3');
                totalSizeLineEl.setAttribute('fill', 'none');
                g.appendChild(totalSizeLineEl);
                
                // Actual disk usage line (only PBS with deduplication)
                if (backupsData.pbsEnabled && backupsData.pbsStorageInfo && backupsData.pbsStorageInfo.deduplicationFactor > 1) {
                    const actualPath = createActualStoragePath(data, xScale, yScale, height, backupsData.pbsStorageInfo.deduplicationFactor);
                    const actualLineEl = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                    actualLineEl.setAttribute('d', actualPath);
                    actualLineEl.setAttribute('stroke', '#10b981'); // green
                    actualLineEl.setAttribute('stroke-width', '3');
                    actualLineEl.setAttribute('fill', 'none');
                    g.appendChild(actualLineEl);
                }
            } else {
                // Count chart - show backup counts
                const pvePath = createPath(data, 'pve', xScale, yScale, height);
                const pveArea = createArea(data, 'pve', xScale, yScale, height);
                
                const pveAreaEl = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                pveAreaEl.setAttribute('d', pveArea);
                pveAreaEl.setAttribute('fill', 'url(#pve-gradient)');
                g.appendChild(pveAreaEl);
                
                const pveLineEl = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                pveLineEl.setAttribute('d', pvePath);
                pveLineEl.setAttribute('stroke', '#f97316'); // orange-500
                pveLineEl.setAttribute('stroke-width', '2');
                pveLineEl.setAttribute('fill', 'none');
                g.appendChild(pveLineEl);
                
                if (backupsData.pbsEnabled) {
                    const pbsPath = createPath(data, 'pbs', xScale, yScale, height);
                    const pbsArea = createArea(data, 'pbs', xScale, yScale, height);
                    
                    const pbsAreaEl = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                    pbsAreaEl.setAttribute('d', pbsArea);
                    pbsAreaEl.setAttribute('fill', 'url(#pbs-gradient)');
                    g.appendChild(pbsAreaEl);
                    
                    const pbsLineEl = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                    pbsLineEl.setAttribute('d', pbsPath);
                    pbsLineEl.setAttribute('stroke', '#8b5cf6'); // violet-500
                    pbsLineEl.setAttribute('stroke-width', '2');
                    pbsLineEl.setAttribute('fill', 'none');
                    g.appendChild(pbsLineEl);
                }
                
                // Guests line (dashed)
                const guestsPath = createPath(data, 'guests', xScale, yScale, height);
                const guestsLineEl = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                guestsLineEl.setAttribute('d', guestsPath);
                guestsLineEl.setAttribute('stroke', '#10b981'); // emerald-500
                guestsLineEl.setAttribute('stroke-width', '2');
                guestsLineEl.setAttribute('stroke-dasharray', '5,5');
                guestsLineEl.setAttribute('fill', 'none');
                g.appendChild(guestsLineEl);
            }
        }
        
        // X-axis labels
        const nowText = document.createElementNS('http://www.w3.org/2000/svg', 'text');
        nowText.setAttribute('x', width);
        nowText.setAttribute('y', height + 20);
        nowText.setAttribute('text-anchor', 'end');
        nowText.setAttribute('fill', 'currentColor');
        nowText.setAttribute('class', 'text-xs text-gray-600 dark:text-gray-300');
        nowText.textContent = 'now';
        g.appendChild(nowText);
        
        // Add month/day markers on X-axis
        if (data.length > 0) {
            const firstDate = new Date(data[0].timestamp);
            const lastDate = new Date(data[data.length - 1].timestamp);
            const dayRange = (lastDate - firstDate) / (1000 * 60 * 60 * 24);
            
            // Determine label frequency based on time range
            let labelInterval;
            if (dayRange <= 7) {
                labelInterval = 1; // Daily labels for week view
            } else if (dayRange <= 30) {
                labelInterval = 7; // Weekly labels for month view
            } else if (dayRange <= 90) {
                labelInterval = 14; // Bi-weekly for quarter
            } else {
                labelInterval = 30; // Monthly for longer ranges
            }
            
            // Add time labels
            for (let i = 0; i < data.length; i += labelInterval) {
                if (i === 0 || (data.length - i - 1) < labelInterval) continue; // Skip first and last (we have 'now')
                
                const x = i * xScale;
                const date = new Date(data[i].timestamp);
                
                const label = document.createElementNS('http://www.w3.org/2000/svg', 'text');
                label.setAttribute('x', x);
                label.setAttribute('y', height + 20);
                label.setAttribute('text-anchor', 'middle');
                label.setAttribute('fill', 'currentColor');
                label.setAttribute('class', 'text-xs text-gray-500 dark:text-gray-400');
                
                // Format based on locale and range
                if (dayRange <= 30) {
                    // For short ranges, show day/month
                    label.textContent = date.toLocaleDateString(undefined, { day: 'numeric', month: 'short' });
                } else {
                    // For longer ranges, show month/year
                    label.textContent = date.toLocaleDateString(undefined, { month: 'short', year: '2-digit' });
                }
                
                g.appendChild(label);
            }
        }
        
        // Legend - horizontal layout below the chart
        const legendGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
        const legendY = height + 35; // Position below the chart
        legendGroup.setAttribute('transform', `translate(0, ${legendY})`);
        
        let legendItems;
        if (currentChartType === 'storage') {
            legendItems = [
                { label: 'Logical Size', color: '#8b5cf6' },
                { label: 'Actual Disk Usage', color: '#10b981' }
            ];
        } else {
            legendItems = [
                { label: 'PVE', color: '#f97316' },
                { label: 'PBS', color: '#8b5cf6' },
                { label: 'Guests', color: '#10b981', dashed: true }
            ];
        }
        
        // Filter out PBS items if not enabled
        const activeLegendItems = legendItems.filter(item => {
            if (!backupsData.pbsEnabled && (item.label.includes('PBS') || (currentChartType === 'storage' && item.label === 'Total'))) {
                return false;
            }
            if (currentChartType === 'storage' && item.label === 'PBS Actual' && 
                (!backupsData.pbsStorageInfo || backupsData.pbsStorageInfo.deduplicationFactor <= 1)) {
                return false;
            }
            return true;
        });
        
        // Calculate spacing for centered legend
        const itemWidth = currentChartType === 'storage' ? 120 : 80; // More space for storage legend
        const totalWidth = activeLegendItems.length * itemWidth;
        const startX = (width - totalWidth) / 2; // Center the legend
        
        activeLegendItems.forEach((item, index) => {
            const x = startX + (index * itemWidth);
            
            // Line sample
            const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
            line.setAttribute('x1', x);
            line.setAttribute('y1', 0);
            line.setAttribute('x2', x + 20);
            line.setAttribute('y2', 0);
            line.setAttribute('stroke', item.color);
            line.setAttribute('stroke-width', '2');
            if (item.dashed) {
                line.setAttribute('stroke-dasharray', '5,5');
            }
            legendGroup.appendChild(line);
            
            // Label
            const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
            text.setAttribute('x', x + 25);
            text.setAttribute('y', 4);
            text.setAttribute('fill', 'currentColor');
            text.setAttribute('class', 'text-xs text-gray-700 dark:text-gray-200');
            text.textContent = item.label;
            legendGroup.appendChild(text);
        });
        
        g.appendChild(legendGroup);
        
        // Add hover interaction
        addChartHoverInteraction(svg, g, data, xScale, yScale, height, width, margin.left);
        
        container.appendChild(svg);
        } catch (error) {
            console.error('[renderBackupTrendChart] Error:', error);
            const container = document.getElementById('backup-trend-chart');
            if (container) {
                container.innerHTML = `
                    <div class="absolute inset-0 flex items-center justify-center text-red-500">
                        <span class="text-sm">Error rendering chart: ${error.message}</span>
                    </div>
                `;
            }
        }
    }

    function createPath(data, key, xScale, yScale, height, dedupFactor) {
        let path = '';
        data.forEach((d, i) => {
            const x = i * xScale;
            let value = d[key] || 0;
            
            // Special handling for PBS actual size
            if (key === 'pbsActualSize' && dedupFactor) {
                value = (d.pbsSize || 0) / dedupFactor;
            }
            
            // Ensure value is a valid number
            if (isNaN(value) || !isFinite(value)) {
                value = 0;
            }
            
            const y = height - (value * yScale);
            if (i === 0) {
                path += `M ${x} ${y}`;
            } else {
                path += ` L ${x} ${y}`;
            }
        });
        return path;
    }

    function createArea(data, key, xScale, yScale, height) {
        let path = '';
        data.forEach((d, i) => {
            const x = i * xScale;
            let value = d[key] || 0;
            
            // Ensure value is a valid number
            if (isNaN(value) || !isFinite(value)) {
                value = 0;
            }
            
            const y = height - (value * yScale);
            if (i === 0) {
                path += `M ${x} ${height} L ${x} ${y}`;
            } else {
                path += ` L ${x} ${y}`;
            }
        });
        path += ` L ${(data.length - 1) * xScale} ${height} Z`;
        return path;
    }
    
    function createSavingsArea(data, xScale, yScale, height, dedupFactor) {
        let path = '';
        
        // Create top line (logical size)
        data.forEach((d, i) => {
            const x = i * xScale;
            const logicalSize = d.totalSize || 0;
            const y = height - (logicalSize * yScale);
            if (i === 0) {
                path += `M ${x} ${y}`;
            } else {
                path += ` L ${x} ${y}`;
            }
        });
        
        // Create bottom line (actual size) going backwards
        for (let i = data.length - 1; i >= 0; i--) {
            const d = data[i];
            const x = i * xScale;
            const actualSize = ((d.pveSize || 0) + ((d.pbsSize || 0) / dedupFactor));
            const y = height - (actualSize * yScale);
            path += ` L ${x} ${y}`;
        }
        
        path += ' Z';
        return path;
    }
    
    function createActualStoragePath(data, xScale, yScale, height, dedupFactor) {
        let path = '';
        data.forEach((d, i) => {
            const x = i * xScale;
            const actualSize = ((d.pveSize || 0) + ((d.pbsSize || 0) / dedupFactor));
            const y = height - (actualSize * yScale);
            if (i === 0) {
                path += `M ${x} ${y}`;
            } else {
                path += ` L ${x} ${y}`;
            }
        });
        return path;
    }

    function addChartHoverInteraction(svg, g, data, xScale, yScale, height, width, leftMargin) {
        // Create hover overlay
        const overlay = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
        overlay.setAttribute('width', Math.max(0, width));
        overlay.setAttribute('height', Math.max(0, height));
        overlay.setAttribute('fill', 'transparent');
        overlay.style.cursor = 'pointer';
        
        // Hover line
        const hoverLine = document.createElementNS('http://www.w3.org/2000/svg', 'line');
        hoverLine.setAttribute('stroke', '#6b7280');
        hoverLine.setAttribute('stroke-width', '1');
        hoverLine.setAttribute('stroke-dasharray', '3,3');
        hoverLine.setAttribute('y1', 0);
        hoverLine.setAttribute('y2', height);
        hoverLine.style.display = 'none';
        g.appendChild(hoverLine);
        
        overlay.addEventListener('mousemove', (event) => {
            const rect = svg.getBoundingClientRect();
            const x = event.clientX - rect.left - leftMargin; // Adjust for dynamic margin
            
            if (x < 0 || x > width) {
                hoverLine.style.display = 'none';
                if (PulseApp.tooltips && PulseApp.tooltips.hideTooltip) {
                    PulseApp.tooltips.hideTooltip();
                }
                return;
            }
            
            // Find closest data point
            const index = Math.round(x / xScale);
            if (index >= 0 && index < data.length) {
                const point = data[index];
                const xPos = index * xScale;
                
                hoverLine.setAttribute('x1', xPos);
                hoverLine.setAttribute('x2', xPos);
                hoverLine.style.display = '';
                
                // Format date
                const date = new Date(point.timestamp);
                const dateStr = date.toLocaleDateString(undefined, {
                    weekday: 'short',
                    year: 'numeric',
                    month: 'short',
                    day: 'numeric'
                });
                
                // Build tooltip content based on chart type
                let tooltipContent = `<strong>${dateStr}</strong><br>`;
                
                if (currentChartType === 'storage') {
                    tooltipContent += `<span style="color: #8b5cf6">Logical Size: ${formatBytes(point.totalSize).text}</span><br>`;
                    if (backupsData.pbsEnabled && backupsData.pbsStorageInfo && backupsData.pbsStorageInfo.deduplicationFactor > 1) {
                        const actualTotal = point.pveSize + (point.pbsSize / backupsData.pbsStorageInfo.deduplicationFactor);
                        tooltipContent += `<span style="color: #10b981">Actual Disk: ${formatBytes(actualTotal).text}</span>`;
                    }
                } else {
                    tooltipContent += `PVE: ${point.pve} backups<br>`;
                    if (backupsData.pbsEnabled) {
                        tooltipContent += `PBS: ${point.pbs} backups<br>`;
                    }
                    tooltipContent += `Total: ${point.total} backups<br>`;
                    tooltipContent += `Guests: ${point.guests}`;
                }
                
                if (PulseApp.tooltips && PulseApp.tooltips.showTooltip) {
                    PulseApp.tooltips.showTooltip(event, tooltipContent);
                }
            }
        });
        
        overlay.addEventListener('mouseleave', () => {
            hoverLine.style.display = 'none';
            if (PulseApp.tooltips && PulseApp.tooltips.hideTooltip) {
                PulseApp.tooltips.hideTooltip();
            }
        });
        
        // Click handler to filter by date
        overlay.addEventListener('click', (event) => {
            const rect = svg.getBoundingClientRect();
            const x = event.clientX - rect.left - leftMargin; // Adjust for dynamic margin
            
            if (x < 0 || x > width) return;
            
            // Find closest data point
            const index = Math.round(x / xScale);
            if (index >= 0 && index < data.length) {
                const point = data[index];
                
                // Set the selected date filter
                currentFilters.selectedDate = point.date;
                
                // Update the table and chart
                const tbody = document.querySelector('#backups-content tbody');
                if (tbody) {
                    tbody.innerHTML = renderBackupRows();
                }
                
                // Update chart to show selection
                renderBackupTrendChart();
                // Update summary
                updateSummary();
                
            }
        });
        
        g.appendChild(overlay);
    }

    return {
        init,
        updateBackupsInfo,
        sortBy,
        resetFiltersAndSort,
        clearDateFilter
    };
})();