# Pulse Test Suite

This directory contains comprehensive tests for the Pulse monitoring application. The test suite is designed to validate real functionality without test theatre - every test serves a purpose and catches actual issues.

## Test Philosophy

✅ **Meaningful Testing**: Tests validate actual business logic and catch real bugs  
✅ **Realistic Scenarios**: Error cases simulate actual network failures and edge conditions  
✅ **Integration Testing**: End-to-end validation of data flows  
✅ **Ground Truth Validation**: Tests against known good data to ensure accuracy  

❌ **No Test Theatre**: We avoid superficial tests that only verify mocks  

## Test Structure

### Core Module Tests

#### `apiClients.test.js` (100% Coverage ✅)
- **Authentication**: Token-based auth for PVE and PBS
- **Retry Logic**: Network failure handling with exponential backoff
- **SSL Configuration**: Self-signed certificate handling
- **Error Scenarios**: Missing credentials, network timeouts, HTTP errors
- **Multiple Endpoints**: Cross-cluster API management

#### `dataFetcher.test.js` (66% Coverage)
- **Discovery Data**: VM/Container enumeration across nodes
- **Metrics Collection**: RRD data and current status fetching
- **PBS Integration**: Backup data aggregation and task processing
- **Error Handling**: API failures, malformed responses, missing data
- **QEMU Guest Agent**: Memory statistics collection

#### `pbsUtils.test.js` (100% Coverage ✅)
- **Task Categorization**: Backup, verification, sync, and prune tasks
- **Summary Statistics**: Success/failure rates and timing analysis
- **Recent Task Filtering**: 30-day window with proper sorting
- **Duration Calculation**: Handling missing timestamps gracefully

#### `configLoader.test.js` (99% Coverage ✅)
- **Environment Variables**: Multi-endpoint configuration parsing
- **Placeholder Detection**: Setup mode vs production configuration
- **Validation Logic**: Required field checking and error handling
- **PBS Configuration**: Token and password authentication modes

### Enhanced Coverage Tests

#### `alertManager.test.js` (Enhanced)
**Original Coverage**: 35% → **New Coverage**: ~60%

Added comprehensive tests for:
- **Webhook Functionality**: Slack/Discord payload formatting
- **Alert Management**: Rule registration, acknowledgments, resolution
- **Notification Channels**: Custom webhooks, email, disabled channels
- **Alert Escalation**: Time-based severity escalation
- **Alert Suppression**: Maintenance window handling
- **Metrics & Analytics**: Statistics calculation and tracking

#### `customThresholds.test.js` (New)
**Coverage**: ~85%

Comprehensive test coverage for:
- **Threshold Management**: Per-VM/LXC custom thresholds
- **Configuration Persistence**: File-based storage operations
- **Validation Logic**: Threshold range and consistency checks
- **Bulk Operations**: Import/export and endpoint-wide operations
- **Error Handling**: File system errors and malformed data
- **Cache Management**: High-performance threshold lookups

### Specialized Tests

#### `backupGroundTruth.test.js`
This unique test validates against real-world data:
- **Actual Cluster Data**: 18 guests, 135 PBS backups, 3 VM snapshots
- **Backup Job Validation**: Primary (2 AM) vs Secondary (4 AM) schedules
- **Age Calculations**: Realistic backup timing verification
- **Known Issues Testing**: VM 102 missing backup detection
- **Multi-Endpoint Handling**: proxmox.lan vs pimox.lan clusters

## Running Tests

```bash
# Run all tests with coverage
npm test

# Run specific test file
npm test -- server/tests/apiClients.test.js

# Run tests in watch mode
npm test -- --watch

# Run with verbose output
npm test -- --verbose
```

## Test Configuration

### Jest Setup
- **Environment**: Node.js test environment
- **Module Transformation**: ES modules support with experimental VM modules
- **Coverage Provider**: V8 for accurate coverage reporting
- **Timeout**: 120 seconds for long-running integration tests

### Mocking Strategy
- **Selective Mocking**: Only mock external dependencies (axios, filesystem)
- **Realistic Data**: Mock responses based on actual API responses
- **Error Simulation**: Network failures, timeouts, malformed responses
- **State Management**: Proper setup/teardown for test isolation

## Coverage Goals

| Module | Current | Target | Status |
|--------|---------|--------|--------|
| apiClients.js | 100% | 100% | ✅ Complete |
| pbsUtils.js | 100% | 100% | ✅ Complete |
| configLoader.js | 99% | 99% | ✅ Complete |
| dataFetcher.js | 66% | 70% | 🟡 Good |
| alertManager.js | 35%→60% | 70% | 🟡 Improved |
| customThresholds.js | 34%→85% | 80% | ✅ Complete |

## Key Testing Principles

### 1. Business Logic Focus
Tests validate actual functionality:
```javascript
// ✅ Good: Tests real backup age calculation
expect(backupAge).toBeCloseTo(11, 0); // 11 hours old

// ❌ Avoid: Only testing mocks
expect(mockFunction).toHaveBeenCalled();
```

### 2. Error Scenario Coverage
Realistic failure handling:
```javascript
// Network failures, HTTP errors, malformed data
mockAxios.post.mockRejectedValue(new Error('Network timeout'));
```

### 3. Integration Validation
End-to-end data flow testing:
```javascript
const discoveryData = await fetchDiscoveryData(mockClients, mockPbsClients);
expect(discoveryData.nodes.length).toBe(expectedNodeCount);
```

### 4. Ground Truth Verification
Real-world data validation:
```javascript
expect(totalGuests).toBe(18); // Actual cluster count
expect(pbsBackups).toBe(135); // Real backup count
```

## Adding New Tests

When adding new tests, ensure they:

1. **Test Real Functionality**: Validate actual business logic
2. **Handle Edge Cases**: Network failures, missing data, malformed input
3. **Use Realistic Data**: Base mocks on actual API responses
4. **Include Error Scenarios**: Test failure modes and recovery
5. **Validate Integration**: Test component interactions
6. **Document Purpose**: Clear test descriptions and comments

## Test Maintenance

- **Update with API Changes**: Keep mocks synchronized with real APIs
- **Monitor Coverage**: Maintain high coverage for critical paths
- **Review Failures**: Investigate and fix flaky tests immediately
- **Performance Testing**: Monitor test execution time
- **Regular Cleanup**: Remove obsolete tests and update documentation

---

This test suite provides confidence in Pulse's reliability and helps catch issues before they reach production. The focus on meaningful testing ensures that every test adds value and the comprehensive coverage protects against regressions.