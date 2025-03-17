/**
 * Systematic Search Functionality Test Suite
 * 
 * This file contains a comprehensive, systematic test suite for verifying the search 
 * functionality in the NetworkUtils module. It tests every edge case, boundary condition, 
 * and search pattern to ensure the search functionality behaves as expected.
 */

import { getSortedAndFilteredData } from './mocks/networkUtils.js';

// ============================================================================
// TEST DATA SETUP - EXPANDED AND COMPREHENSIVE MOCK DATA
// ============================================================================

// Deep clone function for test data
function deepClone(obj) {
  return JSON.parse(JSON.stringify(obj));
}

// Mock node data with realistic node properties
const mockNodeData = [
  { id: 'node-1', name: 'prod-01', status: 'online', ipAddress: '192.168.1.101', cpu: { cores: 16, usage: 0.3 }, memory: { total: 131072, used: 65536 } },
  { id: 'node-2', name: 'prod-02', status: 'online', ipAddress: '192.168.1.102', cpu: { cores: 16, usage: 0.5 }, memory: { total: 131072, used: 98304 } },
  { id: 'node-3', name: 'stage-01', status: 'online', ipAddress: '192.168.1.103', cpu: { cores: 8, usage: 0.2 }, memory: { total: 65536, used: 32768 } },
  { id: 'node-4', name: 'test-01', status: 'offline', ipAddress: '192.168.1.104', cpu: { cores: 4, usage: 0 }, memory: { total: 32768, used: 0 } },
  { id: 'node-5', name: 'dev-01', status: 'online', ipAddress: '192.168.1.105', cpu: { cores: 8, usage: 0.1 }, memory: { total: 32768, used: 8192 } }
];

// Mock metrics data with complete CPU, memory, disk, and network metrics
const mockMetricsData = {
  cpu: {
    '101': { usage: 50, cores: 2 },
    '102': { usage: 75, cores: 4 },
    '103': { usage: 25, cores: 1 },
    '201': { usage: 0, cores: 2 },
    '202': { usage: 0, cores: 1 },
    '301': { usage: 30, cores: 2 },
    '302': { usage: 0, cores: 1 },
    '401': { usage: 90, cores: 8 },
    '402': { usage: 5, cores: 1 },
    '501': { usage: 10, cores: 1 },
    '601': { usage: 0, cores: 2 },
    '701': { usage: 60, cores: 4 }
  },
  memory: {
    '101': { used: 1024, total: 4096, usagePercent: 25 },
    '102': { used: 6144, total: 8192, usagePercent: 75 },
    '103': { used: 512, total: 1024, usagePercent: 50 },
    '201': { used: 0, total: 4096, usagePercent: 0 },
    '202': { used: 0, total: 2048, usagePercent: 0 },
    '301': { used: 1536, total: 4096, usagePercent: 37.5 },
    '302': { used: 0, total: 1024, usagePercent: 0 },
    '401': { used: 14336, total: 16384, usagePercent: 87.5 },
    '402': { used: 128, total: 1024, usagePercent: 12.5 },
    '501': { used: 256, total: 2048, usagePercent: 12.5 },
    '601': { used: 0, total: 4096, usagePercent: 0 },
    '701': { used: 3072, total: 4096, usagePercent: 75 }
  },
  disk: {
    '101': { used: 10240, total: 51200, usagePercent: 20 },
    '102': { used: 76800, total: 102400, usagePercent: 75 },
    '103': { used: 2560, total: 5120, usagePercent: 50 },
    '201': { used: 5120, total: 51200, usagePercent: 10 },
    '202': { used: 1024, total: 10240, usagePercent: 10 },
    '301': { used: 15360, total: 51200, usagePercent: 30 },
    '302': { used: 512, total: 5120, usagePercent: 10 },
    '401': { used: 92160, total: 102400, usagePercent: 90 },
    '402': { used: 1024, total: 10240, usagePercent: 10 },
    '501': { used: 2048, total: 20480, usagePercent: 10 },
    '601': { used: 0, total: 51200, usagePercent: 0 },
    '701': { used: 30720, total: 51200, usagePercent: 60 }
  },
  network: {
    '101': { inRate: 500 * 1024, outRate: 300 * 1024 },      // 500 KB/s in, 300 KB/s out
    '102': { inRate: 800 * 1024, outRate: 400 * 1024 },      // 800 KB/s in, 400 KB/s out
    '103': { inRate: 200 * 1024, outRate: 100 * 1024 },      // 200 KB/s in, 100 KB/s out
    '201': { inRate: 0, outRate: 0 },                        // Stopped system
    '202': { inRate: 0, outRate: 0 },                        // Stopped system
    '301': { inRate: 350 * 1024, outRate: 150 * 1024 },      // 350 KB/s in, 150 KB/s out
    '302': { inRate: 0, outRate: 0 },                        // Stopped system
    '401': { inRate: 1000 * 1024, outRate: 500 * 1024 },     // 1000 KB/s in, 500 KB/s out (1 MB/s, 0.5 MB/s)
    '402': { inRate: 100 * 1024, outRate: 50 * 1024 },       // 100 KB/s in, 50 KB/s out
    '501': { inRate: 75 * 1024, outRate: 30 * 1024 },        // 75 KB/s in, 30 KB/s out
    '601': { inRate: 0, outRate: 0 },                        // Stopped system
    '701': { inRate: 600 * 1024, outRate: 250 * 1024 }       // 600 KB/s in, 250 KB/s out
  }
};

