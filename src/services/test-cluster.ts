import { createLogger } from '../utils/logger';
import { MockClient } from '../api/mock-client';
import { NodeConfig } from '../types';

// Make sure we're using mock mode
process.env.USE_MOCK_DATA = 'true';
process.env.MOCK_DATA_ENABLED = 'true';
process.env.MOCK_CLUSTER_ENABLED = 'true';
// Cluster resources is now enabled by default, no need to set it explicitly
// process.env.PROXMOX_USE_CLUSTER_RESOURCES = 'true';

// Create a logger
const logger = createLogger('TestClusterResources');

// Get cluster resources from mock client
async function testClusterResources() {
  logger.info('Testing cluster resources feature with MockClient');
  logger.info(`PROXMOX_USE_CLUSTER_RESOURCES env: ${process.env.PROXMOX_USE_CLUSTER_RESOURCES || 'true (default)'}`);
  
  // Create a mock client
  const nodeConfig: NodeConfig = {
    id: 'test-1',
    name: 'test-node',
    host: 'http://localhost:7656',
    tokenId: 'test-token',
    tokenSecret: 'test-secret'
  };
  
  const mockClient = new MockClient(nodeConfig);
  
  try {
    // Connect to the mock server (start one if needed)
    logger.info('Starting mock server...');
    const mockServerScript = require('../../scripts/start-mock-server');
    
    // Connect to the mock server
    logger.info('Connecting to mock server...');
    await mockClient.connect();
    
    // Get cluster resources by calling the method directly
    logger.info('Calling getClusterResources method directly:');
    const results = await mockClient.getClusterResources();
    
    // Log the results
    logger.info(`Got ${results.vms.length} VMs and ${results.containers.length} containers from cluster resources endpoint`);
    
    // Print a few example VMs
    if (results.vms.length > 0) {
      logger.info('Example VMs from cluster resources:');
      for (let i = 0; i < Math.min(3, results.vms.length); i++) {
        const vm = results.vms[i];
        logger.info(`VM: ${vm.id}, Name: ${vm.name}, Node: ${vm.node}, Status: ${vm.status}`);
      }
    }
    
    // Print a few example containers
    if (results.containers.length > 0) {
      logger.info('Example containers from cluster resources:');
      for (let i = 0; i < Math.min(3, results.containers.length); i++) {
        const container = results.containers[i];
        logger.info(`Container: ${container.id}, Name: ${container.name}, Node: ${container.node}, Status: ${container.status}`);
      }
    }
  } catch (error) {
    logger.error('Error testing cluster resources', { error });
  } finally {
    // Disconnect when done
    mockClient.disconnect();
    logger.info('Test complete');
    
    // Exit the process
    setTimeout(() => process.exit(0), 1000);
  }
}

// Run the test
testClusterResources(); 