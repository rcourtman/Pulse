#!/usr/bin/env node

/**
 * Simple test for regex patterns
 */

// Define the regex patterns
const resourceExpressionRegex = /^(cpu|mem(ory)?|disk|network|net)\s*(>|<|>=|<=|=)\s*(\d+)$/i;
const directExpressionRegex = /^(cpu|mem(ory)?|disk|network|net)(>|<|>=|<=|=)(\d+)$/i;
const spacedExpressionRegex = /^(cpu|mem(ory)?|disk|network|net)\s+([<>]=?|=)\s+(\d+)$/i;

// Test terms
const testTerms = [
  'cpu>50',
  'cpu > 50',
  'memory>75',
  'memory > 75',
  'disk>90',
  'disk > 90'
];

// Test each term against each regex
console.log('Testing regex patterns:');
console.log('=======================');

testTerms.forEach(term => {
  console.log(`\nTerm: "${term}"`);
  console.log(`- resourceExpressionRegex: ${resourceExpressionRegex.test(term)}`);
  console.log(`- directExpressionRegex: ${directExpressionRegex.test(term)}`);
  console.log(`- spacedExpressionRegex: ${spacedExpressionRegex.test(term)}`);
  
  // Show match groups for spacedExpressionRegex
  if (spacedExpressionRegex.test(term)) {
    const match = term.match(spacedExpressionRegex);
    console.log('  spacedExpressionRegex groups:', match);
  }
});

// Create a mock implementation of the matchesTerm function
function mockMatchesTerm(term) {
  console.log(`\nTesting term: "${term}"`);
  
  // Try matching with spaced regex
  const spacedMatch = term.match(spacedExpressionRegex);
  if (spacedMatch) {
    console.log(`SPACED MATCH FOUND for "${term}"`);
    console.log('Match groups:', spacedMatch);
    
    let resource = spacedMatch[1].toLowerCase();
    if (resource === 'mem') resource = 'memory';
    
    const operator = spacedMatch[3]; // Get the operator from the capture group
    const value = parseFloat(spacedMatch[4]);
    
    console.log(`Resource: ${resource}, Operator: ${operator}, Value: ${value}`);
    return true;
  }
  
  console.log('No spaced match found');
  return false;
}

console.log('\n\nTesting mock implementation:');
console.log('============================');

testTerms.forEach(term => {
  const result = mockMatchesTerm(term);
  console.log(`Result for "${term}": ${result}`);
}); 