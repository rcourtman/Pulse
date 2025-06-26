PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.pbs = (() => {
    // State management
    let state = {
        pbsData: [],
        selectedTimeRange: '7d',
        selectedInstance: 0, // Which PBS instance tab is selected
        selectedNamespace: 'root', // Current namespace
        selectedNamespaceByInstance: new Map(), // Track namespace per instance
        domElements: new Map(), // Cache DOM elements for updates
        taskRows: new Map(), // Map task UPID to row element
        instanceTabs: null, // Reference to instance tabs container
        activeInstanceIndex: 0
    };

    // Time range constants
    const TIME_RANGES = {
        '24h': 24 * 60 * 60 * 1000,
        '7d': 7 * 24 * 60 * 60 * 1000,
        '30d': 30 * 24 * 60 * 60 * 1000
    };

    // Initialize the PBS tab
    function init() {
        const container = document.getElementById('pbs-instances-container');
        if (!container) {
            return;
        }

        // Clear any existing content
        container.innerHTML = '';
        
        // Remove loading message
        const loadingMsg = document.getElementById('pbs-loading-message');
        if (loadingMsg) loadingMsg.style.display = 'none';
        
        // Get initial data
        const pbsData = PulseApp.state?.get?.('pbsDataArray') || PulseApp.state?.pbs || [];
        if (pbsData.length > 0) {
        }
        
        // Initialize with data
        state.pbsData = pbsData;
        renderMainStructure(container);
    }

    // Render main structure
    function renderMainStructure(container) {
        container.innerHTML = '';
        
        if (!state.pbsData || state.pbsData.length === 0) {
            container.innerHTML = '<p class="text-center text-gray-500 dark:text-gray-400 p-4">Proxmox Backup Server integration is not configured.</p>';
            return;
        }
        
        // Create time range selector
        const timeRangeCard = createTimeRangeCard();
        container.appendChild(timeRangeCard);
        
        // Handle multiple instances
        if (state.pbsData.length > 1) {
            // Create instance tabs
            const instanceTabs = createInstanceTabs();
            container.appendChild(instanceTabs);
        }
        
        // Create content area for the active instance
        const contentArea = document.createElement('div');
        contentArea.id = 'pbs-content';
        contentArea.className = 'space-y-4 mt-4';
        container.appendChild(contentArea);
        
        // Render the active instance
        renderActiveInstance();
    }

    // Create instance tabs for multiple PBS servers
    function createInstanceTabs() {
        const tabsContainer = document.createElement('div');
        tabsContainer.className = 'border-b border-gray-200 dark:border-gray-700 mt-4';
        
        const tabsList = document.createElement('nav');
        tabsList.className = '-mb-px flex space-x-4';
        
        state.pbsData.forEach((instance, index) => {
            const tab = document.createElement('button');
            tab.className = index === state.activeInstanceIndex ? 
                'py-2 px-1 border-b-2 border-blue-500 font-medium text-sm text-blue-600 dark:text-blue-400' :
                'py-2 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm';
            tab.textContent = instance.pbsInstanceName || `PBS Instance ${index + 1}`;
            tab.dataset.instanceIndex = index;
            
            tab.addEventListener('click', () => {
                switchInstance(index);
            });
            
            tabsList.appendChild(tab);
        });
        
        tabsContainer.appendChild(tabsList);
        state.instanceTabs = tabsContainer;
        
        return tabsContainer;
    }

    // Switch to a different PBS instance
    function switchInstance(index) {
        state.activeInstanceIndex = index;
        
        // Update tab styling
        if (state.instanceTabs) {
            state.instanceTabs.querySelectorAll('button').forEach((tab, i) => {
                if (i === index) {
                    tab.className = 'py-2 px-1 border-b-2 border-blue-500 font-medium text-sm text-blue-600 dark:text-blue-400';
                } else {
                    tab.className = 'py-2 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm';
                }
            });
        }
        
        // Re-render the active instance
        renderActiveInstance();
    }

    // Render the currently active PBS instance
    function renderActiveInstance() {
        const contentArea = document.getElementById('pbs-content');
        if (!contentArea) {
            return;
        }
        
        const instance = state.pbsData[state.activeInstanceIndex];
        if (!instance) {
            return;
        }
        
        contentArea.innerHTML = '';
        
        // Get namespaces for this instance
        const namespaces = collectNamespaces(instance);
        
        // If multiple namespaces, show namespace tabs
        if (namespaces.length > 1) {
            const namespaceTabs = createNamespaceTabs(namespaces);
            contentArea.appendChild(namespaceTabs);
        }
        
        // Get selected namespace for this instance
        const selectedNamespace = state.selectedNamespaceByInstance.get(state.activeInstanceIndex) || 'root';
        
        // Filter instance data by namespace
        const filteredInstance = filterInstanceByNamespace(instance, selectedNamespace);
        
        // Create instance content
        const instanceContent = createInstanceContent(filteredInstance, state.activeInstanceIndex);
        contentArea.appendChild(instanceContent);
    }

    // Collect namespaces from instance
    function collectNamespaces(instance) {
        const namespaces = new Set(['root']);
        
        // Check all task types for namespaces
        const taskTypes = ['backupTasks', 'verificationTasks', 'syncTasks', 'pruneTasks'];
        taskTypes.forEach(taskType => {
            if (instance[taskType] && instance[taskType].recentTasks) {
                instance[taskType].recentTasks.forEach(task => {
                    if (task.namespace) {
                        namespaces.add(task.namespace);
                    }
                });
            }
        });
        
        // Check datastores for namespaces
        if (instance.datastores) {
            instance.datastores.forEach(ds => {
                if (ds.namespaces) {
                    ds.namespaces.forEach(ns => namespaces.add(ns));
                }
                // Also check snapshots for namespaces
                if (ds.snapshots) {
                    ds.snapshots.forEach(snap => {
                        if (snap.namespace !== undefined) {
                            namespaces.add(snap.namespace || 'root');
                        }
                    });
                }
            });
        }
        
        return Array.from(namespaces).sort();
    }

    // Create namespace tabs
    function createNamespaceTabs(namespaces) {
        const container = document.createElement('div');
        container.className = 'mb-4';
        
        const tabsDiv = document.createElement('div');
        tabsDiv.className = 'flex flex-wrap gap-2';
        
        const selectedNamespace = state.selectedNamespaceByInstance.get(state.activeInstanceIndex) || 'root';
        
        namespaces.forEach(namespace => {
            const tab = document.createElement('button');
            tab.className = namespace === selectedNamespace ?
                'px-3 py-1 text-sm font-medium text-white bg-blue-600 rounded' :
                'px-3 py-1 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded';
            tab.textContent = namespace;
            
            tab.addEventListener('click', () => {
                state.selectedNamespaceByInstance.set(state.activeInstanceIndex, namespace);
                renderActiveInstance();
            });
            
            tabsDiv.appendChild(tab);
        });
        
        container.appendChild(tabsDiv);
        return container;
    }

    // Filter instance by namespace
    function filterInstanceByNamespace(instance, namespace) {
        const filtered = { ...instance };
        
        // Filter each task type
        const taskTypes = ['backupTasks', 'verificationTasks', 'syncTasks', 'pruneTasks'];
        taskTypes.forEach(taskType => {
            if (filtered[taskType] && filtered[taskType].recentTasks) {
                filtered[taskType] = {
                    ...filtered[taskType],
                    recentTasks: filtered[taskType].recentTasks.filter(task => 
                        (task.namespace || 'root') === namespace
                    )
                };
            }
        });
        
        return filtered;
    }

    // Create instance content
    function createInstanceContent(instance, instanceIndex) {
        const container = document.createElement('div');
        container.className = 'space-y-4';
        
        // Add info banner if node discovery failed
        if (instance.nodeName && instance.nodeName.match(/^\d+\.\d+\.\d+\.\d+$/)) {
            const infoBanner = document.createElement('div');
            infoBanner.className = 'bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4 mb-4';
            infoBanner.innerHTML = `
                <div class="flex items-start gap-3">
                    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-blue-600 dark:text-blue-400 flex-shrink-0 mt-0.5">
                        <circle cx="12" cy="12" r="10"></circle>
                        <line x1="12" y1="16" x2="12" y2="12"></line>
                        <line x1="12" y1="8" x2="12.01" y2="8"></line>
                    </svg>
                    <div>
                        <div class="font-medium text-blue-900 dark:text-blue-100 mb-1">Limited PBS Integration</div>
                        <div class="text-sm text-blue-800 dark:text-blue-200">
                            Some features (server metrics, task history) are unavailable because PBS node discovery requires additional permissions. 
                            Backup data and snapshots are fully functional.
                        </div>
                    </div>
                </div>
            `;
            container.appendChild(infoBanner);
        }
        
        // Server status section
        const serverStatus = createServerStatusSection(instance, instanceIndex);
        container.appendChild(serverStatus);
        
        // Datastores section
        const datastoresSection = createDatastoresSection(instance, instanceIndex);
        container.appendChild(datastoresSection);
        
        // Task summary table
        const summaryTable = createTaskSummaryTable(instance, instanceIndex);
        container.appendChild(summaryTable);
        
        // Add snapshots section
        const snapshotsSection = createSnapshotsSection(instance, instanceIndex);
        if (snapshotsSection) {
            container.appendChild(snapshotsSection);
        }
        
        // Task detail tables
        const taskTables = createTaskTables(instance, instanceIndex);
        container.appendChild(taskTables);
        
        return container;
    }

    // Create time range selector card
    function createTimeRangeCard() {
        const card = document.createElement('div');
        card.className = 'bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4';
        
        const content = document.createElement('div');
        content.className = 'flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3';
        
        // Left side - info and search
        const leftContent = document.createElement('div');
        leftContent.className = 'flex-1 flex flex-col sm:flex-row items-start sm:items-center gap-3';
        
        // Icon and text
        const infoSection = document.createElement('div');
        infoSection.className = 'flex items-start gap-3';
        
        const iconDiv = document.createElement('div');
        iconDiv.className = 'flex-shrink-0 mt-0.5';
        iconDiv.innerHTML = `
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-blue-600 dark:text-blue-400">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="16" x2="12" y2="12"></line>
                <line x1="12" y1="8" x2="12.01" y2="8"></line>
            </svg>
        `;
        infoSection.appendChild(iconDiv);
        
        const textDiv = document.createElement('div');
        textDiv.className = 'flex-1';
        
        const titleDiv = document.createElement('div');
        titleDiv.className = 'font-semibold text-gray-800 dark:text-gray-200 mb-1';
        titleDiv.textContent = 'Time Range & Filters';
        textDiv.appendChild(titleDiv);
        
        const descriptionDiv = document.createElement('div');
        descriptionDiv.id = 'pbs-date-range-description';
        descriptionDiv.className = 'text-sm text-gray-700 dark:text-gray-300';
        textDiv.appendChild(descriptionDiv);
        
        infoSection.appendChild(textDiv);
        leftContent.appendChild(infoSection);
        
        
        content.appendChild(leftContent);
        
        // Right side - time range selector
        const rightContent = document.createElement('div');
        rightContent.className = 'flex items-center gap-2';
        
        // Time range selector
        const selector = document.createElement('select');
        selector.id = 'pbs-time-range-selector';
        selector.className = 'px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500';
        
        const options = [
            { value: '24h', text: 'Last 24 Hours' },
            { value: '7d', text: 'Last 7 Days' },
            { value: '30d', text: 'Last 30 Days' }
        ];
        
        options.forEach(opt => {
            const option = document.createElement('option');
            option.value = opt.value;
            option.textContent = opt.text;
            if (opt.value === state.selectedTimeRange) {
                option.selected = true;
            }
            selector.appendChild(option);
        });
        
        rightContent.appendChild(selector);
        content.appendChild(rightContent);
        
        card.appendChild(content);
        
        // Add event handlers
        selector.addEventListener('change', handleTimeRangeChange);
        
        
        // Update description
        updateDateRangeDescription();
        
        return card;
    }

    // Handle time range change - instant visual update
    function handleTimeRangeChange(e) {
        state.selectedTimeRange = e.target.value;
        updateDateRangeDescription();
        applyFilters();
    }

    // Update date range description
    function updateDateRangeDescription() {
        const description = document.getElementById('pbs-date-range-description');
        if (!description) return;
        
        const now = new Date();
        const rangeMs = TIME_RANGES[state.selectedTimeRange];
        const startDate = new Date(now.getTime() - rangeMs);
        
        const dateFormat = { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' };
        
        description.innerHTML = `
            <div class="text-sm">
                ${startDate.toLocaleDateString('en-US', dateFormat)} - ${now.toLocaleDateString('en-US', dateFormat)}
            </div>
        `;
    }

    // Create task summary table
    function createTaskSummaryTable(instance, instanceIndex) {
        const section = document.createElement('div');
        section.className = 'bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4';
        
        const heading = document.createElement('h4');
        heading.className = 'text-md font-semibold mb-2 text-gray-700 dark:text-gray-300';
        heading.textContent = 'PBS Task Summary';
        section.appendChild(heading);
        
        const subtitle = document.createElement('p');
        subtitle.className = 'text-xs text-gray-500 dark:text-gray-400 mb-2';
        subtitle.textContent = 'Task counts reflect the selected time range filter above';
        section.appendChild(subtitle);
        
        // Add table container with overflow-x-auto for mobile scrolling
        const tableContainer = document.createElement('div');
        tableContainer.className = 'overflow-x-auto';
        
        const table = document.createElement('table');
        table.className = 'min-w-full text-sm';
        
        const thead = document.createElement('thead');
        thead.innerHTML = `
            <tr class="text-xs font-medium text-left text-gray-600 uppercase dark:text-gray-300">
                <th class="p-1 px-2">Task Type</th>
                <th class="p-1 px-2">Status</th>
                <th class="p-1 px-2">Last Successful Run</th>
                <th class="p-1 px-2">Last Failure</th>
            </tr>
        `;
        table.appendChild(thead);
        
        const tbody = document.createElement('tbody');
        tbody.className = 'pbs-task-summary-tbody';
        tbody.dataset.instanceIndex = instanceIndex;
        
        // Add rows for each task type
        const taskTypes = ['Backups', 'Verification', 'Sync', 'Prune/GC'];
        taskTypes.forEach(type => {
            const row = createSummaryRow(type, instance, instanceIndex);
            tbody.appendChild(row);
        });
        
        table.appendChild(tbody);
        tableContainer.appendChild(table);
        section.appendChild(tableContainer);
        
        // Store reference
        const namespace = state.selectedNamespaceByInstance.get(instanceIndex) || 'root';
        state.domElements.set(`summary-${instanceIndex}-${namespace}`, tbody);
        
        return section;
    }

    // Create summary row
    function createSummaryRow(taskType, instance, instanceIndex) {
        const row = document.createElement('tr');
        row.dataset.taskType = taskType;
        
        // Task type cell
        const typeCell = document.createElement('td');
        typeCell.className = 'p-1 px-2 font-semibold text-gray-800 dark:text-gray-200';
        typeCell.textContent = taskType;
        row.appendChild(typeCell);
        
        // Status cell
        const statusCell = document.createElement('td');
        statusCell.className = 'p-1 px-2';
        row.appendChild(statusCell);
        
        // Last success cell
        const successCell = document.createElement('td');
        successCell.className = 'p-1 px-2 text-gray-600 dark:text-gray-400';
        row.appendChild(successCell);
        
        // Last failure cell
        const failureCell = document.createElement('td');
        failureCell.className = 'p-1 px-2 text-gray-600 dark:text-gray-400';
        row.appendChild(failureCell);
        
        // Update with actual data
        updateSummaryRow(row, taskType, instance);
        
        return row;
    }

    // Update summary row with data
    function updateSummaryRow(row, taskType, instance) {
        let taskData;
        if (taskType === 'Backups') taskData = instance.backupTasks;
        else if (taskType === 'Verification') taskData = instance.verificationTasks;
        else if (taskType === 'Sync') taskData = instance.syncTasks;
        else if (taskType === 'Prune/GC') taskData = instance.pruneTasks;
        
        if (!taskData || !taskData.recentTasks) {
            // Check if this is due to missing node discovery
            const isNodeDiscoveryIssue = instance.nodeName && instance.nodeName.match(/^\d+\.\d+\.\d+\.\d+$/);
            if (isNodeDiscoveryIssue) {
                row.cells[1].innerHTML = '<span class="text-gray-400" title="Task history requires PBS node discovery">Unavailable</span>';
                row.cells[2].innerHTML = '<span class="text-gray-400">-</span>';
                row.cells[3].innerHTML = '<span class="text-gray-400">-</span>';
            } else {
                row.cells[1].innerHTML = '<span class="text-gray-500">No tasks</span>';
                row.cells[2].textContent = 'N/A';
                row.cells[3].textContent = 'N/A';
            }
            return;
        }
        
        // Filter tasks by time range
        const cutoffTime = Date.now() - TIME_RANGES[state.selectedTimeRange];
        const cutoffTimeSeconds = cutoffTime / 1000;
        
        const filteredTasks = taskData.recentTasks.filter(task => {
            const taskTime = task.startTime || task.starttime || 0;
            return taskTime >= cutoffTimeSeconds;
        });
        
        // Calculate counts
        const okCount = filteredTasks.filter(t => t.status === 'OK').length;
        const failedCount = filteredTasks.filter(t => t.status && t.status !== 'OK' && !t.status.toLowerCase().includes('running')).length;
        
        // Update status cell
        let statusHTML = '';
        if (okCount === 0 && failedCount === 0) {
            statusHTML = '<span class="text-gray-500">No tasks</span>';
        } else {
            if (okCount > 0) {
                statusHTML = `<span class="text-green-600 dark:text-green-400">${okCount} OK</span>`;
            }
            if (failedCount > 0) {
                if (statusHTML) statusHTML += ' / ';
                statusHTML += `<span class="text-red-600 dark:text-red-400">${failedCount} Failed</span>`;
            }
        }
        row.cells[1].innerHTML = statusHTML;
        
        // Find last times
        const sortedTasks = [...filteredTasks].sort((a, b) => (b.startTime || 0) - (a.startTime || 0));
        const lastOk = sortedTasks.find(t => t.status === 'OK');
        const lastFailed = sortedTasks.find(t => t.status && t.status !== 'OK' && !t.status.toLowerCase().includes('running'));
        
        // Update time cells
        row.cells[2].textContent = lastOk ? PulseApp.utils.formatPbsTimestampRelative(lastOk.startTime) : 'N/A';
        row.cells[3].textContent = lastFailed ? PulseApp.utils.formatPbsTimestampRelative(lastFailed.startTime) : 'N/A';
        
        // Update row styling
        if (failedCount > 0) {
            row.classList.add('bg-red-50', 'dark:bg-red-900/20');
        } else {
            row.classList.remove('bg-red-50', 'dark:bg-red-900/20');
        }
    }

    // Create task tables
    function createTaskTables(instance, instanceIndex) {
        const container = document.createElement('div');
        container.className = 'space-y-4';
        
        
        const taskTypes = [
            { key: 'backupTasks', title: 'Recent Backup Tasks', type: 'backup' },
            { key: 'verificationTasks', title: 'Recent Verification Tasks', type: 'verify' },
            { key: 'syncTasks', title: 'Recent Sync Tasks', type: 'sync' },
            { key: 'pruneTasks', title: 'Recent Prune/GC Tasks', type: 'prune' }
        ];
        
        taskTypes.forEach(taskType => {
            const taskData = instance[taskType.key];
            if (!taskData || !taskData.recentTasks || taskData.recentTasks.length === 0) return;
            
            const section = createTaskSection(taskType, taskData, instanceIndex);
            container.appendChild(section);
        });
        
        return container;
    }

    // Create task section
    function createTaskSection(taskType, taskData, instanceIndex) {
        const section = document.createElement('div');
        section.className = 'bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700';
        
        const header = document.createElement('h4');
        header.className = 'text-md font-semibold p-3 border-b border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 flex items-center justify-between cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors';
        
        const titleContainer = document.createElement('div');
        titleContainer.className = 'flex items-center gap-2';
        
        // Collapse/expand icon
        const collapseIcon = document.createElement('span');
        collapseIcon.className = 'collapse-icon transition-transform duration-200';
        collapseIcon.innerHTML = `
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
        `;
        titleContainer.appendChild(collapseIcon);
        
        const headerSpan = document.createElement('span');
        headerSpan.textContent = taskType.title + ' ';
        titleContainer.appendChild(headerSpan);
        
        const countSpan = document.createElement('span');
        countSpan.className = 'text-xs font-normal text-gray-500';
        countSpan.dataset.taskType = taskType.type;
        countSpan.dataset.instanceIndex = instanceIndex;
        titleContainer.appendChild(countSpan);
        
        header.appendChild(titleContainer);
        
        // Add collapse/expand functionality
        header.addEventListener('click', () => {
            toggleTaskSection(section);
        });
        
        section.appendChild(header);
        
        // Table
        const tableContainer = document.createElement('div');
        tableContainer.className = 'overflow-x-auto task-table-container';
        
        const table = document.createElement('table');
        table.className = 'min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-sm';
        table.dataset.taskType = taskType.type;
        table.dataset.instanceIndex = instanceIndex;
        
        // Table header
        const thead = document.createElement('thead');
        thead.className = 'bg-gray-100 dark:bg-gray-700';
        
        const headerRow = document.createElement('tr');
        headerRow.className = 'text-xs font-medium text-left text-gray-600 uppercase dark:text-gray-300';
        
        // Create sortable headers
        const headers = [
            { text: 'Target', field: 'target', sortable: true },
            { text: 'Status', field: 'status', sortable: true },
            { text: 'Start Time', field: 'time', sortable: true },
            { text: 'Duration', field: 'duration', sortable: true }
        ];
        
        headers.forEach(header => {
            const th = document.createElement('th');
            th.className = 'p-1 px-2';
            if (header.sortable) {
                th.className += ' cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors';
                th.innerHTML = `
                    <div class="flex items-center gap-1">
                        <span>${header.text}</span>
                        <svg class="w-3 h-3 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4"></path>
                        </svg>
                    </div>
                `;
                th.addEventListener('click', () => sortTable(tbody, header.field));
            } else {
                th.textContent = header.text;
            }
            headerRow.appendChild(th);
        });
        
        thead.appendChild(headerRow);
        table.appendChild(thead);
        
        // Table body
        const tbody = document.createElement('tbody');
        tbody.className = 'bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700';
        
        // Add task rows
        taskData.recentTasks.forEach(task => {
            const row = createTaskRow(task, taskType.type);
            tbody.appendChild(row);
        });
        
        table.appendChild(tbody);
        tableContainer.appendChild(table);
        section.appendChild(tableContainer);
        
        // Update count
        updateTaskCount(countSpan, tbody);
        
        // Store reference
        const namespace = state.selectedNamespaceByInstance.get(instanceIndex) || 'root';
        state.domElements.set(`tasks-${instanceIndex}-${namespace}-${taskType.type}`, tbody);
        
        // Default state - start collapsed if more than 5 tasks
        if (taskData.recentTasks.length > 5) {
            section.dataset.collapsed = 'true';
            tableContainer.style.display = 'none';
            const icon = header.querySelector('.collapse-icon');
            if (icon) {
                icon.style.transform = 'rotate(-90deg)';
            }
        }
        
        return section;
    }

    // Create task row
    function createTaskRow(task, taskType) {
        const row = document.createElement('tr');
        row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors duration-150 cursor-pointer';
        row.dataset.taskTime = task.startTime || task.starttime || 0;
        row.dataset.upid = task.upid || '';
        
        // Store in map for quick access
        if (task.upid) {
            state.taskRows.set(task.upid, row);
        }
        
        
        // Target cell
        const targetCell = document.createElement('td');
        targetCell.className = 'p-1 px-2 text-sm text-gray-700 dark:text-gray-300';
        targetCell.textContent = parsePbsTaskTarget(task);
        row.appendChild(targetCell);
        
        // Status cell
        const statusCell = document.createElement('td');
        statusCell.className = 'p-1 px-2';
        statusCell.innerHTML = getPbsStatusDisplay(task.status);
        row.appendChild(statusCell);
        
        // Start time cell
        const startCell = document.createElement('td');
        startCell.className = 'p-1 px-2 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap';
        startCell.textContent = task.startTime ? PulseApp.utils.formatPbsTimestampRelative(task.startTime) : 'N/A';
        row.appendChild(startCell);
        
        // Duration cell
        const durationCell = document.createElement('td');
        durationCell.className = 'p-1 px-2 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap';
        durationCell.textContent = formatDuration(task);
        row.appendChild(durationCell);
        
        // Add click handler to show task details
        row.addEventListener('click', () => {
            showTaskDetailsModal(task, taskType);
        });
        
        // Apply initial visibility based on time range
        const cutoffTime = Date.now() - TIME_RANGES[state.selectedTimeRange];
        const cutoffTimeSeconds = cutoffTime / 1000;
        const taskTime = parseInt(row.dataset.taskTime, 10);
        
        if (taskTime && taskTime < cutoffTimeSeconds) {
            row.style.display = 'none';
        }
        
        return row;
    }

    // Main update function - called by socket handler
    function updatePbsInfo(pbsArray) {
        state.pbsData = pbsArray || [];
        
        const container = document.getElementById('pbs-instances-container');
        if (!container) {
            return;
        }
        
        // If no content yet, do initial render
        const contentArea = document.getElementById('pbs-content');
        if (!contentArea) {
            renderMainStructure(container);
        } else {
            // Incremental update
            updateExistingData();
        }
    }

    // Update existing data incrementally
    function updateExistingData() {
        // Update the active instance
        const instance = state.pbsData[state.activeInstanceIndex];
        if (!instance) return;
        
        const namespace = state.selectedNamespaceByInstance.get(state.activeInstanceIndex) || 'root';
        const filteredInstance = filterInstanceByNamespace(instance, namespace);
        
        // Update summary table
        const summaryKey = `summary-${state.activeInstanceIndex}-${namespace}`;
        const summaryTbody = state.domElements.get(summaryKey);
        if (summaryTbody) {
            const rows = summaryTbody.querySelectorAll('tr');
            rows.forEach(row => {
                const taskType = row.dataset.taskType;
                updateSummaryRow(row, taskType, filteredInstance);
            });
        }
        
        // Update task tables
        const taskTypes = [
            { key: 'backupTasks', type: 'backup' },
            { key: 'verificationTasks', type: 'verify' },
            { key: 'syncTasks', type: 'sync' },
            { key: 'pruneTasks', type: 'prune' }
        ];
        
        taskTypes.forEach(taskType => {
            const tasksKey = `tasks-${state.activeInstanceIndex}-${namespace}-${taskType.type}`;
            const tbody = state.domElements.get(tasksKey);
            if (!tbody) return;
            
            const taskData = filteredInstance[taskType.key];
            if (!taskData || !taskData.recentTasks) return;
            
            // Update existing rows and add new ones
            const existingUpids = new Set();
            tbody.querySelectorAll('tr').forEach(row => {
                existingUpids.add(row.dataset.upid);
            });
            
            taskData.recentTasks.forEach(task => {
                if (!task.upid || existingUpids.has(task.upid)) {
                    // Update existing row
                    const row = state.taskRows.get(task.upid);
                    if (row) {
                        updateTaskRow(row, task);
                    }
                } else {
                    // Add new row
                    const newRow = createTaskRow(task, taskType.type);
                    tbody.insertBefore(newRow, tbody.firstChild);
                }
            });
            
            // Update count
            const countSpan = document.querySelector(`span[data-task-type="${taskType.type}"][data-instance-index="${state.activeInstanceIndex}"]`);
            if (countSpan) {
                updateTaskCount(countSpan, tbody);
            }
        });
    }

    // Update task row
    function updateTaskRow(row, task) {
        // Update status if changed
        const statusCell = row.cells[1];
        const currentStatus = statusCell.querySelector('span')?.textContent;
        const newStatus = task.status || 'Running';
        
        if (currentStatus !== newStatus) {
            statusCell.innerHTML = getPbsStatusDisplay(task.status);
        }
        
        // Update duration if task completed
        if (task.endTime && row.cells[3].textContent === 'Running...') {
            row.cells[3].textContent = formatDuration(task);
        }
    }

    // Update task count in header
    function updateTaskCount(countSpan, tbody) {
        const visibleRows = tbody.querySelectorAll('tr:not([style*="display: none"])');
        const total = visibleRows.length;
        const failed = Array.from(visibleRows).filter(r => r.querySelector('.text-red-600')).length;
        const running = Array.from(visibleRows).filter(r => r.querySelector('.text-blue-600')).length;
        
        let text = `(${total} total`;
        if (failed > 0) text += `, ${failed} failed`;
        if (running > 0) text += `, ${running} running`;
        text += ')';
        
        countSpan.textContent = text;
    }

    // Toggle task section collapse/expand
    function toggleTaskSection(section) {
        const tableContainer = section.querySelector('.task-table-container');
        const icon = section.querySelector('.collapse-icon');
        
        if (!tableContainer || !icon) return;
        
        const isCollapsed = section.dataset.collapsed === 'true';
        
        if (isCollapsed) {
            // Expand
            tableContainer.style.display = '';
            icon.style.transform = 'rotate(0deg)';
            section.dataset.collapsed = 'false';
        } else {
            // Collapse
            tableContainer.style.display = 'none';
            icon.style.transform = 'rotate(-90deg)';
            section.dataset.collapsed = 'true';
        }
    }
    
    // Debounce utility
    function debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    }
    
    
    
    // Sort table by field
    function sortTable(tbody, field) {
        const rows = Array.from(tbody.querySelectorAll('tr'));
        const sortDir = tbody.dataset.sortField === field && tbody.dataset.sortDir === 'asc' ? 'desc' : 'asc';
        
        tbody.dataset.sortField = field;
        tbody.dataset.sortDir = sortDir;
        
        rows.sort((a, b) => {
            let aVal, bVal;
            
            switch (field) {
                case 'target':
                    aVal = a.cells[0].textContent;
                    bVal = b.cells[0].textContent;
                    break;
                case 'status':
                    aVal = a.cells[1].textContent;
                    bVal = b.cells[1].textContent;
                    break;
                case 'time':
                    aVal = parseInt(a.dataset.taskTime) || 0;
                    bVal = parseInt(b.dataset.taskTime) || 0;
                    return sortDir === 'asc' ? aVal - bVal : bVal - aVal;
                case 'duration':
                    // Parse duration text back to seconds for proper sorting
                    aVal = parseDurationToSeconds(a.cells[3].textContent);
                    bVal = parseDurationToSeconds(b.cells[3].textContent);
                    return sortDir === 'asc' ? aVal - bVal : bVal - aVal;
            }
            
            // String comparison for target and status
            if (sortDir === 'asc') {
                return aVal.localeCompare(bVal);
            } else {
                return bVal.localeCompare(aVal);
            }
        });
        
        // Re-append rows in sorted order
        rows.forEach(row => tbody.appendChild(row));
        
        // Update sort indicators
        updateSortIndicators(tbody.closest('table'), field, sortDir);
    }
    
    // Parse duration text to seconds
    function parseDurationToSeconds(duration) {
        if (duration === 'N/A' || duration === 'Running...') return 0;
        
        let seconds = 0;
        const parts = duration.match(/(\d+)([hms])/g);
        if (parts) {
            parts.forEach(part => {
                const value = parseInt(part);
                if (part.includes('h')) seconds += value * 3600;
                else if (part.includes('m')) seconds += value * 60;
                else if (part.includes('s')) seconds += value;
            });
        }
        return seconds;
    }
    
    // Update sort indicators
    function updateSortIndicators(table, sortField, sortDir) {
        // Reset all indicators
        table.querySelectorAll('th svg').forEach(svg => {
            svg.classList.remove('text-blue-600', 'dark:text-blue-400', 'rotate-180');
            svg.classList.add('text-gray-400');
        });
        
        // Update active indicator
        const headers = ['target', 'status', 'time', 'duration'];
        const index = headers.indexOf(sortField);
        if (index !== -1) {
            const svg = table.querySelectorAll('th svg')[index];
            svg.classList.remove('text-gray-400');
            svg.classList.add('text-blue-600', 'dark:text-blue-400');
            if (sortDir === 'desc') {
                svg.classList.add('rotate-180');
            }
        }
    }
    
    function applyFilters() {
        // Apply filters to all task rows
        state.taskRows.forEach((row, upid) => {
            let shouldShow = true;
            
            // Time range filter
            const taskTime = parseInt(row.dataset.taskTime, 10);
            const cutoffTime = Date.now() - TIME_RANGES[state.selectedTimeRange];
            const cutoffTimeSeconds = cutoffTime / 1000;
            
            if (taskTime && taskTime < cutoffTimeSeconds) {
                shouldShow = false;
            }
            
            
            // Apply visibility
            row.style.display = shouldShow ? '' : 'none';
        });
        
        // Update all summary counts
        updateAllSummaryCounts();
    }
    
    // Update all summary counts after filtering
    function updateAllSummaryCounts() {
        // Update all summary tables
        state.domElements.forEach((element, key) => {
            if (key.startsWith('summary-')) {
                const parts = key.split('-');
                const instanceIndex = parseInt(parts[1]);
                const instance = state.pbsData[instanceIndex];
                if (instance) {
                    const namespace = parts[2] || 'root';
                    const filteredInstance = filterInstanceByNamespace(instance, namespace);
                    const rows = element.querySelectorAll('tr');
                    rows.forEach(row => {
                        const taskType = row.dataset.taskType;
                        updateSummaryRow(row, taskType, filteredInstance);
                    });
                }
            }
        });
        
        // Update all task counts
        document.querySelectorAll('span[data-task-type]').forEach(span => {
            const instanceIndex = span.dataset.instanceIndex;
            const taskType = span.dataset.taskType;
            const namespace = state.selectedNamespaceByInstance.get(parseInt(instanceIndex)) || 'root';
            const tbody = state.domElements.get(`tasks-${instanceIndex}-${namespace}-${taskType}`);
            if (tbody) {
                updateTaskCount(span, tbody);
            }
        });
    }

    // Utility functions
    function formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }
    function parsePbsTaskTarget(task) {
        if (!task) return 'Unknown';
        
        // Get task type from the task object
        const taskType = task.type || '';
        
        // For backup and verify tasks, try to resolve VM/CT names
        if (['backup', 'verify', 'verificationjob', 'verify_group', 'verify-group'].includes(taskType)) {
            // Check if we have guest info directly on the task
            if (task.guestId) {
                const vmid = task.guestId;
                const guestType = task.guestType || 'vm';
                const guestName = getGuestNameById(vmid, guestType);
                
                if (guestName && guestName !== vmid) {
                    return `${guestName} (${guestType}/${vmid})`;
                } else {
                    return `${guestType}/${vmid}`;
                }
            }
            
            // Use task.id which contains worker_id info
            const workerId = task.id || '';
            
            // Try to parse from worker_id or targetId
            const patterns = [
                /^(vm|ct)\/(\d+)$/,  // Direct vm/101 or ct/100 format
                /:(vm|ct)\/(\d+)$/,  // Ends with :vm/101 or :ct/100
                /\/(vm|ct)\/(\d+)$/, // Ends with /vm/101 or /ct/100
            ];
            
            // Try patterns on workerId
            const testIds = [workerId];
            for (const testId of testIds) {
                for (const pattern of patterns) {
                    const match = testId.match(pattern);
                    if (match) {
                        const guestType = match[1];
                        const vmid = match[2];
                        const guestName = getGuestNameById(vmid, guestType);
                        
                        if (guestName && guestName !== vmid) {
                            return `${guestName} (${guestType}/${vmid})`;
                        } else {
                            return `${guestType}/${vmid}`;
                        }
                    }
                }
            }
            
            // For verification tasks, might have different format
            if (workerId.includes(':')) {
                const parts = workerId.split(':');
                if (parts.length >= 2) {
                    const lastPart = parts[parts.length - 1];
                    const vmMatch = lastPart.match(/^(vm|ct)\/(\d+)$/);
                    if (vmMatch) {
                        const guestType = vmMatch[1];
                        const vmid = vmMatch[2];
                        const guestName = getGuestNameById(vmid, guestType);
                        
                        if (guestName && guestName !== vmid) {
                            return `${guestName} (${guestType}/${vmid})`;
                        } else {
                            return `${guestType}/${vmid}`;
                        }
                    }
                }
                // Return the datastore name for datastore-level verify tasks
                return parts[0];
            }
        } else if (['sync', 'garbage_collection', 'prune'].includes(taskType)) {
            // For these tasks, use the worker ID directly
            return task.id || 'Unknown';
        }
        
        return task.id || 'Unknown';
    }
    
    // Helper function to get guest name by ID
    function getGuestNameById(vmid, type) {
        const vms = PulseApp.state?.get?.('vms') || [];
        const containers = PulseApp.state?.get?.('containers') || [];
        
        // Convert vmid to string for comparison
        const vmidStr = String(vmid);
        
        // Search in appropriate list based on type
        if (type === 'vm' || type === 'qemu') {
            const vm = vms.find(v => String(v.vmid) === vmidStr);
            return vm ? vm.name : vmidStr;
        } else if (type === 'ct' || type === 'lxc') {
            const ct = containers.find(c => String(c.vmid) === vmidStr);
            return ct ? ct.name : vmidStr;
        }
        
        // If type unknown, search both
        const allGuests = [...vms, ...containers];
        const guest = allGuests.find(g => String(g.vmid) === vmidStr);
        return guest ? guest.name : vmidStr;
    }

    function getPbsStatusDisplay(status) {
        if (!status) {
            return `
                <span class="text-blue-600 dark:text-blue-400 text-sm flex items-center gap-1">
                    <svg class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    Running
                </span>
            `;
        }
        
        if (status === 'OK') {
            return `
                <span class="text-green-600 dark:text-green-400 text-sm flex items-center gap-1">
                    <svg class="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
                    </svg>
                    OK
                </span>
            `;
        }
        
        return `
            <span class="text-red-600 dark:text-red-400 text-sm flex items-center gap-1">
                <svg class="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                </svg>
                ${status}
            </span>
        `;
    }

    function formatDuration(task) {
        if (!task.endTime || !task.startTime) {
            return task.status ? 'N/A' : 'Running...';
        }
        
        const duration = task.endTime - task.startTime;
        if (duration < 60) return `${duration}s`;
        if (duration < 3600) return `${Math.floor(duration / 60)}m ${duration % 60}s`;
        
        const hours = Math.floor(duration / 3600);
        const minutes = Math.floor((duration % 3600) / 60);
        return `${hours}h ${minutes}m`;
    }

    // Create server status section
    function createServerStatusSection(instance, instanceIndex) {
        const section = document.createElement('div');
        section.className = 'bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4';
        
        const heading = document.createElement('h4');
        heading.className = 'text-md font-semibold mb-3 text-gray-700 dark:text-gray-300';
        heading.textContent = 'Server Status';
        section.appendChild(heading);
        
        // Add table container with overflow-x-auto for mobile scrolling
        const tableContainer = document.createElement('div');
        tableContainer.className = 'overflow-x-auto';
        
        const table = document.createElement('table');
        table.className = 'min-w-full text-sm';
        
        const thead = document.createElement('thead');
        thead.innerHTML = `
            <tr class="text-xs font-medium text-left text-gray-600 uppercase dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
                <th class="pb-2 pr-4">PBS Version</th>
                <th class="pb-2 px-4">Uptime</th>
                <th class="pb-2 px-4 min-w-[180px]">CPU</th>
                <th class="pb-2 px-4 min-w-[180px]">Memory</th>
                <th class="pb-2 pl-4">Load</th>
            </tr>
        `;
        table.appendChild(thead);
        
        const tbody = document.createElement('tbody');
        const row = document.createElement('tr');
        
        // PBS Version
        const versionInfo = instance.versionInfo || {};
        const versionText = versionInfo.version ? (versionInfo.release ? `${versionInfo.version}-${versionInfo.release}` : versionInfo.version) : '-';
        const versionCell = document.createElement('td');
        versionCell.className = 'py-2 pr-4 text-gray-700 dark:text-gray-300';
        versionCell.textContent = versionText;
        row.appendChild(versionCell);
        
        // Uptime
        const nodeStatus = instance.nodeStatus || {};
        const uptimeCell = document.createElement('td');
        uptimeCell.className = 'py-2 px-4 text-gray-700 dark:text-gray-300';
        if (nodeStatus.uptime) {
            uptimeCell.textContent = PulseApp.utils.formatUptime(nodeStatus.uptime);
        } else {
            uptimeCell.innerHTML = '<span class="text-gray-400" title="Requires PBS node discovery">-</span>';
        }
        row.appendChild(uptimeCell);
        
        // CPU
        const cpuCell = document.createElement('td');
        cpuCell.className = 'py-2 px-4';
        if (nodeStatus.cpu !== null && nodeStatus.cpu !== undefined) {
            const cpuPercent = parseFloat((nodeStatus.cpu * 100).toFixed(1));
            const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent, 'cpu');
            const cpuTooltipText = `${cpuPercent}%`;
            cpuCell.innerHTML = PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass);
        } else {
            cpuCell.innerHTML = '<span class="text-gray-400" title="Requires PBS node discovery">-</span>';
        }
        row.appendChild(cpuCell);
        
        // Memory
        const memCell = document.createElement('td');
        memCell.className = 'py-2 px-4';
        if (nodeStatus.memory && nodeStatus.memory.total && nodeStatus.memory.used !== null) {
            const memUsed = nodeStatus.memory.used;
            const memTotal = nodeStatus.memory.total;
            const memPercent = parseFloat(((memUsed && memTotal > 0) ? (memUsed / memTotal * 100) : 0).toFixed(1));
            const memColorClass = PulseApp.utils.getUsageColor(memPercent, 'memory');
            const memTooltipText = `${PulseApp.utils.formatBytes(memUsed)} / ${PulseApp.utils.formatBytes(memTotal)} (${memPercent}%)`;
            memCell.innerHTML = PulseApp.utils.createProgressTextBarHTML(memPercent, memTooltipText, memColorClass);
        } else {
            memCell.innerHTML = '<span class="text-gray-400" title="Requires PBS node discovery">-</span>';
        }
        row.appendChild(memCell);
        
        // Load
        const loadCell = document.createElement('td');
        loadCell.className = 'py-2 pl-4 text-gray-700 dark:text-gray-300';
        if (nodeStatus.loadavg && Array.isArray(nodeStatus.loadavg) && nodeStatus.loadavg.length >= 1) {
            loadCell.textContent = nodeStatus.loadavg[0].toFixed(2);
        } else {
            loadCell.innerHTML = '<span class="text-gray-400" title="Requires PBS node discovery">-</span>';
        }
        row.appendChild(loadCell);
        
        tbody.appendChild(row);
        table.appendChild(tbody);
        tableContainer.appendChild(table);
        section.appendChild(tableContainer);
        
        return section;
    }
    
    // Create snapshots section
    function createSnapshotsSection(instance, instanceIndex) {
        // Get current namespace
        const namespace = state.selectedNamespaceByInstance.get(instanceIndex) || 'root';
        
        // Collect all snapshots from all datastores for this namespace
        let snapshots = [];
        if (instance.datastores) {
            instance.datastores.forEach(ds => {
                if (ds.snapshots) {
                    const nsSnapshots = ds.snapshots.filter(snap => 
                        (snap.namespace || '') === (namespace === 'root' ? '' : namespace)
                    );
                    snapshots = snapshots.concat(nsSnapshots.map(snap => ({
                        ...snap,
                        datastore: ds.name
                    })));
                }
            });
        }
        
        if (snapshots.length === 0) {
            return null; // Don't show section if no snapshots
        }
        
        const section = document.createElement('div');
        section.className = 'bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4';
        
        // Create collapsible header
        const header = document.createElement('div');
        header.className = 'flex justify-between items-center mb-3 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/30 -m-1 p-1 rounded transition-colors';
        
        const heading = document.createElement('h4');
        heading.className = 'text-md font-semibold text-gray-700 dark:text-gray-300';
        heading.textContent = `Backup Snapshots (${snapshots.length})`;
        
        const collapseIcon = document.createElement('span');
        collapseIcon.className = 'collapse-icon transition-transform duration-200';
        collapseIcon.innerHTML = `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"></path>
        </svg>`;
        
        header.appendChild(heading);
        header.appendChild(collapseIcon);
        section.appendChild(header);
        
        const tableContainer = document.createElement('div');
        tableContainer.className = 'overflow-x-auto transition-all duration-200';
        
        // Add collapse functionality
        header.addEventListener('click', () => {
            const isCollapsed = section.dataset.collapsed === 'true';
            section.dataset.collapsed = isCollapsed ? 'false' : 'true';
            tableContainer.style.display = isCollapsed ? 'block' : 'none';
            collapseIcon.style.transform = isCollapsed ? 'rotate(90deg)' : 'rotate(0deg)';
        });
        
        const table = document.createElement('table');
        table.className = 'min-w-full text-sm';
        
        const thead = document.createElement('thead');
        thead.innerHTML = `
            <tr class="text-xs font-medium text-left text-gray-600 uppercase dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
                <th class="pb-2 pr-4">Guest</th>
                <th class="pb-2 px-4">Time</th>
                <th class="pb-2 px-4">Size</th>
                <th class="pb-2 px-4">Owner</th>
                <th class="pb-2 px-4">Datastore</th>
                <th class="pb-2 px-4">Verified</th>
            </tr>
        `;
        table.appendChild(thead);
        
        const tbody = document.createElement('tbody');
        tbody.className = 'divide-y divide-gray-200 dark:divide-gray-700';
        
        snapshots.sort((a, b) => b['backup-time'] - a['backup-time']);
        
        const recentSnapshots = snapshots.slice(0, 50);
        
        recentSnapshots.forEach(snap => {
            const row = document.createElement('tr');
            row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700/50';
            
            const guestId = snap['backup-id'];
            const guestType = snap['backup-type'] === 'vm' ? 'VM' : 'CT';
            const time = new Date(snap['backup-time'] * 1000);
            const size = snap.size ? formatBytes(snap.size) : '-';
            const verified = snap.verification ? '✓' : '-';
            
            row.innerHTML = `
                <td class="py-2 pr-4">
                    <span class="font-medium">${guestType} ${guestId}</span>
                </td>
                <td class="py-2 px-4 text-gray-600 dark:text-gray-400">
                    ${time.toLocaleString()}
                </td>
                <td class="py-2 px-4">${size}</td>
                <td class="py-2 px-4 text-gray-600 dark:text-gray-400">${snap.owner || '-'}</td>
                <td class="py-2 px-4">${snap.datastore}</td>
                <td class="py-2 px-4 text-center">${verified}</td>
            `;
            
            tbody.appendChild(row);
        });
        
        table.appendChild(tbody);
        tableContainer.appendChild(table);
        section.appendChild(tableContainer);
        
        // Default state - start collapsed if more than 20 snapshots
        if (snapshots.length > 20) {
            section.dataset.collapsed = 'true';
            tableContainer.style.display = 'none';
            collapseIcon.style.transform = 'rotate(0deg)';
        }
        
        return section;
    }

    // Create datastores section
    function createDatastoresSection(instance, instanceIndex) {
        const section = document.createElement('div');
        section.className = 'bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4';
        
        const heading = document.createElement('h4');
        heading.className = 'text-md font-semibold mb-3 text-gray-700 dark:text-gray-300';
        heading.textContent = 'Datastores';
        section.appendChild(heading);
        
        if (!instance.datastores || instance.datastores.length === 0) {
            const noDataMsg = document.createElement('p');
            noDataMsg.className = 'text-sm text-gray-500 dark:text-gray-400 text-center py-4';
            noDataMsg.textContent = 'No datastores found or accessible.';
            section.appendChild(noDataMsg);
            return section;
        }
        
        const tableContainer = document.createElement('div');
        tableContainer.className = 'overflow-x-auto';
        
        const table = document.createElement('table');
        table.className = 'min-w-full text-sm';
        
        const thead = document.createElement('thead');
        thead.innerHTML = `
            <tr class="text-xs font-medium text-left text-gray-600 uppercase dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
                <th class="pb-2 pr-4">Name</th>
                <th class="pb-2 px-4">Path</th>
                <th class="pb-2 px-4">Used</th>
                <th class="pb-2 px-4">Available</th>
                <th class="pb-2 px-4">Total</th>
                <th class="pb-2 px-4 min-w-[150px]">Usage</th>
                <th class="pb-2 px-4">Deduplication</th>
                <th class="pb-2 pl-4">GC Status</th>
            </tr>
        `;
        table.appendChild(thead);
        
        const tbody = document.createElement('tbody');
        tbody.className = 'divide-y divide-gray-200 dark:divide-gray-700';
        
        instance.datastores.forEach(ds => {
            const row = document.createElement('tr');
            row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors';
            
            const totalBytes = ds.total || 0;
            const usedBytes = ds.used || 0;
            const availableBytes = (ds.available !== null && ds.available !== undefined) ? ds.available : (totalBytes > 0 ? totalBytes - usedBytes : 0);
            const usagePercent = totalBytes > 0 ? Math.round((usedBytes / totalBytes) * 100) : 0;
            
            // Apply special styling for high usage
            if (usagePercent >= 95) {
                row.className = 'bg-red-50 dark:bg-red-900/20 hover:bg-red-100 dark:hover:bg-red-900/30';
            } else if (usagePercent >= 85) {
                row.className = 'bg-yellow-50 dark:bg-yellow-900/20 hover:bg-yellow-100 dark:hover:bg-yellow-900/30';
            }
            
            // Name cell
            const nameCell = document.createElement('td');
            nameCell.className = 'py-2 pr-4 text-gray-700 dark:text-gray-300';
            if (usagePercent >= 95) {
                nameCell.className += ' text-red-700 dark:text-red-300 font-semibold';
                nameCell.textContent = `${ds.name || 'N/A'} [CRITICAL: ${usagePercent}% full]`;
            } else if (usagePercent >= 85) {
                nameCell.className += ' text-yellow-700 dark:text-yellow-300 font-semibold';
                nameCell.textContent = `${ds.name || 'N/A'} [WARNING: ${usagePercent}% full]`;
            } else {
                nameCell.textContent = ds.name || 'N/A';
            }
            row.appendChild(nameCell);
            
            // Path cell
            const pathCell = document.createElement('td');
            pathCell.className = 'py-2 px-4 text-gray-500 dark:text-gray-400 text-xs';
            pathCell.textContent = ds.path || 'N/A';
            row.appendChild(pathCell);
            
            // Used cell
            const usedCell = document.createElement('td');
            usedCell.className = 'py-2 px-4 text-gray-700 dark:text-gray-300';
            usedCell.textContent = ds.used !== null ? PulseApp.utils.formatBytes(ds.used) : 'N/A';
            row.appendChild(usedCell);
            
            // Available cell
            const availCell = document.createElement('td');
            availCell.className = 'py-2 px-4 text-gray-700 dark:text-gray-300';
            availCell.textContent = ds.available !== null ? PulseApp.utils.formatBytes(ds.available) : 'N/A';
            row.appendChild(availCell);
            
            // Total cell
            const totalCell = document.createElement('td');
            totalCell.className = 'py-2 px-4 text-gray-700 dark:text-gray-300';
            totalCell.textContent = ds.total !== null ? PulseApp.utils.formatBytes(ds.total) : 'N/A';
            row.appendChild(totalCell);
            
            // Usage cell with progress bar
            const usageCell = document.createElement('td');
            usageCell.className = 'py-2 px-4';
            if (totalBytes > 0) {
                const usageColor = PulseApp.utils.getUsageColor(usagePercent);
                const usageText = `${usagePercent}% (${PulseApp.utils.formatBytes(usedBytes)} of ${PulseApp.utils.formatBytes(totalBytes)})`;
                usageCell.innerHTML = PulseApp.utils.createProgressTextBarHTML(usagePercent, usageText, usageColor, `${usagePercent}%`);
            } else {
                usageCell.textContent = 'N/A';
            }
            row.appendChild(usageCell);
            
            // Deduplication cell
            const dedupCell = document.createElement('td');
            dedupCell.className = 'py-2 px-4 text-gray-700 dark:text-gray-300 font-semibold';
            dedupCell.textContent = ds.deduplicationFactor ? `${ds.deduplicationFactor}x` : 'N/A';
            row.appendChild(dedupCell);
            
            // GC Status cell
            const gcCell = document.createElement('td');
            gcCell.className = 'py-2 pl-4';
            gcCell.innerHTML = getGcStatusDisplay(ds.gcStatus);
            row.appendChild(gcCell);
            
            tbody.appendChild(row);
        });
        
        table.appendChild(tbody);
        tableContainer.appendChild(table);
        section.appendChild(tableContainer);
        
        return section;
    }
    
    // Get GC status display
    function getGcStatusDisplay(gcStatus) {
        if (!gcStatus) {
            return '<span class="text-gray-500 dark:text-gray-400 text-xs">Unknown</span>';
        }
        
        if (gcStatus === 'OK' || gcStatus === 'ok') {
            return '<span class="text-green-600 dark:text-green-400 text-xs font-medium">OK</span>';
        } else if (gcStatus === 'running' || gcStatus === 'Running') {
            return '<span class="text-blue-600 dark:text-blue-400 text-xs font-medium">Running</span>';
        } else if (gcStatus === 'pending' || gcStatus === 'Pending') {
            return '<span class="text-yellow-600 dark:text-yellow-400 text-xs font-medium">Pending</span>';
        } else {
            return `<span class="text-gray-600 dark:text-gray-400 text-xs">${gcStatus}</span>`;
        }
    }
    
    // Show task details modal
    function showTaskDetailsModal(task, taskType) {
        // Create modal overlay
        const overlay = document.createElement('div');
        overlay.className = 'fixed inset-0 bg-black bg-opacity-50 z-50 flex items-center justify-center p-4';
        
        // Create modal content
        const modal = document.createElement('div');
        modal.className = 'bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full max-h-[80vh] overflow-hidden';
        
        // Modal header
        const header = document.createElement('div');
        header.className = 'p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between';
        header.innerHTML = `
            <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Task Details</h3>
            <button class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 transition-colors">
                <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                </svg>
            </button>
        `;
        
        // Close button handler
        header.querySelector('button').addEventListener('click', () => {
            overlay.remove();
        });
        
        // Modal body
        const body = document.createElement('div');
        body.className = 'p-4 overflow-y-auto max-h-[60vh]';
        
        // Parse UPID for additional details
        const upidParts = task.upid ? task.upid.split(':') : [];
        const nodeName = upidParts[3] || 'Unknown';
        const taskTypeFromUpid = upidParts[1] || taskType;
        const pid = upidParts[4] || 'Unknown';
        
        // Format task details
        const details = [
            { label: 'Task Type', value: taskTypeFromUpid },
            { label: 'Target', value: parsePbsTaskTarget(task) },
            { label: 'Status', value: task.status || 'Running', isStatus: true },
            { label: 'Node', value: nodeName },
            { label: 'Process ID', value: pid },
            { label: 'Start Time', value: task.startTime ? new Date(task.startTime * 1000).toLocaleString() : 'N/A' },
            { label: 'End Time', value: task.endTime ? new Date(task.endTime * 1000).toLocaleString() : 'N/A' },
            { label: 'Duration', value: formatDuration(task) },
            { label: 'Worker ID', value: task.id || 'N/A', monospace: true },
            { label: 'Guest', value: task.guest || 'N/A' },
            { label: 'UPID', value: task.upid || 'N/A', monospace: true }
        ];
        
        // Add task log if available
        if (task.log && task.log.length > 0) {
            details.push({ label: 'Task Log', value: task.log.join('\n'), monospace: true, multiline: true });
        }
        
        // Build details HTML
        let detailsHTML = '<div class="space-y-3">';
        details.forEach(detail => {
            if (detail.multiline) {
                detailsHTML += `
                    <div>
                        <div class="text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">${detail.label}</div>
                        <pre class="bg-gray-100 dark:bg-gray-900 p-3 rounded text-xs text-gray-800 dark:text-gray-200 overflow-x-auto">${detail.value}</pre>
                    </div>
                `;
            } else {
                detailsHTML += `
                    <div class="flex flex-col sm:flex-row sm:items-center">
                        <div class="text-sm font-medium text-gray-600 dark:text-gray-400 sm:w-32">${detail.label}:</div>
                        <div class="text-sm text-gray-900 dark:text-gray-100 ${detail.monospace ? 'font-mono text-xs' : ''} ${detail.isStatus ? '' : ''}">
                            ${detail.isStatus ? getPbsStatusDisplay(detail.value) : detail.value}
                        </div>
                    </div>
                `;
            }
        });
        detailsHTML += '</div>';
        
        body.innerHTML = detailsHTML;
        
        // Assemble modal
        modal.appendChild(header);
        modal.appendChild(body);
        overlay.appendChild(modal);
        
        // Close on overlay click
        overlay.addEventListener('click', (e) => {
            if (e.target === overlay) {
                overlay.remove();
            }
        });
        
        // Close on Escape key
        const handleEscape = (e) => {
            if (e.key === 'Escape') {
                overlay.remove();
                document.removeEventListener('keydown', handleEscape);
            }
        };
        document.addEventListener('keydown', handleEscape);
        
        // Add to DOM
        document.body.appendChild(overlay);
    }
    
    // Public API
    return {
        init,
        updatePbsInfo
    };
})();