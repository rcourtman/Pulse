# Cloud Hosted Tier Runtime Readiness Storage Guardrails Production Record

- Date: `2026-04-24`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Result: `passed`
- Evidence tier: `real-external-e2e`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Control-plane image: `pulse-control-plane:ga-audit-5fd645630`
  - Registry DB: `/data/control-plane/tenants.db`
  - Tenant runtime root: `/data/tenants`

## Remediation Added

The production control plane now has storage admission checks on the hosted
tenant creation and runtime rollout paths. Provisioning and runtime replacement
fail closed before mutating tenant/account state when root, tenant-data, Docker
storage, or Docker build-cache thresholds are unsafe.

The live deployment was updated with read-only host mounts for root and Docker
storage inspection plus explicit production thresholds:

- root available floor: `10GiB`
- tenant-data available floor: `5GiB`
- Docker storage available floor: `10GiB`
- Docker build-cache ceiling: `2GiB`
- stale proof tenant maximum age: `1s`
- proof tenant/account matchers:
  `proof,canary,rehearsal,msp_prod,ownerseed,owner_seed`

The operator proof command is:

```bash
docker exec pulse-cloud-control-plane-1 pulse-control-plane cloud audit
```

## Pre-Canary Audit

After deploying the storage-guarded control plane and pruning Docker build
cache, the live audit passed with no tenants:

```text
audit_ok=true
tenant_total=0
docker_managed_total=0
storage_guardrails_enabled=true
storage_ok=true
storage_root_available_bytes=151489216512
storage_root_min_available_bytes=10737418240
storage_data_available_bytes=151489216512
storage_data_min_available_bytes=5368709120
storage_docker_available_bytes=151489216512
storage_docker_min_available_bytes=10737418240
docker_build_cache_total_bytes=0
docker_build_cache_max_bytes=2147483648
proof_tenant_stale_count=0
```

The public readiness endpoints returned `ok` and `ready`.

## Initial External Storage Canary

A fresh disposable MSP account was seeded only for this proof:

- Account: `a_storage_canary_20260424T110506Z`
- Created workspaces:
  - `t-KG9T7TVPFB`
  - `t-5MZMPX97ZP`

The public HTTPS account and portal APIs passed:

1. Initial tenant list returned `200` with `0` workspaces.
2. Workspace creation returned an active tenant for both disposable workspaces.
3. Follow-up tenant list returned `200` with `2` workspaces.
4. Portal dashboard returned `200`, `kind=msp`, and `total=2`.
5. Workspace detail returned `200`, `state=active`, and `plan=msp_starter`
   for both workspaces.
6. Workspace deletion returned `204` for both disposable workspaces.

After the proof, cleanup verification showed zero remaining canary rows in
`accounts`, `users`, `account_memberships`, `stripe_accounts`, `tenants`, and
`hosted_entitlements`. The two canary tenant directories were also absent from
`/data/tenants`.

## Owner Classification Correction

After the initial proof, the project owner clarified that Pulse Cloud has no
multi-tenant customers yet. The active tenant that had been provisioned from a
Stripe checkout event was therefore reclassified as hosted-runtime residue, not
live customer state. During that correction, one old MSP rehearsal account also
recreated a hosted tenant, proving that stale proof account rows were still a
real production risk.

The control plane was stopped, `/data/control-plane/tenants.db` was backed up to
`/root/tenants-pre-ga-empty-cleanup-20260424T111936Z.db`, and the hosted runtime
state was cleaned to the correct empty-customer baseline:

- Removed hosted tenant rows:
  - `t-6AQTH6A5B2`
  - `t-W9WWCGBETH`
- Removed the corresponding managed Docker runtime containers.
- Removed the corresponding `/data/tenants/*` directories.
- Removed `7` orphan paid hosted entitlement rows whose tenant rows no longer
  existed.
- Removed `4` stale MSP proof/rehearsal account rows:
  - `a_msp_prod_20260313105601`
  - `a_msp_prod_fix_20260313111348`
  - `a_msp_owner_seed_20260313124930`
  - `a_ownerseed_20260313T145927`

The audit implementation was then tightened so this residue is no longer
invisible: `pulse-control-plane cloud audit` now fails on stale proof accounts
and paid hosted entitlements whose tenant rows are missing.

## Corrected External Storage Canary

A second disposable MSP canary was run after the empty-customer cleanup and
after deploying `pulse-control-plane:ga-audit-5fd645630`:

- Account: `a_ga_empty_canary_20260424T112557Z`
- Created workspaces:
  - `t-VXX6Z6W0KE`
  - `t-NN991B67FY`

The public HTTPS account and portal APIs passed:

1. Initial tenant list returned `200` with `0` workspaces.
2. Workspace creation returned an active tenant for both disposable workspaces.
3. Follow-up tenant list returned `200` with `2` workspaces.
4. Portal dashboard returned `200`, `kind=msp`, and `total=2`.
5. Workspace detail returned `200`, `state=active`, and `plan=msp_starter`
   for both workspaces.
6. Workspace deletion returned `204` for both disposable workspaces.

Post-canary cleanup verification showed:

- `tenants=0`
- `paid_entitlements=0`
- `ga_empty_canary_accounts=0`
- `stale_msp_proof_accounts=0`
- no remaining directories under `/data/tenants`