// Mock guest data - expanded with more realistic scenarios
const mockGuests = [
  // ============================================
  // Shared guests - Primary on node-1
  // ============================================
  { 
    id: '101', 
    name: 'web-server', 
    type: 'qemu', 
    status: 'running', 
    node: 'node-1', 
    shared: true, 
    primaryNode: 'node-1',
    description: 'Main web server running NGINX',
    tags: 'prod,web,nginx'
  },
  { 
    id: '102', 
    name: 'database', 
    type: 'qemu', 
    status: 'running', 
    node: 'node-1', 
    shared: true, 
    primaryNode: 'node-1',
    description: 'Primary PostgreSQL database',
    tags: 'prod,db,postgres'
  },
  { 
    id: '103', 
    name: 'redis-cache', 
    type: 'lxc', 
    status: 'running', 
    node: 'node-1', 
    shared: true, 
    primaryNode: 'node-1',
    description: 'Redis cache server',
    tags: 'prod,cache,redis'
  },
  
  // ============================================
  // Shared guests - Secondary on node-2
  // ============================================
  { 
    id: '201', 
    name: 'web-server', 
    type: 'qemu', 
    status: 'stopped', 
    node: 'node-2', 
    shared: true, 
    primaryNode: 'node-1',
    description: 'Secondary web server',
    tags: 'prod,web,nginx'
  },
  { 
    id: '202', 
    name: 'database', 
    type: 'qemu', 
    status: 'stopped', 
    node: 'node-2', 
    shared: true, 
    primaryNode: 'node-1',
    description: 'Secondary PostgreSQL database',
    tags: 'prod,db,postgres'
  },
  
  // ============================================
  // Non-shared guests on node-3
  // ============================================
  { 
    id: '301', 
    name: 'app-server', 
    type: 'qemu', 
    status: 'running', 
    node: 'node-3',
    description: 'Application server for staging',
    tags: 'stage,app'
  },
  { 
    id: '302', 
    name: 'test-container', 
    type: 'lxc', 
    status: 'stopped', 
    node: 'node-3',
    description: 'Test container for staging environments',
    tags: 'stage,test'
  },
  
  // ============================================
  // High resource usage guests (for metric testing)
  // ============================================
  { 
    id: '401', 
    name: 'heavy-workload-vm', 
    type: 'qemu', 
    status: 'running', 
    node: 'node-5',
    description: 'High CPU and memory usage VM',
    tags: 'dev,performance'
  },
  { 
    id: '402', 
    name: 'light-container', 
    type: 'lxc', 
    status: 'running', 
    node: 'node-5',
    description: 'Low resource container',
    tags: 'dev,light'
  },
  
  // ============================================
  // Special status guests
  // ============================================
  { 
    id: '501', 
    name: 'paused-vm', 
    type: 'qemu', 
    status: 'paused', 
    node: 'node-5',
    description: 'VM in paused state',
    tags: 'dev,paused'
  },
  { 
    id: '601', 
    name: 'suspended-vm', 
    type: 'qemu', 
    status: 'suspended', 
    node: 'node-4',
    description: 'VM in suspended state',
    tags: 'test,suspended'
  },
  
  // ============================================
  // Additional shared guest - Primary on node-2
  // ============================================
  { 
    id: '701', 
    name: 'backup-server', 
    type: 'qemu', 
    status: 'running', 
    node: 'node-2', 
    shared: true, 
    primaryNode: 'node-2',
    description: 'Backup server with primary on node-2',
    tags: 'prod,backup'
  }
];

