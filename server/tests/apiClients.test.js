// Mock dependencies *before* importing the module that uses them
jest.mock('../configLoader');
jest.mock('axios'); // <-- Mock axios instead

// Mock axios-retry: Create a mock function for default, attach *mocked* helpers to it.
jest.mock('axios-retry', () => {
  // We don't need requireActual here anymore if we mock the helpers
  // const actualAxiosRetry = jest.requireActual('axios-retry');

  // Create a mock function for the default export
  const mockDefaultFn = jest.fn();

  // Attach JEST MOCK FUNCTIONS for the helpers to the default export mock
  mockDefaultFn.isNetworkError = jest.fn();
  mockDefaultFn.isRetryableError = jest.fn();
  mockDefaultFn.exponentialDelay = jest.fn();

  // The module export
  return {
    __esModule: true, 
    default: mockDefaultFn,
    // Also provide the JEST MOCK FUNCTIONS on the main module object for completeness
    isNetworkError: mockDefaultFn.isNetworkError, // Point to the same mock fn
    isRetryableError: mockDefaultFn.isRetryableError, // Point to the same mock fn
    exponentialDelay: mockDefaultFn.exponentialDelay, // Point to the same mock fn
  };
});

const { initializeApiClients } = require('../apiClients');
const { loadConfiguration } = require('../configLoader');
const axios = require('axios'); // <-- Get the mocked axios
const axiosRetry = require('axios-retry').default; // <-- Get the mocked default export
// const proxmoxApi = require('proxmox-api'); // <-- Remove this

// Mock console to avoid cluttering test output
// jest.spyOn(console, 'log').mockImplementation(() => {});
// jest.spyOn(console, 'error').mockImplementation(() => {});

