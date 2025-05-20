document.addEventListener('DOMContentLoaded', function() {
    const PulseApp = window.PulseApp || {};

    function updateAllUITables() {
        if (!PulseApp.state || !PulseApp.state.get('initialDataReceived')) {
            return;
        }
        
        const nodesData = PulseApp.state.get('nodesData');
        const pbsDataArray = PulseApp.state.get('pbsDataArray');

        // PulseApp.ui.nodes?.updateNodesTable(nodesData); // REMOVED - Nodes table is gone
        PulseApp.ui.nodes?.updateNodeSummaryCards(nodesData);
        PulseApp.ui.dashboard?.updateDashboardTable();
        PulseApp.ui.storage?.updateStorageInfo();
        PulseApp.ui.pbs?.updatePbsInfo(pbsDataArray);
        PulseApp.ui.backups?.updateBackupsTab();

        updateLoadingOverlayVisibility(); // Call the helper function

        PulseApp.thresholds?.logging?.checkThresholdViolations();
    }

    function updateLoadingOverlayVisibility() {
        const loadingOverlay = document.getElementById('loading-overlay');
        if (!loadingOverlay) return;

        const isConnected = PulseApp.socketHandler?.isConnected();
        const initialDataReceived = PulseApp.state?.get('initialDataReceived');

        if (loadingOverlay.style.display !== 'none') { // Only act if currently visible
            if (isConnected && initialDataReceived) {
                console.log('[UI Update] Data received and socket connected, hiding loading overlay.');
                loadingOverlay.style.display = 'none';
            } else if (!isConnected) {
                console.log('[UI Update] Socket disconnected. Keeping loading overlay visible.');
            }
            // If initialDataReceived is false, or socket is connected but no data yet, overlay remains.
        } else if (!isConnected && loadingOverlay.style.display === 'none') {
            // If overlay is hidden but socket disconnects, re-show it.
            console.log('[UI Update] Socket disconnected and overlay was hidden. Re-showing loading overlay.');
            loadingOverlay.style.display = 'flex'; // Or 'block', or its original display type
        }
    }

    function initializeModules() {
        PulseApp.state?.init?.();
        PulseApp.config?.init?.();
        PulseApp.utils?.init?.();
        PulseApp.theme?.init?.();
        // Ensure socketHandler.init receives both callbacks if it's designed to accept them
        // If socketHandler.init only expects one, this might need adjustment in socketHandler.js
        PulseApp.socketHandler?.init?.(updateAllUITables, updateLoadingOverlayVisibility); 
        PulseApp.tooltips?.init?.();

        PulseApp.ui = PulseApp.ui || {};
        PulseApp.ui.tabs?.init?.();
        PulseApp.ui.nodes?.init?.();
        PulseApp.ui.dashboard?.init?.();
        PulseApp.ui.storage?.init?.();
        PulseApp.ui.pbs?.initPbsEventListeners?.();
        PulseApp.ui.backups?.init?.();
        PulseApp.ui.thresholds?.init?.();
        PulseApp.ui.common?.init?.();

        PulseApp.thresholds = PulseApp.thresholds || {};
        PulseApp.thresholds.logging?.init?.();
    }

    function validateCriticalElements() {
        const criticalElements = [
            'connection-status',
            'main-table',
            'dashboard-search',
            'dashboard-status-text',
            'app-version'
        ];
        let allFound = true;
        criticalElements.forEach(id => {
            if (!document.getElementById(id)) {
                console.error(`Critical element #${id} not found!`);
            }
        });
        if (!document.querySelector('#main-table tbody')) {
             console.error('Critical element #main-table tbody not found!');
        }
        return allFound;
    }

    function fetchVersion() {
        const versionSpan = document.getElementById('app-version');
        fetch('/api/version')
            .then(response => response.json())
            .then(data => {
                if (versionSpan && data.version) {
                    versionSpan.textContent = data.version;
                }
            })
            .catch(error => {
                console.error('Error fetching version:', error);
                if (versionSpan) {
                    versionSpan.textContent = 'error';
                }
            });
    }

    if (!validateCriticalElements()) {
        console.error("Stopping JS execution due to missing critical elements.");
        return;
    }

    initializeModules();
    fetchVersion();
});
