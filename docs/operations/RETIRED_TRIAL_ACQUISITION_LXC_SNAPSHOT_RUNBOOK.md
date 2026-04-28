# Retired Trial Acquisition + Cloud E2E on Proxmox LXC (Snapshot-Clean)

This runbook defines a clean, repeatable validation loop for retired self-hosted trial acquisition and Pulse Cloud signup behavior on a Pulse binary running in Proxmox LXC.

Goal: every eval run starts from the exact same filesystem and runtime state, so previous runs cannot pollute new results.

## Scope

- Validates ordinary self-hosted v6 does not expose in-app trial acquisition
- Validates probing retired trial acquisition routes does not mutate local entitlements
- Validates default self-hosted UI does not show trial CTAs or paid-only navigation
- Validates real Stripe sandbox checkout completion for Pulse Cloud signup
- Validates cloud post-checkout lifecycle transition to canceled state

## Prerequisites

- Proxmox host access (example: `ssh root@<pve-host>`)
- LXC with Pulse + control plane binaries and env configured
- Test credentials set (recommended for deterministic checks): `admin/adminadminadmin`
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
pct snapshot <ctid> pre-eval-baseline --description "Pulse retired trial acquisition e2e baseline"
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

- `tests/integration/scripts/retired-trial-acquisition-contract.sh`

This script asserts:

1. Login succeeds.
2. Current entitlements can be fetched.
3. `POST /api/license/trial/start` returns `404`, because ordinary self-hosted v6 runtimes no longer expose in-app trial acquisition.
4. The retired route does not return legacy hosted-signup or trial-rate-limit acquisition payloads.
5. Entitlements remain unchanged after the retired route probe.

Run inside container:

```bash
pct push <ctid> tests/integration/scripts/retired-trial-acquisition-contract.sh /tmp/retired-trial-acquisition-contract.sh
pct exec <ctid> -- sh -lc 'chmod +x /tmp/retired-trial-acquisition-contract.sh && PULSE_E2E_USERNAME=admin PULSE_E2E_PASSWORD=adminadminadmin /tmp/retired-trial-acquisition-contract.sh'
```

## Clean Eval Loop (Rollback -> Run)

Use this loop for repeatable runs:

```bash
for i in 1 2 3; do
  pct rollback <ctid> pre-eval-baseline --start 1
  pct exec <ctid> -- sh -lc 'PULSE_E2E_USERNAME=admin PULSE_E2E_PASSWORD=adminadminadmin /tmp/retired-trial-acquisition-contract.sh'
done
```

If each run prints `PASS: self-hosted trial acquisition routes are retired`, state pollution between runs is eliminated.
On a fresh rollback the probe should show `trial_start_code=404` and matching entitlement summaries before and after the route probe.

## Full Sandbox E2E (Playwright)

To prove user-visible end-to-end behavior (including Stripe sandbox checkout completion), run the integration eval pack after each rollback:

```bash
cd tests/integration
PVE_HOST=<pve-host> PVE_CTID=<ctid> ./scripts/run-lxc-sandbox-evals.sh
```

The runner intentionally requires explicit `PVE_HOST` and `PVE_CTID` values so
private sandbox topology is never baked into repo-tracked defaults.

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
