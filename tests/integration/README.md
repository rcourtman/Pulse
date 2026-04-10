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
  - Start hosted Pro trial initiation via `POST /api/license/trial/start` and verify the reused-instance contract
  - Verify the response either returns `409 trial_signup_required` with hosted `/start-pro-trial` handoff or `429 trial_rate_limited` with canonical `Retry-After` backoff when the local retry bucket is already exhausted
  - Verify local entitlements remain unchanged until activation
  - Verify duplicate initiation stays on the hosted-signup retry-burst contract
- `tests/58-self-hosted-trial-rate-limit-ui.spec.ts` — Pulse Pro trial CTA retry-after UI contract:
  - Open `/settings/system/billing` on the real browser shell with free-tier entitlements
  - Stub `429 trial_rate_limited` on `POST /api/license/trial/start`
  - Verify the Pulse Pro CTA surfaces canonical `Retry-After` guidance even if `details.retry_after_seconds` disagrees
- `tests/08-cloud-hosting.spec.ts` — hosted cloud signup contract:
  - Public `/cloud/signup` form creates a real Stripe sandbox checkout session
  - Checkout completes and returns to hosted signup completion page
  - Magic-link request path succeeds via real public endpoint
- `tests/09-cloud-billing-lifecycle.spec.ts` — hosted cloud post-checkout lifecycle:
  - Replays verified Stripe webhook events into control plane
  - Asserts tenant activation after checkout event processing
  - Asserts tenant cancellation after subscription deletion event processing
- `tests/16-dev-runtime-recovery.spec.ts` — managed browser runtime proof:
  - Attaches Playwright to the canonical `5173` browser entrypoint
  - Restarts the managed backend and proves the browser shell recovers through the proxy
- `tests/17-recovery-layout.spec.ts` — desktop Recovery layout regression guard:
  - Mocks a realistic Recovery dataset with human-readable subject labels
  - Proves the focused history table fits the desktop wrapper without horizontal overflow
  - Proves the `Outcome` column stays visible at the right edge
- `tests/18-patrol-runtime-state.spec.ts` — Patrol runtime-state browser guard:
  - Mocks a blocked Patrol runtime with stale healthy summary payloads
  - Proves the real `/ai` route shows Patrol as paused and suppresses stale healthy summary copy

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
- When `PULSE_MULTI_TENANT_ENABLED=true`, the integration harness also seeds a deterministic Enterprise-eval billing state so the multi-tenant suite runs against a licensed surface instead of skipping.

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

### Run Against A Managed Local Backend (No Docker, Deterministic)
```bash
cd tests/integration
PULSE_E2E_USE_LOCAL_BACKEND=1 \
PULSE_MULTI_TENANT_ENABLED=true \
npm test -- tests/03-multi-tenant.spec.ts --project=chromium
```

This mode starts an isolated Pulse backend from the local repo binary in a temporary data directory under `tmp/integration-local-backend`, seeds the requested entitlement profile, writes runtime connection state for Playwright, and cleans everything up in `posttest`.

Each `npm test` invocation gets its own runtime-state file and managed-backend root automatically, so separate managed-local-backend runs can execute in parallel without sharing PID or cleanup state. Shared embedded-frontend and backend-binary refreshes are serialized by the harness.

### Run Against The Managed Hot-Dev Browser Runtime
Canonical one-command verification from the repo root:
```bash
npm run dev:verify
```

Equivalent direct proof command from the integration harness:
```bash
cd tests/integration
PULSE_E2E_USE_HOT_DEV=1 \
PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 \
npm test -- tests/16-dev-runtime-recovery.spec.ts tests/17-recovery-layout.spec.ts tests/18-patrol-runtime-state.spec.ts --project=chromium
```

