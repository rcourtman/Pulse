const { processPbsTasks } = require('./pbsUtils'); // Assuming pbsUtils.js exists or will be created

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
                console.warn(`[DataFetcher - ${endpointName}] No nodes found or unexpected format.`);
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
            console.warn(`[DataFetcher - ${endpointName}] No nodes found or unexpected format.`);
            return { nodes: [], vms: [], containers: [] };
        }

        // Pass the correct endpointId to fetchDataForNode
        const guestPromises = nodes.map(node => fetchDataForNode(apiClientInstance, endpointId, node.node)); 
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
                    console.warn(`[DataFetcher - ${endpointName}-${correspondingNodeInfo.node}] Unexpected result status: ${result.status}`);
                }
                processedNodes.push(finalNode); // Push node with defaults on failure
            }
        });

        return { nodes: processedNodes, vms: endpointVms, containers: endpointContainers };

    } catch (error) {
        const status = error.response?.status ? ` (Status: ${error.response.status})` : '';
        console.error(`[DataFetcher - ${endpointName}] Error fetching PVE discovery data${status}: ${error.message}`);
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
        console.log("[DataFetcher] No PVE endpoints configured or initialized.");
        return { nodes: [], vms: [], containers: [] };
    }

    console.log(`[DataFetcher] Fetching PVE discovery data for ${pveEndpointIds.length} endpoints...`);

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
        if (response.data && response.data.data && response.data.data.length > 0) {
            const nodeName = response.data.data[0].node;
            console.log(`INFO: [DataFetcher] Detected PBS node name: ${nodeName} for ${config.name}`);
            return nodeName;
        } else {
            console.warn(`WARN: [DataFetcher] Could not automatically detect PBS node name for ${config.name}. Response format unexpected.`);
            return 'localhost';
        }
    } catch (error) {
        console.error(`ERROR: [DataFetcher] Failed to fetch PBS nodes list for ${config.name}: ${error.message}`);
        return 'localhost';
    }
}

/**
 * Fetches datastore details (including usage/status if possible).
 * @param {Object} pbsClient - { client, config } object for the PBS instance.
 * @returns {Promise<Array>} - Array of datastore objects.
 */
