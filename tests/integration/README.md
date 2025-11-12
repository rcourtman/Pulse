# Update Integration Tests

End-to-end tests for the Pulse update flow, validating the entire path from UI to backend.

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────────┐
│  Playwright     │────▶│  Pulse Server    │────▶│  Mock GitHub API    │
│  (Browser UI)   │     │  (Test Instance) │     │  (Controlled        │
│                 │     │                  │     │   Responses)        │
└─────────────────┘     └──────────────────┘     └─────────────────────┘
```

## Test Scenarios

> **Note:** The comprehensive Playwright update specs were removed on 2025‑11‑12 after repeated
> release-blocking flakes. We now rely on:
>
> 1. `tests/00-diagnostic.spec.ts` — ensures the containerized stack boots and the login page renders.
> 2. `tests/integration/api/update_flow_test.go` — drives the `/api/updates/*` endpoints directly to
>    verify the backend can discover, plan, apply, and complete an update.
>
> Reintroduce full UI coverage once we have deterministic fixtures and selectors for the update flow.

## Running Tests

### Local Development
```bash
# Start test environment
cd tests/integration
docker-compose up -d

# Run diagnostic Playwright test
npx playwright test tests/00-diagnostic.spec.ts

# Run API integration test from repo root
UPDATE_API_BASE_URL=http://localhost:7655 go test ./tests/integration/api -run TestUpdateFlowIntegration

# Cleanup
docker-compose down -v
```

### CI Pipeline
Tests run automatically on every PR touching update code via `.github/workflows/test-updates.yml`

## Test Data

The mock GitHub server (`mock-github-server/`) provides controllable responses:
- `/api/releases` - List all releases
- `/api/releases/latest` - Latest stable release
- `/download/{version}/pulse-{version}-linux-amd64.tar.gz` - Release tarballs
- `/download/{version}/checksums.txt` - Checksum files

Response behavior can be controlled via environment variables:
- `MOCK_CHECKSUM_ERROR=true` - Return invalid checksums
- `MOCK_NETWORK_ERROR=true` - Simulate network failures
- `MOCK_RATE_LIMIT=true` - Enable aggressive rate limiting
- `MOCK_STALE_RELEASE=true` - Mark releases as stale

## Success Criteria

- ✅ Tests run in CI on every PR touching update code
- ✅ All scenarios pass reliably
- ✅ Tests catch checksum validation issues automatically
- ✅ Frontend UX regressions are blocked