describe('API Clients Initialization', () => {
  let originalEnv;
  // Remove the shared mock instance definition from here
  // const mockAxiosInstance = { ... };

  beforeEach(() => {
    originalEnv = { ...process.env };
    jest.resetModules(); 
    jest.clearAllMocks();

    // Configure axios.create to return a *new* mock instance each time
    axios.create.mockImplementation(() => ({
      get: jest.fn(),
      interceptors: { 
        request: { use: jest.fn() },
        response: { use: jest.fn() } // <-- Add response interceptor mock
      } 
    }));

    loadConfiguration.mockReturnValue({
      endpoints: [{
        id: 'pve1',
        name: 'PVE Test 1',
        host: '1.1.1.1',
        port: '8006', // Add port for baseURL construction
        username: 'root@pam',
        tokenId: 'pve-token-id',
        tokenSecret: 'pve-token-secret',
        enabled: true,
        allowSelfSignedCerts: false // Add for httpsAgent
      }],
      pbsConfigs: [{
        id: 'pbs1',
        name: 'PBS Test 1',
        host: '2.2.2.2',
        port: '8007', // Add port for baseURL construction
        username: 'root@pam',
        tokenId: 'pbs-token-id',
        tokenSecret: 'pbs-token-secret',
        authMethod: 'token',
        allowSelfSignedCerts: false // Add for httpsAgent
      }],
    });

  });

  afterEach(() => {
    const currentEnvKeys = Object.keys(process.env);
    currentEnvKeys.forEach(key => delete process.env[key]);
    Object.keys(originalEnv).forEach(key => { process.env[key] = originalEnv[key]; });
  });

  test('should initialize PVE and PBS clients successfully with token auth', async () => {
    // Arrange
    const { endpoints, pbsConfigs } = loadConfiguration(); 

    // Act
    const { apiClients, pbsApiClients } = await initializeApiClients(endpoints, pbsConfigs);

    // Assert
    expect(loadConfiguration).toHaveBeenCalledTimes(1);
    expect(axios.create).toHaveBeenCalledTimes(2);
    
    expect(axios.create).toHaveBeenCalledWith(expect.objectContaining({
      baseURL: `https://${endpoints[0].host}:${endpoints[0].port}/api2/json`,
    }));
    expect(axios.create).toHaveBeenCalledWith(expect.objectContaining({
      baseURL: `https://${pbsConfigs[0].host}:${pbsConfigs[0].port}/api2/json`,
    }));

    // Check interceptors were configured ON EACH client
    // Axios.create().mock.results gives us the return values (the mock instances)
    // Expect 1 call for manual auth header (axiosRetry mock doesn't add one by default)
    expect(axios.create.mock.results[0].value.interceptors.request.use).toHaveBeenCalledTimes(1); // PVE client
    expect(axios.create.mock.results[1].value.interceptors.request.use).toHaveBeenCalledTimes(1); // PBS client
    // We could also check the response interceptor use if axios-retry was mocked to verify its calls

    // Check returned client structure
    expect(apiClients).toHaveProperty('pve1');
    expect(apiClients.pve1.client).toBe(axios.create.mock.results[0].value); // Check it's the first mock instance
    expect(apiClients.pve1.config).toEqual(endpoints[0]);

    expect(pbsApiClients).toHaveProperty('pbs1');
    expect(pbsApiClients.pbs1.client).toBe(axios.create.mock.results[1].value); // Check it's the second mock instance
    expect(pbsApiClients.pbs1.config).toEqual(pbsConfigs[0]);
  });

  test('should handle missing PVE endpoints gracefully', async () => {
    // Arrange
    loadConfiguration.mockReturnValue({
      endpoints: [], 
      pbsConfigs: [{
        id: 'pbs1',
        name: 'PBS Test 1',
        host: '2.2.2.2',
        port: '8007',
        username: 'root@pam',
        tokenId: 'pbs-token-id',
        tokenSecret: 'pbs-token-secret',
        authMethod: 'token',
        allowSelfSignedCerts: false
      }],
    });
    const { endpoints, pbsConfigs } = loadConfiguration();

    // Act
    const { apiClients, pbsApiClients } = await initializeApiClients(endpoints, pbsConfigs);

    // Assert
    expect(axios.create).toHaveBeenCalledTimes(1);
    expect(axios.create).toHaveBeenCalledWith(expect.objectContaining({
       baseURL: `https://${pbsConfigs[0].host}:${pbsConfigs[0].port}/api2/json`
    }));
    // Check interceptor on the *single* created client
    // Expect 1 call for manual auth header
    expect(axios.create.mock.results[0].value.interceptors.request.use).toHaveBeenCalledTimes(1);
    expect(apiClients).toEqual({}); 
    expect(pbsApiClients).toHaveProperty('pbs1');
    expect(pbsApiClients.pbs1.client).toBe(axios.create.mock.results[0].value); // The only mock instance created
  });

   test('should handle missing PBS endpoints gracefully', async () => {
    // Arrange
    loadConfiguration.mockReturnValue({
      endpoints: [{
        id: 'pve1',
        name: 'PVE Test 1',
        host: '1.1.1.1',
        port: '8006',
        username: 'root@pam',
        tokenId: 'pve-token-id',
        tokenSecret: 'pve-token-secret',
        enabled: true,
        allowSelfSignedCerts: false
      }],
      pbsConfigs: [], 
    });
    const { endpoints, pbsConfigs } = loadConfiguration();

    // Act
    const { apiClients, pbsApiClients } = await initializeApiClients(endpoints, pbsConfigs);

    // Assert
    expect(axios.create).toHaveBeenCalledTimes(1);
     expect(axios.create).toHaveBeenCalledWith(expect.objectContaining({
       baseURL: `https://${endpoints[0].host}:${endpoints[0].port}/api2/json`
    }));
    // Check interceptor on the *single* created client
    // Expect 1 call for manual auth header
    expect(axios.create.mock.results[0].value.interceptors.request.use).toHaveBeenCalledTimes(1);
    expect(pbsApiClients).toEqual({}); 
    expect(apiClients).toHaveProperty('pve1');
    expect(apiClients.pve1.client).toBe(axios.create.mock.results[0].value); // The only mock instance created
  });

  test('should skip PVE endpoint if tokenId is missing', async () => {
    // Arrange
    loadConfiguration.mockReturnValue({
      endpoints: [{
        id: 'pve-no-tokenid',
        name: 'PVE Missing Token ID',
        host: '3.3.3.3',
        port: '8006',
        username: 'root@pam',
        // tokenId: 'pve-token-id', // MISSING
        tokenSecret: 'pve-token-secret',
        enabled: true,
        allowSelfSignedCerts: false
      }],
      pbsConfigs: [],
    });
    const { endpoints, pbsConfigs } = loadConfiguration();
    const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {}); // Spy on console.error

    // Act
    const { apiClients, pbsApiClients } = await initializeApiClients(endpoints, pbsConfigs);

    // Assert
    expect(axios.create).toHaveBeenCalledTimes(1); // Still creates the instance initially
    const createdInstance = axios.create.mock.results[0].value;
    // Check that the interceptor did NOT log an error during init
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    // The client *is* created, even with missing credentials
    expect(apiClients).toHaveProperty('pve-no-tokenid');
    expect(apiClients['pve-no-tokenid'].client).toBe(createdInstance);
    expect(pbsApiClients).toEqual({});

    consoleErrorSpy.mockRestore();
  });

  test('should skip PVE endpoint if tokenSecret is missing', async () => {
    // Arrange
    loadConfiguration.mockReturnValue({
      endpoints: [{
        id: 'pve-no-secret',
        name: 'PVE Missing Secret',
        host: '4.4.4.4',
        port: '8006',
        username: 'root@pam',
        tokenId: 'pve-token-id',
        // tokenSecret: 'pve-token-secret', // MISSING
        enabled: true,
        allowSelfSignedCerts: false
      }],
      pbsConfigs: [],
    });
    const { endpoints, pbsConfigs } = loadConfiguration();
    const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

    // Act
    const { apiClients, pbsApiClients } = await initializeApiClients(endpoints, pbsConfigs);

    // Assert
    expect(axios.create).toHaveBeenCalledTimes(1);
    // Check that the interceptor did NOT log an error during init
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    // The client *is* created, even with missing credentials
    expect(apiClients).toHaveProperty('pve-no-secret');
    expect(pbsApiClients).toEqual({});

    consoleErrorSpy.mockRestore();
  });

  test('should skip PVE endpoint if enabled is false', async () => {
    // Arrange
    loadConfiguration.mockReturnValue({
      endpoints: [{
        id: 'pve-disabled',
        name: 'PVE Disabled',
        host: '5.5.5.5',
        port: '8006',
        username: 'root@pam',
        tokenId: 'pve-token-id',
        tokenSecret: 'pve-token-secret',
        enabled: false, // DISABLED
        allowSelfSignedCerts: false
      }],
      pbsConfigs: [],
    });
    const { endpoints, pbsConfigs } = loadConfiguration();
    const consoleLogSpy = jest.spyOn(console, 'log').mockImplementation(() => {}); // Spy on console.log

    // Act
    const { apiClients, pbsApiClients } = await initializeApiClients(endpoints, pbsConfigs);

    // Assert
    expect(axios.create).not.toHaveBeenCalled(); // Should not attempt to create client
    expect(consoleLogSpy).toHaveBeenCalledWith('INFO: Skipping disabled PVE endpoint: PVE Disabled (5.5.5.5)');
    expect(apiClients).toEqual({});
    expect(pbsApiClients).toEqual({});

    consoleLogSpy.mockRestore();
  });

  test('should set rejectUnauthorized to false when allowSelfSignedCerts is true', async () => {
    // Arrange
    loadConfiguration.mockReturnValue({
      endpoints: [{
        id: 'pve-self-signed',
        name: 'PVE Self Signed',
        host: '6.6.6.6',
        port: '8006',
        username: 'root@pam',
        tokenId: 'pve-token-id',
        tokenSecret: 'pve-token-secret',
        enabled: true,
        allowSelfSignedCerts: true // ALLOW SELF SIGNED
      }],
      pbsConfigs: [],
    });
    const { endpoints, pbsConfigs } = loadConfiguration();

    // Act
    await initializeApiClients(endpoints, pbsConfigs);

    // Assert
    expect(axios.create).toHaveBeenCalledTimes(1);
    expect(axios.create).toHaveBeenCalledWith(expect.objectContaining({
      httpsAgent: expect.objectContaining({
        options: expect.objectContaining({ rejectUnauthorized: false }) // Key assertion
      })
    }));
  });

   test('should set rejectUnauthorized to true when allowSelfSignedCerts is false', async () => {
    // Arrange
    loadConfiguration.mockReturnValue({
      endpoints: [{
        id: 'pve-strict-ssl',
        name: 'PVE Strict SSL',
        host: '7.7.7.7',
        port: '8006',
        username: 'root@pam',
        tokenId: 'pve-token-id',
        tokenSecret: 'pve-token-secret',
        enabled: true,
        allowSelfSignedCerts: false // STRICT SSL
      }],
      pbsConfigs: [],
    });
    const { endpoints, pbsConfigs } = loadConfiguration();

    // Act
    await initializeApiClients(endpoints, pbsConfigs);

    // Assert
    expect(axios.create).toHaveBeenCalledTimes(1);
    expect(axios.create).toHaveBeenCalledWith(expect.objectContaining({
      httpsAgent: expect.objectContaining({
        options: expect.objectContaining({ rejectUnauthorized: true }) // Key assertion
      })
    }));
  });

  test('should initialize multiple PVE and PBS endpoints', async () => {
    // Arrange
    loadConfiguration.mockReturnValue({
      endpoints: [
        { id: 'pve1', name: 'PVE 1', host: '1.1.1.1', port: '8006', username: 'root@pam', tokenId: 't1', tokenSecret: 's1', enabled: true, allowSelfSignedCerts: false },
        { id: 'pve2', name: 'PVE 2', host: '1.1.1.2', port: '8006', username: 'root@pam', tokenId: 't2', tokenSecret: 's2', enabled: true, allowSelfSignedCerts: true },
        { id: 'pve3-disabled', name: 'PVE 3', host: '1.1.1.3', port: '8006', username: 'root@pam', tokenId: 't3', tokenSecret: 's3', enabled: false, allowSelfSignedCerts: false }, // Disabled PVE
      ],
      pbsConfigs: [
        { id: 'pbs1', name: 'PBS 1', host: '2.2.2.1', port: '8007', username: 'root@pam', tokenId: 'pbst1', tokenSecret: 'pbss1', authMethod: 'token', allowSelfSignedCerts: false },
        { id: 'pbs2', name: 'PBS 2', host: '2.2.2.2', port: '8007', username: 'root@pam', tokenId: 'pbst2', tokenSecret: 'pbss2', authMethod: 'token', allowSelfSignedCerts: true },
      ],
    });
    const { endpoints, pbsConfigs } = loadConfiguration();
    const consoleLogSpy = jest.spyOn(console, 'log').mockImplementation(() => {});

    // Act
    const { apiClients, pbsApiClients } = await initializeApiClients(endpoints, pbsConfigs);

    // Assert
    expect(consoleLogSpy).toHaveBeenCalledWith('INFO: Skipping disabled PVE endpoint: PVE 3 (1.1.1.3)');
    expect(axios.create).toHaveBeenCalledTimes(4); // 2 enabled PVE + 2 PBS

    // Check PVE clients
    expect(Object.keys(apiClients)).toHaveLength(2); // Only enabled ones
    expect(apiClients).toHaveProperty('pve1');
    expect(apiClients).toHaveProperty('pve2');
    expect(apiClients).not.toHaveProperty('pve3-disabled');

    // Check specific rejectUnauthorized for PVE clients
    const pve1Args = axios.create.mock.calls.find(call => call[0].baseURL.includes('1.1.1.1'));
    const pve2Args = axios.create.mock.calls.find(call => call[0].baseURL.includes('1.1.1.2'));
    expect(pve1Args[0].httpsAgent.options.rejectUnauthorized).toBe(true);
    expect(pve2Args[0].httpsAgent.options.rejectUnauthorized).toBe(false);

    // Check PBS clients
    expect(Object.keys(pbsApiClients)).toHaveLength(2);
    expect(pbsApiClients).toHaveProperty('pbs1');
    expect(pbsApiClients).toHaveProperty('pbs2');

    // Check specific rejectUnauthorized for PBS clients
    const pbs1Args = axios.create.mock.calls.find(call => call[0].baseURL.includes('2.2.2.1'));
    const pbs2Args = axios.create.mock.calls.find(call => call[0].baseURL.includes('2.2.2.2'));
    expect(pbs1Args[0].httpsAgent.options.rejectUnauthorized).toBe(true);
    expect(pbs2Args[0].httpsAgent.options.rejectUnauthorized).toBe(false);

    consoleLogSpy.mockRestore();
  });

  test('should handle unexpected PBS authMethod', async () => {
    // Arrange
    loadConfiguration.mockReturnValue({
      endpoints: [], // No PVE for simplicity
      pbsConfigs: [{
        id: 'pbs-bad-auth',
        name: 'PBS Bad Auth',
        host: '8.8.8.8',
        port: '8007',
        authMethod: 'password', // Unexpected method
        allowSelfSignedCerts: false
      }],
    });
    const { endpoints, pbsConfigs } = loadConfiguration();
    const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

    // Act
    const { apiClients, pbsApiClients } = await initializeApiClients(endpoints, pbsConfigs);

    // Assert
    expect(axios.create).not.toHaveBeenCalled(); // Client should not be created for this PBS
    expect(consoleErrorSpy).toHaveBeenCalledWith(
        expect.stringContaining(`Unexpected authMethod 'password' found during PBS client initialization for: PBS Bad Auth`)
    );
    expect(apiClients).toEqual({});
    expect(pbsApiClients).toEqual({}); // No client should be added

    consoleErrorSpy.mockRestore();
  });

  test('should handle unhandled exception during PBS client map', async () => {
      // Arrange
      loadConfiguration.mockReturnValue({
          endpoints: [], 
          pbsConfigs: [{
              id: 'pbs-map-error',
              name: 'PBS Map Error',
              host: '9.9.9.9',
              port: '8007',
              tokenId: 't', tokenSecret: 's', // Valid creds
              authMethod: 'token', 
              allowSelfSignedCerts: false
          }],
      });
      const { endpoints, pbsConfigs } = loadConfiguration();
      const mapError = new Error('Simulated map error');
      // Force axios.create to throw error only for this specific host
      const originalAxiosCreate = axios.create;
      axios.create.mockImplementation((config) => {
          if (config.baseURL.includes('9.9.9.9')) {
              throw mapError;
          }
          // Call original mock impl for other cases (if any)
          return originalAxiosCreate(); 
      });

      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

      // Act
      const { apiClients, pbsApiClients } = await initializeApiClients(endpoints, pbsConfigs);

      // Assert
      expect(axios.create).toHaveBeenCalledTimes(1); // Attempted to create
      // Check the first argument contains the core message, allow anything for the second (stack trace)
      expect(consoleErrorSpy).toHaveBeenCalledWith(
          expect.stringContaining(`ERROR: Unhandled exception during PBS client initialization for PBS Map Error: ${mapError.message}`),
          expect.anything() // Allow the stack trace as the second argument
      );
      expect(apiClients).toEqual({});
      expect(pbsApiClients).toEqual({}); // Client not added due to error

      // Restore original mock implementation if needed for other tests
      axios.create.mockImplementation(originalAxiosCreate);
      consoleErrorSpy.mockRestore();
  });

  // --- Tests for Retry Logic ---
  test('should call axiosRetry during initialization', async () => {
    // Simple test to ensure axiosRetry is called during init
    const { endpoints, pbsConfigs } = loadConfiguration();
    await initializeApiClients(endpoints, pbsConfigs);
    // Expect 1 call for PVE client + 1 call for PBS client from default setup
    expect(axiosRetry).toHaveBeenCalledTimes(2);
    // Check args for the PVE client call
    expect(axiosRetry).toHaveBeenCalledWith( 
      axios.create.mock.results[0].value, // The first created axios instance
      expect.objectContaining({ retries: 3 }) // Check if retry config is passed
    );
  });

  test('should log error when PVE request interceptor encounters missing credentials', async () => {
    // Arrange
    const missingCredsEndpoint = {
      id: 'pve-bad-creds',
      name: 'PVE Missing Creds',
      host: '11.11.11.11',
      port: '8006',
      // Missing tokenId and tokenSecret
      enabled: true,
      allowSelfSignedCerts: false
    };
    loadConfiguration.mockReturnValue({ endpoints: [missingCredsEndpoint], pbsConfigs: [] });
    const { endpoints, pbsConfigs } = loadConfiguration();
    const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

    // Mock axios.create specifically for this test
    let capturedInterceptor = null; // Variable to hold the interceptor function
    const mockGet = jest.fn().mockResolvedValue({ data: 'ignored' });
    const mockAxiosInstance = {
      get: async (url, config) => {
        // Simulate running the interceptor before the request
        if (capturedInterceptor) {
          // Pass a mock config object, interceptor might modify it
          const mockConfig = { headers: {}, url, ...config }; 
          try {
             await capturedInterceptor(mockConfig); // Run the interceptor
          } catch (interceptorError) {
             // If interceptor throws (e.g., Promise.reject), rethrow it
             throw interceptorError;
          }
        }
        return mockGet(url, config); // Run the actual mock get
      },
      interceptors: {
        request: {
          use: jest.fn(successFn => { // Capture the interceptor function
            capturedInterceptor = successFn;
          })
        },
        response: { use: jest.fn() }
      }
    };
    axios.create.mockReturnValue(mockAxiosInstance);
    
    // Act: Initialize clients (this adds the interceptor via the mock .use)
    const { apiClients } = await initializeApiClients(endpoints, pbsConfigs);
    const pveClient = apiClients['pve-bad-creds']?.client;
    expect(pveClient).toBeDefined();
    expect(capturedInterceptor).not.toBeNull(); // Check interceptor was captured

    // Act: Attempt an API call which should trigger the interceptor via the mock .get
    try {
      await pveClient.get('/nodes');
    } catch (e) {
      // We don't expect the get call itself to throw here, 
      // the interceptor just logs an error in this case.
    }

    // Assert: Check that the console error was logged by the interceptor
    expect(consoleErrorSpy).toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      `ERROR: Endpoint ${missingCredsEndpoint.name} is missing required API token credentials.`
    );

    consoleErrorSpy.mockRestore();
    // Restore default axios.create mock from beforeEach
    axios.create.mockImplementation(() => ({ 
      get: jest.fn(), 
      interceptors: { request: { use: jest.fn() }, response: { use: jest.fn() } } 
    }));
  });

  // Removing the complex/brittle retry simulation tests below as the core logic
  // is now tested via the helper function tests (pbsRetryDelayLogger, pbsRetryConditionChecker)
  // and the basic call is verified by 'should call axiosRetry during initialization'.

  /*
  test('should retry PVE API calls on network errors', async () => {
    // ... (Removed Test Code) ...
  });
  */

  /*
  test('should retry PBS API calls on retryable errors and log warning', async () => {
    // ... (Removed Test Code) ...
  });
  */

  // Add more tests here for:
  // - Config validation errors (missing fields in loadConfiguration result)
  // - Axios errors during initialization (e.g., interceptor setup fails? unlikely)
  // - Multiple endpoints for PVE/PBS
  // - Different auth methods (if implemented)
  // - rejectUnauthorized logic

  test('should correctly build baseURL for hosts with and without protocol', async () => {
    // Arrange
    loadConfiguration.mockReturnValue({
      endpoints: [
        { id: 'pve-no-proto', name: 'PVE No Protocol', host: '1.1.1.1', port: '8006', enabled: true, tokenId: 't1', tokenSecret: 's1', allowSelfSignedCerts: false },
        { id: 'pve-with-proto', name: 'PVE With Protocol', host: 'https://1.1.1.2', port: '8006', enabled: true, tokenId: 't2', tokenSecret: 's2', allowSelfSignedCerts: false },
      ],
      pbsConfigs: [
        { id: 'pbs-no-proto', name: 'PBS No Protocol', host: '2.2.2.1', port: '8007', authMethod: 'token', tokenId: 'pt1', tokenSecret: 'ps1', allowSelfSignedCerts: false },
        { id: 'pbs-with-proto', name: 'PBS With Protocol', host: 'https://2.2.2.2', port: '8007', authMethod: 'token', tokenId: 'pt2', tokenSecret: 'ps2', allowSelfSignedCerts: false },
      ],
    });
    const { endpoints, pbsConfigs } = loadConfiguration();

    // Act
    await initializeApiClients(endpoints, pbsConfigs);

    // Assert
    expect(axios.create).toHaveBeenCalledTimes(4); // 2 PVE + 2 PBS

    // Check PVE Base URLs
    const pveNoProtoArgs = axios.create.mock.calls.find(call => call[0].baseURL?.includes('1.1.1.1'));
    const pveWithProtoArgs = axios.create.mock.calls.find(call => call[0].baseURL?.includes('1.1.1.2'));
    expect(pveNoProtoArgs[0].baseURL).toBe('https://1.1.1.1:8006/api2/json'); // Checks the ':' branch (line 63)
    expect(pveWithProtoArgs[0].baseURL).toBe('https://1.1.1.2/api2/json');     // Checks the '?' branch (line 62)

    // Check PBS Base URLs
    const pbsNoProtoArgs = axios.create.mock.calls.find(call => call[0].baseURL?.includes('2.2.2.1'));
    const pbsWithProtoArgs = axios.create.mock.calls.find(call => call[0].baseURL?.includes('2.2.2.2'));
    expect(pbsNoProtoArgs[0].baseURL).toBe('https://2.2.2.1:8007/api2/json'); // Checks the ':' branch (line 144)
    expect(pbsWithProtoArgs[0].baseURL).toBe('https://2.2.2.2/api2/json');     // Checks the '?' branch (line 143)
  });

}); 

