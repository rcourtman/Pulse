document.addEventListener('DOMContentLoaded', function() {
    const PulseApp = window.PulseApp || {};

    function initializeModules() {
        PulseApp.state?.init?.(); // Although state is IIFE, init might be added later
        PulseApp.config?.init?.(); // Although config is obj literal, init might be added later
        PulseApp.utils?.init?.(); // Although utils is obj literal, init might be added later
        PulseApp.theme?.init?.();
        PulseApp.socketHandler?.init?.();
        PulseApp.tooltips?.init?.();

        PulseApp.ui = PulseApp.ui || {};
        PulseApp.ui.tabs?.init?.();
        PulseApp.ui.nodes?.init?.();
        PulseApp.ui.dashboard?.init?.();
        PulseApp.ui.storage?.init?.();
        PulseApp.ui.pbs?.initPbsEventListeners?.(); // Specific init for PBS listeners
        PulseApp.ui.backups?.init?.();
        PulseApp.ui.thresholds?.init?.();
        PulseApp.ui.common?.init?.();

        PulseApp.thresholds = PulseApp.thresholds || {};
        PulseApp.thresholds.logging?.init?.();
    }

    function validateCriticalElements() {
        const criticalElements = [
            'connection-status',
            'main-table', // Check for table itself
            // 'custom-tooltip', // Non-critical
            // 'slider-value-tooltip', // Non-critical
            'dashboard-search', // Optional but important
            'dashboard-status-text', // Status display
            'app-version' // Version display
        ];
        let allFound = true;
        criticalElements.forEach(id => {
            if (!document.getElementById(id)) {
                console.error(`Critical element #${id} not found!`);
                // allFound = false; // Decide if you want to stop execution
            }
        });
        // Check for table body specifically needed by dashboard updates
        if (!document.querySelector('#main-table tbody')) {
             console.error('Critical element #main-table tbody not found!');
             // allFound = false;
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

    function updateAllUITables() {
        const nodesData = PulseApp.state.get('nodesData');
        const pbsDataArray = PulseApp.state.get('pbsDataArray');
        const storageData = PulseApp.state.get('storageData');

        PulseApp.ui.nodes?.updateNodesTable(nodesData);
        PulseApp.ui.dashboard?.updateDashboardTable(); // Refreshes data internally
        PulseApp.ui.storage?.updateStorageInfo(); // Uses state internaly
        PulseApp.ui.pbs?.updatePbsInfo(pbsDataArray);
        PulseApp.ui.backups?.updateBackupsTab();

        const loadingOverlay = document.getElementById('loading-overlay');
        if (loadingOverlay && loadingOverlay.style.display !== 'none') {
            if (PulseApp.socketHandler.isConnected()) {
                console.log('[UI Update] Hiding loading overlay.');
                loadingOverlay.style.display = 'none';
            } else {
            }
        }
    }

    // --- Main Execution --- 
    if (!validateCriticalElements()) {
        console.error("Stopping JS execution due to missing critical elements.");
        // Optionally display a user-facing error message here
        return;
    }

    initializeModules();
    fetchVersion();
    PulseApp.ui.storage.fetchStorageData(); // Initial fetch

    setInterval(() => {
        if (PulseApp.state.get('initialDataReceived')) {
            updateAllUITables();
            PulseApp.thresholds.logging?.checkThresholdViolations();
        }
    }, 2000); // UI update interval

    setInterval(PulseApp.ui.storage.fetchStorageData, 30000); // Storage fetch interval
}); 