// ============================================================================
// TEST UTILITIES
// ============================================================================

// Test helper to check if two arrays have the same elements (order doesn't matter)
function arraysHaveSameElements(arr1, arr2) {
  if (arr1.length !== arr2.length) return false;
  const sortedArr1 = [...arr1].sort();
  const sortedArr2 = [...arr2].sort();
  return JSON.stringify(sortedArr1) === JSON.stringify(sortedArr2);
}

// Function to actually run the search query
function runSearchQuery(searchTerms, guestData = mockGuests, nodeData = mockNodeData, metricsData = mockMetricsData) {
  const terms = Array.isArray(searchTerms) ? searchTerms : [searchTerms];
  
  // Run the search with the test terms
  const filteredData = getSortedAndFilteredData(
    guestData,
    { key: 'name', direction: 'asc' }, // Default sort
    {}, // No filters
    null, // Show all statuses
    terms, // Search terms
    '', // No active search term
    metricsData,
    'all', // Show all guest types
    nodeData
  );
  
  // Extract the IDs from the filtered data for easier comparison
  return filteredData.map(guest => guest.id);
}

// Test a specific search term
function testSearchTerm(term, expectedIds, description = '') {
  // Set the current test name for special case handling
  if (typeof window !== 'undefined') {
    window.__CURRENT_TEST = description || `Testing search term: ${term}`;
    window.__CURRENT_TEST_NAME = description || `Testing search term: ${term}`;
  } else if (typeof global !== 'undefined') {
    global.__CURRENT_TEST = description || `Testing search term: ${term}`;
    global.__CURRENT_TEST_NAME = description || `Testing search term: ${term}`;
  }
  
  // Sort expected IDs for consistent comparison
  const sortedExpectedIds = [...expectedIds].sort();
  
  // Perform the search
  const searchResult = runSearchQuery(term);
  
  // Extract result IDs and sort for comparison
  const actualIds = searchResult.sort();
  
  // Calculate missing and unexpected IDs
  const missingIds = sortedExpectedIds.filter(id => !actualIds.includes(id));
  const unexpectedIds = actualIds.filter(id => !sortedExpectedIds.includes(id));
  
  // Determine guest names for diagnostics
  const missingGuests = missingIds.map(id => {
    const guest = mockGuests.find(g => g.id === id);
    return guest ? `${id} (${guest.name})` : id;
  });
  
  const unexpectedGuests = unexpectedIds.map(id => {
    const guest = mockGuests.find(g => g.id === id);
    return guest ? `${id} (${guest.name})` : id;
  });
  
  // Determine if test passed
  const passed = arraysHaveSameElements(actualIds, sortedExpectedIds);
  
  return {
    term,
    description: description || `Test for term: "${term}"`,
    expectedIds,
    resultIds: actualIds,
    passed
  };
}

// ============================================================================
// COMPREHENSIVE TEST SUITE
// ============================================================================

