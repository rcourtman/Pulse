/**
 * Test Runner for Search Functionality Tests
 * Run with: node runSearchTests.js
 */

require('@babel/register')({
  presets: ['@babel/preset-env'],
  plugins: [
    '@babel/plugin-transform-modules-commonjs'
  ]
});

// Import and run the tests
const { runSearchTests } = require('./searchTests');
runSearchTests(); 