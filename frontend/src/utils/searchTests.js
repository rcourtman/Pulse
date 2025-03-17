/**
 * Search Functionality Test Suite
 * 
 * This file contains tests for verifying the search functionality in the NetworkUtils module.
 * Run this file using Node.js to verify all search capabilities are working correctly.
 */

import { getSortedAndFilteredData } from './networkUtils';

// Mock data for testing search functionality - expanded to better match real application structure
const mockGuests = [
  // Primary node guests
  { 
    id: '101', 
    name: 'web-server', 
    type: 'qemu', 
    status: 'running', 
    node: 'node-1', 
    shared: true, 
    primaryNode: 'node-1',
    cpu: 0.5,
    memory: { used: 1024, total: 4096 },
    disk: { used: 10240, total: 51200 }
  },
  { 
    id: '102', 
    name: 'database', 
    type: 'qemu', 
    status: 'running', 
    node: 'node-1', 
    shared: true, 
    primaryNode: 'node-1',
    cpu: 0.7,
    memory: { used: 2048, total: 8192 },
    disk: { used: 20480, total: 102400 }
  },
  
  // Secondary node guests
  { 
    id: '201', 
    name: 'cache-server', 
    type: 'qemu', 
    status: 'stopped', 
    node: 'node-2', 
    shared: true, 
    primaryNode: 'node-1',
    cpu: 0,
    memory: { used: 0, total: 4096 },
    disk: { used: 5120, total: 51200 }
  },
  { 
    id: '202', 
    name: 'backup-db', 
    type: 'lxc', 
    status: 'stopped', 
    node: 'node-2', 
    shared: true, 
    primaryNode: 'node-1',
    cpu: 0,
    memory: { used: 0, total: 2048 },
    disk: { used: 1024, total: 10240 }
  },
  
  // Non-shared guests
  { 
    id: '301', 
    name: 'standalone-app', 
    type: 'qemu', 
    status: 'running', 
    node: 'node-3',
    cpu: 0.3,
    memory: { used: 1536, total: 4096 },
    disk: { used: 15360, total: 51200 }
  },
  { 
    id: '302', 
    name: 'standalone-container', 
    type: 'lxc', 
    status: 'stopped', 
    node: 'node-3',
    cpu: 0,
    memory: { used: 0, total: 1024 },
    disk: { used: 512, total: 5120 }
  }
];

// Mock node data
const mockNodeData = [
  { id: 'node-1', name: 'pve-prod-01' },
  { id: 'node-2', name: 'pve-prod-02' },
  { id: 'node-3', name: 'pve-dev-01' }
];

// Mock metrics data
const mockMetricsData = { 
  cpu: {
    '101': { usage: 0.5 },
    '102': { usage: 0.7 },
    '201': { usage: 0 },
    '202': { usage: 0 },
    '301': { usage: 0.3 },
    '302': { usage: 0 }
  },
  memory: {
    '101': { usagePercent: 25 },
    '102': { usagePercent: 25 },
    '201': { usagePercent: 0 },
    '202': { usagePercent: 0 },
    '301': { usagePercent: 37.5 },
    '302': { usagePercent: 0 }
  },
  disk: {
    '101': { usagePercent: 20 },
    '102': { usagePercent: 20 },
    '201': { usagePercent: 10 },
    '202': { usagePercent: 10 },
    '301': { usagePercent: 30 },
    '302': { usagePercent: 10 }
  },
  network: {
    '101': { inRate: 500 * 1024, outRate: 300 * 1024 },  // Running system
    '102': { inRate: 800 * 1024, outRate: 400 * 1024 },  // Running system
    '201': { inRate: 0, outRate: 0 },                    // Stopped system
    '202': { inRate: 0, outRate: 0 },                    // Stopped system
    '301': { inRate: 350 * 1024, outRate: 150 * 1024 },  // Running system
    '302': { inRate: 0, outRate: 0 }                     // Stopped system
  }
};

// Deep clone function to ensure we don't accidentally modify test data
function deepClone(obj) {
  return JSON.parse(JSON.stringify(obj));
}

// Test helper to check if two arrays have the same elements (order doesn't matter)
function arraysHaveSameElements(arr1, arr2) {
  if (arr1.length !== arr2.length) return false;
  const sortedArr1 = [...arr1].sort();
  const sortedArr2 = [...arr2].sort();
  return JSON.stringify(sortedArr1) === JSON.stringify(sortedArr2);
}

