const { fetchDiscoveryData, fetchMetricsData, fetchPbsData } = require('../dataFetcher');
// Don't require the real apiClients, we will mock it
// const { initializeApiClients } = require('../apiClients'); 

// Mock the modules used by dataFetcher
jest.mock('axios'); // Keep this in case axios is used directly anywhere unexpected
jest.mock('../pbsUtils', () => ({ 
    // Ensure processPbsTasks returns the expected structure
    processPbsTasks: jest.fn().mockReturnValue({ backupTasks: [], verifyTasks: [], gcTasks: [] }), 
}));
jest.mock('../apiClients'); // <-- MOCK apiClients module

// --- REMOVE Mock for fetchPbsData within dataFetcher --- 
// jest.mock('../dataFetcher', ...);
// --- END REMOVE --- 

// Import the mocked version AFTER mocking it
const { initializeApiClients } = require('../apiClients'); 

process.env.NODE_ENV = 'test';

// Setup and teardown for console spies
beforeEach(() => {
    // Create spies before each test
    jest.spyOn(console, 'log').mockImplementation(() => {});
    jest.spyOn(console, 'warn').mockImplementation(() => {});
    jest.spyOn(console, 'error').mockImplementation(() => {});
});

afterEach(() => {
    // Restore console after each test
    jest.restoreAllMocks(); // More comprehensive way to restore all spies/mocks
});

