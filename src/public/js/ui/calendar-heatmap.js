PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.calendarHeatmap = (() => {
    
    const CSS_CLASSES = {
        CALENDAR_CONTAINER: 'calendar-heatmap-container max-w-4xl mx-auto',
        MONTH_LABEL: 'text-xs text-gray-600 dark:text-gray-400 font-medium mb-1',
        DAY_TOOLTIP: 'fixed z-50 px-3 py-2 text-xs text-white bg-gray-900/90 rounded shadow-lg pointer-events-none opacity-0 transition-opacity duration-100 max-w-sm max-h-96 overflow-y-auto',
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

    function createCalendarHeatmap(backupData, guestId = null, filteredGuestIds = null) {
        const container = document.createElement('div');
        container.className = CSS_CLASSES.CALENDAR_CONTAINER;

        // Add header with summary stats (will be updated by updateCalendarContent)
        const header = createCalendarHeader(backupData, guestId, filteredGuestIds);
        container.appendChild(header);
        
        const calendarContent = document.createElement('div');
        calendarContent.className = 'calendar-content';
        container.appendChild(calendarContent);

        const legend = createLegend();
        container.appendChild(legend);

        updateCalendarContent(container, backupData, guestId, filteredGuestIds);

        return container;
    }
    
    function createCalendarHeader(backupData, guestId = null, filteredGuestIds = null) {
        const header = document.createElement('div');
        header.className = 'mb-4 space-y-3';
        
        // Add backup statistics summary
        const stats = calculateBackupStats(backupData, guestId, filteredGuestIds);
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
                <div class="font-semibold text-gray-900 dark:text-gray-100">${stats.activeGuests}</div>
            </div>
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
                <div class="text-xs text-gray-500 dark:text-gray-400">Backup Types</div>
                <div class="flex gap-2 mt-1">
                    ${stats.hasPBS ? '<span class="inline-block w-2 h-2 bg-green-500 rounded-full" title="PBS"></span>' : ''}
                    ${stats.hasPVE ? '<span class="inline-block w-2 h-2 bg-yellow-500 rounded-full" title="PVE"></span>' : ''}
                    ${stats.hasSnapshots ? '<span class="inline-block w-2 h-2 bg-blue-500 rounded-full" title="Snapshots"></span>' : ''}
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
                if (guestId && vmid != guestId) return;
                if (filteredGuestIds && !filteredGuestIds.includes(vmid.toString())) return;
                
                stats.activeGuests.add(vmid);
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
                        <div class="font-semibold text-gray-900 dark:text-gray-100">${stats.activeGuests}</div>
                    </div>
                    <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
                        <div class="text-xs text-gray-500 dark:text-gray-400">Backup Types</div>
                        <div class="flex gap-2 mt-1">
                            ${stats.hasPBS ? '<span class="inline-block w-2 h-2 bg-green-500 rounded-full" title="PBS"></span>' : ''}
                            ${stats.hasPVE ? '<span class="inline-block w-2 h-2 bg-yellow-500 rounded-full" title="PVE"></span>' : ''}
                            ${stats.hasSnapshots ? '<span class="inline-block w-2 h-2 bg-blue-500 rounded-full" title="Snapshots"></span>' : ''}
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
    }

    function processBackupDataForAllYears(backupData, guestId, filteredGuestIds = null) {
        const allData = {};
        
        // Get guest data for hostname lookup
        const vmsData = PulseApp.state.get('vmsData') || [];
        const containersData = PulseApp.state.get('containersData') || [];
        const allGuests = [...vmsData, ...containersData];
        const guestLookup = {};
        allGuests.forEach(guest => {
            guestLookup[guest.vmid] = {
                name: guest.name,
                type: guest.type === 'qemu' ? 'VM' : 'CT'
            };
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
                if (guestId && vmid != guestId) return;
                
                // Skip if filtered guest list is provided and this guest is not in it
                if (filteredGuestIds && !filteredGuestIds.includes(vmid.toString())) return;
                
                if (!backupsByGuestAndDate[vmid]) {
                    backupsByGuestAndDate[vmid] = {};
                }
                
                if (!backupsByGuestAndDate[vmid][dateKey]) {
                    backupsByGuestAndDate[vmid][dateKey] = {
                        date: date,
                        types: new Set(),
                        backups: []
                    };
                }
                
                backupsByGuestAndDate[vmid][dateKey].types.add(source);
                backupsByGuestAndDate[vmid][dateKey].backups.push({
                    type: source,
                    time: date.toLocaleTimeString(),
                    name: item.volid || item.name || item['backup-id'] || 'Backup'
                });
            });
        });
        
        // Process all backup days and group by date
        Object.entries(backupsByGuestAndDate).forEach(([vmid, dateData]) => {
            Object.keys(dateData).forEach(dateKey => {
                // Initialize day data if not exists
                if (!allData[dateKey]) {
                    allData[dateKey] = {
                        guests: [],
                        allTypes: new Set(),
                        hasBackups: true
                    };
                }
                
                const guestInfo = guestLookup[vmid] || { name: `Unknown-${vmid}`, type: 'VM' };
                
                allData[dateKey].guests.push({
                    vmid: vmid,
                    name: guestInfo.name,
                    type: guestInfo.type,
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
            const tasks = guestId 
                ? backupData.backupTasks.filter(task => task.vmid == guestId)
                : backupData.backupTasks;

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
        
        // Convert to sorted month objects
        const months = Array.from(monthsSet).sort().map(monthKey => {
            const [year, month] = monthKey.split('-').map(Number);
            const firstDay = new Date(year, month, 1);
            const lastDay = new Date(year, month + 1, 0);
            
            return {
                name: firstDay.toLocaleString('default', { month: 'short', year: 'numeric' }),
                year: year,
                month: month,
                firstDay: firstDay.getDay(),
                daysInMonth: lastDay.getDate()
            };
        });
        
        return months;
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
            guestLookup[guest.vmid] = {
                name: guest.name,
                type: guest.type === 'qemu' ? 'VM' : 'CT'
            };
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
                if (guestId && vmid != guestId) return;
                
                // Skip if filtered guest list is provided and this guest is not in it
                if (filteredGuestIds && !filteredGuestIds.includes(vmid.toString())) return;
                
                
                if (!backupsByGuestAndDate[vmid]) {
                    backupsByGuestAndDate[vmid] = {};
                }
                
                if (!backupsByGuestAndDate[vmid][dateKey]) {
                    backupsByGuestAndDate[vmid][dateKey] = {
                        date: date,
                        types: new Set(),
                        backups: []
                    };
                }
                
                backupsByGuestAndDate[vmid][dateKey].types.add(source);
                backupsByGuestAndDate[vmid][dateKey].backups.push({
                    type: source,
                    time: date.toLocaleTimeString(),
                    name: item.volid || item.name || item['backup-id'] || 'Backup'
                });
            });
        });
        
        // Process all backup days and determine retention markers
        Object.entries(backupsByGuestAndDate).forEach(([vmid, dateData]) => {
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
                
                const guestInfo = guestLookup[vmid] || { name: `Unknown-${vmid}`, type: 'VM' };
                
                // Check if this guest already exists in this retention level
                const existingGuestIndex = yearData[dateKey].guestsByRetention[retentionLevel].findIndex(g => g.vmid === vmid);
                
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
                        name: guestInfo.name,
                        type: guestInfo.type,
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
            const tasks = guestId 
                ? backupData.backupTasks.filter(task => task.vmid == guestId)
                : backupData.backupTasks;

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
            return 'w-5 h-5 rounded cursor-pointer transition-all duration-200 relative group'; // Largest cells
        } else if (totalMonthCount === 2) {
            return 'w-4 h-4 rounded cursor-pointer transition-all duration-200 relative group'; // Large cells
        } else if (totalMonthCount <= 4) {
            return 'w-3.5 h-3.5 rounded cursor-pointer transition-all duration-200 relative group'; // Medium cells
        } else {
            return 'w-3 h-3 rounded cursor-pointer transition-all duration-200 relative group'; // Original small cells
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
        cell.className = getDayCellClass(totalMonthCount) + ' relative overflow-hidden hover:scale-110 hover:z-10 transform transition-transform duration-200';
        
        // Check if this is today
        const today = new Date();
        const isToday = date.toDateString() === today.toDateString();
        
        if (isToday) {
            // Add today indicator with just a static outline
            cell.className += ' ring-2 ring-blue-500 dark:ring-blue-400';
        }
        
        if (dayData && dayData.hasBackups) {
            // Remove clickable appearance
            // cell.style.cursor = 'pointer';
            
            // Determine color based purely on backup types
            const backupTypes = Array.from(dayData.allTypes || []);
            
            if (backupTypes.length > 1) {
                // Multiple backup types - create split background
                if (backupTypes.includes('pbsSnapshots') && backupTypes.includes('vmSnapshots')) {
                    // PBS (green) + VM Snapshots (blue) - diagonal split
                    cell.style.background = `linear-gradient(135deg, rgb(34 197 94) 50%, rgb(59 130 246) 50%)`;
                } else if (backupTypes.includes('pveBackups') && backupTypes.includes('vmSnapshots')) {
                    // PVE (yellow) + VM Snapshots (blue) - diagonal split
                    cell.style.background = `linear-gradient(135deg, rgb(250 204 21) 50%, rgb(59 130 246) 50%)`;
                } else if (backupTypes.includes('pbsSnapshots') && backupTypes.includes('pveBackups')) {
                    // PBS (green) + PVE (yellow) - diagonal split
                    cell.style.background = `linear-gradient(135deg, rgb(34 197 94) 50%, rgb(250 204 21) 50%)`;
                } else if (backupTypes.length === 3) {
                    // All three types - use stripes
                    cell.style.background = `linear-gradient(45deg, 
                        rgb(34 197 94) 0%, rgb(34 197 94) 33%, 
                        rgb(250 204 21) 33%, rgb(250 204 21) 66%, 
                        rgb(59 130 246) 66%, rgb(59 130 246) 100%)`;
                }
            } else if (backupTypes.length === 1) {
                // Single backup type - use solid color based on type
                if (backupTypes.includes('pbsSnapshots')) {
                    cell.className += ' bg-green-500'; // PBS - green
                } else if (backupTypes.includes('pveBackups')) {
                    cell.className += ' bg-yellow-400'; // PVE - yellow
                } else if (backupTypes.includes('vmSnapshots')) {
                    cell.className += ' bg-blue-400'; // Snapshots - blue
                }
            } else {
                cell.className += ' ' + BACKUP_COLORS.NONE;
            }
            
            // Add failure indicator if present
            if (dayData.hasFailures) {
                cell.className += ' ring-1 ring-red-500 dark:ring-red-400';
            }
            
            // Add small indicator for number of guests
            const totalGuests = dayData.guests ? dayData.guests.length : 0;
            
            if (totalGuests > 3) {
                const countIndicator = document.createElement('div');
                countIndicator.className = 'absolute inset-0 flex items-center justify-center text-[8px] font-bold text-white drop-shadow';
                countIndicator.textContent = totalGuests;
                cell.appendChild(countIndicator);
            }
        } else {
            // No retention markers - gray
            cell.className += ' ' + BACKUP_COLORS.NONE;
        }
        
        cell.dataset.date = date.toISOString().split('T')[0];
        
        // Store guest IDs for this day if available
        if (dayData && dayData.guests && dayData.guests.length > 0) {
            const allGuests = dayData.guests.map(g => g.vmid);
            cell.dataset.guestIds = allGuests.join(',');
        }
        
        // Always add tooltip
        const tooltip = createTooltip(date, dayData);
        cell.appendChild(tooltip);

        // Add mouse event handlers for tooltip only
        cell.addEventListener('mouseenter', (e) => {
            const rect = cell.getBoundingClientRect();
            
            // Smart positioning to avoid viewport cutoff
            let left = rect.right + 10;
            let top = rect.top;
            
            // Check if tooltip would go off-screen horizontally
            if (left + 300 > window.innerWidth) {
                left = rect.left - 310; // Position to the left instead
            }
            
            // Check if tooltip would go off-screen vertically
            if (top + 200 > window.innerHeight) {
                top = window.innerHeight - 210; // Position higher
            }
            
            tooltip.style.left = left + 'px';
            tooltip.style.top = top + 'px';
            tooltip.style.opacity = '1';
            tooltip.style.zIndex = '100';
            
            // If tooltip shows no data but cell is colored, try to recreate tooltip
            if (tooltip.textContent.includes('No backup') && dayData && dayData.hasBackups) {
                const newTooltip = createTooltip(date, dayData);
                tooltip.innerHTML = newTooltip.innerHTML;
            }
        });

        cell.addEventListener('mouseleave', () => {
            tooltip.style.opacity = '0';
        });

        // Calendar cells are no longer clickable - interaction moved to table rows

        return cell;
    }
    
    function highlightGuestsInTable(guestIds) {
        // Call the backups UI function to highlight table rows (non-intrusive)
        if (PulseApp.ui && PulseApp.ui.backups && PulseApp.ui.backups._highlightTableRows) {
            // Remove any existing highlights first
            PulseApp.ui.backups._highlightTableRows([], false);
            // Then highlight the selected guests
            PulseApp.ui.backups._highlightTableRows(guestIds, true);
            
            // Scroll to first matching row
            const firstRow = document.querySelector(`#backups-overview-tbody tr[data-guest-id="${guestIds[0]}"]`);
            if (firstRow) {
                firstRow.scrollIntoView({ behavior: 'smooth', block: 'center' });
            }
        }
    }

    function createTooltip(date, dayData) {
        const tooltip = document.createElement('div');
        tooltip.className = CSS_CLASSES.DAY_TOOLTIP;
        
        // Use consistent date formatting without timezone conversion
        const dateStr = date.toLocaleDateString(undefined, { 
            weekday: 'short', 
            year: 'numeric', 
            month: 'short', 
            day: 'numeric',
            timeZone: 'UTC'  // Force UTC to prevent timezone shifts
        });
        
        if (!dayData || !dayData.guests || dayData.guests.length === 0) {
            tooltip.innerHTML = `<div class="text-xs">${dateStr}<br>No backup activity</div>`;
            return tooltip;
        }
        
        let content = `<div class="font-medium mb-2">${dateStr}</div>`;
        
        // Show guests and their backup types
        content += `<div class="text-xs space-y-1">`;
        
        dayData.guests.forEach(guest => {
            const typeLabels = {
                pbsSnapshots: '<span class="text-green-400">PBS</span>',
                pveBackups: '<span class="text-yellow-400">PVE</span>',
                vmSnapshots: '<span class="text-blue-400">SNAP</span>'
            };
            
            const typesArray = Array.isArray(guest.types) ? guest.types : Array.from(guest.types);
            const labels = typesArray.map(t => {
                return typeLabels[t] || `<span class="text-gray-400">${t}</span>`;
            }).join(' ');
            
            content += `<div>${guest.type} ${guest.vmid} (${guest.name}) ${labels} (${guest.backupCount})</div>`;
        });
        
        content += `</div>`;
        
        if (dayData.hasFailures) {
            content += '<div class="text-red-300 text-xs mt-2">⚠ Contains failures</div>';
        }
        
        tooltip.innerHTML = content;
        return tooltip;
    }

    function createLegend() {
        const legend = document.createElement('div');
        legend.className = 'mt-4 space-y-2';
        
        legend.innerHTML = `
            <div class="flex flex-wrap items-center justify-center gap-4 text-xs">
                <div class="flex items-center gap-2">
                    <div class="flex items-center gap-1">
                        <div class="${CSS_CLASSES.LEGEND_BOX} bg-green-500 dark:bg-green-400"></div>
                        <span>PBS</span>
                    </div>
                    <div class="flex items-center gap-1">
                        <div class="${CSS_CLASSES.LEGEND_BOX} bg-yellow-400 dark:bg-yellow-500"></div>
                        <span>PVE</span>
                    </div>
                    <div class="flex items-center gap-1">
                        <div class="${CSS_CLASSES.LEGEND_BOX} bg-blue-400 dark:bg-blue-500"></div>
                        <span>Snapshots</span>
                    </div>
                </div>
            </div>
        `;
        
        return legend;
    }

    function showBackupDetails(date, dayData) {
        // This will be implemented in the next step to show drill-down details
        // TODO: Create modal or expand section with backup details
    }

    return {
        createCalendarHeatmap
    };
})();