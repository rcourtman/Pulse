// Debug script for testing 'pri' search issue

// Mock implementation of matchesTerm function for testing
function matchesTerm(guest, termLower, nodeData) {
  // Prevent operations on undefined/null terms
  if (!termLower) return true;
  
  // CASE 3: Standard role terminology
  // Matches exact role terms (not as part of other words) using standard terminology
  if (termLower === 'shared' || termLower === 'role') {
    return !!guest.shared;
  }
  
  // Primary role terms: match whole words only, requires shared=true and isPrimary=true
  if (termLower === 'primary' || termLower === 'pri') {
    if (!guest.shared) return false;
    console.log(`Checking primary for ${guest.id}: primaryNode=${guest.primaryNode}, node=${guest.node}, matches=${guest.primaryNode === guest.node}`);
    return guest.primaryNode === guest.node;
  }
  
  // Secondary role terms: match whole words only, requires shared=true and isPrimary=false
  if (termLower === 'secondary' || termLower === 'sec') {
    if (!guest.shared) return false;
    return guest.primaryNode !== guest.node;
  }
  
  // CASE 5: Single character searches (including role abbreviations)
  if (termLower.length === 1) {
    const searchText = getFullSearchableText(guest, nodeData);
    console.log(`Single-char search for ${termLower} in ${guest.id}: "${searchText}"`);
    return searchText.includes(termLower);
  }
  
  // Default: search in all text fields
  const searchText = getFullSearchableText(guest, nodeData);
  console.log(`Full search for ${termLower} in ${guest.id}: "${searchText}"`);
  return searchText.includes(termLower);
}

// Improved function to get complete searchable text for a guest
function getFullSearchableText(guest, nodeData) {
  // Include ALL searchable properties
  const nodeName = guest.node || '';
  
  // Build full searchable text by concatenating ALL searchable fields
  const searchableFields = [
    guest.name || '',
    guest.id || '',
    guest.status || '',
    // Add VM/CT descriptive terms
    guest.type === 'qemu' ? 'vm virtual machine' : 'ct container',
    nodeName,
    // Add role descriptive terms if shared
    guest.shared ? (guest.primaryNode === guest.node ? 'primary pri p' : 'secondary sec s') : '',
    // Add shared indicator if applicable
    guest.shared ? 'shared role' : 'none',
    // Any other custom properties that should be searchable
    guest.description || '',
    guest.tags || ''
  ];
  
  // Join all fields with spaces and convert to lowercase
  const fullSearchText = searchableFields.join(' ').toLowerCase();
  return fullSearchText;
}

// Mock test data - simplified version of actual app data
const testGuests = [
  { 
    id: '101', 
    name: 'web-server',
    status: 'running',
    type: 'qemu',
    node: 'pve-prod-01',
    shared: true,
    primaryNode: 'pve-prod-01'  // This is primary on this node
  },
  { 
    id: '102', 
    name: 'database',
    status: 'running',
    type: 'qemu',
    node: 'pve-prod-01',
    shared: true,
    primaryNode: 'pve-prod-01'  // This is primary on this node
  },
  { 
    id: '201', 
    name: 'cache-server',
    status: 'stopped',
    type: 'qemu',
    node: 'pve-prod-02',
    shared: true,
    primaryNode: 'pve-prod-01'  // This is secondary on this node
  }
];

// Mock node data
const nodeData = [
  { id: 'pve-prod-01', name: 'Production Node 1' },
  { id: 'pve-prod-02', name: 'Production Node 2' }
];

// Test specific search terms directly
function testSearch(searchTerm) {
  console.log(`\n===== TESTING SEARCH TERM: "${searchTerm}" =====`);
  
  const result = testGuests.filter(guest => {
    const matches = matchesTerm(guest, searchTerm.toLowerCase(), nodeData);
    console.log(`  Guest ${guest.id} (${guest.name}) matches ${searchTerm}? ${matches ? 'YES' : 'NO'}`);
    return matches;
  });
  
  console.log(`\nResults for "${searchTerm}":`);
  console.log(`  Found ${result.length} guests:`);
  
  if (result.length === 0) {
    console.log("  NO RESULTS FOUND!");
  } else {
    result.forEach(guest => {
      console.log(`  - ${guest.id}: ${guest.name} (${guest.node}, shared=${guest.shared}, primaryNode=${guest.primaryNode})`);
      console.log(`    Primary? ${guest.primaryNode === guest.node ? 'YES' : 'NO'}`);
    });
  }
  
  // Print expected vs actual for primaries
  const expectedPrimaries = testGuests.filter(g => g.shared && g.primaryNode === g.node);
  console.log(`\nExpected primaries: ${expectedPrimaries.length} guests`);
  expectedPrimaries.forEach(g => console.log(`  - ${g.id}: ${g.name}`));
  
  // Verify if all primaries were found
  const allPrimariesFound = expectedPrimaries.every(
    expected => result.some(res => res.id === expected.id)
  );
  
  console.log(`\nAll primaries found? ${allPrimariesFound ? 'YES ✅' : 'NO ❌'}`);
  
  if (!allPrimariesFound) {
    console.log("Missing primaries:");
    expectedPrimaries.forEach(expected => {
      if (!result.some(res => res.id === expected.id)) {
        console.log(`  - ${expected.id}: ${expected.name}`);
      }
    });
  }
}

// Test various forms of the primary search
console.log("\n********** DEBUGGING 'PRI' SEARCH ISSUE **********");
testSearch('pri');
testSearch('primary');
testSearch('p'); // Single character test

// Test role-prefixed searches to check if they work differently
console.log("\n********** TESTING PREFIXED SEARCHES **********");
testSearch('role:pri');
testSearch('role:primary');

console.log("\n\n********** SEARCH IMPLEMENTATION DETAILS **********");
// Print the relevant implementation from networkUtils.js
console.log(`
Search logic for 'pri' is implemented in networkUtils.js:

// Standard role terminology
if (termLower === 'primary' || termLower === 'pri') {
  if (!guest.shared) return false;
  return guest.primaryNode === guest.node;
}

and in getFullSearchableText:

guest.shared ? (guest.primaryNode === guest.node ? 'primary pri p' : 'secondary sec s') : '',
`); 