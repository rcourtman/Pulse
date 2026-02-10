# Pulse Cloud Operations Runbook (DigitalOcean, Single-Droplet Phase 1)

This runbook is for the Phase 1 architecture: a single DigitalOcean droplet running:

- Traefik as the public edge (`:80`/`:443`) with DigitalOcean DNS-01 wildcard TLS
- Pulse Control Plane behind Traefik at `https://<DOMAIN>/`
- One Docker container per tenant (`pulse-<tenant_id>`) on the `pulse-cloud` Docker network

For canonical secret sources and how I store them locally, see:

- `/Volumes/Development/pulse/repos/pulse-pro/OPERATIONS.md`

## Deploy From Scratch

### Prerequisites

- DigitalOcean droplet: Ubuntu 24.04 (size per capacity plan in `docs/architecture/cloud-container-per-tenant-2026-02.md`)
- Inbound ports allowed: `22/tcp`, `80/tcp`, `443/tcp`
- DNS zone hosted on DigitalOcean (required for Traefik DNS-01 via `DO_AUTH_TOKEN`)
- A DigitalOcean API token with permission to manage DNS in the zone
- Stripe secret key and a webhook signing secret

### DNS Setup

You need both the control-plane hostname and a wildcard for tenant hostnames:

- `cloud.pulserelay.pro` A record -> `<droplet_public_ip>`
- `*.cloud.pulserelay.pro` A record -> `<droplet_public_ip>` (wildcard)

You can configure this in:

- DigitalOcean control panel -> Networking -> Domains
- Or via API tooling (optional)

### First-Time Setup Script

On the droplet:

```bash
cd /path/to/this/repo/deploy/cloud
sudo -E bash setup.sh
```

If you want a `curl | bash` install flow, `setup.sh` supports downloading a deploy bundle tarball:

```bash
export PULSE_CLOUD_BUNDLE_URL="https://example.com/pulse-cloud-deploy-bundle.tgz"
curl -fsSL "https://example.com/setup.sh" | sudo -E bash
```

What it does (idempotent):

- Installs Docker CE + compose plugin from the official Docker apt repo
- Creates data directories under `/data` (mode `700`)
- Installs the deploy bundle to `/opt/pulse-cloud/`
- Ensures `/opt/pulse-cloud/.env` exists (created from `.env.example` if missing)
- `docker compose pull` then `docker compose up -d`
- Verifies `https://<DOMAIN>/healthz` and port `443` is listening

### Configure Environment Variables

Edit:

- `/opt/pulse-cloud/.env`

Required values:

- `DOMAIN` (example: `cloud.pulserelay.pro`)
- `ACME_EMAIL` (Let’s Encrypt contact email)
- `DO_AUTH_TOKEN` (DigitalOcean token for DNS-01 challenges)
- `CP_ADMIN_KEY` (control plane admin API key)
- `STRIPE_WEBHOOK_SECRET`
- `STRIPE_API_KEY`

Lock down permissions:

```bash
sudo chmod 600 /opt/pulse-cloud/.env
```

### Verify After Deploy

From the droplet:

```bash
DOMAIN="$(grep '^DOMAIN=' /opt/pulse-cloud/.env | cut -d= -f2-)"
curl -fsS -k --resolve "${DOMAIN}:443:127.0.0.1" "https://${DOMAIN}/healthz"
curl -fsS -k --resolve "${DOMAIN}:443:127.0.0.1" "https://${DOMAIN}/readyz"
curl -fsS -k --resolve "${DOMAIN}:443:127.0.0.1" "https://${DOMAIN}/status" | jq
docker compose -f /opt/pulse-cloud/docker-compose.yml ps
```

## Stripe Webhook Configuration

Create a Stripe webhook endpoint:

- URL: `https://cloud.pulserelay.pro/api/stripe/webhook`

Subscribe to these events:

- `checkout.session.completed`
- `customer.subscription.updated`
- `customer.subscription.deleted`
- `invoice.payment_failed`

Then set `STRIPE_WEBHOOK_SECRET` in `/opt/pulse-cloud/.env` to the signing secret for that endpoint.

