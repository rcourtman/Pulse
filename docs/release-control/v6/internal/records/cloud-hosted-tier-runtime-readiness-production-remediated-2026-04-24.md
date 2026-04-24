# Cloud Hosted Tier Runtime Readiness Production Remediated Record

- Date: `2026-04-24`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Result: `passed`
- Evidence tier: `real-external-e2e`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Registry DB: `/data/control-plane/tenants.db`
  - Tenant runtime root: `/data/tenants`
  - Tenant runtime image after remediation: `pulse-runtime:ga-recovery-9dbaaa7efeb5`

## Production Blocker Remediated

The live Pulse Cloud host previously blocked GA readiness because `/dev/vda1`
was full (`154G` used of `154G`), Docker/containerd retained old build and
runtime state, tenant JSON logs were unbounded, and old proof tenants had been
left as a standing production fleet.

The user explicitly approved clearing old proof tenants. No customer tenants
were intentionally retained during this remediation.

## Remediation Actions

1. Removed stale Pulse build, deploy, and source directories from `/tmp`,
   reclaiming roughly `32.7G`.
2. Pruned Docker/containerd build and image cache after confirming no customer
   tenant fleet needed to remain.
3. Installed host-level Docker log defaults in `/etc/docker/daemon.json`:
   bounded `json-file` logs with `max-size=10m`, `max-file=3`, and
   `live-restore=true`.
4. Installed `/etc/logrotate.d/docker-containers` with the same `10M` and
   `rotate 3` policy, then forced one rotation of existing container logs.
5. Stopped only the control plane, backed up
   `/data/control-plane/tenants.db`, removed `85` old `pulse-t-*` proof tenant
   containers, cleared proof tenant registry and billing/account rows, removed
   old `/data/tenants/t-*` proof tenant directories, and restarted the control
   plane.

## Stable Clean Baseline

After the control plane had restarted and settled for roughly `75s`, the live
host reported:

- `tenants=0`
- `stripe_accounts=0`
- `accounts=0`
- `users=0`
- `hosted_entitlements=0`
- `tenant_containers=0`
- `running_containers=2`
- `unhealthy_containers=0`
- root filesystem: `12G` used, `142G` available, `8%` full

The only running services were the control plane and Traefik. The control-plane
health and readiness endpoints returned `ok` and `ready`.

## External Canary

A fresh disposable MSP account was seeded only for this proof:

- Account: `a_ga_canary_20260424T082432Z`
- Created workspaces:
  - `t-JZNJF2AW7S`
  - `t-AB21TTA2FC`

The live public HTTPS API passed the following checks:

1. Initial MSP tenant list returned `200` with `0` workspaces.
2. Workspace creation returned `201` for both disposable workspaces.
3. Follow-up tenant list returned `200` with both workspaces.
4. MSP member invite returned `202`.
5. MSP member list returned `200` with the expected members.
6. Portal dashboard returned `200`, `kind=msp`, and `total=2`.
7. Workspace detail returned `200`, `plan=msp_starter`, and `state=active`
   for both workspaces.
8. Public signup boundary returned `400` with `code=tier_unavailable`.
9. Workspace deletion returned `204` for both disposable workspaces.

After the proof, the canary account, users, billing rows, and tenant rows were
removed again.

## Post-Proof State

The final production state after the canary cleanup was:

- `tenants=0`
- `stripe_accounts=0`
- `accounts=0`
- `users=0`
- `hosted_entitlements=0`
- `tenant_containers=0`
- `running_containers=2`
- `unhealthy_containers=0`
- root filesystem: `13G` used, `142G` available, `8%` full
- Docker images: `6.369GB` total, `3.249GB` reclaimable
- Docker build cache: `985.8MB`

## Build Contract Follow-Up

An attempted fresh production runtime image build from the repository runtime
target still required installer signing material:

`installer-ssh-public-key is required for rendered installers`

No local secret bypass was used. This did not block the live production proof
because the deployed runtime image passed the external canary, but the release
build contract should be tightened so production tenant runtime images can be
built without depending on full installer signing inputs unless the release
pipeline intentionally requires them.

## Conclusion

`cloud-hosted-tier-runtime-readiness` can be treated as `passed` again. The
production storage exhaustion was remediated, stale proof tenants were removed,
host-level log retention now exists, and a fresh disposable MSP canary proved
workspace create/list/member/dashboard/detail/boundary/delete behavior over the
live public HTTPS control-plane surface.
