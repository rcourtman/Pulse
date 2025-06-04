const { fetchDiscoveryData, fetchMetricsData, fetchPbsData, clearCaches } = require('../dataFetcher');
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
    // Clear caches to prevent test pollution
    clearCaches();
    
    // Restore environment (can now access originalEnv)
    const currentEnvKeys = Object.keys(process.env);
    currentEnvKeys.forEach(key => delete process.env[key]);
    Object.keys(originalEnv).forEach(key => { process.env[key] = originalEnv[key]; });
    // --- Remove console restore --- 
    // jest.restoreAllMocks(); 
  });

  describe('fetchDiscoveryData', () => {
    test('should return empty structure when no PVE clients configured', async () => {
      const mockPbsFunction = jest.fn().mockResolvedValue([]);
      
      const result = await fetchDiscoveryData({}, mockPbsApiClient, mockPbsFunction);
      
      expect(result.nodes).toEqual([]);
      expect(result.vms).toEqual([]);
      expect(result.containers).toEqual([]);
      expect(result.pbs).toEqual([]);
      expect(mockPbsFunction).toHaveBeenCalled();
    });

    test('should fetch basic PVE cluster data successfully', async () => {
      const mockClient = {
        primary: {
          client: {
            get: jest.fn()
              .mockResolvedValueOnce({ data: { data: [{ type: 'cluster', nodes: 1 }] } })
              .mockResolvedValue({ data: { data: [] } })
          },
          config: { id: 'primary', name: 'Primary PVE' }
        }
      };

      const result = await fetchDiscoveryData(mockClient, {});

      expect(result).toHaveProperty('nodes');
      expect(result).toHaveProperty('vms'); 
      expect(result).toHaveProperty('containers');
      expect(result).toHaveProperty('pbs');
      expect(result).toHaveProperty('pveBackups');
      expect(Array.isArray(result.nodes)).toBe(true);
      expect(Array.isArray(result.vms)).toBe(true);
      expect(Array.isArray(result.containers)).toBe(true);
    });

    test('should handle bad node storage data gracefully', async () => {
      const mockClient = {
        primary: {
          client: {
            get: jest.fn().mockResolvedValue({ data: { data: [] } })
          },
          config: { id: 'primary', name: 'Primary PVE' }
        }
      };

      const result = await fetchDiscoveryData(mockClient, {});

      expect(result).toHaveProperty('nodes');
      expect(Array.isArray(result.nodes)).toBe(true);
    });

    test('should handle missing node data gracefully', async () => {
      const mockClient = {
        primary: {
          client: {
            get: jest.fn().mockResolvedValue({ data: { data: null } })
          },
          config: { id: 'primary', name: 'Primary PVE' }
        }
      };

      const result = await fetchDiscoveryData(mockClient, {});

      expect(result).toEqual({
        nodes: [],
        vms: [],
        containers: [],
        pbs: [],
        pveBackups: expect.any(Object)
      });
    });

    test('should work with multiple PVE endpoints', async () => {
      const mockClients = {
        pve1: {
          client: {
            get: jest.fn().mockResolvedValue({ data: { data: [] } })
          },
          config: { id: 'pve1', name: 'PVE1' }
        },
        pve2: {
          client: {
            get: jest.fn().mockRejectedValue(new Error('Network error'))
          },
          config: { id: 'pve2', name: 'PVE2' }
        }
      };

      const result = await fetchDiscoveryData(mockClients, {});

      expect(result).toEqual({
        nodes: [],
        vms: [],
        containers: [],
        pbs: [],
        pveBackups: expect.any(Object)
      });
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

    test('should integrate PVE and PBS data successfully', async () => {
      const mockPveClient = {
        primary: {
          client: {
            get: jest.fn().mockResolvedValue({ data: { data: [] } })
          },
          config: { id: 'primary', name: 'Primary PVE' }
        }
      };
      const mockPbsClient = {
        'pbs-1': {
          client: {
            get: jest.fn().mockResolvedValue({ data: { data: [] } })
          },
          config: { name: 'PBS Instance' }
        }
      };

      const result = await fetchDiscoveryData(mockPveClient, mockPbsClient);

      expect(result).toHaveProperty('nodes');
      expect(result).toHaveProperty('vms');
      expect(result).toHaveProperty('containers');
      expect(result).toHaveProperty('pbs');
      expect(result).toHaveProperty('pveBackups');
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
      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient, mockPbsFunction);

      expect(result.nodes).toHaveLength(0);
      expect(result.vms).toHaveLength(0);
      expect(result.containers).toHaveLength(0);
      expect(mockPbsFunction).toHaveBeenCalledWith(mockPbsApiClient);
      expect(result.pbs).toEqual([]);
    });

    test('should work without PBS clients configured', async () => {
      const mockPveClient = {
        primary: {
          client: {
            get: jest.fn().mockResolvedValue({ data: { data: [] } })
          },
          config: { id: 'primary', name: 'Primary PVE' }
        }
      };

      const result = await fetchDiscoveryData(mockPveClient, {});

      expect(result).toHaveProperty('pbs');
    });

    test('should handle error fetching Containers (lxc)', async () => {
      // Arrange: Uses the default mock clients from beforeEach
      const nodeNameGood = 'node-good';
      const nodeNameBad = 'node-bad-guests'; // This node will have the LXC fetch error
      const endpointId = 'primary'; // Default endpointId from mockPveApiClient setup
      mockPveClientInstance.get.mockImplementation(async (url) => {
        if (url === '/cluster/status') {
          return { data: { data: [{ type: 'cluster', nodes: 2, name: 'test-cluster' }] } };
        }
        if (url === '/nodes') {
          return { data: { data: [
            { node: nodeNameGood, status: 'online', id: `node/${nodeNameGood}` },
            { node: nodeNameBad, status: 'online', id: `node/${nodeNameBad}` }
          ]}};
        }
        if (url === `/nodes/${nodeNameGood}/status`) return { data: { data: { cpu: 0.1, uptime: 10 } } };
        if (url === `/nodes/${nodeNameGood}/storage`) return { data: { data: [] } };
        if (url === `/nodes/${nodeNameGood}/qemu`) return { data: { data: [ { vmid: 100, name: 'vm-good', status: 'running' } ] } };
        if (url === `/nodes/${nodeNameGood}/lxc`) return { data: { data: [] } };
        if (url === `/nodes/${nodeNameBad}/status`) return { data: { data: { cpu: 0.2, uptime: 20 } } };
        if (url === `/nodes/${nodeNameBad}/storage`) return { data: { data: [] } };
        if (url === `/nodes/${nodeNameBad}/qemu`) return { data: { data: [] } };
        if (url === `/nodes/${nodeNameBad}/lxc`) {
          throw new Error('Simulated LXC Fetch Error');
        }
        throw new Error(`Unexpected API call in mock: ${url}`);
      });

      const result = await fetchDiscoveryData(mockPveApiClient, mockPbsApiClient);

      expect(result.nodes).toHaveLength(2);
      const goodNodeResult = result.nodes.find(n => n.node === nodeNameGood);
      const badNodeResult = result.nodes.find(n => n.node === nodeNameBad);
      expect(goodNodeResult).toBeDefined();
      expect(badNodeResult).toBeDefined();
      expect(result.vms).toHaveLength(1);
      expect(result.vms[0].vmid).toBe(100);
      expect(result.containers).toHaveLength(0);
    });

    test('should handle API failures gracefully', async () => {
      // Test the behavior: when APIs fail, return empty results instead of crashing
      const failingApiClient = {
        primary: {
          client: {
            get: jest.fn().mockRejectedValue(new Error('API unavailable'))
          },
          config: { id: 'primary', name: 'Primary PVE' }
        }
      };

      const result = await fetchDiscoveryData(failingApiClient, {});

      // Verify behavior: should return empty structure, not crash
      expect(result).toEqual({
        nodes: [],
        vms: [],
        containers: [],
        pbs: [],
        pveBackups: expect.any(Object)
      });
    });

    test('should handle invalid node status data gracefully', async () => {
      const mockClient = {
        primary: {
          client: {
            get: jest.fn().mockResolvedValue({ data: { data: null } })
          },
          config: { id: 'primary', name: 'Primary PVE' }
        }
      };

      const result = await fetchDiscoveryData(mockClient, {});

      expect(result).toEqual({
        nodes: [],
        vms: [],
        containers: [],
        pbs: [],
        pveBackups: expect.any(Object)
      });
    });


    test('should handle malformed API responses gracefully', async () => {
      const invalidApiClient = {
        primary: {
          client: {
            get: jest.fn().mockResolvedValue({ data: { data: 'invalid-format' } })
          },
          config: { id: 'primary', name: 'Primary PVE' }
        }
      };
      
      const result = await fetchDiscoveryData(invalidApiClient, {});
      
      expect(result).toEqual({
        nodes: [],
        vms: [],
        containers: [],
        pbs: [],
        pveBackups: expect.any(Object)
      });
    });

    test('should continue working when some endpoints fail', async () => {
      const mixedClients = {
        primary: {
          client: {
            get: jest.fn().mockResolvedValue({ data: { data: [] } })
          },
          config: { id: 'primary', name: 'Working PVE' }
        },
        broken: null
      };
      
      const result = await fetchDiscoveryData(mixedClients, {});
      
      expect(result).toEqual({
        nodes: [],
        vms: [],
        containers: [],
        pbs: [],
        pveBackups: expect.any(Object)
      });
    });

    test('should work without PBS clients', async () => {
      const pveOnlyClient = {
        primary: {
          client: {
            get: jest.fn().mockResolvedValue({ data: { data: [] } })
          },
          config: { id: 'primary', name: 'Primary PVE' }
        }
      };
      
      const result = await fetchDiscoveryData(pveOnlyClient, null);
      
      expect(result.pbs).toEqual([]);
      expect(result).toHaveProperty('nodes');
      expect(result).toHaveProperty('vms');
      expect(result).toHaveProperty('containers');
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

    test('should fetch VM metrics successfully', async () => {
        const mockApiClients = {
            'primary': {
                client: {
                    get: jest.fn().mockResolvedValue({ data: { data: [{ cpu: 0.5 }] } })
                },
                config: { name: 'Primary PVE' }
            }
        };

        const runningVms = [
            { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm-test' }
        ];

        const result = await fetchMetricsData(runningVms, [], mockApiClients);

        expect(Array.isArray(result)).toBe(true);
        expect(result.length).toBeGreaterThanOrEqual(0);
    });

    test('should fetch container metrics successfully', async () => {
        const mockApiClients = {
            'primary': {
                client: {
                    get: jest.fn().mockResolvedValue({ data: { data: [{ cpu: 0.2 }] } })
                },
                config: { name: 'Primary PVE' }
            }
        };

        const runningContainers = [
            { endpointId: 'primary', node: 'node2', vmid: 101, type: 'lxc', name: 'ct-test' }
        ];

        const result = await fetchMetricsData([], runningContainers, mockApiClients);

        expect(Array.isArray(result)).toBe(true);
        expect(result.length).toBeGreaterThanOrEqual(0);
    });

    test('should fetch metrics for multiple guests successfully', async () => {
        const mockApiClients = {
            'primary': {
                client: {
                    get: jest.fn().mockResolvedValue({ data: { data: [{ cpu: 0.1 }] } })
                },
                config: { name: 'Primary PVE' }
            }
        };

        const runningVms = [
            { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm1' }
        ];
        const runningContainers = [
            { endpointId: 'primary', node: 'node1', vmid: 101, type: 'lxc', name: 'ct1' }
        ];

        const result = await fetchMetricsData(runningVms, runningContainers, mockApiClients);

        expect(Array.isArray(result)).toBe(true);
        expect(result.length).toBeGreaterThanOrEqual(0);
    });

    test('should handle missing API client gracefully', async () => {
        const mockApiClients = {
            'primary': {
                client: {
                    get: jest.fn().mockResolvedValue({ data: { data: [{ cpu: 0.5 }] } })
                },
                config: { name: 'Primary PVE' }
            }
        };

        const runningVms = [
            { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm-good' }, 
            { endpointId: 'missing', node: 'nodeX', vmid: 999, type: 'qemu', name: 'vm-bad' }
        ];

        const result = await fetchMetricsData(runningVms, [], mockApiClients);

        expect(Array.isArray(result)).toBe(true);
        expect(result.length).toBeGreaterThanOrEqual(0);
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
        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);

        expect(Array.isArray(result)).toBe(true);
    });

    test('should handle API error when fetching current status for one guest', async () => {
        const runningVms = [
            { endpointId: 'primary', node: 'node1', vmid: 100, type: 'qemu', name: 'vm-ok' },
            { endpointId: 'primary', node: 'node1', vmid: 101, type: 'qemu', name: 'vm-fail-current' }
        ];

        const error = new Error('Current Status Fetch Failed');
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ cpu: 0.1 }] } });
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: { cpu: 0.11 } } });
        mockPveClientInstance.get.mockResolvedValueOnce({ data: { data: [{ cpu: 0.2 }] } });
        mockPveClientInstance.get.mockRejectedValueOnce(error);

        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);

        expect(result).toHaveLength(1);
        expect(result[0].id).toBe(100);
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

        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);

        expect(result).toHaveLength(1);
        expect(result[0].id).toBe(100);
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
      const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
      expect(result).toHaveLength(1);
      expect(result[0].current.guest_mem_total_bytes).toBeUndefined();
    });

    test('should handle unexpected QEMU guest agent response format', async () => {
      const runningVms = [
        { endpointId: 'primary', node: 'node1', vmid: 104, type: 'qemu', name: 'vm-agent-bad-format', agent: '1' }
      ];
       mockPveClientInstance.get
        .mockResolvedValueOnce({ data: { data: [{ time: 1, cpu: 0.5 }] } }) // RRD
        .mockResolvedValueOnce({ data: { data: { cpu: 0.5, mem: 1024, disk: 2048, agent: 1 } } }); // Current status
      mockPveClientInstance.post = jest.fn().mockResolvedValueOnce({ data: { data: { result: { unexpected: "data" } } } });

      const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
      expect(result).toHaveLength(1);
      expect(result[0].current.guest_mem_total_bytes).toBeUndefined();
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
      const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);
      expect(result).toHaveLength(1);
      expect(result[0].current.guest_mem_total_bytes).toBeUndefined();
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

        const result = await fetchMetricsData(runningVms, [], mockCurrentApiClients);

        expect(result).toHaveLength(0);
        expect(mockPveClientInstance.get).toHaveBeenCalledTimes(2);
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

    test('should fetch PBS data successfully', async () => {
        const mockClients = {
            'pbs-1': {
                client: {
                    get: jest.fn()
                        .mockResolvedValueOnce({ data: { data: [{ node: 'pbs-node' }] } })
                        .mockResolvedValue({ data: { data: [] } })
                },
                config: { name: 'PBS Instance' }
            }
        };

        const result = await fetchPbsData(mockClients);

        expect(result).toHaveLength(1);
        expect(result[0]).toHaveProperty('pbsEndpointId');
        expect(result[0]).toHaveProperty('pbsInstanceName');
        expect(result[0]).toHaveProperty('status');
    });

    test('should handle error fetching PBS node name (and skip subsequent calls)', async () => {
        const pbsId = 'pbs-err-node';
        const mockPbsClient = { get: jest.fn() };
        const mockClients = { [pbsId]: { client: mockPbsClient, config: { name: 'PBS Node Err' } } };
        const nodeError = new Error('Node fetch failed');

        // Mock /nodes to fail
        mockPbsClient.get.mockRejectedValueOnce(nodeError);

        const result = await fetchPbsData(mockClients);

        expect(result).toHaveLength(1);
        expect(result[0].status).toBe('error');
        expect(result[0].datastores).toBeUndefined();
        expect(result[0].backupTasks).toBeUndefined();
        expect(mockPbsClient.get).toHaveBeenCalledTimes(1);
        expect(mockPbsClient.get).toHaveBeenCalledWith('/nodes');
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

        const result = await fetchPbsData(mockClients);

        expect(result).toHaveLength(1);
        expect(result[0]).toMatchObject({
            pbsEndpointId: pbsId,
            status: 'ok',
            nodeName: pbsNodeName,
            datastores: [],
        });
        expect(mockPbsClient.get).toHaveBeenCalledTimes(6);
    });

     test('should handle partial datastore failures', async () => {
        const mockClients = {
            'pbs-1': {
                client: {
                    get: jest.fn()
                        .mockResolvedValueOnce({ data: { data: [{ node: 'node1' }] } })
                        .mockResolvedValueOnce({ data: { data: [{ store: 'ds1' }] } })
                        .mockRejectedValueOnce(new Error('Snapshot fetch failed'))
                },
                config: { name: 'PBS 1' }
            }
        };

        const result = await fetchPbsData(mockClients);
        
        expect(result).toHaveLength(1);
        expect(result[0].status).toBe('ok');
    });

    test('should handle PBS task fetch failures', async () => {
        const mockClients = {
            'pbs-1': {
                client: {
                    get: jest.fn()
                        .mockResolvedValueOnce({ data: { data: [{ node: 'pbs-node' }] } })
                        .mockRejectedValue(new Error('Task fetch failed'))
                },
                config: { name: 'PBS Instance' }
            }
        };

        const result = await fetchPbsData(mockClients);

        expect(result).toHaveLength(1);
        expect(result[0]).toHaveProperty('status');
    });

    test('should handle multiple PBS instances with mixed results', async () => {
        const mockClients = {
            'pbs-1': {
                client: {
                    get: jest.fn()
                        .mockResolvedValueOnce({ data: { data: [{ node: 'node1' }] } })
                        .mockResolvedValue({ data: { data: [] } })
                },
                config: { name: 'PBS 1' }
            },
            'pbs-2': {
                client: {
                    get: jest.fn().mockRejectedValue(new Error('Connection failed'))
                },
                config: { name: 'PBS 2' }
            }
        };

        const result = await fetchPbsData(mockClients);
        
        expect(result).toHaveLength(2);
        expect(result.some(r => r.status === 'ok')).toBe(true);
        expect(result.some(r => r.status === 'error')).toBe(true);
    });

    test('should return error status and log warnings if /nodes response is invalid (e.g. empty array)', async () => {
      // Arrange
      const pbsId = 'pbs-bad-nodes';
      const mockPbsBadNodesClient = { get: jest.fn() };
      const mockPbsBadNodesApiClients = {
        [pbsId]: { client: mockPbsBadNodesClient, config: { id: pbsId, name: 'PBS Bad Nodes' } }
      };
      mockPbsBadNodesClient.get.mockResolvedValueOnce({ data: { data: [] } });

      const result = await fetchPbsData(mockPbsBadNodesApiClients);

      expect(mockPbsBadNodesClient.get).toHaveBeenCalledTimes(1);
      expect(mockPbsBadNodesClient.get).toHaveBeenCalledWith('/nodes');
      expect(result).toHaveLength(1);
      expect(result[0]).toMatchObject({ 
        pbsEndpointId: pbsId,
        pbsInstanceName: 'PBS Bad Nodes',
        status: 'error'
      });
    });

    test('should handle empty datastore usage with fallback', async () => {
        const mockClients = {
            'pbs-1': {
                client: {
                    get: jest.fn()
                        .mockResolvedValueOnce({ data: { data: [{ node: 'pbs-node' }] } })
                        .mockResolvedValueOnce({ data: { data: [] } })
                        .mockResolvedValue({ data: { data: [] } })
                },
                config: { name: 'PBS Instance' }
            }
        };

        const result = await fetchPbsData(mockClients);

        expect(result).toHaveLength(1);
        expect(result[0]).toHaveProperty('status');
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

      const result = await fetchPbsData(mockClients);

      expect(result).toHaveLength(1);
      expect(result[0]).toMatchObject({
          pbsEndpointId: pbsId,
          status: 'ok',
          nodeName: pbsNodeName,
          datastores: [],
      });
      expect(mockPbsClient.get).toHaveBeenCalledTimes(6);
    });

    test('should handle failure of both datastore usage and config fetch', async () => {
        // Arrange
        const pbsId = 'pbs-double-ds-fail';
        const pbsNodeName = 'pbs-node-double-fail';
        const mockPbsClient = { get: jest.fn() };
        const mockClients = { [pbsId]: { client: mockPbsClient, config: { name: 'PBS Double DS Fail' } } };
        const usageError = new Error('Usage API Failed');
        const configError = new Error('Config API Failed');

        mockPbsClient.get
            .mockResolvedValueOnce({ data: { data: [{ node: pbsNodeName }] } })
            .mockRejectedValueOnce(usageError)
            .mockRejectedValueOnce(configError)
            .mockRejectedValueOnce(new Error('Dedup fetch failed'))
            .mockResolvedValueOnce({ data: { data: [] } })
            .mockResolvedValueOnce({ data: { data: [{ upid: 'task1' }] } });

        const result = await fetchPbsData(mockClients);

        expect(mockPbsClient.get).toHaveBeenCalledWith('/status/datastore-usage');
        expect(mockPbsClient.get).toHaveBeenCalledWith('/config/datastore');
        expect(mockPbsClient.get).toHaveBeenCalledTimes(6);
        const callsDoubleFailTest = mockPbsClient.get.mock.calls;
        expect(callsDoubleFailTest[5][0]).toBe(`/nodes/${pbsNodeName}/tasks`);
        expect(result).toHaveLength(1);
        expect(result[0].status).toBe('ok');
        expect(result[0].datastores).toEqual([]);
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

        const result = await fetchPbsData(mockClients);

        expect(result).toHaveLength(1);
        expect(result[0]).toMatchObject({
            pbsEndpointId: pbsId,
            pbsInstanceName: 'PBS Task Fetch Error',
            status: 'ok',
            nodeName: pbsNodeName,
            datastores: [{ name: datastoreName, total: 1, used: 0, available: undefined, gcStatus: 'unknown' , snapshots: []}],
        });
        expect(result[0]).toHaveProperty('backupTasks');
        expect(result[0]).toHaveProperty('verifyTasks');
        expect(result[0]).toHaveProperty('gcTasks');
        expect(mockPbsClient.get).toHaveBeenCalledTimes(6);
        expect(mockPbsClient.get).toHaveBeenCalledWith(`/nodes/${pbsNodeName}/tasks`, expect.any(Object));
    });

  }); // End describe fetchPbsData

}); // End describe Data Fetcher
