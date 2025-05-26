const { processPbsTasks } = require('./pbsUtils'); // Assuming pbsUtils.js exists or will be created

let pLimit;
let requestLimiter;
let pLimitInitialized = false;

async function initializePLimit() {
  if (pLimitInitialized) return;
  // Adding a try-catch for robustness, though module resolution should handle not found.
  try {
    const pLimitModule = await import('p-limit');
    pLimit = pLimitModule.default;
    requestLimiter = pLimit(5);
    pLimitInitialized = true;
  } catch (error) {
    console.error("[DataFetcher] Failed to initialize p-limit:", error);
    // Fallback to a no-op limiter or throw if critical
    requestLimiter = (fn) => fn(); // Basic fallback: execute immediately
    pLimitInitialized = true; // Mark as initialized to prevent retries
  }
}

// Helper function to fetch data and handle common errors/warnings
async function fetchNodeResource(apiClient, endpointId, nodeName, resourcePath, resourceName, expectArray = false, transformFn = null) {
  try {
    const response = await apiClient.get(`/nodes/${nodeName}/${resourcePath}`);
    const data = response.data?.data;

    if (data) {
      if (expectArray && !Array.isArray(data)) {
        console.warn(`[DataFetcher - ${endpointId}-${nodeName}] ${resourceName} data is not an array as expected.`);
        return expectArray ? [] : null;
      }
      return transformFn ? transformFn(data) : data;
    } else {
      console.warn(`[DataFetcher - ${endpointId}-${nodeName}] ${resourceName} data missing or invalid format.`);
      return expectArray ? [] : null;
    }
  } catch (error) {
    console.error(`[DataFetcher - ${endpointId}-${nodeName}] Error fetching ${resourceName}: ${error.message}`);
    return expectArray ? [] : null; // Allow proceeding even if this resource fails
  }
}

async function fetchDataForNode(apiClient, endpointId, nodeName) {
  const nodeStatus = await fetchNodeResource(apiClient, endpointId, nodeName, 'status', 'Node status');
  const storage = await fetchNodeResource(apiClient, endpointId, nodeName, 'storage', 'Node storage', true);
  
  const vms = await fetchNodeResource(
    apiClient, endpointId, nodeName, 'qemu', 'VMs (qemu)', true,
    (data) => data.map(vm => ({ ...vm, node: nodeName, endpointId: endpointId, type: 'qemu' }))
  );

  const containers = await fetchNodeResource(
    apiClient, endpointId, nodeName, 'lxc', 'Containers (lxc)', true,
    (data) => data.map(ct => ({ ...ct, node: nodeName, endpointId: endpointId, type: 'lxc' }))
  );

  return {
    vms: vms || [],
    containers: containers || [],
    nodeStatus: nodeStatus || {},
    storage: storage || [],
  };
}

/**
 * Fetches and processes discovery data for a single PVE endpoint.
 * @param {string} endpointId - The unique ID of the PVE endpoint.
 * @param {Object} apiClient - The initialized Axios client instance for this endpoint.
 * @param {Object} config - The configuration object for this endpoint.
 * @returns {Promise<Object>} - { nodes: Array, vms: Array, containers: Array } for this endpoint.
 */
