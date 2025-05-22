const { processPbsTasks } = require('./pbsUtils'); // Assuming pbsUtils.js exists or will be created
const database = require('./database'); // Will be used by server/index.js, but good to be aware of

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
        // console.log("[DataFetcher] No PBS instances configured or initialized.");
        return pbsDataResults;
    }

    // console.log(`[DataFetcher] Fetching discovery data for ${pbsClientIds.length} PBS instances...`);
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
            // console.log(`INFO: [DataFetcher - ${instanceName}] Starting fetch. Initial status: ${instanceData.status}`);

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

  // console.log(`[DataFetcher] Discovery cycle completed. Found: ${aggregatedResult.nodes.length} PVE nodes, ${aggregatedResult.vms.length} VMs, ${aggregatedResult.containers.length} CTs, ${aggregatedResult.pbs.length} PBS instances.`);
  
  return aggregatedResult;
}

/**
 * Fetches dynamic metric data for running PVE guests.
 * @param {Array} runningVms - Array of running VM objects.
 * @param {Array} runningContainers - Array of running Container objects.
 * @param {Object} currentApiClients - Initialized PVE API clients.
 * @param {Object} metricAggregationBuffers - Object to store raw data for later aggregation.
 * @returns {Promise<Array>} - Array of metric data objects.
 */
