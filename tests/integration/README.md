# Integration Tests (Playwright)

End-to-end Playwright tests that validate critical user flows against a running Pulse instance.

## Architecture

```text
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────────┐
│  Playwright     │────▶│  Pulse Server    │────▶│  Mock GitHub API    │
│  (Browser UI)   │     │  (Test Instance) │     │  (Controlled        │
│                 │     │                  │     │   Responses)        │
└─────────────────┘     └──────────────────┘     └─────────────────────┘
```

## Test Scenarios

- `tests/00-diagnostic.spec.ts` — smoke test that the stack boots and the UI renders.
- `tests/01-core-e2e.spec.ts` — critical UI flows:
  - Bootstrap setup wizard (fresh instance)
  - Login + authenticated state
  - Alerts thresholds create/delete
  - Settings persistence across refresh
  - Add/delete a Proxmox node (test-only)

## Running Tests

### Local Development (Docker compose stack)
```bash
cd tests/integration
./scripts/setup.sh   # one-time (installs deps + builds docker images)
npm test
```

The docker-compose stack seeds a deterministic bootstrap token for first-run setup:
- Override via `PULSE_E2E_BOOTSTRAP_TOKEN`
- Default token value is defined in `tests/integration/docker-compose.test.yml`

Credentials used by the E2E suite can be overridden:
- `PULSE_E2E_USERNAME` (default `admin`)
- `PULSE_E2E_PASSWORD` (default `adminadminadmin`)
- `PULSE_E2E_ALLOW_NODE_MUTATION=1` to enable the optional "Add Proxmox node" test (disabled by default for safety)

### Run Against An Existing Pulse Instance
```bash
cd tests/integration
PULSE_E2E_SKIP_DOCKER=1 \
PULSE_BASE_URL='http://your-pulse-host:7655' \
PULSE_E2E_USERNAME='admin' \
PULSE_E2E_PASSWORD='adminadminadmin' \
npm test
```

If the instance is behind self-signed TLS:
```bash
PULSE_E2E_INSECURE_TLS=1 PULSE_E2E_SKIP_DOCKER=1 PULSE_BASE_URL='https://...' npm test
```

### CI Pipeline
- Core E2E flows run via `.github/workflows/test-e2e.yml`
- Update flow coverage remains in `.github/workflows/test-updates.yml`

## Test Data (Update Flow Only)

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

- ✅ Core E2E flows pass reliably in CI
- ✅ Update flow remains covered via API integration test + smoke UI check