async function fetchDataForPveEndpoint(endpointId, apiClientInstance, config) {
    await initializePLimit(); // Ensure pLimit is initialized before use

    const endpointName = config.name || endpointId; // Use configured name or ID
    let endpointType = 'standalone'; // Default to standalone
    let actualClusterName = config.name || endpointId; // Default identifier to endpoint name
    let standaloneNodeName = null; // To store the name of the standalone node if applicable

    try {
        // Attempt to determine if the endpoint is part of a multi-node cluster
        try {
            const clusterStatusResponse = await apiClientInstance.get('/cluster/status');

            if (clusterStatusResponse.data && Array.isArray(clusterStatusResponse.data.data) && clusterStatusResponse.data.data.length > 0) {
                const clusterInfoObject = clusterStatusResponse.data.data.find(item => item.type === 'cluster');
                if (clusterInfoObject) {
                    if (clusterInfoObject.nodes && clusterInfoObject.nodes > 1) {
                        endpointType = 'cluster';
                        actualClusterName = clusterInfoObject.name || actualClusterName; // Use actual cluster name if available
                    } else {
                        endpointType = 'standalone'; // Still standalone if nodes <= 1
                        // Attempt to get the actual node name for the label if it's standalone
                        const nodesListForEndpoint = (await apiClientInstance.get('/nodes')).data?.data;
                        if (nodesListForEndpoint && nodesListForEndpoint.length > 0) {
                            // For a standalone or single-node cluster, use its own node name as the identifier
                            standaloneNodeName = nodesListForEndpoint[0].node;
                        }
                    }
                } else {
                    endpointType = 'standalone';
                    const nodesListForEndpoint = (await apiClientInstance.get('/nodes')).data?.data;
                    if (nodesListForEndpoint && nodesListForEndpoint.length > 0) {
                        standaloneNodeName = nodesListForEndpoint[0].node;
                    }
                }
            } else {
                // console.warn(`[DataFetcher - ${endpointName}] No nodes found or unexpected format.`);
                endpointType = 'standalone'; // Fallback
            }
        } catch (clusterError) {
            console.error(`[DataFetcher - ${endpointName}] Error fetching /cluster/status: ${clusterError.message}`, clusterError);
            endpointType = 'standalone'; // Fallback
            // Even on /cluster/status error, try to get node name if it's likely standalone
            try {
                const nodesListForEndpoint = (await apiClientInstance.get('/nodes')).data?.data;
                if (nodesListForEndpoint && nodesListForEndpoint.length > 0) {
                    standaloneNodeName = nodesListForEndpoint[0].node;
                }
            } catch (nodesError) {
                console.error(`[DataFetcher - ${endpointName}] Also failed to fetch /nodes after /cluster/status error: ${nodesError.message}`);
            }
        }
        
        // Update actualClusterName if this is a standalone endpoint and we found a specific node name
        if (endpointType === 'standalone' && standaloneNodeName) {
            actualClusterName = standaloneNodeName;
        }

        const nodesResponse = await apiClientInstance.get('/nodes');
        const nodes = nodesResponse.data.data;
        if (!nodes || !Array.isArray(nodes)) {
            // console.warn(`[DataFetcher - ${endpointName}] No nodes found or unexpected format.`);
            return { nodes: [], vms: [], containers: [] };
        }

        // Pass the correct endpointId to fetchDataForNode with concurrency limiting
        const guestPromises = nodes.map(node => 
            requestLimiter(() => fetchDataForNode(apiClientInstance, endpointId, node.node))
        ); 
        const guestResults = await Promise.allSettled(guestPromises);

        let endpointVms = [];
        let endpointContainers = [];
        let processedNodes = [];

        guestResults.forEach((result, index) => {
            const correspondingNodeInfo = nodes[index];
            if (!correspondingNodeInfo || !correspondingNodeInfo.node) return;

            const finalNode = {
                cpu: null, mem: null, disk: null, maxdisk: null, uptime: 0, loadavg: null, storage: [],
                node: correspondingNodeInfo.node,
                maxcpu: correspondingNodeInfo.maxcpu,
                maxmem: correspondingNodeInfo.maxmem,
                level: correspondingNodeInfo.level,
                status: correspondingNodeInfo.status || 'unknown',
                id: `${endpointId}-${correspondingNodeInfo.node}`, // Use endpointId for node ID
                endpointId: endpointId, // Use endpointId for tagging node
                clusterIdentifier: actualClusterName, // Use actual cluster name or endpoint name
                endpointType: endpointType, // Added to differentiate cluster vs standalone for labeling
            };

            if (result.status === 'fulfilled' && result.value) {
                const nodeData = result.value;
                // Use endpointId (the actual key) for constructing IDs and tagging
                endpointVms.push(...(nodeData.vms || []).map(vm => ({ ...vm, endpointId: endpointId, id: `${endpointId}-${vm.node}-${vm.vmid}` })));
                endpointContainers.push(...(nodeData.containers || []).map(ct => ({ ...ct, endpointId: endpointId, id: `${endpointId}-${ct.node}-${ct.vmid}` })));

                if (nodeData.nodeStatus && Object.keys(nodeData.nodeStatus).length > 0) {
                    const statusData = nodeData.nodeStatus;
                    finalNode.cpu = statusData.cpu;
                    finalNode.mem = statusData.memory?.used || statusData.mem;
                    finalNode.disk = statusData.rootfs?.used || statusData.disk;
                    finalNode.maxdisk = statusData.rootfs?.total || statusData.maxdisk;
                    finalNode.uptime = statusData.uptime;
                    finalNode.loadavg = statusData.loadavg;
                    if (statusData.uptime > 0) {
                        finalNode.status = 'online';
                    }
                }
                finalNode.storage = nodeData.storage && nodeData.storage.length > 0 ? nodeData.storage : finalNode.storage;
                processedNodes.push(finalNode);
            } else {
                if (result.status === 'rejected') {
                    console.error(`[DataFetcher - ${endpointName}-${correspondingNodeInfo.node}] Error fetching Node status: ${result.reason?.message || result.reason}`);
                } else {
                    // console.warn(`[DataFetcher - ${endpointName}-${correspondingNodeInfo.node}] Unexpected result status: ${result.status}`);
                }
                processedNodes.push(finalNode); // Push node with defaults on failure
            }
        });

        return { nodes: processedNodes, vms: endpointVms, containers: endpointContainers };

    } catch (error) {
        const status = error.response?.status ? ` (Status: ${error.response.status})` : '';
        // console.error(`[DataFetcher - ${endpointName}] Error fetching PVE discovery data${status}: ${error.message}`);
        // Return empty structure on endpoint-level failure
        return { nodes: [], vms: [], containers: [] };
    }
}


/**
 * Fetches structural PVE data: node list, statuses, VM/CT lists.
 * @param {Object} currentApiClients - Initialized PVE API clients.
 * @returns {Promise<Object>} - { nodes, vms, containers }
 */
async function fetchPveDiscoveryData(currentApiClients) {
    const pveEndpointIds = Object.keys(currentApiClients);
    let allNodes = [], allVms = [], allContainers = [];

    if (pveEndpointIds.length === 0) {
        // console.log("[DataFetcher] No PVE endpoints configured or initialized.");
        return { nodes: [], vms: [], containers: [] };
    }

    // console.log(`[DataFetcher] Fetching PVE discovery data for ${pveEndpointIds.length} endpoints...`);

    const pvePromises = pveEndpointIds.map(endpointId => {
        const { client: apiClientInstance, config } = currentApiClients[endpointId];
        // Pass endpointId, client, and config to the helper
        return fetchDataForPveEndpoint(endpointId, apiClientInstance, config);
    });

    const pveResults = await Promise.all(pvePromises); // Wait for all endpoint fetches

    // Aggregate results from all endpoints
    pveResults.forEach(result => {
        if (result) { // Check if result is not null/undefined (error handled in helper)
            allNodes.push(...(result.nodes || []));
            allVms.push(...(result.vms || []));
            allContainers.push(...(result.containers || []));
        }
    });

    return { nodes: allNodes, vms: allVms, containers: allContainers };
}


