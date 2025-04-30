const { processPbsTasks } = require('./pbsUtils'); // Assuming pbsUtils.js exists or will be created

// Helper function reused from index.js (or import if shared)
async function fetchDataForNode(apiClient, endpointId, nodeName) {
  const nodeData = {
    vms: [],
    containers: [],
    nodeStatus: {},
    storage: []
  };

  // Fetch node status
  try {
    const statusResponse = await apiClient.get(`/nodes/${nodeName}/status`);
    if (statusResponse.data?.data) {
        nodeData.nodeStatus = statusResponse.data.data;
    } else {
        console.warn(`[DataFetcher - ${endpointId}-${nodeName}] Node status data missing or invalid format.`);
    }
  } catch (error) {
       console.error(`[DataFetcher - ${endpointId}-${nodeName}] Error fetching node status: ${error.message}`);
       // Allow proceeding even if status fails
  }

  // Fetch node storage
  try {
    const storageResponse = await apiClient.get(`/nodes/${nodeName}/storage`);
    if (storageResponse.data?.data && Array.isArray(storageResponse.data.data)) {
        nodeData.storage = storageResponse.data.data;
    } else {
        console.warn(`[DataFetcher - ${endpointId}-${nodeName}] Node storage data missing or invalid format.`);
    }
  } catch (error) {
      console.error(`[DataFetcher - ${endpointId}-${nodeName}] Error fetching node storage: ${error.message}`);
      // Allow proceeding even if storage fails
  }


  // --- Fetch VMs --- 
  try {
      const vmsResponse = await apiClient.get(`/nodes/${nodeName}/qemu`);
      if (vmsResponse.data?.data && Array.isArray(vmsResponse.data.data)) {
        nodeData.vms = vmsResponse.data.data.map(vm => ({ 
            ...vm, node: nodeName, endpointId: endpointId, type: 'qemu' 
        }));
      }
  } catch (error) {
       console.error(`[DataFetcher - ${endpointId}-${nodeName}] Error fetching VMs (qemu): ${error.message}`);
       // Proceed without VMs if fetch fails
  }
  
  // --- Fetch containers --- 
  try {
      const ctsResponse = await apiClient.get(`/nodes/${nodeName}/lxc`);
      if (ctsResponse.data?.data && Array.isArray(ctsResponse.data.data)) {
         nodeData.containers = ctsResponse.data.data.map(ct => ({ 
             ...ct, node: nodeName, endpointId: endpointId, type: 'lxc' 
         }));
      }
  } catch (error) {
       console.error(`[DataFetcher - ${endpointId}-${nodeName}] Error fetching Containers (lxc): ${error.message}`);
       // Proceed without containers if fetch fails
  }


  // Return all collected data, even if some parts failed.
  return {
      vms: nodeData.vms,
      containers: nodeData.containers,
      nodeStatus: nodeData.nodeStatus,
      storage: nodeData.storage,
  };
}

/**
 * Fetches structural PVE data: node list, statuses, VM/CT lists.
 * @param {Object} currentApiClients - Initialized PVE API clients.
 * @returns {Promise<Object>} - { nodes, vms, containers }
 */
