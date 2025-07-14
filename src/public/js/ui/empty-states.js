// Empty state UI components
PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.emptyStates = (() => {
    
    function createEmptyState(type, context = {}) {
        const emptyStates = {
            'no-guests': {
                icon: `<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="mx-auto mb-4 text-gray-300 dark:text-gray-600">
                    <rect width="20" height="14" x="2" y="5" rx="2" ry="2"/>
                    <line x1="2" y1="10" x2="22" y2="10"/>
                </svg>`,
                title: 'No Virtual Machines or Containers',
                message: 'No VMs or containers are currently configured on this node.',
                actions: []
            },
            'no-results': {
                icon: `<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="mx-auto mb-4 text-gray-300 dark:text-gray-600">
                    <circle cx="11" cy="11" r="8"/>
                    <path d="m21 21-4.35-4.35"/>
                    <line x1="11" y1="8" x2="11" y2="14"/>
                    <line x1="8" y1="11" x2="14" y2="11"/>
                </svg>`,
                title: 'No Matching Results',
                message: _buildFilterMessage(context),
                actions: [{
                    text: 'Clear Filters',
                    onclick: 'PulseApp.ui.common.resetDashboardView()'
                }]
            },
            'no-storage': {
                icon: `<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="mx-auto mb-4 text-gray-300 dark:text-gray-600">
                    <ellipse cx="12" cy="5" rx="9" ry="3"/>
                    <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
                    <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
                </svg>`,
                title: 'No Storage Configured',
                message: 'No storage repositories are configured on this system.',
                actions: []
            },
            'no-pbs': {
                icon: `<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="mx-auto mb-4 text-gray-300 dark:text-gray-600">
                    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
                    <line x1="12" y1="8" x2="12" y2="12"/>
                    <line x1="12" y1="16" x2="12.01" y2="16"/>
                </svg>`,
                title: 'No Remote Backup Servers',
                message: 'No remote backup servers (PBS) are configured or available.',
                actions: []
            },
            'loading': {
                icon: `<div class="mx-auto mb-4">
                    <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500"></div>
                </div>`,
                title: 'Loading...',
                message: 'Fetching data from the server.',
                actions: []
            },
            'error': {
                icon: `<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="mx-auto mb-4 text-red-400 dark:text-red-600">
                    <circle cx="12" cy="12" r="10"/>
                    <line x1="12" y1="8" x2="12" y2="12"/>
                    <line x1="12" y1="16" x2="12.01" y2="16"/>
                </svg>`,
                title: 'Error Loading Data',
                message: context.error || 'Failed to load data. Please try again.',
                actions: [{
                    text: 'Retry',
                    onclick: 'location.reload()'
                }]
            },
            'config-required': {
                icon: `<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="mx-auto mb-4 text-yellow-500 dark:text-yellow-400">
                    <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
                    <line x1="12" y1="9" x2="12" y2="13"/>
                    <line x1="12" y1="17" x2="12.01" y2="17"/>
                </svg>`,
                title: 'Configuration Required',
                message: 'Pulse needs to be configured with your Proxmox credentials before it can start monitoring.',
                actions: [{
                    text: 'Configure Now',
                    onclick: 'window.location.href="/"'
                }]
            }
        };

        const state = emptyStates[type] || emptyStates['no-results'];
        
        return `
            <div class="flex flex-col items-center justify-center py-12 px-4">
                ${state.icon}
                <h3 class="text-lg font-medium text-gray-700 dark:text-gray-300 mb-2">${state.title}</h3>
                <p class="text-sm text-gray-700 dark:text-gray-300 text-center max-w-md mb-6">${state.message}</p>
                ${state.actions.length > 0 ? `
                    <div class="flex gap-3">
                        ${state.actions.map(action => `
                            <button onclick="${action.onclick}" class="px-4 py-2 bg-blue-500 text-white text-sm rounded-md hover:bg-blue-600 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:focus:ring-offset-gray-800 transition-colors">
                                ${action.text}
                            </button>
                        `).join('')}
                    </div>
                ` : ''}
            </div>
        `;
    }

    function _buildFilterMessage(context) {
        const filters = [];
        
        if (context.filterType && context.filterType !== 'all') {
            filters.push(`Type: ${context.filterType.toUpperCase()}`);
        }
        
        if (context.filterStatus && context.filterStatus !== 'all') {
            filters.push(`Status: ${context.filterStatus}`);
        }
        
        if (context.searchTerms && context.searchTerms.length > 0) {
            filters.push(`Search: "${context.searchTerms.join(', ')}"`);
        }
        
        if (context.thresholds && context.thresholds.length > 0) {
            filters.push(`Thresholds: ${context.thresholds.join(', ')}`);
        }
        
        if (filters.length === 0) {
            return 'No items match your current view.';
        }
        
        return `No items found matching: ${filters.join(' • ')}`;
    }

    function createTableEmptyState(type, context, colspan) {
        const emptyStateHtml = createEmptyState(type, context);
        return `<tr><td colspan="${colspan}" class="p-0">${emptyStateHtml}</td></tr>`;
    }

    return {
        createEmptyState,
        createTableEmptyState
    };
})();