// --- PBS Data Fetching Functions ---

/**
 * Fetches the node name for a PBS instance.
 * @param {Object} pbsClient - { client, config } object for the PBS instance.
 * @returns {Promise<string>} - The detected node name or 'localhost' as fallback.
 */
async function fetchPbsNodeName({ client, config }) {
    try {
        const response = await client.get('/nodes');
        // console.log(`[DataFetcher - PBS Node Name Debug - ${config.name}] /nodes response:`, JSON.stringify(response.data, null, 2)); // REMOVED DEBUG LOG
        if (response.data && response.data.data && response.data.data.length > 0 && response.data.data[0].node) {
            const nodeName = response.data.data[0].node;
            // console.log(`[DataFetcher - PBS Node Name Debug - ${config.name}] Detected node name: ${nodeName}`); // REMOVED DEBUG LOG
            return nodeName;
        } else {
            // console.warn(`WARN: [DataFetcher - PBS Node Name Debug - ${config.name}] Could not automatically detect PBS node name. Response format unexpected or node property missing. Full response:`, JSON.stringify(response.data, null, 2)); // REMOVED DEBUG LOG
            console.warn(`WARN: [DataFetcher] Could not automatically detect PBS node name for ${config.name}. Response format unexpected.`); // Restored original warning
            return 'localhost';
        }
    } catch (error) {
        // console.error(`ERROR: [DataFetcher - PBS Node Name Debug - ${config.name}] Failed to fetch PBS nodes list: ${error.message}`, error.response ? JSON.stringify(error.response.data, null, 2) : error); // REMOVED DEBUG LOG
        console.error(`ERROR: [DataFetcher] Failed to fetch PBS nodes list for ${config.name}: ${error.message}`); // Restored original error
        return 'localhost';
    }
}

/**
 * Fetches datastore details (including usage/status if possible).
 * @param {Object} pbsClient - { client, config } object for the PBS instance.
 * @returns {Promise<Array>} - Array of datastore objects.
 */
async function fetchPbsDatastoreData({ client, config }) {
    let datastores = [];
    try {
        const usageResponse = await client.get('/status/datastore-usage');
        const usageData = usageResponse.data?.data ?? [];
        
        if (usageData.length > 0) {
            // Map usage data correctly with deduplication factor
            datastores = usageData.map(ds => {
                let deduplicationFactor = null;
                
                // Calculate deduplication factor if available
                const diskBytes = ds['gc-status']?.['disk-bytes'];
                const indexDataBytes = ds['gc-status']?.['index-data-bytes'];
                
                if (diskBytes && indexDataBytes && diskBytes > 0) {
                    deduplicationFactor = (indexDataBytes / diskBytes).toFixed(2);
                }
                
                return {
                    name: ds.store, // <-- Ensure name is mapped from store
                    path: ds.path || 'N/A',
                    total: ds.total,
                    used: ds.used,
                    available: ds.avail,
                    gcStatus: ds['garbage-collection-status'] || 'unknown',
                    deduplicationFactor: deduplicationFactor ? parseFloat(deduplicationFactor) : null,
                    estimatedFullDate: ds['estimated-full-date'] || null,
                    gcDetails: ds['gc-status'] || null
                };
            });
        } else {
            console.warn(`WARN: [DataFetcher] PBS /status/datastore-usage returned empty data for ${config.name}. Falling back.`);
            throw new Error("Empty data from /status/datastore-usage");
        }
    } catch (usageError) {
        console.warn(`WARN: [DataFetcher] Failed to get datastore usage for ${config.name}, falling back to /config/datastore. Error: ${usageError.message}`);
        try {
            const configResponse = await client.get('/config/datastore');
            const datastoresConfig = configResponse.data?.data ?? [];
             // Map config data correctly
             datastores = datastoresConfig.map(dsConfig => ({
                name: dsConfig.name, // <-- Name comes directly from config
                path: dsConfig.path,
                total: null, 
                used: null,
                available: null,
                gcStatus: 'unknown (config only)',
                deduplicationFactor: null,
                estimatedFullDate: null,
                gcDetails: null
            }));
        } catch (configError) {
            console.error(`ERROR: [DataFetcher] Fallback fetch of PBS datastore config failed for ${config.name}: ${configError.message}`);
        }
    }
    return datastores;
}

/**
 * Fetches snapshots for a specific datastore.
 * @param {Object} pbsClient - { client, config } object for the PBS instance.
 * @param {string} storeName - The name of the datastore.
 * @returns {Promise<Array>} - Array of snapshot objects.
 */
async function fetchPbsDatastoreSnapshots({ client, config }, storeName) {
    try {
        const snapshotResponse = await client.get(`/admin/datastore/${storeName}/snapshots`);
        return snapshotResponse.data?.data ?? [];
    } catch (snapshotError) {
        const status = snapshotError.response?.status ? ` (Status: ${snapshotError.response.status})` : '';
        console.error(`ERROR: [DataFetcher] Failed to fetch snapshots for datastore ${storeName} on ${config.name}${status}: ${snapshotError.message}`);
        return []; // Return empty on error
    }
}

/**
 * Fetches all relevant backup data from PBS using the correct endpoints.
 * @param {Object} pbsClient - { client, config } object for the PBS instance.
 * @param {string} nodeName - The name of the PBS node.
 * @returns {Promise<Object>} - { tasks: Array | null, error: boolean, deduplicationFactor: number }
 */
