PulseApp.alerts = (() => {
    let activeAlerts = [];
    let alertHistory = [];
    let alertRules = [];
    let alertGroups = [];
    let alertMetrics = {};
    let notificationContainer = null;
    let alertsInitialized = false;
    let alertsEnabled = true; // Track if alerts are enabled
    let alertDropdown = null;
    let dropdownUpdateTimeout = null;
    let alertStormMode = false;
    let alertsReceivedTimestamps = [];
    let toastRateLimitCount = 0;
    let lastToastTime = 0;
    let pendingAlertToasts = [];
    let pendingResolvedToasts = [];
    let resolvedToastTimeout = null;
    let timestampUpdateInterval = null;
    let serverTimeOffset = 0; // Offset between server and client clocks

    // Configuration - More subtle and less intrusive
    const MAX_NOTIFICATIONS = 3; // Reduced from 5
    const NOTIFICATION_TIMEOUT = 5000; // Reduced from 10 seconds to 5
    const ACKNOWLEDGED_CLEANUP_DELAY = 300000; // 5 minutes
    const MAX_ACTIVE_ALERTS = 100; // Prevent memory exhaustion
    const ALERT_STORM_THRESHOLD = 10; // Alerts per second to trigger storm mode
    const ALERT_COLORS = {
        'active': 'bg-amber-500 border-amber-600 text-white',
        'resolved': 'bg-green-500 border-green-600 text-white'
    };

    const ALERT_ICONS = {
        'active': `<svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path></svg>`,
        'resolved': `<svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"></path></svg>`
    };

    const GROUP_COLORS = {
        'critical_alerts': '#f59e0b',
        'system_performance': '#f59e0b',
        'storage_alerts': '#8b5cf6',
        'availability_alerts': '#f59e0b',
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
            
            // Check if alerts are enabled and hide/show indicator accordingly
            checkAlertsEnabled().then(enabled => {
                alertsEnabled = enabled;
                const indicator = document.getElementById('alerts-indicator');
                if (indicator) {
                    indicator.style.display = alertsEnabled ? '' : 'none';
                    updateHeaderIndicator(); // Initialize with current state
                }
            });
        }, 100);
        
        alertsInitialized = true;
    }

    async function loadInitialData() {
        try {
            alertsInitialized = true; // Set this early so we can show notifications for initial alerts
            // First, calculate server time offset
            const startTime = Date.now();
            const [alertsResponse, groupsResponse] = await Promise.all([
                fetch('/api/alerts'),
                fetch('/api/alerts/groups')
            ]);
            
            // Use the server's response time to estimate clock offset
            if (alertsResponse.headers.get('date')) {
                const serverTime = new Date(alertsResponse.headers.get('date')).getTime();
                const clientTime = Date.now();
                const requestDuration = clientTime - startTime;
                // Estimate server time at moment of request (compensate for network delay)
                const estimatedServerTime = serverTime - (requestDuration / 2);
                serverTimeOffset = estimatedServerTime - startTime;
            }
            
            if (alertsResponse.ok) {
                const alertsData = await alertsResponse.json();
                
                // Active alerts are ONLY what the server says is active
                activeAlerts = alertsData.active || [];
                alertRules = alertsData.rules || [];
                alertMetrics = alertsData.stats?.metrics || {};
                
                // Load persisted history from server
                if (alertsData.history) {
                    alertHistory = alertsData.history;
                }
                
                // Merge active alerts into history (in case some are already acknowledged)
                if (Array.isArray(activeAlerts)) {
                    activeAlerts.forEach(alert => {
                        if (!alert) return;
                        const existingIndex = alertHistory.findIndex(h => h && h.id === alert.id);
                        if (existingIndex >= 0) {
                            // Update existing entry with latest data
                            alertHistory[existingIndex] = alert;
                        } else {
                            // Add new alert to history
                            alertHistory.unshift(alert);
                        }
                    });
                }
                
                // Sort history by triggeredAt (newest first)
                alertHistory.sort((a, b) => (b.triggeredAt || 0) - (a.triggeredAt || 0));
                
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
        // Remove any existing container first
        const existing = document.getElementById('pulse-notifications');
        if (existing) {
            existing.remove();
        }
        
        notificationContainer = document.createElement('div');
        notificationContainer.id = 'pulse-notifications';
        notificationContainer.className = 'fixed bottom-4 right-4 z-50 space-y-2';
        notificationContainer.style.cssText = 'position: fixed; bottom: 1rem; right: 1rem; z-index: 9999; max-width: 280px; pointer-events: none;';
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
            alertDropdown.className = 'absolute bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg hidden';
            
            // More compact positioning for the dropdown
            alertDropdown.style.position = 'fixed';
            alertDropdown.style.zIndex = '1000';
            alertDropdown.style.top = '60px';
            
            // Mobile-responsive positioning
            if (window.innerWidth < 640) {
                // On mobile, use full width with small margins
                alertDropdown.style.left = '10px';
                alertDropdown.style.right = '10px';
                alertDropdown.style.width = 'auto';
                alertDropdown.style.maxWidth = 'calc(100vw - 20px)';
            } else {
                // On desktop, position from right
                alertDropdown.style.right = '20px';
                alertDropdown.style.width = '420px';
            }
            
            alertDropdown.style.maxHeight = '80vh'; // Use viewport height for better mobile support
            alertDropdown.style.overflowY = 'auto';
            alertDropdown.style.overflowX = 'hidden'; // Prevent horizontal scroll
            
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
                window.socket.on('alertRulesRefreshed', handleRulesRefreshed);
                
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

        // Fixed click handler logic - only handle alerts-specific clicks
        document.addEventListener('click', (e) => {
            const indicator = document.getElementById('alerts-indicator');
            const dropdown = document.getElementById('alerts-dropdown');
            
            if (!indicator || !dropdown) return;
            
            // Only handle clicks related to alerts - ignore PBS tab interactions, chart interactions, and select dropdowns
            if (e.target.closest('.pbs-tab, .pbs-content, .pbs-section, .mini-chart, .chart-overlay, select, .alert-threshold-input')) {
                return;
            }
            
            // Debug logging
            const clickedIndicator = indicator.contains(e.target);
            const clickedDropdown = dropdown.contains(e.target);
            
            // If clicking the indicator, toggle the dropdown
            if (clickedIndicator && !clickedDropdown) {
                e.preventDefault();
                e.stopPropagation();
                toggleDropdown();
                return;
            }
            
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
        
        // Handle window resize to update dropdown position
        let resizeTimeout;
        window.addEventListener('resize', () => {
            clearTimeout(resizeTimeout);
            resizeTimeout = setTimeout(() => {
                // If dropdown is open, update its position
                if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
                    openDropdown(); // This will recalculate position based on new window size
                }
            }, 100);
        });
    }


    function toggleDropdown() {
        if (!alertDropdown) return;
        
        if (alertDropdown.classList.contains('hidden')) {
            openDropdown();
        } else {
            closeDropdown();
        }
    }
    
    // Process queued resolved toast notifications
    function processResolvedToastQueue() {
        if (pendingResolvedToasts.length === 0) return;
        
        // Group all pending resolved alerts
        const resolvedAlerts = [...pendingResolvedToasts];
        pendingResolvedToasts = [];
        
        if (resolvedAlerts.length === 1) {
            // Single resolved alert
            const alert = resolvedAlerts[0];
            showNotification({
                message: alert.message,
                guest: { name: alert.guest }
            }, 'resolved');
        } else {
            // Multiple resolved alerts - show summary
            const guestNames = [...new Set(resolvedAlerts.map(a => a.guest))];
            if (guestNames.length <= 3) {
                showNotification({
                    message: `Resolved: ${guestNames.join(', ')}`
                }, 'resolved');
            } else {
                showNotification({
                    message: `${resolvedAlerts.length} alerts resolved`
                }, 'resolved');
            }
        }
    }
    
    // Process queued toast notifications
    let toastProcessingTimeout = null;
    function processToastQueue() {
        // Don't process during alert storm
        if (alertStormMode) {
            pendingAlertToasts = []; // Clear queue during storm
            return;
        }
        
        const now = Date.now();
        const timeSinceLastToast = now - lastToastTime;
        
        // If enough time has passed, show next toast
        if (timeSinceLastToast >= 500 && pendingAlertToasts.length > 0) { // 500ms between toasts
            // Group alerts that arrived close together
            const recentAlerts = [];
            const cutoffTime = now - 1000; // Group alerts from last second
            
            while (pendingAlertToasts.length > 0 && pendingAlertToasts[0].timestamp >= cutoffTime) {
                recentAlerts.push(pendingAlertToasts.shift());
            }
            
            if (recentAlerts.length === 1) {
                // Single alert
                const alert = recentAlerts[0];
                showNotification({
                    message: `Alert: ${alert.guest}`,
                    guest: { name: alert.guest }
                }, 'alert');
            } else if (recentAlerts.length > 1) {
                // Multiple alerts - show summary
                const guestNames = [...new Set(recentAlerts.map(a => a.guest))];
                if (guestNames.length <= 3) {
                    showNotification({
                        message: `Alerts: ${guestNames.join(', ')}`
                    }, 'alert');
                } else {
                    showNotification({
                        message: `${recentAlerts.length} alerts from ${guestNames.length} guests`
                    }, 'alert');
                }
            }
            
            lastToastTime = now;
        }
        
        // Schedule next processing if more alerts pending
        if (pendingAlertToasts.length > 0 && !toastProcessingTimeout) {
            toastProcessingTimeout = setTimeout(() => {
                toastProcessingTimeout = null;
                processToastQueue();
            }, 500);
        }
    }

    function openDropdown() {
        if (!alertDropdown) return;
        
        // Update dropdown position based on current indicator position
        const indicator = document.getElementById('alerts-indicator');
        if (indicator) {
            const rect = indicator.getBoundingClientRect();
            alertDropdown.style.top = (rect.bottom + 8) + 'px';
            
            // Mobile-responsive positioning
            if (window.innerWidth < 640) {
                // On mobile, use full width with small margins
                alertDropdown.style.left = '10px';
                alertDropdown.style.right = '10px';
                alertDropdown.style.width = 'auto';
                alertDropdown.style.maxWidth = 'calc(100vw - 20px)';
            } else {
                // On desktop, position from right
                alertDropdown.style.left = 'auto';
                alertDropdown.style.right = (window.innerWidth - rect.right) + 'px';
                alertDropdown.style.width = '420px';
                alertDropdown.style.maxWidth = '';
            }
        }
        
        alertDropdown.classList.remove('hidden');
        updateDropdownContent();
        
        // Start live timestamp updates
        startTimestampUpdates();
    }

    function closeDropdown() {
        if (!alertDropdown) return;
        
        alertDropdown.classList.add('hidden');
        
        // Stop timestamp updates when dropdown is closed
        stopTimestampUpdates();
    }

    function updateDropdownContent() {
        if (!alertDropdown) return;
        
        try {
            // During alert storm, skip updates if too frequent
            if (alertStormMode && dropdownUpdateTimeout) {
                return;
            }

            const now = Date.now() + serverTimeOffset;
        
        // Combine active alerts and history
        const allAlerts = [];
        
        // Add active alerts from server (these are the only truly active ones)
        if (Array.isArray(activeAlerts)) {
            activeAlerts.forEach(alert => {
                if (alert) {
                    allAlerts.push({...alert, isActive: true});
                }
            });
        }
        
        // Add alerts from history that are actually resolved
        if (Array.isArray(alertHistory)) {
            alertHistory.forEach(alert => {
                if (!alert) return;
                // Skip if this alert is already in activeAlerts
                if (activeAlerts.find(a => a && a.id === alert.id)) {
                    return;
                }
            
                // Only include resolved alerts from history
                if (alert.resolved) {
                    allAlerts.push({...alert, isActive: false});
                }
            });
        }
        
        // Sort by triggeredAt timestamp (newest first)
        allAlerts.sort((a, b) => (b.triggeredAt || 0) - (a.triggeredAt || 0));
        
        if (allAlerts.length === 0) {
            alertDropdown.innerHTML = `
                <div class="p-3 text-center text-gray-500 dark:text-gray-400">
                    <svg class="w-6 h-6 mx-auto mb-1 text-gray-300 dark:text-gray-600" fill="currentColor" viewBox="0 0 20 20">
                        ${ALERT_ICONS.active}
                    </svg>
                    <p class="text-xs mb-3">No alerts</p>
                </div>
            `;
            return;
        }

        // Group alerts by time periods
        const timeGroups = {
            recent: [],      // Last 5 minutes
            lastHour: [],    // 5 mins - 1 hour
            lastDay: [],     // 1 hour - 24 hours
            older: []        // Older than 24 hours
        };
        
        allAlerts.forEach(alert => {
            // If no triggeredAt, put in recent (these are likely new alerts)
            if (!alert.triggeredAt) {
                timeGroups.recent.push(alert);
                return;
            }
            
            const age = now - alert.triggeredAt;
            if (age < 5 * 60 * 1000) {
                timeGroups.recent.push(alert);
            } else if (age < 60 * 60 * 1000) {
                timeGroups.lastHour.push(alert);
            } else if (age < 24 * 60 * 60 * 1000) {
                timeGroups.lastDay.push(alert);
            } else {
                timeGroups.older.push(alert);
            }
        });

        let content = '';
        
        // Unacknowledged alerts header
        const unacknowledgedAlerts = activeAlerts.filter(a => !a.acknowledged && !a.resolved);
        if (unacknowledgedAlerts.length > 0) {
            content += `
                <div class="border-b border-gray-200 dark:border-gray-700 p-2">
                    <div class="flex items-center justify-between">
                        <h3 class="text-xs font-medium text-gray-900 dark:text-gray-100">
                            ${unacknowledgedAlerts.length} active alert${unacknowledgedAlerts.length !== 1 ? 's' : ''}
                        </h3>
                        <button onclick="event.stopPropagation(); PulseApp.alerts.markAllAsAcknowledged()" 
                                class="text-xs px-2 py-1 bg-green-500 text-white rounded hover:bg-green-600 focus:outline-none">
                            Ack All
                        </button>
                    </div>
                </div>
            `;
        }
        
        content += '<div class="max-h-96 overflow-y-auto">';
        
        // Helper function to create time group section
        const createTimeGroup = (title, alerts) => {
            if (alerts.length === 0) return '';
            
            return `
                <div class="border-b border-gray-100 dark:border-gray-700 last:border-b-0">
                    <div class="px-2 py-1 bg-gray-50 dark:bg-gray-700/50 text-xs text-gray-500 dark:text-gray-400 font-medium sticky top-0 z-10">
                        ${title}
                    </div>
                    ${alerts.map(alert => createCompactAlertCard(alert, alert.acknowledged || alert.resolved)).join('')}
                </div>
            `;
        };
        
        // Add time groups
        content += createTimeGroup('Recent', timeGroups.recent);
        content += createTimeGroup('Last Hour', timeGroups.lastHour);
        content += createTimeGroup('Last 24 Hours', timeGroups.lastDay);
        content += createTimeGroup('Older', timeGroups.older);
        
        content += '</div>';
        
        // Preserve scroll position when updating content
        const scrollContainer = alertDropdown.querySelector('.max-h-96.overflow-y-auto');
        const currentScrollTop = scrollContainer ? scrollContainer.scrollTop : 0;
        
        alertDropdown.innerHTML = content;
        
        // Restore scroll position after updating
        const newScrollContainer = alertDropdown.querySelector('.max-h-96.overflow-y-auto');
        if (newScrollContainer && currentScrollTop > 0) {
            newScrollContainer.scrollTop = currentScrollTop;
        }
    } catch (error) {
        console.error('[Alerts] Error updating dropdown content:', error);
        // Attempt to show a basic error message in the dropdown
        if (alertDropdown) {
            alertDropdown.innerHTML = `
                <div class="p-4 text-sm text-red-600 dark:text-red-400">
                    Error displaying alerts. Please refresh the page.
                </div>
            `;
        }
    }
    }

    function createCompactAlertCard(alert, acknowledged = false) {
        // Add safety checks for malformed alert objects
        if (!alert || !alert.guest) {
            console.error('[Alerts] Invalid alert object:', alert);
            return '';
        }
        
        const isResolved = alert.resolved || false;
        const isAcknowledged = acknowledged || alert.acknowledged || false;
        const isPending = alert.state === 'pending';
        
        // Visual hierarchy:
        // 1. Active unacknowledged: Full color (amber) - demands attention
        // 2. Pending: Muted amber with progress - waiting for sustained threshold
        // 3. Acknowledged: Greyed out - seen but still active
        // 4. Resolved: Greyed out - no longer active
        let alertColor, alertBg, cardClasses;
        
        if (isResolved) {
            // Resolved alerts - muted appearance
            alertColor = 'border-gray-300 dark:border-gray-500';
            alertBg = 'bg-gray-50/50 dark:bg-transparent';
            cardClasses = `relative border-l-2 ${alertColor} ${alertBg} p-2 border-b border-gray-200 dark:border-gray-700 last:border-b-0`;
        } else if (isAcknowledged) {
            // Acknowledged but not resolved - grey out like resolved, but with a subtle green hint
            alertColor = 'border-gray-300 dark:border-gray-500';
            alertBg = 'bg-gray-50/50 dark:bg-transparent';
            cardClasses = `border-l-2 ${alertColor} ${alertBg} p-2 border-b border-gray-200 dark:border-gray-700 last:border-b-0`;
        } else if (isPending) {
            // Pending alerts - muted amber with progress indicator
            alertColor = 'border-amber-400/50 dark:border-amber-500/50';
            alertBg = 'bg-amber-50/50 dark:bg-amber-900/10';
            cardClasses = `border-l-4 ${alertColor} ${alertBg} p-2 border-b border-gray-100 dark:border-gray-700 last:border-b-0`;
        } else {
            // Active unacknowledged alerts - full color and prominent
            alertColor = 'border-amber-500 dark:border-amber-400';
            alertBg = 'bg-amber-50 dark:bg-amber-900/20';
            cardClasses = `border-l-4 ${alertColor} ${alertBg} p-2 border-b border-gray-100 dark:border-gray-700 last:border-b-0`;
        }
        
        // Use triggeredAt, or fall back to acknowledgedAt for ack alerts, or current time as last resort
        const triggeredAt = alert.triggeredAt || alert.acknowledgedAt || (Date.now() + serverTimeOffset);
        // For resolved alerts, use the actual duration. For active alerts, calculate from now
        const duration = isResolved && alert.resolvedAt ? 
            Math.max(0, Math.round((alert.resolvedAt - triggeredAt) / 1000)) :
            Math.max(0, Math.round(((Date.now() + serverTimeOffset) - triggeredAt) / 1000));
        const durationStr = formatDuration(duration);
        
        const icon = isResolved ? ALERT_ICONS.resolved : ALERT_ICONS.active;
        
        let currentValueDisplay = '';
        try {
            if (alert.metric === 'status') {
                currentValueDisplay = alert.currentValue || '';
            } else if (alert.metric === 'network_combined' || alert.currentValue === 'anomaly_detected') {
                currentValueDisplay = 'Network anomaly';
            } else if (typeof alert.currentValue === 'number') {
                const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(alert.metric);
                const isIOMetric = ['diskread', 'diskwrite', 'netin', 'netout'].includes(alert.metric);
                
                if (isIOMetric) {
                    // Format I/O metrics properly
                    currentValueDisplay = formatMetricValue(alert.currentValue, alert.metric);
                } else if (isPercentageMetric) {
                    // Show decimal for values < 1%
                    const displayValue = alert.currentValue < 1 ? alert.currentValue.toFixed(1) : Math.round(alert.currentValue);
                    currentValueDisplay = `${displayValue}%`;
                } else {
                    // Other numeric values
                    const displayValue = alert.currentValue < 1 ? alert.currentValue.toFixed(1) : Math.round(alert.currentValue);
                    currentValueDisplay = `${displayValue}`;
                }
            } else if (typeof alert.currentValue === 'object' && alert.currentValue !== null) {
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
                // Don't display duration strings as current value
                const isDurationString = typeof alert.currentValue === 'string' && /^\d+[smhd]$/.test(alert.currentValue);
                currentValueDisplay = isDurationString ? '' : (alert.currentValue || '');
            }
        } catch (e) {
            console.error('[Alerts] Error processing currentValue:', e, alert);
            currentValueDisplay = '';
        }
        
        // Muted text classes for acknowledged/resolved alerts
        const nameClass = isResolved || isAcknowledged ? 'text-xs font-medium text-gray-500 dark:text-gray-400' : 
                         'text-xs font-medium text-gray-900 dark:text-gray-100';
        const valueClass = isResolved || isAcknowledged ? 'text-xs text-gray-500 dark:text-gray-400' : 
                          'text-xs text-gray-600 dark:text-gray-400';
        const ruleClass = isResolved || isAcknowledged ? 'text-xs text-gray-500 dark:text-gray-400' : 
                         'text-xs text-gray-600 dark:text-gray-400';
        
        return `
            <div class="${cardClasses}">
                <div class="flex items-center space-x-2">
                    <div class="flex-shrink-0 ${isResolved || isAcknowledged ? 'opacity-60' : ''}">
                        ${icon}
                    </div>
                    <div class="flex-1 min-w-0">
                        <div class="flex items-center space-x-1">
                            <span class="${nameClass}">
                                ${alert.guest?.type === 'node' ? `Node: ${alert.guest.name || 'Unknown'}` : (alert.guest?.name || 'Unknown Guest')}
                            </span>
                            ${(() => {
                                // Handle different alert types
                                if (alert.type === 'state_change') {
                                    // State change alerts show status
                                    return `<span class="text-xs font-medium bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-1.5 py-0.5 rounded">
                                        ${alert.currentValue ? alert.currentValue.toUpperCase() : 'STATUS'}
                                    </span>`;
                                } else if (alert.metric && alert.metric !== 'bundled') {
                                    // Metric alerts show the metric name
                                    return `<span class="text-xs font-medium bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-1.5 py-0.5 rounded">
                                        ${alert.metric.toUpperCase()}
                                    </span>`;
                                }
                                return '';
                            })()}
                            <span class="${valueClass}">
                                ${alert.metric === 'bundled' ? '' : currentValueDisplay}
                            </span>
                            ${isResolved ? '<span class="text-xs border border-gray-300 dark:border-gray-500 text-gray-500 dark:text-gray-400 px-1 rounded text-[10px]">resolved</span>' : 
                              isAcknowledged ? '<span class="text-xs border border-gray-300 dark:border-gray-500 text-gray-500 dark:text-gray-400 px-1 rounded text-[10px]">ack</span>' : 
                              isPending ? '<span class="text-xs border border-amber-400/50 dark:border-amber-500/50 text-amber-600 dark:text-amber-400 px-1 rounded text-[10px]">pending</span>' : ''}
                        </div>
                        <div class="${ruleClass}" style="white-space: normal; overflow: visible;">
                            ${(() => {
                                // Skip showing message details for acknowledgment alerts
                                if (alert.message && alert.message.includes('acknowledged by')) {
                                    return '';
                                }
                                // Handle legacy bundled alerts
                                if (alert.metric === 'bundled') {
                                    return '<div class="text-xs text-gray-500 dark:text-gray-400">Multiple metrics (legacy alert)</div>';
                                }
                                
                                // Handle state change alerts
                                if (alert.type === 'state_change') {
                                    const ruleName = alert.rule?.name || alert.rule?.id || 'State change';
                                    return `<div class="text-xs font-medium">${ruleName}</div>`;
                                }
                                
                                // Display metric information if available
                                if (alert.metric && alert.metric !== 'bundled') {
                                    const metricName = alert.metric.charAt(0).toUpperCase() + alert.metric.slice(1);
                                    const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(alert.metric);
                                    
                                    // If we have current and threshold values
                                    if (alert.currentValue !== undefined && alert.threshold !== undefined) {
                                        const currentVal = alert.currentValue < 1 ? parseFloat((alert.currentValue || 0).toFixed(1)) : Math.round(alert.currentValue || 0);
                                        const thresholdVal = alert.threshold < 1 ? parseFloat((alert.threshold || 0).toFixed(1)) : Math.round(alert.threshold || 0);
                                    
                                    // Only show progress bar for percentage metrics
                                    if (isPercentageMetric) {
                                        const excess = currentVal - thresholdVal;
                                        let barColor = isResolved || isAcknowledged ? 'bg-gray-400' : 'bg-red-500';
                                        if (!isResolved && !isAcknowledged) {
                                            if (excess <= 5) barColor = 'bg-yellow-500';
                                            if (excess > 20) barColor = 'bg-red-600';
                                        }
                                        
                                        return `
                                            <div class="mb-0.5">
                                                <div class="flex justify-between text-xs mb-0.5">
                                                    <span class="text-gray-500 dark:text-gray-400">Threshold</span>
                                                    <span class="text-gray-500 dark:text-gray-400">${thresholdVal}%</span>
                                                </div>
                                                <div class="w-full bg-gray-200 dark:bg-gray-700 rounded h-2 relative">
                                                    <div class="bg-gray-300 dark:bg-gray-600 h-2 rounded absolute top-0 left-0" style="width: ${Math.min(thresholdVal, 100)}%; z-index: 1;"></div>
                                                    <div class="${barColor} h-2 rounded absolute top-0 left-0" style="width: ${Math.min(currentVal, 100)}%; z-index: 2;"></div>
                                                    ${thresholdVal < 100 && thresholdVal > 0 ? `<div class="absolute top-0 bg-orange-400 h-2" style="left: ${thresholdVal}%; width: 2px; z-index: 3;"></div>` : ''}
                                                </div>
                                            </div>
                                        `;
                                        } else {
                                            // For I/O metrics, just show the values
                                            return `
                                                <div class="mb-0.5">
                                                    <div class="flex justify-between text-xs">
                                                        <span class="font-medium">${metricName}</span>
                                                        <span>${formatMetricValue(currentVal, alert.metric)} / ${formatMetricValue(thresholdVal, alert.metric)}</span>
                                                    </div>
                                                </div>
                                            `;
                                        }
                                    } else {
                                        // No threshold data, just show metric name
                                        return `<div class="text-xs font-medium">${metricName} alert</div>`;
                                    }
                                }
                                
                                // Fallback to parsing message for older alerts
                                if (!alert.message) {
                                    return '';
                                }
                                
                                const parts = alert.message.split(': ');
                                const metricsText = parts.slice(1).join(': ');
                                
                                // Always show metric info if available but no detailed display
                                if (alert.metric && alert.metric !== 'bundled') {
                                    const metricName = alert.metric.charAt(0).toUpperCase() + alert.metric.slice(1);
                                    const value = alert.currentValue ? ` (${formatMetricValue(alert.currentValue, alert.metric)})` : '';
                                    return `<div class="text-xs">${metricName} threshold exceeded${value}</div>`;
                                }
                                
                                // If no metrics in message, show basic info
                                if (!metricsText) {
                                    return '<div class="text-xs">Threshold exceeded</div>';
                                }
                                
                                const metrics = metricsText.split(', ');
                                
                                return metrics.map(metric => {
                                    const trimmed = metric.trim();
                                    // Parse metric like "CPU: 56% (≥20%)"
                                    const percentMatch = trimmed.match(/^(.+?):\s*(\d+)%\s*\(≥(\d+)%\)$/);
                                    
                                    if (percentMatch) {
                                        const [, name, current, threshold] = percentMatch;
                                        const currentVal = parseInt(current);
                                        const thresholdVal = parseInt(threshold);
                                        
                                        // Determine color based on how much it exceeds threshold
                                        const excess = currentVal - thresholdVal;
                                        let barColor = isResolved || isAcknowledged ? 'bg-gray-400' : 'bg-red-500';
                                        if (!isResolved && !isAcknowledged) {
                                            if (excess <= 5) barColor = 'bg-yellow-500';
                                            if (excess > 20) barColor = 'bg-red-600';
                                        }
                                        
                                        return `
                                            <div class="mb-0.5">
                                                <div class="flex justify-between text-xs mb-0.5">
                                                    <span class="text-gray-500 dark:text-gray-400">Threshold</span>
                                                    <span class="text-gray-500 dark:text-gray-400">${thresholdVal}%</span>
                                                </div>
                                                <div class="w-full bg-gray-200 dark:bg-gray-700 rounded h-2 relative">
                                                    <div class="bg-gray-300 dark:bg-gray-600 h-2 rounded absolute top-0 left-0" style="width: ${Math.min(thresholdVal, 100)}%; z-index: 1;"></div>
                                                    <div class="${barColor} h-2 rounded absolute top-0 left-0" style="width: ${Math.min(currentVal, 100)}%; z-index: 2;"></div>
                                                    ${thresholdVal < 100 && thresholdVal > 0 ? `<div class="absolute top-0 bg-orange-400 h-2" style="left: ${thresholdVal}%; width: 2px; z-index: 3;"></div>` : ''}
                                                </div>
                                            </div>
                                        `;
                                    } 
                                    
                                    // Parse I/O metrics like "Disk Read: 2.5 MB/s (≥1 MB/s)" or "Network In: 831.67 GB/s (≥1 MB/s)"
                                    const ioMatch = trimmed.match(/^(.+?):\s*([\d.]+)\s*([KMGT]?B\/s)\s*\(≥([\d.]+)\s*([KMGT]?B\/s)\)$/);
                                    if (ioMatch) {
                                        const [, name, current, currentUnit, threshold, thresholdUnit] = ioMatch;
                                        const currentVal = parseFloat(current);
                                        const thresholdVal = parseFloat(threshold);
                                        
                                        const unitMultipliers = { 'B/s': 1/1048576, 'KB/s': 1/1024, 'MB/s': 1, 'GB/s': 1024, 'TB/s': 1048576 };
                                        const currentMBps = currentVal * (unitMultipliers[currentUnit] || 1);
                                        const thresholdMBps = thresholdVal * (unitMultipliers[thresholdUnit] || 1);
                                        
                                        const maxDisplayMBps = Math.max(thresholdMBps * 2, 10); // At least 10 MB/s or 2x threshold
                                        const currentPercent = Math.min((currentMBps / maxDisplayMBps) * 100, 100);
                                        const thresholdPercent = Math.min((thresholdMBps / maxDisplayMBps) * 100, 100);
                                        
                                        // Determine color based on how much it exceeds threshold
                                        const excessRatio = currentMBps / thresholdMBps;
                                        let barColor = isResolved || isAcknowledged ? 'bg-gray-400' : 'bg-blue-500';
                                        if (!isResolved && !isAcknowledged) {
                                            if (excessRatio > 1) barColor = 'bg-yellow-500';
                                            if (excessRatio > 2) barColor = 'bg-orange-500';
                                            if (excessRatio > 5) barColor = 'bg-red-500';
                                        }
                                        
                                        return `
                                            <div class="mb-0.5">
                                                <div class="flex justify-between text-xs mb-0.5">
                                                    <span class="font-medium">${name}</span>
                                                    <span>${currentVal}${currentUnit} / ${thresholdVal}${thresholdUnit}</span>
                                                </div>
                                                <div class="w-full bg-gray-200 dark:bg-gray-700 rounded h-2 relative">
                                                    <div class="bg-gray-300 dark:bg-gray-600 h-2 rounded absolute top-0 left-0" style="width: ${thresholdPercent}%; z-index: 1;"></div>
                                                    <div class="${barColor} h-2 rounded absolute top-0 left-0" style="width: ${currentPercent}%; z-index: 2;"></div>
                                                    ${thresholdPercent < 90 ? `<div class="absolute top-0 bg-gray-400 h-2" style="left: ${thresholdPercent}%; width: 1px; z-index: 3;"></div>` : ''}
                                                </div>
                                            </div>
                                        `;
                                    }
                                    
                                    // Fallback for other metrics
                                    return `<div class="mb-1">${trimmed}</div>`;
                                }).join('');
                            })()}
                            <div class="mt-1 text-xs text-gray-500 alert-timestamp" 
                                 data-triggered-at="${alert.triggeredAt || alert.acknowledgedAt || (Date.now() + serverTimeOffset)}"
                                 data-resolved-at="${alert.resolvedAt || ''}"
                                 data-is-resolved="${isResolved}"
                                 data-is-acknowledged="${isAcknowledged}">
                                ${isResolved ? 
                                    formatTimestamp(triggeredAt) + ' • ' + durationStr : 
                                    durationStr}
                                ${!isResolved && isAcknowledged ? ' • acknowledged' : ''}
                            </div>
                        </div>
                    </div>
                    <div class="flex-shrink-0 space-x-1">
                        ${!isAcknowledged && !isResolved && alert.isActive ? `
                            <button onclick="event.stopPropagation(); PulseApp.alerts.acknowledgeAlert('${alert.id}', '${alert.ruleId}');" 
                                    class="text-xs px-1 py-0.5 bg-green-500 text-white rounded hover:bg-green-600 focus:outline-none transition-all"
                                    data-alert-id="${alert.id}"
                                    title="Acknowledge alert">
                                ✓
                            </button>
                        ` : ''}
                    </div>
                </div>
                ${(() => {
                    // Show progress bar for pending I/O alerts
                    if (isPending && ['diskread', 'diskwrite', 'netin', 'netout'].includes(alert.metric)) {
                        const now = Date.now() + serverTimeOffset;
                        const elapsed = now - (alert.startTime || alert.triggeredAt || now);
                        const duration = alert.ioSustainedPeriod || 30000; // Use configured period or default to 30s
                        const progress = Math.min(100, (elapsed / duration) * 100);
                        const remaining = Math.max(0, duration - elapsed);
                        const remainingSeconds = Math.ceil(remaining / 1000);
                        
                        return `
                            <div class="px-3 pb-2">
                                <div class="flex justify-between text-xs mb-1">
                                    <span class="text-amber-600 dark:text-amber-400">Waiting for sustained activity</span>
                                    <span class="text-gray-500 dark:text-gray-400">${remainingSeconds}s</span>
                                </div>
                                <div class="w-full bg-gray-200 dark:bg-gray-700 rounded h-1.5">
                                    <div class="bg-amber-400/70 dark:bg-amber-500/70 h-1.5 rounded transition-all duration-1000" style="width: ${progress}%"></div>
                                </div>
                            </div>
                        `;
                    }
                    return '';
                })()}
            </div>
        `;
    }

    function handleNewAlert(alert) {
        // Detect alert storm
        const now = Date.now();
        alertsReceivedTimestamps.push(now);
        // Keep only timestamps from last second
        alertsReceivedTimestamps = alertsReceivedTimestamps.filter(ts => now - ts < 1000);
        
        if (alertsReceivedTimestamps.length >= ALERT_STORM_THRESHOLD) {
            if (!alertStormMode) {
                alertStormMode = true;
                console.warn('[Alerts] Alert storm detected! Entering protective mode.');
                showNotification({
                    message: 'High alert volume detected - notifications limited'
                }, 'warning');
            }
        } else if (alertStormMode && alertsReceivedTimestamps.length < ALERT_STORM_THRESHOLD / 2) {
            alertStormMode = false;
        }
        
        // Ensure alert has a triggeredAt timestamp
        if (!alert.triggeredAt) {
            alert.triggeredAt = now;
        }
        
        // Check for duplicate alerts by guest and metric
        const existingIndex = activeAlerts.findIndex(a => {
            // Match by guest and metric type
            return a.metric === alert.metric &&
                   a.guest.vmid === alert.guest.vmid && 
                   a.guest.node === alert.guest.node &&
                   a.guest.endpointId === alert.guest.endpointId;
        });
        
        if (existingIndex >= 0) {
            activeAlerts[existingIndex] = alert;
            // Update in history too if it exists
            const historyIndex = alertHistory.findIndex(h => h.id === alert.id);
            if (historyIndex >= 0) {
                alertHistory[historyIndex] = alert;
            }
        } else {
            activeAlerts.unshift(alert);
            
            // Add to history with timestamp
            const alertWithTimestamp = {
                ...alert,
                triggeredAt: alert.triggeredAt || now
            };
            alertHistory.unshift(alertWithTimestamp);
            
            // Limit the size of activeAlerts array
            if (activeAlerts.length > MAX_ACTIVE_ALERTS) {
                // Remove oldest acknowledged alerts first, then oldest unacknowledged
                const acknowledged = activeAlerts.filter(a => a.acknowledged);
                const unacknowledged = activeAlerts.filter(a => !a.acknowledged);
                
                if (acknowledged.length > MAX_ACTIVE_ALERTS / 2) {
                    // Keep only recent acknowledged alerts
                    const recentAcknowledged = acknowledged.slice(0, MAX_ACTIVE_ALERTS / 2);
                    activeAlerts = [...unacknowledged.slice(0, MAX_ACTIVE_ALERTS / 2), ...recentAcknowledged];
                } else {
                    activeAlerts = activeAlerts.slice(0, MAX_ACTIVE_ALERTS);
                }
            }
            
            // Limit history size
            if (alertHistory.length > 200) {
                alertHistory = alertHistory.slice(0, 200);
            }
            
            // Queue toast notification
            pendingAlertToasts.push({
                guest: alert.guest.name,
                message: alert.message,
                timestamp: now
            });
            
            // Schedule toast processing with a small delay to allow grouping
            if (!toastProcessingTimeout) {
                toastProcessingTimeout = setTimeout(() => {
                    toastProcessingTimeout = null;
                    processToastQueue();
                }, 100); // 100ms delay to collect simultaneous alerts
            }
        }
        
        updateHeaderIndicator();
        
        // Debounced dropdown update
        if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
            if (dropdownUpdateTimeout) {
                clearTimeout(dropdownUpdateTimeout);
            }
            dropdownUpdateTimeout = setTimeout(() => {
                updateDropdownContent();
                dropdownUpdateTimeout = null;
            }, alertStormMode ? 500 : 100); // Longer delay during storm
        }
        
        document.dispatchEvent(new CustomEvent('pulseAlert', { detail: alert }));
    }

    function handleResolvedAlert(alert) {
        try {
            // Find the alert in activeAlerts by ID
            const activeIndex = activeAlerts.findIndex(a => a.id === alert.id);
            
            if (activeIndex >= 0) {
                // Mark as resolved and move to history
                const resolvedAlert = {
                    ...activeAlerts[activeIndex],
                    resolved: true,
                    resolvedAt: Date.now() + serverTimeOffset
                };
                
                // Add to history
                alertHistory.unshift(resolvedAlert);
                
                // Limit history size
                if (alertHistory.length > 200) {
                    alertHistory = alertHistory.slice(0, 200);
                }
                
                // Remove from active alerts
                activeAlerts.splice(activeIndex, 1);
            }
            
            updateHeaderIndicator();
            
            // Queue resolved toast notification
            if (alert && alert.guest && alert.guest.name) {
                pendingResolvedToasts.push({
                    guest: alert.guest.name,
                    message: `Resolved: ${alert.guest.name}`,
                    timestamp: Date.now()
                });
                
                // Schedule resolved toast processing with a small delay to allow grouping
                if (!resolvedToastTimeout) {
                    resolvedToastTimeout = setTimeout(() => {
                        resolvedToastTimeout = null;
                        processResolvedToastQueue();
                    }, 200); // 200ms delay to collect simultaneous resolved alerts
                }
            }
            
            if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
                updateDropdownContent();
            }
            
            document.dispatchEvent(new CustomEvent('pulseAlertResolved', { detail: alert }));
        } catch (error) {
            console.error('[Alerts] Error handling resolved alert:', error);
        }
    }

    function updateHeaderIndicator() {
        const indicator = document.getElementById('alerts-indicator');
        if (!indicator) return;

        const unacknowledgedAlerts = Array.isArray(activeAlerts) ? 
            activeAlerts.filter(a => a && !a.acknowledged) : [];
        const count = unacknowledgedAlerts.length;
        
        // Always show the button, just change its appearance based on unacknowledged alert count
        let className = 'text-xs px-1.5 py-0.5 rounded bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400 cursor-pointer relative flex-shrink-0 transition-colors';
        
        if (count === 0) {
            className = 'text-xs px-1.5 py-0.5 rounded bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400 cursor-pointer relative flex-shrink-0 transition-colors';
        } else {
            className = 'text-xs px-1.5 py-0.5 rounded bg-amber-500 text-white cursor-pointer relative flex-shrink-0 transition-colors';
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

    function showNotification(alert, type = 'alert') {
        // Check if alerts are enabled before showing notifications
        if (!alertsEnabled) {
            return;
        }
        
        // Ensure notification container exists
        if (!notificationContainer) {
            createNotificationContainer();
        }
        
        const notification = document.createElement('div');
        notification.className = `pointer-events-auto transform transition-all duration-200 ease-out opacity-0 translate-y-2 scale-95`;
        
        // Simple styling based on notification type
        let colorClass, title;
        if (type === 'resolved') {
            colorClass = 'bg-green-50 border border-green-200 text-green-700 dark:bg-green-900/20 dark:border-green-700 dark:text-green-200';
            title = 'Resolved';
        } else if (type === 'warning') {
            colorClass = 'bg-yellow-50 border border-yellow-200 text-yellow-700 dark:bg-yellow-900/20 dark:border-yellow-700 dark:text-yellow-200';
            title = 'Warning';
        } else {
            colorClass = 'bg-amber-50 border border-amber-200 text-amber-700 dark:bg-amber-900/20 dark:border-amber-700 dark:text-amber-200';
            title = alert.message && alert.message.includes('acknowledged') ? 'Success' : 'Alert';
        }
        
        const icon = ALERT_ICONS.active;
        
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
        
        // Optimistically update the UI immediately
        // First check activeAlerts
        let alertIndex = activeAlerts.findIndex(a => a.id === alertId);
        let targetArray = activeAlerts;
        
        // If not found in activeAlerts, check alertHistory
        if (alertIndex < 0) {
            alertIndex = alertHistory.findIndex(a => a.id === alertId);
            targetArray = alertHistory;
        }
        
        if (alertIndex >= 0) {
            // Store original state in case we need to rollback
            const originalState = {
                acknowledged: targetArray[alertIndex].acknowledged,
                acknowledgedAt: targetArray[alertIndex].acknowledgedAt
            };
            
            // Update local state immediately
            targetArray[alertIndex].acknowledged = true;
            targetArray[alertIndex].acknowledgedAt = Date.now() + serverTimeOffset;
            
            // Update UI immediately
            updateHeaderIndicator();
            if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
                updateDropdownContent();
            }
            
            // Schedule cleanup
            scheduleAcknowledgedCleanup(alertId);
            
            try {
                // Send to server in background
                const response = await fetch(`/api/alerts/${alertId}/acknowledge`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ userId: 'web-user', note: 'Acknowledged via web interface' })
                });
                
                if (!response.ok) {
                    // Rollback on failure
                    targetArray[alertIndex].acknowledged = originalState.acknowledged;
                    targetArray[alertIndex].acknowledgedAt = originalState.acknowledgedAt;
                    
                    // Cancel cleanup
                    if (cleanupTimeouts.has(alertId)) {
                        clearTimeout(cleanupTimeouts.get(alertId));
                        cleanupTimeouts.delete(alertId);
                    }
                    
                    // Update UI to reflect rollback
                    updateHeaderIndicator();
                    if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
                        updateDropdownContent();
                    }
                    
                    const errorText = await response.text().catch(() => 'Unknown error');
                    console.error('[Alerts] Acknowledge failed with status:', response.status, errorText);
                    showToastNotification(`Failed to acknowledge alert`, 'error');
                }
            } catch (error) {
                console.error('[Alerts] Failed to acknowledge alert:', error);
                
                // Rollback on network error
                if (alertIndex >= 0) {
                    targetArray[alertIndex].acknowledged = originalState.acknowledged;
                    targetArray[alertIndex].acknowledgedAt = originalState.acknowledgedAt;
                    
                    // Cancel cleanup
                    if (cleanupTimeouts.has(alertId)) {
                        clearTimeout(cleanupTimeouts.get(alertId));
                        cleanupTimeouts.delete(alertId);
                    }
                    
                    // Update UI
                    updateHeaderIndicator();
                    if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
                        updateDropdownContent();
                    }
                }
                
                showToastNotification(`Failed to acknowledge alert`, 'error');
            }
        }
        
        acknowledgeInProgress.delete(alertId);
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
                        activeAlerts[alertIndex].acknowledgedAt = Date.now() + serverTimeOffset;
                        
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
        
        if (errorCount > 0) {
            //     message: `${errorCount} alerts failed to acknowledge` 
        }
        // No "success" notification needed - user can see alerts are acknowledged
    }

    function updateAlertsFromState(state) {
        // Don't process alert updates if alerts are disabled
        if (!alertsEnabled) {
            return;
        }
        
        if (state && state.alerts) {
            // Ensure activeAlerts is always an array
            if (state.alerts.active !== undefined) {
                const previousAlerts = [...activeAlerts];
                // Trust the server's active alerts list
                activeAlerts = Array.isArray(state.alerts.active) ? state.alerts.active : [];
                
                // Check for new alerts that weren't in the previous list
                const previousIds = new Set(previousAlerts.map(a => a.id));
                const newAlerts = activeAlerts.filter(alert => !previousIds.has(alert.id) && !alert.acknowledged && !alert.resolved);
                
                // Show notifications for new alerts
                if (newAlerts.length > 0) {
                    // Group alerts by metric type
                    const alertsByMetric = {};
                    newAlerts.forEach(alert => {
                        const metric = alert.metric || 'unknown';
                        if (!alertsByMetric[metric]) {
                            alertsByMetric[metric] = [];
                        }
                        alertsByMetric[metric].push(alert);
                    });
                    
                    // Show grouped notifications
                    Object.entries(alertsByMetric).forEach(([metric, alerts]) => {
                        if (alerts.length === 1) {
                            // Single alert for this metric
                            const alert = alerts[0];
                            const guestName = alert.guest?.name || 'Unknown';
                            showNotification({
                                message: `${metric.charAt(0).toUpperCase() + metric.slice(1)} alert: ${guestName}`,
                                guest: alert.guest,
                                metric: alert.metric
                            }, 'alert');
                        } else {
                            // Multiple alerts for this metric
                            const guestNames = alerts
                                .map(a => a.guest?.name)
                                .filter(Boolean);
                            
                            if (guestNames.length === 0) {
                                // No guest names available
                                showNotification({
                                    message: `${alerts.length} ${metric} alerts`,
                                    metric: metric
                                }, 'alert');
                            } else if (guestNames.length <= 3) {
                                showNotification({
                                    message: `${metric.charAt(0).toUpperCase() + metric.slice(1)} alerts: ${guestNames.join(', ')}`,
                                    metric: metric
                                }, 'alert');
                            } else {
                                showNotification({
                                    message: `${alerts.length} ${metric} alerts from multiple guests`,
                                    metric: metric
                                }, 'alert');
                            }
                        }
                    });
                }
                
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
                acknowledgedAt: alert.acknowledgedAt || (Date.now() + serverTimeOffset),
                acknowledgedBy: alert.acknowledgedBy
            };
            updateHeaderIndicator();
            if (alertDropdown && !alertDropdown.classList.contains('hidden')) {
                updateDropdownContent();
            }
        }
    }
    
    function handleRulesRefreshed() {
        // Trigger the alerts UI to reload its configuration
        if (window.PulseApp && window.PulseApp.ui && window.PulseApp.ui.alerts && window.PulseApp.ui.alerts.loadSavedConfiguration) {
            window.PulseApp.ui.alerts.loadSavedConfiguration();
        }
        
        // Also reload the alert data to get any new rules
        loadInitialData();
    }
    
    // Get metric unit
    function getMetricUnit(metric) {
        switch(metric) {
            case 'diskread':
            case 'diskwrite':
            case 'netin':
            case 'netout':
                return 'B/s';
            case 'cpu':
            case 'memory':
            case 'disk':
                return '%';
            default:
                return '';
        }
    }
    
    // Format metric value with appropriate unit
    function formatMetricValue(value, metric) {
        const unit = getMetricUnit(metric);
        if (unit === 'B/s') {
            // Convert bytes/s to human readable
            if (value >= 1073741824) {
                return `${(value / 1073741824).toFixed(1)}GB/s`;
            } else if (value >= 1048576) {
                return `${(value / 1048576).toFixed(1)}MB/s`;
            } else if (value >= 1024) {
                return `${(value / 1024).toFixed(1)}KB/s`;
            } else {
                return `${value}B/s`;
            }
        } else if (unit === '%') {
            return `${Math.round(value)}%`;
        } else {
            return `${value}`;
        }
    }
    
    // Format duration with proper units
    function formatDuration(seconds) {
        if (seconds < 60) {
            return `${seconds}s`;
        } else if (seconds < 120) {
            // Show "1m 15s" format for 60-119 seconds
            const minutes = Math.floor(seconds / 60);
            const remainingSeconds = seconds % 60;
            if (remainingSeconds === 0) {
                return `${minutes}m`;
            }
            return `${minutes}m ${remainingSeconds}s`;
        } else if (seconds < 3600) {
            const minutes = Math.floor(seconds / 60);
            return `${minutes}m`;
        } else if (seconds < 86400) {
            const hours = Math.floor(seconds / 3600);
            const remainingMinutes = Math.floor((seconds % 3600) / 60);
            if (remainingMinutes === 0) {
                return `${hours}h`;
            }
            return `${hours}h ${remainingMinutes}m`;
        } else {
            const days = Math.floor(seconds / 86400);
            return `${days}d`;
        }
    }
    
    // Format timestamp to readable time
    function formatTimestamp(timestamp, skipDate = false) {
        const date = new Date(timestamp);
        const now = new Date();
        
        // If skipDate is true, just return time
        if (skipDate) {
            return date.toLocaleTimeString('en-GB', { 
                hour: '2-digit', 
                minute: '2-digit' 
            });
        }
        
        // If it's today, just show the time
        if (date.toDateString() === now.toDateString()) {
            return date.toLocaleTimeString('en-GB', { 
                hour: '2-digit', 
                minute: '2-digit' 
            });
        }
        
        // If it's yesterday
        const yesterday = new Date(now);
        yesterday.setDate(yesterday.getDate() - 1);
        if (date.toDateString() === yesterday.toDateString()) {
            return 'Yesterday ' + date.toLocaleTimeString('en-GB', { 
                hour: '2-digit', 
                minute: '2-digit' 
            });
        }
        
        // Otherwise show date and time
        return date.toLocaleDateString('en-GB', { 
            day: 'numeric',
            month: 'short'
        }) + ' ' + date.toLocaleTimeString('en-GB', { 
            hour: '2-digit', 
            minute: '2-digit' 
        });
    }
    
    // Start live timestamp updates
    function startTimestampUpdates() {
        // Clear any existing interval
        stopTimestampUpdates();
        
        // Update timestamps immediately
        updateAllTimestamps();
        
        // Then update every second
        timestampUpdateInterval = setInterval(updateAllTimestamps, 1000);
    }
    
    // Stop timestamp updates
    function stopTimestampUpdates() {
        if (timestampUpdateInterval) {
            clearInterval(timestampUpdateInterval);
            timestampUpdateInterval = null;
        }
    }
    
    // Update all visible timestamps
    function updateAllTimestamps() {
        if (!alertDropdown || alertDropdown.classList.contains('hidden')) return;
        
        // Don't update resolved alerts timestamps - they don't change
        const timestampElements = alertDropdown.querySelectorAll('.alert-timestamp[data-is-resolved="false"]');
        
        if (timestampElements.length === 0) {
            return;
        }
        
        // Adjust current time by server offset to match server clock
        const now = Date.now() + serverTimeOffset;
        
        timestampElements.forEach((element) => {
            const triggeredAt = parseInt(element.dataset.triggeredAt);
            const isAcknowledged = element.dataset.isAcknowledged === 'true';
            
            if (triggeredAt && !isNaN(triggeredAt)) {
                // Calculate duration for active alerts only
                const rawDiff = now - triggeredAt;
                const duration = Math.max(0, Math.round(rawDiff / 1000));
                
                let text = formatDuration(duration);
                if (isAcknowledged) {
                    text += ' • acknowledged';
                }
                
                // Only update if text has changed to avoid unnecessary DOM updates
                if (element.textContent !== text) {
                    element.textContent = text;
                }
            }
        });
    }
    
    // Cleanup function to prevent memory leaks
    function cleanup() {
        // Clear all cleanup timeouts
        for (const timeoutId of cleanupTimeouts.values()) {
            clearTimeout(timeoutId);
        }
        cleanupTimeouts.clear();
        
        // Clear dropdown update timeout
        if (dropdownUpdateTimeout) {
            clearTimeout(dropdownUpdateTimeout);
            dropdownUpdateTimeout = null;
        }
        
        // Clear toast processing timeout
        if (toastProcessingTimeout) {
            clearTimeout(toastProcessingTimeout);
            toastProcessingTimeout = null;
        }
        
        // Clear timestamp update interval
        stopTimestampUpdates();
        
        // Clear pending toasts
        pendingAlertToasts = [];
        pendingResolvedToasts = [];
        
        // Clear timeouts
        if (resolvedToastTimeout) {
            clearTimeout(resolvedToastTimeout);
            resolvedToastTimeout = null;
        }
        
        // Remove socket listeners if needed
        if (window.socket) {
            window.socket.off('alert', handleNewAlert);
            window.socket.off('alertResolved', handleResolvedAlert);
            window.socket.off('alertAcknowledged', handleAcknowledgedAlert);
            window.socket.off('alertRulesRefreshed', handleRulesRefreshed);
        }
    }

    // Helper function for toast notifications
    function showToastNotification(message, type = 'alert') {
        if (window.PulseApp && window.PulseApp.ui && window.PulseApp.ui.toastNotifications) {
            window.PulseApp.ui.toastNotifications.show(message, type);
        } else {
            // Fallback to basic notification
            showNotification({ message }, type);
        }
    }

    // Acknowledge all alerts for a specific VM
    async function acknowledgeVMAlerts(vmid, node, endpointId) {
        // Find all unacknowledged alerts for this VM
        const vmAlerts = activeAlerts.filter(a => 
            !a.acknowledged && 
            !a.resolved &&
            a.guest.vmid === vmid &&
            a.guest.node === node &&
            a.guest.endpointId === endpointId
        );
        
        if (vmAlerts.length === 0) {
            return;
        }
        
        // Acknowledge each alert
        for (const alert of vmAlerts) {
            await acknowledgeAlert(alert.id, alert.ruleId);
        }
        
        showToastNotification(`Acknowledged ${vmAlerts.length} alert${vmAlerts.length !== 1 ? 's' : ''} for ${vmAlerts[0].guest.name}`, 'success');
    }
    
    // Test function to manually trigger a notification
    function testNewAlertNotification() {
        // First check if container exists
        if (!notificationContainer) {
            createNotificationContainer();
        }
        
        // Create a simple test div
        const testDiv = document.createElement('div');
        testDiv.style.cssText = 'position: fixed; bottom: 20px; right: 20px; background: red; color: white; padding: 20px; z-index: 99999; font-size: 18px; font-weight: bold; border: 3px solid black;';
        testDiv.textContent = 'TEST ALERT NOTIFICATION';
        testDiv.onclick = () => testDiv.remove();
        document.body.appendChild(testDiv);
        
        // Also try the regular notification
        showNotification({
            message: 'Test Alert: debian',
            guest: { name: 'debian' }
        }, 'alert');
    }
    
    // Public API
    return {
        init,
        showNotification,
        showAlertsDropdown: openDropdown,
        hideAlertsDropdown: closeDropdown,
        updateAlertsFromState,
        acknowledgeAlert,
        acknowledgeVMAlerts,
        testNewAlertNotification,  // Add to public API for testing
        suppressAlert,
        markAllAsAcknowledged,
        toggleAcknowledgedSection,
        getActiveAlerts: () => activeAlerts,
        getAlertHistory: () => alertHistory,
        cleanup
    };
    
    // Check if alerts are enabled in the system configuration
    async function checkAlertsEnabled() {
        try {
            const config = await PulseApp.apiClient.get('/api/config');
            return config.advanced?.alerts?.enabled !== false;
        } catch (error) {
            console.error('[Alerts] Failed to check alerts enabled status:', error);
            return true; // Default to enabled if we can't check
        }
    }
    
})();

// Auto-initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', PulseApp.alerts.init);
} else {
    PulseApp.alerts.init();
}

// Listen for alerts enabled/disabled changes from settings (global listener)
window.addEventListener('alertsEnabledChanged', (event) => {
    alertsEnabled = event.detail.enabled;
    const indicator = document.getElementById('alerts-indicator');
    
    if (indicator) {
        // Simply hide/show the indicator
        indicator.style.display = alertsEnabled ? '' : 'none';
    }
    
    // Close dropdown if alerts are disabled
    if (!alertsEnabled) {
        const dropdown = document.getElementById('alerts-dropdown');
        if (dropdown) {
            PulseApp.alerts.hideAlertsDropdown();
        }
    }
});