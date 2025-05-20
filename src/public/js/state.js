PulseApp.state = (() => {
    const savedSortState = JSON.parse(localStorage.getItem('pulseSortState')) || {};
    const savedFilterState = JSON.parse(localStorage.getItem('pulseFilterState')) || {};
    const savedThresholdState = JSON.parse(localStorage.getItem('pulseThresholdState')) || {};
    const savedPbsShowDetails = JSON.parse(localStorage.getItem('pulsePbsShowDetails')) || {};

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
        sortState: {
            nodes: { column: null, direction: 'asc', ...(savedSortState.nodes || {}) },
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
        },
        activeLogSessions: {},
        thresholdLogEntries: [],
        activeLoggingThresholds: null,
        pbsShowDetails: savedPbsShowDetails
    };

    // Initialize thresholdState by merging saved state with defaults
    Object.keys(internalState.thresholdState).forEach(type => {
        const savedTypeState = savedThresholdState[type] || {};
        if (internalState.thresholdState[type].hasOwnProperty('operator')) { // Assuming this structure means it's the advanced threshold type
            internalState.thresholdState[type] = {
                operator: savedTypeState.operator || '>=',
                input: savedTypeState.input || '',
                // Preserve any other default properties if they exist
                ...internalState.thresholdState[type],
                ...savedTypeState // This ensures saved values overwrite defaults but keeps other default props
            };
        } else { // Simple value threshold
            internalState.thresholdState[type] = {
                value: savedTypeState.value || 0,
                // Preserve any other default properties
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
            nodes: internalState.sortState.nodes,
            main: internalState.sortState.main,
            backups: internalState.sortState.backups
        };
        localStorage.setItem('pulseSortState', JSON.stringify(stateToSave));
    }

    function savePbsShowDetailsState() {
        localStorage.setItem('pulsePbsShowDetails', JSON.stringify(internalState.pbsShowDetails));
    }

    return {
        get: (key) => internalState[key],
        set: (key, value) => {
            internalState[key] = value;
            if (['groupByNode', 'filterGuestType', 'filterStatus', 'backupsFilterHealth', 'backupsFilterGuestType', 'thresholdState'].includes(key)) {
                saveFilterState();
            }
        },
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
        getActiveLogSession: (sessionId) => internalState.activeLogSessions[sessionId],
        getAllActiveLogSessions: () => internalState.activeLogSessions,
        addActiveLogSession: (sessionId, sessionData) => {
            internalState.activeLogSessions[sessionId] = sessionData;
        },
        removeActiveLogSession: (sessionId) => {
            delete internalState.activeLogSessions[sessionId];
        },
        addLogEntry: (sessionId, entry) => {
            if (internalState.activeLogSessions[sessionId]) {
                internalState.activeLogSessions[sessionId].entries.push(entry);
            } else {
                console.warn(`Attempted to add log entry to non-existent session: ${sessionId}`);
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
        getPbShowDetailsState: (instanceId, defaultValue) => {
            if (typeof internalState.pbsShowDetails[instanceId] === 'boolean') {
                return internalState.pbsShowDetails[instanceId];
            }
            return defaultValue;
        },
        setPbShowDetailsState: (instanceId, value) => {
            internalState.pbsShowDetails[instanceId] = !!value;
            savePbsShowDetailsState();
        }
    };
})();
