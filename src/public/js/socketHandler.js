PulseApp.socketHandler = (() => {
    let socket = null;

    // Expose the UI update function reference
    let updateAllUITablesRef = () => { 
        console.warn('[socketHandler] updateAllUITablesRef is not yet assigned.');
    };

    function init(updateFunctionRef) {
        socket = io();
        updateAllUITablesRef = updateFunctionRef; // Assign the function passed from main.js

        socket.on('connect', handleConnect);
        socket.on('disconnect', handleDisconnect);
        socket.on('initialState', handleInitialState);
        socket.on('rawData', handleRawData);
        socket.on('pbsInitialStatus', handlePbsInitialStatus);

        socket.onAny((eventName, ...args) => {
        });
    }

    function handleConnect() {
        const connectionStatus = document.getElementById('connection-status');
        if (connectionStatus) {
            connectionStatus.textContent = 'Connected';
            connectionStatus.classList.remove('disconnected', 'bg-gray-200', 'dark:bg-gray-700', 'text-gray-600', 'dark:text-gray-400', 'bg-red-100', 'dark:bg-red-800/30', 'text-red-700', 'dark:text-red-300');
            connectionStatus.classList.add('connected', 'bg-green-100', 'dark:bg-green-800/30', 'text-green-700', 'dark:text-green-300');
        }
        requestFullData();
    }

    function handleDisconnect(reason) {
        const connectionStatus = document.getElementById('connection-status');
        if (connectionStatus) {
            connectionStatus.textContent = 'Disconnected';
            connectionStatus.classList.remove('connected', 'bg-green-100', 'dark:bg-green-800/30', 'text-green-700', 'dark:text-green-300');
            connectionStatus.classList.add('disconnected', 'bg-red-100', 'dark:bg-red-800/30', 'text-red-700', 'dark:text-red-300');
        }
        PulseApp.state.set('wasConnected', false);

        const loadingOverlay = document.getElementById('loading-overlay');
        if (loadingOverlay) {
          const loadingText = loadingOverlay.querySelector('p');
          if (loadingText) {
              loadingText.textContent = 'Connection lost.';

          }
          loadingOverlay.style.display = 'flex';
        }
    }

    function handleInitialState(state) {
        console.log('[socketHandler] Received initial state:', state);
        if (state && state.loading) {
             // Update status text to indicate loading
             const statusText = document.getElementById('dashboard-status-text');
             if (statusText) {
                 statusText.textContent = 'Loading initial data...';
             }
             // Ensure loading overlay is visible
            const loadingOverlay = document.getElementById('loading-overlay');
            if (loadingOverlay && loadingOverlay.style.display === 'none') {
                const loadingText = loadingOverlay.querySelector('p');
                if (loadingText) {
                    loadingText.textContent = 'Loading data...'; // Or a specific initial loading message
                }
                 loadingOverlay.style.display = 'flex';
            }
        }
    }

    function handleRawData(jsonData) {
        try {
            const data = typeof jsonData === 'string' ? JSON.parse(jsonData) : jsonData;

            PulseApp.state.set('nodesData', data.nodes || []);
            PulseApp.state.set('vmsData', data.vms || []);
            PulseApp.state.set('containersData', data.containers || []);

            if (data.hasOwnProperty('metrics')) {
                 PulseApp.state.set('metricsData', data.metrics || []);
            } else {
            }

            if (data.hasOwnProperty('pbs')) {
                PulseApp.state.set('pbsDataArray', Array.isArray(data.pbs) ? data.pbs : []);
            } else {
            }

            if (PulseApp.ui && PulseApp.ui.tabs) {
                PulseApp.ui.tabs.updateTabAvailability();
            } else {
                console.warn('[socketHandler] PulseApp.ui.tabs not available for updateTabAvailability');
            }


            if (!PulseApp.state.get('initialDataReceived')) {
              PulseApp.state.set('initialDataReceived', true);

            }
            
            // --- Trigger UI update after processing data --- 
            if (typeof updateAllUITablesRef === 'function') {
                updateAllUITablesRef();
            } else {
                 console.error('[socketHandler] updateAllUITablesRef is not a function!');
            }
            // --- END Trigger ---

        } catch (e) {
            console.error('Error processing received rawData:', e, jsonData);
        }
    }

    function handlePbsInitialStatus(pbsStatusArray) {
        if (Array.isArray(pbsStatusArray)) {
            const initialPbsData = pbsStatusArray.map(statusInfo => ({
                ...statusInfo,
                backupTasks: { recentTasks: [], summary: {} },
                datastores: [],
                verificationTasks: { summary: {} },
                syncTasks: { summary: {} },
                pruneTasks: { summary: {} },
                nodeName: null
            }));
            PulseApp.state.set('pbsDataArray', initialPbsData);

            if (PulseApp.ui && PulseApp.ui.pbs) {
                PulseApp.ui.pbs.updatePbsInfo(initialPbsData);
            } else {
                console.warn('[socketHandler] PulseApp.ui.pbs not available for updatePbsInfo');
            }
            if (PulseApp.ui && PulseApp.ui.tabs) {
                PulseApp.ui.tabs.updateTabAvailability();
            } else {
                console.warn('[socketHandler] PulseApp.ui.tabs not available for updateTabAvailability');
            }
        } else {
            console.warn('[socket] Received non-array data for pbsInitialStatus:', pbsStatusArray);
        }
    }

    function requestFullData() {
        console.log('Requesting full data reload from server...');
        const loadingOverlay = document.getElementById('loading-overlay');
        if (loadingOverlay) {
          const loadingText = loadingOverlay.querySelector('p');
          if (loadingText) {
              loadingText.textContent = 'Connected. Reloading data...';
          }
          loadingOverlay.style.display = 'flex';
        }
        if (socket) {
            socket.emit('requestData');
        } else {
            console.error('Cannot request data: socket not initialized.');
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