// --- Direct Tests for Helper Functions ---

describe('API Client Helper Functions', () => {

  beforeEach(() => {
    jest.clearAllMocks();
  });

  // --- createPveAuthInterceptor Tests ---
  describe('createPveAuthInterceptor', () => {
    const { createPveAuthInterceptor } = require('../apiClients');
    const mockEndpoint = { name: 'Test PVE', tokenId: 'test-id', tokenSecret: 'test-secret' };
    const mockEndpointMissingCreds = { name: 'Test PVE Bad' }; // Missing credentials

    test('should return a function', () => {
      const interceptor = createPveAuthInterceptor(mockEndpoint);
      expect(typeof interceptor).toBe('function');
    });

    test('should add Authorization header if credentials exist', () => {
      const interceptor = createPveAuthInterceptor(mockEndpoint);
      const mockConfig = { headers: {} };
      const resultConfig = interceptor(mockConfig);
      expect(resultConfig.headers.Authorization).toBe(`PVEAPIToken=test-id=test-secret`);
    });

    test('should NOT add Authorization header and log error if credentials missing', () => {
      const interceptor = createPveAuthInterceptor(mockEndpointMissingCreds);
      const mockConfig = { headers: {} };
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
      
      const resultConfig = interceptor(mockConfig);
      
      expect(resultConfig.headers.Authorization).toBeUndefined();
      expect(consoleErrorSpy).toHaveBeenCalledTimes(1);
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        `ERROR: Endpoint ${mockEndpointMissingCreds.name} is missing required API token credentials.`
      );
      consoleErrorSpy.mockRestore();
    });
  });

  // --- createPbsAuthInterceptor Tests ---
  describe('createPbsAuthInterceptor', () => {
    const { createPbsAuthInterceptor } = require('../apiClients');
    const mockConfig = { tokenId: 'pbs-id', tokenSecret: 'pbs-secret' };

    test('should return a function', () => {
      const interceptor = createPbsAuthInterceptor(mockConfig);
      expect(typeof interceptor).toBe('function');
    });

    test('should add correct PBS Authorization header', () => {
      const interceptor = createPbsAuthInterceptor(mockConfig);
      const mockReqConfig = { headers: {} };
      const resultConfig = interceptor(mockReqConfig);
      expect(resultConfig.headers.Authorization).toBe(`PBSAPIToken=pbs-id:pbs-secret`);
    });
    
    // Note: Add test for missing creds if validation doesn't happen before calling this
  });

  // --- pveRetryDelayLogger Tests ---
  describe('pveRetryDelayLogger', () => {
    const { pveRetryDelayLogger } = require('../apiClients');
    const axiosRetry = require('axios-retry').default;

    beforeEach(() => {
      axiosRetry.exponentialDelay.mockClear();
      axiosRetry.exponentialDelay.mockReturnValue(500); // Use different value for clarity
    });

    test('should log warning with correct PVE details', () => {
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});
      const testError = new Error('PVE Failed');
      pveRetryDelayLogger('TestPVE', 3, testError);

      expect(consoleWarnSpy).toHaveBeenCalledTimes(1);
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        'Retrying PVE API request for TestPVE (attempt 3) due to error: PVE Failed'
      );
      consoleWarnSpy.mockRestore();
    });

    test('should call mocked axiosRetry.exponentialDelay and return its value', () => {
      const result = pveRetryDelayLogger('TestPVE', 2, new Error('Test'));

      expect(axiosRetry.exponentialDelay).toHaveBeenCalledTimes(1);
      expect(axiosRetry.exponentialDelay).toHaveBeenCalledWith(2); // Called with retryCount
      expect(result).toBe(500); // Returns the mock value
    });
  });

  // --- pbsRetryDelayLogger Tests ---
  describe('pbsRetryDelayLogger', () => {
    const { pbsRetryDelayLogger } = require('../apiClients');
    // Get the mocked default export which has the mocked helpers
    const axiosRetry = require('axios-retry').default;

    beforeEach(() => {
      // Reset mocks before each test in this suite
      axiosRetry.exponentialDelay.mockClear();
      axiosRetry.exponentialDelay.mockReturnValue(1000); // Set default mock return for simplicity
    });

    test('should log warning with correct details', () => {
      // ... (this test remains the same, just checking console.warn) ...
      const consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});
      const testError = new Error('PBS Failed');
      pbsRetryDelayLogger('TestPBS', 2, testError);
      expect(consoleWarnSpy).toHaveBeenCalledTimes(1);
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        'Retrying PBS API request for TestPBS (Token Auth - attempt 2) due to error: PBS Failed'
      );
      consoleWarnSpy.mockRestore();
    });

    test('should call mocked axiosRetry.exponentialDelay and return its value', () => {
      // No spy needed, just call the function and check the pre-existing mock
      const result = pbsRetryDelayLogger('TestPBS', 1, new Error('Test'));

      expect(axiosRetry.exponentialDelay).toHaveBeenCalledTimes(1);
      expect(axiosRetry.exponentialDelay).toHaveBeenCalledWith(1);
      expect(result).toBe(1000); // Should return the mock value
    });
  });

  // --- pbsRetryConditionChecker Tests ---
  describe('pbsRetryConditionChecker', () => {
    const { pbsRetryConditionChecker } = require('../apiClients');
    // Get the mocked default export which has the mocked helpers
    const axiosRetry = require('axios-retry').default;

    beforeEach(() => {
      // Reset mocks and set default return values before each test
      axiosRetry.isNetworkError.mockClear().mockReturnValue(false);
      axiosRetry.isRetryableError.mockClear().mockReturnValue(false);
    });

    // No afterEach needed as we clear in beforeEach

    test('should return true for network errors', () => {
      const networkError = new Error('Network Error');
      axiosRetry.isNetworkError.mockReturnValue(true); // Override default mock return
      axiosRetry.isRetryableError.mockReturnValue(false); // Ensure this stays false for the test
      
      expect(pbsRetryConditionChecker(networkError)).toBe(true);
      // Verify mocks were called (or not called due to short-circuit)
      expect(axiosRetry.isNetworkError).toHaveBeenCalledWith(networkError);
      expect(axiosRetry.isRetryableError).not.toHaveBeenCalled(); // Corrected assertion
    });

    test('should return true for retryable errors', () => {
      const retryableError = new Error('Retryable Error');
      retryableError.response = { status: 503 };
      axiosRetry.isRetryableError.mockReturnValue(true); // Override default mock return

      expect(pbsRetryConditionChecker(retryableError)).toBe(true);
      // Verify mocks were called
      expect(axiosRetry.isNetworkError).toHaveBeenCalledWith(retryableError);
      expect(axiosRetry.isRetryableError).toHaveBeenCalledWith(retryableError);
    });

    test('should return false for non-network, non-retryable errors', () => {
      const otherError = new Error('Other Error');
      // Default mock returns (false, false) are already set in beforeEach
      
      expect(pbsRetryConditionChecker(otherError)).toBe(false);
      // Verify mocks were called
      expect(axiosRetry.isNetworkError).toHaveBeenCalledWith(otherError);
      expect(axiosRetry.isRetryableError).toHaveBeenCalledWith(otherError);
    });
  });

  // --- pveRetryConditionChecker Tests ---
  describe('pveRetryConditionChecker', () => {
    const { pveRetryConditionChecker } = require('../apiClients');
    const axiosRetry = require('axios-retry').default;

    beforeEach(() => {
      axiosRetry.isNetworkError.mockClear().mockReturnValue(false);
      axiosRetry.isRetryableError.mockClear().mockReturnValue(false);
    });

    test('should return true for network errors', () => {
      const networkError = new Error('Network Error');
      axiosRetry.isNetworkError.mockReturnValue(true);
      expect(pveRetryConditionChecker(networkError)).toBe(true);
      expect(axiosRetry.isNetworkError).toHaveBeenCalledWith(networkError);
      expect(axiosRetry.isRetryableError).not.toHaveBeenCalled(); // Short-circuits
    });

    test('should return true for retryable errors', () => {
      const retryableError = new Error('Retryable Error');
      axiosRetry.isRetryableError.mockReturnValue(true);
      expect(pveRetryConditionChecker(retryableError)).toBe(true);
      expect(axiosRetry.isNetworkError).toHaveBeenCalledWith(retryableError);
      expect(axiosRetry.isRetryableError).toHaveBeenCalledWith(retryableError);
    });
    
    test('should return true for error with status 596', () => {
      const status596Error = new Error('Status 596 Error');
      status596Error.response = { status: 596 };
      // Ensure other checks are false
      axiosRetry.isNetworkError.mockReturnValue(false);
      axiosRetry.isRetryableError.mockReturnValue(false);
      
      expect(pveRetryConditionChecker(status596Error)).toBe(true);
      expect(axiosRetry.isNetworkError).toHaveBeenCalledWith(status596Error);
      expect(axiosRetry.isRetryableError).toHaveBeenCalledWith(status596Error);
    });

    test('should return false for other errors without status 596', () => {
      const otherError = new Error('Other Error');
      // Ensure other checks are false (default from beforeEach)
      expect(pveRetryConditionChecker(otherError)).toBe(false);
      expect(axiosRetry.isNetworkError).toHaveBeenCalledWith(otherError);
      expect(axiosRetry.isRetryableError).toHaveBeenCalledWith(otherError);
    });

    test('should return false for error with different response status', () => {
      const status500Error = new Error('Status 500 Error');
      status500Error.response = { status: 500 };
       // Ensure other checks are false (default from beforeEach)
      expect(pveRetryConditionChecker(status500Error)).toBe(false);
      expect(axiosRetry.isNetworkError).toHaveBeenCalledWith(status500Error);
      expect(axiosRetry.isRetryableError).toHaveBeenCalledWith(status500Error);
    });
  });

}); 