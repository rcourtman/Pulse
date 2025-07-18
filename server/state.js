const AlertManager = require('./alertManager');

const state = {
  nodes: [],
  vms: [],
  containers: [],
  metrics: [],
  pbs: [], // Array to hold data for each PBS instance
  pveBackups: { // Add PVE backup data
    backupTasks: [],
    storageBackups: [],
    guestSnapshots: []
  },
  isConfigPlaceholder: false, // Add this flag
  endpoints: [], // Add endpoint configurations for client
  pbsConfigs: [], // Add PBS configurations for client
  
  // Enhanced monitoring data
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
  
  // Connection health tracking
  connections: new Map(), // endpointId -> { status, lastSeen, errorCount, responseTime }
  
  // Runtime statistics
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
  
  // These were pbsInstances, allPbsTasks, aggregatedPbsTaskSummary.
  // Task-related data might be part of each PBS instance object in state.pbs,
  // or handled separately if needed by the client for global views.
  allPbsTasks: [], 
  aggregatedPbsTaskSummary: {}
};

// Initialize alert manager
const alertManager = new AlertManager();

// Track performance metrics
let performanceHistory = [];
const MAX_PERFORMANCE_HISTORY = 288; // 24 hours at 5-minute intervals

function init() {
  console.log('[State Manager] Initializing enhanced state management...');
  
  // Set up alert event handlers
  alertManager.on('alert', (alert) => {
    // Could emit to websockets, send notifications, etc.
    updateAlertStats();
  });
  
  alertManager.on('alertResolved', (alert) => {
    updateAlertStats();
  });
  
  // Initialize performance tracking
  state.performance.lastDiscoveryTime = Date.now();
  state.performance.lastMetricsTime = Date.now();
  
  console.log('[State Manager] Enhanced state management initialized');
}

function getState() {
  // Return a comprehensive state object
  return {
    nodes: state.nodes,
    vms: state.vms,
    containers: state.containers,
    metrics: state.metrics, // Assuming metrics are updated elsewhere
    pbs: state.pbs, // This is what's sent to the client and should now be correct
    pveBackups: state.pveBackups, // Add PVE backup data
    isConfigPlaceholder: state.isConfigPlaceholder,
    endpoints: state.endpoints, // Add endpoint configurations
    pbsConfigs: state.pbsConfigs, // Add PBS configurations
    
    // Enhanced monitoring data
    performance: state.performance,
    connections: Object.fromEntries(state.connections), // Convert Map to object for serialization
    stats: state.stats,
    
    // PBS specific data structures - these might be derived or directly set
    // If they are part of each item in state.pbs, the client can aggregate them.
    // If they are global summaries, ensure they are updated correctly in updateDiscoveryData.
    allPbsTasks: state.allPbsTasks, 
    aggregatedPbsTaskSummary: state.aggregatedPbsTaskSummary,
    
    // Alerts
    alerts: getAlertInfo(), // Corrected: Call the local getAlertInfo function
    
    // Custom Thresholds
    customThresholds: getCustomThresholds()
  };
}

function updateGuestUptimeFromMetrics(metrics) {
  // Update uptime for VMs and containers based on metrics data
  if (!metrics || metrics.length === 0) return;
  
  metrics.forEach(metric => {
    if (metric.current && metric.current.uptime !== undefined) {
      // Find and update VM
      const vmIndex = state.vms.findIndex(vm => 
        vm.vmid === metric.id && 
        vm.node === metric.node && 
        vm.endpointId === metric.endpointId
      );
      
      if (vmIndex !== -1) {
        state.vms[vmIndex].uptime = metric.current.uptime;
        console.log(`[State Manager] Updated VM ${metric.id} uptime to ${metric.current.uptime} seconds`);
      } else {
        // Try containers
        const ctIndex = state.containers.findIndex(ct => 
          ct.vmid === metric.id && 
          ct.node === metric.node && 
          ct.endpointId === metric.endpointId
        );
        
        if (ctIndex !== -1) {
          state.containers[ctIndex].uptime = metric.current.uptime;
          console.log(`[State Manager] Updated container ${metric.id} uptime to ${metric.current.uptime} seconds`);
        }
      }
    }
  });
}

