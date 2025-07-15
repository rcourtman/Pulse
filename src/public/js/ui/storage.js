PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.storage = (() => {
    // Cache for computed values to avoid recalculation
    const contentBadgeCache = new Map();
    const iconCache = new Map();
    const contentBadgeHTMLCache = new Map(); // Cache for complete content badge HTML
    let storageChartData = null; // Cache storage chart data
    let isUpdatingCharts = false; // Prevent concurrent chart updates
    let chartUpdateTimeout = null; // Debounce timer for chart updates
    let currentStorageView = 'node'; // 'node' or 'storage'

    function _initMobileScrollIndicators() {
        const tableContainer = document.querySelector('#storage .table-container');
        const scrollHint = document.querySelector('#storage .scroll-hint');
        
        if (!tableContainer || !scrollHint) return;
        
        let scrollHintTimer;
        
        // Hide scroll hint after 5 seconds or on first scroll
        const hideScrollHint = () => {
            if (scrollHint) {
                scrollHint.style.display = 'none';
            }
        };
        
        scrollHintTimer = setTimeout(hideScrollHint, 5000);
        
        // Handle scroll events
        tableContainer.addEventListener('scroll', () => {
            hideScrollHint();
            clearTimeout(scrollHintTimer);
        }, { passive: true });
        
        // Also hide on table container click/touch
        tableContainer.addEventListener('touchstart', () => {
            hideScrollHint();
            clearTimeout(scrollHintTimer);
        }, { passive: true });
    }

    function _initTableFixedLine() {
        // No longer needed - using CSS border styling instead
    }

    function getStorageTypeIcon(type) {
        if (iconCache.has(type)) {
            return iconCache.get(type);
        }

        let icon;
        switch(type) {
            case 'dir':
                icon = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-yellow-600 dark:text-yellow-400"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>';
                break;
            case 'lvm':
            case 'lvmthin':
                icon = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-purple-600 dark:text-purple-400"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle><path d="M23 21v-2a4 4 0 0 0-3-3.87"></path><path d="M16 3.13a4 4 0 0 1 0 7.75"></path></svg>';
                break;
            case 'zfs':
            case 'zfspool':
                icon = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-red-600 dark:text-red-400"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline></svg>';
                break;
            case 'nfs':
            case 'cifs':
                icon = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-blue-600 dark:text-blue-400"><path d="M16 17l5-5-5-5"></path><path d="M8 17l-5-5 5-5"></path></svg>';
                break;
            case 'cephfs':
            case 'rbd':
                icon = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-indigo-600 dark:text-indigo-400"><path d="M18 8h1a4 4 0 0 1 0 8h-1"></path><path d="M2 8h16v9a4 4 0 0 1-4 4H6a4 4 0 0 1-4-4V8z"></path><line x1="6" y1="1" x2="6" y2="4"></line><line x1="10" y1="1" x2="10" y2="4"></line><line x1="14" y1="1" x2="14" y2="4"></line></svg>';
                break;
            default:
                icon = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-gray-500"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>';
        }
        
        iconCache.set(type, icon);
        return icon;
    }

    function getContentBadgeDetails(contentType) {
        if (contentBadgeCache.has(contentType)) {
            return contentBadgeCache.get(contentType);
        }

        let details = {
            badgeClass: 'bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300',
            tooltip: `Content type: ${contentType}`
        };

        switch(contentType) {
            case 'iso':
                details.badgeClass = 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300';
                details.tooltip = 'ISO images (e.g., for OS installation)';
                break;
            case 'vztmpl':
                details.badgeClass = 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300';
                details.tooltip = 'Container templates';
                break;
            case 'backup':
                details.badgeClass = 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300';
                details.tooltip = 'VM/Container backup files (vzdump)';
                break;
            case 'images':
                details.badgeClass = 'bg-teal-100 dark:bg-teal-900/50 text-teal-700 dark:text-teal-300';
                details.tooltip = 'VM disk images (qcow2, raw, etc.)';
                break;
            case 'rootdir':
                 details.badgeClass = 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300';
                 details.tooltip = 'Storage for container root filesystems';
                 break;
             case 'snippets':
                 details.badgeClass = 'bg-pink-100 dark:bg-pink-900/50 text-pink-700 dark:text-pink-300';
                 details.tooltip = 'Snippet files (e.g., cloud-init configs)';
                 break;
        }
        
        contentBadgeCache.set(contentType, details);
        return details;
    }

    function getContentBadgesHTML(contentString) {
        if (!contentString) return '-';
        
        if (contentBadgeHTMLCache.has(contentString)) {
            return contentBadgeHTMLCache.get(contentString);
        }

        const contentTypes = contentString.split(',').map(ct => ct.trim()).filter(ct => ct);
        
        // Simplify display - just show comma-separated list with subtle styling
        const result = contentTypes.length > 0 
            ? `<span class="text-gray-500 dark:text-gray-400">${contentTypes.join(', ')}</span>`
            : '-';
            
        contentBadgeHTMLCache.set(contentString, result);
        return result;
    }

    function sortNodeStorageData(storageArray) {
        if (!storageArray || !Array.isArray(storageArray)) return [];
        const sortedArray = [...storageArray];
        sortedArray.sort((a, b) => {
            const nameA = String(a.storage || '').toLowerCase();
            const nameB = String(b.storage || '').toLowerCase();
            return nameA.localeCompare(nameB);
        });
        return sortedArray;
    }

    let currentSortOrder = 'name'; // 'name', 'usage-asc', 'usage-desc'

    // Transform node-centric storage data to storage-centric view
    function transformToStorageView(nodes) {
        const storageMap = new Map();
        const pbsStorageByCapacity = new Map(); // Track PBS storages by capacity signature
        
        nodes.forEach(node => {
            if (!node || !node.node || !Array.isArray(node.storage)) return;
            
            node.storage.forEach(store => {
                // For local storage, keep it separate per node
                const isLocal = store.shared === 0;
                const key = isLocal ? `${node.node}:${store.storage}` : store.storage;
                
                // For PBS storage, check if we've seen this capacity signature before
                if (store.type === 'pbs' && store.total > 0) {
                    const capacityKey = `${store.total}-${store.used}-${store.avail}`;
                    if (pbsStorageByCapacity.has(capacityKey)) {
                        // This PBS storage has the same capacity as another - likely the same physical storage
                        const existingKey = pbsStorageByCapacity.get(capacityKey);
                        const existing = storageMap.get(existingKey);
                        if (existing && existing.type === 'pbs') {
                            // Add this as an alias
                            if (!existing.aliases) {
                                existing.aliases = [];
                            }
                            // Check if this alias already exists
                            const aliasExists = existing.aliases.some(alias => alias.name === store.storage);
                            if (!aliasExists) {
                                existing.aliases.push({
                                    name: store.storage,
                                    node: node.node
                                });
                            }
                            // Also add this node to the nodes list if not already there
                            if (!existing.nodes.includes(node.node)) {
                                existing.nodes.push(node.node);
                            }
                            return; // Skip adding as separate entry
                        }
                    } else {
                        pbsStorageByCapacity.set(capacityKey, key);
                    }
                }
                
                if (!storageMap.has(key)) {
                    // First occurrence
                    storageMap.set(key, {
                        storage: store.storage,
                        type: store.type,
                        content: store.content,
                        shared: store.shared,
                        enabled: store.enabled,
                        active: store.active,
                        used: store.used,
                        avail: store.avail,
                        total: store.total,
                        reportingNode: store.total > 0 ? node.node : null,
                        nodes: [node.node],
                        isLocal: isLocal,
                        originalNode: isLocal ? node.node : null
                    });
                } else {
                    // Subsequent occurrences - add node to access list
                    const existing = storageMap.get(key);
                    if (!existing.nodes.includes(node.node)) {
                        existing.nodes.push(node.node);
                    }
                    
                    // Update data from the node that reports actual capacity
                    if (store.total > 0) {
                        existing.used = store.used;
                        existing.avail = store.avail;
                        existing.total = store.total;
                        existing.reportingNode = node.node;
                        // Update enabled/active status if this is the reporting node
                        existing.enabled = store.enabled;
                        existing.active = store.active;
                    }
                }
            });
        });
        
        return Array.from(storageMap.values());
    }

    function calculateStorageSummary(nodes) {
        let totalUsed = 0;
        let totalAvailable = 0;
        let totalCapacity = 0;
        let storageCount = 0;
        let criticalCount = 0;
        let warningCount = 0;

        nodes.forEach(node => {
            if (node && node.storage && Array.isArray(node.storage)) {
                node.storage.forEach(store => {
                    if (store.enabled !== 0 && store.active !== 0) {
                        totalUsed += store.used || 0;
                        totalAvailable += store.avail || 0;
                        totalCapacity += store.total || 0;
                        storageCount++;
                        
                        const usagePercent = store.total > 0 ? (store.used / store.total) * 100 : 0;
                        if (usagePercent >= 90) criticalCount++;
                        else if (usagePercent >= 80) warningCount++;
                    }
                });
            }
        });

        return {
            totalUsed,
            totalAvailable,
            totalCapacity,
            storageCount,
            criticalCount,
            warningCount,
            usagePercent: totalCapacity > 0 ? (totalUsed / totalCapacity) * 100 : 0
        };
    }

    function createStorageSummaryCard(summary) {
        const usageColorClass = PulseApp.utils.getUsageColor(summary.usagePercent);
        
        return `
            <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4 mb-4">
                <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                    <div class="flex-1">
                        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Storage Summary</h3>
                        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
                            <div>
                                <div class="text-gray-500 dark:text-gray-400">Total Storage</div>
                                <div class="font-medium text-gray-900 dark:text-gray-100">${summary.storageCount}</div>
                            </div>
                            <div>
                                <div class="text-gray-500 dark:text-gray-400">Total Capacity</div>
                                <div class="font-medium text-gray-900 dark:text-gray-100">${PulseApp.utils.formatBytes(summary.totalCapacity)}</div>
                            </div>
                            <div class="col-span-2">
                                <div class="text-gray-500 dark:text-gray-400">Storage Usage</div>
                                <div class="font-medium ${usageColorClass}">${PulseApp.utils.formatBytes(summary.totalUsed)} / ${PulseApp.utils.formatBytes(summary.totalCapacity)} (${summary.usagePercent.toFixed(0)}%)</div>
                            </div>
                        </div>
                    </div>
                    <div class="flex-1 max-w-sm">
                        <div class="flex items-center justify-between mb-1">
                            <span class="text-xs text-gray-500 dark:text-gray-400">Overall Usage</span>
                            <span class="text-xs font-medium ${usageColorClass}">${summary.usagePercent.toFixed(1)}%</span>
                        </div>
                        ${PulseApp.utils.createProgressTextBarHTML(summary.usagePercent, '', usageColorClass, '')}
                        ${summary.criticalCount > 0 || summary.warningCount > 0 ? `
                            <div class="flex gap-3 mt-2 text-xs">
                                ${summary.criticalCount > 0 ? `<span class="text-red-600 dark:text-red-400">● ${summary.criticalCount} critical</span>` : ''}
                                ${summary.warningCount > 0 ? `<span class="text-yellow-600 dark:text-yellow-400">● ${summary.warningCount} warning</span>` : ''}
                            </div>
                        ` : ''}
                    </div>
                </div>
            </div>
        `;
    }

    // Incremental table update using DOM diffing (copied from dashboard pattern)
    function _updateStorageTableIncremental(tableBody, storageByNode, sortedNodeNames) {
        const existingRows = new Map();
        const nodeHeaders = new Map();

        // Build map of existing rows
        const children = tableBody.children;
        for (let i = 0; i < children.length; i++) {
            const row = children[i];
            if (row.classList.contains('node-storage-header')) {
                const nodeText = row.querySelector('td').textContent.trim();
                nodeHeaders.set(nodeText, row);
            } else {
                // For storage rows, use a composite key of node+storage
                const cells = row.querySelectorAll('td');
                if (cells.length > 0 && cells[0].querySelector('.sticky-col-content')) {
                    const storageId = cells[0].querySelector('.sticky-col-content').textContent.trim();
                    const nodeId = row.dataset.node;
                    if (storageId && nodeId) {
                        const key = `${nodeId}-${storageId}`;
                        existingRows.set(key, row);
                    }
                }
            }
        }

        // Process each node group
        let currentIndex = 0;
        sortedNodeNames.forEach(nodeName => {
            const nodeStorageData = storageByNode[nodeName] || [];
            
            // Handle node header
            let nodeHeader = nodeHeaders.get(nodeName);
            if (!nodeHeader) {
                // Create new node header
                nodeHeader = PulseApp.ui.common.createTableRow({
                    classes: 'bg-gray-50 dark:bg-gray-700/50 node-storage-header',
                    baseClasses: ''
                });
                nodeHeader.innerHTML = PulseApp.ui.common.generateNodeGroupHeaderCellHTML(nodeName, 7, 'td');
            }
            
            // Move or insert node header at correct position
            if (tableBody.children[currentIndex] !== nodeHeader) {
                tableBody.insertBefore(nodeHeader, tableBody.children[currentIndex] || null);
            }
            currentIndex++;

            if (nodeStorageData.length === 0) {
                // Handle empty state for node
                let emptyRow = tableBody.children[currentIndex];
                if (!emptyRow || !emptyRow.querySelector('[class*="no-storage"]')) {
                    const noDataRow = document.createElement('tr');
                    if (PulseApp.ui.emptyStates) {
                        noDataRow.innerHTML = `<td colspan="7" class="p-0">${PulseApp.ui.emptyStates.createEmptyState('no-storage')}</td>`;
                    } else {
                        noDataRow.innerHTML = `<td colspan="7" class="p-2 px-3 text-sm text-gray-500 dark:text-gray-400 italic">No storage configured or found for this node.</td>`;
                    }
                    if (emptyRow) {
                        tableBody.replaceChild(noDataRow, emptyRow);
                    } else {
                        tableBody.insertBefore(noDataRow, tableBody.children[currentIndex] || null);
                    }
                }
                currentIndex++;
            } else {
                // Process storage rows for this node
                nodeStorageData.forEach(store => {
                    const storeWithNode = { ...store, node: nodeName };
                    const rowKey = `${nodeName}-${store.storage}`;
                    let existingRow = existingRows.get(rowKey);
                    
                    if (existingRow) {
                        // Update existing row if needed
                        _updateStorageRow(existingRow, storeWithNode);
                        if (tableBody.children[currentIndex] !== existingRow) {
                            tableBody.insertBefore(existingRow, tableBody.children[currentIndex] || null);
                        }
                        existingRows.delete(rowKey);
                    } else {
                        // Create new row
                        const newRow = _createStorageRow(storeWithNode);
                        tableBody.insertBefore(newRow, tableBody.children[currentIndex] || null);
                    }
                    currentIndex++;
                });
            }
        });

        // Remove any remaining rows that weren't in the new data
        existingRows.forEach(row => row.remove());
        
        // Remove any orphaned elements
        while (tableBody.children[currentIndex]) {
            tableBody.removeChild(tableBody.children[currentIndex]);
        }
    }

    // Update storage table for storage-centric view
    function _updateStorageTableStorageView(tableBody, storageData) {
        // Clear existing content
        tableBody.innerHTML = '';
        
        // Filter out entries with no capacity (0 B) - these are the greyed out entries
        const filteredData = storageData.filter(store => store.total > 0);
        
        if (filteredData.length === 0) {
            const emptyRow = document.createElement('tr');
            if (PulseApp.ui.emptyStates) {
                emptyRow.innerHTML = `<td colspan="8" class="p-0">${PulseApp.ui.emptyStates.createEmptyState('no-storage')}</td>`;
            } else {
                emptyRow.innerHTML = '<td colspan="8" class="p-4 text-center text-gray-500 dark:text-gray-400">No storage data available.</td>';
            }
            tableBody.appendChild(emptyRow);
            return;
        }
        
        // Sort storage data based on current sort order
        let sortedData = [...filteredData];
        if (currentSortOrder === 'usage-desc') {
            sortedData.sort((a, b) => {
                const percentA = a.total > 0 ? (a.used / a.total) * 100 : 0;
                const percentB = b.total > 0 ? (b.used / b.total) * 100 : 0;
                return percentB - percentA;
            });
        } else if (currentSortOrder === 'usage-asc') {
            sortedData.sort((a, b) => {
                const percentA = a.total > 0 ? (a.used / a.total) * 100 : 0;
                const percentB = b.total > 0 ? (b.used / b.total) * 100 : 0;
                return percentA - percentB;
            });
        } else {
            // Default name sort
            sortedData.sort((a, b) => a.storage.localeCompare(b.storage));
        }
        
        // Create rows for each storage
        sortedData.forEach(store => {
            const row = _createStorageViewRow(store);
            tableBody.appendChild(row);
        });
    }
    
    // Create a row for storage-centric view
    function _createStorageViewRow(store) {
        const row = document.createElement('tr');
        const isDisabled = store.enabled === 0 || store.active === 0;
        const usagePercent = store.total > 0 ? (store.used / store.total) * 100 : 0;
        const isWarning = usagePercent >= 80 && usagePercent < 90;
        const isCritical = usagePercent >= 90;
        
        // Use helper with special row handling
        let specialBgClass = '';
        let additionalClasses = '';
        
        if (isDisabled) {
            additionalClasses = 'opacity-50 grayscale-[50%]';
        }
        if (isCritical) {
            specialBgClass = 'bg-red-50 dark:bg-red-900/10';
        } else if (isWarning) {
            specialBgClass = 'bg-yellow-50 dark:bg-yellow-900/10';
        }
        
        // Replace the existing row element with one from helper
        const newRow = PulseApp.ui.common.createTableRow({
            classes: additionalClasses,
            isSpecialRow: !!(specialBgClass),
            specialBgClass: specialBgClass
        });
        
        // Copy attributes from original row
        row.className = newRow.className;

        const usageTooltipText = `${PulseApp.utils.formatBytes(store.used)} / ${PulseApp.utils.formatBytes(store.total)} (${usagePercent.toFixed(1)}%)`;
        const usageColorClass = PulseApp.utils.getUsageColor(usagePercent);
        const usageBarHTML = PulseApp.utils.createProgressTextBarHTML(usagePercent, usageTooltipText, usageColorClass, `${usagePercent.toFixed(0)}%`);

        const sharedText = store.shared === 1 
            ? '<span class="text-green-600 dark:text-green-400 text-xs">Shared</span>' 
            : '<span class="text-gray-400 dark:text-gray-500 text-xs">Local</span>';

        // Use cached content badge HTML instead of processing inline
        const contentBadges = getContentBadgesHTML(store.content);

        const warningBadge = isCritical ? ' <span class="inline-block w-2 h-2 bg-red-500 rounded-full ml-1"></span>' : 
                            (isWarning ? ' <span class="inline-block w-2 h-2 bg-yellow-500 rounded-full ml-1"></span>' : '');

        // Create sticky storage name column - simplified now
        let storageName = store.storage || 'N/A';
        
        if (store.isLocal) {
            // For local storage, show which node it belongs to
            storageName = `${store.storage} (${store.originalNode})`;
        } else if (store.aliases && store.aliases.length > 0) {
            // For PBS storage with aliases, show the primary name
            // Additional names will be shown in the Nodes column
            storageName = store.storage;
        }
        
        const storageNameContent = `${storageName}${warningBadge}`;
        const stickyStorageCell = PulseApp.ui.common.createStickyColumn(storageNameContent, {
            additionalClasses: 'text-gray-700 dark:text-gray-300 min-w-[150px]'
        });
        row.appendChild(stickyStorageCell);
        
        // Create nodes column
        let nodesContent = '';
        let nodesTitle = '';
        if (store.isLocal) {
            nodesContent = `<span class="text-gray-500 text-xs truncate block">Local to ${store.originalNode}</span>`;
            nodesTitle = `Local to ${store.originalNode}`;
        } else if (store.aliases && store.aliases.length > 0) {
            // For PBS storage with aliases, show all remote names
            const uniqueNames = new Set([store.storage, ...store.aliases.map(a => a.name)]);
            const allNames = Array.from(uniqueNames).join(', ');
            const nodesList = store.nodes.join(', ');
            nodesContent = `
                <div class="text-xs max-w-[150px]">
                    <div class="text-gray-600 dark:text-gray-400 truncate">${nodesList}</div>
                    <div class="text-gray-500 dark:text-gray-500 text-xs truncate">PBS: ${allNames}</div>
                </div>
            `;
            nodesTitle = `Nodes: ${nodesList}\nPBS remotes: ${allNames}`;
        } else {
            // For other shared storage
            const nodesText = store.nodes.join(', ');
            if (store.reportingNode && store.nodes.length > 1) {
                nodesContent = `
                    <div class="text-xs max-w-[150px]">
                        <div class="text-gray-600 dark:text-gray-400 truncate">${nodesText}</div>
                        <div class="text-gray-500 dark:text-gray-500 text-xs truncate">(capacity from ${store.reportingNode})</div>
                    </div>
                `;
                nodesTitle = `Nodes: ${nodesText}\nCapacity reported from: ${store.reportingNode}`;
            } else {
                nodesContent = `<span class="text-xs text-gray-600 dark:text-gray-400 truncate block">${nodesText}</span>`;
                nodesTitle = `Nodes: ${nodesText}`;
            }
        }
        const nodesCell = PulseApp.ui.common.createTableCell(nodesContent, 'p-1 px-2 min-w-[100px] max-w-[150px]');
        nodesCell.title = nodesTitle;
        row.appendChild(nodesCell);
        
        // Create content cell with truncation
        const contentCell = PulseApp.ui.common.createTableCell(
            `<div class="truncate max-w-[120px]">${contentBadges}</div>`, 
            'p-1 px-2 whitespace-nowrap text-xs'
        );
        contentCell.title = store.content || '';
        row.appendChild(contentCell);
        
        row.appendChild(PulseApp.ui.common.createTableCell(store.type || 'N/A', 'p-1 px-2 whitespace-nowrap text-xs'));
        row.appendChild(PulseApp.ui.common.createTableCell(sharedText, 'p-1 px-2 whitespace-nowrap text-center'));
        
        // Create dual content structure for usage cell (progress bar + chart)
        // Use the reporting node for charts (node that has the actual capacity data)
        const nodeForChart = store.reportingNode || store.nodes[0] || store.originalNode || 'unknown';
        const storageId = `${nodeForChart}-${store.storage}`;
        const chartId = `chart-${storageId}-disk`;
        const storageChartHTML = `<div id="${chartId}" class="usage-chart-container h-3.5 w-full" data-storage-id="${store.storage}" data-node="${nodeForChart}"></div>`;
        const usageCellHTML = `<div class="w-full"><div class="metric-text">${usageBarHTML}</div><div class="metric-chart">${storageChartHTML}</div></div>`;
        row.appendChild(PulseApp.ui.common.createTableCell(usageCellHTML, 'p-1 px-2 min-w-[200px]'));
        row.appendChild(PulseApp.ui.common.createTableCell(PulseApp.utils.formatBytes(store.avail), 'p-1 px-2 whitespace-nowrap'));
        row.appendChild(PulseApp.ui.common.createTableCell(PulseApp.utils.formatBytes(store.total), 'p-1 px-2 whitespace-nowrap'));
        return row;
    }

    // Update existing storage row without destroying chart containers
    function _updateStorageRow(row, store) {
        // Only update cells that need updating, preserve chart containers
        const cells = row.querySelectorAll('td');
        if (cells.length < 7) return;

        const usagePercent = store.total > 0 ? (store.used / store.total) * 100 : 0;
        const isWarning = usagePercent >= 80 && usagePercent < 90;
        const isCritical = usagePercent >= 90;
        
        // Update row classes if needed
        const isDisabled = store.enabled === 0 || store.active === 0;
        if (isDisabled && !row.classList.contains('opacity-50')) {
            row.classList.add('opacity-50', 'grayscale-[50%]');
        } else if (!isDisabled && row.classList.contains('opacity-50')) {
            row.classList.remove('opacity-50', 'grayscale-[50%]');
        }

        // Update background color classes
        row.classList.remove('bg-red-50', 'dark:bg-red-900/10', 'bg-yellow-50', 'dark:bg-yellow-900/10');
        if (isCritical) {
            row.classList.add('bg-red-50', 'dark:bg-red-900/10');
        } else if (isWarning) {
            row.classList.add('bg-yellow-50', 'dark:bg-yellow-900/10');
        }

        // Update usage cell - preserve chart container
        const usageCell = cells[4];
        const metricTextDiv = usageCell.querySelector('.metric-text');
        if (metricTextDiv) {
            // Only update the progress bar content
            const usageTooltipText = `${PulseApp.utils.formatBytes(store.used)} / ${PulseApp.utils.formatBytes(store.total)} (${usagePercent.toFixed(1)}%)`;
            const usageColorClass = PulseApp.utils.getUsageColor(usagePercent);
            const usageBarHTML = PulseApp.utils.createProgressTextBarHTML(usagePercent, usageTooltipText, usageColorClass, `${usagePercent.toFixed(0)}%`);
            metricTextDiv.innerHTML = usageBarHTML;
        }

        // Update available and total cells
        cells[5].textContent = PulseApp.utils.formatBytes(store.avail);
        cells[6].textContent = PulseApp.utils.formatBytes(store.total);
    }

    function updateStorageInfo() {
        // Check if we're in charts mode before updating
        const storageContainer = document.getElementById('storage');
        const isChartsMode = storageContainer && storageContainer.classList.contains('charts-mode');

        const nodes = PulseApp.state.get('nodesData') || [];

        // Get the existing table and tbody from the HTML
        const table = document.getElementById('storage-table');
        if (!table) return;
        
        let tbody = table.querySelector('tbody');
        
        if (!Array.isArray(nodes) || nodes.length === 0) {
            if (!tbody) {
                tbody = document.createElement('tbody');
                tbody.className = 'divide-y divide-gray-200 dark:divide-gray-600';
                table.appendChild(tbody);
            }
            tbody.innerHTML = '';
            const emptyRow = document.createElement('tr');
            if (PulseApp.ui.emptyStates) {
                emptyRow.innerHTML = `<td colspan="${colSpan}" class="p-0">${PulseApp.ui.emptyStates.createEmptyState('no-storage')}</td>`;
            } else {
                emptyRow.innerHTML = `<td colspan="${colSpan}" class="p-4 text-center text-gray-500 dark:text-gray-400">No node or storage data available.</td>`;
            }
            tbody.appendChild(emptyRow);
            return;
        }
        
        // Create tbody if it doesn't exist
        if (!tbody) {
            tbody = document.createElement('tbody');
            tbody.className = 'divide-y divide-gray-200 dark:divide-gray-600';
            table.appendChild(tbody);
        }

        // Pre-sort storage data for each node
        const storageByNode = nodes.reduce((acc, node) => {
            if (node && node.node) {
                let storageData = Array.isArray(node.storage) ? [...node.storage] : [];
                
                // Apply current sort order
                if (currentSortOrder === 'usage-desc') {
                    storageData.sort((a, b) => {
                        const percentA = a.total > 0 ? (a.used / a.total) * 100 : 0;
                        const percentB = b.total > 0 ? (b.used / b.total) * 100 : 0;
                        return percentB - percentA; // Descending
                    });
                } else if (currentSortOrder === 'usage-asc') {
                    storageData.sort((a, b) => {
                        const percentA = a.total > 0 ? (a.used / a.total) * 100 : 0;
                        const percentB = b.total > 0 ? (b.used / b.total) * 100 : 0;
                        return percentA - percentB; // Ascending
                    });
                } else {
                    // Default name sort
                    storageData = sortNodeStorageData(storageData);
                }
                
                acc[node.node] = storageData;
            }
            return acc;
        }, {});

        const nodeKeys = Object.keys(storageByNode);

        if (nodeKeys.length === 0) {
            tbody.innerHTML = '';
            const emptyRow = document.createElement('tr');
            if (PulseApp.ui.emptyStates) {
                emptyRow.innerHTML = `<td colspan="${colSpan}" class="p-0">${PulseApp.ui.emptyStates.createEmptyState('no-storage')}</td>`;
            } else {
                emptyRow.innerHTML = `<td colspan="${colSpan}" class="p-4 text-center text-gray-500 dark:text-gray-400">No storage data found associated with nodes.</td>`;
            }
            tbody.appendChild(emptyRow);
            return;
        }

        const sortedNodeNames = Object.keys(storageByNode).sort((a, b) => a.localeCompare(b));

        // Always recreate table header to handle view changes
        let thead = table.querySelector('thead');
        if (!thead) {
            thead = document.createElement('thead');
            table.insertBefore(thead, tbody);
        }
        
        const sortIndicator = (order) => {
            if (currentSortOrder === order) {
                return order === 'usage-desc' ? ' ↓' : ' ↑';
            }
            return '';
        };
        
        const nodesHeader = currentStorageView === 'storage' ? 
            '<th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2 min-w-[100px] max-w-[150px]">Nodes</th>' : '';
        
        const colSpan = currentStorageView === 'storage' ? 8 : 7;
        
        thead.innerHTML = `
            <tr class="border-b border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-700 text-xs font-medium tracking-wider text-left text-gray-600 uppercase dark:text-gray-300">
              <th class="sticky left-0 top-0 bg-gray-50 dark:bg-gray-700 z-20 p-1 px-2 min-w-[150px]">Storage</th>
              ${nodesHeader}
              <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2 ${currentStorageView === 'storage' ? 'max-w-[120px]' : ''}">Content</th>
              <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2">Type</th>
              <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2">Shared</th>
              <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2 cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-600 select-none" id="usage-sort-header">
                Usage${sortIndicator('usage-desc')}${sortIndicator('usage-asc')}
              </th>
              <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2">Avail</th>
              <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2">Total</th>
            </tr>
          `;
        
        // Get the table container for scroll position preservation
        const tableContainer = storageContainer.querySelector('.table-container')

        // Preserve scroll position
        const currentScrollLeft = tableContainer.scrollLeft;
        const currentScrollTop = tableContainer.scrollTop;

        // Calculate dynamic column widths for responsive display
        let maxStorageLength = 0;
        let maxTypeLength = 0;
        
        sortedNodeNames.forEach(nodeName => {
            const nodeStorageData = storageByNode[nodeName];
            nodeStorageData.forEach(store => {
                const storageLength = (store.storage || 'N/A').length;
                const typeLength = (store.type || 'N/A').length;
                if (storageLength > maxStorageLength) maxStorageLength = storageLength;
                if (typeLength > maxTypeLength) maxTypeLength = typeLength;
            });
        });
        
        // Set CSS variables for column widths with responsive limits
        const storageColWidth = Math.min(Math.max(maxStorageLength * 7 + 12, 150), 250);
        const typeColWidth = Math.min(Math.max(maxTypeLength * 7 + 12, 60), 120);
        const htmlElement = document.documentElement;
        if (htmlElement) {
            htmlElement.style.setProperty('--storage-name-col-width', `${storageColWidth}px`);
            htmlElement.style.setProperty('--storage-type-col-width', `${typeColWidth}px`);
        }

        // Use different update methods based on current view
        if (currentStorageView === 'storage') {
            // Transform data to storage-centric view
            const storageData = transformToStorageView(nodes);
            _updateStorageTableStorageView(tbody, storageData);
        } else {
            // Use incremental update for node view
            _updateStorageTableIncremental(tbody, storageByNode, sortedNodeNames);
        }
        
        // Add click handler for sort (only if not already added)
        const usageSortHeader = document.getElementById('usage-sort-header');
        if (usageSortHeader && !usageSortHeader.hasAttribute('data-click-bound')) {
            usageSortHeader.setAttribute('data-click-bound', 'true');
            usageSortHeader.addEventListener('click', () => {
                // Cycle through sort orders: name -> usage-desc -> usage-asc -> name
                if (currentSortOrder === 'name') {
                    currentSortOrder = 'usage-desc';
                } else if (currentSortOrder === 'usage-desc') {
                    currentSortOrder = 'usage-asc';
                } else {
                    currentSortOrder = 'name';
                }
                updateStorageInfo();
            });
        }
        
        // Initialize mobile scroll indicators
        if (window.innerWidth < 768) {
            setTimeout(() => _initMobileScrollIndicators(), 100);
        }
        
        // Initialize fixed table line
        _initTableFixedLine();
        
        // Restore scroll position
        if (tableContainer && (currentScrollLeft > 0 || currentScrollTop > 0)) {
            tableContainer.scrollLeft = currentScrollLeft;
            tableContainer.scrollTop = currentScrollTop;
        }

        // Update charts after table is rendered, but only if in charts mode
        if (isChartsMode) {
            // Use requestAnimationFrame to ensure DOM is fully updated
            requestAnimationFrame(() => {
                updateStorageCharts();
                // Also check time range availability when switching to storage tab
                updateTimeRangeAvailability();
            });
        }
    }

    function createNodeStorageSummaryCard(nodeName, storageList) {
        const card = document.createElement('div');
        card.className = 'bg-white dark:bg-gray-800 shadow-md rounded-lg p-2 border border-gray-200 dark:border-gray-700 flex flex-col gap-1 flex-1 min-w-0 sm:min-w-[280px]';
        
        // Get active storages and sort by usage percentage
        const activeStorages = [];
        
        storageList.forEach(store => {
            if (store.enabled !== 0 && store.active !== 0 && store.total > 0) {
                const usagePercent = (store.used / store.total) * 100;
                activeStorages.push({
                    name: store.storage,
                    total: store.total,
                    used: store.used || 0,
                    avail: store.avail || 0,
                    usagePercent: usagePercent,
                    shared: store.shared === 1,
                    type: store.type
                });
            }
        });
        
        activeStorages.sort((a, b) => b.usagePercent - a.usagePercent);
        
        // Count warnings/critical
        let criticalCount = 0;
        let warningCount = 0;
        activeStorages.forEach(s => {
            if (s.usagePercent >= 90) criticalCount++;
            else if (s.usagePercent >= 80) warningCount++;
        });
        
        card.innerHTML = `
            <div class="flex justify-between items-center">
                <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200 truncate">${nodeName}</h3>
                <div class="flex items-center gap-1">
                    ${criticalCount > 0 ? `<span class="text-[10px] text-red-600 dark:text-red-400">● ${criticalCount}</span>` : ''}
                    ${warningCount > 0 ? `<span class="text-[10px] text-yellow-600 dark:text-yellow-400">● ${warningCount}</span>` : ''}
                    <span class="text-xs text-gray-500 dark:text-gray-400">${activeStorages.length}</span>
                </div>
            </div>
            ${activeStorages.map(storage => {
                const color = PulseApp.utils.getUsageColor(storage.usagePercent);
                const progressColorClass = {
                    red: 'bg-red-500/60 dark:bg-red-500/50',
                    yellow: 'bg-yellow-500/60 dark:bg-yellow-500/50',
                    green: 'bg-green-500/60 dark:bg-green-500/50'
                }[color] || 'bg-gray-500/60 dark:bg-gray-500/50';
                
                return `
                    <div class="text-[10px] text-gray-500 dark:text-gray-400">
                        <div class="flex items-center gap-1 mb-0.5">
                            <span class="font-medium truncate flex-1">${storage.name}:</span>
                            ${storage.shared ? '<span class="text-[9px] text-green-600 dark:text-green-400">●</span>' : ''}
                            <span class="text-[9px]">${storage.usagePercent.toFixed(0)}%</span>
                        </div>
                        <div class="relative w-full h-1 rounded-full overflow-hidden bg-gray-200 dark:bg-gray-600">
                            <div class="absolute top-0 left-0 h-full ${progressColorClass} rounded-full" style="width: ${storage.usagePercent}%;"></div>
                        </div>
                    </div>
                `;
            }).join('')}
        `;
        
        return card;
    }


    function createStorageCard(store, nodeName) {
        const usagePercent = store.total > 0 ? (store.used / store.total) * 100 : 0;
        const isWarning = usagePercent >= 80 && usagePercent < 90;
        const isCritical = usagePercent >= 90;
        const isDisabled = store.enabled === 0 || store.active === 0;
        
        const usageColorClass = PulseApp.utils.getUsageColor(usagePercent);
        const usageBarHTML = PulseApp.utils.createProgressTextBarHTML(usagePercent, '', usageColorClass, `${usagePercent.toFixed(0)}%`);
        
        let cardClasses = 'bg-white dark:bg-gray-800 shadow-md rounded-lg p-3 border border-gray-200 dark:border-gray-700 flex flex-col gap-2 transition-all duration-150 ease-out hover:shadow-lg hover:-translate-y-0.5';
        if (isDisabled) {
            cardClasses += ' opacity-50 grayscale-[50%]';
        }
        if (isCritical) {
            cardClasses += ' ring-2 ring-red-500 border-red-500';
        } else if (isWarning) {
            cardClasses += ' ring-1 ring-yellow-500 border-yellow-500';
        }
        
        const contentTypes = store.content ? store.content.split(',').map(ct => ct.trim()).filter(ct => ct) : [];
        const sharedBadge = store.shared === 1 
            ? '<span class="text-[10px] bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 px-1.5 py-0.5 rounded">Shared</span>' 
            : '';
        
        const card = document.createElement('div');
        card.className = cardClasses;
        
        card.innerHTML = `
            <div class="flex justify-between items-start">
                <div class="flex-1 min-w-0">
                    <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200 truncate" title="${store.storage || 'N/A'}">${store.storage || 'N/A'}</h3>
                    <div class="text-[10px] text-gray-500 dark:text-gray-400">${nodeName}</div>
                </div>
                <div class="flex items-center gap-1">
                    ${sharedBadge}
                    ${isCritical ? '<span class="w-2 h-2 bg-red-500 rounded-full"></span>' : (isWarning ? '<span class="w-2 h-2 bg-yellow-500 rounded-full"></span>' : '')}
                </div>
            </div>
            
            <div class="space-y-1">
                <div class="flex justify-between text-[11px] text-gray-500 dark:text-gray-400">
                    <span>Type: ${store.type || 'N/A'}</span>
                    <span class="${usageColorClass} font-medium">${usagePercent.toFixed(0)}%</span>
                </div>
                ${usageBarHTML}
                <div class="flex justify-between text-[11px] text-gray-500 dark:text-gray-400">
                    <span>${PulseApp.utils.formatBytes(store.used)} used</span>
                    <span>${PulseApp.utils.formatBytes(store.avail)} free</span>
                </div>
            </div>
            
            ${contentTypes.length > 0 ? `
                <div class="text-[10px] text-gray-500 dark:text-gray-400 pt-1 border-t border-gray-100 dark:border-gray-700">
                    <span class="font-medium">Content:</span> ${contentTypes.join(', ')}
                </div>
            ` : ''}
        `;
        
        return card;
    }

    function _createStorageRow(store) {
        const row = document.createElement('tr');
        row.dataset.node = store.node; // Add node data attribute for incremental updates
        const isDisabled = store.enabled === 0 || store.active === 0;
        const usagePercent = store.total > 0 ? (store.used / store.total) * 100 : 0;
        const isWarning = usagePercent >= 80 && usagePercent < 90;
        const isCritical = usagePercent >= 90;
        
        // Use helper with special row handling
        let specialBgClass = '';
        let additionalClasses = '';
        
        if (isDisabled) {
            additionalClasses = 'opacity-50 grayscale-[50%]';
        }
        if (isCritical) {
            specialBgClass = 'bg-red-50 dark:bg-red-900/10';
        } else if (isWarning) {
            specialBgClass = 'bg-yellow-50 dark:bg-yellow-900/10';
        }
        
        // Replace the existing row element with one from helper
        const newRow = PulseApp.ui.common.createTableRow({
            classes: additionalClasses,
            isSpecialRow: !!(specialBgClass),
            specialBgClass: specialBgClass
        });
        
        // Copy attributes from original row
        row.className = newRow.className;

        const usageTooltipText = `${PulseApp.utils.formatBytes(store.used)} / ${PulseApp.utils.formatBytes(store.total)} (${usagePercent.toFixed(1)}%)`;
        const usageColorClass = PulseApp.utils.getUsageColor(usagePercent);
        const usageBarHTML = PulseApp.utils.createProgressTextBarHTML(usagePercent, usageTooltipText, usageColorClass, `${usagePercent.toFixed(0)}%`);

        const sharedText = store.shared === 1 
            ? '<span class="text-green-600 dark:text-green-400 text-xs">Shared</span>' 
            : '<span class="text-gray-400 dark:text-gray-500 text-xs">Local</span>';

        // Use cached content badge HTML instead of processing inline
        const contentBadges = getContentBadgesHTML(store.content);

        const warningBadge = isCritical ? ' <span class="inline-block w-2 h-2 bg-red-500 rounded-full ml-1"></span>' : 
                            (isWarning ? ' <span class="inline-block w-2 h-2 bg-yellow-500 rounded-full ml-1"></span>' : '');

        // Create sticky storage name column
        const storageNameContent = `${store.storage || 'N/A'}${warningBadge}`;
        const stickyStorageCell = PulseApp.ui.common.createStickyColumn(storageNameContent, {
            additionalClasses: 'text-gray-700 dark:text-gray-300'
        });
        row.appendChild(stickyStorageCell);
        
        // Create regular cells
        row.appendChild(PulseApp.ui.common.createTableCell(contentBadges, 'p-1 px-2 whitespace-nowrap text-xs'));
        row.appendChild(PulseApp.ui.common.createTableCell(store.type || 'N/A', 'p-1 px-2 whitespace-nowrap text-xs'));
        row.appendChild(PulseApp.ui.common.createTableCell(sharedText, 'p-1 px-2 whitespace-nowrap text-center'));
        // Create dual content structure for usage cell (progress bar + chart)
        const storageId = `${store.node}-${store.storage}`;
        const chartId = `chart-${storageId}-disk`;
        const storageChartHTML = `<div id="${chartId}" class="usage-chart-container h-3.5 w-full" data-storage-id="${store.storage}" data-node="${store.node}"></div>`;
        const usageCellHTML = `<div class="w-full"><div class="metric-text">${usageBarHTML}</div><div class="metric-chart">${storageChartHTML}</div></div>`;
        row.appendChild(PulseApp.ui.common.createTableCell(usageCellHTML, 'p-1 px-2 min-w-[200px]'));
        row.appendChild(PulseApp.ui.common.createTableCell(PulseApp.utils.formatBytes(store.avail), 'p-1 px-2 whitespace-nowrap'));
        row.appendChild(PulseApp.ui.common.createTableCell(PulseApp.utils.formatBytes(store.total), 'p-1 px-2 whitespace-nowrap'));
        return row;
    }

    function resetSort() {
        // Reset sort order to default (name)
        currentSortOrder = 'name';
        
        // Update the storage table with default sort
        updateStorageInfo();
    }

    function toggleStorageChartsMode() {
        const storageContainer = document.getElementById('storage');
        const checkbox = document.getElementById('toggle-storage-charts-checkbox');
        const label = checkbox ? checkbox.parentElement : null;
        
        if (checkbox && checkbox.checked) {
            // Switch to charts mode  
            storageContainer.classList.add('charts-mode');
            if (label) label.title = 'Toggle Progress View';
            
            // Show storage-specific chart controls
            const storageChartControls = document.getElementById('storage-chart-controls');
            if (storageChartControls) {
                storageChartControls.classList.remove('hidden');
            }
            
            // CSS classes will handle visibility - no need to set inline styles
            
            // Start fetching chart data if needed and update charts
            if (PulseApp.charts) {
                // First ensure the table is rendered with chart containers
                updateStorageInfo();
                // Then update charts after DOM is ready
                setTimeout(() => {
                    // Fetch initial data if needed
                    updateStorageCharts(false);
                    // Also update time range availability after initial fetch
                    setTimeout(() => updateTimeRangeAvailability(), 100);
                }, 100);
            }
        } else {
            // Switch to progress bars mode
            storageContainer.classList.remove('charts-mode');
            if (label) label.title = 'Toggle Charts View';
            
            // Hide storage-specific chart controls
            const storageChartControls = document.getElementById('storage-chart-controls');
            if (storageChartControls) {
                storageChartControls.classList.add('hidden');
            }
            
            // Remove inline styles to let CSS classes take over
            const storageTab = document.getElementById('storage');
            if (storageTab) {
                storageTab.querySelectorAll('.metric-text').forEach(el => {
                    el.style.display = '';  // Remove inline style
                });
                storageTab.querySelectorAll('.metric-chart').forEach(el => {
                    el.style.display = '';  // Remove inline style
                });
            }
        }
    }

    async function updateStorageCharts(forceRefetch = false) {
        const storageTab = document.getElementById('storage');
        if (!storageTab || isUpdatingCharts) return;
        
        isUpdatingCharts = true;

        try {
            // Only fetch new data if we don't have any cached data yet
            // The API returns ALL data regardless of time range, so we only need to fetch once
            if (!storageChartData) {
                // Fetch storage chart data from API (range parameter is ignored by the server)
                const response = await fetch('/api/storage-charts?range=10080'); // Just use max range
                if (!response.ok) {
                    console.error('[Storage Charts] Failed to fetch storage chart data:', response.status);
                    if (response.status === 429) {
                        // Rate limited - show a message to the user
                        console.warn('[Storage Charts] Rate limited. Please wait a moment before trying again.');
                    }
                    isUpdatingCharts = false;
                    return;
                }
                
                const chartData = await response.json();
                storageChartData = chartData.data || {};
            }
            
            // Get current time range for client-side filtering/display
            const checkedTimeRange = document.querySelector('input[name="storage-time-range"]:checked');
            const timeRange = parseInt(checkedTimeRange ? checkedTimeRange.value : '60');
            const now = Date.now();
            const cutoffTime = now - (timeRange * 60 * 1000);
            
            const storageData = storageChartData;
            
            // Update each storage chart container
            const chartContainers = storageTab.querySelectorAll('.usage-chart-container');
            
            chartContainers.forEach(container => {
                const storageId = container.dataset.storageId;
                const nodeId = container.dataset.node;
                
                if (storageId && nodeId) {
                    const fullStorageId = `${nodeId}-${storageId}`;
                    const storageMetrics = storageData[fullStorageId];
                    
                    if (storageMetrics && storageMetrics.usage && storageMetrics.usage.length > 0) {
                        // Filter data based on selected time range
                        const filteredData = storageMetrics.usage.filter(point => point.timestamp >= cutoffTime);
                        
                        if (filteredData.length > 0) {
                            // Use the existing chart system to create usage charts
                            if (PulseApp.charts && PulseApp.charts.createOrUpdateChart) {
                                // The container should already have the correct ID from _createStorageRow
                                const chartId = container.id;
                                
                                // Only update if container is still in DOM and visible
                                if (container.offsetParent !== null) {
                                    PulseApp.charts.createOrUpdateChart(
                                        chartId,
                                        filteredData,
                                        'disk',
                                        'mini',
                                        fullStorageId
                                    );
                                }
                            }
                        } else {
                            // No data in selected time range
                            if (!container.querySelector('svg')) {
                                container.innerHTML = '<div class="text-[9px] text-gray-400 text-center leading-4">NO DATA</div>';
                            }
                        }
                    } else {
                        // No data available - show placeholder only if container is empty
                        if (!container.querySelector('svg')) {
                            container.innerHTML = '<div class="text-[9px] text-gray-400 text-center leading-4">DISK</div>';
                        }
                    }
                }
            });
            
        } catch (error) {
            console.error('[Storage Charts] Error updating storage charts:', error);
        } finally {
            isUpdatingCharts = false;
            // Update time range availability after chart update
            updateTimeRangeAvailability();
        }
    }

    function updateTimeRangeAvailability() {
        // For storage charts, check actual data age to grey out unavailable time ranges
        if (!storageChartData) return;
        
        // Find the oldest data point across all storages
        let oldestDataTime = null;
        Object.values(storageChartData).forEach(storage => {
            if (storage.usage && storage.usage.length > 0) {
                const firstDataPoint = storage.usage[0];
                if (firstDataPoint && firstDataPoint.timestamp) {
                    const dataTime = firstDataPoint.timestamp;
                    if (!oldestDataTime || dataTime < oldestDataTime) {
                        oldestDataTime = dataTime;
                    }
                }
            }
        });
        
        if (!oldestDataTime) {
            // No data at all - disable everything
            const timeRangeInputs = document.querySelectorAll('input[name="storage-time-range"]');
            const timeRangeLabels = document.querySelectorAll('label[data-storage-time-range]');
            
            timeRangeInputs.forEach(input => {
                input.disabled = true;
            });
            
            timeRangeLabels.forEach(label => {
                label.classList.add('opacity-50', 'cursor-not-allowed');
                label.classList.remove('cursor-pointer', 'hover:bg-gray-50', 'dark:hover:bg-gray-700');
                label.title = 'No data available yet';
            });
            return;
        }
        
        // Calculate data age in minutes
        const now = Date.now();
        const dataAgeMinutes = (now - oldestDataTime) / (1000 * 60);
        
        // Check each time range
        const timeRangeInputs = document.querySelectorAll('input[name="storage-time-range"]');
        const timeRangeLabels = document.querySelectorAll('label[data-storage-time-range]');
        
        timeRangeInputs.forEach(input => {
            const rangeMinutes = parseInt(input.value);
            const hasEnoughData = dataAgeMinutes >= rangeMinutes * 0.8; // Allow if we have at least 80% of the range
            input.disabled = !hasEnoughData;
        });
        
        timeRangeLabels.forEach(label => {
            const rangeMinutes = parseInt(label.dataset.storageTimeRange);
            const hasEnoughData = dataAgeMinutes >= rangeMinutes * 0.8;
            
            if (!hasEnoughData) {
                label.classList.add('opacity-50', 'cursor-not-allowed');
                label.classList.remove('cursor-pointer', 'hover:bg-gray-50', 'dark:hover:bg-gray-700');
                
                // Calculate how much more time is needed
                const minutesNeeded = Math.ceil(rangeMinutes * 0.8 - dataAgeMinutes);
                if (minutesNeeded > 60) {
                    const hoursNeeded = Math.ceil(minutesNeeded / 60);
                    label.title = `Need ${hoursNeeded}h more data`;
                } else {
                    label.title = `Need ${minutesNeeded}m more data`;
                }
            } else {
                label.classList.remove('opacity-50', 'cursor-not-allowed');
                label.classList.add('cursor-pointer', 'hover:bg-gray-50', 'dark:hover:bg-gray-700');
                label.title = '';
            }
        });
        
        // If current selection is disabled, switch to the smallest available range
        const currentRadio = document.querySelector('input[name="storage-time-range"]:checked');
        if (currentRadio && currentRadio.disabled) {
            console.log('[Storage] Current time range is disabled, switching to available range');
            const firstEnabledRadio = document.querySelector('input[name="storage-time-range"]:not(:disabled)');
            if (firstEnabledRadio) {
                console.log('[Storage] Switching to time range:', firstEnabledRadio.value);
                firstEnabledRadio.checked = true;
                firstEnabledRadio.dispatchEvent(new Event('change', { bubbles: true }));
            } else {
                console.log('[Storage] No enabled time ranges available');
            }
        }
    }

    function init() {
        // Initialize storage charts toggle
        const storageChartsToggleCheckbox = document.getElementById('toggle-storage-charts-checkbox');
        if (storageChartsToggleCheckbox) {
            storageChartsToggleCheckbox.addEventListener('change', toggleStorageChartsMode);
        }
        
        // Initialize storage view toggle
        const storageViewInputs = document.querySelectorAll('input[name="storage-view"]');
        
        // Restore saved storage view preference
        const savedStorageView = localStorage.getItem('pulseStorageView');
        if (savedStorageView) {
            currentStorageView = savedStorageView;
            const savedInput = document.querySelector(`input[name="storage-view"][value="${savedStorageView}"]`);
            if (savedInput) {
                savedInput.checked = true;
            }
        }
        
        storageViewInputs.forEach(input => {
            input.addEventListener('change', (e) => {
                currentStorageView = e.target.value;
                localStorage.setItem('pulseStorageView', currentStorageView);
                updateStorageInfo();
            });
        });

        // Initialize storage time range controls
        const storageTimeRangeInputs = document.querySelectorAll('input[name="storage-time-range"]');
        
        // Restore saved time range preference
        const savedTimeRange = localStorage.getItem('pulseStorageChartTimeRange');
        if (savedTimeRange) {
            const savedInput = document.querySelector(`input[name="storage-time-range"][value="${savedTimeRange}"]`);
            if (savedInput && !savedInput.disabled) {
                savedInput.checked = true;
            }
        }
        
        storageTimeRangeInputs.forEach(input => {
            input.addEventListener('change', () => {
                // Save the selected time range to localStorage
                localStorage.setItem('pulseStorageChartTimeRange', input.value);
                
                // Only update charts if charts mode is active
                const storageContainer = document.getElementById('storage');
                if (storageContainer && storageContainer.classList.contains('charts-mode')) {
                    // Debounce chart updates to prevent rapid clicking issues
                    if (chartUpdateTimeout) {
                        clearTimeout(chartUpdateTimeout);
                    }
                    chartUpdateTimeout = setTimeout(() => {
                        updateStorageCharts(false); // Don't refetch, just re-render with new time range
                    }, 100);
                }
            });
        });
        
        // Check time range availability on init if charts mode is already active
        const storageContainer = document.getElementById('storage');
        if (storageContainer && storageContainer.classList.contains('charts-mode')) {
            // Small delay to ensure everything is loaded
            setTimeout(() => updateTimeRangeAvailability(), 100);
        }
    }

    return {
        updateStorageInfo,
        resetSort,
        toggleStorageChartsMode,
        init
    };
})();
