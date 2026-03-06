# Trial + Cloud E2E on Proxmox LXC (Snapshot-Clean)

This runbook defines a clean, repeatable end-to-end trial validation loop for a Pulse binary running in Proxmox LXC.

Goal: every eval run starts from the exact same filesystem and runtime state, so previous runs cannot pollute new results.

## Scope

- Validates hosted trial initiation from a self-hosted Pulse instance
- Validates initiation does not mint local entitlements before activation
- Validates initiation rate limiting and idempotent redirect behavior
- Validates real Stripe sandbox checkout completion for Pulse Cloud signup
- Validates cloud post-checkout lifecycle transition to canceled state
- Validates post-checkout trial activation state in Pulse (`/settings?trial=activated`)

## Prerequisites

- Proxmox host access (example: `ssh root@<pve-host>`)
- LXC with Pulse + control plane binaries and env configured
- Test credentials set (recommended for deterministic checks): `admin/admin`
- Required tools inside LXC: `curl`, `jq`, `ca-certificates`
- Stripe sandbox keys and test recurring prices configured in control-plane env
- Playwright runner host that can reach both Pulse (`:7655`) and control-plane (`:8443`)

Install required tools inside container:

```bash
pct exec <ctid> -- sh -lc 'apt-get update -y && apt-get install -y curl jq ca-certificates'
```

## Snapshot Capability Requirement

Proxmox snapshots are not available on `dir` rootfs storage. Use snapshot-capable storage (`zfspool`, `lvmthin`, `ceph`, etc.).

Check current rootfs:

```bash
pct config <ctid> | rg '^rootfs:'
```

If required, move container rootfs to snapshot-capable storage (example: `local-zfs`):

```bash
pct stop <ctid>
pct move-volume <ctid> rootfs local-zfs --delete 1
pct start <ctid>
```

## Baseline Snapshot Setup

Create a baseline snapshot once the container is in known-good state:

```bash
pct snapshot <ctid> pre-eval-baseline --description "Pulse trial e2e baseline"
pct listsnapshot <ctid>
```

## Service Startup Requirement

After rollback, Pulse and control plane must start automatically. Use `systemd` units and enable them:

```bash
pct exec <ctid> -- sh -lc 'systemctl enable --now pulse.service pulse-control-plane.service'
```

Verify listeners:

```bash
pct exec <ctid> -- sh -lc 'ss -lntp | grep -E ":(7655|8443)\\b"'
```

## Contract Probe Script

Use:

- `tests/integration/scripts/trial-signup-contract.sh`

This script asserts:

1. Login succeeds.
2. Pre-trial entitlements are fetched and trial is eligible.
3. `POST /api/license/trial/start` returns `409` with `trial_signup_required` and a hosted `action_url`.
4. Post-initiation entitlements remain locally unactivated (`trial_eligible=true`, `subscription_state=expired`).
5. Second immediate trial start is rejected with `429` (rate limited).

Run inside container:

```bash
pct push <ctid> tests/integration/scripts/trial-signup-contract.sh /tmp/trial-signup-contract.sh
pct exec <ctid> -- sh -lc 'chmod +x /tmp/trial-signup-contract.sh && PULSE_E2E_USERNAME=admin PULSE_E2E_PASSWORD=admin /tmp/trial-signup-contract.sh'
```

## Clean Eval Loop (Rollback -> Run)

Use this loop for repeatable runs:

```bash
for i in 1 2 3; do
  pct rollback <ctid> pre-eval-baseline --start 1
  pct exec <ctid> -- sh -lc 'PULSE_E2E_USERNAME=admin PULSE_E2E_PASSWORD=admin /tmp/trial-signup-contract.sh'
done
```

If each run prints `PASS: hosted trial signup initiation contract validated`, state pollution between runs is eliminated.

## Full Sandbox E2E (Playwright)

To prove user-visible end-to-end behavior (including Stripe sandbox checkout completion), run the integration eval pack after each rollback:

```bash
cd tests/integration
PVE_HOST=<pve-host> PVE_CTID=<ctid> ./scripts/run-lxc-sandbox-evals.sh
```

### Optional: Inject Latest Control-Plane Binary Per Rollback

If your snapshot contains an older control-plane binary, build a Linux binary from current source and inject it on every rollback:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/pulse-control-plane-e2e-linux-amd64 ./cmd/pulse-control-plane
cd tests/integration
PVE_HOST=<pve-host> \
PVE_CTID=<ctid> \
PULSE_E2E_CP_BINARY=/tmp/pulse-control-plane-e2e-linux-amd64 \
./scripts/run-lxc-sandbox-evals.sh
```

`run-lxc-sandbox-evals.sh` will copy the binary into `/opt/pulse-test/bin/pulse-control-plane` inside the container after each snapshot rollback.

For full hosted Pro checkout completion, run a separate control-plane/browser flow that can consume the verification email and finish Stripe sandbox checkout. The local LXC probe above validates the self-hosted handoff contract only.

Use Stripe sandbox test card defaults unless overridden:

- `4242 4242 4242 4242`
- expiry `12/34`
- CVC `123`
- postal code `10001`

If any scenario fails, rollback to `pre-eval-baseline` before re-running.