function updateDiscoveryData({ nodes, vms, containers, pbs, pveBackups, allPbsTasks, aggregatedPbsTaskSummary }, duration = 0, errors = []) {
  const startTime = Date.now();
  
  try {
    // Update main data
    state.nodes = nodes || [];
    state.vms = vms || [];
    state.containers = containers || [];
    state.pbs = pbs || [];
    
    // Update PVE backup data
    if (pveBackups) {
      state.pveBackups = {
        backupTasks: pveBackups.backupTasks || [],
        storageBackups: pveBackups.storageBackups || [],
        guestSnapshots: pveBackups.guestSnapshots || []
      };
    }
    
    // If the discovery data structure nests these under the main 'pbs' array (e.g., from fetchPbsData),
    // they might not be separate top-level items in the discoveryData object passed here.
    // If they are indeed separate, this update is fine.
    // If they are part of the main 'pbs' array items, this might be redundant or need adjustment
    // based on how fetchDiscoveryData structures the final object.
    if (allPbsTasks !== undefined) { // Check for presence before assigning
        state.allPbsTasks = allPbsTasks;
    }
    if (aggregatedPbsTaskSummary !== undefined) { // Check for presence before assigning
        state.aggregatedPbsTaskSummary = aggregatedPbsTaskSummary;
    }
    
    // Update performance metrics
    state.performance.lastDiscoveryTime = startTime;
    state.performance.discoveryDuration = duration;
    
    if (errors.length > 0) {
      state.performance.errorCount += errors.length;
      updateConnectionHealth(errors);
    } else {
      state.performance.successCount++;
    }
    
    // Calculate runtime statistics
    calculateRuntimeStats();
    
    // Track performance history
    addPerformanceSnapshot('discovery', duration, errors.length);
    
    console.log(`[State Manager] Discovery update completed. Duration: ${duration}ms, Errors: ${errors.length}`);
    
  } catch (error) {
    console.error('[State Manager] Error updating discovery data:', error);
    state.performance.errorCount++;
  }
}

function updateMetricsData(metrics, duration = 0, errors = []) {
  const startTime = Date.now();
  
  try {
    state.metrics = metrics || [];
    
    // Update uptime for VMs and containers from metrics data
    updateGuestUptimeFromMetrics(metrics);
    
    // Update performance metrics
    state.performance.lastMetricsTime = startTime;
    state.performance.metricsDuration = duration;
    
    if (errors.length > 0) {
      state.performance.errorCount += errors.length;
      updateConnectionHealth(errors);
    } else {
      state.performance.successCount++;
    }
    
    // Calculate average response time
    updateAverageResponseTime(duration);
    
    // Check metrics against alert rules
    checkAlertsForMetrics();
    
    // Update runtime statistics with latest metrics
    calculateRuntimeStats();
    
    // Track performance history
    addPerformanceSnapshot('metrics', duration, errors.length);
    
    console.log(`[State Manager] Metrics update completed. Duration: ${duration}ms, Metrics: ${metrics.length}, Errors: ${errors.length}`);
    
  } catch (error) {
    console.error('[State Manager] Error updating metrics data:', error);
    state.performance.errorCount++;
  }
}

function calculateRuntimeStats() {
  try {
    const allGuests = [...state.vms, ...state.containers];
    
    // Guest statistics
    state.stats.totalGuests = allGuests.length;
    state.stats.runningGuests = allGuests.filter(g => g.status === 'running').length;
    state.stats.stoppedGuests = allGuests.filter(g => g.status === 'stopped').length;
    
    // Node statistics
    state.stats.totalNodes = state.nodes.length;
    state.stats.healthyNodes = state.nodes.filter(n => n.status === 'online').length;
    state.stats.warningNodes = state.nodes.filter(n => 
      n.status === 'online' && (
        (n.cpu && n.cpu > 80) || 
        (n.mem && n.maxmem && (n.mem / n.maxmem) > 0.9)
      )
    ).length;
    state.stats.errorNodes = state.nodes.filter(n => n.status !== 'online').length;
    
    // Average usage calculations
    const runningGuests = allGuests.filter(g => g.status === 'running');
    if (runningGuests.length > 0 && state.metrics.length > 0) {
      let totalCpu = 0, totalMemory = 0, totalDisk = 0, count = 0;
      
      state.metrics.forEach(metric => {
        if (metric.current) {
          const guest = runningGuests.find(g => 
            g.vmid === metric.id && g.node === metric.node && g.endpointId === metric.endpointId
          );
          if (guest) {
            if (metric.current.cpu !== undefined) {
              totalCpu += metric.current.cpu * 100;
            }
            if (metric.current.mem !== undefined && guest.maxmem) {
              totalMemory += (metric.current.mem / guest.maxmem) * 100;
            }
            if (metric.current.disk !== undefined && guest.maxdisk) {
              totalDisk += (metric.current.disk / guest.maxdisk) * 100;
            }
            count++;
          }
        }
      });
      
      if (count > 0) {
        state.stats.avgCpuUsage = Math.round(totalCpu / count);
        state.stats.avgMemoryUsage = Math.round(totalMemory / count);
        state.stats.avgDiskUsage = Math.round(totalDisk / count);
      }
    }
    
    state.stats.lastUpdated = Date.now();
    
  } catch (error) {
    console.error('[State Manager] Error calculating runtime stats:', error);
  }
}

