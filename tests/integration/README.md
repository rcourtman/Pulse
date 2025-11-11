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

1. **Happy Path**: Valid checksums, successful update
2. **Bad Checksums**: Server rejects update, UI shows error once (not twice)
3. **Rate Limiting**: Multiple rapid requests are throttled gracefully
4. **Network Failure**: UI retries with exponential backoff
5. **Stale Release**: Backend refuses to install flagged releases

## Frontend Validation

- UpdateProgressModal appears exactly once
- Error messages are user-friendly (not raw API errors)
- Modal can be dismissed after error
- No duplicate modals on error

## Running Tests

### Local Development
```bash
# Start test environment
cd tests/integration
docker-compose up -d

# Run tests
npm test

# View logs
docker-compose logs -f pulse-test
docker-compose logs -f mock-github

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
