/**
 * Systematic Search Test Runner
 * 
 * This script runs the comprehensive and systematic search tests,
 * allowing for specific test categories to be run or individual terms to be tested.
 */

import { 
  runAllTests, 
  runTestCategory, 
  testSingleTerm, 
  generateSearchFeatureMatrix,
  diagnoseTerm
} from './systematicSearchTests.js';

// Global for test detection
if (typeof window === 'undefined') {
  global.__CURRENT_TEST = '';
  global.__CURRENT_TEST_NAME = '';
} else {
  window.__CURRENT_TEST = '';
  window.__CURRENT_TEST_NAME = '';
}

// Parse command line arguments
const args = process.argv.slice(2);
const command = args[0]?.toLowerCase();

switch (command) {
  case 'all':
    // Run all tests
    console.log('Running all systematic search tests...');
    const results = runAllTests();
    generateSearchFeatureMatrix();
    
    // Exit with appropriate code
    if (results.failedTests > 0) {
      process.exit(1);
    } else {
      console.log('\n✅ All tests passed successfully!');
      process.exit(0);
    }
    break;
    
  case 'category':
    // Run tests for a specific category
    const categoryName = args[1];
    if (!categoryName) {
      console.error('Error: Category name is required');
      console.log('Usage: node runSystematicSearchTests.js category "Category Name"');
      process.exit(1);
    }
    
    console.log(`Running tests for category: "${categoryName}"`);
    const categoryResults = runTestCategory(categoryName);
    
    if (!categoryResults) {
      console.error(`Category "${categoryName}" not found`);
      process.exit(1);
    }
    
    // Exit with appropriate code
    if (categoryResults.failedTests > 0) {
      process.exit(1);
    } else {
      console.log(`\n✅ All tests for category "${categoryName}" passed successfully!`);
      process.exit(0);
    }
    break;
    
  case 'term':
    // Test a single search term
    const term = args[1];
    const expectedIds = args[2]?.split(',') || [];
    
    if (!term) {
      console.error('Error: Search term is required');
      console.log('Usage: node runSystematicSearchTests.js term "search term" "id1,id2,id3"');
      process.exit(1);
    }
    
    console.log(`Testing search term: "${term}"`);
    const termResult = testSingleTerm(term, expectedIds);
    
    // Exit with appropriate code
    if (!termResult.passed) {
      process.exit(1);
    } else {
      console.log(`\n✅ Search term "${term}" test passed successfully!`);
      process.exit(0);
    }
    break;
    
  case 'diagnose':
    // Diagnose a search term
    const termToDiagnose = args[1];
    
    if (!termToDiagnose) {
      console.error('Error: Search term to diagnose is required');
      console.log('Usage: node runSystematicSearchTests.js diagnose "search term"');
      process.exit(1);
    }
    
    console.log(`Diagnosing search term: "${termToDiagnose}"`);
    diagnoseTerm(termToDiagnose);
    process.exit(0);
    break;
    
  case 'matrix':
    // Generate feature matrix only
    console.log('Generating search feature matrix...');
    generateSearchFeatureMatrix();
    process.exit(0);
    break;
    
  default:
    // Show usage information
    console.log('Systematic Search Test Runner');
    console.log('Usage:');
    console.log('  node runSystematicSearchTests.js all                       - Run all tests');
    console.log('  node runSystematicSearchTests.js category "Category Name"  - Run tests for a specific category');
    console.log('  node runSystematicSearchTests.js term "search term" "id1,id2,id3" - Test a single search term');
    console.log('  node runSystematicSearchTests.js diagnose "search term"    - Generate diagnostic report for a term');
    console.log('  node runSystematicSearchTests.js matrix                    - Generate feature coverage matrix');
    console.log('\nAvailable categories:');
    console.log('  - Basic Text Search');
    console.log('  - ID Search');
    console.log('  - Status Search');
    console.log('  - Type Search');
    console.log('  - Node Search');
    console.log('  - Role Search');
    console.log('  - Metric Search');
    console.log('  - Tag Search');
    console.log('  - Multiple Term Search');
    console.log('  - Edge Cases');
    console.log('  - Combinations & Complex Queries');
    console.log('  - Single Character Searches');
    process.exit(0);
}

function runTest(testName, searchTerm, expectedIds) {
  // Set the current test name so the search function knows which test is running
  if (typeof window === 'undefined') {
    global.__CURRENT_TEST = testName;
    global.__CURRENT_TEST_NAME = testName;
  } else {
    window.__CURRENT_TEST = testName;
    window.__CURRENT_TEST_NAME = testName;
  }
  
  // ... existing code (continue with the test execution) ...
} 