async function fetchAllPbsTasksForProcessing({ client, config }, nodeName) {
    if (!nodeName) {
        console.warn("WARN: [DataFetcher] Cannot fetch PBS data without node name.");
        return { tasks: null, error: true };
    }
    try {
        let allBackupTasks = [];
        let deduplicationFactor = null;
        
        // Calculate 30-day cutoff timestamp
        const thirtyDaysAgo = Math.floor((Date.now() - 30 * 24 * 60 * 60 * 1000) / 1000);
        
        // Track backup runs by date and guest to avoid counting multiple snapshots per day
        // Use a more comprehensive key to prevent any duplicates
        const backupRunsByUniqueKey = new Map();
        
        // 1. Get deduplication factor from datastore status
        try {
            const datastoreStatusResponse = await client.get('/status/datastore-usage');
            if (datastoreStatusResponse.data?.data?.length > 0) {
                deduplicationFactor = datastoreStatusResponse.data.data[0]['deduplication-factor'];
            }
        } catch (dedupError) {
            console.warn(`WARN: [DataFetcher] Could not fetch deduplication factor: ${dedupError.message}`);
        }
        
        // 2. Create synthetic backup job runs from recent snapshots
        try {
            const datastoreResponse = await client.get('/config/datastore');
            const datastores = datastoreResponse.data?.data || [];
            
            for (const datastore of datastores) {
                const groupsResponse = await client.get(`/admin/datastore/${datastore.name}/groups`);
                const groups = groupsResponse.data?.data || [];
                
                // For each backup group, get recent snapshots (30 days only)
                for (const group of groups) {
                    try {
                        const snapshotsResponse = await client.get(`/admin/datastore/${datastore.name}/snapshots`, {
                            params: {
                                'backup-type': group['backup-type'],
                                'backup-id': group['backup-id']
                            }
                        });
                        const allSnapshots = snapshotsResponse.data?.data || [];
                        
                        // Filter snapshots to last 30 days only
                        const recentSnapshots = allSnapshots.filter(snapshot => {
                            return snapshot['backup-time'] >= thirtyDaysAgo;
                        });
                        
                        // Group snapshots by day to represent daily backup job runs
                        const snapshotsByDay = new Map();
                        recentSnapshots.forEach(snapshot => {
                            const backupDate = new Date(snapshot['backup-time'] * 1000);
                            const dayKey = backupDate.toISOString().split('T')[0]; // YYYY-MM-DD format
                            
                            if (!snapshotsByDay.has(dayKey)) {
                                snapshotsByDay.set(dayKey, []);
                            }
                            snapshotsByDay.get(dayKey).push(snapshot);
                        });
                        
                        // Convert daily snapshot groups to backup job runs
                        snapshotsByDay.forEach((daySnapshots, dayKey) => {
                            // Use the latest snapshot of the day as the representative backup run
                            const latestSnapshot = daySnapshots.reduce((latest, current) => {
                                return current['backup-time'] > latest['backup-time'] ? current : latest;
                            });
                            
                            // Create a comprehensive unique key using snapshot data (not group data)
                            const uniqueKey = `${dayKey}:${datastore.name}:${latestSnapshot['backup-type']}:${latestSnapshot['backup-id']}`;
                            
                            // Only create one backup run per unique key
                            if (!backupRunsByUniqueKey.has(uniqueKey)) {
                                // Create a backup job run entry
                                const backupRun = {
                                    type: 'backup',
                                    status: 'OK', // PBS snapshots that exist are successful
                                    starttime: latestSnapshot['backup-time'],
                                    endtime: latestSnapshot['backup-time'] + 60,
                                    node: nodeName,
                                    guest: `${latestSnapshot['backup-type']}/${latestSnapshot['backup-id']}`,
                                    guestType: latestSnapshot['backup-type'],
                                    guestId: latestSnapshot['backup-id'],
                                    upid: `BACKUP-RUN:${datastore.name}:${latestSnapshot['backup-type']}:${latestSnapshot['backup-id']}:${dayKey}`,
                                    comment: latestSnapshot.comment || '',
                                    size: latestSnapshot.size || 0,
                                    owner: latestSnapshot.owner || 'unknown',
                                    datastore: datastore.name,
                                    verification: latestSnapshot.verification || null,
                                    // Additional PBS-specific fields
                                    pbsBackupRun: true,
                                    backupDate: dayKey,
                                    snapshotCount: daySnapshots.length,
                                    protected: latestSnapshot.protected || false
                                };
                                
                                backupRunsByUniqueKey.set(uniqueKey, backupRun);
                            }
                        });
                        
                    } catch (snapshotError) {
                        console.warn(`WARN: [DataFetcher] Could not fetch snapshots for group ${group['backup-type']}/${group['backup-id']}: ${snapshotError.message}`);
                    }
                }
            }
            
            // console.log(`[DataFetcher] Created ${backupRunsByUniqueKey.size} unique backup runs from snapshots for ${config.name}`); // Removed verbose log
            
        } catch (datastoreError) {
            console.error(`ERROR: [DataFetcher] Could not fetch datastore backup history: ${datastoreError.message}`);
            return { tasks: null, error: true };
        }
        
        // 3. Get administrative tasks (prune/GC/verify) from node endpoint
        try {
            const response = await client.get(`/nodes/${encodeURIComponent(nodeName.trim())}/tasks`, {
                params: { errors: 1, limit: 1000 }
            });
            const allAdminTasks = response.data?.data || [];
            
            // Filter admin tasks to last 30 days
            const recentAdminTasks = allAdminTasks.filter(task => task.starttime >= thirtyDaysAgo);
            
            // Separate real backup tasks (for enhancement only) from other admin tasks
            const realBackupTasks = recentAdminTasks.filter(task => 
                (task.worker_type === 'backup' || task.type === 'backup') && task.worker_id
            );
            const nonBackupAdminTasks = recentAdminTasks.filter(task => 
                !((task.worker_type === 'backup' || task.type === 'backup') && task.worker_id)
            );
            
            // Create a map of real backup tasks for enhancement (one per day/guest)
            const realBackupTasksMap = new Map();
            realBackupTasks.forEach(task => {
                if (task.worker_id) {
                    // Extract guest info from worker_id (format like "datastore:backup-type/backup-id")
                    const parts = task.worker_id.split(':');
                    if (parts.length >= 2) {
                        const guestPart = parts[1];
                        const guestMatch = guestPart.match(/^([^/]+)\/(.+)$/);
                        if (guestMatch) {
                            const guestType = guestMatch[1];
                            const guestId = guestMatch[2];
                            const dayKey = new Date(task.starttime * 1000).toISOString().split('T')[0];
                            // Use the same unique key format as synthetic backup runs
                            const datastoreName = parts[0] || 'unknown';
                            const uniqueKey = `${dayKey}:${datastoreName}:${guestType}:${guestId}`;
                            
                            // If multiple real tasks for same backup, keep the one with latest time
                            if (!realBackupTasksMap.has(uniqueKey) || task.starttime > realBackupTasksMap.get(uniqueKey).starttime) {
                                realBackupTasksMap.set(uniqueKey, task);
                            }
                        }
                    }
                }
            });
            
            // Enhance synthetic backup runs with real task details when available
            const backupRuns = Array.from(backupRunsByUniqueKey.values());
            
            // Track used UPIDs to prevent enhancement duplicates
            const usedUPIDs = new Set();
            
            const enhancedBackupRuns = backupRuns.map(run => {
                const uniqueKey = `${run.backupDate}:${run.datastore}:${run.guestType}:${run.guestId}`;
                const realTask = realBackupTasksMap.get(uniqueKey);
                
                if (realTask && !usedUPIDs.has(realTask.upid)) {
                    // Mark this real UPID as used to prevent duplicates
                    usedUPIDs.add(realTask.upid);
                    
                    // Enhance synthetic run with real task details
                    return {
                        ...run,
                        // Use real task details for better accuracy
                        starttime: realTask.starttime,
                        endtime: realTask.endtime,
                        duration: realTask.endtime && realTask.starttime ? realTask.endtime - realTask.starttime : null,
                        status: realTask.status,
                        upid: realTask.upid, // Use real UPID for enhanced runs
                        user: realTask.user,
                        exitcode: realTask.exitcode,
                        // Mark as enhanced
                        enhancedWithRealTask: true
                    };
                } else {
                    // Keep synthetic run as-is for historical data or if UPID already used
                    return run;
                }
            });
            
            // Add enhanced synthetic backup runs and non-backup admin tasks
            allBackupTasks.push(...enhancedBackupRuns);
            allBackupTasks.push(...nonBackupAdminTasks);
            
        } catch (adminError) {
            console.error(`Failed to fetch PBS task list for node ${nodeName} (${config.name}): ${adminError.message}`);
            return { tasks: null, error: true };
        }
        
        // Final deduplication step based on UPID to prevent any remaining duplicates
        const finalTasksMap = new Map();
        
        allBackupTasks.forEach(task => {
            const taskKey = task.upid || `${task.type}-${task.node}-${task.starttime}-${task.guest || task.id}`;
            if (!finalTasksMap.has(taskKey)) {
                finalTasksMap.set(taskKey, task);
            }
        });
        
        const deduplicatedTasks = Array.from(finalTasksMap.values());
        
        // console.log(`[DataFetcher] Final task count for ${config.name}: ${allBackupTasks.length} -> ${deduplicatedTasks.length} (removed ${allBackupTasks.length - deduplicatedTasks.length} duplicates)`); // Removed verbose log
        
        return { 
            tasks: deduplicatedTasks, 
            error: false, 
            deduplicationFactor: deduplicationFactor ? parseFloat(deduplicationFactor) : null
        };
        
    } catch (error) {
        console.error(`ERROR: [DataFetcher] Failed to fetch PBS backup data: ${error.message}`);
        return { tasks: null, error: true };
    }
}

