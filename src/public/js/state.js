PulseApp.state = (() => {
    const savedSortState = JSON.parse(localStorage.getItem('pulseSortState')) || {};
    const savedFilterState = JSON.parse(localStorage.getItem('pulseFilterState')) || {};
    const savedThresholdState = JSON.parse(localStorage.getItem('pulseThresholdState')) || {};

    let internalState = {
        nodesData: [],
        vmsData: [],
        containersData: [],
        metricsData: [],
        dashboardData: [],
        pbsDataArray: [],
        dashboardHistory: {},
        initialDataReceived: false,
        isThresholdRowVisible: false,
        groupByNode: savedFilterState.groupByNode ?? true,
        filterGuestType: savedFilterState.filterGuestType || 'all',
        filterStatus: savedFilterState.filterStatus || 'all',
        backupsFilterHealth: savedFilterState.backupsFilterHealth || 'all',
        backupsFilterGuestType: savedFilterState.backupsFilterGuestType || 'all',
        backupsSearchTerm: '',
        
        // Enhanced monitoring data
        alerts: {
            active: [],
            stats: {},
            rules: []
        },
        performance: {
            lastDiscoveryTime: null,
            lastMetricsTime: null,
            discoveryDuration: 0,
            metricsDuration: 0,
            errorCount: 0,
            successCount: 0,
            avgResponseTime: 0,
            peakMemoryUsage: 0
        },
        stats: {
            totalGuests: 0,
            runningGuests: 0,
            stoppedGuests: 0,
            totalNodes: 0,
            healthyNodes: 0,
            warningNodes: 0,
            errorNodes: 0,
            avgCpuUsage: 0,
            avgMemoryUsage: 0,
            avgDiskUsage: 0,
            lastUpdated: null
        },
        isConfigPlaceholder: false,
        
        sortState: {
            main: { column: 'id', direction: 'asc', ...(savedSortState.main || {}) },
            backups: { column: 'latestBackupTime', direction: 'desc', ...(savedSortState.backups || {}) }
        },
        thresholdState: {
            cpu: { value: 0 },
            memory: { value: 0 },
            disk: { value: 0 },
            diskread: { value: 0 },
            diskwrite:{ value: 0 },
            netin:    { value: 0 },
            netout:   { value: 0 }
        }
    };

    // Initialize thresholdState by merging saved state with defaults
    Object.keys(internalState.thresholdState).forEach(type => {
        const savedTypeState = savedThresholdState[type] || {};
        if (internalState.thresholdState[type].hasOwnProperty('operator')) {
            internalState.thresholdState[type] = {
                operator: savedTypeState.operator || '>=',
                input: savedTypeState.input || '',
                ...internalState.thresholdState[type],
                ...savedTypeState
            };
        } else {
            internalState.thresholdState[type] = {
                value: savedTypeState.value || 0,
                ...internalState.thresholdState[type],
                ...savedTypeState
            };
        }
    });

    function saveFilterState() {
        const stateToSave = {
            groupByNode: internalState.groupByNode,
            filterGuestType: internalState.filterGuestType,
            filterStatus: internalState.filterStatus,
            backupsFilterHealth: internalState.backupsFilterHealth,
            backupsFilterGuestType: internalState.backupsFilterGuestType
        };
        localStorage.setItem('pulseFilterState', JSON.stringify(stateToSave));
        localStorage.setItem('pulseThresholdState', JSON.stringify(internalState.thresholdState));
    }

    function saveSortState() {
        const stateToSave = {
            main: internalState.sortState.main,
            backups: internalState.sortState.backups
        };
        localStorage.setItem('pulseSortState', JSON.stringify(stateToSave));
    }

    function updateState(newData) {
        try {
            console.log('[State] Updating state with new data:', Object.keys(newData));
            
            // Update core data arrays
            if (newData.nodes) internalState.nodesData = newData.nodes;
            if (newData.vms) internalState.vmsData = newData.vms;
            if (newData.containers) internalState.containersData = newData.containers;
            if (newData.metrics) internalState.metricsData = newData.metrics;
            if (newData.pbs) internalState.pbsDataArray = newData.pbs;
            
            // Update enhanced monitoring data
            if (newData.alerts) {
                internalState.alerts = {
                    active: newData.alerts.active || [],
                    stats: newData.alerts.stats || {},
                    rules: newData.alerts.rules || []
                };
            }
            
            if (newData.performance) {
                internalState.performance = { ...internalState.performance, ...newData.performance };
            }
            
            if (newData.stats) {
                internalState.stats = { ...internalState.stats, ...newData.stats };
            }
            
            // Update configuration status
            if (newData.hasOwnProperty('isConfigPlaceholder')) {
                internalState.isConfigPlaceholder = newData.isConfigPlaceholder;
            }
            
            // Combine VMs and containers for dashboard
            internalState.dashboardData = [...internalState.vmsData, ...internalState.containersData];
            
            // Mark that we've received initial data
            if (!internalState.initialDataReceived && internalState.dashboardData.length > 0) {
                internalState.initialDataReceived = true;
                console.log('[State] Initial data received and processed');
            }
            
            // Update dashboard history for charts
            updateDashboardHistoryFromMetrics();
            
        } catch (error) {
            console.error('[State] Error updating state:', error);
        }
    }

    function updateDashboardHistoryFromMetrics() {
        try {
            internalState.metricsData.forEach(metric => {
                if (metric && metric.current) {
                    const guestId = `${metric.endpointId}-${metric.node}-${metric.id}`;
                    const dataPoint = {
                        timestamp: Date.now(),
                        cpu: metric.current.cpu * 100 || 0,
                        memory: metric.current.mem || 0,
                        disk: metric.current.disk || 0,
                        diskread: metric.current.diskread || 0,
                        diskwrite: metric.current.diskwrite || 0,
                        netin: metric.current.netin || 0,
                        netout: metric.current.netout || 0
                    };
                    
                    if (!internalState.dashboardHistory[guestId] || !Array.isArray(internalState.dashboardHistory[guestId])) {
                        internalState.dashboardHistory[guestId] = [];
                    }
                    
                    const history = internalState.dashboardHistory[guestId];
                    history.push(dataPoint);
                    if (history.length > PulseApp.config.AVERAGING_WINDOW_SIZE) {
                        history.shift();
                    }
                }
            });
        } catch (error) {
            console.error('[State] Error updating dashboard history:', error);
        }
    }

    return {
        get: (key) => internalState[key],
        set: (key, value) => {
            internalState[key] = value;
            if (['groupByNode', 'filterGuestType', 'filterStatus', 'backupsFilterHealth', 'backupsFilterGuestType', 'thresholdState'].includes(key)) {
                saveFilterState();
            }
        },
        
        // Enhanced state management
        updateState,
        getFullState: () => ({ ...internalState }),
        
        setSortState: (tableType, column, direction) => {
            if (internalState.sortState[tableType]) {
                internalState.sortState[tableType] = { column, direction };
                saveSortState();
            } else {
                console.warn(`Attempted to set sort state for unknown table type: ${tableType}`);
            }
        },
        getSortState: (tableType) => internalState.sortState[tableType],
        saveFilterState: saveFilterState,
        getThresholdState: () => internalState.thresholdState,
        setThresholdValue: (type, value) => {
            if (internalState.thresholdState[type]) {
                internalState.thresholdState[type].value = parseInt(value) || 0;
                saveFilterState();
            } else {
                console.warn(`Attempted to set threshold for unknown type: ${type}`);
            }
        },
        getDashboardHistory: () => internalState.dashboardHistory,
        updateDashboardHistory: (guestId, dataPoint) => {
            if (!internalState.dashboardHistory[guestId] || !Array.isArray(internalState.dashboardHistory[guestId])) {
                internalState.dashboardHistory[guestId] = [];
            }
            const history = internalState.dashboardHistory[guestId];
            history.push(dataPoint);
            if (history.length > PulseApp.config.AVERAGING_WINDOW_SIZE) history.shift();
        },
        clearDashboardHistoryEntry: (guestId) => {
            delete internalState.dashboardHistory[guestId];
        },
        forceRefreshDashboard: () => {
            if (PulseApp.socketHandler && typeof PulseApp.socketHandler.requestData === 'function') {
                PulseApp.socketHandler.requestData();
            }
        },
        
        // Alert and performance data getters
        getAlerts: () => internalState.alerts,
        getPerformance: () => internalState.performance,
        getStats: () => internalState.stats
    };
})();