function updateConnectionHealth(errors) {
  errors.forEach(error => {
    const endpointId = error.endpointId || 'unknown';
    const connection = state.connections.get(endpointId) || {
      status: 'healthy',
      lastSeen: Date.now(),
      errorCount: 0,
      responseTime: 0
    };
    
    connection.errorCount++;
    connection.lastSeen = Date.now();
    connection.status = connection.errorCount > 5 ? 'error' : 'warning';
    
    state.connections.set(endpointId, connection);
  });
}

function updateAverageResponseTime(newDuration) {
  const alpha = 0.1; // Exponential moving average factor
  if (state.performance.avgResponseTime === 0) {
    state.performance.avgResponseTime = newDuration;
  } else {
    state.performance.avgResponseTime = (alpha * newDuration) + ((1 - alpha) * state.performance.avgResponseTime);
  }
}

function addPerformanceSnapshot(type, duration, errorCount) {
  const snapshot = {
    timestamp: Date.now(),
    type,
    duration,
    errorCount,
    memoryUsage: process.memoryUsage().heapUsed,
    guestCount: state.stats.totalGuests,
    nodeCount: state.stats.totalNodes
  };
  
  performanceHistory.unshift(snapshot);
  if (performanceHistory.length > MAX_PERFORMANCE_HISTORY) {
    performanceHistory = performanceHistory.slice(0, MAX_PERFORMANCE_HISTORY);
  }
  
  // Update peak memory usage
  if (snapshot.memoryUsage > state.performance.peakMemoryUsage) {
    state.performance.peakMemoryUsage = snapshot.memoryUsage;
  }
}

async function checkAlertsForMetrics() {
  try {
    const allGuests = [...state.vms, ...state.containers];
    console.log(`[State Manager] Checking alerts for ${allGuests.length} guests`);
    
    // Write to debug file
    const fs = require('fs');
    fs.appendFileSync('/opt/pulse/state-debug.log', `[${new Date().toISOString()}] checkAlertsForMetrics called with ${allGuests.length} guests\n`);
    
    // Log a sample of guest data to see what we're working with
    if (allGuests.length > 0) {
      const sampleGuest = allGuests[0];
      console.log(`[State Manager] Sample guest data:`, {
        name: sampleGuest.name,
        disk: sampleGuest.disk,
        maxdisk: sampleGuest.maxdisk,
        status: sampleGuest.status
      });
      fs.appendFileSync('/opt/pulse/state-debug.log', `[${new Date().toISOString()}] Sample: ${sampleGuest.name} disk=${sampleGuest.disk} maxdisk=${sampleGuest.maxdisk}\n`);
    }
    
    await alertManager.checkMetrics(allGuests, state.metrics);
    
    // Also check node alerts if we have per-guest threshold rule with node thresholds
    const perGuestRule = alertManager.getRules().find(r => r.type === 'per_guest_thresholds' && r.enabled);
    if (perGuestRule && (perGuestRule.nodeThresholds || perGuestRule.globalNodeThresholds)) {
      await alertManager.checkNodeMetrics(state.nodes, perGuestRule.nodeThresholds, perGuestRule.globalNodeThresholds);
    }
  } catch (error) {
    console.error('[State Manager] Error checking alerts:', error);
  }
}

function updateAlertStats() {
  // This could update dashboard counters, etc.
  const alertStats = alertManager.getEnhancedAlertStats();
  state.stats.activeAlerts = alertStats.active;
  state.stats.alertsLast24h = alertStats.last24Hours;
}

function getAlertInfo() {
  try {
    const alertInfo = {
      active: alertManager.getActiveAlerts(),
      stats: alertManager.getEnhancedAlertStats(),
      rules: alertManager.getRules()
    };
    
    // Test serialization to catch any circular references early
    JSON.stringify(alertInfo);
    return alertInfo;
  } catch (error) {
    console.error('[State Manager] Error serializing alert info:', error.message);
    
    // Try to identify which part is causing the circular reference
    try {
      JSON.stringify(alertManager.getActiveAlerts());
      console.log('[State Manager] Active alerts serialization: OK');
    } catch (activeError) {
      console.error('[State Manager] Active alerts circular reference detected');
    }
    
    try {
      JSON.stringify(alertManager.getEnhancedAlertStats());
      console.log('[State Manager] Alert stats serialization: OK');
    } catch (statsError) {
      console.error('[State Manager] Alert stats circular reference detected');
    }
    
    try {
      JSON.stringify(alertManager.getRules());
      console.log('[State Manager] Alert rules serialization: OK');
    } catch (rulesError) {
      console.error('[State Manager] Alert rules circular reference detected');
    }
    
    // Return safe empty alert data if serialization fails
    return {
      active: [],
      stats: {
        active: 0,
        acknowledged: 0,
        last24Hours: 0,
        lastHour: 0,
        totalRules: 0,
        suppressedRules: 0,
        metrics: { totalFired: 0, totalResolved: 0, totalAcknowledged: 0, averageResolutionTime: 0, falsePositiveRate: 0 },
        groups: []
      },
      rules: []
    };
  }
}