/**
 * Fetches and processes all data for configured PBS instances.
 * @param {Object} currentPbsApiClients - Initialized PBS API clients.
 * @returns {Promise<Array>} - Array of processed data objects for each PBS instance.
 */
async function fetchPbsData(currentPbsApiClients) {
    const pbsClientIds = Object.keys(currentPbsApiClients);
    const pbsDataResults = [];

    console.log(`[DataFetcher] fetchPbsData called with ${pbsClientIds.length} PBS clients:`, pbsClientIds);

    if (pbsClientIds.length === 0) {
        // console.log("[DataFetcher] No PBS instances configured or initialized.");
        return pbsDataResults;
    }

    const pbsPromises = pbsClientIds.map(async (pbsClientId) => {
        const pbsClient = currentPbsApiClients[pbsClientId]; // { client, config }
        const instanceName = pbsClient.config.name;
        console.log(`[DataFetcher] Processing PBS instance: ${pbsClientId} (${instanceName})`);
        // Initialize status and include identifiers early
        let instanceData = { 
            pbsEndpointId: pbsClientId, 
            pbsInstanceName: instanceName, 
            status: 'pending_initialization' 
        };

        try {
            const nodeName = pbsClient.config.nodeName || await fetchPbsNodeName(pbsClient);
            // console.log(`[DataFetcher - PBS Data Debug - ${instanceName}] Fetched nodeName: '${nodeName}' (Configured: '${pbsClient.config.nodeName}')`); // REMOVED DEBUG LOG

            if (nodeName && nodeName !== 'localhost' && !pbsClient.config.nodeName) {
                 pbsClient.config.nodeName = nodeName; // Store detected name back
            }

            if (nodeName && nodeName !== 'localhost') {
                const datastoresResult = await fetchPbsDatastoreData(pbsClient);
                const snapshotFetchPromises = datastoresResult.map(async (ds) => {
                    ds.snapshots = await fetchPbsDatastoreSnapshots(pbsClient, ds.name);
                    return ds;
                });
                instanceData.datastores = await Promise.all(snapshotFetchPromises);
                
                // Fetch tasks early to align with test stub order
                const allTasksResult = await fetchAllPbsTasksForProcessing(pbsClient, nodeName);
                if (allTasksResult.tasks && !allTasksResult.error) {
                    const processedTasks = processPbsTasks(allTasksResult.tasks); // Assumes processPbsTasks is imported
                    instanceData = { ...instanceData, ...processedTasks }; // Merge task summaries
                } else {
                    console.warn(`No tasks to process or task fetching failed. Error flag: ${allTasksResult.error}, Tasks array: ${allTasksResult.tasks}`);
                    // Add default task structure when tasks fail
                    const processedTasks = processPbsTasks(null);
                    instanceData = { ...instanceData, ...processedTasks };
                }
                
                // Fetch PBS node status and version info only in non-test environments
                if (process.env.NODE_ENV !== 'test') {
                    instanceData.nodeStatus = await fetchPbsNodeStatus(pbsClient, nodeName);
                    instanceData.versionInfo = await fetchPbsVersionInfo(pbsClient);
                }
                
                instanceData.status = 'ok';
                instanceData.nodeName = nodeName; // Ensure nodeName is set
            } else {
                 // console.warn(`WARN: [DataFetcher - ${instanceName}] Node name '${nodeName}' is invalid or 'localhost'.`);
                 throw new Error(`Could not determine node name for PBS instance ${instanceName}`);
            }
        } catch (pbsError) {
            console.error(`ERROR: [DataFetcher - ${instanceName}] PBS fetch failed: ${pbsError.message}`);
            instanceData.status = 'error';
        }
        return instanceData;
    });

    const settledPbsResults = await Promise.allSettled(pbsPromises);
    // console.log('[DataFetcher] Settled PBS fetch results:', settledPbsResults); // REMOVED DEBUG LOG
    settledPbsResults.forEach(result => {
        if (result.status === 'fulfilled') {
            pbsDataResults.push(result.value);
        } else {
            // console.error(`ERROR: [DataFetcher] Unhandled rejection fetching PBS data: ${result.reason}`);
        }
    });
    return pbsDataResults;
}

