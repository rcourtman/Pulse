# Cloud Hosted Tier Runtime Readiness Storage Blocker

- Date: `2026-04-23`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Result: `blocked`
- Environment:
  - Live control plane: `https://cloud.pulserelay.pro`
  - Host: `root@pulse-cloud`
  - Registry DB: `/data/control-plane/tenants.db`
  - Tenant runtime root: `/data/tenants`

## Blocking Facts

1. A fresh live MSP rehearsal was intentionally stopped before creating the
   canary account or workspaces because the production host root filesystem was
   full:
   - `/dev/vda1` mounted at `/` reported `154G` used of `154G`
   - inode usage was only `6%`, so this is block exhaustion rather than inode
     exhaustion
2. The space pressure is dominated by runtime/build retention, not tenant data:
   - `/var/lib/containerd`: `109G`
   - `/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs`: `99G`
   - `/var/lib/containerd/io.containerd.content.v1.content`: `10G`
   - `/var/lib/docker`: `8.5G`
   - `/var/lib/docker/containers`: `8.1G`
   - `/tmp`: `34G`
   - `/data`: `846M`
3. Docker reported substantial reclaimable runtime/build state:
   - images: `116.2GB` total, `109.9GB` reclaimable
   - containers: `52.36GB` total, `6.828GB` reclaimable
   - build cache: `21.28GB` total, `15.48GB` private/reclaimable
4. The live control-plane registry currently carries a standing hosted fleet
   rather than an empty production system:
   - tenant states: `77 active`, `3 canceled`, `4 deleted`, `1 suspended`
   - accounts: `97` total (`93 individual`, `4 msp`)
   - active plan distribution included `50` `v5_pro_annual_grandfathered`,
     `13` `msp_starter`, `10` `v5_pro_monthly_grandfathered`, and `4`
     `cloud_starter`
5. Docker runtime state showed `93` containers, `83` running, and `81` unhealthy.
   Sampled tenant health checks were failing with:
   - `OCI runtime exec failed: write /tmp/runc-process...: no space left on device`
6. Docker host-level retention policy is absent:
   - `/etc/docker/daemon.json` was not present
   - tenant containers use Docker's default `json-file` log driver with no
     `max-size` or `max-file`
   - no Docker/container prune timer was present in `systemctl list-timers`
7. Container JSON logs are already materially contributing to the issue:
   individual tenant log files were sampled at roughly `140M`-`185M`, with no
   configured rotation.
8. Historical build artifacts remain on the production host under `/tmp`, with
   old Pulse build/source directories contributing tens of gigabytes, including
   multi-gigabyte directories from March 2026.

## Why The Gate Cannot Be Treated As Passed

The previously recorded hosted production proofs remain valuable point-in-time
functional evidence, but the current live production environment no longer
meets the operational floor needed for GA. A customer-facing hosted tier cannot
be called ready while tenant health checks fail from disk exhaustion, old proof
tenants remain as a standing production fleet, and the host has no Docker
retention or log-bounding policy.

This also blocks the requested fresh MSP production rehearsal. Running new
workspace provisioning into a full root filesystem would produce misleading
evidence and risk additional production damage.

## Required Unblock Steps

1. Immediate live-host containment, with explicit destructive-action approval:
   - remove stale Pulse build/source directories from `/tmp`
   - remove stopped pre-rollout tenant containers
   - prune old build cache and unused images
   - classify the active hosted proof/canary tenant fleet before deprovisioning
     or retaining any tenant
2. Add a production retention policy:
   - Docker log rotation for tenant containers
   - scheduled Docker/containerd build-cache and unused-image cleanup
   - documented canary/proof tenant lifecycle and cleanup ownership
3. Add production storage guardrails:
   - disk pressure monitoring and alerting for `/`, `/var/lib/containerd`,
     `/var/lib/docker`, `/tmp`, and `/data`
   - provisioning admission should fail closed before creating a tenant when
     required runtime storage is below the safe threshold
4. Move live release builds away from persistent production `/tmp` state and
   favor digest-pinned images built outside the production runtime host.
5. After cleanup and guardrails are in place, rerun the fresh production MSP
   rehearsal and hosted-runtime proof from a new canary.

## Conclusion

`cloud-hosted-tier-runtime-readiness` is blocked again as of `2026-04-23`.
The code-level MSP readiness fixes are still useful, but the live hosted
production environment is not GA-ready until this storage and retention issue is
fixed and re-proven.

## Immediate Repo Containment

The control-plane Docker manager now creates new tenant runtime containers with
bounded `json-file` logs (`CP_TENANT_LOG_MAX_SIZE`, default `10m`, and
`CP_TENANT_LOG_MAX_FILE`, default `3`). This prevents future tenant containers
from accumulating unbounded Docker JSON logs after the fix is deployed and
tenant containers are recreated, but it does not reclaim the existing production
host or replace the required live-host cleanup steps above.