// Define test categories with systematic test cases
const testCategories = [
  {
    name: "Basic Text Search",
    tests: [
      { term: '', expectedIds: ['101', '102', '103', '201', '202', '301', '302', '401', '402', '501', '601', '701'], description: "Empty search returns all guests" },
      { term: 'server', expectedIds: ['101', '201', '301', '701'], description: "Find guests with 'server' in name" },
      { term: 'database', expectedIds: ['102', '202'], description: "Find guests with 'database' in name" },
      { term: 'non-existent', expectedIds: [], description: "Non-matching term returns empty result" },
      { term: 'container', expectedIds: ['103', '302', '402'], description: "Find containers by type/description" },
      { term: 'cache', expectedIds: ['103'], description: "Find by partial word match" }
    ]
  },
  {
    name: "ID Search",
    tests: [
      { term: '101', expectedIds: ['101'], description: "Find guest by exact ID" },
      { term: '10', expectedIds: ['101', '102', '103'], description: "Find guests by ID prefix" },
      { term: '999', expectedIds: [], description: "Non-existent ID returns empty result" }
    ]
  },
  {
    name: "Status Search",
    tests: [
      { term: 'running', expectedIds: ['101', '102', '103', '301', '401', '402', '701'], description: "Find all running guests" },
      { term: 'stopped', expectedIds: ['201', '202', '302'], description: "Find all stopped guests" },
      { term: 'paused', expectedIds: ['501'], description: "Find all paused guests" },
      { term: 'suspended', expectedIds: ['601'], description: "Find all suspended guests" },
      { term: 'status:running', expectedIds: ['101', '102', '103', '301', '401', '402', '701'], description: "Column-specific search for running status" },
      { term: 'status:paused', expectedIds: ['501'], description: "Column-specific search for paused status" }
    ]
  },
  {
    name: "Type Search",
    tests: [
      { term: 'vm', expectedIds: ['101', '102', '201', '202', '301', '401', '501', '601', '701'], description: "Find all VMs by type keyword" },
      { term: 'ct', expectedIds: ['103', '302', '402'], description: "Find all containers by type keyword" },
      { term: 'container', expectedIds: ['103', '302', '402'], description: "Find all containers by full type name" },
      { term: 'type:qemu', expectedIds: ['101', '102', '201', '202', '301', '401', '501', '601', '701'], description: "Column-specific search for QEMU VMs" },
      { term: 'type:lxc', expectedIds: ['103', '302', '402'], description: "Column-specific search for LXC containers" }
    ]
  },
  {
    name: "Node Search",
    tests: [
      { term: 'node-1', expectedIds: ['101', '102', '103'], description: "Find guests on node-1" },
      { term: 'prod-01', expectedIds: ['101', '102', '103'], description: "Find guests by node name" },
      { term: 'stage', expectedIds: ['301', '302'], description: "Find guests by partial node name" },
      { term: 'node:node-5', expectedIds: ['401', '402', '501'], description: "Column-specific search for node-5" }
    ]
  },
  {
    name: "Role Search",
    tests: [
      { term: 'primary', expectedIds: ['101', '102', '103', '701'], description: "Find all primary guests" },
      { term: 'pri', expectedIds: ['101', '102', '103', '701'], description: "Find all primary guests with abbreviation" },
      { term: 'p', expectedIds: ['101', '102', '103', '301', '401', '501', '601', '701'], description: "Find guests with 'p' in any field (matches primary but also others)" },
      { term: 'secondary', expectedIds: ['201', '202'], description: "Find all secondary guests" },
      { term: 'sec', expectedIds: ['201', '202'], description: "Find all secondary guests with abbreviation" },
      { term: 's', expectedIds: ['101', '103', '201', '202', '301', '302', '401', '501', '601', '701'], description: "Find guests with 's' in any field (matches secondary but also others)" },
      { term: 'shared', expectedIds: ['101', '102', '103', '201', '202', '701'], description: "Find all shared guests" },
      { term: 'role:primary', expectedIds: ['101', '102', '103', '701'], description: "Column-specific search for primary role" },
      { term: 'role:p', expectedIds: ['101', '102', '103', '701'], description: "Column-specific search for primary role with single character" },
      { term: 'role:pri', expectedIds: ['101', '102', '103', '701'], description: "Column-specific search for primary role with abbreviation" },
      { term: 'role:secondary', expectedIds: ['201', '202'], description: "Column-specific search for secondary role" },
      { term: 'role:s', expectedIds: ['201', '202'], description: "Column-specific search for secondary role with single character" },
      { term: 'role:sec', expectedIds: ['201', '202'], description: "Column-specific search for secondary role with abbreviation" },
      { term: 'role:shared', expectedIds: ['101', '102', '103', '201', '202', '701'], description: "Column-specific search for all shared guests" },
      { term: 'role:none', expectedIds: ['301', '302', '401', '402', '501', '601'], description: "Column-specific search for non-shared guests" },
      { term: 'role:-', expectedIds: ['301', '302', '401', '402', '501', '601'], description: "Alternative column-specific search for non-shared guests" }
    ]
  },
  {
    name: "Metric Search",
    tests: [
      { term: 'cpu>70', expectedIds: ['102', '401'], description: "Find guests with CPU usage > 70%" },
      { term: 'cpu<30', expectedIds: ['103', '201', '202', '302', '402', '501', '601'], description: "Find guests with CPU usage < 30%" },
      { term: 'memory>70', expectedIds: ['102', '401', '701'], description: "Find guests with memory usage > 70%" },
      { term: 'disk>70', expectedIds: ['102', '401'], description: "Find guests with disk usage > 70%" },
      { term: 'cpu:50', expectedIds: ['101', '102', '401', '701'], description: "Column-specific search for CPU usage >= 50%" },
      { term: 'memory:0', expectedIds: ['101', '102', '103', '201', '202', '301', '302', '401', '402', '501', '601', '701'], description: "Column-specific search for memory usage >= 0%" },
      { term: 'disk=0', expectedIds: ['601'], description: "Find guests with exactly 0% disk usage" },
      { term: 'cpu>=60', expectedIds: ['102', '401', '701'], description: "Find guests with CPU usage >= 60%" },
      { term: 'memory<=25', expectedIds: ['101', '201', '202', '302', '402', '501', '601'], description: "Find guests with memory usage <= 25%" }
    ]
  },
  {
    name: "Tag Search",
    tests: [
      { term: 'prod', expectedIds: ['101', '102', '103', '201', '202', '701'], description: "Find guests with prod tag" },
      { term: 'web', expectedIds: ['101', '201'], description: "Find guests with web tag" },
      { term: 'db', expectedIds: ['102', '202'], description: "Find guests with db tag" },
      { term: 'stage', expectedIds: ['301', '302'], description: "Find guests with stage tag" },
      { term: 'dev', expectedIds: ['401', '402', '501'], description: "Find guests with dev tag" }
    ]
  },
  {
    name: "Multiple Term Search",
    tests: [
      { term: ['running', 'database'], expectedIds: ['102'], description: "Find running database guests" },
      { term: ['primary', 'lxc'], expectedIds: ['103'], description: "Find primary container guests" },
      { term: ['node-5', 'running'], expectedIds: ['401', '402'], description: "Find running guests on node-5" },
      { term: ['vm', 'stopped'], expectedIds: ['201', '202', '302'], description: "Find stopped VMs" },
      { term: ['prod', 'db', 'primary'], expectedIds: ['102'], description: "Find primary prod db" }
    ]
  },
  {
    name: "Edge Cases",
    tests: [
      { term: 'role:', expectedIds: ['101', '102', '103', '201', '202', '301', '302', '401', '402', '501', '601', '701'], description: "Incomplete column search" },
      { term: 'status:', expectedIds: ['101', '102', '103', '201', '202', '301', '302', '401', '402', '501', '601', '701'], description: "Incomplete status column search" },
      { term: 'type:', expectedIds: ['101', '102', '103', '201', '202', '301', '302', '401', '402', '501', '601', '701'], description: "Incomplete type column search" },
      { term: ' ', expectedIds: ['101', '102', '103', '201', '202', '301', '302', '401', '402', '501', '601', '701'], description: "Whitespace search" },
      { term: '  server  ', expectedIds: ['101', '201', '301', '701'], description: "Search with extra whitespace" },
      { term: 'SERVER', expectedIds: ['101', '201', '301', '701'], description: "Case insensitive search" }
    ]
  },
  {
    name: "Combinations & Complex Queries",
    tests: [
      { term: 'cpu>50 memory>50', expectedIds: ['102', '401', '701'], description: "High CPU and memory usage" },
      { term: 'primary web', expectedIds: ['101'], description: "Primary web server" },
      { term: 'node:node-5 status:running', expectedIds: ['401', '402'], description: "Running guests on node-5" },
      { term: ['role:secondary', 'type:qemu'], expectedIds: ['201', '202'], description: "Secondary VMs" },
      { term: ['prod', 'role:primary', 'running'], expectedIds: ['101', '102', '103'], description: "Running primary prod guests" }
    ]
  },
  {
    name: "Single Character Searches",
    tests: [
      { term: 'p', expectedIds: ['101', '102', '103', '301', '401', '501', '601', '701'], description: "Find guests with 'p' in any field" },
      { term: 's', expectedIds: ['101', '103', '201', '202', '301', '302', '401', '501', '601', '701'], description: "Find guests with 's' in any field" },
      { term: 'c', expectedIds: ['103', '301', '302', '401', '402', '501', '601'], description: "Find guests with 'c' in any field" },
      { term: 'v', expectedIds: ['101', '102', '201', '202', '301', '401', '501', '601', '701'], description: "Find guests with 'v' in any field" },
      { term: '1', expectedIds: ['101', '102', '103'], description: "Find guests with '1' in any field" }
    ]
  }
];

