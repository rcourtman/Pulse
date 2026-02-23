# Pulse Cloud Operations Runbook (DigitalOcean, Single-Droplet Phase 1)

This runbook is for the Phase 1 architecture: a single DigitalOcean droplet running:

- Traefik as the public edge (`:80`/`:443`) with Cloudflare DNS-01 wildcard TLS
- Pulse Control Plane behind Traefik at `https://<DOMAIN>/`
- One Docker container per tenant (`pulse-<tenant_id>`) on the `pulse-cloud` Docker network

Secrets and credentials are stored separately and not checked into this repository.

## Deploy From Scratch

### Prerequisites

- DigitalOcean droplet: Ubuntu 24.04 (size per capacity plan in `docs/architecture/cloud-container-per-tenant-2026-02.md`)
- Inbound ports allowed: `22/tcp`, `80/tcp`, `443/tcp`
- DNS hosted in Cloudflare (required for Traefik DNS-01 via `CF_DNS_API_TOKEN`)
- A Cloudflare API token with DNS edit permission for the zone
- Stripe secret key and a webhook signing secret

### DNS Setup

You need both the control-plane hostname and a wildcard for tenant hostnames:

- `cloud.pulserelay.pro` A record -> `<droplet_public_ip>`
- `*.cloud.pulserelay.pro` A record -> `<droplet_public_ip>` (wildcard)

You can configure this in:

- Cloudflare DNS dashboard (recommended)
- Or Cloudflare API tooling (optional)

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
- `CF_DNS_API_TOKEN` (Cloudflare token for DNS-01 challenges)
- `CP_ENV=production`
- `TRAEFIK_IMAGE` (digest-pinned Traefik image ref)
- `CONTROL_PLANE_IMAGE` (digest-pinned control-plane image ref)
- `CP_ADMIN_KEY` (control plane admin API key)
- `CP_PULSE_IMAGE` (digest-pinned tenant image ref)
- `CP_TRIAL_SIGNUP_PRICE_ID` (Stripe recurring price ID used for hosted trial checkout)
- `CP_ALLOW_DOCKERLESS_PROVISIONING=false`
- `CP_REQUIRE_EMAIL_PROVIDER=true`
- `RESEND_API_KEY` (required in production)
- `PULSE_EMAIL_FROM` (e.g. `noreply@pulserelay.pro`)
- `STRIPE_WEBHOOK_SECRET`
- `STRIPE_API_KEY`

Stripe key mode must match environment:

- `CP_ENV=production` -> `STRIPE_API_KEY` must be `sk_live_...`
- `CP_ENV=staging` -> `STRIPE_API_KEY` must be `sk_test_...`

Recommended rate-limit overrides (all are requests/minute per source IP):

- `CP_RL_WEBHOOK_PER_MINUTE` (default `120`)
- `CP_RL_MAGIC_VERIFY_PER_MINUTE` (default `30`)
- `CP_RL_SESSION_PER_MINUTE` (default `60`)
- `CP_RL_ADMIN_PER_MINUTE` (default `120`)
- `CP_RL_ACCOUNT_PER_MINUTE` (default `300`)
- `CP_RL_PORTAL_PER_MINUTE` (default `300`)

Lock down permissions:

```bash
sudo chmod 600 /opt/pulse-cloud/.env
```

### Verify After Deploy

From the droplet:

```bash
DOMAIN="$(grep '^DOMAIN=' /opt/pulse-cloud/.env | cut -d= -f2-)"
ADMIN_KEY="$(grep '^CP_ADMIN_KEY=' /opt/pulse-cloud/.env | cut -d= -f2-)"
curl -fsS -k --resolve "${DOMAIN}:443:127.0.0.1" "https://${DOMAIN}/healthz"
curl -fsS -k --resolve "${DOMAIN}:443:127.0.0.1" "https://${DOMAIN}/readyz"
curl -fsS -k --resolve "${DOMAIN}:443:127.0.0.1" "https://${DOMAIN}/status" -H "X-Admin-Key: ${ADMIN_KEY}" | jq
docker compose -f /opt/pulse-cloud/docker-compose.yml ps
```

### Launch Gate Preflight (Operator Machine)

Run the automated preflight from this repository before any launch/cutover:

```bash
cd deploy/cloud
./preflight-live.sh
```

Defaults:

- `DOMAIN=cloud.pulserelay.pro`
- `SSH_TARGET=root@cloud-host`
- `ADMIN_KEY_FILE=${XDG_CONFIG_HOME:-$HOME/.config}/pulse-cloud/admin_key`
- `EXPECT_CP_ENV=production`
- `EXPECT_STRIPE_MODE=live`
- `CHECK_STRIPE_CHECKOUT_PROBE=false`

Override any of these env vars when validating staging or a different host.

### Staging (Stripe Test Mode)

Use staging for full end-to-end hosted-signup and provisioning drills without real card charges.