This mode attaches Playwright to the canonical dev browser entrypoint on `http://127.0.0.1:5173`, uses the repo-root managed runtime wrappers as the control surface, and writes browser runtime connection state for Playwright instead of targeting the backend port directly. Those wrappers are backed by `scripts/hot-dev-bg.sh`, but the wrapper surface is the canonical operator contract.
When you run the wrapper as `npm run dev:verify`, the managed launcher hands a verification-lock path into the integration runner, and the runner holds that lock for the actual pretest, Playwright, and posttest lifetime so unrelated backend source churn does not invalidate the recovery proofs mid-run.
When no explicit `PULSE_BASE_URL`, `PLAYWRIGHT_BASE_URL`, or runtime-state file is present, the shared integration browser-default helper now prefers the managed hot-dev browser shell on `http://127.0.0.1:5173` whenever `hot-dev-bg` is already running, and only falls back to the embedded frontend on `:7655` when there is no managed dev session to attach to.
When both are set, `PLAYWRIGHT_BASE_URL` now wins for the browser target while
`PULSE_BASE_URL` remains the backend-oriented base for pretest health checks,
explicit API helpers, and non-browser setup work. Use that split when you need
Playwright on fresh frontend code but still want backend provisioning or
health checks against the canonical Pulse API port.

Example:
```bash
cd tests/integration
PULSE_E2E_SKIP_DOCKER=1 \
PULSE_BASE_URL='http://127.0.0.1:7655' \
PLAYWRIGHT_BASE_URL='http://127.0.0.1:4174' \
npm test -- tests/30-setup-platform-connections-handoff.spec.ts --project=chromium
```

If the managed runtime is not already running, the harness starts it. If you need the harness to reclaim existing unmanaged `5173`/`7655` listeners first, add `PULSE_E2E_HOT_DEV_TAKEOVER=1`.

The managed proof pack bounces the real backend through `npm run dev:backend-restart`, kills the supervised `hot-dev.sh` owner process to prove full runtime recovery, verifies that the browser shell reports those outages and then recovers through the proxy, and keeps a desktop Recovery layout guard on the same canonical browser entrypoint. When the harness attaches to an already-running managed dev session, `posttest` leaves that runtime running.

For deterministic paid-feature runs against an existing instance, provide one of:
- `PULSE_E2E_BILLING_STATE_PATH=/absolute/path/to/billing.json` to let the harness write the billing state file directly.
- `PULSE_E2E_ENTITLEMENT_WRITE_COMMAND='ssh host "cat > /etc/pulse/billing.json"'` to pipe the billing JSON to a remote/local writer command.

The harness understands these profiles:
- `PULSE_E2E_ENTITLEMENT_PROFILE=multi-tenant` for Enterprise/MSP multi-tenant coverage.
- `PULSE_E2E_ENTITLEMENT_PROFILE=infra` for Pro/relay/reporting-style journeys.

### Snapshot-Clean Proxmox LXC Trial SAT
For hosted trial initiation validation against a fresh LXC each run:

- Runbook: `docs/operations/TRIAL_E2E_LXC_SNAPSHOT_RUNBOOK.md`
- Probe script: `tests/integration/scripts/trial-signup-contract.sh`
  - Exercises `POST /api/license/trial/start` from a clean snapshot and proves
    the hosted-signup retry-burst contract: duplicate attempts keep returning
    `409 trial_signup_required` with hosted redirects while the burst remains
    open, then transition to `429 trial_rate_limited` plus `Retry-After`
    backoff metadata once the retry burst is exhausted and the limiter
    actually engages
- Pulse Pro browser proof: `tests/58-self-hosted-trial-rate-limit-ui.spec.ts`
  - Exercises `/settings/system/billing` on the real browser shell and proves
    the rendered CTA surfaces canonical `Retry-After` guidance when
    `trial_rate_limited` is returned, including header precedence over a
    conflicting `details.retry_after_seconds` payload
- Full sandbox orchestration (multi-tenant + trial + cloud, with per-scenario snapshot reset):
  - `tests/integration/scripts/run-lxc-sandbox-evals.sh`
  - Includes hosted trial initiation validation and cloud subscription cancellation lifecycle verification

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
