PulseApp.socketHandler = (() => {
    let socket = null;
    let isConnected = false;
    let reconnectAttempts = 0;
    const maxReconnectAttempts = 10;
    const reconnectDelay = 2000; // 2 seconds

    function init() {
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
        
        // Development features
        socket.on('hotReload', handleHotReload);
        
        // Configuration reload events
        socket.on('configurationReloaded', handleConfigurationReloaded);
        socket.on('configurationError', handleConfigurationError);

        // Handle connection errors
        socket.on('connect_error', handleConnectError);
        socket.on('reconnect', handleReconnect);
        socket.on('reconnect_error', handleReconnectError);
        socket.on('reconnect_failed', handleReconnectFailed);
    }

    function handleConnect() {
        isConnected = true;
        reconnectAttempts = 0;
        updateConnectionStatus('connected');
        
        // Request initial data
        socket.emit('requestData');
    }

    function handleDisconnect(reason) {
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


    function handleHotReload() {
        // Check if we're in development mode (hot reload is only enabled in dev)
        // Since this is client-side code, we can't access process.env directly
        const isDevelopment = window.location.hostname === 'localhost' || 
                             window.location.hostname === '127.0.0.1' ||
                             window.location.port === '7655';
        
        if (isDevelopment) {
            window.location.reload();
        }
    }
    
    function handleConfigurationReloaded(data) {
        // Show notification to user
        if (PulseApp.alerts && PulseApp.alerts.showNotification) {
            PulseApp.alerts.showNotification({
                message: 'Configuration has been updated and reloaded'
            });
        }
        
        // Request fresh data with new configuration
        socket.emit('requestData');
    }
    
    function handleConfigurationError(data) {
        console.error('[Socket] Configuration reload error:', data.error);
        
        // Show error notification to user
        if (PulseApp.alerts && PulseApp.alerts.showNotification) {
            PulseApp.alerts.showNotification({
                message: 'Failed to reload configuration: ' + data.error
            });
        }
    }

    function handleConnectError(error) {
        console.error('[Socket] Connection error:', error);
        updateConnectionStatus('error');
    }

    function handleReconnect() {
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
            // Capture scroll position of main table container before updates
            const mainTableContainer = document.querySelector('.table-container');
            let savedScrollTop = 0;
            let savedScrollLeft = 0;
            
            if (mainTableContainer) {
                savedScrollTop = mainTableContainer.scrollTop;
                savedScrollLeft = mainTableContainer.scrollLeft;
            }

            // Update node summary cards
            if (PulseApp.ui && PulseApp.ui.nodes && data.nodes) {
                PulseApp.ui.nodes.updateNodeSummaryCards(data.nodes);
            }

            // Update main dashboard
            if (PulseApp.ui && PulseApp.ui.dashboard) {
                PulseApp.ui.dashboard.updateDashboardTable();
            }
            
            // Restore scroll position after updates
            if (mainTableContainer && (savedScrollTop > 0 || savedScrollLeft > 0)) {
                const restoreScroll = () => {
                    mainTableContainer.scrollTop = savedScrollTop;
                    mainTableContainer.scrollLeft = savedScrollLeft;
                };
                
                // Multiple restoration attempts
                setTimeout(restoreScroll, 0);
                setTimeout(restoreScroll, 16);
                setTimeout(restoreScroll, 50);
                setTimeout(restoreScroll, 100);
                requestAnimationFrame(restoreScroll);
                
                // Final verification
                setTimeout(() => {
                    if (Math.abs(mainTableContainer.scrollTop - savedScrollTop) > 10) {
                        mainTableContainer.scrollTo({
                            top: savedScrollTop,
                            left: savedScrollLeft,
                            behavior: 'instant'
                        });
                    } else {
                    }
                }, 200);
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
            // Only update if backups tab is active and we have relevant data
            const backupsTab = document.querySelector('[data-tab="backups"]');
            const isBackupsTabActive = backupsTab && backupsTab.classList.contains('active');
            
            if (PulseApp.ui && PulseApp.ui.backups && (data.pbs || data.pveBackups) && isBackupsTabActive) {
                PulseApp.ui.backups.updateBackupsTab(false); // Mark as API update, not user action
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