## Mobile Proof Residue Recheck

After the corrected canary, a same-day disposable mobile proof run created one
MSP proof workspace:

- Account: `a_JJ8PBFMVN3`
- Workspace: `t-RB6M98924S`

The account display name and workspace display name both identified the state as
Pulse Mobile GA proof residue, not customer state. The tenant registry was
backed up to
`/root/tenants-pre-ga-mobile-proof-cleanup-20260424T113639Z.db`, the proof
tenant/account rows were removed, the managed runtime container and
`/data/tenants/t-RB6M98924S` directory were removed, and production
`CP_PROOF_TENANT_MAX_AGE` was tightened to `1s` so future proof residue is
surfaced by `pulse-control-plane cloud audit` almost immediately.

## Mobile Approval Proof Residue Recheck

A later same-day mobile approval proof left one additional disposable proof
workspace:

- Account: `a_ZTYQ41MVYG`
- Workspace: `t-KAQ6WKJX8V`

The account display name identified the state as
`Pulse Mobile GA Proof 20260424 Approval`. The recurring audit failed the
baseline with `proof_tenant_stale_count=1` and
`proof_account_stale_count=1`, proving the tightened proof-residue gate now
catches this class of drift.

The tenant registry was backed up to
`/root/tenants-pre-ga-mobile-proof-approval-cleanup-20260424T135425Z.db`, the
proof tenant/account rows were removed, the managed runtime container and
`/data/tenants/t-KAQ6WKJX8V` directory were removed, and the audit returned to
the empty hosted-customer baseline.

## Final Audit

The final production audit after proof-residue cleanup passed:

```text
audit_ok=true
tenant_total=0
tenant_active=0
tenant_registry_unhealthy_active=0
docker_managed_total=0
docker_managed_running=0
docker_managed_unhealthy=0
storage_guardrails_enabled=true
storage_ok=true
storage_root_status=ok
storage_root_available_bytes=151212212224
storage_root_min_available_bytes=10737418240
storage_data_status=ok
storage_data_available_bytes=151212212224
storage_data_min_available_bytes=5368709120
storage_docker_status=ok
storage_docker_available_bytes=151212212224
storage_docker_min_available_bytes=10737418240
docker_build_cache_status=ok
docker_build_cache_total_bytes=0
docker_build_cache_max_bytes=2147483648
proof_tenant_stale_count=0
proof_account_stale_count=0
hosted_paid_orphan_entitlement_count=0
```

Host and Docker state at the same point:

```text
/dev/vda1       154G   14G  141G   9% /
Images          12        2         6.882GB   2.407GB reclaimable
Containers      2         2         30.98MB   0B reclaimable
Local Volumes   3         1         212.4MB   212.4MB reclaimable
Build Cache     0         0         0B        0B
```

Running managed/runtime services:

```text
pulse-cloud-control-plane-1 pulse-control-plane:ga-audit-5fd645630 Up
pulse-cloud-traefik-1 a9890c898f37 Up
```

## Recurring Audit Monitor

The live host now has a recurring GA audit monitor installed:

- Script: `/opt/pulse-cloud/cloud-audit-monitor.sh`
- Unit: `/etc/systemd/system/pulse-cloud-audit.service`
- Timer: `/etc/systemd/system/pulse-cloud-audit.timer`
- Latest log: `/var/log/pulse-cloud-audit/latest.log`
- Status file: `/var/lib/pulse-cloud-audit/status.env`
- Prometheus textfile metric:
  `/var/lib/node_exporter/textfile_collector/pulse_cloud_audit.prom`
- Managed source:
  `pulse-pro:ops/pulse-cloud/audit/`

The timer runs every five minutes after boot and fails
`pulse-cloud-audit.service` if `pulse-control-plane cloud audit` fails or does
not print `audit_ok=true`.

Live verification after installing the managed monitor returned:

```text
Result=success
ExecMainStatus=0
PULSE_CLOUD_AUDIT_STATUS=ok
PULSE_CLOUD_AUDIT_EXIT_CODE=0
PULSE_CLOUD_AUDIT_AUDIT_OK=true
pulse_cloud_audit_success 1
pulse_cloud_audit_exit_code 0
```

A later timer-fired run also succeeded without manual start:

```text
ExecMainStartTimestamp=Fri 2026-04-24 14:19:07 UTC
ExecMainExitTimestamp=Fri 2026-04-24 14:19:08 UTC
ExecMainStatus=0
PULSE_CLOUD_AUDIT_CHECKED_AT=2026-04-24T14:19:07Z
pulse_cloud_audit_success 1
```

## Conclusion

`cloud-hosted-tier-runtime-readiness` can be treated as `passed` for the current
GA multi-tenant scope. The previous storage exhaustion blocker is no longer only
manually cleaned up: hosted tenant provisioning and runtime rollout now have
production admission guardrails, the recurring live audit monitor proves the
storage, stale-proof-tenant, stale-proof-account, and orphan-paid-entitlement
floor, and a fresh external MSP canary passed create, list, portal, detail,
delete, and cleanup verification on the live public control plane.

The correct GA baseline for Pulse Cloud today is empty of hosted customer
tenants. As of this record, that baseline is true in the registry, Docker, and
`/data/tenants`.
