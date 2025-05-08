PulseApp.socketHandler = (() => {
    let socket = null;
    let uiUpdateCallback = () => { console.warn('[socketHandler] uiUpdateCallback not assigned.'); };
    let loadingOverlayCallback = () => { console.warn('[socketHandler] loadingOverlayCallback not assigned.'); };

    function init(updateFunctionRef, overlayUpdateRef) {
        socket = io();
        uiUpdateCallback = updateFunctionRef;
        loadingOverlayCallback = overlayUpdateRef;

        socket.on('connect', handleConnect);
        socket.on('disconnect', handleDisconnect);
        socket.on('initialState', handleInitialState);
        socket.on('rawData', handleRawData);
        socket.on('pbsInitialStatus', handlePbsInitialStatus);
        socket.on('hotReload', () => {
            console.log('[socketHandler] Hot reload requested. Reloading page...');
            window.location.reload();
        });

        // Optional: for debugging all events
        // socket.onAny((eventName, ...args) => {
        //     console.log(`[Socket Event Debug] Event: ${eventName}`, args);
        // });
    }

    function updateConnectionStatusUI(isConnected) {
        const connectionStatus = document.getElementById('connection-status');
        if (!connectionStatus) return;

        const statusText = isConnected ? 'Connected' : 'Disconnected';
        const addClasses = isConnected 
            ? ['connected', 'bg-green-100', 'dark:bg-green-800/30', 'text-green-700', 'dark:text-green-300']
            : ['disconnected', 'bg-red-100', 'dark:bg-red-800/30', 'text-red-700', 'dark:text-red-300'];
        const removeClasses = isConnected
            ? ['disconnected', 'bg-gray-200', 'dark:bg-gray-700', 'text-gray-600', 'dark:text-gray-400', 'bg-red-100', 'dark:bg-red-800/30', 'text-red-700', 'dark:text-red-300']
            : ['connected', 'bg-green-100', 'dark:bg-green-800/30', 'text-green-700', 'dark:text-green-300', 'bg-gray-200', 'dark:bg-gray-700', 'text-gray-600', 'dark:text-gray-400'];

        connectionStatus.textContent = statusText;
        connectionStatus.classList.remove(...removeClasses);
        connectionStatus.classList.add(...addClasses);
    }

    function handleConnect() {
        updateConnectionStatusUI(true);
        PulseApp.state.set('wasConnected', true);
        requestFullData(); // Request data on new connection or reconnection
        if (typeof loadingOverlayCallback === 'function') {
            loadingOverlayCallback(); // Update overlay based on current state
        }
    }

    function handleDisconnect(reason) {
        updateConnectionStatusUI(false);
        PulseApp.state.set('wasConnected', false);
        if (typeof loadingOverlayCallback === 'function') {
            loadingOverlayCallback(); // This should show the overlay with "Connection lost"
        }
    }

    function handleInitialState(state) {
        console.log('[socketHandler] Received initial state:', state);
        const isPlaceholder = state.isConfigPlaceholder || false;
        PulseApp.state.set('isConfigPlaceholder', isPlaceholder);
        PulseApp.state.set('initialDataReceived', false); // Mark that full data hasn't arrived yet

        const statusText = document.getElementById('dashboard-status-text');
        if (statusText) {
            statusText.textContent = isPlaceholder ? 'Configuration Required' : 'Loading initial data...';
        }
        
        if (typeof loadingOverlayCallback === 'function') {
            loadingOverlayCallback(); // Update overlay based on placeholder status and lack of data
        }
    }

    function handleRawData(jsonData) {
        try {
            const data = typeof jsonData === 'string' ? JSON.parse(jsonData) : jsonData;
            
            PulseApp.state.set('isConfigPlaceholder', data.isConfigPlaceholder || false);
            PulseApp.state.set('nodesData', data.nodes || []);
            PulseApp.state.set('vmsData', data.vms || []);
            PulseApp.state.set('containersData', data.containers || []);
            PulseApp.state.set('metricsData', data.metrics || []); // Ensure metrics are always set
            PulseApp.state.set('pbsDataArray', Array.isArray(data.pbs) ? data.pbs : []);

            if (PulseApp.ui?.tabs) {
                PulseApp.ui.tabs.updateTabAvailability();
            }

            // Set initialDataReceived to true only after successfully processing raw data
            PulseApp.state.set('initialDataReceived', true); 

            if (typeof loadingOverlayCallback === 'function') {
                loadingOverlayCallback(); // Hide overlay if not placeholder and data received
            }

            if (typeof uiUpdateCallback === 'function') {
                uiUpdateCallback();
            } else {
                 console.error('[socketHandler] uiUpdateCallback is not a function!');
            }
        } catch (e) {
            console.error('Error processing received rawData:', e, jsonData);
        }
    }

    function handlePbsInitialStatus(pbsStatusArray) {
        if (Array.isArray(pbsStatusArray)) {
            const initialPbsData = pbsStatusArray.map(statusInfo => ({
                ...statusInfo, // Spread existing status info
                // Initialize other fields if they might be missing from statusInfo
                backupTasks: statusInfo.backupTasks || { recentTasks: [], summary: {} },
                datastores: statusInfo.datastores || [],
                verificationTasks: statusInfo.verificationTasks || { summary: {} },
                syncTasks: statusInfo.syncTasks || { summary: {} },
                pruneTasks: statusInfo.pruneTasks || { summary: {} },
                nodeName: statusInfo.nodeName || null 
            }));
            PulseApp.state.set('pbsDataArray', initialPbsData);
            // Don't set initialDataReceived here; wait for full rawData

            if (PulseApp.ui?.pbs) {
                PulseApp.ui.pbs.updatePbsInfo(initialPbsData);
            }
            if (PulseApp.ui?.tabs) {
                PulseApp.ui.tabs.updateTabAvailability();
            }
            if (typeof loadingOverlayCallback === 'function') {
                loadingOverlayCallback(); // Update overlay, might still show "Loading..."
            }
        } else {
            console.warn('[socket] Received non-array data for pbsInitialStatus:', pbsStatusArray);
        }
    }

    function requestFullData() {
        console.log('Requesting full data reload from server...');
        if (socket && socket.connected) { // Check if socket is connected before emitting
            socket.emit('requestData');
        } else {
            console.error('Cannot request data: socket not initialized or not connected.');
        }
    }

    function isConnected() {
        return socket && socket.connected;
    }

    return {
        init,
        requestFullData,
        isConnected
    };
})();
