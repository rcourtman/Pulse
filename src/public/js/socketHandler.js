PulseApp.socketHandler = (() => {
    let socket = null;
    let isConnected = false;
    let reconnectAttempts = 0;
    const maxReconnectAttempts = 10;
    const reconnectDelay = 2000; // 2 seconds

    function init() {
        console.log('[Socket] Initializing socket connection...');
        createSocket();
    }

    function createSocket() {
        if (socket) {
            socket.removeAllListeners();
            socket.disconnect();
        }

        socket = io();
        window.socket = socket; // Make socket available globally for alerts

        setupEventListeners();
    }

    function setupEventListeners() {
        socket.on('connect', handleConnect);
        socket.on('disconnect', handleDisconnect);
        socket.on('rawData', handleRawData);
        socket.on('initialState', handleInitialState);
        socket.on('requestError', handleRequestError);
        
        // Enhanced monitoring events
        socket.on('alert', handleAlert);
        socket.on('alertResolved', handleAlertResolved);
        
        // Development features
        socket.on('hotReload', handleHotReload);

        // Handle connection errors
        socket.on('connect_error', handleConnectError);
        socket.on('reconnect', handleReconnect);
        socket.on('reconnect_error', handleReconnectError);
        socket.on('reconnect_failed', handleReconnectFailed);
    }

    function handleConnect() {
        console.log('[Socket] Connected to server');
        isConnected = true;
        reconnectAttempts = 0;
        updateConnectionStatus('connected');
        
        // Request initial data
        socket.emit('requestData');
    }

    function handleDisconnect(reason) {
        console.log('[Socket] Disconnected from server:', reason);
        isConnected = false;
        updateConnectionStatus('disconnected');
        
        // If it's not a planned disconnect, try to reconnect
        if (reason !== 'io client disconnect') {
            attemptReconnect();
        }
    }

    function handleRawData(data) {
        try {
            // Update main application state
            if (PulseApp.state) {
                PulseApp.state.updateState(data);
            }
            
            // Update alerts system with new state
            if (PulseApp.alerts && data.alerts) {
                PulseApp.alerts.updateAlertsFromState(data);
            }
            
            // Process UI updates based on tab
            updateUIFromData(data);
            
        } catch (error) {
            console.error('[Socket] Error processing raw data:', error);
        }
    }

    function handleInitialState(data) {
        console.log('[Socket] Received initial state:', data);
        
        try {
            if (PulseApp.state) {
                PulseApp.state.updateState(data);
            }
            
            updateUIFromData(data);
            
        } catch (error) {
            console.error('[Socket] Error processing initial state:', error);
        }
    }

    function handleRequestError(error) {
        console.error('[Socket] Request error:', error);
        updateConnectionStatus('error');
    }

    function handleAlert(alert) {
        console.log('[Socket] Received alert:', alert);
        
        // Forward to alerts handler
        if (PulseApp.alerts) {
            // The alerts handler will be called directly from its socket listeners
            // This is just for any additional processing
        }
    }

    function handleAlertResolved(alert) {
        console.log('[Socket] Alert resolved:', alert);
        
        // Forward to alerts handler
        if (PulseApp.alerts) {
            // The alerts handler will be called directly from its socket listeners
            // This is just for any additional processing
        }
    }

    function handleHotReload() {
        console.log('[Socket] Hot reload triggered');
        if (process.env.NODE_ENV === 'development') {
            window.location.reload();
        }
    }

    function handleConnectError(error) {
        console.error('[Socket] Connection error:', error);
        updateConnectionStatus('error');
    }

    function handleReconnect() {
        console.log('[Socket] Reconnected successfully');
        reconnectAttempts = 0;
        updateConnectionStatus('connected');
    }

    function handleReconnectError(error) {
        console.error('[Socket] Reconnection error:', error);
        reconnectAttempts++;
        updateConnectionStatus('reconnecting');
    }

    function handleReconnectFailed() {
        console.error('[Socket] Reconnection failed - max attempts reached');
        updateConnectionStatus('failed');
    }

    function attemptReconnect() {
        if (reconnectAttempts < maxReconnectAttempts) {
            reconnectAttempts++;
            updateConnectionStatus('reconnecting');
            
            setTimeout(() => {
                console.log(`[Socket] Attempting to reconnect... (${reconnectAttempts}/${maxReconnectAttempts})`);
                socket.connect();
            }, reconnectDelay * reconnectAttempts); // Exponential backoff
        } else {
            updateConnectionStatus('failed');
        }
    }

    function updateConnectionStatus(status) {
        const statusElement = document.getElementById('connection-status');
        if (!statusElement) return;

        // Clear previous classes
        statusElement.className = statusElement.className
            .replace(/\b(connected|disconnected|reconnecting|error|failed)\b/g, '')
            .trim();

        let statusText, statusClass;

        switch (status) {
            case 'connected':
                statusText = 'Connected';
                statusClass = 'connected text-xs px-2 py-1 rounded-full bg-green-100 dark:bg-green-900 text-green-600 dark:text-green-400';
                break;
            case 'disconnected':
                statusText = 'Disconnected';
                statusClass = 'disconnected text-xs px-2 py-1 rounded-full bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400';
                break;
            case 'reconnecting':
                statusText = `Reconnecting... (${reconnectAttempts}/${maxReconnectAttempts})`;
                statusClass = 'reconnecting text-xs px-2 py-1 rounded-full bg-yellow-100 dark:bg-yellow-900 text-yellow-600 dark:text-yellow-400 animate-pulse';
                break;
            case 'error':
                statusText = 'Connection Error';
                statusClass = 'error text-xs px-2 py-1 rounded-full bg-red-100 dark:bg-red-900 text-red-600 dark:text-red-400';
                break;
            case 'failed':
                statusText = 'Connection Failed';
                statusClass = 'failed text-xs px-2 py-1 rounded-full bg-red-100 dark:bg-red-900 text-red-600 dark:text-red-400';
                break;
            default:
                statusText = 'Unknown';
                statusClass = 'text-xs px-2 py-1 rounded-full bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400';
        }

        statusElement.textContent = statusText;
        statusElement.className = statusClass;
    }

    function updateUIFromData(data) {
        try {
            // Hide loading overlay when we receive data
            const loadingOverlay = document.getElementById('loading-overlay');
            if (loadingOverlay && (data.nodes || data.vms || data.containers || data.pbs)) {
                loadingOverlay.style.display = 'none';
            }

            // Update different UI sections based on current tab
            const activeTab = document.querySelector('.tab.active');
            if (!activeTab) return;

            const tabName = activeTab.getAttribute('data-tab');

            switch (tabName) {
                case 'main':
                    updateMainTab(data);
                    break;
                case 'storage':
                    updateStorageTab(data);
                    break;
                case 'pbs':
                    updatePbsTab(data);
                    break;
                case 'backups':
                    updateBackupsTab(data);
                    break;
            }

            // Update performance indicators if available
            updatePerformanceIndicators(data);

        } catch (error) {
            console.error('[Socket] Error updating UI from data:', error);
        }
    }

    function updateMainTab(data) {
        try {
            // Update node summary cards
            if (PulseApp.ui && PulseApp.ui.nodes && data.nodes) {
                PulseApp.ui.nodes.updateNodeSummaryCards(data.nodes);
            }

            // Update main dashboard
            if (PulseApp.ui && PulseApp.ui.dashboard) {
                PulseApp.ui.dashboard.updateDashboardTable();
            }

        } catch (error) {
            console.error('[Socket] Error updating main tab:', error);
        }
    }

    function updateStorageTab(data) {
        try {
            if (PulseApp.ui && PulseApp.ui.storage && data.nodes) {
                PulseApp.ui.storage.updateStorageInfo();
            }
        } catch (error) {
            console.error('[Socket] Error updating storage tab:', error);
        }
    }

    function updatePbsTab(data) {
        try {
            if (PulseApp.ui && PulseApp.ui.pbs && data.pbs) {
                PulseApp.ui.pbs.updatePbsInfo(data.pbs);
            }
        } catch (error) {
            console.error('[Socket] Error updating PBS tab:', error);
        }
    }

    function updateBackupsTab(data) {
        try {
            if (PulseApp.ui && PulseApp.ui.backups && data.pbs) {
                PulseApp.ui.backups.updateBackupsTab();
            }
        } catch (error) {
            console.error('[Socket] Error updating backups tab:', error);
        }
    }

    function updatePerformanceIndicators(data) {
        try {
            // Update performance stats if available
            if (data.stats) {
                updateStatsDisplay(data.stats);
            }

            // Update any health indicators
            if (data.performance) {
                updateHealthDisplay(data.performance);
            }

        } catch (error) {
            console.error('[Socket] Error updating performance indicators:', error);
        }
    }

    function updateStatsDisplay(stats) {
        // Update various stats in the UI
        try {
            const statusText = document.getElementById('dashboard-status-text');
            if (statusText && stats.totalGuests !== undefined) {
                const runningText = stats.runningGuests ? `${stats.runningGuests} running` : '0 running';
                const stoppedText = stats.stoppedGuests ? `${stats.stoppedGuests} stopped` : '0 stopped';
                statusText.textContent = `${stats.totalGuests} total guests (${runningText}, ${stoppedText})`;
            }

        } catch (error) {
            console.error('[Socket] Error updating stats display:', error);
        }
    }

    function updateHealthDisplay(performance) {
        // Update health indicators in the UI
        try {
            // Could add health indicators to the header or other parts of the UI
            // For now, this is just a placeholder for future enhancements

        } catch (error) {
            console.error('[Socket] Error updating health display:', error);
        }
    }

    // Manual data request
    function requestData() {
        if (socket && isConnected) {
            socket.emit('requestData');
        } else {
            console.warn('[Socket] Cannot request data - not connected');
        }
    }

    // Get connection status
    function getConnectionStatus() {
        return {
            connected: isConnected,
            reconnectAttempts,
            socket: socket ? socket.id : null
        };
    }

    // Manual reconnect
    function reconnect() {
        if (socket) {
            reconnectAttempts = 0;
            socket.disconnect();
            socket.connect();
        }
    }

    // Cleanup
    function destroy() {
        if (socket) {
            socket.removeAllListeners();
            socket.disconnect();
            socket = null;
        }
        isConnected = false;
        window.socket = null;
    }

    // Public API
    return {
        init,
        requestData,
        getConnectionStatus,
        reconnect,
        destroy
    };
})();

// Auto-initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', PulseApp.socketHandler.init);
} else {
    PulseApp.socketHandler.init();
}