## Monitoring Tenants

### Public Status

```bash
DOMAIN="$(grep '^DOMAIN=' /opt/pulse-cloud/.env | cut -d= -f2-)"
curl -fsS "https://${DOMAIN}/status" | jq
```

### Admin Tenant Listing

The control plane currently authenticates admin endpoints with `CP_ADMIN_KEY` via:

- `X-Admin-Key: <key>` header, or
- `Authorization: Bearer <key>`

Example:

```bash
DOMAIN="$(grep '^DOMAIN=' /opt/pulse-cloud/.env | cut -d= -f2-)"
ADMIN_KEY="$(grep '^CP_ADMIN_KEY=' /opt/pulse-cloud/.env | cut -d= -f2-)"
curl -fsS "https://${DOMAIN}/admin/tenants" -H "X-Admin-Key: ${ADMIN_KEY}" | jq
```

## Restart A Tenant

Tenant containers are named `pulse-<tenant_id>`.

```bash
docker restart "pulse-t-ABCDEFGHJK"
```

## Manually Provision A Tenant (Testing)

In the current control plane implementation, tenant provisioning is exposed via account-scoped endpoints, but account creation is normally driven by Stripe signup/webhooks.

For ad-hoc testing on a non-production droplet, the simplest path is:

1. Use Stripe (preferred): run the normal signup/checkout flow and let `checkout.session.completed` provision the first tenant automatically.
2. Or create records directly in the control plane registry DB (advanced, not recommended in production):
   - DB path: `/data/control-plane/tenants.db`
   - You must create an `accounts` row before calling `POST /api/accounts/{account_id}/tenants`.

If you already have an `account_id`:

```bash
DOMAIN="$(grep '^DOMAIN=' /opt/pulse-cloud/.env | cut -d= -f2-)"
ADMIN_KEY="$(grep '^CP_ADMIN_KEY=' /opt/pulse-cloud/.env | cut -d= -f2-)"
ACCOUNT_ID="a_0123456789"

curl -fsS -X POST "https://${DOMAIN}/api/accounts/${ACCOUNT_ID}/tenants" \
  -H "X-Admin-Key: ${ADMIN_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"display_name":"Test Workspace"}' | jq
```

## Backup And Restore

### How Backups Work

The daily backup script:

- Script: `/opt/pulse-cloud/backup.sh` (source: `deploy/cloud/backup.sh`)
- Local snapshots: `/data/backups/daily/YYYY-MM-DD/`
- Captures:
  - `/data/tenants/*` (one tenant at a time)
  - `/data/control-plane/*`
- Writes logs to: `/var/log/pulse-cloud-backup.log`
- Retains: 7 local daily backups by default

Remote sync to DigitalOcean Spaces is supported via either `rclone` or `s3cmd`:

- `rclone` (recommended): set `PULSE_RCLONE_REMOTE`
- `s3cmd`: set `PULSE_S3CMD_BUCKET`

No credentials are hardcoded; `rclone`/`s3cmd` must already be configured on the droplet.

### Configure Remote Backup (rclone)

1. Configure rclone for DO Spaces (stores credentials in rclone config):

```bash
rclone config
```

2. Test listing:

```bash
rclone lsf "do-spaces:your-bucket" | head
```

3. Run a one-off backup with remote sync:

```bash
sudo PULSE_RCLONE_REMOTE="do-spaces:your-bucket" /opt/pulse-cloud/backup.sh
```

### Configure Remote Backup (s3cmd)

1. Configure s3cmd for DO Spaces:

```bash
s3cmd --configure
```

2. Run a one-off backup with remote sync:

```bash
sudo PULSE_S3CMD_BUCKET="s3://your-bucket" /opt/pulse-cloud/backup.sh
```

### Cron

```bash
sudo crontab -e
```

Add:

```cron
0 3 * * * /opt/pulse-cloud/backup.sh
```

### Restore A Single Tenant

Assume:

- Tenant id: `t-ABCDEFGHJK`
- Backup day: `2026-02-10`

