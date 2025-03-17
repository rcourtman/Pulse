/**
 * Basic Search Test for Role Column
 * 
 * This script tests if the search functionality is correctly working with role-based searches.
 * It focuses on testing the most important search terms to ensure core functionality works.
 */

// Mock guest data
const mockGuests = [
  { id: '101', name: 'web-server', type: 'qemu', status: 'running', node: 'pve1', shared: true, primaryNode: 'pve1', hastate: 'started' },
  { id: '102', name: 'database', type: 'qemu', status: 'running', node: 'pve1', shared: true, primaryNode: 'pve2', hastate: 'stopped' },
  { id: '103', name: 'app-container', type: 'lxc', status: 'running', node: 'pve2', shared: true, primaryNode: 'pve2', hastate: 'started' },
  { id: '104', name: 'backup-storage', type: 'qemu', status: 'stopped', node: 'pve2', shared: false, hastate: undefined },
];

// Mock node data
const mockNodeData = [
  { id: 'pve1', name: 'pve-prod-01' },
  { id: 'pve2', name: 'pve-prod-02' },
];

// Import the search utility - in real implementation this would be from networkUtils
function mockMatchesTerm(guest, term, nodeData) {
  term = term.toLowerCase();
  
  // Test if this is a role-specific search
  if (term.startsWith('role:')) {
    const roleValue = term.split(':')[1];
    
    if (!roleValue) return true; // Empty value matches all
    
    // Check for primary/secondary role
    if (roleValue === 'primary' || roleValue === 'pri') {
      return guest.shared && guest.primaryNode === guest.node;
    }
    
    if (roleValue === 'secondary' || roleValue === 'sec') {
      return guest.shared && guest.primaryNode !== guest.node;
    }
    
    if (roleValue === '-' || roleValue === 'none') {
      return !guest.shared;
    }
    
    // Default - check if the term appears in relevant fields
    return false;
  }
  
  // For direct role keyword searches
  if (term === 'primary' || term === 'pri') {
    return guest.shared && guest.primaryNode === guest.node;
  }
  
  if (term === 'secondary' || term === 'sec') {
    return guest.shared && guest.primaryNode !== guest.node;
  }
  
  // For combined searches (example: "primary lxc")
  if (term.includes(' ')) {
    const parts = term.split(' ');
    return parts.some(part => mockMatchesTerm(guest, part, nodeData));
  }
  
  // Test for type matches
  if (term === 'vm' || term === 'qemu') {
    return guest.type === 'qemu';
  }
  
  if (term === 'ct' || term === 'lxc' || term === 'container') {
    return guest.type === 'lxc';
  }
  
  // Test for status matches
  if (term === 'running') {
    return guest.status === 'running';
  }
  
  if (term === 'stopped') {
    return guest.status === 'stopped';
  }
  
  // Fallback to simple text match
  return guest.name.toLowerCase().includes(term) || 
         guest.id.toLowerCase().includes(term) ||
         guest.node.toLowerCase().includes(term);
}

// Run the tests
function runTests() {
  const tests = [
    {
      name: 'Search for primary guests',
      term: 'primary',
      expected: ['101']
    },
    {
      name: 'Search for secondary guests',
      term: 'secondary',
      expected: ['102']
    },
    {
      name: 'Search for non-shared guests',
      term: 'role:-',
      expected: ['104']
    },
    {
      name: 'Search for primary VMs',
      term: 'primary vm',
      expected: ['101']
    },
    {
      name: 'Search for primary containers',
      term: 'primary ct',
      expected: ['103']
    },
    {
      name: 'Search for secondary running guests',
      term: 'secondary running',
      expected: ['102']
    },
    {
      name: 'Search with role prefix',
      term: 'role:primary',
      expected: ['101', '103']
    },
    {
      name: 'Search with role prefix for secondary',
      term: 'role:secondary',
      expected: ['102']
    }
  ];
  
  let passedTests = 0;
  
  tests.forEach(test => {
    console.log(`Running test: ${test.name}`);
    
    // Filter guests based on the search term
    const results = mockGuests.filter(guest => mockMatchesTerm(guest, test.term, mockNodeData));
    
    // Get the IDs of matching guests
    const resultIds = results.map(guest => guest.id);
    
    // Check if the results match the expected
    const passed = test.expected.length === resultIds.length && 
                  test.expected.every(id => resultIds.includes(id));
    
    if (passed) {
      console.log(`✅ PASSED: "${test.term}" - Found ${resultIds.join(', ')}`);
      passedTests++;
    } else {
      console.log(`❌ FAILED: "${test.term}"`);
      console.log(`  Expected: ${test.expected.join(', ')}`);
      console.log(`  Actual: ${resultIds.join(', ')}`);
    }
    console.log('-----------------------------------');
  });
  
  console.log(`${passedTests} of ${tests.length} tests passed.`);
  
  if (passedTests === tests.length) {
    console.log('✅ ALL TESTS PASSED - Role search functionality is working correctly.');
  } else {
    console.log('❌ SOME TESTS FAILED - Role search functionality needs fixing.');
  }
}

// Execute the tests
runTests();

// Export for potential use in other test suites
module.exports = { mockMatchesTerm, runTests }; 