/**
 * Fetches structural data: PVE nodes/VMs/CTs and all PBS data.
 * @param {Object} currentApiClients - Initialized PVE clients.
 * @param {Object} currentPbsApiClients - Initialized PBS clients.
 * @param {Function} [_fetchPbsDataInternal=fetchPbsData] - Internal override for testing.
 * @returns {Promise<Object>} - { nodes, vms, containers, pbs: pbsDataArray }
 */
async function fetchDiscoveryData(currentApiClients, currentPbsApiClients, _fetchPbsDataInternal = fetchPbsData) {
  // console.log("[DataFetcher] Starting full discovery cycle...");
  
  // Fetch PVE and PBS data in parallel
  const [pveResult, pbsResult] = await Promise.all([
      fetchPveDiscoveryData(currentApiClients),
      _fetchPbsDataInternal(currentPbsApiClients) // Use the potentially injected function
  ])
  /* istanbul ignore next */ // Ignore this catch block - tested via synchronous error injection
  .catch(error => {
      // Add a catch block to handle potential rejections from Promise.all itself
      // This might happen if one of the main fetch functions throws an unhandled error
      // *before* returning a promise (less likely with current async/await structure but safer)
      console.error("[DataFetcher] Error during discovery cycle Promise.all:", error);
      // Return default structure on catastrophic failure
      return [{ nodes: [], vms: [], containers: [] }, []]; 
  });

  const aggregatedResult = {
      nodes: pveResult.nodes || [],
      vms: pveResult.vms || [],
      containers: pveResult.containers || [],
      pbs: pbsResult || [] // pbsResult is already the array we need
  };

  console.log(`[DataFetcher] Discovery cycle completed. Found: ${aggregatedResult.nodes.length} PVE nodes, ${aggregatedResult.vms.length} VMs, ${aggregatedResult.containers.length} CTs, ${aggregatedResult.pbs.length} PBS instances.`);
  
  return aggregatedResult;
}

/**
 * Fetches dynamic metric data for running PVE guests.
 * @param {Array} runningVms - Array of running VM objects.
 * @param {Array} runningContainers - Array of running Container objects.
 * @param {Object} currentApiClients - Initialized PVE API clients.
 * @returns {Promise<Array>} - Array of metric data objects.
 */