Steps:

1. Stop the tenant container.
2. Restore the tenant dir from backup.
3. Start the container.
4. Verify health.

```bash
TENANT="t-ABCDEFGHJK"
DAY="2026-02-10"

docker stop -t 30 "pulse-${TENANT}"
rsync -a --delete "/data/backups/daily/${DAY}/tenants/${TENANT}/" "/data/tenants/${TENANT}/"
docker start "pulse-${TENANT}"
```

## Rollout New Version

The architecture requires digest-pinned sequential rollouts with snapshot-before-update and rollback-on-failure.

- Script: `/opt/pulse-cloud/rollout.sh` (source: `deploy/cloud/rollout.sh`)

### Roll Out A New Digest

Preferred: pass a full image ref:

```bash
sudo /opt/pulse-cloud/rollout.sh "ghcr.io/rcourtman/pulse@sha256:..."
```

Alternative: pass only `sha256:...` (repo inferred from `CP_PULSE_IMAGE` in `/opt/pulse-cloud/.env`):

```bash
sudo /opt/pulse-cloud/rollout.sh "sha256:..."
```

Behavior:

- Pre-pulls the new image
- Iterates tenants sequentially (first tenant is canary)
- Stops the tenant (30s timeout)
- Snapshots `/data/tenants/<id>/` after stop
- Recreates the container with the new image
- Health checks `GET /api/health` via container IP on the `pulse-cloud` network
- On failure: restores snapshot, brings back the prior container, halts the rollout

### Rollback Procedure (Manual)

If a rollout is halted:

- The script leaves the pre-rollout container as `pulse-<tenant>.pre-rollout-<run_id>`
- Snapshots are under `/data/backups/rollout/<run_id>/`

To force rollback for a tenant:

1. Stop and remove the new container.
2. Restore snapshot to `/data/tenants/<id>/`.
3. Rename and start the pre-rollout container back to `pulse-<id>`.

## Scale Up

### Phase 2: Upgrade Droplet (Vertical Scale)

When approaching the Phase 1 comfort limit, upgrade to a larger droplet:

- Stop heavy workloads during resize
- Validate post-resize:
  - Traefik routes and TLS
  - Control plane `/readyz`
  - Tenant `/api/health`

### Phase 3: Migrate To Kubernetes (Horizontal Scale)

Per the architecture doc, do not pre-build Kubernetes.

Trigger conditions:

- Tenant count and/or workload makes vertical scaling insufficient
- You need scheduling/eviction/autoscaling characteristics not available with single-host Docker

## Debugging

### Containers and Logs

```bash
docker compose -f /opt/pulse-cloud/docker-compose.yml ps
docker logs --tail 200 traefik
docker logs --tail 200 control-plane

docker ps --filter "label=pulse.managed=true" --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}'
docker logs --tail 200 "pulse-t-ABCDEFGHJK"
```

### Common Issues

- TLS not issuing:
  - Ensure `DO_AUTH_TOKEN` is valid and has DNS permissions
  - Ensure DNS zone is hosted on DigitalOcean
  - Check Traefik logs for DNS-01 errors
- Control plane 5xx:
  - Check `docker logs control-plane`
  - Check `/data/control-plane/tenants.db` exists and is writable by the container
- Tenant unreachable:
  - Ensure container has Traefik labels and is on `pulse-cloud` network
  - Check tenant `/api/health` using container IP from `docker inspect`

## Emergency Procedures

### Kill A Runaway Tenant Container

```bash
docker kill "pulse-t-ABCDEFGHJK"
docker rm -f "pulse-t-ABCDEFGHJK"
```

If you need to bring it back, use the control plane to recreate it, or restore from backup and recreate the container using the control plane provisioning path.

### Force-Stop All Tenants

```bash
docker ps --filter "label=pulse.managed=true" -q | xargs -r docker stop -t 30
```

### Roll Back A Bad Deploy (Control Plane / Traefik)

Control plane and Traefik are managed by compose:

```bash
cd /opt/pulse-cloud
docker compose pull
docker compose up -d
docker compose ps
```