function getCustomThresholds() {
  try {
    const customThresholdManager = require('./customThresholds');
    return customThresholdManager.getAllThresholds();
  } catch (error) {
    console.error('[State Manager] Error getting custom thresholds:', error);
    return [];
  }
}

function clearMetricsData() {
  state.metrics = [];
  console.log('[State Manager] Metrics data cleared');
}

function hasData() {
  return state.nodes.length > 0 || state.vms.length > 0 || state.containers.length > 0 || state.pbs.length > 0;
}

function setConfigPlaceholderStatus(isPlaceholder) {
  state.isConfigPlaceholder = isPlaceholder;
}

function setEndpointConfigurations(endpoints, pbsConfigs) {
  // Store endpoint configurations for client use
  state.endpoints = endpoints.map(endpoint => ({
    id: endpoint.id,
    name: endpoint.name,
    host: endpoint.host,
    port: endpoint.port,
    enabled: endpoint.enabled
  }));
  
  state.pbsConfigs = pbsConfigs.map(config => ({
    id: config.id,
    name: config.name,
    host: config.host,
    port: config.port
  }));
}

function getPerformanceHistory(limit = 50) {
  return performanceHistory.slice(0, limit);
}

function getConnectionHealth() {
  const connections = {};
  for (const [endpointId, health] of state.connections) {
    connections[endpointId] = { ...health };
  }
  return connections;
}

function getHealthSummary() {
  const now = Date.now();
  const fiveMinutesAgo = now - (5 * 60 * 1000);
  
  // Recent performance
  const recentSnapshots = performanceHistory.filter(s => s.timestamp >= fiveMinutesAgo);
  const avgDuration = recentSnapshots.length > 0 
    ? recentSnapshots.reduce((sum, s) => sum + s.duration, 0) / recentSnapshots.length 
    : 0;
  const recentErrors = recentSnapshots.reduce((sum, s) => sum + s.errorCount, 0);
  
  // Connection health
  let healthyConnections = 0;
  let totalConnections = state.connections.size;
  for (const connection of state.connections.values()) {
    if (connection.status === 'healthy') healthyConnections++;
  }
  
  return {
    overall: recentErrors === 0 && healthyConnections === totalConnections ? 'healthy' : 
             recentErrors < 3 ? 'warning' : 'error',
    performance: {
      avgResponseTime: Math.round(avgDuration),
      recentErrors,
      memoryUsage: process.memoryUsage().heapUsed,
      uptime: process.uptime()
    },
    connections: {
      healthy: healthyConnections,
      total: totalConnections
    },
    alerts: alertManager.getEnhancedAlertStats(),
    lastUpdate: state.stats.lastUpdated
  };
}

// Update PBS servers (for push mode)
function updatePbsServers(pbsServers) {
  if (Array.isArray(pbsServers)) {
    state.pbs = pbsServers;
    // Mark push-mode servers that haven't sent data recently as offline
    state.pbs.forEach(pbs => {
      if (pbs.pushMode && pbs.lastPushReceived) {
        // Mark as offline if no push received in 2 minutes
        if (Date.now() - pbs.lastPushReceived > 120000) {
          pbs.online = false;
          pbs.error = 'No data received in 2 minutes';
        }
      }
    });
  }
}

// Graceful cleanup
function destroy() {
  console.log('[State Manager] Cleaning up...');
  if (alertManager) {
    alertManager.destroy();
  }
  state.connections.clear();
  performanceHistory = [];
}

function getAlertManager() {
  return alertManager;
}

module.exports = {
  init,
  getState,
  setConfigPlaceholderStatus,
  setEndpointConfigurations,
  updateDiscoveryData,
  updateMetricsData,
  clearMetricsData,
  hasData,
  updatePbsServers,
  
  // Enhanced monitoring functions
  getPerformanceHistory,
  getConnectionHealth,
  getHealthSummary,
  getAlertInfo,
  destroy,
  
  // Alert manager access - make non-enumerable to prevent accidental serialization
  get alertManager() { return alertManager; },
  getAlertManager
};