async function fetchPveDiscoveryData(currentApiClients) {
    const pveEndpointIds = Object.keys(currentApiClients);
    let tempNodes = [], tempVms = [], tempContainers = [];

    if (pveEndpointIds.length === 0) {
        console.log("[DataFetcher] No PVE endpoints configured or initialized.");
        return { nodes: [], vms: [], containers: [] };
    }

    console.log(`[DataFetcher] Fetching PVE discovery data for ${pveEndpointIds.length} endpoints...`);
    const pvePromises = pveEndpointIds.map(endpointId =>
        (async () => {
            const { client: apiClientInstance, config } = currentApiClients[endpointId];
            const endpointName = config.name || endpointId;
            try {
                const nodesResponse = await apiClientInstance.get('/nodes');
                const nodes = nodesResponse.data.data;
                if (!nodes || !Array.isArray(nodes)) {
                    console.warn(`[DataFetcher - ${endpointName}] No nodes found or unexpected format.`);
                    return { endpointId: endpointName, status: 'fulfilled', value: { nodes: [], vms: [], containers: [] } };
                }

                const guestPromises = nodes.map(node => fetchDataForNode(apiClientInstance, endpointName, node.node));
                const guestResults = await Promise.allSettled(guestPromises);

                let endpointVms = [];
                let endpointContainers = [];
                let processedNodes = []; // New array to store results

                // Initialize endpointNodes with basic info from the /nodes call
                const defaultNode = {
                    cpu: null,
                    mem: null,
                    disk: null,
                    maxdisk: null,
                    uptime: 0,
                    loadavg: null,
                    status: 'unknown', // Default status
                    storage: [],       // Default storage
                };

                // Process guest results and merge status/storage into processedNodes
                guestResults.forEach((result, index) => {
                    const correspondingNodeInfo = nodes[index]; // Get the original info from /nodes
                    if (!correspondingNodeInfo || !correspondingNodeInfo.node) return;

                    const baseNode = {
                         // Explicit Defaults first:
                         cpu: null,
                         mem: null,
                         disk: null,
                         maxdisk: null,
                         uptime: 0,
                         loadavg: null,
                         status: 'unknown',
                         storage: [],
                         // Explicitly copy known/expected fields from correspondingNodeInfo:
                         node: correspondingNodeInfo.node,
                         maxcpu: correspondingNodeInfo.maxcpu, // Assuming these exist
                         maxmem: correspondingNodeInfo.maxmem,
                         level: correspondingNodeInfo.level,
                         // Set status based on correspondingNodeInfo, falling back to the default above:
                         status: correspondingNodeInfo.status || 'unknown',
                         // Set IDs:
                         id: `${endpointName}-${correspondingNodeInfo.node}`,
                         endpointId: endpointName,
                    };

                    if (result.status === 'fulfilled' && result.value) {
                        // --- Guest fetch succeeded --- 
                        const nodeData = result.value;
                        const currentEndpointId = endpointId;
                        endpointVms.push(...nodeData.vms.map(vm => ({...vm, endpointId: currentEndpointId, id: `${endpointName}-${vm.node}-${vm.vmid}`})));
                        endpointContainers.push(...nodeData.containers.map(ct => ({...ct, endpointId: currentEndpointId, id: `${endpointName}-${ct.node}-${ct.vmid}`})));
 
                        // Build the final node object, merging status/storage or keeping defaults
                        let finalNode = { ...baseNode }; // Copy base node
                        // Only merge status if nodeData.nodeStatus is not empty
                        if(nodeData.nodeStatus && Object.keys(nodeData.nodeStatus).length > 0) {
                            const statusData = nodeData.nodeStatus;
                            finalNode.cpu = statusData.cpu;
                            finalNode.mem = statusData.memory?.used || statusData.mem;
                            finalNode.disk = statusData.rootfs?.used || statusData.disk;
                            finalNode.maxdisk = statusData.rootfs?.total || statusData.maxdisk;
                            finalNode.uptime = statusData.uptime;
                            finalNode.loadavg = statusData.loadavg;
                            finalNode.status = statusData.uptime > 0 ? 'online' : baseNode.status; // Use baseNode status if uptime is 0
                        }
                        finalNode.storage = nodeData.storage || baseNode.storage; // Use baseNode storage if nodeData.storage is missing
                        
                        processedNodes.push(finalNode);
                         
                    } else { // Includes result.status === 'rejected' or other unexpected cases
                         // --- Guest fetch failed OR nodeData missing --- 
                         if (result.status === 'rejected') {
                              console.error(`[DataFetcher - ${endpointName}] Error processing node ${correspondingNodeInfo.node}: ${result.reason?.message || result.reason}`);
                         } else {
                              // Handle cases where status is fulfilled but value might be invalid
                              console.warn(`[DataFetcher - ${endpointName}] Unexpected result status for node ${correspondingNodeInfo.node}: ${result.status}`);
                         }
                         // Push the base node object (which has defaults correctly applied) 
                         processedNodes.push(baseNode);
                    }
                });
                
                // Return the newly constructed processedNodes array
                return { endpointId: endpointName, status: 'fulfilled', value: { nodes: processedNodes, vms: endpointVms, containers: endpointContainers } };
            } 
            /* istanbul ignore next */ // Ignore this catch block - tested via side effects (logging, filtering)
            catch (error) {
                 // This catch block handles failures in the initial /nodes call
                 const status = error.response?.status ? ` (Status: ${error.response.status})` : '';
                 console.error(`[DataFetcher - ${endpointName}] Error fetching PVE discovery data${status}: ${error.message}`);
                 // Return a specific structure indicating failure for THIS endpoint
                 return { endpointId: endpointName, status: 'rejected', reason: error.message || String(error) };
            }
        })()
    );

    const pveOutcomes = await Promise.allSettled(pvePromises);
    
    // Aggregate results from all endpoints, including partially successful ones
    pveOutcomes.forEach(endpointOutcome => {
        if (endpointOutcome.status === 'fulfilled') {
             if (endpointOutcome.value.status === 'fulfilled' && endpointOutcome.value.value) {
                 const { nodes, vms, containers } = endpointOutcome.value.value;
                 tempNodes.push(...nodes);
                 if (vms && Array.isArray(vms)) {
                     vms.forEach(vm => tempVms.push(vm));
                 }
                 if (containers && Array.isArray(containers)) {
                     containers.forEach(ct => tempContainers.push(ct));
                 }
            } else if (endpointOutcome.value.status === 'rejected') {
                // Log the reason for endpoint-level failure (e.g., /nodes failed)
                 console.error(`[DataFetcher] PVE discovery failed for endpoint: ${endpointOutcome.value.endpointId}. Reason: ${endpointOutcome.value.reason}`);
            }
        } else { 
            // This handles cases where the outer promise itself rejected (less likely with current structure)
            const reason = endpointOutcome.reason?.message || endpointOutcome.reason;
            // We might not know the endpoint ID here easily
            console.error(`[DataFetcher] Unhandled error processing PVE endpoint: ${reason}`);
        }
    });

    return { nodes: tempNodes, vms: tempVms, containers: tempContainers };
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
            params: { since: sinceTimestamp, limit: 1000, errors: 1 }
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
        let instanceData = { /* Initial structure */ };

        try {
            const nodeName = pbsClient.config.nodeName || await fetchPbsNodeName(pbsClient);
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

                const allTasksResult = await fetchAllPbsTasksForProcessing(pbsClient, nodeName);
                if (allTasksResult.tasks) {
                    const processedTasks = processPbsTasks(allTasksResult.tasks); // Assumes processPbsTasks is imported
                    instanceData = { ...instanceData, ...processedTasks }; // Merge task summaries
                }
                instanceData.status = 'ok';
                instanceData.nodeName = nodeName; // Ensure nodeName is set
            } else {
                 throw new Error(`Could not determine node name for PBS instance ${instanceName}`);
            }
        } catch (pbsError) {
            console.error(`ERROR: [DataFetcher] PBS fetch failed for ${instanceName}: ${pbsError.message}`);
            instanceData.status = 'error';
        }
        instanceData.pbsEndpointId = pbsClientId;
        instanceData.pbsInstanceName = instanceName;
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
        const { endpointId, node, vmid, type, name } = guest;
        if (!guestsByEndpointNode[endpointId]) {
            guestsByEndpointNode[endpointId] = {};
        }
        if (!guestsByEndpointNode[endpointId][node]) {
            guestsByEndpointNode[endpointId][node] = [];
        }
        guestsByEndpointNode[endpointId][node].push({ vmid, type, name: name || 'unknown' }); 
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
            
            // --- ADDED: Create promises to fetch metrics for each guest ---
            guestsOnNode.forEach(guestInfo => {
                const { vmid, type, name: guestName } = guestInfo;
                metricPromises.push(
                    (async () => {
                        try {
                            const pathPrefix = type === 'qemu' ? 'qemu' : 'lxc';
                            // Fetch RRD and Current Status data
                            const [rrdData, currentData] = await Promise.all([
                                apiClientInstance.get(`/nodes/${nodeName}/${pathPrefix}/${vmid}/rrddata`, { params: { timeframe: 'hour', cf: 'AVERAGE' } }),
                                apiClientInstance.get(`/nodes/${nodeName}/${pathPrefix}/${vmid}/status/current`)
                            ]);

                            const metricData = {
                                id: vmid,
                                guestName: guestName, 
                                node: nodeName,
                                type: type,
                                endpointId: endpointId, 
                                endpointName: endpointName, 
                                data: rrdData?.data?.data?.length > 0 ? rrdData.data.data : [],
                                current: currentData?.data?.data || null
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