Recommended Stripe setup:
- Use a dedicated Stripe Sandbox (for example: `Pulse Staging`)
- Keep staging/test credentials separate from live credentials
- Store local copies in your local secret store, for example:
  - `${XDG_CONFIG_HOME:-$HOME/.config}/pulse-cloud/stripe/test_secret_key`
  - `${XDG_CONFIG_HOME:-$HOME/.config}/pulse-cloud/stripe/test_price_id`
  - `${XDG_CONFIG_HOME:-$HOME/.config}/pulse-cloud/stripe/test_webhook_secret`

1. Create staging env file from template:

```bash
cd /opt/pulse-cloud
cp .env.staging.example .env
chmod 600 .env
```

2. Set required staging values:
- `CP_ENV=staging`
- `STRIPE_API_KEY=sk_test_...`
- `STRIPE_WEBHOOK_SECRET=whsec_...` (from Stripe test-mode webhook endpoint)
- `CP_TRIAL_SIGNUP_PRICE_ID=price_...` (test-mode recurring price)

If your operator machine has local staging Stripe secret files, you can populate `.env` safely:

```bash
cd /opt/pulse-cloud
PULSE_STRIPE_SECRET_DIR="${PULSE_STRIPE_SECRET_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/pulse-cloud/stripe}"
sudo sed -i "s|^CP_ENV=.*|CP_ENV=staging|" .env
sudo sed -i "s|^STRIPE_API_KEY=.*|STRIPE_API_KEY=$(cat "${PULSE_STRIPE_SECRET_DIR}/test_secret_key")|" .env
sudo sed -i "s|^CP_TRIAL_SIGNUP_PRICE_ID=.*|CP_TRIAL_SIGNUP_PRICE_ID=$(cat "${PULSE_STRIPE_SECRET_DIR}/test_price_id")|" .env
sudo sed -i "s|^STRIPE_WEBHOOK_SECRET=.*|STRIPE_WEBHOOK_SECRET=$(cat "${PULSE_STRIPE_SECRET_DIR}/test_webhook_secret")|" .env
```

3. Run setup in staging mode:

```bash
cd /path/to/repo/deploy/cloud
sudo -E PULSE_CLOUD_EXPECT_ENV=staging PULSE_CLOUD_EXPECT_STRIPE_MODE=test bash setup.sh
```

4. Run staging preflight (includes checkout probe by default):

```bash
cd deploy/cloud
./preflight-staging.sh
```

`preflight-staging.sh` defaults:
- `DOMAIN=cloud-staging.pulserelay.pro`
- `SSH_TARGET=root@cloud-staging-host`
- `EXPECT_CP_ENV=staging`
- `EXPECT_STRIPE_MODE=test`
- `CHECK_STRIPE_CHECKOUT_PROBE=true`

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

### Control Plane Status (Admin-Key Protected)

```bash
DOMAIN="$(grep '^DOMAIN=' /opt/pulse-cloud/.env | cut -d= -f2-)"
ADMIN_KEY="$(grep '^CP_ADMIN_KEY=' /opt/pulse-cloud/.env | cut -d= -f2-)"
curl -fsS "https://${DOMAIN}/status" -H "X-Admin-Key: ${ADMIN_KEY}" | jq
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

### Session Logout (Control Plane)

`POST /auth/logout` revokes all active sessions for the authenticated user and clears `pulse_cp_session`.

### Prometheus Metrics (Admin-Key Protected by Default)

```bash
DOMAIN="$(grep '^DOMAIN=' /opt/pulse-cloud/.env | cut -d= -f2-)"
ADMIN_KEY="$(grep '^CP_ADMIN_KEY=' /opt/pulse-cloud/.env | cut -d= -f2-)"
curl -fsS "https://${DOMAIN}/metrics" -H "X-Admin-Key: ${ADMIN_KEY}" | head
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

If you already have an `account_id`, you must call workspace APIs using a valid control-plane session (magic link login). These account-scoped APIs are no longer admin-key authenticated in production.

```bash
echo "Use the cloud UI/session cookie, then call /api/accounts/{account_id}/tenants"
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
- Runs SQLite integrity checks (`PRAGMA quick_check`) on copied `.db` files
- Publishes backup health metric file: `/var/lib/node_exporter/textfile_collector/pulse_cloud_backup.prom`

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

### Monthly Restore Drill (Required)

Run a non-destructive restore drill from the latest backup:

```bash
DAY="$(date -u +%F)"
TENANT="t-ABCDEFGHJK"
sudo /opt/pulse-cloud/restore-drill.sh --day "${DAY}" --tenant "${TENANT}" --keep-output
```

If the day does not exist yet (before backup cron runs), use the latest snapshot day under `/data/backups/daily/`.

## Alerting

Import alert rules from:

- `/opt/pulse-cloud/prometheus-alerts.yml`

Rules cover:

- Stripe webhook 5xx failures
- Provisioning errors
- Elevated unhealthy tenant checks
- Backup failure/staleness

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
  - Ensure `CF_DNS_API_TOKEN` is valid and has DNS edit permissions
  - Ensure DNS zone is hosted on Cloudflare
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
