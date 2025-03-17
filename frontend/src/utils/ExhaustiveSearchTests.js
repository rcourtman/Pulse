/**
 * Exhaustive Search Test Suite
 * 
 * This test suite verifies ALL possible search patterns, combinations, and edge cases.
 * It uses both mock data and real-world data structures to ensure accuracy.
 */

const { getSortedAndFilteredData } = require('./networkUtils');

// Create a more extensive mock data set with varied properties
const generateExhaustiveTestData = () => {
  // Base test data similar to the real application
  const guests = [
    // PRIMARY GUESTS
    { 
      id: '101', 
      name: 'web-server',  // Plain name
      type: 'qemu', 
      status: 'running', 
      node: 'pve-prod-01', 
      shared: true, 
      primaryNode: 'pve-prod-01',
      tags: ['web', 'production']
    },
    { 
      id: '102', 
      name: 'database-primary',  // Has "primary" in name
      type: 'qemu', 
      status: 'running', 
      node: 'pve-prod-01', 
      shared: true, 
      primaryNode: 'pve-prod-01',
      tags: ['db', 'primary', 'production']
    },
    { 
      id: '103', 
      name: 'sprint-server',  // Has "pri" as substring
      type: 'qemu', 
      status: 'paused', 
      node: 'pve-prod-01', 
      shared: true, 
      primaryNode: 'pve-prod-01',
      tags: ['sprint', 'production']
    },
    
    // SECONDARY GUESTS
    { 
      id: '201', 
      name: 'web-secondary', 
      type: 'qemu', 
      status: 'running', 
      node: 'pve-prod-02', 
      shared: true, 
      primaryNode: 'pve-prod-01',
      tags: ['web', 'secondary']
    },
    { 
      id: '202', 
      name: 'sec-database', // Has "sec" at start of name
      type: 'lxc', 
      status: 'stopped', 
      node: 'pve-prod-02', 
      shared: true, 
      primaryNode: 'pve-prod-01',
      tags: ['db', 'backup']
    },
    { 
      id: '203', 
      name: 'prism-backup',  // Has "pri" as substring
      type: 'lxc', 
      status: 'running', 
      node: 'pve-prod-02', 
      shared: true, 
      primaryNode: 'pve-prod-01',
      tags: ['backup']
    },
    
    // NON-SHARED GUESTS
    { 
      id: '301', 
      name: 'standalone-pri-app',  // Has "pri" in name but is NOT primary
      type: 'qemu', 
      status: 'running', 
      node: 'pve-dev-01',
      shared: false,
      tags: ['dev', 'private']
    },
    { 
      id: '302', 
      name: 'security-container',  // Has "sec" as substring but is NOT secondary
      type: 'lxc', 
      status: 'stopped', 
      node: 'pve-dev-01',
      shared: false,
      tags: ['security', 'dev']
    },
    
    // EDGE CASES
    { 
      id: '401', 
      name: 'p', // Single-letter name matching a role search
      type: 'qemu', 
      status: 'running', 
      node: 'pve-edge-01', 
      shared: true, 
      primaryNode: 'pve-edge-01',
      tags: ['test']
    },
    { 
      id: '402', 
      name: 's', // Single-letter name matching a role search
      type: 'qemu', 
      status: 'running', 
      node: 'pve-edge-01', 
      shared: true, 
      primaryNode: 'pve-prod-01',
      tags: ['test']
    }
  ];
  
  // Add optional metrics data
  const metricsData = {
    cpu: {},
    memory: {},
    disk: {},
    network: {}
  };
  
  guests.forEach(guest => {
    const id = guest.id;
    metricsData.cpu[id] = { usage: Math.random() * 100 };
    metricsData.memory[id] = { usagePercent: Math.random() * 100 };
    metricsData.disk[id] = { usagePercent: Math.random() * 100 };
    metricsData.network[id] = { inRate: Math.random() * 10000000, outRate: Math.random() * 10000000 };
  });
  
  // Node data for name resolution
  const nodeData = [
    { id: 'pve-prod-01', name: 'prod-cluster-1' },
    { id: 'pve-prod-02', name: 'prod-cluster-2' },
    { id: 'pve-dev-01', name: 'dev-cluster' },
    { id: 'pve-edge-01', name: 'edge-node' }
  ];
  
  return { guests, metricsData, nodeData };
};

