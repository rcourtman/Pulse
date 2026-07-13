# Pulse for MSPs (Provider Operations Guide)

This guide covers running Pulse as a managed service provider: one central
deployment monitoring multiple client estates, with per-client isolation,
alert routing, and reporting. It assumes you have read
[DEPLOYMENT_MODELS.md](DEPLOYMENT_MODELS.md) for the deployment-model overview.

## Deployment models

**Provider-hosted MSP (canonical).** A control plane runs one isolated Pulse
runtime per client workspace. Alerts, webhook destinations, branded report
settings, users, audit history, and metrics stay inside the client runtime;
duplicate hostnames across clients never collide because they never share a
runtime namespace.

The canonical install is the deploy bundle at
[`deploy/provider-msp/`](../deploy/provider-msp/): a Docker Compose stack
(Traefik ingress with wildcard TLS, a hardened Docker socket proxy, and the
control plane), a guided `setup.sh` for fresh hosts, `upgrade.sh` for
backup-gated upgrades, and `run-install-proof.sh` for an end-to-end fresh
install proof. `.env.example` in that directory doubles as the operator
runbook. Start there rather than wiring containers by hand; among other
things the compose stack provides the `pulse.provider-msp.role=traefik` and
`pulse.provider-msp.role=control-plane` container labels that client
workspace provisioning requires for isolated tenant networking, and it
terminates TLS — the management portal sets a `__Host-` (HTTPS-only) session
cookie, so the portal does not work over plain HTTP.

Day-2 operations run through the `pulse-control-plane` binary (via
`docker compose run --rm control-plane …` in the bundle):

```bash
pulse-control-plane provider-msp bootstrap --account-name "Your MSP" --owner-email you@example.com
pulse-control-plane provider-msp status
pulse-control-plane provider-msp backup
pulse-control-plane provider-msp recover   # restore workspaces from backup or disk
pulse-control-plane provider-msp preflight # pre-install environment checks
```

### Portal sign-in and sessions

The management portal signs you in with one-time links, not passwords. With
no email provider configured (the bundle default), the portal cannot send
those links itself; the sign-in page says so and points at the host command
that prints one:

```bash
# Owner sign-in link (also safe to re-run any time; it never duplicates the account)
docker compose run --rm control-plane provider-msp bootstrap \
  --account-name "Your MSP" --owner-email you@example.com

# Sign-in link for an invited teammate
docker compose run --rm control-plane provider-msp portal-link --email teammate@example.com
```

Teammates are invited from the portal Access tab; without an email provider
the invitation email is not sent, so print their first sign-in link with
`portal-link` after inviting them. To let the portal send sign-in links and
invitations itself, set `RESEND_API_KEY` (plus `PULSE_EMAIL_FROM` and
`PULSE_EMAIL_REPLY_TO`) in `.env` and restart the control plane.

Portal sessions last 7 days on provider-hosted control planes; override with
`CP_SESSION_TTL` (Go duration, e.g. `12h`, `168h`).

Each client runtime is a normal Pulse instance, so it connects to that
client's infrastructure with the standard methods: agents push over HTTPS for
hosts, and Proxmox/PBS polling reaches across networks through your existing
VPN or tunnel to the client site.

**Shared-process organizations (alternative).** One Pulse process serves
multiple organizations with isolated data directories, org-bound tokens, and
per-org alert/webhook/notification state. This is documented in
[MULTI_TENANT.md](MULTI_TENANT.md) and gated by `PULSE_MULTI_TENANT_ENABLED=true`
plus a licence carrying the `multi_tenant` capability. It is designed for one
owner separating internal estates (sites, departments, environments); the
isolated-runtime model above is the canonical choice for separate customer
businesses.

## Network topology and ingress isolation

