# Cloud Hosted Tier Runtime Readiness Storage Guardrails Production Record

- Date: `2026-04-24`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Result: `passed`
- Evidence tier: `real-external-e2e`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Control-plane image: `pulse-control-plane:ga-storage-8fa98ad0a`
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
- stale proof tenant maximum age: `24h`
- proof tenant matchers: `proof,canary,rehearsal`

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

## External Storage Canary

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

## Live Tenant Boundary

During the proof window, the restarted control plane also provisioned one
non-proof tenant from a Stripe checkout event:

- Tenant: `t-6AQTH6A5B2`
- Account: `a_W3PT2W0YFR`
- State: `active`
- Plan version: `stripe`
- Runtime container: `08ebca7f70cb...`
- Registry health: `health_check_ok=1`

This tenant is live customer/prospect data, not a proof/canary/rehearsal tenant.
It was intentionally retained. The final audit below is therefore a stronger
post-proof state than an empty lab: the control plane passed admission and audit
with a real active hosted tenant and its managed runtime present.

## Final Audit

The final production audit after canary cleanup passed:

```text
audit_ok=true
tenant_total=1
tenant_active=1
tenant_registry_unhealthy_active=0
docker_managed_total=1
docker_managed_running=1
docker_managed_unhealthy=0
storage_guardrails_enabled=true
storage_ok=true
storage_root_status=ok
storage_root_available_bytes=151487172608
storage_root_min_available_bytes=10737418240
storage_data_status=ok
storage_data_available_bytes=151487172608
storage_data_min_available_bytes=5368709120
storage_docker_status=ok
storage_docker_available_bytes=151487172608
storage_docker_min_available_bytes=10737418240
docker_build_cache_status=ok
docker_build_cache_total_bytes=0
docker_build_cache_max_bytes=2147483648
proof_tenant_stale_count=0
```

Host and Docker state at the same point:

```text
/dev/vda1       154G   13G  142G   9% /
Images          11        3         6.763GB   2.934GB reclaimable
Containers      3         3         45.06kB   0B reclaimable
Local Volumes   3         1         212.4MB   212.4MB reclaimable
Build Cache     0         0         0B        0B
```

Running managed/runtime services:

```text
pulse-t-6AQTH6A5B2 pulse-runtime:ga-copyupfix-c4f1e8d7cb1f Up (healthy)
pulse-cloud-control-plane-1 pulse-control-plane:ga-storage-8fa98ad0a Up
pulse-cloud-traefik-1 a9890c898f37 Up
```

## Conclusion

`cloud-hosted-tier-runtime-readiness` can be treated as `passed` for the current
GA multi-tenant scope. The previous storage exhaustion blocker is no longer only
manually cleaned up: hosted tenant provisioning and runtime rollout now have
production admission guardrails, the live audit command proves the storage and
stale-proof-tenant floor, and a fresh external MSP canary passed create, list,
portal, detail, delete, and cleanup verification on the live public control
plane.

The remaining operational follow-up is to wire the audit command into recurring
production monitoring/alerting so drift is surfaced before a human asks for a
GA proof again.