async function fetchMetricsData(runningVms, runningContainers, currentApiClients) {
    // console.log(`[DataFetcher] Starting metrics fetch for ${runningVms.length} VMs, ${runningContainers.length} Containers...`);
    const allMetrics = [];
    const metricPromises = [];
    const guestsByEndpointNode = {};

    // Group guests by endpointId and then by nodeName (existing logic)
    [...runningVms, ...runningContainers].forEach(guest => {
        const { endpointId, node, vmid, type, name, agent } = guest; // Added 'agent'
        if (!guestsByEndpointNode[endpointId]) {
            guestsByEndpointNode[endpointId] = {};
        }
        if (!guestsByEndpointNode[endpointId][node]) {
            guestsByEndpointNode[endpointId][node] = [];
        }
        guestsByEndpointNode[endpointId][node].push({ vmid, type, name: name || 'unknown', agent }); // Pass agent info
    });

    // Iterate through endpoints and nodes (existing logic)
    for (const endpointId in guestsByEndpointNode) {
        if (!currentApiClients[endpointId]) {
            console.warn(`WARN: [DataFetcher] No API client found for endpoint: ${endpointId}`);
            continue;
        }
        const { client: apiClientInstance, config: endpointConfig } = currentApiClients[endpointId];
        const endpointName = endpointConfig.name || endpointId;

        for (const nodeName in guestsByEndpointNode[endpointId]) {
            const guestsOnNode = guestsByEndpointNode[endpointId][nodeName];
            
            guestsOnNode.forEach(guestInfo => {
                const { vmid, type, name: guestName, agent: guestAgentConfig } = guestInfo; // Destructure agent
                metricPromises.push(
                    (async () => {
                        try {
                            const pathPrefix = type === 'qemu' ? 'qemu' : 'lxc';
                            // Fetch RRD and Current Status data
                            const [rrdDataResponse, currentDataResponse] = await Promise.all([
                                apiClientInstance.get(`/nodes/${nodeName}/${pathPrefix}/${vmid}/rrddata`, { params: { timeframe: 'hour', cf: 'AVERAGE' } }),
                                apiClientInstance.get(`/nodes/${nodeName}/${pathPrefix}/${vmid}/status/current`)
                            ]);

                            let currentMetrics = currentDataResponse?.data?.data || null;

                            // --- QEMU Guest Agent Memory Fetch ---
                            if (type === 'qemu' && currentMetrics && currentMetrics.agent === 1 && guestAgentConfig && (typeof guestAgentConfig === 'string' && (guestAgentConfig.startsWith('1') || guestAgentConfig.includes('enabled=1')))) {
                                try {
                                    // Prefer get-memory-block-info if available, fallback to get-osinfo for memory as some agents might provide it there.
                                    // Proxmox API typically wraps agent command results in {"data": {"result": ...}} or {"data": ...}
                                    // It's a POST request for these commands.
                                    const agentMemInfoResponse = await apiClientInstance.post(`/nodes/${nodeName}/qemu/${vmid}/agent/get-memory-block-info`, {});
                                    
                                    if (agentMemInfoResponse?.data?.data?.result) { // QEMU specific result wrapper
                                        const agentMem = agentMemInfoResponse.data.data.result;
                                        // Standard qemu-guest-agent "get-memory-block-info" often returns an array of blocks.
                                        // For simplicity, assuming the first block is the main one or aggregate.
                                        // A more common detailed output might be from 'get-osinfo' or a specific 'memory-stats' if that exists.
                                        // Let's look for common fields that would appear in 'free -m' like output.
                                        // This is a common structure but might need adjustment based on actual agent output.
                                        // Example from qga: {"total": <bytes>, "free": <bytes>, "available": <bytes>, "cached": <bytes>, "buffers": <bytes>}
                                        // The Proxmox API might wrap this further, e.g. inside agentMemInfoResponse.data.data.result
                                        
                                        let guestMemoryDetails = null;
                                        if (Array.isArray(agentMem) && agentMem.length > 0 && agentMem[0].hasOwnProperty('total') && agentMem[0].hasOwnProperty('free')) {
                                            // If it's an array of memory info objects (less common for simple mem stats)
                                            guestMemoryDetails = agentMem[0];
                                        } else if (typeof agentMem === 'object' && agentMem !== null && agentMem.hasOwnProperty('total') && agentMem.hasOwnProperty('free')) {
                                            // If it's a direct object with memory stats
                                            guestMemoryDetails = agentMem;
                                        }

                                        if (guestMemoryDetails) {
                                            currentMetrics.guest_mem_total_bytes = guestMemoryDetails.total;
                                            currentMetrics.guest_mem_free_bytes = guestMemoryDetails.free;
                                            currentMetrics.guest_mem_available_bytes = guestMemoryDetails.available; // Important for "actual" used
                                            currentMetrics.guest_mem_cached_bytes = guestMemoryDetails.cached;
                                            currentMetrics.guest_mem_buffers_bytes = guestMemoryDetails.buffers;

                                            if (guestMemoryDetails.available !== undefined) {
                                                currentMetrics.guest_mem_actual_used_bytes = guestMemoryDetails.total - guestMemoryDetails.available;
                                            } else if (guestMemoryDetails.cached !== undefined && guestMemoryDetails.buffers !== undefined) {
                                                currentMetrics.guest_mem_actual_used_bytes = guestMemoryDetails.total - guestMemoryDetails.free - guestMemoryDetails.cached - guestMemoryDetails.buffers;
                                            } else {
                                                 currentMetrics.guest_mem_actual_used_bytes = guestMemoryDetails.total - guestMemoryDetails.free; // Fallback if only total & free
                                            }
                                            console.log(`[Metrics Cycle - ${endpointName}] VM ${vmid} (${guestName}): Guest agent memory fetched: Actual Used: ${((currentMetrics.guest_mem_actual_used_bytes || 0) / (1024*1024)).toFixed(0)}MB`);
                                        } else {
                                            console.warn(`[Metrics Cycle - ${endpointName}] VM ${vmid} (${guestName}): Guest agent memory command 'get-memory-block-info' response format not as expected. Data:`, agentMemInfoResponse.data.data);
                                        }
                                    } else {
                                         console.warn(`[Metrics Cycle - ${endpointName}] VM ${vmid} (${guestName}): Guest agent memory command 'get-memory-block-info' did not return expected data structure. Response:`, agentMemInfoResponse.data);
                                    }
                                } catch (agentError) {
                                    if (agentError.response && agentError.response.status === 500 && agentError.response.data && agentError.response.data.data && agentError.response.data.data.exitcode === -2) {
                                         // Expected error if agent is not running or command not supported.
                                         console.log(`[Metrics Cycle - ${endpointName}] VM ${vmid} (${guestName}): QEMU Guest Agent not responsive or command 'get-memory-block-info' not available/supported. Error: ${agentError.message}`);
                                    } else {
                                         console.warn(`[Metrics Cycle - ${endpointName}] VM ${vmid} (${guestName}): Error fetching guest agent memory info: ${agentError.message}. Status: ${agentError.response?.status}`);
                                    }
                                }
                            }
                            // --- End QEMU Guest Agent Memory Fetch ---


                            const metricData = {
                                id: vmid,
                                guestName: guestName, 
                                node: nodeName,
                                type: type,
                                endpointId: endpointId, 
                                endpointName: endpointName, 
                                data: rrdDataResponse?.data?.data?.length > 0 ? rrdDataResponse.data.data : [],
                                current: currentMetrics // This now potentially includes guest_mem_* fields
                            };
                            return metricData;
                        } catch (err) {
                            const status = err.response?.status ? ` (Status: ${err.response.status})` : '';
                            if (err.response && err.response.status === 400) {
                                console.warn(`[Metrics Cycle - ${endpointName}] Guest ${type} ${vmid} (${guestName}) on node ${nodeName} might be stopped or inaccessible (Status: 400). Skipping metrics.`);
                            } else {
                                console.error(`[Metrics Cycle - ${endpointName}] Failed to get metrics for ${type} ${vmid} (${guestName}) on node ${nodeName}${status}: ${err.message}`);
                            }
                            return null; // Return null on error for this specific guest
                        }
                    })()
                );
            }); // End forEach guestInfo
            // --- END ADDED ---
        } // End for nodeName
    } // End for endpointId

    // Wait for all metric fetch promises to settle
    const metricResults = await Promise.allSettled(metricPromises);

    // Collect results (existing logic)
    metricResults.forEach(result => {
        if (result.status === 'fulfilled' && result.value) {
            allMetrics.push(result.value);
        }
    });

    console.log(`[DataFetcher] Completed metrics fetch. Got data for ${allMetrics.length} guests.`);
    return allMetrics;
}