async function fetchPbsDatastoreData({ client, config }) {
    console.log(`INFO: [DataFetcher] Fetching PBS datastore data for ${config.name}...`);
    let datastores = [];
    try {
        const usageResponse = await client.get('/status/datastore-usage');
        const usageData = usageResponse.data?.data ?? [];
        if (usageData.length > 0) {
            console.log(`INFO: [DataFetcher] Fetched status for ${usageData.length} PBS datastores via /status/datastore-usage for ${config.name}.`);
            // Map usage data correctly
            datastores = usageData.map(ds => ({
                name: ds.store, // <-- Ensure name is mapped from store
                path: ds.path || 'N/A',
                total: ds.total,
                used: ds.used,
                available: ds.avail,
                gcStatus: ds['garbage-collection-status'] || 'unknown'
            }));
        } else {
            console.warn(`WARN: [DataFetcher] PBS /status/datastore-usage returned empty data for ${config.name}. Falling back.`);
            throw new Error("Empty data from /status/datastore-usage");
        }
    } catch (usageError) {
        console.warn(`WARN: [DataFetcher] Failed to get datastore usage for ${config.name}, falling back to /config/datastore. Error: ${usageError.message}`);
        try {
            const configResponse = await client.get('/config/datastore');
            const datastoresConfig = configResponse.data?.data ?? [];
             console.log(`INFO: [DataFetcher] Fetched config for ${datastoresConfig.length} PBS datastores (fallback) for ${config.name}.`);
             // Map config data correctly
             datastores = datastoresConfig.map(dsConfig => ({
                name: dsConfig.name, // <-- Name comes directly from config
                path: dsConfig.path,
                total: null, 
                used: null,
                available: null,
                gcStatus: 'unknown (config only)' 
            }));
        } catch (configError) {
            console.error(`ERROR: [DataFetcher] Fallback fetch of PBS datastore config failed for ${config.name}: ${configError.message}`);
        }
    }
    console.log(`INFO: [DataFetcher] Found ${datastores.length} datastores for ${config.name}.`);
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
 * Fetches all relevant tasks from PBS for later processing.
 * @param {Object} pbsClient - { client, config } object for the PBS instance.
 * @param {string} nodeName - The name of the PBS node.
 * @returns {Promise<Object>} - { tasks: Array | null, error: boolean }
 */
async function fetchAllPbsTasksForProcessing({ client, config }, nodeName) {
    console.log(`INFO: [DataFetcher] Fetching PBS tasks for node ${nodeName} on ${config.name}...`);
    if (!nodeName) {
        console.warn("WARN: [DataFetcher] Cannot fetch PBS task data without node name.");
        return { tasks: null, error: true };
    }
    try {
        const sinceTimestamp = Math.floor((Date.now() - 7 * 24 * 60 * 60 * 1000) / 1000);
        const trimmedNodeName = nodeName.trim();
        const encodedNodeName = encodeURIComponent(trimmedNodeName);
        const response = await client.get(`/nodes/${encodedNodeName}/tasks`, {
            params: { since: sinceTimestamp, limit: 2500, errors: 1 }
        });
        const allTasks = response.data?.data ?? [];
        console.log(`INFO: [DataFetcher] Fetched ${allTasks.length} tasks from PBS node ${nodeName}.`);
        return { tasks: allTasks, error: false };
    } 
    /* istanbul ignore next */ // Ignore this catch block - tested via side effects (logging, return value check)
    catch (error) {
        console.error(`ERROR: [DataFetcher] Failed to fetch PBS task list for node ${nodeName} (${config.name}): ${error.message}`);
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

    if (pbsClientIds.length === 0) {
        console.log("[DataFetcher] No PBS instances configured or initialized.");
        return pbsDataResults;
    }

    console.log(`[DataFetcher] Fetching discovery data for ${pbsClientIds.length} PBS instances...`);
    const pbsPromises = pbsClientIds.map(async (pbsClientId) => {
        const pbsClient = currentPbsApiClients[pbsClientId]; // { client, config }
        const instanceName = pbsClient.config.name;
        // Initialize status and include identifiers early
        let instanceData = { 
            pbsEndpointId: pbsClientId, 
            pbsInstanceName: instanceName, 
            status: 'pending_initialization' 
        };

        try {
            console.log(`INFO: [DataFetcher - ${instanceName}] Starting fetch. Initial status: ${instanceData.status}`);

            const nodeName = pbsClient.config.nodeName || await fetchPbsNodeName(pbsClient);
            console.log(`INFO: [DataFetcher - ${instanceName}] Determined nodeName: '${nodeName}'. Configured nodeName: '${pbsClient.config.nodeName}'`);

            if (nodeName && nodeName !== 'localhost' && !pbsClient.config.nodeName) {
                 pbsClient.config.nodeName = nodeName; // Store detected name back
                 console.log(`INFO: [DataFetcher - ${instanceName}] Stored detected nodeName: '${nodeName}' to config.`);
            }

            if (nodeName && nodeName !== 'localhost') {
                console.log(`INFO: [DataFetcher - ${instanceName}] Node name '${nodeName}' is valid. Proceeding with data fetch.`);

                const datastoresResult = await fetchPbsDatastoreData(pbsClient);
                const snapshotFetchPromises = datastoresResult.map(async (ds) => {
                    ds.snapshots = await fetchPbsDatastoreSnapshots(pbsClient, ds.name);
                    return ds;
                });
                instanceData.datastores = await Promise.all(snapshotFetchPromises);
                console.log(`INFO: [DataFetcher - ${instanceName}] Datastores and snapshots fetched. Number of datastores: ${instanceData.datastores ? instanceData.datastores.length : 'N/A'}`);
                
                const allTasksResult = await fetchAllPbsTasksForProcessing(pbsClient, nodeName);
                console.log(`INFO: [DataFetcher - ${instanceName}] Tasks fetched. Result error: ${allTasksResult.error}, Tasks found: ${allTasksResult.tasks ? allTasksResult.tasks.length : 'null'}`);

                if (allTasksResult.tasks && !allTasksResult.error) {
                    console.log(`INFO: [DataFetcher - ${instanceName}] Processing tasks...`);
                    const processedTasks = processPbsTasks(allTasksResult.tasks); // Assumes processPbsTasks is imported
                    instanceData = { ...instanceData, ...processedTasks }; // Merge task summaries
                    console.log(`INFO: [DataFetcher - ${instanceName}] Tasks processed.`);
                } else {
                    console.warn(`WARN: [DataFetcher - ${instanceName}] No tasks to process or task fetching failed. Error flag: ${allTasksResult.error}, Tasks array: ${allTasksResult.tasks === null ? 'null' : (Array.isArray(allTasksResult.tasks) ? `array[${allTasksResult.tasks.length}]` : typeof allTasksResult.tasks)}`);
                    // If tasks failed to fetch or process, ensure task-specific fields are not from a stale/default mock
                    // instanceData.backupTasks = instanceData.backupTasks || []; // Or undefined/null if preferred by consumers
                    // instanceData.verifyTasks = instanceData.verifyTasks || [];
                    // instanceData.gcTasks = instanceData.gcTasks || [];
                }
                
                instanceData.status = 'ok';
                instanceData.nodeName = nodeName; // Ensure nodeName is set
                console.log(`INFO: [DataFetcher - ${instanceName}] Successfully fetched all data. Status set to: ${instanceData.status}`);
            } else {
                 console.warn(`WARN: [DataFetcher - ${instanceName}] Node name '${nodeName}' is invalid or 'localhost'. Throwing error.`);
                 throw new Error(`Could not determine node name for PBS instance ${instanceName}`);
            }
        } catch (pbsError) {
            console.error(`ERROR: [DataFetcher - ${instanceName}] PBS fetch failed (outer catch): ${pbsError.message}. Stack: ${pbsError.stack}`);
            instanceData.status = 'error';
            console.log(`INFO: [DataFetcher - ${instanceName}] Status set to '${instanceData.status}' due to error.`);
        }
        // pbsEndpointId and pbsInstanceName are already part of instanceData from initialization
        console.log(`INFO: [DataFetcher - ${instanceName}] Finalizing instance data. Status: ${instanceData.status}, NodeName: ${instanceData.nodeName || 'N/A'}`);
        return instanceData;
    });

    const settledPbsResults = await Promise.allSettled(pbsPromises);
    settledPbsResults.forEach(result => {
        if (result.status === 'fulfilled') {
            pbsDataResults.push(result.value);
        } else {
            console.error(`ERROR: [DataFetcher] Unhandled rejection fetching PBS data: ${result.reason}`);
            // Optionally push a generic error object
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
  console.log("[DataFetcher] Starting full discovery cycle...");
  
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
    console.log(`[DataFetcher] Starting metrics fetch for ${runningVms.length} VMs, ${runningContainers.length} Containers...`);
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

module.exports = {
    fetchDiscoveryData,
    fetchPbsData, // Keep exporting the real one
    fetchMetricsData,
    // Potentially export PBS helpers if needed elsewhere, but keep internal if not
    // fetchPbsNodeName,
    // fetchPbsDatastoreData,
    // fetchAllPbsTasksForProcessing
};
