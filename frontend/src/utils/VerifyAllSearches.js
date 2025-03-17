/**
 * Comprehensive Search Verification Script
 * 
 * This script runs a battery of tests to verify that ALL search patterns work correctly.
 * It combines both standard tests and special test cases for edge conditions.
 */

import { runSearchTests } from './searchTests.js';

console.log('===============================================================');
console.log('= COMPREHENSIVE SEARCH VERIFICATION                           =');
console.log('===============================================================');
console.log('This script verifies ALL search patterns work correctly, including:');
console.log('- Basic text searches');
console.log('- Role-specific searches (pri, sec, primary, secondary)');
console.log('- Column-based searches (role:pri, type:vm, etc.)');
console.log('- Single character searches (p, s, v, etc.)');
console.log('- Edge cases and potentially problematic patterns');
console.log('\nRunning standard search test suite...');

// Execute all standard tests
const results = runSearchTests();

// Output summary
console.log('\n===============================================================');
console.log('= VERIFICATION RESULTS                                        =');
console.log('===============================================================');

if (results.general.failed === 0 && results.role.filter(r => !r.passed).length === 0) {
  console.log('✅ ALL TESTS PASSED!');
  console.log(`  - ${results.general.passed} general tests passed`);
  console.log(`  - ${results.role.filter(r => r.passed).length} role-specific tests passed`);
  console.log('\nThe search functionality is working correctly for all patterns.');
  console.log('Key validations:');
  console.log('  - "pri" correctly returns primary guests');
  console.log('  - "sec" correctly returns secondary guests');
  console.log('  - Single-character searches work properly');
  console.log('  - Combined searches apply AND logic correctly');
} else {
  console.log('❌ SOME TESTS FAILED!');
  console.log(`  - ${results.general.failed} general tests failed`);
  console.log(`  - ${results.role.filter(r => !r.passed).length} role-specific tests failed`);
  
  // Show detailed failures
  console.log('\nFailed tests:');
  if (results.general.failed > 0) {
    results.general.details
      .filter(detail => !detail.passed)
      .forEach(detail => {
        console.log(`  - ${detail.name}: ${detail.description}`);
        console.log(`    Expected: ${JSON.stringify(detail.expectedIds)}`);
        console.log(`    Actual:   ${JSON.stringify(detail.actualIds)}`);
      });
  }
  
  const failedRoleTests = results.role.filter(r => !r.passed);
  if (failedRoleTests.length > 0) {
    console.log('\nFailed role-specific tests:');
    failedRoleTests.forEach(r => {
      console.log(`  - "${r.term}": Expected ${JSON.stringify(r.expectedIds)}, got ${JSON.stringify(r.resultIds)}`);
    });
  }
}

// Execution instructions
console.log('\n===============================================================');
console.log('= HOW TO USE THIS VERIFICATION TOOL                          =');
console.log('===============================================================');
console.log('Run this script after making any changes to the search functionality:');
console.log('  node frontend/src/utils/VerifyAllSearches.js');
console.log('\nIf you add new search capabilities, update searchTests.js to include tests');
console.log('for the new functionality.');

// Exit with appropriate code
if (results.general.failed === 0 && results.role.filter(r => !r.passed).length === 0) {
  process.exit(0);
} else {
  process.exit(1);
} 