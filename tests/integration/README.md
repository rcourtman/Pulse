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
- `tests/02-navigation-perf.spec.ts` — route transition performance budgets for:
  - Infrastructure → Workloads
  - Workloads → Infrastructure
- `tests/06-theme-visual.spec.ts` — visual regression baselines for light/dark auth surfaces:
  - Logged-out login page (full page + form card)
  - Authenticated Settings → Authentication page
- `tests/07-trial-signup-return.spec.ts` — trial workflow contract:
  - Click "Start 14-day Pro Trial" redirects to hosted signup URL
  - Complete real Stripe sandbox checkout
  - Return to Pulse and verify real trial expiry/countdown state (not lifetime)
- `tests/08-cloud-hosting.spec.ts` — hosted cloud signup contract:
  - Public `/cloud/signup` form creates a real Stripe sandbox checkout session
  - Checkout completes and returns to hosted signup completion page
  - Magic-link request path succeeds via real public endpoint
- `tests/09-cloud-billing-lifecycle.spec.ts` — hosted cloud post-checkout lifecycle:
  - Replays verified Stripe webhook events into control plane
  - Asserts tenant activation after checkout event processing
  - Asserts tenant cancellation after subscription deletion event processing

## Running Tests

### Local Development (Docker compose stack)
```bash
cd tests/integration
./scripts/setup.sh   # one-time (installs deps + builds docker images)
npm test
```

### Eval Packs (No Manual Steps)
Run the curated scenario pack (multi-tenant, trial-signup, cloud-hosting, cloud-billing-lifecycle) and emit a report:
```bash
cd tests/integration
npm run evals
```

Filter to one scenario:
```bash
npm run evals -- --scenario trial-signup
```

Reports are written to:
- `tests/integration/eval-results/<timestamp>/report.json`
- `tests/integration/eval-results/<timestamp>/report.md`

Cloud lifecycle evals require these environment variables (test-mode only):
- `PULSE_CP_ADMIN_KEY`
- `PULSE_E2E_STRIPE_API_KEY`
- `PULSE_E2E_STRIPE_WEBHOOK_SECRET`

### Agentic Mode (External Browser Agent)
The eval runner supports external browser-capable agents through a command template:
```bash
cd tests/integration
PULSE_EVAL_MODE=agentic \
PULSE_EVAL_AGENT_COMMAND_TEMPLATE='<your-agent-command-using {{task_file}} and {{result_json}}>' \
npm run evals
```

Supported placeholders in `PULSE_EVAL_AGENT_COMMAND_TEMPLATE`:
- `{{task_file}}`
- `{{result_json}}`
- `{{scenario_id}}`
- `{{base_url}}`

The docker-compose stack seeds a deterministic bootstrap token for first-run setup:
- Override via `PULSE_E2E_BOOTSTRAP_TOKEN`
- Default token value is defined in `tests/integration/docker-compose.test.yml`

Credentials used by the E2E suite can be overridden:
- `PULSE_E2E_USERNAME` (default `admin`)
- `PULSE_E2E_PASSWORD` (default `adminadminadmin`)
- `PULSE_E2E_ALLOW_NODE_MUTATION=1` to enable the optional "Add Proxmox node" test (disabled by default for safety)
- `PULSE_E2E_PERF=1` to enable navigation performance budget checks
- `PULSE_E2E_PERF_ITERATIONS` (default `3`)
- `PULSE_E2E_PERF_INFRA_TO_WORKLOADS_BUDGET_MS` (default `2200`)
- `PULSE_E2E_PERF_WORKLOADS_TO_INFRA_BUDGET_MS` (default `2200`)

### Run Against An Existing Pulse Instance
```bash
cd tests/integration
PULSE_E2E_SKIP_DOCKER=1 \
PULSE_BASE_URL='http://your-pulse-host:7655' \
PULSE_E2E_USERNAME='admin' \
PULSE_E2E_PASSWORD='adminadminadmin' \
npm test
```

### Snapshot-Clean Proxmox LXC Trial SAT
For real trial workflow validation against a fresh LXC each run:

- Runbook: `docs/operations/TRIAL_E2E_LXC_SNAPSHOT_RUNBOOK.md`
- Probe script: `tests/integration/scripts/trial-signup-contract.sh`
- Full sandbox orchestration (multi-tenant + trial + cloud, with per-scenario snapshot reset):
  - `tests/integration/scripts/run-lxc-sandbox-evals.sh`
  - Includes post-trial expiry downgrade verification and cloud subscription cancellation lifecycle verification

Example:
```bash
cd tests/integration
./scripts/run-lxc-sandbox-evals.sh
```

If the snapshot has stale binaries, inject the latest Linux control-plane build on each rollback:
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/pulse-control-plane-e2e-linux-amd64 ./cmd/pulse-control-plane
cd tests/integration
PULSE_E2E_CP_BINARY=/tmp/pulse-control-plane-e2e-linux-amd64 ./scripts/run-lxc-sandbox-evals.sh
```

### Run Theme Visual Regression Suite
```bash
cd tests/integration
PULSE_E2E_SKIP_DOCKER=1 \
PULSE_BASE_URL='http://your-pulse-host:7655' \
PULSE_E2E_USERNAME='admin' \
PULSE_E2E_PASSWORD='your-password' \
npm run test:visual
```

To refresh baselines intentionally:
```bash
npm run test:visual:update
```

When running against an existing instance (`PULSE_E2E_SKIP_DOCKER=1`), authenticated
visual snapshots require explicit credentials:
- `PULSE_E2E_USERNAME`
- `PULSE_E2E_PASSWORD`

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