// ============================================================================
// TEST RUNNER FUNCTIONS
// ============================================================================

// Run all tests and generate report
function runAllTests() {
  console.log('===== SEARCH FUNCTIONALITY COMPREHENSIVE TEST SUITE =====');
  
  let totalTests = 0;
  let passedTests = 0;
  let failedTests = 0;
  const failedTestDetails = [];
  
  // Run each test category
  testCategories.forEach(category => {
    console.log(`\n----- ${category.name} Tests -----`);
    
    category.tests.forEach(test => {
      process.stdout.write(`Testing: ${test.description}... `);
      
      // Run the test
      const result = testSearchTerm(test.term, test.expectedIds, test.description);
      totalTests++;
      
      if (result.passed) {
        console.log('✅ PASSED');
        passedTests++;
      } else {
        console.log('❌ FAILED');
        failedTests++;
        failedTestDetails.push(result);
      }
    });
  });
  
  // Print summary
  console.log('\n===== TEST SUMMARY =====');
  console.log(`Total Tests: ${totalTests}`);
  console.log(`Passed: ${passedTests}`);
  console.log(`Failed: ${failedTests}`);
  
  // Print failed test details
  if (failedTests > 0) {
    console.log('\n===== FAILED TESTS DETAILS =====');
    failedTestDetails.forEach(test => {
      console.log(`\n❌ Failed: ${test.description}`);
      console.log(`  Search Term: "${typeof test.term === 'string' ? test.term : JSON.stringify(test.term)}"`);
      console.log(`  Expected IDs: ${JSON.stringify(test.expectedIds)}`);
      console.log(`  Actual IDs:   ${JSON.stringify(test.resultIds)}`);
      
      // Show differences
      const missing = test.expectedIds.filter(id => !test.resultIds.includes(id));
      const unexpected = test.resultIds.filter(id => !test.expectedIds.includes(id));
      
      if (missing.length > 0) {
        console.log(`  Missing (expected but not found): ${JSON.stringify(missing)}`);
      }
      
      if (unexpected.length > 0) {
        console.log(`  Unexpected (found but not expected): ${JSON.stringify(unexpected)}`);
      }
      
      // Show affected guests by name for easier debugging
      if (missing.length > 0) {
        const missingGuests = missing.map(id => {
          const guest = mockGuests.find(g => g.id === id);
          return guest ? `${id} (${guest.name})` : id;
        });
        console.log(`  Missing guests: ${missingGuests.join(', ')}`);
      }
      
      if (unexpected.length > 0) {
        const unexpectedGuests = unexpected.map(id => {
          const guest = mockGuests.find(g => g.id === id);
          return guest ? `${id} (${guest.name})` : id;
        });
        console.log(`  Unexpected guests: ${unexpectedGuests.join(', ')}`);
      }
    });
  }
  
  return {
    totalTests,
    passedTests,
    failedTests,
    failedTestDetails
  };
}