Run the management UI and agent check-in on separate, separately firewalled
ports. See [Split-Port Agent Ingest](CONFIGURATION.md#split-port-agent-ingest-network-isolation)
for the full reference.

```bash
FRONTEND_PORT=7655              # management UI + API: private network / VPN only
PULSE_AGENT_INGEST_PORT=7656    # agent check-in only: reachable from client sites
PULSE_AGENT_CONNECT_URL=https://agents.example.com:7656
```

Firewall baseline:

| Surface | Port | Reachable from |
|---------|------|----------------|
| Management UI + API | `FRONTEND_PORT` (7655) | Provider staff network / VPN only |
| Agent ingest | `PULSE_AGENT_INGEST_PORT` (7656) | Client sites (or client VPN tunnels) |
| Prometheus metrics | 9091 | Provider monitoring network only |

The dedicated agent port serves **only** `/api/agents/*`; every other path,
including login and the management API, returns `404`. Agent check-in
authenticates with an `agent:report`-scoped API token, which cannot read
monitoring data or change settings — the token scope and the port isolation
are independent layers.

If agents reach the central server over per-client VPN tunnels instead of the
public internet, the same split still applies: expose only the agent port into
the tunnels and keep the management port out of them.

### Validation checklist (run after setup, repeat after network changes)

1. **Agent port serves agent ingest only.** Both must return `404`:

   ```bash
   curl -sk -o /dev/null -w '%{http_code}\n' https://agents.example.com:7656/          # 404
   curl -sk -o /dev/null -w '%{http_code}\n' https://agents.example.com:7656/api/login # 404
   ```

2. **Management port is not reachable from a client site.** From a client
   network (or through a client tunnel), a connection to `FRONTEND_PORT` must
   time out or be refused by your firewall — not answer.

3. **Agent tokens cannot manage.** A request to a management endpoint with an
   agent token must be rejected:

   ```bash
   curl -sk -o /dev/null -w '%{http_code}\n' \
     -H "X-API-Token: <agent:report token>" https://pulse.internal:7655/api/notifications/webhooks  # 401/403
   ```

4. **Cross-tenant isolation (shared-process mode only).** A token bound to one
   organization must get `403` when targeting another organization AND when
   targeting the default org (a leaked client-site token must not read the
   provider's own estate):

   ```bash
   curl -sk -o /dev/null -w '%{http_code}\n' \
     -H "X-API-Token: <org-A token>" -H "X-Pulse-Org-ID: org-b" \
     https://pulse.internal:7655/api/alerts/active  # 403
   curl -sk -o /dev/null -w '%{http_code}\n' \
     -H "X-API-Token: <org-A token>" -H "X-Pulse-Org-ID: default" \
     https://pulse.internal:7655/api/alerts/active  # 403
   ```

   Keep your own monitoring estate in its own organization too, rather than
   in the default org, so every boundary in the instance is an explicit org
   boundary.

## Connecting a client's Proxmox or PBS over your VPN

Most MSP estates pair one or two Proxmox nodes per client site with a
site-to-site VPN or tunnel back to the provider network. Proxmox and PBS are
**polled**: the client's Pulse runtime reaches out to the client-site API
(port `8006` for PVE, `8007` for PBS) — nothing at the client site connects
inbound to the runtime for this. That inverts the agent direction, so check
both paths in your firewall:

| Traffic | Direction | Port |
|---------|-----------|------|
| Proxmox/PBS polling | provider → client site, through the tunnel | 8006 / 8007 |
| Agent check-in (hosts) | client site → provider agent ingest | `PULSE_AGENT_INGEST_PORT` (7656) |

Per client, the steps are:

1. Make the client's PVE/PBS API address reachable from the Docker host that
   runs the client workspaces (route or interface into that client's
   tunnel). From the host, `curl -sk https://<client-pve>:8006` should
   answer before you involve Pulse.
2. Open the client's workspace (portal → workspace → **Open**) and add the
   node under **Settings → Infrastructure**, using the tunnel-reachable
   address. The guided flow generates a setup command to run once on the
   client's Proxmox host (over SSH through the same tunnel); it creates the
   monitoring user, API token, and permissions. Self-signed certificates
   are handled automatically (the certificate fingerprint is pinned on
   first connect).
3. If you create the token by hand instead, note that Proxmox
   privilege-separated tokens need ACLs on the **token** as well as the
   user, and the built-in `PVEAuditor` role is not sufficient on its own —
   see [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for the exact role setup.

Because each client workspace is its own runtime, overlapping RFC1918
subnets across client sites never collide inside Pulse: each workspace only
ever dials its own client's tunnel addresses.

## Per-client alert routing

Configure notification destinations inside each client's scope — the client
runtime in the provider-hosted model, or the organization in shared-process
mode. A per-client Gotify server, Slack channel, or PSA endpoint only ever
sees that client's alerts.

Webhook targets on private IPs (a Gotify server reached over a VPN tunnel,
for example) are blocked by default for SSRF safety. Allow them once in
**Settings → System → Network → Webhook Security**; the allowlist is
instance-wide and applies to every organization, including ones created
later.

Alert webhook payloads carry the firing tenant's identity (`{{.TenantID}}`,
`{{.TenantName}}`), so a single central PSA endpoint can also route by client.
For ticket bridges (ConnectWise and similar), use the delivery contract —
stable severity/type fields, `X-Pulse-Event-ID` deduplication, and HMAC-signed
deliveries via `signingSecret` — documented in [WEBHOOKS.md](WEBHOOKS.md).

In the provider-hosted model, client runtimes receive `PULSE_TENANT_ID` and
`PULSE_TENANT_NAME` (the workspace display name) from the control plane, so
payloads carry a human-readable client label automatically. A display-name
change applies on the client runtime's next rollout, which recreates the
container. Shared-process organizations stamp the org ID and display name
automatically.

## Per-client reports

Each client runtime (or organization) generates its own reports, scoped to
that client's resources:

- **UI**: Settings → Data & Reports.
- **API**: `GET /api/admin/reports/generate` (single resource) and
  `POST /api/admin/reports/generate-multi` (up to 50 resources per report),
  returning PDF or CSV. In shared-process mode, scope with `X-Pulse-Org-ID`
  or an org-bound token.
- **Schedules**: `GET`/`POST /api/admin/reports/schedules`,
  `PUT`/`DELETE /api/admin/reports/schedules/{id}`, and
  `POST /api/admin/reports/schedules/{id}/run`. Schedules can target explicit
  resources and/or comma-separated resource tags, choose weekly or monthly
  cadence, and deliver PDF or CSV output by email or to disk.

Report branding (logo + display name) supports a provider-wide default via
environment (`PULSE_REPORT_PROVIDER_BRAND_DISPLAY_NAME`,
`PULSE_REPORT_PROVIDER_BRAND_LOGO_PATH` or `..._LOGO_BASE64` +
`..._LOGO_FORMAT`) plus a settings-based override. In the provider-hosted
model each client runtime has its own settings, so the override is
per-client; in shared-process mode the settings override applies
instance-wide, so all organizations share one brand (usually yours). Branding
requires the `white_label` entitlement on the licence.

Scheduled reports are tenant-local. In provider-hosted MSP, each client
runtime stores its own schedules in `report_schedules.json`, writes generated
outputs under `reports/generated/`, and applies its own SMTP settings,
recipients, resource tags, branding, and entitlement checks. If email delivery
is selected before SMTP is configured, Pulse records the run and saves the
report to disk instead of sending it. The Pulse Account portal may show whether
a workspace has an enabled report schedule, but it does not render cross-client
reports or collect report data in the provider control plane.

## Licensing

MSP and Enterprise capabilities (`multi_tenant`, `unlimited`, `white_label`)
are carried on the licence key. MSP plans are sized by client workspace count
(Starter 5, Growth 15, Scale 40); workspace creation is blocked, not billed,
when the limit is reached. MSP and Enterprise keys are issued through sales —
contact support to get set up or to join the MSP design-partner program.

In the provider-hosted model the licence is a signed file
(`CP_PROVIDER_MSP_LICENSE_FILE`) that also binds your control plane's
entitlement lease signing key:

1. `setup.sh` generates `CP_ENTITLEMENT_SIGNING_PRIVATE_KEY` locally; the
   private key never leaves your host.
2. Send the derived public key
   (`./setup.sh --print-lease-signing-public-key`) with your licence request.
3. The issued licence binds that key. The control plane refuses to start in
   provider mode if the licence and key do not match, so a misconfigured
   stack fails at startup instead of provisioning client workspaces that
   silently run unlicensed.

Client runtimes lease their entitlements from your control plane (the
control plane injects the refresh endpoint; nothing phones Pulse Cloud) and
verify each lease through the licence chain: Pulse's embedded key signs your
licence, your licence binds your signing key, your signing key signs the
lease. Leases carry the MSP capability set plus `white_label`, so branded
per-client reports work inside every client workspace. When the licence
expires, leases stop verifying after the grace period and client runtimes
fall back to Community behavior; renew and restart the control plane to
restore them.
