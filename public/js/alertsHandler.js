PulseApp.alerts = (() => {
    let activeAlerts = [];
    let alertHistory = [];
    let alertRules = [];
    let alertGroups = [];
    let alertMetrics = {};
    let notificationContainer = null;
    let alertsInitialized = false;
    let alertDropdown = null;

    // Configuration - More subtle and less intrusive
    const MAX_NOTIFICATIONS = 3; // Reduced from 5
    const NOTIFICATION_TIMEOUT = 5000; // Reduced from 10 seconds to 5
    const ACKNOWLEDGED_CLEANUP_DELAY = 300000; // 5 minutes
    const ALERT_COLORS = {
        'active': 'bg-red-500 border-red-600 text-white',
        'resolved': 'bg-green-500 border-green-600 text-white'
    };

    const ALERT_ICONS = {
        'active': `<svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path></svg>`,
        'resolved': `<svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"></path></svg>`
    };

    const GROUP_COLORS = {
        'critical_alerts': '#ef4444',
        'system_performance': '#f59e0b',
        'storage_alerts': '#8b5cf6',
        'availability_alerts': '#ef4444',
        'network_alerts': '#10b981',
        'custom': '#6b7280'
    };

    function init() {
        if (alertsInitialized) return;
        
        
        // Add a small delay to ensure DOM is fully ready
        setTimeout(() => {
            createNotificationContainer();
            setupHeaderIndicator();
            setupEventListeners();
            loadInitialData();
            
            // Ensure button is visible after initialization
            const indicator = document.getElementById('alerts-indicator');
            if (indicator) {
                updateHeaderIndicator(); // Initialize with current state
            } else {
                console.error('[Alerts] Failed to create alerts indicator button');
            }
        }, 100);
        
        alertsInitialized = true;
    }

    async function loadInitialData() {
        try {
            const [alertsResponse, groupsResponse] = await Promise.all([
                fetch('/api/alerts'),
                fetch('/api/alerts/groups')
            ]);
            
            if (alertsResponse.ok) {
                const alertsData = await alertsResponse.json();
                activeAlerts = alertsData.active || [];
                alertRules = alertsData.rules || [];
                alertMetrics = alertsData.stats?.metrics || {};
                updateHeaderIndicator();
            } else {
                console.error('[Alerts] Failed to fetch alerts:', alertsResponse.status);
            }
            
            if (groupsResponse.ok) {
                const groupsData = await groupsResponse.json();
                alertGroups = groupsData.groups || [];
            } else {
                console.error('[Alerts] Failed to fetch groups:', groupsResponse.status);
            }
        } catch (error) {
            console.error('[Alerts] Failed to load initial alert data:', error);
        }
    }

    function createNotificationContainer() {
        notificationContainer = document.createElement('div');
        notificationContainer.id = 'pulse-notifications';
        notificationContainer.className = 'fixed bottom-4 right-4 z-40 space-y-2 pointer-events-none';
        notificationContainer.style.maxWidth = '280px';
        document.body.appendChild(notificationContainer);
    }

    function setupHeaderIndicator() {
        const connectionStatus = document.getElementById('connection-status');
        if (connectionStatus) {
            // Remove any existing alerts indicator to avoid duplicates
            const existingIndicator = document.getElementById('alerts-indicator');
            if (existingIndicator) {
                existingIndicator.remove();
            }
            
            const alertsIndicator = document.createElement('div');
            alertsIndicator.id = 'alerts-indicator';
            alertsIndicator.className = 'text-xs px-1.5 py-0.5 rounded bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400 cursor-pointer relative flex-shrink-0 transition-colors';
            alertsIndicator.title = 'Click to manage alerts';
            alertsIndicator.textContent = '0';
            
            // Subtle styling that matches the header aesthetic
            alertsIndicator.style.minWidth = '20px';
            alertsIndicator.style.textAlign = 'center';
            alertsIndicator.style.userSelect = 'none';
            alertsIndicator.style.zIndex = '40';
            alertsIndicator.style.fontSize = '10px';
            alertsIndicator.style.lineHeight = '1';
            
            // Insert the indicator before the connection status
            connectionStatus.parentNode.insertBefore(alertsIndicator, connectionStatus);
            
            // Create dropdown as a sibling, positioned relative to the header container
            alertDropdown = document.createElement('div');
            alertDropdown.id = 'alerts-dropdown';
            alertDropdown.className = 'absolute bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg overflow-hidden hidden';
            
            // More compact positioning for the dropdown
            alertDropdown.style.position = 'fixed';
            alertDropdown.style.zIndex = '1000';
            alertDropdown.style.top = '60px';
            alertDropdown.style.right = '20px';
            alertDropdown.style.maxWidth = '320px';
            alertDropdown.style.maxHeight = '400px';
            
            // Append dropdown to body for better positioning control
            document.body.appendChild(alertDropdown);
            
        } else {
            console.error('[Alerts] connection-status element not found');
        }
    }

    function setupEventListeners() {
        let socketListenersSetup = false;
        
        // Wait for socket to be available and set up event listeners
        const setupSocketListeners = () => {
            if (window.socket && !socketListenersSetup) {
                // Set up alert event listeners
                window.socket.on('alert', handleNewAlert);
                window.socket.on('alertResolved', handleResolvedAlert);
                window.socket.on('alertAcknowledged', handleAcknowledgedAlert);
                
                // Handle reconnection - reload alert data when reconnected
                window.socket.on('connect', () => {
                    loadInitialData();
                });
                
                window.socket.on('disconnect', () => {
                });
                
                socketListenersSetup = true;
                return true;
            }
            return false;
        };
        
        // Try to setup immediately, or wait for socket
        if (!setupSocketListeners()) {
            const checkSocket = setInterval(() => {
                if (setupSocketListeners()) {
                    clearInterval(checkSocket);
                }
            }, 100);
            
            // Give up after 10 seconds
            setTimeout(() => clearInterval(checkSocket), 10000);
        }

        // Fixed click handler logic
        document.addEventListener('click', (e) => {
            const indicator = document.getElementById('alerts-indicator');
            const dropdown = document.getElementById('alerts-dropdown');
            
            if (!indicator || !dropdown) return;
            
            // Debug logging
            const clickedIndicator = indicator.contains(e.target);
            const clickedDropdown = dropdown.contains(e.target);
            
            // If clicking the indicator, open alert management modal directly
            if (clickedIndicator && !clickedDropdown) {
                e.preventDefault();
                e.stopPropagation();
                openAlertManagementModal();
                return;
            }
            
            // If clicking inside the dropdown, do nothing (let dropdown handle its own clicks)
            if (clickedDropdown) {
                return;
            }
            
            // If clicking outside both indicator and dropdown, close dropdown
            if (!clickedIndicator && !clickedDropdown && !dropdown.classList.contains('hidden')) {
                closeDropdown();
            }
        });

        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') {
                closeDropdown();
            }
        });
    }

    function openAlertManagementModal() {
        // Open the alert management modal directly on the "Alerts > Current Alerts" tab
        if (PulseApp.ui && PulseApp.ui.alertManagementModal) {
            PulseApp.ui.alertManagementModal.openModal();
        } else {
            console.error('Alert management modal not available');
        }
    }

    function toggleDropdown() {
        if (!alertDropdown) return;
        
        if (alertDropdown.classList.contains('hidden')) {
            openDropdown();
        } else {
            closeDropdown();
        }
    }

    function openDropdown() {
        if (!alertDropdown) return;
        
        // Update dropdown position based on current indicator position
        const indicator = document.getElementById('alerts-indicator');
        if (indicator) {
            const rect = indicator.getBoundingClientRect();
            alertDropdown.style.top = (rect.bottom + 8) + 'px';
            alertDropdown.style.right = (window.innerWidth - rect.right) + 'px';
        }
        
        alertDropdown.classList.remove('hidden');
        updateDropdownContent();
    }

    function closeDropdown() {
        if (!alertDropdown) return;
        
        alertDropdown.classList.add('hidden');
    }

    function updateDropdownContent() {
        if (!alertDropdown) return;

        const unacknowledgedAlerts = activeAlerts.filter(a => !a.acknowledged);
        const acknowledgedAlerts = activeAlerts.filter(a => a.acknowledged);

        // Check if acknowledged section was previously expanded
        const acknowledgedSection = alertDropdown.querySelector('.acknowledged-alerts-content');
        const wasExpanded = acknowledgedSection && !acknowledgedSection.classList.contains('hidden');
        const scrollPosition = acknowledgedSection ? acknowledgedSection.scrollTop : 0;

        if (activeAlerts.length === 0) {
            alertDropdown.innerHTML = `
                <div class="p-3 text-center text-gray-500 dark:text-gray-400">
                    <svg class="w-6 h-6 mx-auto mb-1 text-gray-300 dark:text-gray-600" fill="currentColor" viewBox="0 0 20 20">
                        ${ALERT_ICONS.active}
                    </svg>
                    <p class="text-xs mb-3">No active alerts</p>
                    <button onclick="PulseApp.alerts.hideAlertsDropdown(); if (PulseApp.ui && PulseApp.ui.alertManagementModal) { PulseApp.ui.alertManagementModal.openModal(); } else { console.error('Alert management modal not available'); }" 
                            class="w-full px-3 py-2 bg-blue-500 hover:bg-blue-600 text-white text-xs font-medium rounded transition-colors">
                        Manage Alerts
                    </button>
                </div>
            `;
            return;
        }

        let content = '';

        // Unacknowledged alerts section
        if (unacknowledgedAlerts.length > 0) {
            content += `
                <div class="border-b border-gray-200 dark:border-gray-700 p-2">
                    <div class="flex items-center justify-between">
                        <h3 class="text-xs font-medium text-gray-900 dark:text-gray-100">
                            ${unacknowledgedAlerts.length} alert${unacknowledgedAlerts.length !== 1 ? 's' : ''}
                        </h3>
                        <button onclick="PulseApp.alerts.markAllAsAcknowledged()" 
                                class="text-xs px-2 py-1 bg-green-500 text-white rounded hover:bg-green-600 focus:outline-none">
                            Ack All
                        </button>
                    </div>
                </div>
                <div class="max-h-64 overflow-y-auto">
                    ${unacknowledgedAlerts.slice(0, 8).map(alert => createCompactAlertCard(alert, false)).join('')}
                    ${unacknowledgedAlerts.length > 8 ? `
                        <div class="p-2 text-center text-xs text-gray-500 dark:text-gray-400 border-t border-gray-200 dark:border-gray-700">
                            +${unacknowledgedAlerts.length - 8} more unacknowledged
                        </div>
                    ` : ''}
                </div>
            `;
        }

        // Acknowledged alerts section (collapsed, greyed out)
        if (acknowledgedAlerts.length > 0) {
            const acknowledgedSection = `
                <div class="border-t border-gray-200 dark:border-gray-700">
                    <button onclick="PulseApp.alerts.toggleAcknowledgedSection()" 
                            class="w-full p-2 text-left text-xs text-gray-500 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700/50 flex items-center justify-between acknowledged-toggle">
                        <span>${acknowledgedAlerts.length} acknowledged alert${acknowledgedAlerts.length !== 1 ? 's' : ''}</span>
                        <svg class="w-3 h-3 transform transition-transform ${wasExpanded ? 'rotate-180' : ''}" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                        </svg>
                    </button>
                    <div class="acknowledged-alerts-content max-h-32 overflow-y-auto ${wasExpanded ? '' : 'hidden'}">
                        ${acknowledgedAlerts.map(alert => createCompactAlertCard(alert, true)).join('')}
                    </div>
                </div>
            `;
            content += acknowledgedSection;
        }

        // If only acknowledged alerts exist
        if (unacknowledgedAlerts.length === 0 && acknowledgedAlerts.length > 0) {
            content = `
                <div class="p-3 text-center text-gray-500 dark:text-gray-400">
                    <svg class="w-6 h-6 mx-auto mb-1 text-green-300 dark:text-green-600" fill="currentColor" viewBox="0 0 20 20">
                        ${ALERT_ICONS.resolved}
                    </svg>
                    <p class="text-xs">All alerts acknowledged</p>
                </div>
                ${content}
            `;
        }

        // Add Manage Alerts button to the bottom
        content += `
            <div class="border-t border-gray-200 dark:border-gray-700 p-2">
                <button onclick="PulseApp.alerts.hideAlertsDropdown(); if (PulseApp.ui && PulseApp.ui.alertManagementModal) { PulseApp.ui.alertManagementModal.openModal(); } else { console.error('Alert management modal not available'); }" 
                        class="w-full px-3 py-2 bg-blue-500 hover:bg-blue-600 text-white text-xs font-medium rounded transition-colors">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3 inline mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                    </svg>
                    Manage Alerts
                </button>
            </div>
        `;
        
        alertDropdown.innerHTML = content;

        // Restore scroll position if acknowledged section exists and was expanded
        if (wasExpanded && scrollPosition > 0) {
            const newAcknowledgedSection = alertDropdown.querySelector('.acknowledged-alerts-content');
            if (newAcknowledgedSection) {
                newAcknowledgedSection.scrollTop = scrollPosition;
            }
        }
    }

    function createCompactAlertCard(alert, acknowledged = false) {
        const alertColor = 'border-red-400';
        const alertBg = 'bg-red-50 dark:bg-red-900/10';
        
        // If acknowledged, heavily grey out the entire card
        const cardClasses = acknowledged ? 
            'border-l-2 border-gray-300 dark:border-gray-600 bg-gray-50/50 dark:bg-gray-800/30 p-2 border-b border-gray-100 dark:border-gray-700 last:border-b-0 opacity-60' :
            `border-l-2 ${severityColor} ${severityBg} p-2 border-b border-gray-100 dark:border-gray-700 last:border-b-0`;
        
        const duration = Math.round((Date.now() - alert.triggeredAt) / 1000);
        const durationStr = duration < 60 ? `${duration}s` : 
                           duration < 3600 ? `${Math.round(duration/60)}m` : 
                           `${Math.round(duration/3600)}h`;
        
        const icon = ALERT_ICONS.active;
        
        let currentValueDisplay = '';
        if (alert.metric === 'status') {
            currentValueDisplay = alert.currentValue;
        } else if (alert.metric === 'network_combined' || alert.currentValue === 'anomaly_detected') {
            currentValueDisplay = 'Network anomaly';
        } else if (typeof alert.currentValue === 'number') {
            const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(alert.metric);
            currentValueDisplay = `${Math.round(alert.currentValue)}${isPercentageMetric ? '%' : ''}`;
        } else if (typeof alert.currentValue === 'object' && alert.currentValue !== null) {
            // Handle compound threshold alerts (multiple metrics)
            const values = [];
            for (const [metric, value] of Object.entries(alert.currentValue)) {
                const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(metric);
                const formattedValue = typeof value === 'number' 
                    ? `${Math.round(value)}${isPercentageMetric ? '%' : ''}`
                    : value;
                values.push(`${metric}: ${formattedValue}`);
            }
            currentValueDisplay = values.join(', ');
        } else {
            currentValueDisplay = alert.currentValue || '';
        }
        
        // Muted text classes for acknowledged alerts
        const nameClass = acknowledged ? 'text-xs font-medium text-gray-500 dark:text-gray-500 truncate' : 'text-xs font-medium text-gray-900 dark:text-gray-100 truncate';
        const valueClass = acknowledged ? 'text-xs text-gray-400 dark:text-gray-500' : 'text-xs text-gray-500 dark:text-gray-400';
        const ruleClass = acknowledged ? 'text-xs text-gray-400 dark:text-gray-500 truncate' : 'text-xs text-gray-500 dark:text-gray-400 truncate';
        
        return `
            <div class="${cardClasses}">
                <div class="flex items-center space-x-2">
                    <div class="flex-shrink-0 ${acknowledged ? 'opacity-50' : ''}">
                        ${icon}
                    </div>
                    <div class="flex-1 min-w-0">
                        <div class="flex items-center space-x-1">
                            <span class="${nameClass}">
                                ${alert.guest.name}
                            </span>
                            <span class="${valueClass}">
                                ${currentValueDisplay}
                            </span>
                            ${acknowledged ? '<span class="text-xs bg-green-500/70 text-white px-1 rounded opacity-70">✓</span>' : ''}
                        </div>
                        <div class="${ruleClass}">
                            ${alert.ruleName} • ${durationStr}${acknowledged ? ' • acknowledged' : ''}
                        </div>
                    </div>
                    <div class="flex-shrink-0 space-x-1">
                        ${!acknowledged ? `
                            <button onclick="PulseApp.alerts.acknowledgeAlert('${alert.id}', '${alert.ruleId}');" 
                                    class="text-xs px-1 py-0.5 bg-green-500 text-white rounded hover:bg-green-600 focus:outline-none transition-all"
                                    data-alert-id="${alert.id}"
                                    title="Acknowledge alert">
                                ✓
                            </button>
                        ` : ''}
                    </div>
                </div>
            </div>
        `;
    }

    function handleNewAlert(alert) {
        
        const existingIndex = activeAlerts.findIndex(a => 
            a.ruleId === alert.ruleId && 
            a.guest.vmid === alert.guest.vmid && 
            a.guest.node === alert.guest.node
        );
        
        if (existingIndex >= 0) {
            activeAlerts[existingIndex] = alert;
        } else {
            activeAlerts.unshift(alert);
        }
        
        updateHeaderIndicator();
        
        
        if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
            updateDropdownContent();
        }
        
        document.dispatchEvent(new CustomEvent('pulseAlert', { detail: alert }));
    }

    function handleResolvedAlert(alert) {
        
        activeAlerts = activeAlerts.filter(a => 
            !(a.guest.vmid === alert.guest.vmid && 
              a.guest.node === alert.guest.node && 
              a.ruleId === alert.ruleId)
        );
        
        updateHeaderIndicator();
        
        
        if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
            updateDropdownContent();
        }
        
        document.dispatchEvent(new CustomEvent('pulseAlertResolved', { detail: alert }));
    }

    function updateHeaderIndicator() {
        const indicator = document.getElementById('alerts-indicator');
        if (!indicator) return;

        const unacknowledgedAlerts = activeAlerts.filter(a => !a.acknowledged);
        const count = unacknowledgedAlerts.length;
        
        // Always show the button, just change its appearance based on unacknowledged alert count
        let className = 'text-xs px-1.5 py-0.5 rounded bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400 cursor-pointer relative flex-shrink-0 transition-colors';
        
        if (count === 0) {
            className = 'text-xs px-1.5 py-0.5 rounded bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400 cursor-pointer relative flex-shrink-0 transition-colors';
        } else {
            className = 'text-xs px-1.5 py-0.5 rounded bg-red-400 text-white cursor-pointer relative flex-shrink-0 transition-colors';
        }
        
        indicator.className = className;
        indicator.style.minWidth = '20px';
        indicator.style.textAlign = 'center';
        indicator.style.userSelect = 'none';
        indicator.style.zIndex = '40';
        indicator.style.fontSize = '10px';
        indicator.style.lineHeight = '1';
        
        indicator.textContent = count === 0 ? '0' : `${count}`;
        indicator.title = count === 0 ? 'No active alerts' : 
                        `${count} unacknowledged alert${count !== 1 ? 's' : ''} - Click to view`;
    }

    function showNotification(alert, type = 'info') {
        const notification = document.createElement('div');
        notification.className = `pointer-events-auto transform transition-all duration-200 ease-out opacity-0 translate-y-2 scale-95`;
        
        // More subtle styling with muted colors
        const colorClasses = {
            'critical': 'bg-red-50 border border-red-200 text-red-700 dark:bg-red-900/20 dark:border-red-700 dark:text-red-200',
            'warning': 'bg-yellow-50 border border-yellow-200 text-yellow-700 dark:bg-yellow-900/20 dark:border-yellow-700 dark:text-yellow-200',
            'info': 'bg-blue-50 border border-blue-200 text-blue-700 dark:bg-blue-900/20 dark:border-blue-700 dark:text-blue-200',
            'resolved': 'bg-green-50 border border-green-200 text-green-700 dark:bg-green-900/20 dark:border-green-700 dark:text-green-200'
        };
        
        const colorClass = colorClasses[type] || colorClasses.info;
        const icon = ALERT_ICONS.active;
        
        const title = type === 'resolved' ? 'Resolved' : 
                     type === 'info' ? (alert.message && alert.message.includes('acknowledged') ? 'Success' : 'Alert') :
                     'Alert';
        
        const message = alert.message || `${alert.guest?.name || 'Unknown'}`;
        
        notification.innerHTML = `
            <div class="w-64 ${colorClass} shadow-sm rounded-lg pointer-events-auto overflow-hidden backdrop-blur-sm">
                <div class="p-2">
                    <div class="flex items-center gap-2">
                        <div class="flex-shrink-0">
                            ${icon}
                        </div>
                        <div class="flex-1 min-w-0">
                            <p class="text-xs font-medium leading-tight">${title}</p>
                            <p class="text-xs opacity-80 leading-tight truncate">${message}</p>
                        </div>
                        <div class="flex-shrink-0">
                            <button class="inline-flex text-current hover:opacity-70 focus:outline-none transition-opacity p-0.5" onclick="this.closest('.pointer-events-auto').remove()">
                                <svg class="h-3 w-3" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                                </svg>
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        `;

        notificationContainer.appendChild(notification);

        requestAnimationFrame(() => {
            notification.className = notification.className.replace('opacity-0 translate-y-2 scale-95', 'opacity-100 translate-y-0 scale-100');
        });

        // Smarter timing: resolved alerts and acknowledgments disappear faster
        const isLowPriority = type === 'resolved' || 
                             (alert.message && alert.message.includes('acknowledged')) ||
                             (alert.message && alert.message.includes('suppressed'));
        
        const timeoutDuration = isLowPriority ? 2500 : NOTIFICATION_TIMEOUT;

        setTimeout(() => {
            if (notification.parentNode) {
                notification.className = notification.className.replace('opacity-100 translate-y-0 scale-100', 'opacity-0 translate-y-2 scale-95');
                setTimeout(() => {
                    if (notification.parentNode) {
                        notification.remove();
                    }
                }, 200);
            }
        }, timeoutDuration);

        while (notificationContainer.children.length > MAX_NOTIFICATIONS) {
            notificationContainer.removeChild(notificationContainer.firstChild);
        }
    }

    // Track alerts currently being acknowledged to prevent duplicate requests
    const acknowledgeInProgress = new Set();
    
    async function acknowledgeAlert(alertId, ruleId) {
        // Prevent duplicate acknowledgements
        if (acknowledgeInProgress.has(alertId)) {
            return;
        }
        
        acknowledgeInProgress.add(alertId);
        
        try {
            // Update button immediately to show loading state
            const buttons = document.querySelectorAll(`button[data-alert-id="${alertId}"]`);
            buttons.forEach(btn => {
                btn.disabled = true;
                btn.classList.add('opacity-50', 'cursor-not-allowed');
                btn.innerHTML = '<span class="inline-block animate-spin">⟳</span>';
            });
            
            const response = await fetch(`/api/alerts/${alertId}/acknowledge`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ userId: 'web-user', note: 'Acknowledged via web interface' })
            });
            
            
            if (response.ok) {
                const result = await response.json().catch(() => ({}));
                
                // Find and update the alert in local array
                const alertIndex = activeAlerts.findIndex(a => a.id === alertId);
                if (alertIndex >= 0) {
                    activeAlerts[alertIndex].acknowledged = true;
                    activeAlerts[alertIndex].acknowledgedAt = Date.now();
                    
                    // Schedule cleanup of this acknowledged alert after 5 minutes
                    scheduleAcknowledgedCleanup(alertId);
                    
                    updateHeaderIndicator();
                    if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
                        updateDropdownContent();
                    }
                }
            } else {
                const errorText = await response.text().catch(() => 'Unknown error');
                console.error('[Alerts] Acknowledge failed with status:', response.status, errorText);
                throw new Error(`Server responded with status ${response.status}: ${errorText}`);
            }
        } catch (error) {
            console.error('[Alerts] Failed to acknowledge alert:', error);
            // Show user feedback for acknowledgment failures
            showToastNotification(`Failed to acknowledge alert: ${error.message}`, 'error');
            
            // Restore button state on error
            const buttons = document.querySelectorAll(`button[data-alert-id="${alertId}"]`);
            buttons.forEach(btn => {
                btn.disabled = false;
                btn.classList.remove('opacity-50', 'cursor-not-allowed');
                btn.innerHTML = '✓';
            });
        } finally {
            acknowledgeInProgress.delete(alertId);
        }
    }

    // Track cleanup timeouts to prevent memory leaks
    const cleanupTimeouts = new Map();
    
    function scheduleAcknowledgedCleanup(alertId) {
        // Clear any existing timeout for this alert
        if (cleanupTimeouts.has(alertId)) {
            clearTimeout(cleanupTimeouts.get(alertId));
        }
        
        const timeoutId = setTimeout(() => {
            activeAlerts = activeAlerts.filter(a => a.id !== alertId);
            updateHeaderIndicator();
            if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
                updateDropdownContent();
            }
            cleanupTimeouts.delete(alertId);
        }, ACKNOWLEDGED_CLEANUP_DELAY);
        
        cleanupTimeouts.set(alertId, timeoutId);
    }

    function toggleAcknowledgedSection() {
        if (!alertDropdown) return;
        
        const content = alertDropdown.querySelector('.acknowledged-alerts-content');
        const arrow = alertDropdown.querySelector('.acknowledged-toggle svg');
        
        if (content && arrow) {
            const isHidden = content.classList.contains('hidden');
            
            if (isHidden) {
                content.classList.remove('hidden');
                arrow.classList.add('rotate-180');
            } else {
                content.classList.add('hidden');
                arrow.classList.remove('rotate-180');
            }
            
        }
    }

    async function suppressAlert(ruleId, node, vmid) {
        try {
            
            const response = await fetch('/api/alerts/suppress', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    ruleId,
                    guestFilter: { node, vmid },
                    duration: 3600000, // 1 hour
                    reason: 'Suppressed via web interface'
                })
            });
            
            
            if (response.ok) {
                const result = await response.json().catch(() => ({}));
                await loadInitialData();
                if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
                    updateDropdownContent();
                }
            } else {
                const errorText = await response.text().catch(() => 'Unknown error');
                console.error('[Alerts] Suppress failed with status:', response.status, errorText);
                throw new Error(`Server responded with status ${response.status}: ${errorText}`);
            }
        } catch (error) {
            console.error('[Alerts] Failed to suppress alert:', error);
            showToastNotification(`Failed to suppress alert: ${error.message}`, 'error');
        }
    }

    async function markAllAsAcknowledged() {
        
        
        const unacknowledgedAlerts = activeAlerts.filter(alert => !alert.acknowledged);
        
        
        if (unacknowledgedAlerts.length === 0) {
            // Don't show annoying "no alerts" popup - user can see this visually
            return;
        }
        
        
        let successCount = 0;
        let errorCount = 0;
        
        for (const alert of unacknowledgedAlerts) {
            try {
                
                const response = await fetch(`/api/alerts/${alert.id}/acknowledge`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ 
                        userId: 'bulk-operation', 
                        note: 'Bulk acknowledged via dropdown' 
                    })
                });
                
                
                if (response.ok) {
                    successCount++;
                    // Update local alert
                    const alertIndex = activeAlerts.findIndex(a => a.id === alert.id);
                    if (alertIndex >= 0) {
                        activeAlerts[alertIndex].acknowledged = true;
                        activeAlerts[alertIndex].acknowledgedAt = Date.now();
                        
                        // Schedule cleanup of this acknowledged alert after 5 minutes
                        scheduleAcknowledgedCleanup(alert.id);
                        
                    }
                } else {
                    errorCount++;
                    const errorText = await response.text().catch(() => 'Unknown error');
                    console.error(`[Alerts] Failed to acknowledge alert ${alert.id}:`, response.status, errorText);
                }
            } catch (error) {
                errorCount++;
                console.error(`[Alerts] Failed to acknowledge alert ${alert.id}:`, error);
            }
        }
        
        
        // Update UI
        updateHeaderIndicator();
        if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
            updateDropdownContent();
        }
        
        // Only show notifications for errors - success is visual (alerts disappear/change)
        if (errorCount > 0) {
            // showNotification({ 
            //     message: `${errorCount} alerts failed to acknowledge` 
            // }, 'warning');
        }
        // No "success" notification needed - user can see alerts are acknowledged
    }

    function updateAlertsFromState(state) {
        if (state && state.alerts) {
            if (state.alerts.active) {
                // Preserve local acknowledgment state to prevent race conditions
                const newAlerts = state.alerts.active.map(serverAlert => {
                    const localAlert = activeAlerts.find(local => local.id === serverAlert.id);
                    if (localAlert && localAlert.acknowledged && !serverAlert.acknowledged) {
                        // Keep local acknowledgment if server hasn't caught up yet
                        return { ...serverAlert, acknowledged: true, acknowledgedAt: localAlert.acknowledgedAt };
                    }
                    return serverAlert;
                });
                
                activeAlerts = newAlerts;
                updateHeaderIndicator();
                if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
                    updateDropdownContent();
                }
            }
        }
    }
    
    // Additional socket event handlers
    
    function handleAcknowledgedAlert(alert) {
        
        // Update existing alert with server data
        const existingIndex = activeAlerts.findIndex(a => a.id === alert.id);
        if (existingIndex >= 0) {
            activeAlerts[existingIndex] = { 
                ...activeAlerts[existingIndex], 
                acknowledged: true, 
                acknowledgedAt: alert.acknowledgedAt || Date.now(),
                acknowledgedBy: alert.acknowledgedBy
            };
            updateHeaderIndicator();
            if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
                updateDropdownContent();
            }
        }
    }
    
    // Cleanup function to prevent memory leaks
    function cleanup() {
        // Clear all cleanup timeouts
        for (const timeoutId of cleanupTimeouts.values()) {
            clearTimeout(timeoutId);
        }
        cleanupTimeouts.clear();
        
        // Remove socket listeners if needed
        if (window.socket) {
            window.socket.off('alert', handleNewAlert);
            window.socket.off('alertResolved', handleResolvedAlert);
            window.socket.off('alertAcknowledged', handleAcknowledgedAlert);
        }
    }

    // Helper function for toast notifications
    function showToastNotification(message, type = 'info') {
        if (window.PulseApp && window.PulseApp.ui && window.PulseApp.ui.toastNotifications) {
            window.PulseApp.ui.toastNotifications.show(message, type);
        } else {
            // Fallback to basic notification
            showNotification({ message }, type);
        }
    }

    // Public API
    return {
        init,
        showNotification,
        showAlertsDropdown: openDropdown,
        hideAlertsDropdown: closeDropdown,
        updateAlertsFromState,
        acknowledgeAlert,
        suppressAlert,
        markAllAsAcknowledged,
        toggleAcknowledgedSection,
        getActiveAlerts: () => activeAlerts,
        getAlertHistory: () => alertHistory,
        cleanup
    };
})();

// Auto-initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', PulseApp.alerts.init);
} else {
    PulseApp.alerts.init();
}