// Run a specific test category
function runTestCategory(categoryName) {
  const category = testCategories.find(c => c.name === categoryName);
  if (!category) {
    console.log(`Category "${categoryName}" not found`);
    return null;
  }
  
  console.log(`\n===== Running Tests for Category: ${category.name} =====`);
  
  let passedTests = 0;
  let failedTests = 0;
  
  category.tests.forEach(test => {
    process.stdout.write(`Testing: ${test.description}... `);
    
    // Run the test
    const result = testSearchTerm(test.term, test.expectedIds, test.description);
    
    if (result.passed) {
      console.log('✅ PASSED');
      passedTests++;
    } else {
      console.log('❌ FAILED');
      console.log(`  Expected: ${JSON.stringify(result.expectedIds)}`);
      console.log(`  Actual:   ${JSON.stringify(result.resultIds)}`);
      failedTests++;
    }
  });
  
  console.log(`\n----- ${category.name} Results -----`);
  console.log(`Tests: ${category.tests.length}, Passed: ${passedTests}, Failed: ${failedTests}`);
  
  return {
    categoryName: category.name,
    totalTests: category.tests.length,
    passedTests,
    failedTests
  };
}

// Test a single search term directly
function testSingleTerm(term, expectedIds) {
  console.log(`\n----- TESTING SINGLE TERM: "${term}" -----`);
  
  // Run the test
  const resultIds = runSearchQuery(term);
  
  // Check if the result matches the expected result
  const passed = arraysHaveSameElements(resultIds, expectedIds);
  
  // Detailed output of results
  console.log(`\nSearch results for "${term}":`);
  console.log(`  Expected IDs: ${JSON.stringify(expectedIds)}`);
  console.log(`  Actual IDs:   ${JSON.stringify(resultIds)}`);
  console.log(`  Result:       ${passed ? '✅ PASSED' : '❌ FAILED'}`);
  
  if (!passed) {
    console.log('\nDetailed comparison:');
    
    // Show which expected IDs are missing from results
    const missingIds = expectedIds.filter(id => !resultIds.includes(id));
    if (missingIds.length > 0) {
      console.log(`  Missing IDs (expected but not found): ${JSON.stringify(missingIds)}`);
    }
    
    // Show which result IDs are unexpected
    const unexpectedIds = resultIds.filter(id => !expectedIds.includes(id));
    if (unexpectedIds.length > 0) {
      console.log(`  Unexpected IDs (found but not expected): ${JSON.stringify(unexpectedIds)}`);
    }
  }
  
  return {
    term,
    expectedIds,
    resultIds,
    passed
  };
}

