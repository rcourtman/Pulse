# Mock Mode Development Guide

Pulse ships with a mock data pipeline so you can iterate on UI and backend
changes without touching real infrastructure. This guide collects everything you
need to know about running in mock mode during development.

---

## Why Mock Mode?

- Exercise dashboards, alert timelines, and charts with predictable sample data.
- Reproduce edge cases (offline nodes, noisy containers, backup failures) by
  tweaking configuration values rather than waiting for production incidents.
- Swap between synthetic and live data without rebuilding services.

---

## Starting the Dev Stack

```bash
# Launch backend + frontend with hot reload
./scripts/hot-dev.sh
```

The script exposes:
- Frontend: `http://localhost:7655` (Vite hot module reload)
- Backend API: `http://localhost:7656`

---

## Toggling Mock Data

The npm helpers and `toggle-mock.sh` wrapper point the backend at different
`.env` files and restart the relevant services automatically.

```bash
npm run mock:on     # Enable mock mode
npm run mock:off    # Return to real data
npm run mock:status # Display current state
npm run mock:edit   # Open mock.env in $EDITOR
```

Equivalent shell invocations:

```bash
./scripts/toggle-mock.sh on
./scripts/toggle-mock.sh off
./scripts/toggle-mock.sh status
```

When switching:
- `mock.env` (or `mock.env.local`) feeds configuration values to the backend.
- `PULSE_DATA_DIR` swaps between `/opt/pulse/tmp/mock-data` (synthetic) and
  `/etc/pulse` (real data) so test credentials never mix with production ones.
- The backend process restarts; the frontend stays hot-reloading.

---

## Customising Mock Fixtures

`mock.env` exposes the knobs most developers care about:

```bash
PULSE_MOCK_MODE=false            # Enable/disable mock mode
PULSE_MOCK_NODES=7               # Number of synthetic nodes
PULSE_MOCK_VMS_PER_NODE=5        # Average VM count per node
PULSE_MOCK_LXCS_PER_NODE=8       # Average container count per node
PULSE_MOCK_RANDOM_METRICS=true   # Toggle metric jitter
PULSE_MOCK_STOPPED_PERCENT=20    # Percentage of guests stopped/offline
PULSE_ALLOW_DOCKER_UPDATES=true  # Treat Docker builds as update-capable (skips restart)
```

When `PULSE_ALLOW_DOCKER_UPDATES` (or `PULSE_MOCK_MODE`) is enabled the backend
exposes the full update flow inside containers, fakes the deployment type to
`mock`, and suppresses the automatic process exit that normally follows a
successful upgrade. This is what the Playwright update suite uses inside CI.

Create `mock.env.local` for personal tweaks that should not be committed:

```bash
cp mock.env mock.env.local
$EDITOR mock.env.local
```

The toggle script prioritises `.local` files, falling back to the shared
defaults when none are present.

---

## Troubleshooting

- **Backend did not restart:** flip mock mode off/on again (`npm run mock:off`,
  then `npm run mock:on`) to force a reload.
- **Ports already in use:** confirm nothing else is listening on `7655`/`7656`
  (`lsof -i :7655` / `lsof -i :7656`) and kill stray processes.
- **Data feels stale:** delete `/opt/pulse/tmp/mock-data` and toggle mock mode
  back on to regenerate fixtures.

---

## Limitations

- Mock data focuses on happy-path flows; use real Proxmox/PBS environments
  before shipping changes that touch API integrations.
- Webhook payloads are synthetically generated and omit provider-specific
  quirks—test with real channels for production rollouts.
- Encrypt/decrypt flows still use the local crypto stack; do not treat mock mode
  as a sandbox for experimenting with credential formats.

For more advanced scenarios, inspect `scripts/hot-dev.sh` and the mock seeders
under `internal/mock` for additional entry points.