// Test a single search term directly
function testSingleSearchTerm(term, expectedIds) {
  console.log(`\n----- TESTING SINGLE TERM: "${term}" -----`);
  
  // Clone the mock data to avoid any side effects
  const guestData = deepClone(mockGuests);
  const nodeData = deepClone(mockNodeData);
  const metricsData = deepClone(mockMetricsData);
  
  // Find what objects actually match the search term for debugging
  let searchableTextResults = [];
  guestData.forEach(guest => {
    // This reproduces how the searchable text is generated
    const nodeName = (nodeData.find(n => n.id === guest.node) || {}).name || guest.node || '';
    const isShared = guest.shared || false;
    const isPrimary = isShared && guest.primaryNode === guest.node;
    
    // Include role information in searchable text with additional keywords
    let roleText = '';
    if (isShared) {
      roleText = isPrimary ? 'primary pri' : 'secondary sec';
    }
    
    const searchableText = [
      guest.name || '',
      guest.id || '',
      guest.status || '',
      guest.type === 'qemu' ? 'vm virtual machine' : 'ct container',
      nodeName,
      roleText
    ].join(' ').toLowerCase();
    
    const match = searchableText.includes(term.toLowerCase());
    if (match) {
      searchableTextResults.push({
        id: guest.id,
        searchableText
      });
    }
  });
  
  console.log(`Searchable text matches for "${term}":`);
  if (searchableTextResults.length === 0) {
    console.log('  NONE FOUND in searchable text');
  } else {
    searchableTextResults.forEach(result => {
      console.log(`  ID ${result.id}: "${result.searchableText}"`);
    });
  }
  
  // Run the search with the test term
  const filteredData = getSortedAndFilteredData(
    guestData,
    { key: 'name', direction: 'asc' }, // Default sort
    {}, // No filters
    null, // Show all statuses
    [term], // Single search term as an array
    '', // No active search term
    metricsData,
    'all', // Show all guest types
    nodeData
  );
  
  // Extract the IDs from the filtered data for easier comparison
  const resultIds = filteredData.map(guest => guest.id);
  
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
    passed,
    searchableTextResults
  };
}

// Role-specific test runner
function testRoleSearches() {
  console.log('\n===== ROLE SEARCH SPECIFIC TESTS =====');
  
  // Directly test various role-related search terms
  const roleTests = [
    // Standalone terms
    { term: 'role', expectedIds: ['101', '102', '201', '202'] },
    { term: 'shared', expectedIds: ['101', '102', '201', '202'] },
    { term: 'primary', expectedIds: ['101', '102'] },
    { term: 'pri', expectedIds: ['101', '102'] }, // This is the problematic one in the real app
    { term: 'secondary', expectedIds: ['201', '202'] },
    { term: 'sec', expectedIds: ['201', '202'] },
    
    // Single character searches - should match any item containing that letter
    { term: 'p', expectedIds: ['101', '102', '201', '202', '301', '302'] }, // All have 'p' in node names or other fields
    { term: 's', expectedIds: ['101', '102', '201', '202', '301', '302'] }, // All have 's' in various fields
    { term: 'd', expectedIds: ['101', '102', '201', '202', '301', '302'] }, // All have 'd' in node names or other fields
    { term: 'v', expectedIds: ['101', '102', '201', '202', '301', '302'] }, // All have 'v' in 'vm' or 'virtual' or 'dev'
    
    // Prefixed searches
    { term: 'role:primary', expectedIds: ['101', '102'] },
    { term: 'role:p', expectedIds: ['101', '102'] },
    { term: 'role:pri', expectedIds: ['101', '102'] },
    { term: 'role:secondary', expectedIds: ['201', '202'] },
    { term: 'role:s', expectedIds: ['201', '202'] },
    { term: 'role:sec', expectedIds: ['201', '202'] },
    { term: 'role:-', expectedIds: ['301', '302'] },
    { term: 'role:none', expectedIds: ['301', '302'] }
  ];
  
  // Run each role test and report results
  const roleResults = roleTests.map(test => testSingleSearchTerm(test.term, test.expectedIds));
  
  // Summary
  const passedRoleTests = roleResults.filter(r => r.passed).length;
  console.log('\n----- ROLE SEARCH TESTS SUMMARY -----');
  console.log(`Total: ${roleTests.length}, Passed: ${passedRoleTests}, Failed: ${roleTests.length - passedRoleTests}`);
  
  if (passedRoleTests < roleTests.length) {
    console.log('\nFailed role tests:');
    roleResults
      .filter(r => !r.passed)
      .forEach(r => {
        console.log(`  "${r.term}": Expected ${JSON.stringify(r.expectedIds)}, got ${JSON.stringify(r.resultIds)}`);
      });
  }
  
  return roleResults;
}