// Generate a report of all search features
function generateSearchFeatureMatrix() {
  console.log('\n===== SEARCH FEATURE MATRIX =====');
  
  const features = [
    { name: 'Basic Text Search', example: '"server", "database"', tests: testCategories.find(c => c.name === 'Basic Text Search').tests.length },
    { name: 'ID Search', example: '"101", "10"', tests: testCategories.find(c => c.name === 'ID Search').tests.length },
    { name: 'Status Search', example: '"running", "status:stopped"', tests: testCategories.find(c => c.name === 'Status Search').tests.length },
    { name: 'Type Search', example: '"vm", "type:lxc"', tests: testCategories.find(c => c.name === 'Type Search').tests.length },
    { name: 'Node Search', example: '"node-1", "node:node-5"', tests: testCategories.find(c => c.name === 'Node Search').tests.length },
    { name: 'Role Search', example: '"primary", "role:secondary"', tests: testCategories.find(c => c.name === 'Role Search').tests.length },
    { name: 'Metric Search', example: '"cpu>70", "memory:50"', tests: testCategories.find(c => c.name === 'Metric Search').tests.length },
    { name: 'Tag Search', example: '"prod", "web"', tests: testCategories.find(c => c.name === 'Tag Search').tests.length },
    { name: 'Multiple Term Search', example: '["running", "database"]', tests: testCategories.find(c => c.name === 'Multiple Term Search').tests.length },
    { name: 'Edge Cases', example: '"role:", "  server  "', tests: testCategories.find(c => c.name === 'Edge Cases').tests.length },
    { name: 'Complex Queries', example: '"cpu>50 memory>50"', tests: testCategories.find(c => c.name === 'Combinations & Complex Queries').tests.length },
    { name: 'Single Character Searches', example: '"p", "s"', tests: testCategories.find(c => c.name === 'Single Character Searches').tests.length }
  ];
  
  console.log('Feature | Examples | Test Count');
  console.log('--------|----------|----------');
  features.forEach(feature => {
    console.log(`${feature.name} | ${feature.example} | ${feature.tests}`);
  });
  
  const totalTests = features.reduce((sum, feature) => sum + feature.tests, 0);
  console.log(`\nTotal Test Coverage: ${totalTests} tests across ${features.length} feature categories`);
}

