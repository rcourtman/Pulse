PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.storage = (() => {
    // Cache for computed values to avoid recalculation
    const contentBadgeCache = new Map();
    const iconCache = new Map();
    const contentBadgeHTMLCache = new Map(); // Cache for complete content badge HTML

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
                            <div>
                                <div class="text-gray-500 dark:text-gray-400">Used</div>
                                <div class="font-medium text-gray-900 dark:text-gray-100">${PulseApp.utils.formatBytes(summary.totalUsed)}</div>
                            </div>
                            <div>
                                <div class="text-gray-500 dark:text-gray-400">Available</div>
                                <div class="font-medium text-gray-900 dark:text-gray-100">${PulseApp.utils.formatBytes(summary.totalAvailable)}</div>
                            </div>
                        </div>
                    </div>
                    <div class="flex-1 max-w-sm">
                        <div class="flex items-center justify-between mb-1">
                            <span class="text-xs text-gray-600 dark:text-gray-400">Overall Usage</span>
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

    function updateStorageInfo() {
        const contentDiv = document.getElementById('storage-info-content');
        if (!contentDiv) return;
        
        // Find existing table container and preserve its scroll position
        const existingTableContainer = contentDiv.querySelector('.table-container');
        const currentScrollLeft = existingTableContainer ? existingTableContainer.scrollLeft : 0;
        const currentScrollTop = existingTableContainer ? existingTableContainer.scrollTop : 0;
        
        contentDiv.innerHTML = '';
        contentDiv.className = '';

        const nodes = PulseApp.state.get('nodesData') || [];

        if (!Array.isArray(nodes) || nodes.length === 0) {
            if (PulseApp.ui.emptyStates) {
                contentDiv.innerHTML = PulseApp.ui.emptyStates.createEmptyState('no-storage');
            } else {
                contentDiv.innerHTML = '<p class="text-gray-500 dark:text-gray-400 p-4 text-center">No node or storage data available.</p>';
            }
            return;
        }

        const container = document.createElement('div');

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
          if (PulseApp.ui.emptyStates) {
              container.innerHTML += PulseApp.ui.emptyStates.createEmptyState('no-storage');
          } else {
              container.innerHTML += '<p class="text-gray-500 dark:text-gray-400 p-4 text-center">No storage data found associated with nodes.</p>';
          }
          contentDiv.appendChild(container);
          return;
        }

        const sortedNodeNames = Object.keys(storageByNode).sort((a, b) => a.localeCompare(b));

        // Add node storage summary cards
        const summaryCardsContainer = document.createElement('div');
        summaryCardsContainer.className = 'mb-3';
        
        const cardsGrid = document.createElement('div');
        cardsGrid.className = 'grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3';
        
        sortedNodeNames.forEach(nodeName => {
            const nodeStorageData = storageByNode[nodeName];
            if (nodeStorageData.length > 0) {
                const card = createNodeStorageSummaryCard(nodeName, nodeStorageData);
                cardsGrid.appendChild(card);
            }
        });
        
        summaryCardsContainer.appendChild(cardsGrid);
        container.appendChild(summaryCardsContainer);

        // Table view with scroll container
        const tableContainer = document.createElement('div');
        tableContainer.className = 'table-container max-h-[80vh] overflow-y-auto overflow-x-auto border border-gray-200 dark:border-gray-700 rounded overflow-hidden scrollbar';
        
        const table = document.createElement('table');
        table.className = 'w-full text-sm border-collapse table-auto min-w-full';

            const thead = document.createElement('thead');
            const sortIndicator = (order) => {
                if (currentSortOrder === order) {
                    return order === 'usage-desc' ? ' ↓' : ' ↑';
                }
                return '';
            };
            
            thead.innerHTML = `
                <tr class="border-b border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-700 text-xs font-medium tracking-wider text-left text-gray-600 uppercase dark:text-gray-300">
                  <th class="sticky left-0 top-0 bg-gray-50 dark:bg-gray-700 z-20 p-1 px-2 border-r border-gray-300 dark:border-gray-600">Storage</th>
                  <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2">Content</th>
                  <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2">Type</th>
                  <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2">Shared</th>
                  <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2 cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-600 select-none" id="usage-sort-header">
                    Usage${sortIndicator('usage-desc')}${sortIndicator('usage-asc')}
                  </th>
                  <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2">Avail</th>
                  <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2">Total</th>
                </tr>
              `;
            table.appendChild(thead);

            const tbody = document.createElement('tbody');
            tbody.className = 'divide-y divide-gray-200 dark:divide-gray-600';

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
        const storageColWidth = Math.min(Math.max(maxStorageLength * 7 + 12, 100), 200);
        const typeColWidth = Math.min(Math.max(maxTypeLength * 7 + 12, 60), 120);
        const htmlElement = document.documentElement;
        if (htmlElement) {
            htmlElement.style.setProperty('--storage-name-col-width', `${storageColWidth}px`);
            htmlElement.style.setProperty('--storage-type-col-width', `${typeColWidth}px`);
        }

        sortedNodeNames.forEach(nodeName => {
          const nodeStorageData = storageByNode[nodeName]; // Already sorted

          const nodeHeaderRow = document.createElement('tr');
          nodeHeaderRow.className = 'bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs node-storage-header';
          nodeHeaderRow.innerHTML = PulseApp.ui.common.generateNodeGroupHeaderCellHTML(`${nodeName}`, 7, 'td');
          tbody.appendChild(nodeHeaderRow);

          if (nodeStorageData.length === 0) {
            const noDataRow = document.createElement('tr');
            if (PulseApp.ui.emptyStates) {
                noDataRow.innerHTML = `<td colspan="7" class="p-0">${PulseApp.ui.emptyStates.createEmptyState('no-storage')}</td>`;
            } else {
                noDataRow.innerHTML = `<td colspan="7" class="p-2 px-3 text-sm text-gray-500 dark:text-gray-400 italic">No storage configured or found for this node.</td>`;
            }
            tbody.appendChild(noDataRow);
            return;
          }

          // Use pre-sorted data
          nodeStorageData.forEach(store => {
              const row = _createStorageRow(store);
              tbody.appendChild(row);
          });
        });

        table.appendChild(tbody);
        tableContainer.appendChild(table);
        container.appendChild(tableContainer);
        contentDiv.appendChild(container);
        
        // Add click handler for sort
        const usageSortHeader = document.getElementById('usage-sort-header');
        if (usageSortHeader) {
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
        
        // Restore scroll position to the new table container with multiple timing strategies
        if (tableContainer && (currentScrollLeft > 0 || currentScrollTop > 0)) {
            const restoreScroll = () => {
                tableContainer.scrollLeft = currentScrollLeft;
                tableContainer.scrollTop = currentScrollTop;
            };
            
            // Multiple restoration attempts with different timing
            setTimeout(restoreScroll, 0);
            setTimeout(restoreScroll, 16);
            setTimeout(restoreScroll, 50);
            setTimeout(restoreScroll, 100);
            requestAnimationFrame(restoreScroll);
            
            // Final verification and fallback
            setTimeout(() => {
                if (Math.abs(tableContainer.scrollTop - currentScrollTop) > 10) {
                    tableContainer.scrollTo({
                        top: currentScrollTop,
                        left: currentScrollLeft,
                        behavior: 'instant'
                    });
                }
            }, 200);
        }
    }

    function createNodeStorageSummaryCard(nodeName, storageList) {
        const card = document.createElement('div');
        card.className = 'bg-white dark:bg-gray-800 shadow-md rounded-lg p-2 border border-gray-200 dark:border-gray-700 flex flex-col gap-1';
        
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
        
        // Sort by usage percentage (most full first)
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
                    <div class="text-[10px] text-gray-600 dark:text-gray-400">
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
                <div class="flex justify-between text-[11px] text-gray-600 dark:text-gray-400">
                    <span>Type: ${store.type || 'N/A'}</span>
                    <span class="${usageColorClass} font-medium">${usagePercent.toFixed(0)}%</span>
                </div>
                ${usageBarHTML}
                <div class="flex justify-between text-[11px] text-gray-600 dark:text-gray-400">
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
        const isDisabled = store.enabled === 0 || store.active === 0;
        const usagePercent = store.total > 0 ? (store.used / store.total) * 100 : 0;
        const isWarning = usagePercent >= 80 && usagePercent < 90;
        const isCritical = usagePercent >= 90;
        
        let rowClasses = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700';
        if (isDisabled) {
            rowClasses += ' opacity-50 grayscale-[50%]';
        }
        if (isCritical) {
            rowClasses += ' bg-red-50 dark:bg-red-900/10';
        } else if (isWarning) {
            rowClasses += ' bg-yellow-50 dark:bg-yellow-900/10';
        }
        row.className = rowClasses;

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

        row.innerHTML = `
            <td class="sticky left-0 ${isCritical ? 'bg-red-50 dark:bg-red-900/20' : (isWarning ? 'bg-yellow-50 dark:bg-yellow-900/20' : 'bg-white dark:bg-gray-800')} z-10 p-1 px-2 whitespace-nowrap overflow-hidden text-ellipsis max-w-0 text-gray-900 dark:text-gray-100 border-r border-gray-300 dark:border-gray-600">${store.storage || 'N/A'}${warningBadge}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300 text-xs">${contentBadges}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300 text-xs">${store.type || 'N/A'}</td>
            <td class="p-1 px-2 whitespace-nowrap text-center">${sharedText}</td>
            <td class="p-1 px-2 text-gray-600 dark:text-gray-300 min-w-[250px]">${usageBarHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300">${PulseApp.utils.formatBytes(store.avail)}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300">${PulseApp.utils.formatBytes(store.total)}</td>
        `;
        return row;
    }

    return {
        updateStorageInfo
    };
})();