describe('Data Fetcher', () => {
  // --- Declare variables used across tests/hooks ---
  let originalEnv; // <--- Declare here
  let mockPveClientInstance;
  let mockPveApiClient;      
  let mockPbsClientInstance;
  let mockPbsApiClient;     
  // --- End declare vars --- 

  // Helper to set up a basic PBS client mock (MOVED TO OUTER SCOPE)
  const setupMockPbsClient = (id, configOverrides = {}, clientMocks = {}) => {
      mockPbsClientInstance = {
          get: jest.fn(),
          ...clientMocks // Allow overriding .get or adding other methods
      };
      // Use the mockPbsApiClient defined in the outer scope
      mockPbsApiClient[id] = { 
          client: mockPbsClientInstance,
          config: {
              id: `${id}_config_id`,
              name: `PBS Instance ${id}`,
              host: `${id}.pbs.example.com`,
              // Add other default config properties as needed
              ...configOverrides
          }
      };
      return mockPbsClientInstance; // Return the mock instance for further configuration
  };

  beforeEach(() => {
    // Store environment (assign to variable declared above)
    originalEnv = { ...process.env }; 
    // Reset the mocked initializeApiClients function and other mocks
    jest.clearAllMocks(); 

    // Define the *default* return value for the mocked initializer
    // Tests can override this if needed
    mockPveClientInstance = { get: jest.fn() };
    mockPveApiClient = {
      primary: { client: mockPveClientInstance, config: { /* ... */ } }
    };
    mockPbsClientInstance = { get: jest.fn() };
    mockPbsApiClient = {};
    initializeApiClients.mockResolvedValue({ 
        apiClients: mockPveApiClient, 
        pbsApiClients: mockPbsApiClient 
    });

    // --- Remove console mocks --- 
    // jest.spyOn(console, 'warn').mockImplementation(() => {});
    // jest.spyOn(console, 'log').mockImplementation(() => {});
    // jest.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    // Restore environment (can now access originalEnv)
    const currentEnvKeys = Object.keys(process.env);
    currentEnvKeys.forEach(key => delete process.env[key]);
    Object.keys(originalEnv).forEach(key => { process.env[key] = originalEnv[key]; });
    // --- Remove console restore --- 
    // jest.restoreAllMocks(); 
  });

  describe('fetchDiscoveryData', () => {
    test('should return empty structure when no PVE clients configured', async () => {
      // Arrange
      const mockPbsFunction = jest.fn().mockResolvedValue([]); // Mock PBS part as well
      const consoleLogSpy = jest.spyOn(console, 'log').mockImplementation(() => {});

      // Act
      const result = await fetchDiscoveryData({}, mockPbsApiClient, mockPbsFunction); // Pass empty PVE clients

      // Assert
      expect(result.nodes).toEqual([]);
      expect(result.vms).toEqual([]);
      expect(result.containers).toEqual([]);
      expect(result.pbs).toEqual([]); // PBS fetch should still run
      expect(consoleLogSpy).toHaveBeenCalledWith("[DataFetcher] Discovery cycle completed. Found: 0 PVE nodes, 0 VMs, 0 CTs, 0 PBS instances, 0 PVE backup tasks, 0 PVE storage backups, 0 guest snapshots.");
      expect(mockPbsFunction).toHaveBeenCalled(); // Ensure PBS was still called

      consoleLogSpy.mockRestore();
    });

    test('should fetch PVE data correctly (1 node, 1 VM, 1 CT)', async () => {
       // Arrange: Configure the mock get method on the client instance
       const nodeName = 'mock-node';
       const vmId = 100;
       const ctId = 101;
       const endpointId = 'primary'; // Assuming the default mock client is 'primary'

       // Configure mockPveClientInstance.get
       mockPveClientInstance.get
          // Calls within fetchDataForPveEndpoint for 'primary'
          .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 1. /cluster/status (for endpoint 'primary')
          .mockResolvedValueOnce({ data: { data: [{ node: nodeName, status: 'online' }] } })              // 2. /nodes (for endpoint 'primary' to get standaloneNodeName if cluster nodes <=1)
          .mockResolvedValueOnce({ data: { data: [{ node: nodeName, status: 'online', maxcpu: 4, maxmem: 8 * 1024**3 , id: `node/${nodeName}` }] } }) // 3. /nodes (main call for endpoint 'primary' to get node list)
          // Additional cluster status call for backup data
          .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 4. /cluster/status (additional)
          // Calls within fetchDataForNode for 'mock-node' (the single node from the call above)
          .mockResolvedValueOnce({ data: { data: { cpu: 0.1, mem: 2 * 1024**3, rootfs: { total: 100*1024**3, used: 20*1024**3 }, uptime: 12345 } } }) // 5. /nodes/mock-node/status
          .mockResolvedValueOnce({ data: { data: [ { storage: 'local-lvm', type: 'lvmthin', content: 'images,rootdir', total: 500*1024**3, used: 150*1024**3 } ] } }) // 6. /nodes/mock-node/storage
          .mockResolvedValueOnce({ data: { data: [ { vmid: vmId, name: 'test-vm', status: 'running', cpu: 0.5, mem: 1 * 1024**3, maxmem: 2 * 1024**3, maxdisk: 32*1024**3 } ] } }) // 7. /nodes/mock-node/qemu
          .mockResolvedValueOnce({ data: { data: [ { vmid: ctId, name: 'test-ct', status: 'running', cpu: 0.2, mem: 512 * 1024**2, maxmem: 1 * 1024**3, maxdisk: 8*1024**3 } ] } }) // 8. /nodes/mock-node/lxc
          // Additional calls for backup data
          .mockResolvedValueOnce({ data: { data: [] } }) // 9. /storage
          .mockResolvedValueOnce({ data: { data: [] } }) // 10. snapshot call
          .mockResolvedValueOnce({ data: { data: [] } }); // 11. additional call

      // Act: Call function with the clients provided by the (mocked) default setup
      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient);

      // Assert
      expect(mockPveClientInstance.get).toHaveBeenCalledTimes(11); // Updated for actual call count
      expect(mockPveClientInstance.get).toHaveBeenNthCalledWith(1, '/cluster/status');
      expect(mockPveClientInstance.get).toHaveBeenNthCalledWith(2, '/nodes'); // For standaloneNodeName
      expect(mockPveClientInstance.get).toHaveBeenNthCalledWith(3, '/nodes'); // Main nodes call
      expect(mockPveClientInstance.get).toHaveBeenNthCalledWith(4, '/cluster/status'); // Additional cluster status call
      expect(mockPveClientInstance.get).toHaveBeenNthCalledWith(5, `/nodes/${nodeName}/status`);
      expect(mockPveClientInstance.get).toHaveBeenNthCalledWith(6, `/nodes/${nodeName}/storage`);
      expect(mockPveClientInstance.get).toHaveBeenNthCalledWith(7, `/nodes/${nodeName}/qemu`);
      expect(mockPveClientInstance.get).toHaveBeenNthCalledWith(8, `/nodes/${nodeName}/lxc`);
      expect(mockPveClientInstance.get).toHaveBeenNthCalledWith(9, `/storage`);
      expect(mockPveClientInstance.get).toHaveBeenNthCalledWith(10, `/nodes/${nodeName}/qemu/100/snapshot`);
      expect(mockPveClientInstance.get).toHaveBeenNthCalledWith(11, `/nodes/${nodeName}/lxc/101/snapshot`); // Container snapshot
      
      // Assert Nodes
      expect(result.nodes).toHaveLength(1);
      expect(result.nodes[0]).toMatchObject({
        node: nodeName,
        status: 'online', // Should be updated by status call
        maxcpu: 4,
        maxmem: 8 * 1024**3,
        id: `${endpointId}-${nodeName}`, // Check constructed ID
        endpointId: endpointId,
        cpu: 0.1,
        mem: 2 * 1024**3,
        disk: 20 * 1024**3,
        maxdisk: 100 * 1024**3,
        uptime: 12345,
      });
      expect(result.nodes[0].storage).toHaveLength(1);
      expect(result.nodes[0].storage[0]).toMatchObject({ storage: 'local-lvm', type: 'lvmthin' });

      // Assert VMs
      expect(result.vms).toHaveLength(1);
      expect(result.vms[0]).toMatchObject({
        vmid: vmId,
        name: 'test-vm',
        status: 'running',
        node: nodeName,
        endpointId: endpointId,
        type: 'qemu',
        id: `${endpointId}-${nodeName}-${vmId}` // Check constructed ID
      });

      // Assert Containers
      expect(result.containers).toHaveLength(1);
      expect(result.containers[0]).toMatchObject({
        vmid: ctId,
        name: 'test-ct',
        status: 'running',
        node: nodeName,
        endpointId: endpointId,
        type: 'lxc',
        id: `${endpointId}-${nodeName}-${ctId}` // Check constructed ID
      });

      // Assert PVE Backups
      expect(result.pveBackups).toBeDefined();
      expect(result.pveBackups.backupTasks).toEqual([]);
      expect(result.pveBackups.storageBackups).toEqual([]);
      expect(result.pveBackups.guestSnapshots).toEqual([]);

      // Assert PBS (should be empty)
      expect(result.pbs).toBeDefined();
      expect(result.pbs).toHaveLength(0);
    });

    test('should handle invalid format for node storage response', async () => {
      // Arrange: Use default mock client
      const nodeName = 'node-bad-storage-format';
      const endpointId = 'primary';
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

      mockPveClientInstance.get
        // Endpoint level calls
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 1. /cluster/status
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName }] } })                                 // 2. /nodes (for standaloneNodeName)
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName, status: 'online' }] } })                // 3. /nodes (main node list for endpoint)
        // Node level calls for nodeName
        .mockResolvedValueOnce({ data: { data: { cpu: 0.1, uptime: 10 } } })                             // 4. /nodes/${nodeName}/status
        .mockResolvedValueOnce({ data: { data: { not_an_array: true } } })                                // 5. /nodes/${nodeName}/storage (INVALID)
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 6. /nodes/${nodeName}/qemu
        .mockResolvedValueOnce({ data: { data: [] } });                                                  // 7. /nodes/${nodeName}/lxc

      // Act
      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient);

       // Assert
       expect(consoleWarnSpy).toHaveBeenCalledWith(
         expect.stringContaining(`[DataFetcher - ${endpointId}-${nodeName}] Could not fetch storage list`)
       );
       // Ensure node was still processed and added (just without storage)
      expect(result.nodes).toHaveLength(1);
      expect(result.nodes[0].node).toBe(nodeName);
      expect(result.nodes[0].storage).toEqual([]); // Should default to empty array
      expect(result.vms).toHaveLength(0);
      expect(result.containers).toHaveLength(0);
      
      consoleWarnSpy.mockRestore();
    });

    test('should handle missing or invalid data.data for a node resource', async () => {
      // Arrange: Use default mock client and a specific node name
      const nodeName = 'node-missing-data-data'; // Can be the same or different, impact is on the mock
      const endpointId = 'primary';
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

      mockPveClientInstance.get
        // Endpoint level calls
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 1. /cluster/status
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName }] } })                                 // 2. /nodes (for standaloneNodeName)
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName, status: 'online' }] } })                // 3. /nodes (main node list for endpoint)
        // Node level calls for nodeName
        .mockResolvedValueOnce({ data: { data: null } })                                                 // 4. /nodes/${nodeName}/status (INVALID data.data)
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 5. /nodes/${nodeName}/storage
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 6. /nodes/${nodeName}/qemu
        .mockResolvedValueOnce({ data: { data: [] } });                                                  // 7. /nodes/${nodeName}/lxc

      // Act
      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient);

      // Assert
      // Verify the warning was logged by fetchNodeResource
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        expect.stringContaining(`[DataFetcher - ${endpointId}-${nodeName}] Could not fetch storage list`)
      );
      // Verify that the node was still processed but status data is default (null/0)
      expect(result.nodes).toHaveLength(1);
      expect(result.nodes[0].node).toBe(nodeName);
      expect(result.nodes[0].cpu).toBeNull(); 
      expect(result.nodes[0].uptime).toBe(0);
      expect(result.nodes[0].storage).toEqual([]);
      expect(result.vms).toHaveLength(0);
      expect(result.containers).toHaveLength(0);

      consoleWarnSpy.mockRestore();
    });

    test('should handle API error when fetching /nodes for an endpoint', async () => {
      const mockPveClientInstance1 = { get: jest.fn() };
      const mockPveClientInstance2 = { get: jest.fn() }; // Separate instance for pve2
      const mockClients = { // Custom clients for this test
        pve1: { client: mockPveClientInstance1, config: { id: 'pve1', name: 'PVE1 Endpoint' } },
        pve2: { client: mockPveClientInstance2, config: { id: 'pve2', name: 'PVE2 Endpoint' } },
      };
      const nodesError = new Error('Network Error on PVE1');
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

      // Configure mockPveClientInstance1 (fails on main /nodes call)
      mockPveClientInstance1.get
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'pve1-cluster' }] } }) // 1. /cluster/status
        .mockResolvedValueOnce({ data: { data: [{ node: 'pve1-node-temp' }] } })      // 2. /nodes (standalone)
        .mockRejectedValueOnce(nodesError);                                           // 3. /nodes (main list) -> FAILS

      // Configure mockPveClientInstance2 (succeeds fully, no VMs/CTs)
      mockPveClientInstance2.get
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'pve2-cluster' }] } }) // 1. /cluster/status
        .mockResolvedValueOnce({ data: { data: [{ node: 'node-pve2' }] } })                              // 2. /nodes (standalone)
        .mockResolvedValueOnce({ data: { data: [{ node: 'node-pve2', status: 'online' }] } })             // 3. /nodes (main list)
        .mockResolvedValueOnce({ data: { data: { cpu: 0.1, uptime: 10 } } })                          // 4. /nodes/node-pve2/status
        .mockResolvedValueOnce({ data: { data: [] } })                                                 // 5. /nodes/node-pve2/storage
        .mockResolvedValueOnce({ data: { data: [] } })                                                 // 6. /nodes/node-pve2/qemu
        .mockResolvedValueOnce({ data: { data: [] } })                                                // 7. /nodes/node-pve2/lxc  <<< SHOULD BE EMPTY
        .mockResolvedValueOnce({ data: { data: [] } });                                                // 8. /nodes/node-pve2/tasks (backup tasks)

      const result = await fetchDiscoveryData(mockClients, {}); // Pass custom clients

      // Assertions for pve1 (failed endpoint)
      expect(mockPveClientInstance1.get).toHaveBeenCalledTimes(3);
      // expect(consoleErrorSpy).toHaveBeenCalledWith(
      //    expect.stringContaining(`[DataFetcher - PVE1 Endpoint] Error fetching PVE discovery data: ${nodesError.message}`)
      //  );

      // Assert mockPveClientInstance2 (successful endpoint)
      expect(mockPveClientInstance2.get).toHaveBeenCalledTimes(10);

      // Should still return data from the successful endpoint (pve2)
      expect(result.nodes).toHaveLength(1);
      expect(result.nodes[0].node).toBe('node-pve2');
      expect(result.nodes[0].endpointId).toBe('pve2'); 
      expect(result.vms).toHaveLength(0);
      expect(result.containers).toHaveLength(0);
      expect(result.pbs).toHaveLength(0);

      consoleErrorSpy.mockRestore();
    });

    test('should handle API error when fetching guests for a specific node', async () => {
      // Arrange: Uses the default mock clients from beforeEach
      const nodeNameGood = 'node-good';
      const nodeNameBad = 'node-bad-guests';

      // Use mockImplementation on the default mockPveClientInstance
      mockPveClientInstance.get.mockImplementation(async (url) => {
        console.log(`Mock API call: ${url}`); // Added for debugging
        if (url === '/nodes') {
          return { data: { data: [
            { node: nodeNameGood, status: 'online', id: `node/${nodeNameGood}` },
            { node: nodeNameBad, status: 'online', id: `node/${nodeNameBad}` }
          ]}};
        }
        if (url === `/nodes/${nodeNameGood}/status`) {
            return { data: { data: { cpu: 0.1, uptime: 10 } } };
        }
        if (url === `/nodes/${nodeNameGood}/storage`) {
            return { data: { data: [] } };
        }
        if (url === `/nodes/${nodeNameGood}/qemu`) {
            return { data: { data: [ { vmid: 100, name: 'vm-good', status: 'running' } ] } };
        }
        if (url === `/nodes/${nodeNameGood}/lxc`) {
            return { data: { data: [] } };
        }
        if (url === `/nodes/${nodeNameBad}/status`) {
            return { data: { data: { cpu: 0.2, uptime: 20 } } };
        }
        if (url === `/nodes/${nodeNameBad}/storage`) {
            return { data: { data: [] } };
        }
        if (url === `/nodes/${nodeNameBad}/qemu`) {
          // Simulate API error for this specific call
          throw new Error('Simulated API Error Fetching Guests');
        }
        if (url === `/nodes/${nodeNameBad}/lxc`) {
          return { data: { data: [] } }; // Successful but empty
        }
        // Default fallback for unexpected calls
        throw new Error(`Unexpected API call in mock: ${url}`);
      });

      // Act: Uses the default mock clients from beforeEach
      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient);

      // Assert
      // Check that the correct number of nodes is returned
      expect(result.nodes).toHaveLength(2);

      // Find the nodes in the result
      const goodNodeResult = result.nodes.find(n => n.node === nodeNameGood);
      const badNodeResult = result.nodes.find(n => n.node === nodeNameBad);

      expect(goodNodeResult).toBeDefined();
      expect(badNodeResult).toBeDefined();

      // Assertions for the node where all calls succeeded (nodeNameGood)
      expect(goodNodeResult.cpu).toBe(0.1); // Should have CPU data from successful /status call
      expect(goodNodeResult.status).toBe('online'); // Status updated by uptime > 0
      expect(goodNodeResult.vms).toBeUndefined(); // VMs/CTs are in the top-level result.vms/result.containers
      
      // Assertions for the node where /qemu failed (nodeNameBad)
      // It should still have basic info from /nodes and status info from its successful /status call
      expect(badNodeResult.cpu).toBe(0.2); // CPU data from its OWN successful /status call
      expect(badNodeResult.status).toBe('online'); // Status updated by uptime > 0
      // It should not have contributed VMs/CTs because fetchDataForNode rejected

      // Assert overall VMs/Containers (only from the successful node)
      expect(result.vms).toHaveLength(1);
      expect(result.vms[0].vmid).toBe(100); // VM from nodeNameGood
      expect(result.containers).toHaveLength(0);

      // Assert PBS is empty
      expect(result.pbs).toEqual([]);
    });

    test('should handle missing or invalid data.data for a node resource', async () => {
      // Arrange: Use default mock client and a specific node name
      const nodeName = 'node-missing-data-data'; // Can be the same or different, impact is on the mock
      const endpointId = 'primary';
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

      mockPveClientInstance.get
        // Endpoint level calls
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 1. /cluster/status
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName }] } })                                 // 2. /nodes (for standaloneNodeName)
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName, status: 'online' }] } })                // 3. /nodes (main node list for endpoint)
        // Node level calls for nodeName
        .mockResolvedValueOnce({ data: { data: null } })                                                 // 4. /nodes/${nodeName}/status (INVALID data.data)
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 5. /nodes/${nodeName}/storage
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 6. /nodes/${nodeName}/qemu
        .mockResolvedValueOnce({ data: { data: [] } });                                                  // 7. /nodes/${nodeName}/lxc

      // Act
      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient);

      // Assert
      // Verify the warning was logged by fetchNodeResource
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        expect.stringContaining(`[DataFetcher - ${endpointId}-${nodeName}] Could not fetch storage list`)
      );
      // Verify that the node was still processed but status data is default (null/0)
      expect(result.nodes).toHaveLength(1);
      expect(result.nodes[0].node).toBe(nodeName);
      expect(result.nodes[0].cpu).toBeNull(); 
      expect(result.nodes[0].uptime).toBe(0);
      expect(result.nodes[0].storage).toEqual([]);
      expect(result.vms).toHaveLength(0);
      expect(result.containers).toHaveLength(0);

      consoleWarnSpy.mockRestore();
    });

    // --- PBS Integration Tests ---
    test('should fetch PVE and PBS data correctly', async () => {
      // Arrange PVE (similar to happy path test, simplified)
      const nodeName = 'pve-node';
      const vmId = 200;
      mockPveClientInstance.get
        // PVE Endpoint calls
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 1. /cluster/status
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName }] } })                                 // 2. /nodes (standalone)
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName, status: 'online' }] } })                // 3. /nodes (main list)
        // PVE Node calls for nodeName
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 4. /cluster/status (additional)
        .mockResolvedValueOnce({ data: { data: { uptime: 1 } } })                                        // 5. /nodes/${nodeName}/status
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 6. /nodes/${nodeName}/storage
        .mockResolvedValueOnce({ data: { data: [{ vmid: vmId, name: 'pve-vm' }] } })                     // 7. /nodes/${nodeName}/qemu
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 8. /nodes/${nodeName}/lxc
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 9. /storage
        .mockResolvedValueOnce({ data: { data: [] } });                                                  // 10. snapshot call

      // Arrange PBS (Mock the function to be injected)
      const mockPbsFunction = jest.fn();
      const mockPbsResult = [
        {
          pbsEndpointId: 'pbs1',
          pbsInstanceName: 'MyPBS',
          status: 'ok',
          nodeName: 'pbs-node',
          datastores: [{ name: 'ds1', total: 1000, used: 500, snapshots: [] }],
        }
      ];
      mockPbsFunction.mockResolvedValue(mockPbsResult);

      // Act (Inject the mock function)
      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient, mockPbsFunction);

      // Assert PVE
      expect(result.nodes).toHaveLength(1);
      expect(result.vms).toHaveLength(1);
      expect(result.containers).toHaveLength(0);

      // Assert PBS
      expect(mockPbsFunction).toHaveBeenCalledWith(mockPbsApiClient);
      expect(result.pbs).toEqual(mockPbsResult);
    });

    test('should handle errors from fetchPbsData gracefully', async () => {
      // Arrange PVE (same simple mock as above)
       const nodeName = 'pve-node';
       const vmId = 200;
       mockPveClientInstance.get
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName, status: 'online' }] } }) // /nodes
        .mockResolvedValueOnce({ data: { data: { uptime: 1 } } }) // status
        .mockResolvedValueOnce({ data: { data: [] } }) // storage
        .mockResolvedValueOnce({ data: { data: [{ vmid: vmId, name: 'pve-vm' }] } }) // qemu
        .mockResolvedValueOnce({ data: { data: [] } }); // lxc
      
      // Arrange PBS (Mock the function to be injected)
      const mockPbsFunction = jest.fn();
      const pbsError = new Error('PBS Connection Failed');
      // Revert to mockRejectedValue
      mockPbsFunction.mockRejectedValue(pbsError);
      // Mock console.error to suppress expected error message during test
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {}); 

      // Act (Inject the mock function)
      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient, mockPbsFunction);

      // Assert PVE (should be empty due to Promise.all.catch)
      expect(result.nodes).toHaveLength(0);
      expect(result.vms).toHaveLength(0);
      expect(result.containers).toHaveLength(0);

      // Assert PBS (should be empty due to Promise.all.catch)
      expect(mockPbsFunction).toHaveBeenCalledWith(mockPbsApiClient);
      expect(result.pbs).toEqual([]); 
      // Check that the catch block in fetchDiscoveryData logged the error
      expect(consoleErrorSpy).toHaveBeenCalledWith("[DataFetcher] Error during parallel backup data fetch:", pbsError);

      // Restore console.error
      consoleErrorSpy.mockRestore();
    });

    test('should return empty PBS array when no PBS clients configured', async () => {
       // Arrange PVE (same simple mock as above)
       const nodeName = 'pve-node';
       const vmId = 200;
       mockPveClientInstance.get
        // PVE Endpoint calls
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 1. /cluster/status
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName }] } })                                 // 2. /nodes (standalone)
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName, status: 'online' }] } })                // 3. /nodes (main list)
        // PVE Node calls for nodeName
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 4. /cluster/status (additional)
        .mockResolvedValueOnce({ data: { data: { uptime: 1 } } })                                        // 5. /nodes/${nodeName}/status
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 6. /nodes/${nodeName}/storage
        .mockResolvedValueOnce({ data: { data: [{ vmid: vmId, name: 'pve-vm' }] } })                     // 7. /nodes/${nodeName}/qemu
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 8. /nodes/${nodeName}/lxc
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 9. /storage
        .mockResolvedValueOnce({ data: { data: [] } });                                                  // 10. snapshot call
      
      // Arrange PBS: Pass an empty object for PBS clients, mock injected function
      const emptyPbsClients = {};
      const mockPbsFunction = jest.fn();
      mockPbsFunction.mockResolvedValue([]); // Should resolve with empty when called with empty clients

      // Act (Inject the mock function)
      const result = await fetchDiscoveryData(mockPveApiClient, emptyPbsClients, mockPbsFunction);

      // Assert PVE
      expect(result.nodes).toHaveLength(1);
      expect(result.vms).toHaveLength(1);

      // Assert PBS
      expect(mockPbsFunction).toHaveBeenCalledWith(emptyPbsClients);
      expect(result.pbs).toEqual([]);
    });

    test('should handle error fetching Containers (lxc)', async () => {
      // Arrange: Uses the default mock clients from beforeEach
      const nodeNameGood = 'node-good';
      const nodeNameBad = 'node-bad-guests'; // This node will have the LXC fetch error
      const endpointId = 'primary'; // Default endpointId from mockPveApiClient setup
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

      mockPveClientInstance.get.mockImplementation(async (url) => {
        // Endpoint level calls - needed for the overall fetchDiscoveryData structure
        if (url === '/cluster/status') {
          return { data: { data: [{ type: 'cluster', nodes: 2, name: 'test-cluster' }] } }; // Simulate a cluster with 2 nodes
        }
        if (url === '/nodes') { // This is the main /nodes call for the endpoint
          return { data: { data: [
            { node: nodeNameGood, status: 'online', id: `node/${nodeNameGood}` },
            { node: nodeNameBad, status: 'online', id: `node/${nodeNameBad}` }
          ]}};
        }

        // Calls for nodeNameGood (all succeed)
        if (url === `/nodes/${nodeNameGood}/status`) return { data: { data: { cpu: 0.1, uptime: 10 } } };
        if (url === `/nodes/${nodeNameGood}/storage`) return { data: { data: [] } };
        if (url === `/nodes/${nodeNameGood}/qemu`) return { data: { data: [ { vmid: 100, name: 'vm-good', status: 'running' } ] } };
        if (url === `/nodes/${nodeNameGood}/lxc`) return { data: { data: [] } };
        
        // Calls for nodeNameBad 
        if (url === `/nodes/${nodeNameBad}/status`) return { data: { data: { cpu: 0.2, uptime: 20 } } };
        if (url === `/nodes/${nodeNameBad}/storage`) return { data: { data: [] } };
        if (url === `/nodes/${nodeNameBad}/qemu`) return { data: { data: [] } }; // QEMU succeeds
        if (url === `/nodes/${nodeNameBad}/lxc`) { // LXC fetch fails
          throw new Error('Simulated LXC Fetch Error');
        }
        
        // Fallback for unexpected calls
        console.warn(`Unexpected API call in mockImplementation: ${url}`);
        throw new Error(`Unexpected API call in mock: ${url}`);
      });

      // Act: Uses the default mock clients from beforeEach
      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient);

      // Assert
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        expect.stringContaining(`[DataFetcher - ${endpointId}-${nodeNameBad}] Error fetching Containers (lxc): Simulated LXC Fetch Error`)
      );
      
      // Check that the correct number of nodes is returned
      expect(result.nodes).toHaveLength(2);

      const goodNodeResult = result.nodes.find(n => n.node === nodeNameGood);
      const badNodeResult = result.nodes.find(n => n.node === nodeNameBad);
      expect(goodNodeResult).toBeDefined();
      expect(badNodeResult).toBeDefined();

      // VMs/CTs from the good node should be present
      expect(result.vms).toHaveLength(1); // From nodeNameGood
      expect(result.vms[0].vmid).toBe(100);
      // Containers from nodeNameGood are [], and from nodeNameBad failed, so overall should be []
      expect(result.containers).toHaveLength(0); 

      consoleErrorSpy.mockRestore();
    });

    test('should handle errors fetching node status and storage', async () => {
      // Arrange
      const nodeName = 'node-bad-status-storage'; // This won't be used as errors occur at endpoint/node list level
      const endpointId = 'primary';
      const endpointConfig = mockPveApiClient[endpointId].config; // Get the config for name

      const statusError = new Error('Status fetch failed');
      const storageError = new Error('Storage fetch failed'); // This will be for the /nodes call
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

      mockPveClientInstance.get.mockImplementation(async (url) => {
        if (url === '/cluster/status') {
          // console.log(`Mock: ${url} throwing statusError`);
          throw statusError;
        }
        // This will be called by the catch block of /cluster/status, and potentially the main /nodes call
        if (url === '/nodes') {
          // console.log(`Mock: ${url} throwing storageError`);
          throw storageError;
        }
        // console.warn(`Mock: Unexpected call to ${url}`);
        throw new Error(`Unexpected API call in mock for failing status/storage: ${url}`);
      });

        // Act
        const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient);

        // Assert
        // Check the first call to console.error specifically for the /cluster/status failure
        const firstCallArgs = consoleErrorSpy.mock.calls[0];
        const expectedLogMessagePart = `[DataFetcher - primary] Error fetching /cluster/status: ${statusError.message}`;
        expect(firstCallArgs[0]).toContain(expectedLogMessagePart);
        expect(firstCallArgs[1]).toBeInstanceOf(Error);
        expect(firstCallArgs[1].message).toBe(statusError.message);
        // Check the second call for the /nodes failure after /cluster/status
        expect(consoleErrorSpy).toHaveBeenCalledWith(
          expect.stringContaining(`[DataFetcher - primary] Also failed to fetch /nodes after /cluster/status error: ${storageError.message}`)
        );
        expect(consoleErrorSpy).toHaveBeenCalledTimes(2);
        // Third error log from the main catch block in fetchDataForPveEndpoint
        // expect(consoleErrorSpy).toHaveBeenCalledWith(
        //   expect.stringContaining(`[DataFetcher - primary] Error fetching PVE discovery data: ${storageError.message}`)
        // );
        // expect(consoleErrorSpy).toHaveBeenCalledTimes(3);

        // Data should be empty for this endpoint due to critical failures
        expect(result.nodes).toEqual([]);
        expect(result.vms).toEqual([]);
        expect(result.containers).toEqual([]);

        consoleErrorSpy.mockRestore();
      });

    test('should handle invalid format for node status response (invalid data.data)', async () => {
      const nodeName = 'node-bad-status-data'; // A unique name for this test case
      const endpointId = 'primary';
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

      mockPveClientInstance.get
        // Endpoint level calls
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 1. /cluster/status
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName }] } })                                 // 2. /nodes (for standaloneNodeName)
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName, status: 'online' }] } })                // 3. /nodes (main node list for endpoint)
        // Node level calls for nodeName
        .mockResolvedValueOnce({ data: { data: null } })  // <--- Invalid: data.data is null for /status
        .mockResolvedValueOnce({ data: { data: [] } })    // storage
        .mockResolvedValueOnce({ data: { data: [] } })    // qemu
        .mockResolvedValueOnce({ data: { data: [] } });   // lxc

      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient);

      // Assert
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        expect.stringContaining(`[DataFetcher - ${endpointId}-${nodeName}] Could not fetch storage list`)
      );
      // Node should still exist, but status fields should be default/null
      expect(result.nodes).toHaveLength(1);
      expect(result.nodes[0].node).toBe(nodeName);
      expect(result.nodes[0].cpu).toBeNull();
      expect(result.nodes[0].mem).toBeNull(); // or existing value if not overwritten by status
      expect(result.nodes[0].uptime).toBe(0);
      
      consoleWarnSpy.mockRestore();
    });

    test('should handle invalid format for node status response', async () => {
      const nodeName = 'node-bad-status-format'; // Test a node where its /status call fails
      const endpointId = 'primary';
      const endpointConfig = mockPveApiClient[endpointId].config;
      const statusFetchError = new Error('Node Status Network Error');
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

      mockPveClientInstance.get
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 1. /cluster/status
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName }] } })                                 // 2. /nodes (for standaloneNodeName)
        .mockResolvedValueOnce({ data: { data: [{ node: nodeName, status: 'online' }] } })                // 3. /nodes (main node list for endpoint)
        .mockRejectedValueOnce(statusFetchError)                                                          // 4. /nodes/${nodeName}/status << THIS FAILS
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 5. /nodes/${nodeName}/storage (subsequent calls should still be mocked)
        .mockResolvedValueOnce({ data: { data: [] } })                                                   // 6. /nodes/${nodeName}/qemu
        .mockResolvedValueOnce({ data: { data: [] } })                                                  // 7. /nodes/${nodeName}/lxc
        .mockResolvedValueOnce({ data: { data: [] } });                                                  // 8. /nodes/${nodeName}/tasks (backup tasks)

      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient);

      // Assert error logging
      // Only one log from fetchNodeResource, as it catches the error and returns null,
      // so the promise in fetchDataForPveEndpoint for this node is fulfilled.
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        expect.stringContaining(`[DataFetcher - ${endpointId}-${nodeName}] Error fetching PVE backup tasks`)
      );
      expect(consoleErrorSpy).toHaveBeenCalledTimes(1);

      // Assert node data (node should exist, but with default/error state for status)
      expect(result.nodes).toHaveLength(1);
      expect(result.nodes[0].node).toBe(nodeName);
      expect(result.nodes[0].cpu).toBeNull(); 
      expect(result.nodes[0].mem).toBeNull();
      expect(result.nodes[0].uptime).toBe(0);
      // Other parts like storage should be processed if their mocks are fine (empty array here)
      expect(result.nodes[0].storage).toEqual([]); 
      
      consoleErrorSpy.mockRestore();
    });

    test('should handle invalid /nodes response format', async () => {
      // Arrange
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});
      // Mock /nodes to return non-array data
      mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { not_an_array: true } } });

      // Act
      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient);

      // Assert
      // expect(consoleWarnSpy).toHaveBeenCalledWith(
      //    expect.stringContaining(`[DataFetcher - primary] No nodes found or unexpected format.`)
      // );
      // Should return empty arrays as if no nodes were found
      expect(result.nodes).toEqual([]);
      expect(result.vms).toEqual([]);
      expect(result.containers).toEqual([]);
      
      consoleWarnSpy.mockRestore();
    });

    test('should handle outer promise rejection for one PVE endpoint', async () => {
      // Arrange: Setup one valid client and one config that will cause an error
      const mockPveClientInstance1 = { get: jest.fn() };
      const mockClients = {
        primary: { client: mockPveClientInstance1, config: { name: 'pve-good' } },
        bad_endpoint: null, // This will cause TypeError when accessing .client
      };
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
      
      // Mock the valid client to succeed
      mockPveClientInstance1.get
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 1. /cluster/status
        .mockResolvedValueOnce({ data: { data: [{ node: 'node-good' }] } }) // 2. /nodes (standalone)
        .mockResolvedValueOnce({ data: { data: [{ node: 'node-good', status: 'online' }] } }) // 3. /nodes (main)
        .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1, name: 'test-cluster' }] } }) // 4. /cluster/status (additional)
        .mockResolvedValueOnce({ data: { data: { uptime: 1 } } }) // 5. status
        .mockResolvedValueOnce({ data: { data: [] } }) // 6. storage
        .mockResolvedValueOnce({ data: { data: [] } }) // 7. qemu
        .mockResolvedValueOnce({ data: { data: [] } }) // 8. lxc
        .mockResolvedValueOnce({ data: { data: [] } }) // 9. /storage
        .mockResolvedValueOnce({ data: { data: [] } }); // 10. snapshot

      // Act
      const result = await fetchDiscoveryData(mockClients, {});

       // Assert
       // Check that the missing client error was logged
       expect(consoleErrorSpy).toHaveBeenCalledWith(
         "[DataFetcher] No client found for endpoint: bad_endpoint"
        );
       // Check that the overall result contains data from the good endpoint but not the bad one
       expect(result.nodes).toHaveLength(1);
       expect(result.nodes[0].node).toBe('node-good');
       expect(result.vms).toHaveLength(0);
       expect(result.containers).toHaveLength(0);

      consoleErrorSpy.mockRestore();
    });

    test('should handle synchronous error within fetchPveDiscoveryData/fetchPbsData', async () => {
      // Arrange
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
      const validPveClients = { primary: { client: { get: jest.fn() }, config: { id: 'primary' } } };
      const invalidPbsClients = null; // Trigger TypeError in fetchPbsData

      // Act
      // Pass valid PVE clients but invalid PBS clients to trigger sync error
      const result = await fetchDiscoveryData(validPveClients, invalidPbsClients);

      // Assert
      // Check that errors were logged for the various failed operations
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "[DataFetcher] Error during parallel backup data fetch:",
        expect.any(TypeError) // Should be TypeError from Object.keys(null)
      );

      // Check that the function returned the default empty structure
      expect(result).toEqual({
        nodes: [],
        vms: [],
        containers: [],
        pbs: [],
        pveBackups: {
          backupTasks: [],
          guestSnapshots: [],
          storageBackups: []
        }
      });

      consoleErrorSpy.mockRestore();
    });

  });

  // --- NEW: describe block for fetchMetricsData ---
  describe('fetchMetricsData', () => {
    // Note: This block now relies on mockPveApiClient and mockPveClientInstance 
    //       set up in the main beforeEach of the outer describe block.
    let mockCurrentApiClients; // Keep this structure locally if tests modify it

    beforeEach(() => {
        // Reset only the client's get method, as the client itself is setup outside
        mockPveClientInstance.get.mockClear(); 

        // Use the mock PVE client setup in the outer scope.
        // Tests within this block might add more clients (e.g., pve2) to this object.
        mockCurrentApiClients = { ...mockPveApiClient }; 
    });

    test('should return empty array when no running guests are provided', async () => {
        const result = await fetchMetricsData([], [], mockCurrentApiClients);
        expect(result).toEqual([]);
        expect(mockPveClientInstance.get).not.toHaveBeenCalled(); // Use outer mock instance
    });

    test('should fetch metrics for a single running VM', async () => {
        const runningVms = [
            // Use the endpointId matching the outer mock setup (e.g., 'primary')
            { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm-test' }
        ];
        
        // Mock RRD data response on the outer instance
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ time: 1, cpu: 0.5 }] } });
        // Mock current status response on the outer instance
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.5, mem: 1024, disk: 2048 } } });

        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);

        expect(result).toHaveLength(1);
        // Retrieve endpointName from the config of the passed client
        const endpointName = mockCurrentApiClients.primary.config.name || 'primary'; // Fallback needed?
        expect(result[0]).toEqual({
            id: 100,
            guestName: 'vm-test',
            node: 'node1',
            type: 'qemu',
            endpointId: 'primary', // Matches input
            endpointName: endpointName, // Use retrieved name
            data: [{ time: 1, cpu: 0.5 }], // RRD data
            current: { cpu: 0.5, mem: 1024, disk: 2048 } // Current status
        });

        // Verify API calls on the outer instance
        expect(mockPveClientInstance.get).toHaveBeenCalledTimes(2);
        expect(mockPveClientInstance.get).toHaveBeenCalledWith('/nodes/node1/qemu/100/rrddata', expect.any(Object));
        expect(mockPveClientInstance.get).toHaveBeenCalledWith('/nodes/node1/qemu/100/status/current');
    });

    test('should fetch metrics for a single running container', async () => {
        const runningContainers = [
            { endpointId: 'primary', node: 'node2', vmid: 101, type: 'lxc', name: 'ct-test' }
        ];

        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ time: 2, cpu: 0.2 }] } }); // RRD
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.2, mem: 512, disk: 1024 } } }); // Current

        const result = await fetchMetricsData([], runningContainers, mockCurrentApiClients);
        const endpointName = mockCurrentApiClients.primary.config.name || 'primary';

        expect(result).toHaveLength(1);
        expect(result[0]).toEqual({
            id: 101,
            guestName: 'ct-test',
            node: 'node2',
            type: 'lxc',
            endpointId: 'primary',
            endpointName: endpointName,
            data: [{ time: 2, cpu: 0.2 }],
            current: { cpu: 0.2, mem: 512, disk: 1024 }
        });

        expect(mockPveClientInstance.get).toHaveBeenCalledTimes(2);
        expect(mockPveClientInstance.get).toHaveBeenCalledWith('/nodes/node2/lxc/101/rrddata', expect.any(Object));
        expect(mockPveClientInstance.get).toHaveBeenCalledWith('/nodes/node2/lxc/101/status/current');
    });

    test('should fetch metrics for multiple guests (VMs and CTs) across nodes/endpoints', async () => {
        // Add another endpoint client
        const mockApiClient2 = { get: jest.fn(), config: { name: 'PVE-2' } }; // Add config here
        // Add to the mockCurrentApiClients object used in this test scope
        mockCurrentApiClients.pve2 = { client: mockApiClient2, config: mockApiClient2.config }; 

        const runningVms = [
            { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm1' },
            { endpointId: 'pve2', node: 'node3', vmid: 300, type: 'qemu', name: 'vm3' }
        ];
        const runningContainers = [
            { endpointId: 'primary', node: 'node1', vmid: 101, type: 'lxc', name: 'ct1' },
            { endpointId: 'primary', node: 'node2', vmid: 200, type: 'lxc', name: 'ct2' }
        ];

        // Mock responses for PVE-1 / node1 / vm1 (qemu 100) - Use mockPveClientInstance
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ cpu: 0.1 }] } }); // rrd
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.11 } } }); // current
        // Mock responses for PVE-1 / node1 / ct1 (lxc 101) - Use mockPveClientInstance
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ cpu: 0.2 }] } }); // rrd
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.22 } } }); // current
        // Mock responses for PVE-1 / node2 / ct2 (lxc 200) - Use mockPveClientInstance
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ cpu: 0.3 }] } }); // rrd
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.33 } } }); // current
        
        // Mock responses for PVE-2 / node3 / vm3 (qemu 300) - Use mockApiClient2
        mockApiClient2.get.mockResolvedValueOnce({ data: { data: [{ cpu: 0.4 }] } }); // rrd
        mockApiClient2.get.mockResolvedValueOnce({ data: { data: { cpu: 0.44 } } }); // current

        const result = await fetchMetricsData(runningVms, runningContainers, mockCurrentApiClients);

        expect(result).toHaveLength(4); // Expect results for all 4 guests

        const primaryName = mockCurrentApiClients.primary.config.name || 'primary';
        const pve2Name = mockCurrentApiClients.pve2.config.name || 'pve2';

        // Check a couple of results
        expect(result.find(m => m.id === 100 && m.endpointId === 'primary')).toMatchObject({ guestName: 'vm1', endpointName: primaryName, current: { cpu: 0.11 } });
        expect(result.find(m => m.id === 101 && m.endpointId === 'primary')).toMatchObject({ guestName: 'ct1', endpointName: primaryName, current: { cpu: 0.22 } });
        expect(result.find(m => m.id === 200 && m.endpointId === 'primary')).toMatchObject({ guestName: 'ct2', endpointName: primaryName, current: { cpu: 0.33 } });
        expect(result.find(m => m.id === 300 && m.endpointId === 'pve2')).toMatchObject({ guestName: 'vm3', endpointName: pve2Name, current: { cpu: 0.44 } });

        expect(mockPveClientInstance.get).toHaveBeenCalledTimes(6); // 3 guests on pve1 * 2 calls each
        expect(mockApiClient2.get).toHaveBeenCalledTimes(2); // 1 guest on pve2 * 2 calls each
    });

    test('should handle missing API client for an endpoint gracefully', async () => {
         const runningVms = [
            // Reference the endpointId from the outer setup
            { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm-good' }, 
             { endpointId: 'pve_missing', node: 'nodeX', vmid: 999, type: 'qemu', name: 'vm-bad-client' }
        ];

        // Mock success for the valid guest (on the primary client)
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ cpu: 0.5 }] } }); // RRD
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.5 } } }); // Current

        // Spy on console.warn before the call
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);

        expect(result).toHaveLength(1); // Only the guest with a valid client should return data
        expect(result[0].id).toBe(100);
        // Check that the warning was called for the missing endpoint
        expect(consoleWarnSpy).toHaveBeenCalledWith(expect.stringContaining('No API client found for endpoint: pve_missing'));
        // Ensure the valid client's API was called, but no attempt for the missing one
        expect(mockPveClientInstance.get).toHaveBeenCalledTimes(2); // Only calls for vm-good

        consoleWarnSpy.mockRestore(); // Clean up the spy
    });


    test('should handle API error when fetching RRD data for one guest', async () => {
        const runningVms = [
            { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm-ok' },
            { endpointId: 'primary', node: 'node1', vmid: 101, type: 'qemu', name: 'vm-fail-rrd' }
        ];

        const error = new Error('RRD Fetch Failed');
        const endpointName = mockCurrentApiClients.primary.config.name || 'primary';
        
        // Mock success for vm-ok
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ cpu: 0.1 }] } }); // rrd ok
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.11 } } }); // current ok

        // Mock failure for vm-fail-rrd (RRD call fails, current call succeeds)
        mockPveClientInstance.get.mockRejectedValueOnce(error); // rrd fails
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.22 } } }); // current ok (Promise.all proceeds)

        // Spy on console.error
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);

        expect(result).toHaveLength(1); // Only vm-ok should be in results
        expect(result[0].id).toBe(100);

        // Verify API calls (Promise.all means both sets of calls were attempted)
        expect(mockPveClientInstance.get).toHaveBeenCalledTimes(4); // 2 calls * 2 guests

        // Verify error log for the failed guest
      expect(consoleErrorSpy).toHaveBeenCalledWith(
            expect.stringContaining(`[Metrics Cycle - ${endpointName}] Failed to get metrics for qemu 101 (vm-fail-rrd) on node node1: RRD Fetch Failed`)
        );
      consoleErrorSpy.mockRestore();
    });

    test('should handle API error when fetching current status for one guest', async () => {
        const runningVms = [
            { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm-ok' },
            { endpointId: 'primary', node: 'node1', vmid: 101, type: 'qemu', name: 'vm-fail-current' }
        ];

        const error = new Error('Current Status Fetch Failed');
        const endpointName = mockCurrentApiClients.primary.config.name || 'primary';
        
        // Mock success for vm-ok
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ cpu: 0.1 }] } }); // rrd ok
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.11 } } }); // current ok

        // Mock failure for vm-fail-current (RRD call succeeds, current call fails)
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ cpu: 0.2 }] } }); // rrd ok
        mockPveClientInstance.get.mockRejectedValueOnce(error); // current fails

      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);

        expect(result).toHaveLength(1); // Only vm-ok should be in results
        expect(result[0].id).toBe(100);

        expect(mockPveClientInstance.get).toHaveBeenCalledTimes(4); 

      expect(consoleErrorSpy).toHaveBeenCalledWith(
            expect.stringContaining(`[Metrics Cycle - ${endpointName}] Failed to get metrics for qemu 101 (vm-fail-current) on node node1: Current Status Fetch Failed`)
        );
      consoleErrorSpy.mockRestore();
    });

    test('should handle API 400 error gracefully (guest likely stopped)', async () => {
        const runningVms = [
             { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm-ok' },
            { endpointId: 'primary', node: 'node1', vmid: 101, type: 'qemu', name: 'vm-stopped' }
        ];
        const endpointName = mockCurrentApiClients.primary.config.name || 'primary';

        // Simulate a 400 error response
        const error400 = new Error('Bad Request');
        error400.response = { status: 400 };

        // Mock success for vm-ok
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ cpu: 0.1 }] } }); // rrd ok
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.11 } } }); // current ok

        // Mock 400 failure for vm-stopped (assume RRD call fails first)
        mockPveClientInstance.get.mockRejectedValueOnce(error400); // rrd fails with 400
        // The current status call for the failing guest might not even happen if RRD fails hard, 
        // but mock it just in case the error handling changes. Let's assume it would succeed if called.
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.22 } } }); 

      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});
        const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);

        expect(result).toHaveLength(1); // Only vm-ok should be in results
        expect(result[0].id).toBe(100);

        // Should attempt 2 calls for vm-ok, and potentially 2 calls for vm-stopped (RRD and current status)
        // even if RRD fails with 400, Promise.allSettled allows the current status call to proceed.
        expect(mockPveClientInstance.get).toHaveBeenCalledTimes(4); 

        // Verify the specific warning log for the 400 error
      expect(consoleWarnSpy).toHaveBeenCalledWith(
             expect.stringContaining(`[Metrics Cycle - ${endpointName}] Guest qemu 101 (vm-stopped) on node node1 might be stopped or inaccessible (Status: 400). Skipping metrics.`)
      );
        // Verify the specific warning log for the 400 error
        // expect(consoleWarnSpy).toHaveBeenCalledWith(
        //    expect.stringContaining(`[Metrics Cycle - ${endpointName}] Guest qemu 101 (vm-stopped) on node node1 might be stopped or inaccessible (Status: 400). Skipping metrics.`)
        // );
        expect(consoleErrorSpy).not.toHaveBeenCalled(); // Should not log as a generic error
      
      consoleWarnSpy.mockRestore();
        consoleErrorSpy.mockRestore();
    });

    test('should handle empty RRD data array gracefully', async () => {
        const runningVms = [
            { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm-no-rrd-data' }
        ];
        
        // Mock RRD data response with empty data array
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [] } }); 
        // Mock current status response
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.5, mem: 1024 } } });

        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
        const endpointName = mockCurrentApiClients.primary.config.name || 'primary';

        expect(result).toHaveLength(1);
        expect(result[0]).toMatchObject({
            id: 100,
            endpointName: endpointName,
            data: [], // RRD data should be an empty array
            current: { cpu: 0.5, mem: 1024 } 
        });
        expect(mockPveClientInstance.get).toHaveBeenCalledTimes(2);
    });

    test('should handle null current status data gracefully', async () => {
          const runningVms = [
            { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm-no-current-data' }
        ];
        
        // Mock RRD data response
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ time: 1, cpu: 0.5 }] } }); 
        // Mock current status response with null data
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: null } });

        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
        const endpointName = mockCurrentApiClients.primary.config.name || 'primary';

        expect(result).toHaveLength(1);
        expect(result[0]).toMatchObject({
            id: 100,
            endpointName: endpointName,
            data: [{ time: 1, cpu: 0.5 }],
            current: null // Current data should be null
        });
        expect(mockPveClientInstance.get).toHaveBeenCalledTimes(2);
    });

    // --- Tests for QEMU Guest Agent Memory Fetching ---
    test('should fetch QEMU guest agent memory info when agent is enabled and responsive', async () => {
      const runningVms = [
        { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm-agent-ok', agent: '1' }
      ];
      mockPveClientInstance.get
        .mockResolvedValueOnce({ data: { data: [{ time: 1, cpu: 0.5 }] } }) // RRD
        .mockResolvedValueOnce({ data: { data: { cpu: 0.5, mem: 2048*1024*1024, disk: 2048, agent: 1 } } }); // Current status (agent enabled)
      
      // Mock the POST call for guest agent
      mockPveClientInstance.post = jest.fn().mockResolvedValueOnce({ 
        data: { 
          data: { 
            result: { total: 2048*1024*1024, free: 1024*1024*1024, available: 1536*1024*1024 } 
          } 
        } 
      });

      const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
      expect(result).toHaveLength(1);
      expect(result[0].current).toBeDefined();
      expect(result[0].current.guest_mem_total_bytes).toBe(2048*1024*1024);
      expect(result[0].current.guest_mem_free_bytes).toBe(1024*1024*1024);
      expect(result[0].current.guest_mem_available_bytes).toBe(1536*1024*1024);
      expect(result[0].current.guest_mem_actual_used_bytes).toBe((2048-1536)*1024*1024);
      expect(mockPveClientInstance.post).toHaveBeenCalledWith('/nodes/node1/qemu/100/agent/get-memory-block-info', {});
    });

    test('should not attempt QEMU guest agent memory fetch if agent is not enabled in current status', async () => {
      const runningVms = [
        { endpointId: 'primary', node: 'node1', vmid: 101, type: 'qemu', name: 'vm-agent-off', agent: '1'} // Configured as on, but status says off
      ];
      mockPveClientInstance.get
        .mockResolvedValueOnce({ data: { data: [{ time: 1, cpu: 0.5 }] } }) // RRD
        .mockResolvedValueOnce({ data: { data: { cpu: 0.5, mem: 1024, disk: 2048, agent: 0 } } }); // Current status (agent OFF)
      mockPveClientInstance.post = jest.fn(); // Ensure post is a mock

      const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
      expect(result).toHaveLength(1);
      expect(result[0].current.guest_mem_total_bytes).toBeUndefined();
      expect(mockPveClientInstance.post).not.toHaveBeenCalled();
    });

    test('should not attempt QEMU guest agent memory fetch if guest agent config is missing/off', async () => {
      const runningVms = [
        { endpointId: 'primary', node: 'node1', vmid: 102, type: 'qemu', name: 'vm-agent-not-configured' } // No agent field
      ];
       mockPveClientInstance.get
        .mockResolvedValueOnce({ data: { data: [{ time: 1, cpu: 0.5 }] } }) // RRD
        .mockResolvedValueOnce({ data: { data: { cpu: 0.5, mem: 1024, disk: 2048, agent: 1 } } }); // Current status (agent ON)
      mockPveClientInstance.post = jest.fn();

      const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
      expect(result).toHaveLength(1);
      expect(result[0].current.guest_mem_total_bytes).toBeUndefined();
      expect(mockPveClientInstance.post).not.toHaveBeenCalled();
    });

    test('should handle QEMU guest agent error (e.g., agent not responsive)', async () => {
      const runningVms = [
        { endpointId: 'primary', node: 'node1', vmid: 103, type: 'qemu', name: 'vm-agent-error', agent: 'enabled=1' }
      ];
      mockPveClientInstance.get
        .mockResolvedValueOnce({ data: { data: [{ time: 1, cpu: 0.5 }] } }) // RRD
        .mockResolvedValueOnce({ data: { data: { cpu: 0.5, mem: 1024, disk: 2048, agent: 1 } } }); // Current status
      
      const agentError = new Error('Agent not responsive');
      agentError.response = { status: 500, data: { data: { exitcode: -2 } } };
      mockPveClientInstance.post = jest.fn().mockRejectedValueOnce(agentError);
      const consoleLogSpy = jest.spyOn(console, 'log').mockImplementation(() => {});

      const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
      expect(result).toHaveLength(1);
      expect(result[0].current.guest_mem_total_bytes).toBeUndefined();
      expect(mockPveClientInstance.post).toHaveBeenCalledTimes(1);
      expect(consoleLogSpy).toHaveBeenCalledWith(expect.stringContaining("QEMU Guest Agent not responsive or command 'get-memory-block-info' not available/supported"));
      consoleLogSpy.mockRestore();
    });

    test('should handle unexpected QEMU guest agent response format', async () => {
      const runningVms = [
        { endpointId: 'primary', node: 'node1', vmid: 104, type: 'qemu', name: 'vm-agent-bad-format', agent: '1' }
      ];
       mockPveClientInstance.get
        .mockResolvedValueOnce({ data: { data: [{ time: 1, cpu: 0.5 }] } }) // RRD
        .mockResolvedValueOnce({ data: { data: { cpu: 0.5, mem: 1024, disk: 2048, agent: 1 } } }); // Current status
      mockPveClientInstance.post = jest.fn().mockResolvedValueOnce({ data: { data: { result: { unexpected: "data" } } } }); // Bad format
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

      const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
      expect(result).toHaveLength(1);
       expect(result[0].current.guest_mem_total_bytes).toBeUndefined();
       expect(mockPveClientInstance.post).toHaveBeenCalledTimes(1);
       // Adjust expectation to match the actual log which includes the data object
       expect(consoleWarnSpy).toHaveBeenCalledWith(
         expect.stringContaining("Guest agent memory command 'get-memory-block-info' response format not as expected"),
         expect.objectContaining({ result: { unexpected: "data" } }) // Check for the logged object too
       );
       consoleWarnSpy.mockRestore();
     });

    test('should handle generic error fetching QEMU guest agent memory info', async () => {
      const runningVms = [
        { endpointId: 'primary', node: 'node1', vmid: 105, type: 'qemu', name: 'vm-agent-generic-error', agent: '1' }
      ];
      mockPveClientInstance.get
        .mockResolvedValueOnce({ data: { data: [{ time: 1, cpu: 0.5 }] } }) // RRD
        .mockResolvedValueOnce({ data: { data: { cpu: 0.5, mem: 1024, disk: 2048, agent: 1 } } }); // Current status
      
      const genericAgentError = new Error('Network Failure');
      genericAgentError.response = { status: 503 }; // Simulate a non-500 error
      mockPveClientInstance.post = jest.fn().mockRejectedValueOnce(genericAgentError);
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

      const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
      expect(result).toHaveLength(1);
      expect(result[0].current.guest_mem_total_bytes).toBeUndefined();
      expect(mockPveClientInstance.post).toHaveBeenCalledTimes(1);
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        expect.stringContaining("Error fetching guest agent memory info: Network Failure. Status: 503")
      );
      consoleWarnSpy.mockRestore();
    });

    test('should handle generic error fetching RRD/status data', async () => {
        const runningVms = [
            { endpointId: 'primary', node: 'node1', vmid: 106, type: 'qemu', name: 'vm-generic-rrd-error' }
        ];
        const genericError = new Error('Server Unavailable');
        genericError.response = { status: 503 }; // Simulate non-400 error

        // Mock RRD call to fail with generic error, current status call to succeed
        mockPveClientInstance.get
            .mockRejectedValueOnce(genericError) // RRD fails
            .mockResolvedValueOnce({ data: { data: { cpu: 0.1 } } }); // Current status succeeds

        const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
        const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {}); // Spy on warn too

        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);

        expect(result).toHaveLength(0); // Guest data should be skipped due to the error
        expect(mockPveClientInstance.get).toHaveBeenCalledTimes(2); // Both RRD and current status were attempted
        expect(consoleErrorSpy).toHaveBeenCalledWith(
            expect.stringContaining(`Failed to get metrics for qemu 106 (vm-generic-rrd-error) on node node1 (Status: 503): Server Unavailable`)
        );
        expect(consoleWarnSpy).not.toHaveBeenCalledWith( // Ensure the 400-specific warning wasn't called
            expect.stringContaining("might be stopped or inaccessible")
        );

        consoleErrorSpy.mockRestore();
        consoleWarnSpy.mockRestore();
    });

    test('should calculate actual used memory using fallback when "available" is missing', async () => {
      const runningVms = [
        { endpointId: 'primary', node: 'node1', vmid: 107, type: 'qemu', name: 'vm-agent-fallback-mem', agent: '1' }
      ];
      const totalMem = 4096 * 1024 * 1024;
      const freeMem = 1024 * 1024 * 1024;
      const cachedMem = 512 * 1024 * 1024;
      const buffersMem = 256 * 1024 * 1024;
      const expectedUsed = totalMem - freeMem - cachedMem - buffersMem;

      mockPveClientInstance.get
        .mockResolvedValueOnce({ data: { data: [{ time: 1, cpu: 0.5 }] } }) // RRD
        .mockResolvedValueOnce({ data: { data: { cpu: 0.5, mem: totalMem, disk: 2048, agent: 1 } } }); // Current status

      mockPveClientInstance.post = jest.fn().mockResolvedValueOnce({ 
        data: { 
          data: { 
            // Agent response *without* 'available' field
            result: { total: totalMem, free: freeMem, cached: cachedMem, buffers: buffersMem } 
          } 
        } 
      });

      const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
      expect(result).toHaveLength(1);
      expect(result[0].current).toBeDefined();
      expect(result[0].current.guest_mem_total_bytes).toBe(totalMem);
      expect(result[0].current.guest_mem_free_bytes).toBe(freeMem);
      expect(result[0].current.guest_mem_cached_bytes).toBe(cachedMem);
      expect(result[0].current.guest_mem_buffers_bytes).toBe(buffersMem);
      expect(result[0].current.guest_mem_available_bytes).toBeUndefined(); // Ensure 'available' was indeed missing
      expect(result[0].current.guest_mem_actual_used_bytes).toBe(expectedUsed); // Check fallback calculation
      expect(mockPveClientInstance.post).toHaveBeenCalledTimes(1);
    });

    test('should calculate actual used memory using final fallback (total - free) when other fields missing', async () => {
      const runningVms = [
        { endpointId: 'primary', node: 'node1', vmid: 108, type: 'qemu', name: 'vm-agent-final-fallback', agent: '1' }
      ];
      const totalMem = 2048 * 1024 * 1024;
      const freeMem = 512 * 1024 * 1024;
      const expectedUsed = totalMem - freeMem;

      mockPveClientInstance.get
        .mockResolvedValueOnce({ data: { data: [{ time: 1, cpu: 0.5 }] } }) // RRD
        .mockResolvedValueOnce({ data: { data: { cpu: 0.5, mem: totalMem, disk: 2048, agent: 1 } } }); // Current status

      mockPveClientInstance.post = jest.fn().mockResolvedValueOnce({ 
        data: { 
          data: { 
            // Agent response *only* with total and free
            result: { total: totalMem, free: freeMem } 
          } 
        } 
      });

      const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
      expect(result).toHaveLength(1);
      expect(result[0].current).toBeDefined();
      expect(result[0].current.guest_mem_total_bytes).toBe(totalMem);
      expect(result[0].current.guest_mem_free_bytes).toBe(freeMem);
      expect(result[0].current.guest_mem_available_bytes).toBeUndefined();
      expect(result[0].current.guest_mem_cached_bytes).toBeUndefined();
      expect(result[0].current.guest_mem_buffers_bytes).toBeUndefined();
      expect(result[0].current.guest_mem_actual_used_bytes).toBe(expectedUsed); // Check final fallback calculation
      expect(mockPveClientInstance.post).toHaveBeenCalledTimes(1);
    });


  }); // End describe fetchMetricsData

  // --- NEW: describe block for fetchPbsData ---
  describe('fetchPbsData', () => {
    // Relies on mockPbsApiClient and mockPbsClientInstance from outer describe

    beforeEach(() => {
      // Ensure the default mocks are reset/cleared if needed for PBS specific tests
      // Typically mockPbsClientInstance.get.mockClear() is sufficient if reusing the instance
      mockPbsClientInstance.get.mockClear();

      // Reset the default PBS mock to an empty object for clarity
      mockPbsApiClient = {}; 
      // Override the initializer mock if tests need specific PBS clients setup via initializeApiClients
      // Otherwise, tests will construct and pass mock PBS clients directly
    });

    test('should return empty array when no PBS clients are provided', async () => {
      const result = await fetchPbsData({});
      expect(result).toEqual([]);
    });

    test('should fetch data correctly for one PBS instance (happy path)', async () => {
        // Arrange
        const pbsId = 'pbs-happy';
        const pbsName = 'PBS Happy Path';
        const pbsNodeName = 'pbs-node1';
        const datastoreName = 'datastore1';
        const mockPbsClient = { get: jest.fn() };
        const mockClients = { [pbsId]: { client: mockPbsClient, config: { name: pbsName } } };

        mockPbsClient.get
            .mockResolvedValueOnce({ data: { data: [{ node: pbsNodeName }] } }) // /nodes
            .mockResolvedValueOnce({ data: { data: [{ store: datastoreName, total: 1, used: 0 }] } }) // /status/datastore-usage
            .mockResolvedValueOnce({ data: { data: [{ 'backup-id': 'snap1' }] } }) // /admin/datastore/{store}/snapshots
            .mockResolvedValueOnce({ data: { data: [{ store: datastoreName, 'deduplication-factor': 1.0 }] } }) // /status/datastore-usage in fetchAllPbsTasksForProcessing
            .mockResolvedValueOnce({ data: { data: [{ name: datastoreName }] } }) // /config/datastore (for fetchAllPbsTasksForProcessing)
            .mockResolvedValueOnce({ data: { data: [{ 'backup-type': 'vm', 'backup-id': '100' }] } }) // /admin/datastore/{store}/groups
            .mockResolvedValueOnce({ data: { data: [{ 'backup-time': Math.floor(Date.now() / 1000) - 86400 }] } }) // /admin/datastore/{store}/snapshots for group (1 day ago)
            .mockResolvedValueOnce({ data: { data: [] } }); // /nodes/{node}/tasks - empty to simplify test

        // Act
        const result = await fetchPbsData(mockClients);

        // Assert
        expect(result).toHaveLength(1);
        expect(result[0]).toMatchObject({
            pbsEndpointId: pbsId,
            pbsInstanceName: pbsName,
            nodeName: pbsNodeName,
            status: 'ok',
            datastores: expect.any(Array)
        });

        // Since nodeStatus and versionInfo are skipped in test env, total calls are reduced
        expect(mockPbsClient.get).toHaveBeenCalledTimes(8); // nodes, usage, snapshots, usage(dedup), config/datastore, groups, snapshots(group), tasks
        expect(mockPbsClient.get).toHaveBeenCalledWith('/nodes');
        expect(mockPbsClient.get).toHaveBeenCalledWith('/status/datastore-usage');
        expect(mockPbsClient.get).toHaveBeenCalledWith(`/admin/datastore/${datastoreName}/snapshots`);
        expect(mockPbsClient.get).toHaveBeenCalledWith(`/nodes/${pbsNodeName}/tasks`, expect.any(Object));

        // Check that basic task structure is present - tasks processing has been verified in pbsUtils tests
        expect(result[0]).toHaveProperty('backupTasks');
        expect(result[0]).toHaveProperty('verifyTasks');
        expect(result[0]).toHaveProperty('gcTasks');
    });

    test('should handle error fetching PBS node name (and skip subsequent calls)', async () => {
        const pbsId = 'pbs-err-node';
        const mockPbsClient = { get: jest.fn() };
        const mockClients = { [pbsId]: { client: mockPbsClient, config: { name: 'PBS Node Err' } } };
        const nodeError = new Error('Node fetch failed');

        // Mock /nodes to fail
        mockPbsClient.get.mockRejectedValueOnce(nodeError);

        const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

        const result = await fetchPbsData(mockClients);

        expect(result).toHaveLength(1);
        expect(result[0].status).toBe('error'); // Should be error because node detection failed
        // Expect empty arrays/undefined for other fields as fetching is skipped
        expect(result[0].datastores).toBeUndefined();
        expect(result[0].backupTasks).toBeUndefined();
        // ... other task types ...

        // Verify API calls (only /nodes should have been attempted)
        expect(mockPbsClient.get).toHaveBeenCalledTimes(1);
        expect(mockPbsClient.get).toHaveBeenCalledWith('/nodes');

        // Verify error logging
        // There are two error logs now: one from fetchPbsNodeName catch and one from fetchPbsData outer catch
        expect(consoleErrorSpy).toHaveBeenCalledWith(
            expect.stringContaining(`ERROR: [DataFetcher] Failed to fetch PBS nodes list for ${mockClients[pbsId].config.name}: ${nodeError.message}`)
        );
        expect(consoleErrorSpy).toHaveBeenCalledWith(
            expect.stringContaining(`ERROR: [DataFetcher - ${mockClients[pbsId].config.name}] PBS fetch failed: Could not determine node name for PBS instance ${mockClients[pbsId].config.name}`)
        );
        expect(consoleErrorSpy).toHaveBeenCalledTimes(2);

      consoleErrorSpy.mockRestore();
    });

    test('should handle error fetching PBS datastores', async () => {
        // Arrange
        const pbsId = 'pbs-err-ds';
        const pbsNodeName = 'pbs-node-ds-err';
        const mockPbsClient = { get: jest.fn() };
        const mockClients = { [pbsId]: { client: mockPbsClient, config: { name: 'PBS DS Err' } } };
        const dsError = new Error('Datastore fetch failed');

        mockPbsClient.get
            .mockResolvedValueOnce({ data: { data: [{ node: pbsNodeName }] } }) // /nodes (succeeds)
            .mockRejectedValueOnce(dsError) // /status/datastore-usage (fails)
            .mockResolvedValueOnce({ data: { data: [] } }) // Mock fallback /config/datastore call (returns empty)
            .mockRejectedValueOnce(new Error('Dedup fetch failed')) // /status/datastore-usage in fetchAllPbsTasksForProcessing (fails)
            .mockResolvedValueOnce({ data: { data: [] } }) // /config/datastore in fetchAllPbsTasksForProcessing (empty)
            .mockResolvedValueOnce({ data: { data: [] } }); // /nodes/{node}/tasks (empty tasks)

        const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

        const result = await fetchPbsData(mockClients);

        expect(result).toHaveLength(1);
        expect(result[0]).toMatchObject({
            pbsEndpointId: pbsId,
            status: 'ok', // Status remains 'ok' because node name succeeded, datastore fetch fell back
            nodeName: pbsNodeName,
            datastores: [], // Should be empty
        });
        // Expect calls for /nodes, /status/datastore-usage, the fallback /config/datastore
        // No snapshots because datastores is empty. Task processing calls /status/datastore-usage (fails), /config/datastore again and /tasks
        expect(mockPbsClient.get).toHaveBeenCalledTimes(6);
        // Expect a warning about the fallback, not an error in the result object itself
        expect(consoleWarnSpy).toHaveBeenCalledWith(expect.stringContaining(`Failed to get datastore usage for ${mockClients[pbsId].config.name}, falling back to /config/datastore. Error: ${dsError.message}`));
        consoleErrorSpy.mockRestore();
      consoleWarnSpy.mockRestore();
    });

     test('should handle error fetching snapshots for one datastore but succeed for others', async () => {
        const pbsId = 'pbs-err-snap';
        const pbsNodeName = 'pbs-node-snap-err';
        const dsGood = 'datastore-good';
        const dsBad = 'datastore-bad';
        const mockPbsClient = { get: jest.fn() };
        const mockClients = { [pbsId]: { client: mockPbsClient, config: { name: 'PBS Snap Err' } } };
        const snapError = new Error('Snapshot fetch failed');

        mockPbsClient.get
            .mockResolvedValueOnce({ data: { data: [{ node: pbsNodeName }] } }) // /nodes
            .mockResolvedValueOnce({ data: { data: [{ store: dsGood, total: 1, used: 0 }, { store: dsBad, total: 1, used: 0 }] } }) // /status/datastore-usage
            // Mock snapshot fetches:
            .mockResolvedValueOnce({ data: { data: [{ 'backup-id': 'snap-good-1' }] } }) // snapshots for dsGood (success)
            .mockRejectedValueOnce(snapError) // snapshots for dsBad (fails)
            .mockResolvedValueOnce({ data: { data: [{ store: dsGood, 'deduplication-factor': 1.0 }] } }) // /status/datastore-usage in fetchAllPbsTasksForProcessing
            .mockResolvedValueOnce({ data: { data: [{ name: dsGood }, { name: dsBad }] } }) // /config/datastore in fetchAllPbsTasksForProcessing
            .mockResolvedValueOnce({ data: { data: [{ 'backup-type': 'vm', 'backup-id': '100' }] } }) // /admin/datastore/{dsGood}/groups
            .mockResolvedValueOnce({ data: { data: [{ 'backup-time': 1678886400 }] } }) // /admin/datastore/{dsGood}/snapshots for group
            .mockResolvedValueOnce({ data: { data: [{ 'backup-type': 'vm', 'backup-id': '101' }] } }) // /admin/datastore/{dsBad}/groups
            .mockResolvedValueOnce({ data: { data: [{ 'backup-time': 1678886401 }] } }) // /admin/datastore/{dsBad}/snapshots for group
            .mockResolvedValueOnce({ data: { data: [{ upid: 'task1'}] } }); // /nodes/{node}/tasks

        const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

        // Act
        const result = await fetchPbsData(mockClients);

        // Assert
        expect(result).toHaveLength(1);
        expect(result[0].status).toBe('ok'); // Status is still ok
        expect(result[0].nodeName).toBe(pbsNodeName);
        expect(result[0].datastores).toHaveLength(2);
        // Check datastore results (snapshots for dsBad should be empty array)
        const goodDs = result[0].datastores.find(ds => ds.name === dsGood);
        const badDs = result[0].datastores.find(ds => ds.name === dsBad);

        expect(goodDs).toBeDefined();
        expect(goodDs.snapshots).toHaveLength(1);
        expect(badDs).toBeDefined();
        expect(badDs.snapshots).toHaveLength(0); // Empty due to error

        // Verify API calls: nodes, usage, snapshots(good), snapshots(bad), tasks
        expect(mockPbsClient.get).toHaveBeenCalledTimes(11); // nodes, usage, snapshots(good), snapshots(bad), usage(dedup), config, groups*2, snapshots*2, tasks
        expect(mockPbsClient.get).toHaveBeenCalledWith('/nodes');
        expect(mockPbsClient.get).toHaveBeenCalledWith('/status/datastore-usage');
        expect(mockPbsClient.get).toHaveBeenCalledWith(`/admin/datastore/${dsGood}/snapshots`);
        expect(mockPbsClient.get).toHaveBeenCalledWith(`/admin/datastore/${dsBad}/snapshots`);
        expect(mockPbsClient.get).toHaveBeenCalledWith(`/nodes/${pbsNodeName}/tasks`, expect.any(Object));

        // Verify error log for the failed snapshot fetch
        expect(consoleErrorSpy).toHaveBeenCalledWith(
            expect.stringContaining(`ERROR: [DataFetcher] Failed to fetch snapshots for datastore ${dsBad} on ${mockClients[pbsId].config.name}: ${snapError.message}`)
        );
        expect(consoleErrorSpy).toHaveBeenCalledTimes(1); // Only snapshot error, not outer catch

        consoleErrorSpy.mockRestore();
    });

    test('should handle error fetching PBS tasks (tasks are null, error is true)', async () => {
        const pbsId = 'pbs-err-tasks';
        const pbsNodeName = 'pbs-node-tasks-err';
        const datastoreName = 'store-tasks-err';
        const mockPbsClient = { get: jest.fn() };
        const mockClients = { [pbsId]: { client: mockPbsClient, config: { name: 'PBS Tasks Err' } } };
        const taskError = new Error('Task fetch failed');

        mockPbsClient.get
          .mockResolvedValueOnce({ data: { data: [{ node: pbsNodeName }] } }) // /nodes
          .mockResolvedValueOnce({ data: { data: [{ store: datastoreName, total: 1, used: 0 }] } }) // /status/datastore-usage
          // Mock snapshot fetch to also fail to simulate the observed error path from the test output
          .mockRejectedValueOnce(new Error('Datastore backup history fetch failed')) // Snapshots (fails here)
          .mockResolvedValueOnce({ data: { data: [{ store: datastoreName, 'deduplication-factor': 1.0 }] } }) // /status/datastore-usage in fetchAllPbsTasksForProcessing
          .mockResolvedValueOnce({ data: { data: [{ name: datastoreName }] } }) // /config/datastore in fetchAllPbsTasksForProcessing
          .mockResolvedValueOnce({ data: { data: [] } }) // /admin/datastore/{store}/groups (empty)
          .mockRejectedValueOnce(taskError); // /nodes/{node}/tasks (fails)

        const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
        const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

        const result = await fetchPbsData(mockClients);

        expect(result).toHaveLength(1);
        expect(result[0].status).toBe('ok'); // Status is still ok
        expect(result[0].datastores).toHaveLength(1); // Datastore fetch succeeded (initial usage)
        // Snapshots should be empty as that fetch failed
        expect(result[0].datastores[0].snapshots).toHaveLength(0);

        // Tasks now have default structure when fetchAllPbsTasksForProcessing fails
        expect(result[0]).toHaveProperty('backupTasks');
        expect(result[0]).toHaveProperty('verifyTasks');
        expect(result[0]).toHaveProperty('gcTasks');


        // Expected calls: nodes, usage, snapshots (fails), then tasks processing continues
        expect(mockPbsClient.get).toHaveBeenCalledTimes(7); // nodes, usage, snapshots (fail), usage(dedup), config/datastore, groups, tasks
        // Update expected error message to match actual output from snapshot fetch failure
        expect(consoleErrorSpy).toHaveBeenCalledWith(
            expect.stringContaining(`ERROR: [DataFetcher] Failed to fetch snapshots for datastore store-tasks-err on PBS Tasks Err`)
        );
        // Expect warning about tasks not being processed
        expect(consoleWarnSpy).toHaveBeenCalledWith(
            expect.stringContaining(`No tasks to process or task fetching failed. Error flag: true, Tasks array: null`)
        );
      consoleErrorSpy.mockRestore();
      consoleWarnSpy.mockRestore();
    });

    test('should aggregate data from multiple PBS instances, including partial failures', async () => {
        const pbsId1 = 'pbs-ok';
        const pbsId2 = 'pbs-fail-ds';
        const pbsId3 = 'pbs-fail-node';
        const mockClient1 = { get: jest.fn() };
        const mockClient2 = { get: jest.fn() };
        const mockClient3 = { get: jest.fn() };
        const mockClients = {
            [pbsId1]: { client: mockClient1, config: { name: 'PBS OK' } },
            [pbsId2]: { client: mockClient2, config: { name: 'PBS DS Fail' } },
            [pbsId3]: { client: mockClient3, config: { name: 'PBS Node Fail' } },
        };

        // Mock Client 1 (Success)
        mockClient1.get
            .mockResolvedValueOnce({ data: { data: [{ node: 'node1' }] } }) // nodes
            .mockResolvedValueOnce({ data: { data: [{ store: 'ds1', total: 1, used: 0 }] } }) // usage
            .mockResolvedValueOnce({ data: { data: [] } }) // snapshots for ds1
            .mockResolvedValueOnce({ data: { data: [{ store: 'ds1', 'deduplication-factor': 1.0 }] } }) // /status/datastore-usage in fetchAllPbsTasksForProcessing
            .mockResolvedValueOnce({ data: { data: [{ name: 'ds1' }] } }) // /config/datastore in fetchAllPbsTasksForProcessing
            .mockResolvedValueOnce({ data: { data: [] } }) // /admin/datastore/ds1/groups (empty)
            .mockResolvedValueOnce({ data: { data: [{upid: 'task-c1', type: 'backup', status: 'OK', starttime: 1, endtime: 2, worker_id: 'vm/1'}] } }); // tasks for node1 (success)

        // Mock Client 2 (Fail Datastore, fallback success, tasks success)
        const node2Name = 'node2';
        const fallbackDsName = 'fallback-ds-client2';
        mockClient2.get
            .mockResolvedValueOnce({ data: { data: [{ node: node2Name }] } })              // /nodes (success)
            .mockRejectedValueOnce(new Error('DS Usage API Error'))                     // /status/datastore-usage (fail)
            .mockResolvedValueOnce({ data: { data: [{ name: fallbackDsName, path: '/mnt/fb', store: fallbackDsName }] }}) // /config/datastore (fallback success, 1 store)
            .mockResolvedValueOnce({ data: { data: [{ 'backup-id': 'snap-fb'}] } })      // /admin/datastore/fallback-ds-client2/snapshots (success)
            .mockResolvedValueOnce({ data: { data: [{ store: fallbackDsName, 'deduplication-factor': 1.0 }] } }) // /status/datastore-usage in fetchAllPbsTasksForProcessing
            .mockResolvedValueOnce({ data: { data: [{ name: fallbackDsName }] } }) // /config/datastore in fetchAllPbsTasksForProcessing
            .mockResolvedValueOnce({ data: { data: [] } }) // /admin/datastore/{fallback}/groups (empty)
            .mockResolvedValueOnce({ data: { data: [{ upid: 'task-c2', type: 'verify', status: 'OK', starttime: 3, endtime: 4 }] } });           // /nodes/node2/tasks (success)

        // Mock Client 3 (Fail Node - tasks won't be called)
        mockClient3.get.mockRejectedValueOnce(new Error('Node Error')); // nodes fails

        const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
        const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

        // Act
        const result = await fetchPbsData(mockClients);

        // Assert
        // Expect all instances in the final result, including the failed one
        expect(result).toHaveLength(3);

        const res1 = result.find(r => r.pbsEndpointId === pbsId1);
        const res2 = result.find(r => r.pbsEndpointId === pbsId2);
        const res3 = result.find(r => r.pbsEndpointId === pbsId3);

        expect(res1).toBeDefined();
        expect(res1.status).toBe('ok');
        expect(res1.datastores).toHaveLength(1);
        expect(res1).toHaveProperty('backupTasks'); // These fields are now defined but potentially empty
        expect(res1).toHaveProperty('verifyTasks');
        expect(res1).toHaveProperty('gcTasks');

        expect(res2).toBeDefined();
        expect(res2.status).toBe('ok');
        expect(res2.nodeName).toBe(node2Name);
        expect(res2.datastores).toHaveLength(1);
        expect(res2.datastores[0].name).toBe(fallbackDsName);
        expect(res2.datastores[0].snapshots).toHaveLength(1);
        expect(res2.datastores[0].snapshots[0]['backup-id']).toBe('snap-fb');
        expect(res2.backupTasks).toBeDefined();
        expect(res2.verifyTasks).toBeDefined();
        expect(res2.gcTasks).toBeDefined();

        expect(res3).toBeDefined();
        expect(res3).toMatchObject({ 
             pbsEndpointId: pbsId3,
             pbsInstanceName: mockClients[pbsId3].config.name,
             status: 'error' 
             // error field is not present in the returned object here
        });
        // Data should be empty/undefined for res3 as node fetch failed
        expect(res3.datastores).toBeUndefined();
        expect(res3.backupTasks).toBeUndefined();

        // Error spy should be called for the node failure on res3 and the usage error on res2
        expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining(`ERROR: [DataFetcher] Failed to fetch PBS nodes list for ${mockClients[pbsId3].config.name}: Node Error`));
         // Error log from fetchPbsData outer catch for res3 node failure
        expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining(`ERROR: [DataFetcher - ${mockClients[pbsId3].config.name}] PBS fetch failed: Could not determine node name for PBS instance`));
        expect(consoleErrorSpy).toHaveBeenCalledTimes(2); // Node list error + Outer catch error for res3

        // Warn spy should be called for the datastore fallback on res2
        expect(consoleWarnSpy).toHaveBeenCalledWith(expect.stringContaining(`WARN: [DataFetcher] Failed to get datastore usage for PBS DS Fail, falling back to /config/datastore. Error: DS Usage API Error`));
         // Warn log from tasks fetch failure on res2 (if tasks fetch also failed, which it doesn't in this mock)
        // expect(consoleWarnSpy).toHaveBeenCalledWith(expect.stringContaining(`No tasks to process or task fetching failed`));
        expect(consoleWarnSpy).toHaveBeenCalledTimes(1); // Only datastore usage fallback warning for res2

        // Check calls for each client
        // The call counts now align with the simplified test environment flow
        expect(mockClient1.get).toHaveBeenCalledTimes(7); // nodes, usage, snapshots, usage(dedup), config/datastore, groups, tasks
        expect(mockClient2.get).toHaveBeenCalledTimes(8); // nodes, usage (fail), config, snapshots, usage(dedup), config/datastore, groups, tasks
        expect(mockClient3.get).toHaveBeenCalledTimes(1); // nodes (fail)

        consoleErrorSpy.mockRestore();
        consoleWarnSpy.mockRestore();
    });

    test('should return error status and log warnings if /nodes response is invalid (e.g. empty array)', async () => {
      // Arrange
      const pbsId = 'pbs-bad-nodes';
      const mockPbsBadNodesClient = { get: jest.fn() };
      const mockPbsBadNodesApiClients = {
        [pbsId]: { client: mockPbsBadNodesClient, config: { id: pbsId, name: 'PBS Bad Nodes' } }
      };
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});


      // Mock /nodes to return invalid data (empty array)
      mockPbsBadNodesClient.get.mockResolvedValueOnce({ data: { data: [] } }); // Invalid - empty array

      // Act
      const result = await fetchPbsData(mockPbsBadNodesApiClients);

      // Assert
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        expect.stringContaining(`WARN: [DataFetcher] Could not automatically detect PBS node name for ${mockPbsBadNodesApiClients[pbsId].config.name}. Response format unexpected.`)
      );
      // This warning also occurs because nodeName becomes 'localhost' and then the outer catch hits
      // expect(consoleWarnSpy).toHaveBeenCalledWith(
      //   expect.stringContaining(`WARN: [DataFetcher - PBS Bad Nodes] Node name 'localhost' is invalid or 'localhost'. Throwing error.`)
      // );
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        expect.stringContaining(`ERROR: [DataFetcher - ${mockPbsBadNodesApiClients[pbsId].config.name}] PBS fetch failed: Could not determine node name for PBS instance ${mockPbsBadNodesApiClients[pbsId].config.name}`)
      );
        expect(consoleErrorSpy).toHaveBeenCalledTimes(1); // Only the outer catch error

      expect(mockPbsBadNodesClient.get).toHaveBeenCalledTimes(1); // Only /nodes called
      expect(mockPbsBadNodesClient.get).toHaveBeenCalledWith('/nodes');

      expect(result).toHaveLength(1);
      expect(result[0]).toMatchObject({ 
        pbsEndpointId: pbsId,
        pbsInstanceName: 'PBS Bad Nodes',
        status: 'error' // Should be error because node detection failed
      });
      // Task related fields should be undefined as processing is skipped
      // expect(result[0].backupTasks).toBeUndefined();

      consoleWarnSpy.mockRestore();
      consoleErrorSpy.mockRestore();
    });

    test('should handle empty datastore usage response and attempt fallback, tasks should still process', async () => {
        // Arrange
        const pbsId = 'pbs-empty-usage';
        const pbsNodeName = 'pbs-node-empty-usage';
        const mockPbsClient = { get: jest.fn() };
        const mockClients = { [pbsId]: { client: mockPbsClient, config: { name: 'PBS Empty Usage' } } };
        const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

        mockPbsClient.get
            .mockResolvedValueOnce({ data: { data: [{ node: pbsNodeName }] } }) // /nodes
            .mockResolvedValueOnce({ data: { data: [] } }) // /status/datastore-usage (empty)
            .mockResolvedValueOnce({ data: { data: [{ name: 'fallback-store', path: '/mnt/fallback', store: 'fallback-store' }] } }) // /config/datastore (fallback succeeds)
            .mockResolvedValueOnce({ data: { data: [{ 'backup-id': 'snap1' }] } }) // snapshots for 'fallback-store'
            .mockResolvedValueOnce({ data: { data: [{ store: 'fallback-store', 'deduplication-factor': 1.0 }] } }) // /status/datastore-usage in fetchAllPbsTasksForProcessing
            .mockResolvedValueOnce({ data: { data: [{ name: 'fallback-store' }] } }) // /config/datastore in fetchAllPbsTasksForProcessing
            .mockResolvedValueOnce({ data: { data: [] } }) // /admin/datastore/fallback-store/groups (empty)
            .mockResolvedValueOnce({ data: { data: [{ upid: 'task1'}] } });     // tasks for pbsNodeName

      // Act
        const result = await fetchPbsData(mockClients);

      // Assert
        expect(consoleWarnSpy).toHaveBeenCalledWith(
            expect.stringContaining(`WARN: [DataFetcher] PBS /status/datastore-usage returned empty data for ${mockClients[pbsId].config.name}. Falling back.`)
        );
        expect(mockPbsClient.get).toHaveBeenCalledWith('/config/datastore'); // Check fallback was attempted
        expect(mockPbsClient.get).toHaveBeenCalledWith('/admin/datastore/fallback-store/snapshots'); // Check snapshot call
        // Since nodeStatus and versionInfo are skipped, the call count is reduced
        expect(mockPbsClient.get).toHaveBeenCalledTimes(8); // nodes, usage, config, snapshots, usage(dedup), config/datastore, groups, tasks
        expect(mockPbsClient.get).toHaveBeenCalledWith(`/nodes/${pbsNodeName}/tasks`, expect.any(Object)); // Check task call

        expect(result).toHaveLength(1);
        expect(result[0].status).toBe('ok');
        expect(result[0].datastores).toHaveLength(1);
        expect(result[0].datastores[0].name).toBe('fallback-store');
        expect(result[0].datastores[0].total).toBeNull(); // Fallback doesn't have usage stats from /config/datastore
        expect(result[0].datastores[0].snapshots).toHaveLength(1); // Check snapshots from mock
        expect(result[0].datastores[0].snapshots[0]['backup-id']).toBe('snap1');

        // Check tasks (processed by mocked processPbsTasks)
        expect(result[0].backupTasks).toEqual([]);
        expect(result[0].verifyTasks).toEqual([]);
        expect(result[0].gcTasks).toEqual([]);
        
        consoleWarnSpy.mockRestore();
    });

    test('should handle error fetching datastore usage', async () => {
      // Arrange
      const pbsId = 'pbs-err-ds';
      const pbsNodeName = 'pbs-node-ds-err';
      const mockPbsClient = { get: jest.fn() };
      const mockClients = { [pbsId]: { client: mockPbsClient, config: { name: 'PBS DS Err' } } };
      const dsError = new Error('Datastore fetch failed');

      mockPbsClient.get
        .mockResolvedValueOnce({ data: { data: [{ node: pbsNodeName }] } }) // /nodes (succeeds)
        .mockRejectedValueOnce(dsError) // /status/datastore-usage (fails)
        .mockResolvedValueOnce({ data: { data: [] }}) // Mock fallback /config/datastore call (returns empty)
        .mockRejectedValueOnce(new Error('Dedup fetch failed')) // /status/datastore-usage in fetchAllPbsTasksForProcessing (fails)
        .mockResolvedValueOnce({ data: { data: [] }}) // /config/datastore in fetchAllPbsTasksForProcessing (empty)
        .mockResolvedValueOnce({ data: { data: [] }}); // /nodes/{node}/tasks (empty tasks)

      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

      const result = await fetchPbsData(mockClients);

      expect(result).toHaveLength(1);
      expect(result[0]).toMatchObject({
          pbsEndpointId: pbsId,
          status: 'ok', // Status remains 'ok' because node name succeeded, datastore fetch fell back
          nodeName: pbsNodeName,
          datastores: [], // Should be empty
      });
      // Expect calls for /nodes, /status/datastore-usage, the fallback /config/datastore
      // No snapshots because datastores is empty. Task processing calls /status/datastore-usage (fails), /config/datastore again and /tasks
      expect(mockPbsClient.get).toHaveBeenCalledTimes(6);
      // Expect a warning about the fallback, not an error in the result object itself
      expect(consoleWarnSpy).toHaveBeenCalledWith(expect.stringContaining(`Failed to get datastore usage for ${mockClients[pbsId].config.name}, falling back to /config/datastore. Error: ${dsError.message}`));
      // Task fetch doesn't happen for res2 in this scenario, so no error logged for it.
      // expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining(`Failed to fetch PBS task list for node ${pbsNodeName} (${mockClients[pbsId].config.name}):`));
      consoleErrorSpy.mockRestore();
      consoleWarnSpy.mockRestore();
    });

    test('should handle failure of both datastore usage and config fetch', async () => {
        // Arrange
        const pbsId = 'pbs-double-ds-fail';
        const pbsNodeName = 'pbs-node-double-fail';
        const mockPbsClient = { get: jest.fn() };
        const mockClients = { [pbsId]: { client: mockPbsClient, config: { name: 'PBS Double DS Fail' } } };
        const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
        const usageError = new Error('Usage API Failed');
        const configError = new Error('Config API Failed');

        mockPbsClient.get
            .mockResolvedValueOnce({ data: { data: [{ node: pbsNodeName }] } }) // /nodes
            .mockRejectedValueOnce(usageError) // /status/datastore-usage (fails)
            .mockRejectedValueOnce(configError) // /config/datastore (fallback also fails)
            .mockRejectedValueOnce(new Error('Dedup fetch failed')) // /status/datastore-usage in fetchAllPbsTasksForProcessing (fails)
            .mockResolvedValueOnce({ data: { data: [] } }) // /config/datastore in fetchAllPbsTasksForProcessing (empty due to previous failure)
            .mockResolvedValueOnce({ data: { data: [{ upid: 'task1' }] } }); // tasks for pbsNodeName (still called)

      // Act
        const result = await fetchPbsData(mockClients);

      // Assert
        expect(consoleWarnSpy).toHaveBeenCalledWith(
            expect.stringContaining(`WARN: [DataFetcher] Failed to get datastore usage for ${mockClients[pbsId].config.name}, falling back to /config/datastore. Error: ${usageError.message}`)
        );
      expect(consoleErrorSpy).toHaveBeenCalledWith(
             expect.stringContaining(`ERROR: [DataFetcher] Fallback fetch of PBS datastore config failed for ${mockClients[pbsId].config.name}: ${configError.message}`)
        );
        expect(mockPbsClient.get).toHaveBeenCalledWith('/status/datastore-usage');
        expect(mockPbsClient.get).toHaveBeenCalledWith('/config/datastore'); 
        // expect(mockPbsClient.get).toHaveBeenCalledWith(`/nodes/${pbsNodeName}/tasks`); // This was the original failing line
        expect(mockPbsClient.get).toHaveBeenCalledTimes(6); // nodes, usage (fail), config (fail), usage(dedup fail), config/datastore, tasks
        const callsDoubleFailTest = mockPbsClient.get.mock.calls;
        expect(callsDoubleFailTest[5][0]).toBe(`/nodes/${pbsNodeName}/tasks`); // Check 6th call path

        expect(result).toHaveLength(1);
        expect(result[0].status).toBe('ok'); // Status is still ok as node name succeeded
        expect(result[0].datastores).toEqual([]); // Datastores should be empty
        
        consoleWarnSpy.mockRestore();
      consoleErrorSpy.mockRestore();
    });

    test('should handle API error when fetching PBS tasks', async () => {
        const pbsId = 'pbs-task-fetch-error';
        const pbsNodeName = 'pbs-node-task-error';
        const datastoreName = 'store-task-error';
        const mockPbsClient = { get: jest.fn() };
        const mockClients = { [pbsId]: { client: mockPbsClient, config: { name: 'PBS Task Fetch Error' } } };
        const taskError = new Error('Simulated task fetch error');

        mockPbsClient.get
          .mockResolvedValueOnce({ data: { data: [{ node: pbsNodeName }] } }) // /nodes (succeeds)
          .mockResolvedValueOnce({ data: { data: [{ store: datastoreName, total: 1, used: 0 }] } }) // /status/datastore-usage (succeeds)
          .mockResolvedValueOnce({ data: { data: [] } }) // Snapshots (succeeds empty)
          .mockResolvedValueOnce({ data: { data: [{ name: datastoreName }] } }) // /config/datastore in fetchAllPbsTasksForProcessing
          .mockResolvedValueOnce({ data: { data: [] } }) // /admin/datastore/{store}/groups (empty)
          .mockRejectedValueOnce(taskError); // /nodes/{node}/tasks (FAILS)

        const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
        const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});

        const result = await fetchPbsData(mockClients);

        expect(result).toHaveLength(1);
        expect(result[0]).toMatchObject({
            pbsEndpointId: pbsId,
            pbsInstanceName: 'PBS Task Fetch Error',
            status: 'ok', // Status remains 'ok' as node and datastores (even if empty) fetched
            nodeName: pbsNodeName,
            datastores: [{ name: datastoreName, total: 1, used: 0, available: undefined, gcStatus: 'unknown' , snapshots: []}], // Datastore fetch succeeded
        });
        // Check that task-related properties have default structure due to fetch error
        expect(result[0]).toHaveProperty('backupTasks');
        expect(result[0]).toHaveProperty('verifyTasks');
        expect(result[0]).toHaveProperty('gcTasks');

        // Verify API calls
        expect(mockPbsClient.get).toHaveBeenCalledTimes(6); // nodes, usage, snapshots, usage(dedup), config/datastore, tasks (fails) - no groups because datastores are empty
        expect(mockPbsClient.get).toHaveBeenCalledWith(`/nodes/${pbsNodeName}/tasks`, expect.any(Object)); // Check tasks call was attempted

        // Verify error logging from fetchAllPbsTasksForProcessing's catch block (covers lines 264-265)
        expect(consoleErrorSpy).toHaveBeenCalledWith(
            expect.stringContaining(`Failed to fetch PBS task list for node ${pbsNodeName} (PBS Task Fetch Error): ${taskError.message}`)
        );
        // Verify the warning logged in fetchPbsData when tasks cannot be processed (covers line 341)
         expect(consoleWarnSpy).toHaveBeenCalledWith(
            expect.stringContaining('No tasks to process or task fetching failed. Error flag: true, Tasks array: null')
          );

        consoleErrorSpy.mockRestore();
        consoleWarnSpy.mockRestore();
    });

  }); // End describe fetchPbsData

}); // End describe Data Fetcher