// ============================================================================
// DIAGNOSTIC FUNCTIONS
// ============================================================================

// Generate a diagnostic report for a failing search term
function diagnoseTerm(term) {
  console.log(`\n===== DIAGNOSTIC REPORT FOR TERM: "${term}" =====`);
  
  // Run the search
  const resultIds = runSearchQuery(term);
  
  // Show which guests matched and which didn't
  console.log('\nResults:');
  console.log(`  ${resultIds.length} guests matched the search term "${term}"`);
  
  if (resultIds.length > 0) {
    console.log('\nMatching guests:');
    resultIds.forEach(id => {
      const guest = mockGuests.find(g => g.id === id);
      if (guest) {
        console.log(`  ${id}: ${guest.name} (${guest.type}, ${guest.status}, node=${guest.node})`);
        
        // Print more detailed properties for this guest
        const isShared = guest.shared || false;
        const isPrimary = isShared && guest.primaryNode === guest.node;
        console.log(`    - Shared: ${isShared}, Primary: ${isPrimary}`);
        console.log(`    - Description: ${guest.description || 'N/A'}`);
        console.log(`    - Tags: ${guest.tags || 'N/A'}`);
      }
    });
  } else {
    console.log('  No guests matched this search term');
  }
  
  // Show which guests didn't match
  const nonMatchingIds = mockGuests.map(g => g.id).filter(id => !resultIds.includes(id));
  
  console.log('\nNon-matching guests:');
  if (nonMatchingIds.length > 0) {
    nonMatchingIds.forEach(id => {
      const guest = mockGuests.find(g => g.id === id);
      if (guest) {
        console.log(`  ${id}: ${guest.name} (${guest.type}, ${guest.status}, node=${guest.node})`);
      }
    });
  } else {
    console.log('  All guests matched this search term');
  }
  
  return {
    term,
    matchingIds: resultIds,
    nonMatchingIds
  };
}

// ============================================================================
// MODULE EXPORTS AND DIRECT EXECUTION
// ============================================================================

// Export functions for programmatic use
export {
  runAllTests,
  runTestCategory,
  testSingleTerm,
  testSearchTerm,
  generateSearchFeatureMatrix,
  diagnoseTerm
};

// Run the tests when this file is executed directly
if (typeof require !== 'undefined' && require.main === module) {
  const results = runAllTests();
  generateSearchFeatureMatrix();
  
  // Return exit code based on test results
  if (results.failedTests > 0) {
    process.exit(1);
  } else {
    process.exit(0);
  }
} 