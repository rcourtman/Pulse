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
runtime namespace. The stack is operated with the `pulse-control-plane`
binary:

```bash
pulse-control-plane provider-msp bootstrap --account-name "Your MSP" --owner-email you@example.com
pulse-control-plane provider-msp status
pulse-control-plane provider-msp backup
pulse-control-plane provider-msp recover   # restore workspaces from backup or disk
pulse-control-plane provider-msp preflight # pre-install environment checks
```

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

- **UI**: Settings → Reports.
- **API**: `GET /api/admin/reports/generate` (single resource) and
  `POST /api/admin/reports/generate-multi` (up to 50 resources per report),
  returning PDF or CSV. In shared-process mode, scope with `X-Pulse-Org-ID`
  or an org-bound token.

Report branding (logo + display name) supports a provider-wide default via
environment (`PULSE_REPORT_PROVIDER_BRAND_DISPLAY_NAME`,
`PULSE_REPORT_PROVIDER_BRAND_LOGO_PATH` or `..._LOGO_BASE64` +
`..._LOGO_FORMAT`) plus a settings-based override. In the provider-hosted
model each client runtime has its own settings, so the override is
per-client; in shared-process mode the settings override applies
instance-wide, so all organizations share one brand (usually yours). Branding
requires the `white_label` entitlement on the licence.

Pulse does not yet schedule recurring reports; generate monthly client reports
on demand from the UI, or call the report API from your own scheduler with an
org-bound token.

## Licensing

MSP and Enterprise capabilities (`multi_tenant`, `unlimited`, `white_label`)
are carried on the licence key. MSP plans are sized by client workspace count
(Starter 5, Growth 15, Scale 40); workspace creation is blocked, not billed,
when the limit is reached. MSP and Enterprise keys are issued through sales —
contact support to get set up or to join the MSP design-partner program.
