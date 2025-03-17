// Standalone test for 'pri' search
// Simple script to test that 'pri' search works properly

// Mock implementation of required functions
function matchesTerm(guest, termLower, nodeData) {
  // Prevent operations on undefined/null terms
  if (!termLower) return true;
  
  // Handle column-specific searches
  if (termLower.includes(':')) {
    const [prefix, value] = termLower.split(':', 2);
    
    if (prefix.trim().toLowerCase() === 'role') {
      const roleValue = (value || '').trim().toLowerCase();
      
      // Non-shared checks
      if (roleValue === '-' || roleValue === 'none') {
        return !guest.shared;
      }
      
      // Need to be shared for other role searches
      if (!guest.shared) return false;
      
      const isPrimary = guest.primaryNode === guest.node;
      
      // Primary checks
      if (roleValue === 'p' || roleValue.startsWith('pri') || roleValue === 'primary') {
        console.log(`Column search ${termLower} for ${guest.id}: isPrimary=${isPrimary}`);
        return isPrimary;
      }
      
      // Secondary checks
      if (roleValue === 's' || roleValue.startsWith('sec') || roleValue === 'secondary') {
        return !isPrimary;
      }
      
      return false;
    }
    
    // Unknown column, just do text search
    return getFullSearchableText(guest).includes(termLower);
  }
  
  // Standard role terminology
  if (termLower === 'primary' || termLower === 'pri') {
    console.log(`Standard term "${termLower}" check for ${guest.id}: shared=${guest.shared}, isPrimary=${guest.primaryNode === guest.node}`);
    if (!guest.shared) return false;
    return guest.primaryNode === guest.node;
  }
  
  // Secondary role terms
  if (termLower === 'secondary' || termLower === 'sec') {
    if (!guest.shared) return false;
    return guest.primaryNode !== guest.node;
  }
  
  // Single letter searches
  if (termLower.length === 1) {
    const searchText = getFullSearchableText(guest);
    console.log(`Single char search "${termLower}" for ${guest.id} in: "${searchText}"`);
    return searchText.includes(termLower);
  }
  
  // Default text search
  return getFullSearchableText(guest).includes(termLower);
}

function getFullSearchableText(guest) {
  // Build full searchable text
  const searchableFields = [
    guest.name || '',
    guest.id || '',
    guest.status || '',
    guest.type === 'qemu' ? 'vm virtual machine' : 'ct container',
    guest.node || '',
    guest.shared ? (guest.primaryNode === guest.node ? 'primary pri p' : 'secondary sec s') : '',
    guest.shared ? 'shared role' : 'none'
  ];
  
  return searchableFields.join(' ').toLowerCase();
}

// Mock guests
const guests = [
  {
    id: '101',
    name: 'web-server-primary',
    node: 'node1',
    shared: true,
    primaryNode: 'node1', // Primary role on current node
    status: 'running',
    type: 'qemu'
  },
  {
    id: '102',
    name: 'db-primary',
    node: 'node1',
    shared: true,
    primaryNode: 'node1', // Primary role on current node
    status: 'running',
    type: 'qemu'
  },
  {
    id: '201',
    name: 'web-server-secondary',
    node: 'node2',
    shared: true,
    primaryNode: 'node1', // Secondary role on current node (node2)
    status: 'running',
    type: 'qemu'
  },
  {
    id: '202',
    name: 'db-secondary',
    node: 'node2',
    shared: true,
    primaryNode: 'node1', // Secondary role on current node (node2)
    status: 'running',
    type: 'qemu'
  },
  {
    id: '301',
    name: 'standalone-app',
    node: 'node3',
    shared: false, // Not shared
    status: 'running',
    type: 'qemu'
  },
  {
    id: '302',
    name: 'prince-app', // Has "pri" in the name, but not primary
    node: 'node3',
    shared: false,
    status: 'running',
    type: 'qemu'
  }
];

// Run tests for various search terms
function runTest(searchTerm) {
  console.log(`\n===== TESTING SEARCH FOR: "${searchTerm}" =====`);
  
  const results = guests.filter(guest => {
    const matches = matchesTerm(guest, searchTerm.toLowerCase(), []);
    const searchableText = getFullSearchableText(guest);
    
    console.log(`Guest ${guest.id} (${guest.name}): ${matches ? 'MATCH' : 'NO MATCH'}`);
    console.log(`  Shared: ${guest.shared}, Primary node: ${guest.primaryNode}, Current node: ${guest.node}`);
    console.log(`  Is Primary on this node? ${guest.shared && guest.primaryNode === guest.node}`);
    console.log(`  Searchable text: "${searchableText}"`);
    console.log(`  Contains 'pri'? ${searchableText.includes('pri')}`);
    console.log(`  Contains 'primary'? ${searchableText.includes('primary')}`);
    
    return matches;
  });
  
  console.log(`\nRESULTS FOR "${searchTerm}":`);
  console.log(`  Found ${results.length} matches:`);
  
  if (results.length > 0) {
    results.forEach(r => console.log(`  - ${r.id}: ${r.name}`));
  } else {
    console.log('  NO MATCHES');
  }
  
  // Check expectation - primary search should find primary nodes
  const expectedPrimaries = guests.filter(g => g.shared && g.primaryNode === g.node);
  if (searchTerm === 'pri' || searchTerm === 'primary' || searchTerm === 'role:pri' || searchTerm === 'role:primary') {
    const allPrimariesFound = expectedPrimaries.every(p => results.some(r => r.id === p.id));
    const onlyPrimariesFound = results.every(r => expectedPrimaries.some(p => p.id === r.id));
    
    console.log(`\nTEST RESULTS:`);
    console.log(`  Expected ${expectedPrimaries.length} primaries: ${expectedPrimaries.map(p => p.id).join(', ')}`);
    console.log(`  All primary nodes found? ${allPrimariesFound ? 'YES ✅' : 'NO ❌'}`);
    console.log(`  Only primary nodes found? ${onlyPrimariesFound ? 'YES ✅' : 'NO ❌'}`);
    
    if (!allPrimariesFound || !onlyPrimariesFound) {
      console.log(`\n❌ TEST FAILED`);
    } else {
      console.log(`\n✅ TEST PASSED`);
    }
  }
}

console.log("🔍 TESTING 'PRI' SEARCH FUNCTIONALITY");
console.log("===================================");

runTest('pri');
runTest('primary');
runTest('role:pri');
runTest('role:primary');
runTest('p'); // Should match primary guests and others containing 'p'
runTest('prince'); // Should match guest 302 ("prince-app") 