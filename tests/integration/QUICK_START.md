# Quick Start Guide - Update Integration Tests

This guide will help you get the update integration tests running quickly.

## Prerequisites

- Docker and Docker Compose
- Node.js 18+ and npm
- Go 1.23+ (for building mock server)

## Setup (One-time)

```bash
cd tests/integration
./scripts/setup.sh
```text

This will:
- Install npm dependencies
- Install Playwright browsers
- Build Docker images for test environment

## Running Tests

### Run All Tests

```bash
npm test
```

This runs all test suites with appropriate configurations.

### Run Specific Test Suite

```bash
# Happy path only
./scripts/run-tests.sh happy

# Bad checksums
./scripts/run-tests.sh checksums

# Rate limiting
./scripts/run-tests.sh rate-limit

# Network failures
./scripts/run-tests.sh network

# Stale releases
./scripts/run-tests.sh stale

# Frontend validation
./scripts/run-tests.sh frontend
```

### Interactive Mode

```bash
# Open Playwright UI
npm run test:ui

# Debug mode
npm run test:debug

# Run in headed browser
npm run test:headed
```

## Manual Docker Control

```bash
# Start test environment
npm run docker:up

# View logs
npm run docker:logs

# Stop environment
npm run docker:down

# Rebuild images
npm run docker:rebuild
```

## Accessing Test Services

While the test environment is running:

- **Pulse UI**: <http://localhost:7655>
- **Mock GitHub API**: <http://localhost:8080>
- **Health checks**:
  - <http://localhost:7655/api/health>
  - <http://localhost:8080/health>

## Viewing Test Results

After running tests:

```bash
# View HTML report
npm run test:report

# Reports are saved to:
# - playwright-report/ (HTML report)
# - test-results/ (screenshots, videos)
```

## Test Scenarios

### 1. Diagnostic Smoke Test (`00-diagnostic.spec.ts`)
- Ensures the containerized stack boots and the UI renders.

### 2. Core E2E Flows (`01-core-e2e.spec.ts`)
- First-run setup wizard (fresh instance)
- Login/logout + authenticated state
- Alerts thresholds create/delete
- Settings persistence across refresh
- Add/delete a Proxmox node (test-only)

## Troubleshooting

### Tests failing to start

```bash
# Check Docker is running
docker ps

# Rebuild images
npm run docker:rebuild

# Check logs
npm run docker:logs
```

### Port conflicts

If ports 7655 or 8080 are in use:

```bash
# Find and stop conflicting processes
lsof -i :7655
lsof -i :8080
```

### Clean slate

```bash
# Remove all test containers and volumes
docker-compose -f docker-compose.test.yml down -v

# Clean Docker
docker system prune -f

# Reinstall
./scripts/setup.sh
```

## CI Integration

Tests run automatically on every PR that touches:
- `internal/updates/**`
- `internal/api/updates.go`
- `frontend-modern/src/components/Update*.tsx`
- `frontend-modern/src/api/updates.ts`
- `frontend-modern/src/stores/updates.ts`
- `tests/integration/**`

See `.github/workflows/test-updates.yml` for CI configuration.

## Success Criteria

✅ All test scenarios pass reliably
✅ Tests catch checksum validation issues (like v4.28.0)
✅ Frontend UX regressions are blocked
✅ Tests run in CI on every relevant PR

## Architecture

```text
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────────┐
│  Playwright     │────▶│  Pulse Server    │────▶│  Mock GitHub API    │
│  (Browser UI)   │     │  (Test Instance) │     │  (Controlled        │
│                 │     │                  │     │   Responses)        │
└─────────────────┘     └──────────────────┘     └─────────────────────┘
```

The mock GitHub server provides controllable responses for testing different scenarios via environment variables:
- `MOCK_CHECKSUM_ERROR=true` - Return invalid checksums
- `MOCK_NETWORK_ERROR=true` - Simulate network failures
- `MOCK_RATE_LIMIT=true` - Enable aggressive rate limiting
- `MOCK_STALE_RELEASE=true` - Mark releases as stale

## Writing New Tests

1. Add test file to `tests/` directory
2. Use helpers from `tests/helpers.ts`
3. Follow existing test patterns
4. Update `run-tests.sh` if new environment config needed
5. Update CI workflow if needed

Example:

```typescript
import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateToSettings } from './helpers';

test('my new test', async ({ page }) => {
  await ensureAuthenticated(page);
  await navigateToSettings(page);

  // Your test logic here
});
```

## Getting Help

- Check the [main README](./README.md) for detailed information
- Review existing test files for examples
- Check Docker logs for service issues
- Review Playwright documentation: <https://playwright.dev>
