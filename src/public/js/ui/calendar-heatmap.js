PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.calendarHeatmap = (() => {
    
    // Global selection management
    let currentSelectedCell = null;
    let currentSelectedDate = null;
    let onDateSelectCallback = null;
    let preservedSelectedDate = null; // Preserve selection across updates
    let currentDisplayMonth = null; // Current month being displayed (preserved across updates)
    let hasAutoSelectedToday = false; // Prevent repeated auto-selection on API updates
    
    // Helper function to get current backup type filter from global state
    function getCurrentFilterType() {
        return PulseApp.state.get('backupsFilterBackupType') || 'all';
    }
    
    // Helper function to filter tasks by guest with node awareness
    function filterTasksByGuest(tasks, guestId) {
        if (!guestId) return tasks;
        
        return tasks.filter(task => {
            // Match vmid
            const taskVmid = task.vmid || task.guestId;
            if (parseInt(taskVmid, 10) !== parseInt(guestId, 10)) return false;
            
            // For single guest filtering, we need to get the guest node info
            const vmsData = PulseApp.state.get('vmsData') || [];
            const containersData = PulseApp.state.get('containersData') || [];
            const allGuests = [...vmsData, ...containersData];
            const guest = allGuests.find(g => parseInt(g.vmid, 10) === parseInt(guestId, 10));
            
            if (!guest) return true; // Fallback if guest not found
            
            // For PBS tasks (centralized), don't filter by node
            if (task.source === 'pbs') return true;
            
            // For PVE tasks (node-specific), match node/endpoint
            const taskNode = task.node;
            const taskEndpoint = task.endpointId;
            
            // Match by node if available
            if (guest.node && taskNode) {
                return taskNode === guest.node;
            }
            
            // Match by endpointId if available
            if (guest.endpointId && taskEndpoint) {
                return taskEndpoint === guest.endpointId;
            }
            
            // If no node/endpoint info available, include it (fallback)
            return true;
        });
    }
    
    // Helper function to generate unique guest key including node information
    function generateUniqueGuestKey(vmid, backupItem) {
        const itemNode = backupItem.node;
        const itemEndpoint = backupItem.endpointId;
        
        // Create unique key with node/endpoint information
        if (itemNode) {
            return `${vmid}-${itemNode}`;
        }
        if (itemEndpoint) {
            return `${vmid}-${itemEndpoint}`;
        }
        
        // Fallback to simple vmid if no node info
        return vmid.toString();
    }
    
    // Helper function to extract vmid from unique guest key
    function extractVmidFromGuestKey(guestKey) {
        // If it contains a dash, take the part before the first dash
        const dashIndex = guestKey.indexOf('-');
        return dashIndex !== -1 ? guestKey.substring(0, dashIndex) : guestKey;
    }
    
    // Helper function to check if a guest (with potential node info) is in the filtered list
    function isGuestInFilteredList(vmid, backupItem, filteredGuestIds) {
        if (!filteredGuestIds || filteredGuestIds.length === 0) return true;
        
        // Generate the unique key for this guest
        const uniqueKey = generateUniqueGuestKey(vmid, backupItem);
        
        // Check if this unique key is in the filtered list
        if (filteredGuestIds.includes(uniqueKey)) return true;
        
        // Also check for simple vmid match (backward compatibility)
        if (filteredGuestIds.includes(vmid.toString())) return true;
        
        return false;
    }
    
    const CSS_CLASSES = {
        CALENDAR_CONTAINER: 'calendar-heatmap-container max-w-4xl mx-auto',
        MONTH_LABEL: 'text-xs text-gray-600 dark:text-gray-400 font-medium mb-1',
        SELECTED_CELL: ['ring-2', 'ring-blue-500', 'dark:ring-blue-400', 'scale-110', 'z-10'],
        LEGEND_CONTAINER: 'flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400',
        LEGEND_BOX: 'w-3 h-3 rounded-sm',
        YEAR_NAVIGATION: 'flex items-center justify-between mb-4',
        YEAR_BUTTON: 'text-sm text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300 cursor-pointer',
        CURRENT_YEAR: 'text-sm font-medium text-gray-700 dark:text-gray-300'
    };

    const BACKUP_COLORS = {
        NONE: 'bg-gray-200 dark:bg-gray-700',
        LOW: 'bg-green-200 dark:bg-green-900',
        MEDIUM: 'bg-green-400 dark:bg-green-700',
        HIGH: 'bg-green-600 dark:bg-green-500',
        FAILED: 'border-2 border-red-500 dark:border-red-400',
        MIXED: 'bg-gradient-to-br from-green-400 to-yellow-400 dark:from-green-700 dark:to-yellow-700'
    };

    function createCalendarHeatmap(backupData, guestId = null, filteredGuestIds = null, onDateSelect = null, isUserAction = true) {
        const container = document.createElement('div');
        container.className = CSS_CLASSES.CALENDAR_CONTAINER;

        // Store the date selection callback
        onDateSelectCallback = onDateSelect;
        
        // Set initial display month to current month (only if not already set)
        if (!currentDisplayMonth) {
            currentDisplayMonth = new Date();
            currentDisplayMonth.setDate(1); // Set to first of month
        }

        // Month navigation
        const monthNav = createMonthNavigation(container, backupData, guestId, filteredGuestIds);
        container.appendChild(monthNav);
        
        // Calendar grid
        const calendarGrid = createSingleMonthCalendar(backupData, guestId, filteredGuestIds);
        container.appendChild(calendarGrid);
        
        // Legend
        const legend = createLegend();
        container.appendChild(legend);
        
        // Auto-select today's date if filtering to specific guest (only on user actions)
        if (guestId && isUserAction && !hasAutoSelectedToday) {
            hasAutoSelectedToday = true;
            setTimeout(() => {
                const today = new Date();
                const utcToday = new Date(Date.UTC(today.getFullYear(), today.getMonth(), today.getDate()));
                const todayKey = utcToday.toISOString().split('T')[0];
                const todayCell = container.querySelector(`[data-date="${todayKey}"]`);
                
                if (todayCell && (todayCell.classList.contains('bg-slate-600') || 
                                 todayCell.classList.contains('bg-purple-500') || 
                                 todayCell.classList.contains('bg-orange-500') || 
                                 todayCell.classList.contains('bg-yellow-500'))) {
                    // Auto-click today's cell if it has backup data
                    todayCell.click();
                }
            }, 100);
        }
        
        // Restore preserved selection on API updates (not user actions)
        if (!isUserAction && preservedSelectedDate) {
            setTimeout(() => {
                const cellToSelect = container.querySelector(`[data-date="${preservedSelectedDate}"]`);
                if (cellToSelect && onDateSelectCallback) {
                    // Find the day data for this cell
                    const allData = processBackupDataForSingleMonth(backupData, currentDisplayMonth, guestId, filteredGuestIds);
                    const dayData = allData[preservedSelectedDate];
                    if (dayData) {
                        // Manually restore selection without animation
                        cellToSelect.classList.add(...CSS_CLASSES.SELECTED_CELL);
                        currentSelectedCell = cellToSelect;
                        currentSelectedDate = preservedSelectedDate;
                        
                        // Prepare callback data
                        const callbackData = {
                            date: preservedSelectedDate,
                            backups: dayData.guests || [],
                            stats: {
                                totalGuests: dayData.guests ? dayData.guests.length : 0,
                                pbsCount: 0,
                                pveCount: 0,
                                snapshotCount: 0,
                                failureCount: dayData.hasFailures ? 1 : 0
                            }
                        };
                        
                        // Count backup types
                        if (dayData.guests) {
                            dayData.guests.forEach(guest => {
                                const types = Array.isArray(guest.types) ? guest.types : Array.from(guest.types);
                                if (types.includes('pbsSnapshots')) callbackData.stats.pbsCount++;
                                if (types.includes('pveBackups')) callbackData.stats.pveCount++;
                                if (types.includes('vmSnapshots')) callbackData.stats.snapshotCount++;
                            });
                        }
                        
                        // Call with instant flag to prevent animations
                        onDateSelectCallback(callbackData, true);
                    }
                }
            }, 50);
        }

        return container;
    }
    
    function createMonthNavigation(container, backupData, guestId, filteredGuestIds) {
        const nav = document.createElement('div');
        nav.className = 'flex items-center justify-between mb-4 p-3 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700';
        
        const monthYear = currentDisplayMonth.toLocaleString('default', { month: 'long', year: 'numeric' });
        
        nav.innerHTML = `
            <button class="prev-month-btn flex items-center gap-1 px-3 py-1 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
                </svg>
                Previous
            </button>
            
            <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">${monthYear}</h3>
            
            <button class="next-month-btn flex items-center gap-1 px-3 py-1 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors">
                Next
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
            </button>
        `;
        
        // Add navigation event listeners
        nav.querySelector('.prev-month-btn').addEventListener('click', () => {
            currentDisplayMonth.setMonth(currentDisplayMonth.getMonth() - 1);
            refreshMonthView(container, backupData, guestId, filteredGuestIds);
        });
        
        nav.querySelector('.next-month-btn').addEventListener('click', () => {
            const today = new Date();
            // Don't allow navigation beyond current month
            if (currentDisplayMonth.getFullYear() < today.getFullYear() || 
                (currentDisplayMonth.getFullYear() === today.getFullYear() && currentDisplayMonth.getMonth() < today.getMonth())) {
                currentDisplayMonth.setMonth(currentDisplayMonth.getMonth() + 1);
                refreshMonthView(container, backupData, guestId, filteredGuestIds);
            }
        });
        
        return nav;
    }
    
    function createMonthDayCell(date, backupData, guestId, filteredGuestIds) {
        const cell = document.createElement('div');
        cell.className = 'relative w-8 h-8 rounded cursor-pointer transition-transform hover:scale-105';
        
        // Create UTC date key to avoid timezone issues
        const utcDate = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()));
        const dateKey = utcDate.toISOString().split('T')[0];
        const isCurrentMonth = date.getUTCMonth() === currentDisplayMonth.getMonth();
        
        // Check if today using UTC comparison
        const today = new Date();
        const utcToday = new Date(Date.UTC(today.getFullYear(), today.getMonth(), today.getDate()));
        const isToday = utcDate.getTime() === utcToday.getTime();
        
        // Get backup data for this date
        const allData = processBackupDataForSingleMonth(backupData, currentDisplayMonth, guestId, filteredGuestIds);
        const dayData = allData[dateKey];
        
        let shouldShowDay = false;
        let hasBackups = dayData && dayData.hasBackups;
        
        if (hasBackups) {
            const backupTypes = Array.from(dayData.allTypes || []);
            const currentFilterType = getCurrentFilterType();
            
            if (currentFilterType === 'all') {
                shouldShowDay = backupTypes.length > 0;
            } else if (currentFilterType === 'pbs') {
                shouldShowDay = backupTypes.includes('pbsSnapshots');
            } else if (currentFilterType === 'pve') {
                shouldShowDay = backupTypes.includes('pveBackups');
            } else if (currentFilterType === 'snapshots') {
                shouldShowDay = backupTypes.includes('vmSnapshots');
            }
        }
        
        // Apply styling based on state
        if (isCurrentMonth) {
            if (shouldShowDay) {
                // Apply filter-specific color
                const currentFilterType = getCurrentFilterType();
                if (currentFilterType === 'pbs') {
                    cell.classList.add('bg-purple-500', 'dark:bg-purple-600');
                } else if (currentFilterType === 'pve') {
                    cell.classList.add('bg-orange-500', 'dark:bg-orange-600');
                } else if (currentFilterType === 'snapshots') {
                    cell.classList.add('bg-yellow-500', 'dark:bg-yellow-600');
                } else {
                    cell.classList.add('bg-slate-600', 'dark:bg-slate-500');
                }
            } else {
                cell.classList.add('bg-gray-200', 'dark:bg-gray-700');
            }
            
            // Note: Failure indicators removed - now handled by dedicated failures filter
            
            // Add today indicator
            if (isToday) {
                cell.classList.add('border-2', 'border-dotted', 'border-indigo-500', 'dark:border-indigo-400');
            }
        } else {
            // Days from previous/next month
            cell.classList.add('bg-gray-100', 'dark:bg-gray-800', 'opacity-50');
        }
        
        // Add day number
        const dayNumber = document.createElement('div');
        dayNumber.className = `absolute inset-0 flex items-center justify-center text-xs font-medium ${
            isCurrentMonth 
                ? (shouldShowDay ? 'text-white font-bold' : 'text-gray-600 dark:text-gray-400')
                : 'text-gray-400 dark:text-gray-600'
        }`;
        dayNumber.textContent = date.getUTCDate();
        cell.appendChild(dayNumber);
        
        // Store data for click handling
        cell.dataset.date = dateKey;
        if (dayData && dayData.guests) {
            cell.dataset.guestIds = dayData.guests.map(g => g.vmid).join(',');
            cell.dataset.backupTypes = Array.from(dayData.allTypes || []).join(',');
        }
        
        // Add click handler
        cell.addEventListener('click', (e) => {
            e.stopPropagation();
            
            // Only allow clicks on current month days
            if (!isCurrentMonth) return;
            
            // Helper function to check if a cell is today
            const isCellToday = (cellElement) => {
                if (!cellElement || !cellElement.dataset.date) return false;
                const today = new Date();
                const utcToday = new Date(Date.UTC(today.getFullYear(), today.getMonth(), today.getDate()));
                const todayKey = utcToday.toISOString().split('T')[0];
                return cellElement.dataset.date === todayKey;
            };
            
            // Remove previous selection
            if (currentSelectedCell && currentSelectedCell !== cell) {
                currentSelectedCell.classList.remove(...CSS_CLASSES.SELECTED_CELL);
                
                // Restore today indicator if previous cell was today
                if (isCellToday(currentSelectedCell)) {
                    currentSelectedCell.classList.add('border-2', 'border-dotted', 'border-indigo-500', 'dark:border-indigo-400');
                }
            }
            
            // Toggle selection
            if (cell === currentSelectedCell) {
                cell.classList.remove(...CSS_CLASSES.SELECTED_CELL);
                
                // Restore today indicator if this is today's cell
                if (isToday) {
                    cell.classList.add('border-2', 'border-dotted', 'border-indigo-500', 'dark:border-indigo-400');
                }
                
                currentSelectedCell = null;
                currentSelectedDate = null;
                preservedSelectedDate = null;
                
                if (onDateSelectCallback) {
                    onDateSelectCallback(null);
                }
            } else {
                // Remove today indicator classes before adding selection classes to avoid conflicts
                if (isToday) {
                    cell.classList.remove('border-2', 'border-dotted', 'border-indigo-500', 'dark:border-indigo-400');
                }
                
                cell.classList.add(...CSS_CLASSES.SELECTED_CELL);
                currentSelectedCell = cell;
                currentSelectedDate = dateKey;
                preservedSelectedDate = dateKey;
                
                if (onDateSelectCallback && dayData) {
                    const callbackData = {
                        date: dateKey,
                        backups: dayData.guests || [],
                        stats: {
                            totalGuests: dayData.guests ? dayData.guests.length : 0,
                            pbsCount: 0,
                            pveCount: 0,
                            snapshotCount: 0,
                            failureCount: dayData.hasFailures ? 1 : 0
                        }
                    };
                    
                    // Count backup types
                    if (dayData.guests) {
                        dayData.guests.forEach(guest => {
                            const types = Array.isArray(guest.types) ? guest.types : Array.from(guest.types);
                            if (types.includes('pbsSnapshots')) callbackData.stats.pbsCount++;
                            if (types.includes('pveBackups')) callbackData.stats.pveCount++;
                            if (types.includes('vmSnapshots')) callbackData.stats.snapshotCount++;
                        });
                    }
                    
                    onDateSelectCallback(callbackData);
                } else if (onDateSelectCallback) {
                    // Empty day callback
                    onDateSelectCallback({
                        date: dateKey,
                        backups: [],
                        stats: {
                            totalGuests: 0,
                            pbsCount: 0,
                            pveCount: 0,
                            snapshotCount: 0,
                            failureCount: 0
                        }
                    });
                }
            }
        });
        
        return cell;
    }
    
    function createSingleMonthCalendar(backupData, guestId, filteredGuestIds) {
        const calendarContainer = document.createElement('div');
        calendarContainer.className = 'calendar-month-container bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4';
        
        // Day headers
        const dayHeaders = document.createElement('div');
        dayHeaders.className = 'grid grid-cols-7 gap-1 mb-2';
        const dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
        
        dayNames.forEach(day => {
            const header = document.createElement('div');
            header.className = 'text-center text-xs font-medium text-gray-500 dark:text-gray-400 py-2';
            header.textContent = day;
            dayHeaders.appendChild(header);
        });
        
        calendarContainer.appendChild(dayHeaders);
        
        // Calendar grid (6 weeks x 7 days = 42 cells)
        const calendarGrid = document.createElement('div');
        calendarGrid.className = 'calendar-grid grid grid-cols-7 gap-1';
        
        // Generate the month grid using UTC dates
        const firstDay = new Date(Date.UTC(currentDisplayMonth.getFullYear(), currentDisplayMonth.getMonth(), 1));
        const startDate = new Date(firstDay);
        startDate.setUTCDate(startDate.getUTCDate() - firstDay.getUTCDay()); // Go to start of week
        
        // Create 42 cells (6 weeks)
        for (let i = 0; i < 42; i++) {
            const cellDate = new Date(startDate);
            cellDate.setUTCDate(startDate.getUTCDate() + i);
            
            const cell = createMonthDayCell(cellDate, backupData, guestId, filteredGuestIds);
            calendarGrid.appendChild(cell);
        }
        
        calendarContainer.appendChild(calendarGrid);
        return calendarContainer;
    }
    
    function processBackupDataForSingleMonth(backupData, displayMonth, guestId, filteredGuestIds = null) {
        const monthData = {};
        // Use UTC dates for month boundaries to avoid timezone issues
        const startOfMonth = new Date(Date.UTC(displayMonth.getFullYear(), displayMonth.getMonth(), 1));
        const endOfMonth = new Date(Date.UTC(displayMonth.getFullYear(), displayMonth.getMonth() + 1, 0));
        
        // Get guest data for hostname lookup
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const allGuests = [...vmsData, ...containersData];
        const guestLookup = {};
        allGuests.forEach(guest => {
            // Create unique key for guest lookup
            const nodeIdentifier = guest.node || guest.endpointId || '';
            const uniqueKey = nodeIdentifier ? `${guest.vmid}-${nodeIdentifier}` : guest.vmid.toString();
            guestLookup[uniqueKey] = {
                name: guest.name,
                type: guest.type === 'qemu' ? 'VM' : 'CT',
                vmid: guest.vmid,
                node: guest.node,
                endpointId: guest.endpointId
            };
            // Also add simple vmid lookup as fallback for backward compatibility
            if (!guestLookup[guest.vmid]) {
                guestLookup[guest.vmid] = {
                    name: guest.name,
                    type: guest.type === 'qemu' ? 'VM' : 'CT'
                };
            }
        });
        
        // Group backups by guest and date for this month only
        const backupsByGuestAndDate = {};
        
        // Process all backup sources
        ['pbsSnapshots', 'pveBackups', 'vmSnapshots'].forEach(source => {
            if (!backupData[source]) return;
            
            backupData[source].forEach(item => {
                const timestamp = item.ctime || item.snaptime || item['backup-time'];
                if (!timestamp) return;
                
                const date = new Date(timestamp * 1000);
                
                // Create UTC date to avoid timezone issues
                const utcDate = new Date(Date.UTC(
                    date.getUTCFullYear(),
                    date.getUTCMonth(),
                    date.getUTCDate()
                ));
                
                // Only process items within the current month (UTC comparison)
                if (utcDate < startOfMonth || utcDate > endOfMonth) return;
                
                const dateKey = utcDate.toISOString().split('T')[0];
                const vmid = item.vmid || item['backup-id'] || item.backupVMID;
                
                if (!vmid) return;
                
                // Apply filtering logic
                if (guestId && parseInt(vmid, 10) !== parseInt(guestId, 10)) return;
                if (filteredGuestIds && !isGuestInFilteredList(vmid, item, filteredGuestIds)) return;
                
                // Use unique guest key that includes node information
                const uniqueGuestKey = generateUniqueGuestKey(vmid, item);
                
                if (!backupsByGuestAndDate[uniqueGuestKey]) {
                    backupsByGuestAndDate[uniqueGuestKey] = {};
                }
                
                if (!backupsByGuestAndDate[uniqueGuestKey][dateKey]) {
                    backupsByGuestAndDate[uniqueGuestKey][dateKey] = {
                        date: utcDate,
                        types: new Set(),
                        backups: [],
                        vmid: vmid, // Store original vmid for lookups
                        node: item.node,
                        endpointId: item.endpointId
                    };
                }
                
                backupsByGuestAndDate[uniqueGuestKey][dateKey].types.add(source);
                backupsByGuestAndDate[uniqueGuestKey][dateKey].backups.push({
                    type: source,
                    time: date.toLocaleTimeString(), // Keep original timestamp for display
                    name: item.volid || item.name || item['backup-id'] || 'Backup'
                });
            });
        });
        
        // Process backup days and group by date
        Object.entries(backupsByGuestAndDate).forEach(([uniqueGuestKey, dateData]) => {
            Object.keys(dateData).forEach(dateKey => {
                if (!monthData[dateKey]) {
                    monthData[dateKey] = {
                        guests: [],
                        allTypes: new Set(),
                        hasBackups: true
                    };
                }
                
                // Extract vmid from unique key for guest lookup
                const vmid = dateData[dateKey].vmid || extractVmidFromGuestKey(uniqueGuestKey);
                // Use unique key for guest lookup, fall back to vmid if not found
                const guestInfo = guestLookup[uniqueGuestKey] || guestLookup[vmid] || { name: `Unknown-${vmid}`, type: 'VM' };
                
                monthData[dateKey].guests.push({
                    vmid: vmid,
                    uniqueKey: uniqueGuestKey, // Include unique key for proper identification
                    name: guestInfo.name,
                    type: guestInfo.type,
                    node: dateData[dateKey].node,
                    endpointId: dateData[dateKey].endpointId,
                    types: Array.from(dateData[dateKey].types),
                    backupCount: dateData[dateKey].backups.length
                });
                
                dateData[dateKey].types.forEach(type => {
                    monthData[dateKey].allTypes.add(type);
                });
            });
        });
        
        // Process backup tasks for failure detection
        if (backupData.backupTasks) {
            const tasks = filterTasksByGuest(backupData.backupTasks, guestId);
            
            tasks.forEach(task => {
                if (!task.starttime || task.starttime <= 0) return;
                
                const date = new Date(task.starttime * 1000);
                if (isNaN(date.getTime())) return;
                
                // Create UTC date to avoid timezone issues
                const utcDate = new Date(Date.UTC(
                    date.getUTCFullYear(),
                    date.getUTCMonth(),
                    date.getUTCDate()
                ));
                
                // Only process tasks within the current month (UTC comparison)
                if (utcDate < startOfMonth || utcDate > endOfMonth) return;
                
                const dateKey = utcDate.toISOString().split('T')[0];
                
                if (task.status !== 'OK' && monthData[dateKey]) {
                    monthData[dateKey].hasFailures = true;
                }
            });
        }
        
        return monthData;
    }
    
    function refreshMonthView(container, backupData, guestId, filteredGuestIds) {
        // Update month navigation header
        const nav = container.querySelector('.flex.items-center.justify-between');
        if (nav) {
            const monthYear = currentDisplayMonth.toLocaleString('default', { month: 'long', year: 'numeric' });
            nav.querySelector('h3').textContent = monthYear;
            
            // Update next button state
            const today = new Date();
            const nextBtn = nav.querySelector('.next-month-btn');
            if (currentDisplayMonth.getFullYear() >= today.getFullYear() && 
                currentDisplayMonth.getMonth() >= today.getMonth()) {
                nextBtn.disabled = true;
                nextBtn.classList.add('opacity-50', 'cursor-not-allowed');
            } else {
                nextBtn.disabled = false;
                nextBtn.classList.remove('opacity-50', 'cursor-not-allowed');
            }
        }
        
        // Recreate calendar grid
        const oldCalendar = container.querySelector('.calendar-month-container');
        if (oldCalendar) {
            const newCalendar = createSingleMonthCalendar(backupData, guestId, filteredGuestIds);
            oldCalendar.replaceWith(newCalendar);
        }
        
        // Update legend
        const legendContainer = container.querySelector('.legend-container');
        if (legendContainer) {
            updateLegend(legendContainer);
        }
    }
    
    function createCalendarHeader(backupData, guestId = null, filteredGuestIds = null) {
        const header = document.createElement('div');
        header.className = 'mb-4 space-y-3';
        
        
        // Add backup statistics summary
        const stats = calculateBackupStats(backupData, guestId, filteredGuestIds);
        
        // Get guest name for display if filtering
        let guestDisplayText = `${stats.activeGuests}`;
        if (guestId) {
            const vmsData = PulseApp.state.get('vmsData') || [];
            const containersData = PulseApp.state.get('containersData') || [];
            const allGuests = [...vmsData, ...containersData];
            const guest = allGuests.find(g => parseInt(g.vmid, 10) === parseInt(guestId, 10));
            const guestName = guest ? guest.name : `Guest ${guestId}`;
            guestDisplayText = `${stats.activeGuests} (${guestName})`;
        }
        
        const statsDiv = document.createElement('div');
        statsDiv.className = 'grid grid-cols-2 md:grid-cols-4 gap-3 text-sm';
        
        statsDiv.innerHTML = `
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
                <div class="text-xs text-gray-500 dark:text-gray-400">Total Backups</div>
                <div class="font-semibold text-gray-900 dark:text-gray-100">${stats.totalBackups}</div>
            </div>
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
                <div class="text-xs text-gray-500 dark:text-gray-400">Days with Backups</div>
                <div class="font-semibold text-gray-900 dark:text-gray-100">${stats.daysWithBackups}</div>
            </div>
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
                <div class="text-xs text-gray-500 dark:text-gray-400">Active Guests</div>
                <div class="font-semibold text-gray-900 dark:text-gray-100">${guestDisplayText}</div>
            </div>
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
                <div class="text-xs text-gray-500 dark:text-gray-400">Backup Types</div>
                <div class="flex gap-2 mt-1">
                    ${stats.hasPBS ? '<span class="inline-block w-2 h-2 bg-purple-500 rounded-full" title="PBS"></span>' : ''}
                    ${stats.hasPVE ? '<span class="inline-block w-2 h-2 bg-orange-500 rounded-full" title="PVE"></span>' : ''}
                    ${stats.hasSnapshots ? '<span class="inline-block w-2 h-2 bg-yellow-500 rounded-full" title="Snapshots"></span>' : ''}
                </div>
            </div>
        `;
        
        header.appendChild(statsDiv);
        
        return header;
    }
    
    function calculateBackupStats(backupData, guestId = null, filteredGuestIds = null) {
        const stats = {
            totalBackups: 0,
            daysWithBackups: 0,
            activeGuests: new Set(),
            hasPBS: false,
            hasPVE: false,
            hasSnapshots: false
        };
        
        const daysWithData = new Set();
        
        // Process all backup sources
        ['pbsSnapshots', 'pveBackups', 'vmSnapshots'].forEach(source => {
            if (!backupData[source]) return;
            
            backupData[source].forEach(item => {
                const timestamp = item.ctime || item.snaptime || item['backup-time'];
                if (!timestamp) return;
                
                const date = new Date(timestamp * 1000);
                const dateKey = date.toISOString().split('T')[0];
                const vmid = item.vmid || item['backup-id'] || item.backupVMID;
                
                if (!vmid) return;
                
                // Apply filtering logic
                if (guestId && parseInt(vmid, 10) !== parseInt(guestId, 10)) return;
                if (filteredGuestIds && !isGuestInFilteredList(vmid, item, filteredGuestIds)) return;
                
                // Track unique guests using node-aware keys
                const uniqueGuestKey = generateUniqueGuestKey(vmid, item);
                stats.activeGuests.add(uniqueGuestKey);
                daysWithData.add(dateKey);
                stats.totalBackups++;
                
                if (source === 'pbsSnapshots') stats.hasPBS = true;
                if (source === 'pveBackups') stats.hasPVE = true;
                if (source === 'vmSnapshots') stats.hasSnapshots = true;
            });
        });
        
        stats.daysWithBackups = daysWithData.size;
        stats.activeGuests = stats.activeGuests.size;
        
        return stats;
    }

    function createYearNavigation(currentYear, onYearChange) {
        const nav = document.createElement('div');
        nav.className = CSS_CLASSES.YEAR_NAVIGATION;

        const prevButton = document.createElement('button');
        prevButton.className = CSS_CLASSES.YEAR_BUTTON;
        prevButton.innerHTML = '← Previous Year';
        prevButton.onclick = () => {
            const currentYearValue = parseInt(nav.dataset.year || currentYear);
            onYearChange(currentYearValue - 1);
        };

        const yearLabel = document.createElement('span');
        yearLabel.className = CSS_CLASSES.CURRENT_YEAR;
        yearLabel.textContent = currentYear;
        nav.dataset.year = currentYear;

        const nextButton = document.createElement('button');
        nextButton.className = CSS_CLASSES.YEAR_BUTTON;
        nextButton.innerHTML = 'Next Year →';
        nextButton.onclick = () => {
            const currentYearValue = parseInt(nav.dataset.year || currentYear);
            const nextYear = currentYearValue + 1;
            // Don't allow navigation beyond current year
            if (nextYear <= new Date().getFullYear()) {
                onYearChange(nextYear);
            }
        };

        nav.appendChild(prevButton);
        nav.appendChild(yearLabel);
        nav.appendChild(nextButton);

        return nav;
    }

    function updateCalendarContent(container, backupData, guestId, filteredGuestIds = null) {
        if (!container) {
            console.error('[Calendar Heatmap] Container is null');
            return;
        }
        
        const calendarContent = container.querySelector('.calendar-content');
        if (!calendarContent) {
            console.error('[Calendar Heatmap] Calendar content element not found');
            return;
        }
        
        calendarContent.innerHTML = '';

        // Update statistics in header
        const header = container.querySelector('.mb-4.space-y-3');
        if (header) {
            const stats = calculateBackupStats(backupData, guestId, filteredGuestIds);
            
            // Get guest name for display if filtering
            let guestDisplayText = `${stats.activeGuests}`;
            if (guestId) {
                const vmsData = PulseApp.state.get('vmsData') || [];
                const containersData = PulseApp.state.get('containersData') || [];
                const allGuests = [...vmsData, ...containersData];
                const guest = allGuests.find(g => parseInt(g.vmid, 10) === parseInt(guestId, 10));
                const guestName = guest ? guest.name : `Guest ${guestId}`;
                guestDisplayText = `${stats.activeGuests} (${guestName})`;
            }
            
            const statsDiv = header.querySelector('.grid');
            if (statsDiv) {
                statsDiv.innerHTML = `
                    <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
                        <div class="text-xs text-gray-500 dark:text-gray-400">Total Backups</div>
                        <div class="font-semibold text-gray-900 dark:text-gray-100">${stats.totalBackups}</div>
                    </div>
                    <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
                        <div class="text-xs text-gray-500 dark:text-gray-400">Days with Backups</div>
                        <div class="font-semibold text-gray-900 dark:text-gray-100">${stats.daysWithBackups}</div>
                    </div>
                    <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
                        <div class="text-xs text-gray-500 dark:text-gray-400">Active Guests</div>
                        <div class="font-semibold text-gray-900 dark:text-gray-100">${guestDisplayText}</div>
                    </div>
                    <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
                        <div class="text-xs text-gray-500 dark:text-gray-400">Backup Types</div>
                        <div class="flex gap-2 mt-1">
                            ${stats.hasPBS ? '<span class="inline-block w-2 h-2 bg-purple-500 rounded-full" title="PBS"></span>' : ''}
                            ${stats.hasPVE ? '<span class="inline-block w-2 h-2 bg-orange-500 rounded-full" title="PVE"></span>' : ''}
                            ${stats.hasSnapshots ? '<span class="inline-block w-2 h-2 bg-yellow-500 rounded-full" title="Snapshots"></span>' : ''}
                        </div>
                    </div>
                `;
            }
        }

        const allData = processBackupDataForAllYears(backupData, guestId, filteredGuestIds);
        const monthsWithData = generateMonthsWithBackupData(allData);

        // Set responsive grid based on number of months
        calendarContent.className = getResponsiveGridClass(monthsWithData.length);

        monthsWithData.forEach(month => {
            const monthSection = createMonthSection(month, allData, monthsWithData.length);
            calendarContent.appendChild(monthSection);
        });

        // Show message if no months have data
        if (monthsWithData.length === 0) {
            const noDataMessage = document.createElement('div');
            noDataMessage.className = 'text-center text-gray-500 dark:text-gray-400 py-8';
            noDataMessage.textContent = guestId 
                ? `No backup data found for guest ${guestId}`
                : `No backup data found`;
            calendarContent.appendChild(noDataMessage);
        }
        
        // Restore selection if there was one
        if (preservedSelectedDate) {
            setTimeout(() => {
                const cellToSelect = calendarContent.querySelector(`[data-date="${preservedSelectedDate}"]`);
                if (cellToSelect) {
                    // Check if selection is already applied to avoid re-applying styles
                    const isAlreadySelected = cellToSelect.classList.contains('ring-2');
                    
                    if (!isAlreadySelected) {
                        // Find the day data for this cell
                        const dayData = allData[preservedSelectedDate];
                        if (dayData && onDateSelectCallback) {
                            // Manually restore selection without animation
                            cellToSelect.classList.add(...CSS_CLASSES.SELECTED_CELL);
                            currentSelectedCell = cellToSelect;
                            currentSelectedDate = preservedSelectedDate;
                            
                            // Prepare callback data
                            const callbackData = {
                                date: preservedSelectedDate,
                                backups: dayData.guests || [],
                                stats: {
                                    totalGuests: dayData.guests ? dayData.guests.length : 0,
                                    pbsCount: 0,
                                    pveCount: 0,
                                    snapshotCount: 0,
                                    failureCount: dayData.hasFailures ? 1 : 0
                                }
                            };
                            
                            // Count backup types
                            if (dayData.guests) {
                                dayData.guests.forEach(guest => {
                                    const types = Array.isArray(guest.types) ? guest.types : Array.from(guest.types);
                                    if (types.includes('pbsSnapshots')) callbackData.stats.pbsCount++;
                                    if (types.includes('pveBackups')) callbackData.stats.pveCount++;
                                    if (types.includes('vmSnapshots')) callbackData.stats.snapshotCount++;
                                });
                            }
                            
                            // Call with instant flag
                            onDateSelectCallback(callbackData, true);
                        }
                    } else {
                        // Cell is already selected, just update the current references
                        currentSelectedCell = cellToSelect;
                        currentSelectedDate = preservedSelectedDate;
                    }
                }
            }, 50);
        }
        
    }

    function processBackupDataForAllYears(backupData, guestId, filteredGuestIds = null) {
        const allData = {};
        
        // Get guest data for hostname lookup
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const allGuests = [...vmsData, ...containersData];
        const guestLookup = {};
        allGuests.forEach(guest => {
            // Create unique key for guest lookup
            const nodeIdentifier = guest.node || guest.endpointId || '';
            const uniqueKey = nodeIdentifier ? `${guest.vmid}-${nodeIdentifier}` : guest.vmid.toString();
            guestLookup[uniqueKey] = {
                name: guest.name,
                type: guest.type === 'qemu' ? 'VM' : 'CT',
                vmid: guest.vmid,
                node: guest.node,
                endpointId: guest.endpointId
            };
            // Also add simple vmid lookup as fallback for backward compatibility
            if (!guestLookup[guest.vmid]) {
                guestLookup[guest.vmid] = {
                    name: guest.name,
                    type: guest.type === 'qemu' ? 'VM' : 'CT'
                };
            }
        });
        
        // Group all backups by guest and date (no year restriction)
        const backupsByGuestAndDate = {};
        
        // Process all backup sources
        const sources = ['pbsSnapshots', 'pveBackups', 'vmSnapshots'];
        
        sources.forEach(source => {
            if (!backupData[source]) return;

            const items = backupData[source];

            items.forEach(item => {
                const timestamp = item.ctime || item.snaptime || item['backup-time'];
                if (!timestamp) return;

                const date = new Date(timestamp * 1000);
                const dateKey = date.toISOString().split('T')[0];
                
                // Skip future dates
                const now = new Date();
                if (date > now) return;

                const vmid = item.vmid || item['backup-id'] || item.backupVMID;
                if (!vmid) return;
                
                // Skip if filtering by specific guest
                if (guestId && parseInt(vmid, 10) !== parseInt(guestId, 10)) return;
                
                // Skip if filtered guest list is provided and this guest is not in it
                if (filteredGuestIds && !isGuestInFilteredList(vmid, item, filteredGuestIds)) return;
                
                // Use unique guest key that includes node information
                const uniqueGuestKey = generateUniqueGuestKey(vmid, item);
                
                if (!backupsByGuestAndDate[uniqueGuestKey]) {
                    backupsByGuestAndDate[uniqueGuestKey] = {};
                }
                
                if (!backupsByGuestAndDate[uniqueGuestKey][dateKey]) {
                    backupsByGuestAndDate[uniqueGuestKey][dateKey] = {
                        date: utcDate,
                        types: new Set(),
                        backups: [],
                        vmid: vmid, // Store original vmid for lookups
                        node: item.node,
                        endpointId: item.endpointId
                    };
                }
                
                backupsByGuestAndDate[uniqueGuestKey][dateKey].types.add(source);
                backupsByGuestAndDate[uniqueGuestKey][dateKey].backups.push({
                    type: source,
                    time: date.toLocaleTimeString(), // Keep original timestamp for display
                    name: item.volid || item.name || item['backup-id'] || 'Backup'
                });
            });
        });
        
        // Process all backup days and group by date
        Object.entries(backupsByGuestAndDate).forEach(([uniqueGuestKey, dateData]) => {
            Object.keys(dateData).forEach(dateKey => {
                // Initialize day data if not exists
                if (!allData[dateKey]) {
                    allData[dateKey] = {
                        guests: [],
                        allTypes: new Set(),
                        hasBackups: true
                    };
                }
                
                // Extract vmid from unique key for guest lookup
                const vmid = dateData[dateKey].vmid || extractVmidFromGuestKey(uniqueGuestKey);
                // Use unique key for guest lookup, fall back to vmid if not found
                const guestInfo = guestLookup[uniqueGuestKey] || guestLookup[vmid] || { name: `Unknown-${vmid}`, type: 'VM' };
                
                allData[dateKey].guests.push({
                    vmid: vmid,
                    uniqueKey: uniqueGuestKey, // Include unique key for proper identification
                    name: guestInfo.name,
                    type: guestInfo.type,
                    node: dateData[dateKey].node,
                    endpointId: dateData[dateKey].endpointId,
                    types: Array.from(dateData[dateKey].types),
                    backupCount: dateData[dateKey].backups.length
                });
                
                dateData[dateKey].types.forEach(type => {
                    allData[dateKey].allTypes.add(type);
                });
            });
        });

        // Process backup tasks for failure detection
        if (backupData.backupTasks) {
            const tasks = filterTasksByGuest(backupData.backupTasks, guestId);

            tasks.forEach(task => {
                if (!task.starttime || task.starttime <= 0) return;
                
                const date = new Date(task.starttime * 1000);
                if (isNaN(date.getTime())) return;

                const dateKey = date.toISOString().split('T')[0];
                
                if (task.status !== 'OK' && allData[dateKey]) {
                    allData[dateKey].hasFailures = true;
                }
            });
        }

        return allData;
    }

    function generateMonthsWithBackupData(allData) {
        const monthsSet = new Set();
        
        // Get all unique year-month combinations from backup data
        Object.keys(allData).forEach(dateKey => {
            const date = new Date(dateKey);
            const monthKey = `${date.getFullYear()}-${date.getMonth()}`;
            monthsSet.add(monthKey);
        });
        
        // Convert to sorted month objects (most recent first)
        const allMonths = Array.from(monthsSet).sort().reverse().map(monthKey => {
            const [year, month] = monthKey.split('-').map(Number);
            const firstDay = new Date(year, month, 1);
            const lastDay = new Date(year, month + 1, 0);
            
            return {
                name: firstDay.toLocaleString('default', { month: 'short', year: 'numeric' }),
                year: year,
                month: month,
                firstDay: firstDay.getDay(),
                daysInMonth: lastDay.getDate(),
                monthKey: monthKey
            };
        });
        
        // Limit to reasonable number of months for performance and UX
        // Show last 24 months max (2 years) to keep UI manageable
        const maxMonths = 24;
        const months = allMonths.slice(0, maxMonths);
        
        // Sort back to chronological order for display
        return months.reverse();
    }

    function processBackupDataForYear(backupData, guestId, year, filteredGuestIds = null) {
        const yearData = {};
        const startOfYear = new Date(year, 0, 1);
        const endOfYear = new Date(year, 11, 31, 23, 59, 59);
        const today = new Date();
        
        // Get guest data for hostname lookup
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const allGuests = [...vmsData, ...containersData];
        const guestLookup = {};
        allGuests.forEach(guest => {
            // Create unique key for guest lookup
            const nodeIdentifier = guest.node || guest.endpointId || '';
            const uniqueKey = nodeIdentifier ? `${guest.vmid}-${nodeIdentifier}` : guest.vmid.toString();
            guestLookup[uniqueKey] = {
                name: guest.name,
                type: guest.type === 'qemu' ? 'VM' : 'CT',
                vmid: guest.vmid,
                node: guest.node,
                endpointId: guest.endpointId
            };
            // Also add simple vmid lookup as fallback for backward compatibility
            if (!guestLookup[guest.vmid]) {
                guestLookup[guest.vmid] = {
                    name: guest.name,
                    type: guest.type === 'qemu' ? 'VM' : 'CT'
                };
            }
        });
        
        // Group all backups by guest and date
        const backupsByGuestAndDate = {};
        
        // Process all backup sources
        const sources = ['pbsSnapshots', 'pveBackups', 'vmSnapshots'];
        
        sources.forEach(source => {
            if (!backupData[source]) return;

            const items = backupData[source];

            items.forEach(item => {
                const timestamp = item.ctime || item.snaptime || item['backup-time'];
                if (!timestamp) {
                    return;
                }

                const date = new Date(timestamp * 1000);
                if (date < startOfYear || date > endOfYear) return;
                
                const dateKey = date.toISOString().split('T')[0];
                
                // Skip future dates
                const now = new Date();
                if (date > now) {
                    return;
                }

                const vmid = item.vmid || item['backup-id'] || item.backupVMID;
                if (!vmid) {
                    return;
                }
                
                // Skip if filtering by specific guest
                if (guestId && parseInt(vmid, 10) !== parseInt(guestId, 10)) return;
                
                // Skip if filtered guest list is provided and this guest is not in it
                if (filteredGuestIds && !isGuestInFilteredList(vmid, item, filteredGuestIds)) return;
                
                // Use unique guest key that includes node information
                const uniqueGuestKey = generateUniqueGuestKey(vmid, item);
                
                if (!backupsByGuestAndDate[uniqueGuestKey]) {
                    backupsByGuestAndDate[uniqueGuestKey] = {};
                }
                
                if (!backupsByGuestAndDate[uniqueGuestKey][dateKey]) {
                    backupsByGuestAndDate[uniqueGuestKey][dateKey] = {
                        date: utcDate,
                        types: new Set(),
                        backups: [],
                        vmid: vmid, // Store original vmid for lookups
                        node: item.node,
                        endpointId: item.endpointId
                    };
                }
                
                backupsByGuestAndDate[uniqueGuestKey][dateKey].types.add(source);
                backupsByGuestAndDate[uniqueGuestKey][dateKey].backups.push({
                    type: source,
                    time: date.toLocaleTimeString(), // Keep original timestamp for display
                    name: item.volid || item.name || item['backup-id'] || 'Backup'
                });
            });
        });
        
        // Process all backup days and determine retention markers
        Object.entries(backupsByGuestAndDate).forEach(([uniqueGuestKey, dateData]) => {
            const sortedDates = Object.keys(dateData).sort().reverse(); // Most recent first
            
            let lastDaily = null;
            let lastWeekly = null;
            let lastMonthly = null;
            let dailyCount = 0;
            
            sortedDates.forEach(dateKey => {
                const date = new Date(dateKey);
                const daysSinceBackup = Math.floor((today - date) / (1000 * 60 * 60 * 24));
                
                // Initialize day data if not exists
                if (!yearData[dateKey]) {
                    yearData[dateKey] = {
                        retentionLevels: new Set(),
                        guestsByRetention: {},
                        allTypes: new Set(),
                        hasBackups: true // Mark that this day has backups
                    };
                }
                
                // Determine retention level (optional - for special highlighting)
                let retentionLevel = null;
                
                if (daysSinceBackup <= 7 && dailyCount < 7) {
                    // Last 7 days - daily retention
                    retentionLevel = 'daily';
                    dailyCount++;
                } else if (date.getDay() === 0 && daysSinceBackup <= 28) {
                    // Sunday within last 4 weeks - weekly retention
                    if (!lastWeekly || date > lastWeekly) {
                        retentionLevel = 'weekly';
                        lastWeekly = date;
                    }
                } else if (date.getDate() <= 7 && date.getDay() === 0) {
                    // First Sunday of month - monthly retention
                    if (!lastMonthly || date.getMonth() !== lastMonthly.getMonth()) {
                        retentionLevel = 'monthly';
                        lastMonthly = date;
                    }
                } else if (date.getMonth() === 0 && date.getDate() === 1) {
                    // January 1st - yearly retention
                    retentionLevel = 'yearly';
                } else {
                    // Default retention level for any backup that doesn't fit other categories
                    retentionLevel = 'general';
                }
                
                // Always add backup data, regardless of retention level
                yearData[dateKey].retentionLevels.add(retentionLevel);
                
                if (!yearData[dateKey].guestsByRetention[retentionLevel]) {
                    yearData[dateKey].guestsByRetention[retentionLevel] = [];
                }
                
                // Extract vmid from unique key for guest lookup
                const vmid = dateData[dateKey].vmid || extractVmidFromGuestKey(uniqueGuestKey);
                // Use unique key for guest lookup, fall back to vmid if not found
                const guestInfo = guestLookup[uniqueGuestKey] || guestLookup[vmid] || { name: `Unknown-${vmid}`, type: 'VM' };
                
                // Check if this guest already exists in this retention level using unique key
                const existingGuestIndex = yearData[dateKey].guestsByRetention[retentionLevel].findIndex(g => g.uniqueKey === uniqueGuestKey);
                
                if (existingGuestIndex >= 0) {
                    // Merge backup types if guest already exists
                    const existingGuest = yearData[dateKey].guestsByRetention[retentionLevel][existingGuestIndex];
                    const mergedTypes = new Set([...existingGuest.types, ...Array.from(dateData[dateKey].types)]);
                    existingGuest.types = Array.from(mergedTypes);
                    existingGuest.backupCount += dateData[dateKey].backups.length;
                } else {
                    // Add new guest
                    yearData[dateKey].guestsByRetention[retentionLevel].push({
                        vmid: vmid,
                        uniqueKey: uniqueGuestKey, // Include unique key for proper identification
                        name: guestInfo.name,
                        type: guestInfo.type,
                        node: dateData[dateKey].node,
                        endpointId: dateData[dateKey].endpointId,
                        types: Array.from(dateData[dateKey].types),
                        backupCount: dateData[dateKey].backups.length
                    });
                }
                
                dateData[dateKey].types.forEach(type => {
                    yearData[dateKey].allTypes.add(type);
                });
            });
        });

        // Process backup tasks for failure detection
        if (backupData.backupTasks) {
            const tasks = filterTasksByGuest(backupData.backupTasks, guestId);

            tasks.forEach(task => {
                if (!task.starttime || task.starttime <= 0) {
                    return;
                }
                
                const date = new Date(task.starttime * 1000);
                if (isNaN(date.getTime())) {
                    return;
                }
                
                if (date < startOfYear || date > endOfYear) return;

                const dateKey = date.toISOString().split('T')[0];
                
                if (task.status !== 'OK' && yearData[dateKey]) {
                    yearData[dateKey].hasFailures = true;
                }
            });
        }

        return yearData;
    }

    function generateYearMonths(year) {
        const months = [];
        const currentDate = new Date();
        const currentYear = currentDate.getFullYear();
        const currentMonth = currentDate.getMonth();
        
        // Determine how many months to show
        const maxMonth = (year === currentYear) ? currentMonth : 11;
        
        for (let month = 0; month <= maxMonth; month++) {
            const firstDay = new Date(year, month, 1);
            const lastDay = new Date(year, month + 1, 0);
            
            
            months.push({
                name: firstDay.toLocaleString('default', { month: 'short' }),
                year: year,
                month: month,
                firstDay: firstDay.getDay(),
                daysInMonth: lastDay.getDate()
            });
        }
        return months;
    }

    function getResponsiveGridClass(monthCount) {
        if (monthCount === 0) {
            return 'flex items-center justify-center'; // For no data message
        } else if (monthCount === 1) {
            return 'grid grid-cols-1 place-items-center gap-6'; // Center single month
        } else if (monthCount === 2) {
            return 'grid grid-cols-1 lg:grid-cols-2 gap-8 place-items-center'; // 2 months with nice spacing
        } else if (monthCount === 3) {
            return 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6'; // 3 months
        } else if (monthCount === 4) {
            return 'grid grid-cols-1 md:grid-cols-2 gap-4'; // 2x2 grid
        } else if (monthCount <= 6) {
            return 'grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4'; // Up to 3x2
        } else {
            return 'grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4'; // Many months
        }
    }

    function monthHasBackupData(month, yearData) {
        // Check if any day in this month has backup data
        for (let day = 1; day <= month.daysInMonth; day++) {
            const date = new Date(month.year, month.month, day);
            const dateKey = date.toISOString().split('T')[0];
            if (yearData[dateKey] && yearData[dateKey].hasBackups) {
                return true;
            }
        }
        return false;
    }

    function getMonthLabelClass(totalMonthCount) {
        if (totalMonthCount <= 2) {
            return 'text-sm font-medium text-gray-600 dark:text-gray-400 mb-2'; // Larger label for fewer months
        } else if (totalMonthCount <= 4) {
            return 'text-sm text-gray-600 dark:text-gray-400 font-medium mb-1.5';
        } else {
            return 'text-xs text-gray-600 dark:text-gray-400 font-medium mb-1'; // Original size
        }
    }

    function getCalendarGridClass(totalMonthCount) {
        if (totalMonthCount === 1) {
            return 'grid grid-cols-7 gap-1.5'; // Largest cells and spacing for single month
        } else if (totalMonthCount === 2) {
            return 'grid grid-cols-7 gap-1'; // Large cells for 2 months
        } else if (totalMonthCount <= 4) {
            return 'grid grid-cols-7 gap-0.5'; // Medium cells
        } else {
            return 'grid grid-cols-7 gap-0.5'; // Original small cells for many months
        }
    }

    function getDayCellClass(totalMonthCount) {
        if (totalMonthCount === 1) {
            return 'w-5 h-5 rounded cursor-pointer relative transition-transform'; // Largest cells
        } else if (totalMonthCount === 2) {
            return 'w-4 h-4 rounded cursor-pointer relative transition-transform'; // Large cells
        } else if (totalMonthCount <= 4) {
            return 'w-3.5 h-3.5 rounded cursor-pointer relative transition-transform'; // Medium cells
        } else {
            return 'w-3 h-3 rounded cursor-pointer relative transition-transform'; // Original small cells
        }
    }

    function createMonthSection(month, yearData, totalMonthCount) {
        const section = document.createElement('div');
        section.className = '';

        const label = document.createElement('div');
        label.className = getMonthLabelClass(totalMonthCount);
        label.textContent = month.name;
        section.appendChild(label);

        const grid = document.createElement('div');
        grid.className = getCalendarGridClass(totalMonthCount);

        // Add empty cells for days before month starts
        for (let i = 0; i < month.firstDay; i++) {
            const emptyCell = document.createElement('div');
            emptyCell.className = getDayCellClass(totalMonthCount).replace('cursor-pointer transition-all duration-200 relative group', ''); // Same size but no interactions
            grid.appendChild(emptyCell);
        }

        // Add day cells - show all days but only color those with backup data  
        for (let day = 1; day <= month.daysInMonth; day++) {
            // Use UTC to avoid timezone conversion issues
            const date = new Date(Date.UTC(month.year, month.month, day));
            const dateKey = date.toISOString().split('T')[0];
            const dayData = yearData[dateKey];
            
            const dayCell = createDayCell(date, dayData, totalMonthCount);
            grid.appendChild(dayCell);
        }

        section.appendChild(grid);
        return section;
    }

    function createDayCell(date, dayData, totalMonthCount) {
        const cell = document.createElement('div');
        cell.className = getDayCellClass(totalMonthCount) + ' overflow-hidden';
        
        // Check if this is today
        const today = new Date();
        const isToday = date.toDateString() === today.toDateString();
        
        if (isToday) {
            // Add today indicator with a subtle dotted border
            cell.className += ' border-2 border-dotted border-indigo-500 dark:border-indigo-400';
        }
        
        // Make all cells clickable
        cell.style.cursor = 'pointer';
        
        let shouldShowDay = false;
        
        if (dayData && dayData.hasBackups) {
            
            // Check if this day should be shown based on current filter
            const backupTypes = Array.from(dayData.allTypes || []);
            
            if (currentFilterType === 'all') {
                shouldShowDay = backupTypes.length > 0;
            } else if (currentFilterType === 'pbs') {
                shouldShowDay = backupTypes.includes('pbsSnapshots');
            } else if (currentFilterType === 'pve') {
                shouldShowDay = backupTypes.includes('pveBackups');
            } else if (currentFilterType === 'snapshots') {
                shouldShowDay = backupTypes.includes('vmSnapshots');
            }
            
            if (shouldShowDay) {
                // Use a single color for all backup days
                if (currentFilterType === 'pbs') {
                    cell.className += ' bg-purple-500 dark:bg-purple-600';
                } else if (currentFilterType === 'pve') {
                    cell.className += ' bg-orange-500 dark:bg-orange-600';
                } else if (currentFilterType === 'snapshots') {
                    cell.className += ' bg-yellow-500 dark:bg-yellow-600';
                } else {
                    // 'all' filter - use a neutral color
                    cell.className += ' bg-slate-600 dark:bg-slate-500';
                }
            } else {
                cell.className += ' ' + BACKUP_COLORS.NONE;
            }
            
            // Note: Failure indicators removed - now handled by dedicated failures filter
            
            // Add day number to cell
            if (shouldShowDay) {
                const dayNumber = document.createElement('div');
                dayNumber.className = 'absolute inset-0 flex items-center justify-center text-[9px] font-bold text-white drop-shadow-md';
                dayNumber.textContent = date.getDate();
                cell.appendChild(dayNumber);
            }
        }
        
        // Always add day number for non-backup days or filtered out days
        if (!dayData || !dayData.hasBackups || (dayData.hasBackups && !shouldShowDay)) {
            const dayNumber = document.createElement('div');
            dayNumber.className = 'absolute inset-0 flex items-center justify-center text-[9px] font-medium text-gray-500 dark:text-gray-400';
            dayNumber.textContent = date.getDate();
            cell.appendChild(dayNumber);
        }
        
        const dateKey = date.toISOString().split('T')[0];
        cell.dataset.date = dateKey;
        
        // Store guest IDs and backup types for this day if available
        if (dayData && dayData.guests && dayData.guests.length > 0) {
            const allGuests = dayData.guests.map(g => g.vmid);
            cell.dataset.guestIds = allGuests.join(',');
            
            // Store backup types for fast filtering
            const backupTypes = Array.from(dayData.allTypes || []);
            cell.dataset.backupTypes = backupTypes.join(',');
        }
        
        // Add click event handler
        cell.addEventListener('click', (e) => {
            e.stopPropagation();
            
            // Remove previous selection
            if (currentSelectedCell && currentSelectedCell !== cell) {
                currentSelectedCell.classList.remove(...CSS_CLASSES.SELECTED_CELL);
            }
            
            // Toggle selection on current cell
            if (cell === currentSelectedCell) {
                // Deselect if clicking the same cell
                cell.classList.remove(...CSS_CLASSES.SELECTED_CELL);
                currentSelectedCell = null;
                currentSelectedDate = null;
                preservedSelectedDate = null; // Clear preserved selection
                
                // Call callback with null to clear detail view
                if (onDateSelectCallback) {
                    onDateSelectCallback(null);
                }
            } else {
                // Select new cell
                cell.classList.add(...CSS_CLASSES.SELECTED_CELL);
                currentSelectedCell = cell;
                currentSelectedDate = dateKey;
                preservedSelectedDate = dateKey; // Preserve for updates
                
                // Call callback with date data
                if (onDateSelectCallback && dayData) {
                    const callbackData = {
                        date: dateKey,
                        backups: dayData.guests || [],
                        stats: {
                            totalGuests: dayData.guests ? dayData.guests.length : 0,
                            pbsCount: 0,
                            pveCount: 0,
                            snapshotCount: 0,
                            failureCount: dayData.hasFailures ? 1 : 0
                        }
                    };
                    
                    // Count backup types
                    if (dayData.guests) {
                        dayData.guests.forEach(guest => {
                            const types = Array.isArray(guest.types) ? guest.types : Array.from(guest.types);
                            if (types.includes('pbsSnapshots')) callbackData.stats.pbsCount++;
                            if (types.includes('pveBackups')) callbackData.stats.pveCount++;
                            if (types.includes('vmSnapshots')) callbackData.stats.snapshotCount++;
                        });
                    }
                    
                    onDateSelectCallback(callbackData);
                }
            }
        });
        
        // Add hover effect
        cell.addEventListener('mouseenter', () => {
            if (!cell.classList.contains('ring-2')) {
                cell.style.transform = 'scale(1.05)';
            }
        });
        
        cell.addEventListener('mouseleave', () => {
            if (!cell.classList.contains('ring-2')) {
                cell.style.transform = '';
            }
        });

        return cell;
    }

    function createLegend() {
        const legend = document.createElement('div');
        legend.className = 'mt-2 text-center legend-container';
        
        updateLegend(legend);
        
        return legend;
    }
    

    function updateLegend(legendContainer) {
        const currentFilterType = getCurrentFilterType();
        let colorClass = 'bg-slate-600 dark:bg-slate-500';
        let labelText = 'Has backups';
        
        if (currentFilterType === 'pbs') {
            colorClass = 'bg-purple-500 dark:bg-purple-600';
            labelText = 'PBS backups';
        } else if (currentFilterType === 'pve') {
            colorClass = 'bg-orange-500 dark:bg-orange-600';
            labelText = 'PVE backups';
        } else if (currentFilterType === 'snapshots') {
            colorClass = 'bg-yellow-500 dark:bg-yellow-600';
            labelText = 'Snapshots';
        }
        
        legendContainer.innerHTML = `
            <div class="text-xs text-gray-500 dark:text-gray-400">
                <span class="inline-flex items-center gap-1">
                    <span class="inline-block w-3 h-3 ${colorClass} rounded"></span>
                    <span>${labelText}</span>
                </span>
                <span class="mx-3">•</span>
                <span class="inline-flex items-center gap-1">
                    <span class="inline-block w-3 h-3 border-2 border-dotted border-indigo-500 dark:border-indigo-400 rounded"></span>
                    <span>Today</span>
                </span>
            </div>
        `;
    }

    function showBackupDetails(date, dayData) {
        // This will be implemented in the next step to show drill-down details
        // TODO: Create modal or expand section with backup details
    }

    function updateCalendarData(backupData, guestId, filteredGuestIds, onDateSelect) {
        // Store the new callback
        onDateSelectCallback = onDateSelect;
        
        // Find existing calendar container
        const existingContainer = document.querySelector('.calendar-heatmap-container');
        if (!existingContainer) return;
        
        // For API updates, don't recreate the calendar at all - just preserve the existing state
        // The calendar data doesn't need to be updated on every API refresh since the backup
        // data shown in the calendar is historical and doesn't change frequently
        
        // Just ensure we maintain the current selection and callback
        if (preservedSelectedDate && currentSelectedCell) {
            // Update the callback if the detail card needs updating with new data
            if (onDateSelectCallback) {
                const allData = processBackupDataForSingleMonth(backupData, currentDisplayMonth, guestId, filteredGuestIds);
                const dayData = allData[preservedSelectedDate];
                if (dayData) {
                    // Prepare callback data with updated information
                    const callbackData = {
                        date: preservedSelectedDate,
                        backups: dayData.guests || [],
                        stats: {
                            totalGuests: dayData.guests ? dayData.guests.length : 0,
                            pbsCount: 0,
                            pveCount: 0,
                            snapshotCount: 0,
                            failureCount: dayData.hasFailures ? 1 : 0
                        }
                    };
                    
                    // Count backup types
                    if (dayData.guests) {
                        dayData.guests.forEach(guest => {
                            const types = Array.isArray(guest.types) ? guest.types : Array.from(guest.types);
                            if (types.includes('pbsSnapshots')) callbackData.stats.pbsCount++;
                            if (types.includes('pveBackups')) callbackData.stats.pveCount++;
                            if (types.includes('vmSnapshots')) callbackData.stats.snapshotCount++;
                        });
                    }
                    
                    // Update detail card with latest data (instant to prevent flashing)
                    onDateSelectCallback(callbackData, true);
                }
            }
        }
    }

    return {
        createCalendarHeatmap,
        updateCalendarData,
        getSelectedDate: () => preservedSelectedDate,
        clearSelection: () => {
            // Remove visual selection from current cell
            if (currentSelectedCell) {
                currentSelectedCell.classList.remove(...CSS_CLASSES.SELECTED_CELL);
            }
            
            // Clear detail card if there's a callback
            if (onDateSelectCallback) {
                onDateSelectCallback(null, true); // null data, instant update
            }
            
            // Clear internal state
            preservedSelectedDate = null;
            currentSelectedDate = null;
            currentSelectedCell = null;
        },
        resetFilter: () => {
            hasAutoSelectedToday = false; // Reset auto-selection flag
        },
        setDisplayMonth: (date) => {
            currentDisplayMonth = new Date(date);
            currentDisplayMonth.setDate(1);
        }
    };
})();