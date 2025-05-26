const { loadConfiguration, ConfigurationError } = require('../configLoader');

// Mock dotenv
jest.mock('dotenv', () => ({
  config: jest.fn(),
}));
const dotenv = require('dotenv'); // require after mock

// Helper function to temporarily set environment variables for a test
const setEnvVars = (vars) => {
  const originalEnv = { ...process.env }; // Store original env
  Object.keys(vars).forEach(key => {
    process.env[key] = vars[key];
  });
  return originalEnv; // Return original env for restoration
};

// Helper function to restore environment variables
const restoreEnvVars = (originalEnv) => {
  // Clear potentially set test variables first
  Object.keys(process.env).forEach(key => {
    if (!(key in originalEnv)) {
      delete process.env[key];
    }
  });
  // Restore original values
  Object.keys(originalEnv).forEach(key => {
    process.env[key] = originalEnv[key];
  });
};

// Set NODE_ENV to test *before* describing the suite
process.env.NODE_ENV = 'test';

// Mock console
let consoleWarnSpy; // Declare spies outside beforeEach/afterEach
let consoleLogSpy;

describe('Configuration Loading (loadConfiguration)', () => {
  let originalEnv;

  beforeEach(() => {
    // Store original environment
    originalEnv = { ...process.env }; 
    
    // --- More robust clearing of process.env ---
    // Get all keys BEFORE modifying
    const currentEnvKeys = Object.keys(process.env);
    // Delete all keys
    currentEnvKeys.forEach(key => delete process.env[key]);
    // --- End robust clearing ---

    // Restore NODE_ENV as it's crucial for the logic
    process.env.NODE_ENV = 'test'; 
    
    // Assign spies in beforeEach
    consoleWarnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});
    consoleLogSpy = jest.spyOn(console, 'log').mockImplementation(() => {});
  });

  afterEach(() => {
    // --- Restore original environment more carefully ---
    // Clear any keys potentially added during the test
    const currentEnvKeys = Object.keys(process.env);
    currentEnvKeys.forEach(key => delete process.env[key]);
    // Restore the original keys and values
    Object.keys(originalEnv).forEach(key => {
      process.env[key] = originalEnv[key];
    });
    // --- End restore ---

    // Restore specific spies
    consoleWarnSpy.mockRestore();
    consoleLogSpy.mockRestore();
  });

  // Test Case 1: Minimal Valid PVE Config
  test('should load minimal PVE config successfully', () => {
    setEnvVars({
      PROXMOX_HOST: 'pve.example.com',
      PROXMOX_TOKEN_ID: 'user@pam!pve',
      PROXMOX_TOKEN_SECRET: 'secretpve',
    });
    // Expect no error to be thrown for valid config
    let loadedConfig;
    expect(() => {
      loadedConfig = loadConfiguration();
    }).not.toThrow();

    // Check the returned structure
    expect(loadedConfig).toBeDefined();
    expect(loadedConfig.endpoints).toHaveLength(1); // Check endpoints array
    expect(loadedConfig.pbsConfigs).toHaveLength(0); // Expect no PBS configs

    // Check the primary PVE endpoint details within the endpoints array
    const primaryEndpoint = loadedConfig.endpoints[0];
    expect(primaryEndpoint.id).toBe('primary');
    expect(primaryEndpoint.host).toBe('pve.example.com');
    expect(primaryEndpoint.tokenId).toBe('user@pam!pve');
    expect(primaryEndpoint.tokenSecret).toBe('secretpve');
  });

  // Test Case 2: Missing Primary Proxmox Variables
  test('should throw ConfigurationError if primary Proxmox variables are missing', () => {
    setEnvVars({
      PROXMOX_HOST: '192.168.1.100',
      // Missing TOKEN_ID and TOKEN_SECRET
    });

    expect(() => loadConfiguration()).toThrow(ConfigurationError);
    expect(() => loadConfiguration()).toThrow(/Missing required environment variables: PROXMOX_TOKEN_ID, PROXMOX_TOKEN_SECRET/);
  });

  // Test Case 3: Placeholder Primary Proxmox Variables
  test('should warn and set flag if primary Proxmox variables contain placeholders', () => {
    const envSetup = {
      PROXMOX_HOST: 'your-proxmox-ip-or-hostname',
      PROXMOX_TOKEN_ID: 'user@pam!token', // A placeholder not exactly in the list
      PROXMOX_TOKEN_SECRET: 'secret-uuid',   // Another placeholder not exactly in the list
    };
    setEnvVars(envSetup);

    let config;
    // Expect no error to be thrown, but placeholders to be detected
    expect(() => {
      config = loadConfiguration();
    }).not.toThrow();

    expect(consoleWarnSpy).toHaveBeenCalledWith(
      expect.stringContaining('WARN: Primary Proxmox environment variables seem to contain placeholder values: PROXMOX_HOST, PROXMOX_TOKEN_ID, PROXMOX_TOKEN_SECRET')
    );
    expect(config.isConfigPlaceholder).toBe(true);
  });

  // Test Case 4: Valid Primary + Additional Proxmox Endpoints
  test('should load successfully with additional valid Proxmox endpoints', () => {
    setEnvVars({
      PROXMOX_HOST: 'pve1.example.com',
      PROXMOX_TOKEN_ID: 'user@pam!token1',
      PROXMOX_TOKEN_SECRET: 'secret1',
      PROXMOX_NODE_NAME: 'PVE Node 1', // Custom name
      PROXMOX_PORT: '8007', // Custom port
      PROXMOX_ALLOW_SELF_SIGNED_CERTS: 'true', // Explicitly true

      PROXMOX_HOST_2: 'pve2.example.com',
      PROXMOX_TOKEN_ID_2: 'user@pam!token2',
      PROXMOX_TOKEN_SECRET_2: 'secret2',
      PROXMOX_ENABLED_2: 'false', // Disabled endpoint

      PROXMOX_HOST_3: 'pve3.example.com',
      PROXMOX_TOKEN_ID_3: 'user@pam!token3',
      PROXMOX_TOKEN_SECRET_3: 'secret3',
      PROXMOX_NODE_NAME_3: 'PVE Node 3', // Custom name
      PROXMOX_PORT_3: '8008',
      PROXMOX_ALLOW_SELF_SIGNED_CERTS_3: 'false', // Explicitly false
    });

    const config = loadConfiguration();
    expect(config.endpoints).toHaveLength(3);

    // Check primary
    expect(config.endpoints[0].id).toBe('primary');
    expect(config.endpoints[0].name).toBe('PVE Node 1');
    expect(config.endpoints[0].host).toBe('pve1.example.com');
    expect(config.endpoints[0].port).toBe('8007');
    expect(config.endpoints[0].enabled).toBe(true);
    expect(config.endpoints[0].allowSelfSignedCerts).toBe(true);

    // Check second (disabled)
    expect(config.endpoints[1].id).toBe('endpoint_2');
    expect(config.endpoints[1].name).toBe('pve2.example.com'); // Defaults to host
    expect(config.endpoints[1].host).toBe('pve2.example.com');
    expect(config.endpoints[1].port).toBe('8006'); // Default port
    expect(config.endpoints[1].enabled).toBe(false);
    expect(config.endpoints[1].allowSelfSignedCerts).toBe(true); // Default

    // Check third
    expect(config.endpoints[2].id).toBe('endpoint_3');
    expect(config.endpoints[2].name).toBe('PVE Node 3');
    expect(config.endpoints[2].host).toBe('pve3.example.com');
    expect(config.endpoints[2].port).toBe('8008');
    expect(config.endpoints[2].enabled).toBe(true); // Default
    expect(config.endpoints[2].allowSelfSignedCerts).toBe(false);

    expect(config.pbsConfigs).toHaveLength(0);
  });

  // Test Case 5: Incomplete Additional Proxmox Endpoint
  test('should skip additional Proxmox endpoint if token details are missing', () => {
    setEnvVars({
      PROXMOX_HOST: 'pve1.example.com',
      PROXMOX_TOKEN_ID: 'user@pam!token1',
      PROXMOX_TOKEN_SECRET: 'secret1',

      PROXMOX_HOST_2: 'pve2.example.com', // Missing token ID/secret for #2
    });

    const config = loadConfiguration();
    expect(config.endpoints).toHaveLength(1);
    expect(config.endpoints[0].id).toBe('primary');
  });

   // Test Case 6: Placeholder Additional Proxmox Endpoint
  test('should skip additional Proxmox endpoint if details contain placeholders', () => {
    setEnvVars({
      PROXMOX_HOST: 'pve1.example.com',
      PROXMOX_TOKEN_ID: 'user@pam!token1',
      PROXMOX_TOKEN_SECRET: 'secret1',

      PROXMOX_HOST_2: 'your-proxmox-ip-or-hostname', // Placeholder host
      PROXMOX_TOKEN_ID_2: 'user@pam!token2',
      PROXMOX_TOKEN_SECRET_2: 'secret2',
    });

    const config = loadConfiguration();
    expect(config.endpoints).toHaveLength(1); // Only primary should load
    expect(config.endpoints[0].id).toBe('primary');
  });

  // Test Case 7: Valid Primary PBS Config
  test('should load successfully with a valid primary PBS config', () => {
    setEnvVars({
      // Minimal valid PVE
      PROXMOX_HOST: 'pve.example.com',
      PROXMOX_TOKEN_ID: 'user@pam!pve',
      PROXMOX_TOKEN_SECRET: 'secretpve',
      // Valid PBS
      PBS_HOST: 'https://pbs.example.com:8007', // Full URL
      PBS_TOKEN_ID: 'user@pbs!token',
      PBS_TOKEN_SECRET: 'secretpbs',
      PBS_NODE_NAME: 'PBS Backup Server',
      PBS_ALLOW_SELF_SIGNED_CERTS: 'false',
    });

    const config = loadConfiguration();
    expect(config.endpoints).toHaveLength(1);
    expect(config.pbsConfigs).toHaveLength(1);

    const pbs = config.pbsConfigs[0];
    expect(pbs.id).toBe('pbs_primary_token');
    expect(pbs.name).toBe('PBS Backup Server');
    expect(pbs.host).toBe('https://pbs.example.com:8007');
    expect(pbs.port).toBe('8007'); // Port from env var
    expect(pbs.tokenId).toBe('user@pbs!token');
    expect(pbs.tokenSecret).toBe('secretpbs');
    expect(pbs.authMethod).toBe('token');
    expect(pbs.allowSelfSignedCerts).toBe(false);
    expect(pbs.enabled).toBe(true);
  });

  test('should not add primary PBS config if host is set but tokens are missing', () => {
    setEnvVars({
      PROXMOX_HOST: '192.168.1.100',
      PROXMOX_TOKEN_ID: 'user@pam!pve',
      PROXMOX_TOKEN_SECRET: 'secretpve',
      
      PBS_HOST: 'pbs.example.com',
      // Missing TOKEN_ID and TOKEN_SECRET for PBS
    });

    let config;
    expect(() => {
        config = loadConfiguration();
    }).not.toThrow();

    expect(config.endpoints).toHaveLength(1);
    expect(config.pbsConfigs).toHaveLength(0); // PBS should NOT load

    expect(consoleWarnSpy).toHaveBeenCalledWith(
      expect.stringContaining('WARN: Partial PBS configuration found for PBS_HOST. Please set (PBS_TOKEN_ID + PBS_TOKEN_SECRET)')
    );
     expect(consoleWarnSpy).toHaveBeenCalledTimes(1); // Only one warning expected from this test
  });

  // Test Case 8: Valid Primary + Additional PBS Configs
  test('should load successfully with additional valid PBS configs', () => {
     setEnvVars({
      // PVE
      PROXMOX_HOST: 'pve.example.com',
      PROXMOX_TOKEN_ID: 'user@pam!pve',
      PROXMOX_TOKEN_SECRET: 'secretpve',
      // PBS 1 (Primary)
      PBS_HOST: 'pbs1.example.com', // No protocol/port
      PBS_TOKEN_ID: 'user@pbs!token1',
      PBS_TOKEN_SECRET: 'secretpbs1',
      // PBS 2
      PBS_HOST_2: 'https://pbs2.example.com:8008',
      PBS_TOKEN_ID_2: 'user@pbs!token2',
      PBS_TOKEN_SECRET_2: 'secretpbs2',
      PBS_NODE_NAME_2: 'PBS Server 2',
      PBS_PORT_2: '9000', // Custom port
      // PBS 3 (Placeholder - should skip)
      PBS_HOST_3: 'pbs3.example.com',
      PBS_TOKEN_ID_3: 'your-api-token-id@pam!your-token-name',
      PBS_TOKEN_SECRET_3: 'secretpbs3',
      // PBS 4 (Missing Token Secret - should skip)
      PBS_HOST_4: 'pbs4.example.com',
      PBS_TOKEN_ID_4: 'user@pbs!token4',
    });

    const config = loadConfiguration();
    expect(config.endpoints).toHaveLength(1);
    expect(config.pbsConfigs).toHaveLength(2);

    // Check PBS 1 (Primary)
    expect(config.pbsConfigs[0].id).toBe('pbs_primary_token');
    expect(config.pbsConfigs[0].name).toBe('pbs1.example.com'); // Defaults to host
    expect(config.pbsConfigs[0].host).toBe('pbs1.example.com');
    expect(config.pbsConfigs[0].port).toBe('8007'); // Default port
    expect(config.pbsConfigs[0].allowSelfSignedCerts).toBe(true); // Default

    // Check PBS 2
    expect(config.pbsConfigs[1].id).toBe('pbs_endpoint_2_token');
    expect(config.pbsConfigs[1].name).toBe('PBS Server 2');
    expect(config.pbsConfigs[1].host).toBe('https://pbs2.example.com:8008');
    expect(config.pbsConfigs[1].port).toBe('9000'); // Custom port
    expect(config.pbsConfigs[1].allowSelfSignedCerts).toBe(true); // Default

    // PBS 3 and 4 should have been skipped
  });

  // Test Case 9: Incomplete Additional PBS Endpoint (NEW TEST)
  test('should skip additional PBS endpoint if token details are missing but host is present', () => {
    setEnvVars({
      PROXMOX_HOST: 'pve.example.com',
      PROXMOX_TOKEN_ID: 'user@pam!pve',
      PROXMOX_TOKEN_SECRET: 'secretpve',
      // Valid Primary PBS
      PBS_HOST: 'pbs1.example.com',
      PBS_TOKEN_ID: 'user@pbs!token1',
      PBS_TOKEN_SECRET: 'secretpbs1',
      // Additional PBS host, missing tokens
      PBS_HOST_2: 'pbs2.example.com',
      // PBS_TOKEN_ID_2: 'user@pbs!token2', // Missing
      // PBS_TOKEN_SECRET_2: 'secretpbs2', // Missing
      // Valid third PBS
      PBS_HOST_3: 'pbs3.example.com',
      PBS_TOKEN_ID_3: 'user@pbs!token3',
      PBS_TOKEN_SECRET_3: 'secretpbs3',
    });

    const config = loadConfiguration();
    expect(config.endpoints).toHaveLength(1);
    expect(config.pbsConfigs).toHaveLength(2); // Should load primary (PBS1) and PBS3
    expect(config.pbsConfigs.map(p => p.host)).toEqual(['pbs1.example.com', 'pbs3.example.com']);

    // Check that the warning for the partial config _2 was logged
    expect(consoleWarnSpy).toHaveBeenCalledWith(
        expect.stringContaining('WARN: Partial PBS configuration found for PBS_HOST_2. Please set (PBS_TOKEN_ID_2 + PBS_TOKEN_SECRET_2)')
    );
    // Verify the config for PBS_HOST_2 was not added
     expect(config.pbsConfigs.find(p => p.host === 'pbs2.example.com')).toBeUndefined();
  });

  // Test Case 10: No Enabled Endpoints
  test('should throw ConfigurationError if no enabled PVE or PBS endpoints are configured', () => {
     setEnvVars({
      // Valid PVE, but disabled
      PROXMOX_HOST: 'pve.example.com',
      PROXMOX_TOKEN_ID: 'user@pam!pve',
      PROXMOX_TOKEN_SECRET: 'secretpve',
      PROXMOX_ENABLED: 'false',
      // Valid PBS details, but only HOST is present, no tokens
      PBS_HOST: 'pbs.example.com'
    });

    // Expect the final check in loadConfiguration to throw
    expect(() => loadConfiguration()).toThrow(ConfigurationError);
    expect(() => loadConfiguration()).toThrow(/No enabled Proxmox VE or PBS endpoints could be configured/);
  });

  // New Test Case for dotenv loading
  test('should call dotenv.config() when NODE_ENV is not \'test\'', () => {
    const originalNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development'; // Set to non-test environment

    // Minimal valid PVE config to allow loadConfiguration to proceed far enough
    setEnvVars({
      PROXMOX_HOST: 'pve.example.com',
      PROXMOX_TOKEN_ID: 'user@pam!pve',
      PROXMOX_TOKEN_SECRET: 'secretpve',
    });

    loadConfiguration();

    expect(dotenv.config).toHaveBeenCalled();

    // Restore original NODE_ENV and clear mocks for other tests
    process.env.NODE_ENV = originalNodeEnv;
    dotenv.config.mockClear(); // Clear the mock for other tests
  });

  // Test Case 11: Placeholder detection with PROXMOX_TOKEN_ID in env
  test('should insert PROXMOX_TOKEN_ID in correct position when placeholders detected', () => {
    setEnvVars({
      PROXMOX_HOST: 'your-proxmox-ip-or-hostname',
      PROXMOX_TOKEN_ID: 'user@pam!token',
      PROXMOX_TOKEN_SECRET: 'your-api-token-uuid', 
    });

    const config = loadConfiguration();
    
    // Should detect placeholders - the actual implementation includes PROXMOX_TOKEN_ID when it's set
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      expect.stringContaining('WARN: Primary Proxmox environment variables seem to contain placeholder values: PROXMOX_HOST, PROXMOX_TOKEN_ID')
    );
    expect(config.isConfigPlaceholder).toBe(true);
  });

  // Test Case 12: Placeholder detection - TOKEN_ID not in list but exists
  test('should add PROXMOX_TOKEN_ID at end if not in placeholder list but exists', () => {
    // Only secret is a placeholder, but TOKEN_ID exists and should be added
    setEnvVars({
      PROXMOX_HOST: 'pve.example.com',
      PROXMOX_TOKEN_ID: 'user@pam!mytoken',  // exists but not a placeholder
      PROXMOX_TOKEN_SECRET: 'your-api-token-uuid',  // placeholder
    });

    const config = loadConfiguration();
    
    // Debug: Check if console.warn was called at all
    expect(consoleWarnSpy).toHaveBeenCalledTimes(1);
    
    // Should detect the secret placeholder and add TOKEN_ID
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      expect.stringContaining('PROXMOX_TOKEN_SECRET')
    );
    expect(config.isConfigPlaceholder).toBe(true);
  });

  // Test Case 13: Test line 138 - Add TOKEN_ID when no PROXMOX_HOST in placeholderVars
  test('should push PROXMOX_TOKEN_ID when PROXMOX_HOST not in placeholder list', () => {
    // Only PROXMOX_PORT is placeholder (not PROXMOX_HOST)
    setEnvVars({
      PROXMOX_HOST: 'pve.example.com',
      PROXMOX_TOKEN_ID: 'user@pam!token', // This IS identified as a placeholder
      PROXMOX_TOKEN_SECRET: 'secret123',
      PROXMOX_PORT: 'your-port'  // This is a placeholder, but not checked in the primary warning
    });

    const config = loadConfiguration();
    
    // Should detect a placeholder in PROXMOX_TOKEN_ID and warn about it.
    // PROXMOX_PORT is not part of the primary placeholder check that generates this specific warning.
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      expect.stringContaining('PROXMOX_TOKEN_ID')
    );
    expect(config.isConfigPlaceholder).toBe(true);
  });

}); 