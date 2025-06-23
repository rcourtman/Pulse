PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.pbs2 = (() => {
    // State management
    let state = {
        pbsData: [],
        selectedTimeRange: '7d',
        selectedInstance: 0,
        selectedNamespaceByInstance: new Map(),
        searchQuery: '',
        showOnlyFailures: false,
        expandedSections: new Set(['health', 'active', 'failures'])
    };

    // Time range constants
    const TIME_RANGES = {
        '1h': 60 * 60 * 1000,
        '24h': 24 * 60 * 60 * 1000,
        '7d': 7 * 24 * 60 * 60 * 1000,
        '30d': 30 * 24 * 60 * 60 * 1000
    };

    // Initialize the PBS2 tab
    function init() {
        console.log('[PBS2] Initializing PBS2 dashboard');
        const container = document.getElementById('pbs2-instances-container');
        if (!container) {
            console.error('[PBS2] Container not found');
            return;
        }

        // Clear any existing content
        container.innerHTML = '';
        
        // Get initial data
        const pbsData = PulseApp.state?.get?.('pbsDataArray') || PulseApp.state?.pbs || [];
        console.log('[PBS2] Initial PBS data:', pbsData);
        
        // Initialize with data
        state.pbsData = pbsData;
        renderDashboard(container);
    }

    // Main render function
    function renderDashboard(container) {
        container.innerHTML = '';
        container.className = 'p-4 space-y-6';
        
        if (!state.pbsData || state.pbsData.length === 0) {
            container.innerHTML = `
                <div class="text-center py-12">
                    <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"></path>
                    </svg>
                    <h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">No PBS Servers Connected</h3>
                    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">Configure Proxmox Backup Server integration to see backup status.</p>
                </div>
            `;
            return;
        }
        
        // Create main dashboard sections
        const dashboard = document.createElement('div');
        dashboard.className = 'space-y-6';
        
        // 1. Header with controls
        dashboard.appendChild(createHeader());
        
        // 2. Instance selector (if multiple)
        if (state.pbsData.length > 1) {
            dashboard.appendChild(createInstanceSelector());
        }
        
        // 3. Health overview cards
        dashboard.appendChild(createHealthOverview());
        
        // 4. Active tasks section
        dashboard.appendChild(createActiveTasksSection());
        
        // 5. Recent failures section
        dashboard.appendChild(createFailuresSection());
        
        // 6. Task history with charts
        dashboard.appendChild(createTaskHistorySection());
        
        // 7. Storage overview
        dashboard.appendChild(createStorageOverview());
        
        container.appendChild(dashboard);
    }

    // Create header with controls
    function createHeader() {
        const header = document.createElement('div');
        header.className = 'bg-white dark:bg-gray-800 rounded-lg shadow-sm p-4';
        
        header.innerHTML = `
            <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
                <div>
                    <h2 class="text-xl font-semibold text-gray-900 dark:text-gray-100">PBS Dashboard</h2>
                    <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">Monitor and manage your backup infrastructure</p>
                </div>
                
                <div class="flex flex-wrap items-center gap-3">
                    <!-- Search -->
                    <div class="relative">
                        <input type="text" 
                               id="pbs2-search" 
                               placeholder="Search tasks..."
                               class="pl-9 pr-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent">
                        <svg class="absolute left-3 top-2.5 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                        </svg>
                    </div>
                    
                    <!-- Time range selector -->
                    <select id="pbs2-time-range" 
                            class="text-sm px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500">
                        <option value="1h">Last Hour</option>
                        <option value="24h">Last 24 Hours</option>
                        <option value="7d" selected>Last 7 Days</option>
                        <option value="30d">Last 30 Days</option>
                    </select>
                    
                    <!-- Quick filters -->
                    <button id="pbs2-failures-toggle" 
                            class="text-sm px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors">
                        <span class="flex items-center gap-1">
                            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                            </svg>
                            Show Only Failures
                        </span>
                    </button>
                    
                    <!-- Refresh button -->
                    <button id="pbs2-refresh" 
                            class="text-sm px-3 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors">
                        <span class="flex items-center gap-1">
                            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                            </svg>
                            Refresh
                        </span>
                    </button>
                </div>
            </div>
        `;
        
        // Add event listeners
        const searchInput = header.querySelector('#pbs2-search');
        searchInput.addEventListener('input', (e) => {
            state.searchQuery = e.target.value.toLowerCase();
            updateFilteredContent();
        });
        
        const timeRange = header.querySelector('#pbs2-time-range');
        timeRange.addEventListener('change', (e) => {
            state.selectedTimeRange = e.target.value;
            refreshDashboard();
        });
        
        const failuresToggle = header.querySelector('#pbs2-failures-toggle');
        failuresToggle.addEventListener('click', () => {
            state.showOnlyFailures = !state.showOnlyFailures;
            failuresToggle.classList.toggle('bg-red-50', state.showOnlyFailures);
            failuresToggle.classList.toggle('dark:bg-red-900/20', state.showOnlyFailures);
            failuresToggle.classList.toggle('border-red-300', state.showOnlyFailures);
            failuresToggle.classList.toggle('dark:border-red-700', state.showOnlyFailures);
            updateFilteredContent();
        });
        
        const refreshBtn = header.querySelector('#pbs2-refresh');
        refreshBtn.addEventListener('click', () => {
            if (PulseApp.socketHandler) {
                PulseApp.socketHandler.requestData();
            }
        });
        
        return header;
    }

    // Create instance selector
    function createInstanceSelector() {
        const selector = document.createElement('div');
        selector.className = 'bg-white dark:bg-gray-800 rounded-lg shadow-sm p-3';
        
        const tabs = document.createElement('div');
        tabs.className = 'flex flex-wrap gap-2';
        
        state.pbsData.forEach((instance, index) => {
            const tab = document.createElement('button');
            tab.className = index === state.selectedInstance ?
                'px-4 py-2 text-sm font-medium rounded-md bg-blue-600 text-white' :
                'px-4 py-2 text-sm font-medium rounded-md bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors';
            tab.textContent = instance.pbsInstanceName || `PBS ${index + 1}`;
            
            tab.addEventListener('click', () => {
                state.selectedInstance = index;
                refreshDashboard();
            });
            
            tabs.appendChild(tab);
        });
        
        selector.appendChild(tabs);
        return selector;
    }

    // Create health overview cards
    function createHealthOverview() {
        const instance = state.pbsData[state.selectedInstance];
        if (!instance) return document.createElement('div');
        
        const section = document.createElement('div');
        section.className = 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4';
        
        // Calculate metrics
        const metrics = calculateHealthMetrics(instance);
        
        // Status card
        section.appendChild(createMetricCard(
            'System Status',
            metrics.status,
            metrics.statusColor,
            `
                <svg class="h-8 w-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>
            `,
            metrics.statusDetails
        ));
        
        // Active tasks card
        section.appendChild(createMetricCard(
            'Active Tasks',
            metrics.activeTasks.toString(),
            metrics.activeTasks > 0 ? 'blue' : 'gray',
            `
                <svg class="h-8 w-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                </svg>
            `,
            `${metrics.runningBackups} backups, ${metrics.runningVerify} verifications`
        ));
        
        // Success rate card
        section.appendChild(createMetricCard(
            'Success Rate',
            `${metrics.successRate}%`,
            metrics.successRate >= 95 ? 'green' : metrics.successRate >= 80 ? 'yellow' : 'red',
            createProgressRing(metrics.successRate),
            `${metrics.successfulTasks} of ${metrics.totalTasks} tasks`
        ));
        
        // Recent failures card
        section.appendChild(createMetricCard(
            'Recent Failures',
            metrics.recentFailures.toString(),
            metrics.recentFailures > 0 ? 'red' : 'green',
            `
                <svg class="h-8 w-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>
            `,
            metrics.failureDetails
        ));
        
        return section;
    }

    // Create metric card
    function createMetricCard(title, value, color, icon, subtitle) {
        const card = document.createElement('div');
        card.className = 'bg-white dark:bg-gray-800 rounded-lg shadow-sm p-4 border border-gray-200 dark:border-gray-700';
        
        const colorClasses = {
            green: 'text-green-600 dark:text-green-400',
            red: 'text-red-600 dark:text-red-400',
            yellow: 'text-yellow-600 dark:text-yellow-400',
            blue: 'text-blue-600 dark:text-blue-400',
            gray: 'text-gray-600 dark:text-gray-400'
        };
        
        card.innerHTML = `
            <div class="flex items-start justify-between">
                <div class="flex-1">
                    <p class="text-sm font-medium text-gray-600 dark:text-gray-400">${title}</p>
                    <p class="mt-2 text-2xl font-semibold ${colorClasses[color] || colorClasses.gray}">${value}</p>
                    ${subtitle ? `<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">${subtitle}</p>` : ''}
                </div>
                <div class="${colorClasses[color] || colorClasses.gray}">
                    ${icon}
                </div>
            </div>
        `;
        
        return card;
    }

    // Create progress ring SVG
    function createProgressRing(percentage) {
        const radius = 16;
        const circumference = 2 * Math.PI * radius;
        const offset = circumference - (percentage / 100) * circumference;
        
        return `
            <svg class="h-8 w-8 transform -rotate-90">
                <circle cx="20" cy="20" r="${radius}" stroke="currentColor" stroke-width="4" fill="none" opacity="0.3" />
                <circle cx="20" cy="20" r="${radius}" stroke="currentColor" stroke-width="4" fill="none"
                        stroke-dasharray="${circumference}"
                        stroke-dashoffset="${offset}"
                        stroke-linecap="round" />
            </svg>
        `;
    }

    // Create active tasks section
    function createActiveTasksSection() {
        const instance = state.pbsData[state.selectedInstance];
        if (!instance) return document.createElement('div');
        
        const activeTasks = getActiveTasks(instance);
        if (activeTasks.length === 0) return document.createElement('div');
        
        const section = createCollapsibleSection(
            'Active Tasks',
            `${activeTasks.length} running`,
            'active',
            'blue'
        );
        
        const content = section.querySelector('.section-content');
        const grid = document.createElement('div');
        grid.className = 'grid gap-3';
        
        activeTasks.forEach(task => {
            grid.appendChild(createTaskCard(task, 'active'));
        });
        
        content.appendChild(grid);
        return section;
    }

    // Create failures section
    function createFailuresSection() {
        const instance = state.pbsData[state.selectedInstance];
        if (!instance) return document.createElement('div');
        
        const failedTasks = getFailedTasks(instance);
        if (failedTasks.length === 0 && !state.showOnlyFailures) return document.createElement('div');
        
        const section = createCollapsibleSection(
            'Recent Failures',
            failedTasks.length === 0 ? 'No failures' : `${failedTasks.length} failures`,
            'failures',
            failedTasks.length > 0 ? 'red' : 'green'
        );
        
        const content = section.querySelector('.section-content');
        
        if (failedTasks.length === 0) {
            content.innerHTML = `
                <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                    <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                    </svg>
                    <p class="mt-2">No failures in the selected time range</p>
                </div>
            `;
        } else {
            const grid = document.createElement('div');
            grid.className = 'grid gap-3';
            
            failedTasks.forEach(task => {
                grid.appendChild(createTaskCard(task, 'failed'));
            });
            
            content.appendChild(grid);
        }
        
        return section;
    }

    // Create task history section
    function createTaskHistorySection() {
        const instance = state.pbsData[state.selectedInstance];
        if (!instance) return document.createElement('div');
        
        const section = createCollapsibleSection(
            'Task History',
            'All recent tasks',
            'history',
            'gray'
        );
        
        const content = section.querySelector('.section-content');
        
        // Add task type tabs
        const tabs = document.createElement('div');
        tabs.className = 'border-b border-gray-200 dark:border-gray-700 mb-4';
        tabs.innerHTML = `
            <nav class="-mb-px flex space-x-4">
                <button class="task-type-tab active py-2 px-1 border-b-2 border-blue-500 font-medium text-sm text-blue-600 dark:text-blue-400" data-type="all">
                    All Tasks
                </button>
                <button class="task-type-tab py-2 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm" data-type="backup">
                    Backups
                </button>
                <button class="task-type-tab py-2 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm" data-type="verify">
                    Verifications
                </button>
                <button class="task-type-tab py-2 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm" data-type="sync">
                    Sync
                </button>
                <button class="task-type-tab py-2 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 font-medium text-sm" data-type="prune">
                    Prune/GC
                </button>
            </nav>
        `;
        
        // Add tab click handlers
        tabs.querySelectorAll('.task-type-tab').forEach(tab => {
            tab.addEventListener('click', () => {
                tabs.querySelectorAll('.task-type-tab').forEach(t => {
                    t.classList.remove('active', 'border-blue-500', 'text-blue-600', 'dark:text-blue-400');
                    t.classList.add('border-transparent', 'text-gray-500', 'dark:text-gray-400');
                });
                tab.classList.add('active', 'border-blue-500', 'text-blue-600', 'dark:text-blue-400');
                tab.classList.remove('border-transparent', 'text-gray-500', 'dark:text-gray-400');
                
                // Filter tasks
                const type = tab.dataset.type;
                renderTaskHistory(content.querySelector('.task-history-content'), type);
            });
        });
        
        content.appendChild(tabs);
        
        // Task history content
        const historyContent = document.createElement('div');
        historyContent.className = 'task-history-content';
        content.appendChild(historyContent);
        
        // Render all tasks initially
        renderTaskHistory(historyContent, 'all');
        
        return section;
    }

    // Render task history
    function renderTaskHistory(container, taskType) {
        const instance = state.pbsData[state.selectedInstance];
        if (!instance) return;
        
        container.innerHTML = '';
        
        const tasks = getAllTasks(instance, taskType);
        
        if (tasks.length === 0) {
            container.innerHTML = `
                <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                    <p>No tasks found</p>
                </div>
            `;
            return;
        }
        
        // Create table
        const table = document.createElement('div');
        table.className = 'overflow-x-auto';
        table.innerHTML = `
            <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead class="bg-gray-50 dark:bg-gray-700">
                    <tr>
                        <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Type</th>
                        <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Target</th>
                        <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Status</th>
                        <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Started</th>
                        <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Duration</th>
                    </tr>
                </thead>
                <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                    ${tasks.map(task => createTaskRow(task)).join('')}
                </tbody>
            </table>
        `;
        
        container.appendChild(table);
    }

    // Create storage overview
    function createStorageOverview() {
        const instance = state.pbsData[state.selectedInstance];
        if (!instance || !instance.datastores) return document.createElement('div');
        
        const section = createCollapsibleSection(
            'Storage Overview',
            `${instance.datastores.length} datastores`,
            'storage',
            'gray'
        );
        
        const content = section.querySelector('.section-content');
        const grid = document.createElement('div');
        grid.className = 'grid grid-cols-1 lg:grid-cols-2 gap-4';
        
        instance.datastores.forEach(datastore => {
            grid.appendChild(createDatastoreCard(datastore));
        });
        
        content.appendChild(grid);
        return section;
    }

    // Create collapsible section
    function createCollapsibleSection(title, subtitle, id, color = 'gray') {
        const section = document.createElement('div');
        section.className = 'bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700';
        
        const colorClasses = {
            green: 'text-green-600 dark:text-green-400',
            red: 'text-red-600 dark:text-red-400',
            yellow: 'text-yellow-600 dark:text-yellow-400',
            blue: 'text-blue-600 dark:text-blue-400',
            gray: 'text-gray-600 dark:text-gray-400'
        };
        
        const isExpanded = state.expandedSections.has(id);
        
        section.innerHTML = `
            <div class="section-header p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors" data-section="${id}">
                <div class="flex items-center justify-between">
                    <div class="flex items-center gap-3">
                        <svg class="section-chevron h-5 w-5 text-gray-400 transition-transform ${isExpanded ? '' : '-rotate-90'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
                        </svg>
                        <div>
                            <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">${title}</h3>
                            <p class="text-sm ${colorClasses[color]}">${subtitle}</p>
                        </div>
                    </div>
                </div>
            </div>
            <div class="section-content ${isExpanded ? '' : 'hidden'} p-4 pt-0">
                <!-- Content will be added here -->
            </div>
        `;
        
        // Add toggle handler
        const header = section.querySelector('.section-header');
        header.addEventListener('click', () => {
            const content = section.querySelector('.section-content');
            const chevron = section.querySelector('.section-chevron');
            const sectionId = header.dataset.section;
            
            content.classList.toggle('hidden');
            chevron.classList.toggle('-rotate-90');
            
            if (state.expandedSections.has(sectionId)) {
                state.expandedSections.delete(sectionId);
            } else {
                state.expandedSections.add(sectionId);
            }
        });
        
        return section;
    }

    // Create task card for active/failed tasks
    function createTaskCard(task, type) {
        const card = document.createElement('div');
        card.className = type === 'failed' ? 
            'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4' :
            'bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4';
        
        const taskType = getTaskType(task);
        const target = parsePbsTaskTarget(task);
        const startTime = task.startTime ? PulseApp.utils.formatPbsTimestampRelative(task.startTime) : 'Unknown';
        const duration = formatDuration(task);
        
        card.innerHTML = `
            <div class="flex items-start justify-between">
                <div class="flex-1">
                    <div class="flex items-center gap-2">
                        <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${getTaskTypeColor(taskType)}">
                            ${taskType}
                        </span>
                        <span class="text-sm font-medium text-gray-900 dark:text-gray-100">${target}</span>
                    </div>
                    <div class="mt-2 text-sm text-gray-600 dark:text-gray-400">
                        <p>Started: ${startTime}</p>
                        ${type === 'active' ? 
                            `<p>Running for: ${duration}</p>` :
                            `<p>Duration: ${duration}</p>`
                        }
                        ${task.status && task.status !== 'OK' && task.status !== 'Running' ? 
                            `<p class="mt-1 text-red-600 dark:text-red-400">Error: ${task.status}</p>` : 
                            ''
                        }
                    </div>
                </div>
                ${type === 'active' ? `
                    <div class="ml-4">
                        <svg class="animate-spin h-5 w-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24">
                            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                    </div>
                ` : ''}
            </div>
        `;
        
        return card;
    }

    // Create task row for history table
    function createTaskRow(task) {
        const taskType = getTaskType(task);
        const target = parsePbsTaskTarget(task);
        const status = getPbsStatusBadge(task.status);
        const startTime = task.startTime ? PulseApp.utils.formatPbsTimestampRelative(task.startTime) : 'Unknown';
        const duration = formatDuration(task);
        
        return `
            <tr class="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                <td class="px-3 py-2 whitespace-nowrap">
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${getTaskTypeColor(taskType)}">
                        ${taskType}
                    </span>
                </td>
                <td class="px-3 py-2 text-sm text-gray-900 dark:text-gray-100">${target}</td>
                <td class="px-3 py-2 text-sm">${status}</td>
                <td class="px-3 py-2 text-sm text-gray-600 dark:text-gray-400">${startTime}</td>
                <td class="px-3 py-2 text-sm text-gray-600 dark:text-gray-400">${duration}</td>
            </tr>
        `;
    }

    // Create datastore card
    function createDatastoreCard(datastore) {
        const card = document.createElement('div');
        card.className = 'border border-gray-200 dark:border-gray-700 rounded-lg p-4';
        
        const usagePercent = datastore.total > 0 ? Math.round((datastore.used / datastore.total) * 100) : 0;
        const usageColor = usagePercent >= 90 ? 'red' : usagePercent >= 75 ? 'yellow' : 'green';
        
        card.innerHTML = `
            <div class="flex items-start justify-between mb-3">
                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">${datastore.name}</h4>
                <span class="text-sm ${getUsageColor(usagePercent)}">${usagePercent}%</span>
            </div>
            
            <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2 mb-3">
                <div class="h-2 rounded-full ${getUsageBarColor(usagePercent)}" style="width: ${usagePercent}%"></div>
            </div>
            
            <div class="flex justify-between text-xs text-gray-600 dark:text-gray-400">
                <span>${formatBytes(datastore.used)} used</span>
                <span>${formatBytes(datastore.avail)} free</span>
            </div>
            
            ${datastore.namespaces && datastore.namespaces.length > 0 ? `
                <div class="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700">
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                        ${datastore.namespaces.length} namespaces
                    </p>
                </div>
            ` : ''}
        `;
        
        return card;
    }

    // Calculate health metrics
    function calculateHealthMetrics(instance) {
        const cutoffTime = Date.now() - TIME_RANGES[state.selectedTimeRange];
        const cutoffTimeSeconds = cutoffTime / 1000;
        
        let totalTasks = 0;
        let successfulTasks = 0;
        let failedTasks = 0;
        let activeTasks = 0;
        let runningBackups = 0;
        let runningVerify = 0;
        
        // Count tasks
        const taskTypes = ['backupTasks', 'verificationTasks', 'syncTasks', 'pruneTasks'];
        taskTypes.forEach(taskType => {
            if (instance[taskType] && instance[taskType].recentTasks) {
                instance[taskType].recentTasks.forEach(task => {
                    const taskTime = task.startTime || task.starttime || 0;
                    if (taskTime >= cutoffTimeSeconds) {
                        totalTasks++;
                        
                        if (!task.status || task.status.toLowerCase().includes('running')) {
                            activeTasks++;
                            if (taskType === 'backupTasks') runningBackups++;
                            if (taskType === 'verificationTasks') runningVerify++;
                        } else if (task.status === 'OK') {
                            successfulTasks++;
                        } else {
                            failedTasks++;
                        }
                    }
                });
            }
        });
        
        const successRate = totalTasks > 0 ? Math.round((successfulTasks / totalTasks) * 100) : 100;
        
        // Determine overall status
        let status = 'Healthy';
        let statusColor = 'green';
        let statusDetails = 'All systems operational';
        
        if (failedTasks > 0) {
            if (successRate < 80) {
                status = 'Critical';
                statusColor = 'red';
                statusDetails = 'Multiple failures detected';
            } else if (successRate < 95) {
                status = 'Warning';
                statusColor = 'yellow';
                statusDetails = 'Some tasks failing';
            } else {
                status = 'Healthy';
                statusColor = 'green';
                statusDetails = 'Minor issues detected';
            }
        }
        
        const failureDetails = failedTasks === 0 ? 'No failures' : 
            failedTasks === 1 ? '1 task failed' : 
            `${failedTasks} tasks failed`;
        
        return {
            status,
            statusColor,
            statusDetails,
            activeTasks,
            runningBackups,
            runningVerify,
            totalTasks,
            successfulTasks,
            successRate,
            recentFailures: failedTasks,
            failureDetails
        };
    }

    // Get active tasks
    function getActiveTasks(instance) {
        const tasks = [];
        const taskTypes = ['backupTasks', 'verificationTasks', 'syncTasks', 'pruneTasks'];
        
        taskTypes.forEach(taskType => {
            if (instance[taskType] && instance[taskType].recentTasks) {
                instance[taskType].recentTasks.forEach(task => {
                    if (!task.status || task.status.toLowerCase().includes('running')) {
                        tasks.push({ ...task, taskType });
                    }
                });
            }
        });
        
        return tasks.sort((a, b) => (b.startTime || 0) - (a.startTime || 0));
    }

    // Get failed tasks
    function getFailedTasks(instance) {
        const tasks = [];
        const cutoffTime = Date.now() - TIME_RANGES[state.selectedTimeRange];
        const cutoffTimeSeconds = cutoffTime / 1000;
        
        const taskTypes = ['backupTasks', 'verificationTasks', 'syncTasks', 'pruneTasks'];
        
        taskTypes.forEach(taskType => {
            if (instance[taskType] && instance[taskType].recentTasks) {
                instance[taskType].recentTasks.forEach(task => {
                    const taskTime = task.startTime || task.starttime || 0;
                    if (taskTime >= cutoffTimeSeconds && task.status && task.status !== 'OK' && !task.status.toLowerCase().includes('running')) {
                        tasks.push({ ...task, taskType });
                    }
                });
            }
        });
        
        return tasks.sort((a, b) => (b.startTime || 0) - (a.startTime || 0));
    }

    // Get all tasks
    function getAllTasks(instance, filterType = 'all') {
        const tasks = [];
        const cutoffTime = Date.now() - TIME_RANGES[state.selectedTimeRange];
        const cutoffTimeSeconds = cutoffTime / 1000;
        
        const taskTypeMap = {
            backup: ['backupTasks'],
            verify: ['verificationTasks'],
            sync: ['syncTasks'],
            prune: ['pruneTasks'],
            all: ['backupTasks', 'verificationTasks', 'syncTasks', 'pruneTasks']
        };
        
        const taskTypes = taskTypeMap[filterType] || taskTypeMap.all;
        
        taskTypes.forEach(taskType => {
            if (instance[taskType] && instance[taskType].recentTasks) {
                instance[taskType].recentTasks.forEach(task => {
                    const taskTime = task.startTime || task.starttime || 0;
                    if (taskTime >= cutoffTimeSeconds) {
                        // Apply search filter
                        if (state.searchQuery) {
                            const target = parsePbsTaskTarget(task).toLowerCase();
                            if (!target.includes(state.searchQuery)) {
                                return;
                            }
                        }
                        
                        // Apply failure filter
                        if (state.showOnlyFailures && (task.status === 'OK' || !task.status || task.status.toLowerCase().includes('running'))) {
                            return;
                        }
                        
                        tasks.push({ ...task, taskType });
                    }
                });
            }
        });
        
        return tasks.sort((a, b) => (b.startTime || 0) - (a.startTime || 0));
    }

    // Update filtered content
    function updateFilteredContent() {
        // Re-render task history with filters
        const historyContent = document.querySelector('.task-history-content');
        if (historyContent) {
            const activeTab = document.querySelector('.task-type-tab.active');
            const taskType = activeTab ? activeTab.dataset.type : 'all';
            renderTaskHistory(historyContent, taskType);
        }
        
        // Update failure section if needed
        if (state.showOnlyFailures) {
            const container = document.getElementById('pbs2-instances-container');
            if (container) {
                renderDashboard(container);
            }
        }
    }

    // Refresh dashboard
    function refreshDashboard() {
        const container = document.getElementById('pbs2-instances-container');
        if (container) {
            renderDashboard(container);
        }
    }

    // Main update function - called by socket handler
    function updatePbsInfo(pbsArray) {
        console.log('[PBS2] Received update with data:', pbsArray);
        state.pbsData = pbsArray || [];
        refreshDashboard();
    }

    // Utility functions
    function getTaskType(task) {
        if (!task.upid) return 'Unknown';
        const upidParts = task.upid.split(':');
        if (upidParts.length < 2) return 'Unknown';
        
        const typeMap = {
            'backup': 'Backup',
            'verify': 'Verify',
            'sync': 'Sync',
            'garbage_collection': 'GC',
            'prune': 'Prune'
        };
        
        return typeMap[upidParts[1]] || upidParts[1];
    }

    function getTaskTypeColor(type) {
        const colorMap = {
            'Backup': 'bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-300',
            'Verify': 'bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-300',
            'Sync': 'bg-purple-100 text-purple-800 dark:bg-purple-900/50 dark:text-purple-300',
            'GC': 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-300',
            'Prune': 'bg-orange-100 text-orange-800 dark:bg-orange-900/50 dark:text-orange-300'
        };
        
        return colorMap[type] || 'bg-gray-100 text-gray-800 dark:bg-gray-900/50 dark:text-gray-300';
    }

    function parsePbsTaskTarget(task) {
        if (!task || !task.upid) return 'Unknown';
        
        const upidParts = task.upid.split(':');
        if (upidParts.length < 8) return 'Unknown';
        
        const targetId = upidParts[2];
        
        if (targetId.startsWith('vm/')) {
            return targetId.replace('vm/', 'VM ');
        }
        
        return targetId || 'Unknown';
    }

    function getPbsStatusBadge(status) {
        if (!status) {
            return '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-300">Running</span>';
        }
        
        if (status === 'OK') {
            return '<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-300">Success</span>';
        }
        
        return `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-300">${status}</span>`;
    }

    function formatDuration(task) {
        if (!task.endTime || !task.startTime) {
            if (!task.status || task.status.toLowerCase().includes('running')) {
                // Calculate running time
                const start = task.startTime || task.starttime || 0;
                if (start) {
                    const duration = Math.floor(Date.now() / 1000 - start);
                    return formatSeconds(duration);
                }
            }
            return 'N/A';
        }
        
        const duration = task.endTime - task.startTime;
        return formatSeconds(duration);
    }

    function formatSeconds(seconds) {
        if (seconds < 60) return `${seconds}s`;
        if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
        
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        return `${hours}h ${minutes}m`;
    }

    function formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    function getUsageColor(percent) {
        if (percent >= 90) return 'text-red-600 dark:text-red-400';
        if (percent >= 75) return 'text-yellow-600 dark:text-yellow-400';
        return 'text-green-600 dark:text-green-400';
    }

    function getUsageBarColor(percent) {
        if (percent >= 90) return 'bg-red-600 dark:bg-red-400';
        if (percent >= 75) return 'bg-yellow-600 dark:bg-yellow-400';
        return 'bg-green-600 dark:bg-green-400';
    }

    // Public API
    return {
        init,
        updatePbsInfo
    };
})();