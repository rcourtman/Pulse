# Systematic Search Test Suite

This directory contains a comprehensive, systematic test suite for verifying the search functionality in the NetworkUtils module. The test suite is designed to be thorough, covering all edge cases, boundary conditions, and search patterns to ensure the search functionality behaves as expected.

## Overview

The search test implementation follows a formal, methodical approach to ensure complete coverage of all search features. It tests:

- Basic text searching
- ID-based searching
- Status filtering (running, stopped, paused, suspended)
- Type filtering (VM, container)
- Node filtering
- Role searching (primary, secondary, non-shared)
- Metric-based filtering (CPU, memory, disk usage)
- Tag searching
- Multiple term combinations
- Edge cases
- Complex queries
- Single character searches

## Test Data

The test suite uses comprehensive mock data that represents various real-world scenarios:

- Guests with different statuses (running, stopped, paused, suspended)
- VMs and containers
- Shared and non-shared guests
- Primary and secondary nodes
- Guests with varying resource usage (high/low CPU, memory, disk)
- Guests with different tags and descriptions

## Running the Tests

The test suite provides several ways to run tests:

### Run All Tests

```bash
node frontend/src/utils/tests/runSystematicSearchTests.js all
```

This will run all test categories and generate a complete report.

### Run a Specific Test Category

```bash
node frontend/src/utils/tests/runSystematicSearchTests.js category "Category Name"
```

Available categories:
- Basic Text Search
- ID Search
- Status Search
- Type Search
- Node Search
- Role Search
- Metric Search
- Tag Search
- Multiple Term Search
- Edge Cases
- Combinations & Complex Queries
- Single Character Searches

### Test a Single Search Term

```bash
node frontend/src/utils/tests/runSystematicSearchTests.js term "search term" "id1,id2,id3"
```

This allows testing a specific search term against expected results.

### Diagnose a Search Term

```bash
node frontend/src/utils/tests/runSystematicSearchTests.js diagnose "search term"
```

This generates a detailed diagnostic report for a specific search term, showing which guests match and which don't.

### Generate Feature Matrix

```bash
node frontend/src/utils/tests/runSystematicSearchTests.js matrix
```

This generates a matrix of all search features with test counts for each feature.

## Test Structure

The test suite is organized by feature categories, with each category containing multiple test cases. Each test case specifies:

1. A search term or terms to test
2. The expected matching guest IDs
3. A description of what the test is verifying

Tests are executed systematically, with detailed reporting of any failures, including:
- Which specific tests failed
- What the expected vs. actual results were
- Which specific guests were missing or unexpected
- Details about the missing or unexpected guests

## Extending the Tests

To add new test cases:

1. Add a new test object to the appropriate category in the `testCategories` array
2. Specify the term, expectedIds, and description
3. If necessary, add new mock data to represent the scenario being tested

For new feature categories:

1. Add a new category object to the `testCategories` array
2. Add test cases to the new category
3. Update the documentation in this README and the runSystematicSearchTests.js help text

## Automated Integration

This test suite can be integrated into CI/CD pipelines by running the `all` command and checking the exit code (0 for success, 1 for failures). 