// Function to run all exhaustive tests
export function runExhaustiveTests() {
  console.log('===== EXHAUSTIVE SEARCH TEST SUITE =====');
  console.log('Testing all possible search patterns and edge cases');
  
  const { guests, metricsData, nodeData } = generateExhaustiveTestData();
  
  // Collection of all test cases
  const testCases = [
    // 1. ROLE BASED SEARCHES - STANDALONE TERMS
    // Each of these tests a specific role search term
    { category: 'ROLE STANDALONE', term: 'role', description: 'All shared guests', 
      expectIdsContaining: ['101', '102', '103', '201', '202', '203', '401', '402'] },
      
    { category: 'ROLE STANDALONE', term: 'shared', description: 'Alternative for all shared guests', 
      expectIdsContaining: ['101', '102', '103', '201', '202', '203', '401', '402'] },
      
    { category: 'ROLE STANDALONE', term: 'primary', description: 'Finds primary guests', 
      expectIdsContaining: ['101', '102', '103', '401'] },
      
    { category: 'ROLE STANDALONE', term: 'pri', description: 'Short form of primary', 
      expectIdsContaining: ['101', '102', '103', '401'] },
      
    { category: 'ROLE STANDALONE', term: 'p', description: 'Shortest form of primary', 
      expectIdsContaining: ['101', '102', '103', '401'] },
      
    { category: 'ROLE STANDALONE', term: 'secondary', description: 'Finds secondary guests', 
      expectIdsContaining: ['201', '202', '203', '402'] },
      
    { category: 'ROLE STANDALONE', term: 'sec', description: 'Short form of secondary', 
      expectIdsContaining: ['201', '202', '203', '402'] },
      
    { category: 'ROLE STANDALONE', term: 's', description: 'Shortest form of secondary', 
      expectIdsContaining: ['201', '202', '203', '402'] },
    
    // 2. ROLE COLUMN SEARCHES
    { category: 'ROLE COLUMN', term: 'role:primary', description: 'Column search for primary', 
      expectIdsContaining: ['101', '102', '103', '401'] },
      
    { category: 'ROLE COLUMN', term: 'role:pri', description: 'Column search with short primary', 
      expectIdsContaining: ['101', '102', '103', '401'] },
      
    { category: 'ROLE COLUMN', term: 'role:p', description: 'Column search with shortest primary', 
      expectIdsContaining: ['101', '102', '103', '401'] },
      
    { category: 'ROLE COLUMN', term: 'role:secondary', description: 'Column search for secondary', 
      expectIdsContaining: ['201', '202', '203', '402'] },
      
    { category: 'ROLE COLUMN', term: 'role:sec', description: 'Column search with short secondary', 
      expectIdsContaining: ['201', '202', '203', '402'] },
      
    { category: 'ROLE COLUMN', term: 'role:s', description: 'Column search with shortest secondary', 
      expectIdsContaining: ['201', '202', '203', '402'] },
      
    { category: 'ROLE COLUMN', term: 'role:none', description: 'Column search for non-shared', 
      expectIdsContaining: ['301', '302'] },
      
    { category: 'ROLE COLUMN', term: 'role:-', description: 'Column search for non-shared (dash)', 
      expectIdsContaining: ['301', '302'] },
    
    // 3. TEXT SEARCHES WITH ROLE COMPONENTS
    { category: 'TEXT MATCH', term: 'primary-', description: 'Has primary in name but with a suffix', 
      expectIdsContaining: ['102'] },
      
    { category: 'TEXT MATCH', term: 'sprint', description: 'Has pri in middle of name but not as a word', 
      expectIdsContaining: ['103'] },
      
    { category: 'TEXT MATCH', term: 'secondary', description: 'Has secondary in name and/or is secondary', 
      expectIdsContaining: ['201', '202', '203', '402'] },
      
    { category: 'TEXT MATCH', term: 'prism', description: 'Has pri as substring', 
      expectIdsContaining: ['203'] },
    
    // 4. PARTIAL SEARCHES AND EDGE CASES
    // The 'p' search should match shared guests with primary role AND match 'p' in text
    { category: 'SINGLE CHAR', term: 'p', description: 'Single letter p', 
      expectIdsContaining: ['101', '102', '103', '201', '202', '203', '301', '302', '401', '402'] },
      
    // Same for 's'
    { category: 'SINGLE CHAR', term: 's', description: 'Single letter s', 
      expectIdsContaining: ['101', '102', '103', '201', '202', '203', '301', '302', '401', '402'] },
      
    // Test pve prefix in node names
    { category: 'SINGLE CHAR', term: 'pve', description: 'Text in all node IDs', 
      expectIdsContaining: ['101', '102', '103', '201', '202', '203', '301', '302', '401', '402'] },
      
    // Test a mix of properties and values
    { category: 'TYPE SEARCH', term: 'qemu', description: 'VM type search', 
      expectIdsContaining: ['101', '102', '103', '201', '301', '401', '402'] },
      
    { category: 'TYPE SEARCH', term: 'vm', description: 'VM alternate search', 
      expectIdsContaining: ['101', '102', '103', '201', '301', '401', '402'] },
      
    { category: 'TYPE SEARCH', term: 'lxc', description: 'Container type search', 
      expectIdsContaining: ['202', '203', '302'] },
    
    // 5. COMBINED SEARCHES (AND LOGIC)
    { category: 'COMBINED', term: 'primary running', description: 'Primary AND running status', 
      expectIdsContaining: ['101', '102', '401'] },
      
    { category: 'COMBINED', term: 'sec stopped', description: 'Secondary AND stopped status', 
      expectIdsContaining: ['202'] },
      
    // 6. NEGATIVE TESTS - these should NOT match
    { category: 'NEGATIVE', term: 'nonexistent', description: 'Term that appears nowhere', 
      expectIdsContaining: [] }
  ];
  
  // Run the tests
  console.log(`Running ${testCases.length} comprehensive search test cases\n`);
  
  let passCount = 0;
  let failCount = 0;
  
  // Keep track of failures by category
  const failures = {};
  
  testCases.forEach((testCase, index) => {
    console.log(`[${index + 1}/${testCases.length}] Testing "${testCase.term}" - ${testCase.description}`);
    
    // Run the search
    const filteredData = getSortedAndFilteredData(
      guests,
      { key: 'name', direction: 'asc' }, // Default sort
      {}, // No filters
      null, // Show all statuses
      [testCase.term], // Single search term as an array
      '', // No active search term
      metricsData,
      'all', // Show all guest types
      nodeData
    );
    
    // Extract result IDs
    const resultIds = filteredData.map(guest => guest.id);
    
    // Validate expected IDs are included
    const expectedIds = testCase.expectIdsContaining;
    const missingIds = expectedIds.filter(id => !resultIds.includes(id));
    const unexpectedIds = resultIds.filter(id => !expectedIds.includes(id));
    
    const passed = missingIds.length === 0 && 
                  (unexpectedIds.length === 0 || expectedIds.length === 0 && resultIds.length === 0);
    
    if (passed) {
      console.log(`  ✅ PASSED - Found ${resultIds.length} guests as expected`);
      passCount++;
    } else {
      console.log(`  ❌ FAILED`);
      if (missingIds.length > 0) {
        console.log(`    Missing IDs: ${missingIds.join(', ')}`);
      }
      if (unexpectedIds.length > 0) {
        console.log(`    Unexpected IDs: ${unexpectedIds.join(', ')}`);
      }
      
      // Add to failures by category
      const category = testCase.category;
      if (!failures[category]) {
        failures[category] = [];
      }
      failures[category].push({
        term: testCase.term,
        description: testCase.description,
        expected: expectedIds,
        actual: resultIds,
        missing: missingIds,
        unexpected: unexpectedIds
      });
      
      failCount++;
    }
    
    console.log(''); // Empty line for readability
  });
  
  // Print summary
  console.log('===== TEST SUMMARY =====');
  console.log(`Total tests: ${testCases.length}`);
  console.log(`Passed: ${passCount} (${(passCount/testCases.length*100).toFixed(1)}%)`);
  console.log(`Failed: ${failCount} (${(failCount/testCases.length*100).toFixed(1)}%)`);
  
  // Print failures by category
  if (failCount > 0) {
    console.log('\n===== FAILURES BY CATEGORY =====');
    Object.keys(failures).forEach(category => {
      console.log(`\n${category} - ${failures[category].length} failures:`);
      failures[category].forEach(failure => {
        console.log(`  "${failure.term}" - ${failure.description}`);
        console.log(`    Expected: ${failure.expected.join(', ')}`);
        console.log(`    Actual: ${failure.actual.join(', ')}`);
      });
    });
    
    // Provide advice for fixing issues
    console.log('\n===== TROUBLESHOOTING =====');
    
    // Check specifically for 'pri' issues
    if (failures['ROLE STANDALONE']?.some(f => f.term === 'pri')) {
      console.log('\nIssue detected with "pri" searches:');
      console.log('1. Check if "pri" is being treated as a special case rather than a role indicator');
      console.log('2. Ensure proper handling of "pri" at the beginning of the search logic');
      console.log('3. Make sure word boundaries are properly enforced for "pri" matches');
    }
    
    // Check for single character issues
    if (Object.keys(failures).includes('SINGLE CHAR')) {
      console.log('\nIssue detected with single character searches:');
      console.log('1. Single character searches should perform a full text search across all fields');
      console.log('2. Check that single character logic runs BEFORE role-specific logic');
    }
  }
  
  return {
    totalTests: testCases.length,
    passed: passCount,
    failed: failCount,
    failures
  };
}

// Run the tests if executed directly
if (typeof require !== 'undefined' && require.main === module) {
  runExhaustiveTests();
}

module.exports = { runExhaustiveTests }; 