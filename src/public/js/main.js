document.addEventListener('DOMContentLoaded', function() {
    const PulseApp = window.PulseApp || {};

    function updateAllUITables() {
        if (!PulseApp.state || !PulseApp.state.get('initialDataReceived')) {
            return;
        }
        
        const nodesData = PulseApp.state.get('nodesData');
        const pbsDataArray = PulseApp.state.get('pbsDataArray');

        PulseApp.ui.nodes?.updateNodesTable(nodesData);
        PulseApp.ui.dashboard?.updateDashboardTable();
        PulseApp.ui.storage?.updateStorageInfo();
        PulseApp.ui.pbs?.updatePbsInfo(pbsDataArray);
        PulseApp.ui.backups?.updateBackupsTab();

        const loadingOverlay = document.getElementById('loading-overlay');
        if (loadingOverlay && loadingOverlay.style.display !== 'none') {
            if (PulseApp.socketHandler?.isConnected()) {
                console.log('[UI Update] First data received, hiding loading overlay.');
                loadingOverlay.style.display = 'none';
            } else {
                 console.log('[UI Update] Data received, but socket disconnected. Keeping loading overlay.');
            }
        }

        PulseApp.thresholds?.logging?.checkThresholdViolations();
    }

    function initializeModules() {
        PulseApp.state?.init?.();
        PulseApp.config?.init?.();
        PulseApp.utils?.init?.();
        PulseApp.theme?.init?.();
        PulseApp.socketHandler?.init?.(updateAllUITables);
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