# Trial E2E on Proxmox LXC (Snapshot-Clean)

This runbook defines a clean, repeatable end-to-end trial validation loop for a Pulse binary running in Proxmox LXC.

Goal: every eval run starts from the exact same filesystem and runtime state, so previous runs cannot pollute new results.

## Scope

- Validates live trial gate behavior (`trial_signup_required`)
- Validates hosted signup page rendering
- Validates checkout redirect contract to Stripe hosted checkout
- Does **not** complete payment or webhook activation

## Prerequisites

- Proxmox host access (example: `ssh root@<pve-host>`)
- LXC with Pulse + control plane binaries and env configured
- Test credentials set (recommended for deterministic checks): `admin/admin`
- Required tools inside LXC: `curl`, `jq`, `ca-certificates`

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
3. `POST /api/license/trial/start` returns `409` with code `trial_signup_required`.
4. Hosted signup page loads and contains expected marker text.
5. Checkout endpoint returns `303` and redirects to `checkout.stripe.com`.

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

If each run prints `PASS: trial signup contract validated`, state pollution between runs is eliminated.

## Known Limitation

This runbook validates the real checkout redirect contract, but does not automate full Stripe completion in live mode.

To automate final activation end-to-end:

- use Stripe test-mode credentials/prices for non-production automation, or
- complete live checkout manually and verify webhook-driven activation separately.