// Test runner function
function runSearchTests() {
  console.log('===== SEARCH FUNCTIONALITY TEST SUITE =====');
  
  // A simple test harness to run the tests
  const tests = [
    // Basic text search tests
    {
      name: 'Basic text search',
      terms: ['server'],
      expectedIds: ['101', '201'],
      description: 'Should find guests containing "server" in their name'
    },
    {
      name: 'Empty search',
      terms: [''],
      expectedIds: ['101', '102', '201', '202', '301', '302'],
      description: 'Empty search should return all guests'
    },
    
    // Type search tests
    {
      name: 'VM type search',
      terms: ['vm'],
      expectedIds: ['101', '102', '201', '301'],
      description: 'Should find all VM guests'
    },
    {
      name: 'Container type search',
      terms: ['ct'],
      expectedIds: ['202', '302'],
      description: 'Should find all container guests'
    },
    {
      name: 'Type:vm search',
      terms: ['type:vm'],
      expectedIds: ['101', '102', '201', '301'],
      description: 'Column-specific search for VMs'
    },
    
    // Status search tests
    {
      name: 'Running status search',
      terms: ['running'],
      expectedIds: ['101', '102', '301'],
      description: 'Should find running guests'
    },
    {
      name: 'Status:stopped search',
      terms: ['status:stopped'],
      expectedIds: ['201', '202', '302'],
      description: 'Column-specific search for stopped guests'
    },
    
    // Node search tests
    {
      name: 'Node search by name',
      terms: ['prod-01'],
      expectedIds: ['101', '102'],
      description: 'Should find guests on node "prod-01"'
    },
    {
      name: 'Node:node-2 search',
      terms: ['node:node-2'],
      expectedIds: ['201', '202'],
      description: 'Column-specific search for node-2'
    },
    
    // Role search tests - these are the ones having issues
    {
      name: 'Role search (standalone term)',
      terms: ['role'],
      expectedIds: ['101', '102', '201', '202'],
      description: 'Should find all shared guests (primary or secondary)'
    },
    {
      name: 'Shared search (alternative)',
      terms: ['shared'],
      expectedIds: ['101', '102', '201', '202'],
      description: 'Alternative to "role" search'
    },
    {
      name: 'Primary search (standalone)',
      terms: ['primary'],
      expectedIds: ['101', '102'],
      description: 'Should find primary guests without using role prefix'
    },
    {
      name: 'Pri search (short form)',
      terms: ['pri'],
      expectedIds: ['101', '102'],
      description: 'Short form of primary search'
    },
    {
      name: 'Secondary search (standalone)',
      terms: ['secondary'],
      expectedIds: ['201', '202'],
      description: 'Should find secondary guests without using role prefix'
    },
    {
      name: 'Sec search (short form)',
      terms: ['sec'],
      expectedIds: ['201', '202'],
      description: 'Short form of secondary search'
    },
    {
      name: 'Role:primary search',
      terms: ['role:primary'],
      expectedIds: ['101', '102'],
      description: 'Column-specific search for primary role'
    },
    {
      name: 'Role:p search (partial)',
      terms: ['role:p'],
      expectedIds: ['101', '102'],
      description: 'Column-specific search with partial term for primary'
    },
    {
      name: 'Role:pri search (partial)',
      terms: ['role:pri'],
      expectedIds: ['101', '102'],
      description: 'Column-specific search with partial term for primary'
    },
    {
      name: 'Role:secondary search',
      terms: ['role:secondary'],
      expectedIds: ['201', '202'],
      description: 'Column-specific search for secondary role'
    },
    {
      name: 'Role:s search (partial)',
      terms: ['role:s'],
      expectedIds: ['201', '202'],
      description: 'Column-specific search with partial term for secondary'
    },
    {
      name: 'Role:sec search (partial)',
      terms: ['role:sec'],
      expectedIds: ['201', '202'],
      description: 'Column-specific search with partial term for secondary'
    },
    {
      name: 'Role:- search (non-shared)',
      terms: ['role:-'],
      expectedIds: ['301', '302'],
      description: 'Column-specific search for non-shared guests'
    },
    {
      name: 'Role:none search (non-shared)',
      terms: ['role:none'],
      expectedIds: ['301', '302'],
      description: 'Column-specific search for non-shared guests, alternative'
    },
    
    // Multiple term search tests
    {
      name: 'Multiple terms (AND logic)',
      terms: ['primary', 'database'],
      expectedIds: ['102'],
      description: 'Should find guests matching both terms (primary AND database)'
    }
  ];
  
  // Run general tests first
  const results = {
    passed: 0,
    failed: 0,
    details: []
  };
  
  tests.forEach(test => {
    process.stdout.write(`Testing: ${test.name} ... `);
    
    // Run the search with the test terms
    const filteredData = getSortedAndFilteredData(
      mockGuests,
      { key: 'name', direction: 'asc' }, // Default sort
      {}, // No filters
      null, // Show all statuses
      test.terms, // Search terms
      '', // No active search term
      mockMetricsData,
      'all', // Show all guest types
      mockNodeData
    );
    
    // Extract the IDs from the filtered data for easier comparison
    const resultIds = filteredData.map(guest => guest.id);
    
    // Check if the result matches the expected result
    const sortedExpected = [...test.expectedIds].sort();
    const sortedResults = [...resultIds].sort();
    const passed = JSON.stringify(sortedExpected) === JSON.stringify(sortedResults);
    
    if (passed) {
      console.log('✅ PASSED');
      results.passed++;
    } else {
      console.log('❌ FAILED');
      console.log(`  Expected: ${JSON.stringify(sortedExpected)}`);
      console.log(`  Actual:   ${JSON.stringify(sortedResults)}`);
      results.failed++;
    }
    
    results.details.push({
      ...test,
      passed,
      actualIds: resultIds
    });
  });
  
  // Now run detailed role search tests
  const roleResults = testRoleSearches();
  
  // Print summary of all tests
  console.log('\n===== COMPLETE TEST SUMMARY =====');
  console.log(`General tests: ${tests.length}, Passed: ${results.passed}, Failed: ${results.failed}`);
  
  const passedRoleTests = roleResults.filter(r => r.passed).length;
  const failedRoleTests = roleResults.length - passedRoleTests;
  console.log(`Role tests: ${roleResults.length}, Passed: ${passedRoleTests}, Failed: ${failedRoleTests}`);
  
  console.log(`Total: ${tests.length + roleResults.length}, Passed: ${results.passed + passedRoleTests}, Failed: ${results.failed + failedRoleTests}`);
  
  // Print failed tests for quick reference
  if (results.failed > 0 || failedRoleTests > 0) {
    console.log('\n===== FAILED TESTS =====');
    
    // Failed general tests
    results.details
      .filter(detail => !detail.passed)
      .forEach(detail => {
        console.log(`❌ ${detail.name}: ${detail.description}`);
        console.log(`  Expected: ${JSON.stringify(detail.expectedIds)}`);
        console.log(`  Actual:   ${JSON.stringify(detail.actualIds)}`);
      });
    
    // Failed role tests
    roleResults
      .filter(r => !r.passed)
      .forEach(r => {
        console.log(`❌ Role test "${r.term}"`);
        console.log(`  Expected: ${JSON.stringify(r.expectedIds)}`);
        console.log(`  Actual:   ${JSON.stringify(r.resultIds)}`);
      });
  }
  
  return {
    general: results,
    role: roleResults
  };
}

// For programmatic usage
export function runTermTest(term, expectedIds) {
  return testSingleSearchTerm(term, expectedIds);
}

// Run the tests when this file is executed directly
if (typeof require !== 'undefined' && require.main === module) {
  runSearchTests();
}

export { runSearchTests, testRoleSearches, testSingleSearchTerm }; 