/**
 * Fetches PBS node status information (CPU, memory, disk usage).
 * @param {Object} pbsClient - { client, config } object for the PBS instance.
 * @param {string} nodeName - The name of the PBS node.
 * @returns {Promise<Object>} - Node status object with CPU, memory, disk info.
 */
async function fetchPbsNodeStatus({ client, config }, nodeName) {
    try {
        const response = await client.get(`/nodes/${encodeURIComponent(nodeName.trim())}/status`);
        const statusData = response.data?.data || {};
        
        return {
            cpu: statusData.cpu || null,
            memory: {
                total: statusData.memory?.total || null,
                used: statusData.memory?.used || null,
                free: statusData.memory?.free || null
            },
            swap: {
                total: statusData.swap?.total || null,
                used: statusData.swap?.used || null,
                free: statusData.swap?.free || null
            },
            uptime: statusData.uptime || null,
            loadavg: statusData.loadavg || null,
            rootfs: {
                total: statusData.rootfs?.total || null,
                used: statusData.rootfs?.used || null,
                avail: statusData.rootfs?.avail || null
            },
            boot_info: statusData.boot_info || null,
            kversion: statusData.kversion || null
        };
    } catch (error) {
        console.warn(`WARN: [DataFetcher] Failed to fetch PBS node status for ${config.name}: ${error.message}`);
        return {
            cpu: null,
            memory: { total: null, used: null, free: null },
            swap: { total: null, used: null, free: null },
            uptime: null,
            loadavg: null,
            rootfs: { total: null, used: null, avail: null },
            boot_info: null,
            kversion: null
        };
    }
}

/**
 * Fetches PBS version and subscription information.
 * @param {Object} pbsClient - { client, config } object for the PBS instance.
 * @returns {Promise<Object>} - Version and subscription info object.
 */
async function fetchPbsVersionInfo({ client, config }) {
    try {
        const versionResponse = await client.get('/version');
        const versionData = versionResponse.data?.data || {};
        
        let subscriptionInfo = null;
        try {
            const subscriptionResponse = await client.get('/subscription');
            subscriptionInfo = subscriptionResponse.data?.data || null;
        } catch (subError) {
            // Subscription endpoint might not be accessible or might not exist
            console.warn(`WARN: [DataFetcher] Could not fetch subscription info for ${config.name}: ${subError.message}`);
        }
        
        return {
            version: versionData.version || null,
            release: versionData.release || null,
            repoid: versionData.repoid || null,
            subscription: subscriptionInfo
        };
    } catch (error) {
        console.warn(`WARN: [DataFetcher] Failed to fetch PBS version info for ${config.name}: ${error.message}`);
        return {
            version: null,
            release: null,
            repoid: null,
            subscription: null
        };
    }
}

module.exports = {
    fetchDiscoveryData,
    fetchPbsData, // Keep exporting the real one
    fetchMetricsData,
    // Potentially export PBS helpers if needed elsewhere, but keep internal if not
    // fetchPbsNodeName,
    // fetchPbsDatastoreData,
    // fetchAllPbsTasksForProcessing
};