async function fetchMetricsData(runningVms, runningContainers, currentApiClients, metricAggregationBuffers) {
    // console.log(`[DataFetcher] Starting metrics fetch for ${runningVms.length} VMs, ${runningContainers.length} Containers...`);
    const allMetrics = [];
    const metricPromises = [];
    const guestsByEndpointNode = {};
    const currentTime = Date.now(); // Use a single timestamp for all metrics fetched in this cycle

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
                const { vmid, type, name: guestName, agent: guestAgentConfigString } = guestInfo; // agent is a string like "enabled=1" or "1"
                
                metricPromises.push(
                    (async () => {
                        const pathPrefix = type === 'qemu' ? 'qemu' : 'lxc';
                        const baseGuestUrl = `/nodes/${nodeName}/${pathPrefix}/${vmid}`;
                        const guestUniqueId = `${endpointId}-${nodeName}-${vmid}`;
                        
                        const individualMetricPromises = [];

                        // --- RRD Data Promise ---
                        individualMetricPromises.push(apiClientInstance.get(`${baseGuestUrl}/rrddata`, { params: { timeframe: 'hour', cf: 'MAX' } }));
                        // --- Current Status Promise ---
                        individualMetricPromises.push(apiClientInstance.get(`${baseGuestUrl}/status/current`));
                        
                        // --- Agent Memory Promise (QEMU only) ---
                        const isQemuWithAgentEnabled = type === 'qemu' && guestAgentConfigString && (guestAgentConfigString.includes('enabled=1') || guestAgentConfigString === '1');
                        if (isQemuWithAgentEnabled) {
                            individualMetricPromises.push(apiClientInstance.post(`${baseGuestUrl}/agent/get-memory-block-info`, {}));
                        }

                        // Use Promise.allSettled to handle individual promise failures
                        const results = await Promise.allSettled(individualMetricPromises);
                        const rrdResult = results[0];
                        const currentResult = results[1];
                        const agentMemoryResult = isQemuWithAgentEnabled ? results[2] : null;

                        // --- Enhanced Debug Logging ---
                        // console.log(`[DEBUG fetchMetricsDataLoop - ${guestName}] RRD Status: ${rrdResult.status}, Current Status: ${currentResult.status}, Agent Mem Status: ${agentMemoryResult ? agentMemoryResult.status : 'N/A (Not QEMU or no agent)'}`);
                        // if (rrdResult.status === 'rejected') {
                        //      console.log(`[DEBUG fetchMetricsDataLoop - ${guestName}] RRD Reason:`, rrdResult.reason?.message); //, rrdResult.reason);
                        // }
                        // if (currentResult.status === 'rejected') {
                        //     console.log(`[DEBUG fetchMetricsDataLoop - ${guestName}] Current Reason:`, currentResult.reason?.message); //, currentResult.reason);
                        // }
                        // if (agentMemoryResult && agentMemoryResult.status === 'rejected') {
                        //     console.log(`[DEBUG fetchMetricsDataLoop - ${guestName}] Agent Reason:`, agentMemoryResult.reason?.message); //, agentMemoryResult.reason);
                        // }
                        // --- End Enhanced Debug Logging ---

                        // Check if essential data fetching failed
                        if (rrdResult.status === 'rejected' || currentResult.status === 'rejected') {
                            const reason = rrdResult.status === 'rejected' ? rrdResult.reason : currentResult.reason;
                            if (reason.response && reason.response.status === 400) {
                                console.warn(`[Metrics Cycle - ${endpointName}] Guest ${type} ${vmid} (${guestName}) on node ${nodeName} might be stopped or inaccessible (Status: 400). Skipping metrics.`);
                            } else {
                                // This is where the test 'should handle generic error fetching RRD/status data' expects a console.error
                                console.error(`[Metrics Cycle - ${endpointName}] Failed to get RRD/Current metrics for ${type} ${vmid} (${guestName}) on node ${nodeName} (Status: ${reason.response?.status || 'N/A'}): ${reason.message}`);
                            }
                            // console.log(`[DEBUG fetchMetricsDataLoop - ${guestName}] Returning NULL due to RRD/Current rejection.`);
                            return null; // Skip this guest
                        }

                        const rrdData = rrdResult.value?.data?.data || null;
                        let currentMetricsData = currentResult.value?.data?.data || null; // 'let' because agent data might be added
                        
                        // If currentMetricsData is null even if the promise was fulfilled (e.g. empty response from API)
                        if (!currentMetricsData) {
                             console.warn(`[Metrics Cycle - ${endpointName}] Guest ${type} ${vmid} (${guestName}) on node ${nodeName} has no Current metrics data despite successful API call. RRD only is not sufficient. Skipping.`);
                             // console.log(`[DEBUG fetchMetricsDataLoop - ${guestName}] Returning NULL due to no Current data post-fulfillment.`);
                             return null;
                        }

                        // --- Process Agent Memory Info ---
                        let guestMemoryDetails = null;
                        const liveAgentStatusAvailable = currentResult.status === 'fulfilled' && currentResult.value?.data?.data;
                        const liveAgentIsEnabled = liveAgentStatusAvailable && parseInt(currentResult.value.data.data.agent, 10) === 1;

                        if (isQemuWithAgentEnabled && liveAgentIsEnabled && agentMemoryResult && agentMemoryResult.status === 'fulfilled' && agentMemoryResult.value.data && agentMemoryResult.value.data.data && agentMemoryResult.value.data.data.result) {
                            guestMemoryDetails = agentMemoryResult.value.data.data.result;
                            // console.log(`[DEBUG fetchMetricsDataLoop - ${guestName}] QEMU Agent memory details processed.`);
                            if (guestMemoryDetails) {
                                const memInfo = Array.isArray(guestMemoryDetails) && guestMemoryDetails.length > 0 ? guestMemoryDetails[0] : guestMemoryDetails;
                                if (typeof memInfo === 'object' && memInfo !== null && memInfo.hasOwnProperty('total') && memInfo.hasOwnProperty('free')) {
                                    currentMetricsData.guest_mem_total_bytes = memInfo.total;
                                    currentMetricsData.guest_mem_free_bytes = memInfo.free;
                                    currentMetricsData.guest_mem_available_bytes = memInfo.available;
                                    currentMetricsData.guest_mem_cached_bytes = memInfo.cached;
                                    currentMetricsData.guest_mem_buffers_bytes = memInfo.buffers;
                                    if (memInfo.available !== undefined) {
                                        currentMetricsData.guest_mem_actual_used_bytes = memInfo.total - memInfo.available;
                                    } else if (memInfo.cached !== undefined && memInfo.buffers !== undefined) {
                                        currentMetricsData.guest_mem_actual_used_bytes = memInfo.total - memInfo.free - memInfo.cached - memInfo.buffers;
                                    } else {
                                        currentMetricsData.guest_mem_actual_used_bytes = memInfo.total - memInfo.free;
                                    }
                                } else {
                                     console.warn(`[Metrics Cycle - ${endpointName}] VM ${vmid} (${guestName}): Guest agent mem format not as expected (object structure). Data:`, memInfo);
                                }
                            }
                        } else if (isQemuWithAgentEnabled && agentMemoryResult && agentMemoryResult.status === 'rejected') {
                            const qemuAgentError = agentMemoryResult.reason;
                            const agentErrorCode = qemuAgentError.response?.data?.data?.exitcode;
                            const agentErrorMessage = qemuAgentError.message;
                            const agentResponseStatus = qemuAgentError.response?.status;

                            if (agentErrorCode === -2 || (agentResponseStatus === 500 && agentErrorMessage && agentErrorMessage.toLowerCase().includes('command not supported'))) {
                                console.log(`[Metrics Cycle - ${endpointName}] VM ${vmid} (${guestName}): QEMU Guest Agent not responsive or cmd not supported. Status: ${agentResponseStatus || 'N/A'}, Msg: ${agentErrorMessage}`);
                            } else {
                                console.warn(`[Metrics Cycle - ${endpointName}] VM ${vmid} (${guestName}): Error fetching guest agent memory. Status: ${agentResponseStatus || 'N/A'}, Msg: ${agentErrorMessage}`);
                            }
                        } else if (isQemuWithAgentEnabled && liveAgentStatusAvailable && !liveAgentIsEnabled) {
                            // Agent was enabled in discovery, but live status says it's off. Don't use agentMemoryResult.
                            // console.log(`[DEBUG fetchMetricsDataLoop - ${guestName}] QEMU Agent was enabled in discovery but is now reported as OFF in current status. Skipping agent memory data.`);
                        } else if (isQemuWithAgentEnabled && !agentMemoryResult) {
                            // This case implies isQemuWithAgentEnabled was true, but there was no third promise result (e.g., results.length < 3)
                            // This shouldn't happen if individualMetricPromises.push for agent was conditional on isQemuWithAgentEnabled
                            console.warn(`[Metrics Cycle - ${endpointName}] VM ${vmid} (${guestName}): QEMU Agent was enabled in discovery, but no agent memory result was obtained unexpectedly.`);
                        }

                        // If RRD or Current Status failed, skip this guest
                        if (rrdResult.status === 'rejected' || currentResult.status === 'rejected') {
                            // console.log(`[DEBUG fetchMetricsDataLoop - ${guestName}] Returning NULL due to RRD/Current rejection.`);
                            return null; // Skip this guest
                        }

                        // Ensure buffer exists for this guest (for historical data aggregation)
                        if (!metricAggregationBuffers[guestUniqueId]) {
                            metricAggregationBuffers[guestUniqueId] = {};
                        }
                        const guestBuffer = metricAggregationBuffers[guestUniqueId];

                        const addToBuffer = (metricKey, value) => {
                            if (typeof value === 'number' && !isNaN(value)) {
                                if (!guestBuffer[metricKey]) {
                                    guestBuffer[metricKey] = [];
                                }
                                guestBuffer[metricKey].push({ timestamp: currentTime, value });
                                if (guestBuffer[metricKey].length === 1) {
                                    if (metricKey === 'cpu_usage_percent' || metricKey === 'memory_usage_percent' || metricKey === 'disk_usage_percent') {
                                        database.insertMetricData(currentTime, guestUniqueId, metricKey, value, (err) => {
                                            if (err) {
                                                console.error(`[DataFetcher DEBUG] DB ERROR (immediate insert) for ${guestUniqueId} - ${metricKey}:`, err);
                                            }
                                        });
                                    }
                                }
                            }
                        };
                        
                        // Process and buffer standard metrics
                        if (currentMetricsData.cpu !== undefined) {
                            addToBuffer('cpu_usage_percent', currentMetricsData.cpu * 100);
                        }
                        if (currentMetricsData.mem !== undefined && currentMetricsData.maxmem !== undefined && currentMetricsData.maxmem > 0) {
                            addToBuffer('memory_usage_percent', (currentMetricsData.mem / currentMetricsData.maxmem) * 100);
                        }
                         // Use guest_mem_actual_used_bytes if available from agent for host memory usage
                        if (currentMetricsData.guest_mem_actual_used_bytes !== undefined && currentMetricsData.guest_mem_total_bytes !== undefined && currentMetricsData.guest_mem_total_bytes > 0) {
                            addToBuffer('guest_memory_actual_usage_percent', (currentMetricsData.guest_mem_actual_used_bytes / currentMetricsData.guest_mem_total_bytes) * 100);
                        }

                        if (type === 'lxc' && currentMetricsData.disk !== undefined && currentMetricsData.maxdisk !== undefined && currentMetricsData.maxdisk > 0) {
                            addToBuffer('disk_usage_percent', (currentMetricsData.disk / currentMetricsData.maxdisk) * 100);
                        }
                        if (currentMetricsData.diskread !== undefined) {
                            addToBuffer('disk_read_total_bytes', currentMetricsData.diskread);
                        }
                        if (currentMetricsData.diskwrite !== undefined) {
                            addToBuffer('disk_write_total_bytes', currentMetricsData.diskwrite);
                        }
                        if (currentMetricsData.netin !== undefined) {
                            addToBuffer('net_in_total_bytes', currentMetricsData.netin);
                        }
                        if (currentMetricsData.netout !== undefined) {
                            addToBuffer('net_out_total_bytes', currentMetricsData.netout);
                        }
                        
                        // Return the structured metric data for this guest for real-time updates.
                        // The historical data is managed via metricAggregationBuffers.
                        return {
                            id: vmid,
                            node: nodeName,
                            endpointId: endpointId, // Original endpointId for this guest
                            endpointName: endpointName, // Configured name for the endpoint
                            type: type,
                            guestName: guestName, 
                            uniqueId: guestUniqueId,
                            data: rrdData || [], // Historical RRD data (if fetched successfully)
                            current: currentMetricsData // Current status data, potentially augmented with agent info
                        };
                        // Removed the outer try-catch as Promise.allSettled handles individual failures,
                        // and the main error conditions (RRD/Current failure) are handled above.
                    })()
                );
            }); // End forEach guestInfo
        } // End for nodeName
    } // End for endpointId

    // Wait for all metric fetch promises to settle
    const metricResults = await Promise.allSettled(metricPromises);

    // Collect results (existing logic)
    // console.log(`[DEBUG fetchMetricsData End] Processing ${metricResults.length} metricResults...`); // Added log
    metricResults.forEach((result, index) => {
        // console.log(`[DEBUG fetchMetricsData End] Result ${index}: Status: ${result.status}`); // Added log
        if (result.status === 'fulfilled') {
            // console.log(`[DEBUG fetchMetricsData End] Result ${index} Value:`, result.value ? 'Exists' : 'null'); // Added log
            if (result.value) { // Ensure value is not null (it would be if the inner async func returned null)
                allMetrics.push(result.value);
            } else {
                // console.log(`[DEBUG fetchMetricsData End] Result ${index} had status fulfilled but value was null/falsy.`); // Added log
            }
        } else {
            // console.log(`[DEBUG fetchMetricsData End] Result ${index} Rejected Reason:`, result.reason?.message); // Added log
            // Individual promise rejections within the loop are already handled (logged, guest skipped).
            // This .else here would only catch issues with the Promise.allSettled mechanism itself or unhandled errors from the outer loop structure, which are less likely.
        }
    });

    // console.log(`[DataFetcher] Metrics fetch completed. Aggregated ${allMetrics.length} valid guest metrics.`);
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
