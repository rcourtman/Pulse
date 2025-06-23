PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.pbs2 = (() => {
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

    // Initialize the PBS2 tab
    function init() {
        console.log('[PBS2] Initializing PBS2 tab');
        const container = document.getElementById('pbs2-instances-container');
        if (!container) {
            console.error('[PBS2] Container not found');
            return;
        }

        // Clear any existing content
        container.innerHTML = '';
        
        // Remove loading message
        const loadingMsg = document.getElementById('pbs2-loading-message');
        if (loadingMsg) loadingMsg.style.display = 'none';
        
        // Get initial data
        const pbsData = PulseApp.state?.get?.('pbsDataArray') || PulseApp.state?.pbs || [];
        console.log('[PBS2] Initial PBS data:', pbsData);
        
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
        contentArea.id = 'pbs2-content';
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
        const contentArea = document.getElementById('pbs2-content');
        if (!contentArea) return;
        
        const instance = state.pbsData[state.activeInstanceIndex];
        if (!instance) return;
        
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
            tab.textContent = namespace === 'root' ? 'Root' : namespace;
            
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
        
        // Task summary table
        const summaryTable = createTaskSummaryTable(instance, instanceIndex);
        container.appendChild(summaryTable);
        
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
        content.className = 'flex items-start justify-between gap-3';
        
        // Left side - info
        const leftContent = document.createElement('div');
        leftContent.className = 'flex items-start gap-3';
        
        const iconDiv = document.createElement('div');
        iconDiv.className = 'flex-shrink-0 mt-0.5';
        iconDiv.innerHTML = `
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-blue-600 dark:text-blue-400">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="16" x2="12" y2="12"></line>
                <line x1="12" y1="8" x2="12.01" y2="8"></line>
            </svg>
        `;
        leftContent.appendChild(iconDiv);
        
        const textDiv = document.createElement('div');
        textDiv.className = 'flex-1';
        
        const titleDiv = document.createElement('div');
        titleDiv.className = 'font-semibold text-gray-800 dark:text-gray-200 mb-1';
        titleDiv.textContent = 'Time Range';
        textDiv.appendChild(titleDiv);
        
        const descriptionDiv = document.createElement('div');
        descriptionDiv.id = 'pbs2-date-range-description';
        descriptionDiv.className = 'text-sm text-gray-700 dark:text-gray-300';
        textDiv.appendChild(descriptionDiv);
        
        leftContent.appendChild(textDiv);
        content.appendChild(leftContent);
        
        // Right side - selector
        const selectorDiv = document.createElement('div');
        selectorDiv.className = 'flex-shrink-0';
        
        const selector = document.createElement('select');
        selector.id = 'pbs2-time-range-selector';
        selector.className = 'px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500';
        
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
        
        // Add change handler for instant filtering
        selector.addEventListener('change', handleTimeRangeChange);
        
        selectorDiv.appendChild(selector);
        content.appendChild(selectorDiv);
        
        card.appendChild(content);
        
        // Update description
        updateDateRangeDescription();
        
        return card;
    }

    // Handle time range change - instant visual update
    function handleTimeRangeChange(e) {
        state.selectedTimeRange = e.target.value;
        updateDateRangeDescription();
        
        // Instant visual filtering of existing rows
        const cutoffTime = Date.now() - TIME_RANGES[state.selectedTimeRange];
        const cutoffTimeSeconds = cutoffTime / 1000;
        
        // Update all task rows visibility
        state.taskRows.forEach((row, upid) => {
            const taskTime = parseInt(row.dataset.taskTime, 10);
            if (taskTime && taskTime < cutoffTimeSeconds) {
                row.style.display = 'none';
            } else {
                row.style.display = '';
            }
        });
        
        // Update all summary cards and counts
        updateAllSummaryCounts();
    }

    // Update date range description
    function updateDateRangeDescription() {
        const description = document.getElementById('pbs2-date-range-description');
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
        tbody.className = 'pbs2-task-summary-tbody';
        tbody.dataset.instanceIndex = instanceIndex;
        
        // Add rows for each task type
        const taskTypes = ['Backups', 'Verification', 'Sync', 'Prune/GC'];
        taskTypes.forEach(type => {
            const row = createSummaryRow(type, instance, instanceIndex);
            tbody.appendChild(row);
        });
        
        table.appendChild(tbody);
        section.appendChild(table);
        
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
            row.cells[1].innerHTML = '<span class="text-gray-500">No tasks</span>';
            row.cells[2].textContent = 'N/A';
            row.cells[3].textContent = 'N/A';
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
        
        // Add collapse/expand all buttons
        const controlsDiv = document.createElement('div');
        controlsDiv.className = 'flex gap-2 mb-2';
        
        const collapseAllBtn = document.createElement('button');
        collapseAllBtn.className = 'text-xs px-2 py-1 text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 border border-gray-300 dark:border-gray-600 rounded hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors';
        collapseAllBtn.innerHTML = `
            <span class="flex items-center gap-1">
                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="18 15 12 9 6 15"></polyline>
                </svg>
                Collapse All
            </span>
        `;
        collapseAllBtn.addEventListener('click', () => {
            container.querySelectorAll('.bg-white.dark\\:bg-gray-800').forEach(section => {
                if (section.dataset.collapsed !== 'true') {
                    toggleTaskSection(section);
                }
            });
        });
        
        const expandAllBtn = document.createElement('button');
        expandAllBtn.className = 'text-xs px-2 py-1 text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 border border-gray-300 dark:border-gray-600 rounded hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors';
        expandAllBtn.innerHTML = `
            <span class="flex items-center gap-1">
                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="6 9 12 15 18 9"></polyline>
                </svg>
                Expand All
            </span>
        `;
        expandAllBtn.addEventListener('click', () => {
            container.querySelectorAll('.bg-white.dark\\:bg-gray-800').forEach(section => {
                if (section.dataset.collapsed === 'true') {
                    toggleTaskSection(section);
                }
            });
        });
        
        controlsDiv.appendChild(collapseAllBtn);
        controlsDiv.appendChild(expandAllBtn);
        container.appendChild(controlsDiv);
        
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
        
        // Header (now clickable for collapse/expand)
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
        thead.innerHTML = `
            <tr class="text-xs font-medium text-left text-gray-600 uppercase dark:text-gray-300">
                <th class="p-1 px-2">Target</th>
                <th class="p-1 px-2">Status</th>
                <th class="p-1 px-2">Start Time</th>
                <th class="p-1 px-2">Duration</th>
            </tr>
        `;
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
        row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors duration-150';
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
        console.log('[PBS2] Received update with data:', pbsArray);
        state.pbsData = pbsArray || [];
        
        const container = document.getElementById('pbs2-instances-container');
        if (!container) return;
        
        // If no content yet, do initial render
        const contentArea = document.getElementById('pbs2-content');
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
    function parsePbsTaskTarget(task) {
        if (!task || !task.upid) return 'Unknown';
        
        const upidParts = task.upid.split(':');
        if (upidParts.length < 8) return 'Unknown';
        
        const taskType = upidParts[1];
        const targetId = upidParts[2];
        
        // Return appropriate target based on task type
        if (['backup'].includes(taskType)) {
            return targetId.replace('vm/', 'VM ');
        } else if (['verify'].includes(taskType)) {
            return targetId;
        } else if (['sync', 'garbage_collection', 'prune'].includes(taskType)) {
            return targetId;
        }
        
        return targetId || 'Unknown';
    }

    function getPbsStatusDisplay(status) {
        if (!status) {
            return '<span class="text-blue-600 dark:text-blue-400 text-sm">Running...</span>';
        }
        
        if (status === 'OK') {
            return '<span class="text-green-600 dark:text-green-400 text-sm">OK</span>';
        }
        
        return `<span class="text-red-600 dark:text-red-400 text-sm">${status}</span>`;
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

    // Public API
    return {
        init,
        updatePbsInfo
    };
})();