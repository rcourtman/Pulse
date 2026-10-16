# Pulse Security

This document is the canonical security policy for Pulse. It combines our
ongoing hardening guidance with the operational checklists that previously lived
in `docs/SECURITY.md`.

For a high-level overview of the system design and data flow, please refer to
[`ARCHITECTURE.md`](ARCHITECTURE.md).

---

## Critical Security Notice for Container Deployments

### Container SSH Key Policy (BREAKING CHANGE)

**Effective immediately, SSH-based temperature monitoring is blocked in
containerized Pulse deployments.**

#### Why This Change?

Storing SSH private keys inside Docker/LXC containers creates an unacceptable
risk in production environments:

- **Container compromise = infrastructure compromise** – if an attacker gains
  shell access to the Pulse container they obtain the SSH private keys used to
  reach your Proxmox hosts.
- **Keys persist in images** – private keys survive in image layers and can leak
  when images are pushed to registries or shared.
- **No key rotation** – long-lived keys inside containers are difficult to
  rotate safely.
- **Violates least-privilege** – monitoring containers should not hold
  credentials that grant host-level access to the infrastructure they observe.

#### Affected Deployments

✅ **Not affected** – Pulse installed directly on a VM or bare-metal host (no
containers), or homelab environments where you explicitly accept the risk.

❌ **Blocked** – Pulse running in Docker containers, LXC containers, or any
environment where `PULSE_DOCKER=true`/`/.dockerenv` is detected.

#### Migration Path (Production)

Preferred option (no SSH keys, no proxy wiring):

1. Install or upgrade the unified agent (`pulse-agent`) on each Proxmox host with Proxmox integration enabled.
   - Use **Settings → Infrastructure → Install on a host**. Keep the separately
     revealed credential out of the copied command; enter it only at the silent
     prompt on that host, or use the private token-file route in the
     [unified agent guide](docs/UNIFIED_AGENT.md#private-file-installation-linux-macos-and-nas).
   - For manual setup, download and inspect the **agent installer served by your
     Pulse instance**, using its verified HTTPS address, then install with
     `--token-file` and `--enable-proxmox`. GitHub's top-level `install.sh`
     installs the server, not the agent. Do not pipe an unchecked HTTP response
     into a privileged shell or disable certificate verification.

Legacy sensor proxy (removed):

- `pulse-sensor-proxy` is no longer supported. Migrate to `pulse-agent --enable-proxmox` or SSH-based collection.
- Cleanup steps are in `docs/TEMPERATURE_MONITORING.md`.

#### Removing Old SSH Keys

If you previously generated SSH keys inside containers:

First verify that agent-based temperature collection works and retain an
independent administrative login to each host. Identify the old monitoring
public key by its fingerprint, then remove **only that key's entry** from the
host's `authorized_keys`. A comment containing `pulse` is not proof of ownership.

Remove only the corresponding private-key files from the container and any
persistent mount that supplied them; do not use wildcard deletion or remove the
whole SSH directory. Deleting a container file does not erase an old image
layer or backup. Revoking the public key on every target host is the essential
step; replace affected images and protect or retire old backups separately.

#### Security Boundary

```text
┌─────────────────────────────────────┐
│  Proxmox Host                       │
│  ┌───────────────────────────────┐  │
│  │  pulse-agent                  │  │
│  │  · Reads sensors locally      │  │
│  │  · Sends metrics via HTTPS    │  │
│  └───────────────────────────────┘  │
│            │                         │
│            │ HTTPS + API token       │
│            │                         │
│  ┌─────────▼─────────────────────┐  │
│  │  Pulse (Docker/LXC container) │  │
│  │  · No SSH keys                │  │
│  │  · No host root privileges    │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

#### Homelab Exception

If you fully understand the risk and are **not** containerized (VM/bare-metal
install), the legacy SSH flow still works. Use a dedicated monitoring user,
restrict the key to the Pulse sensor wrapper with
`command="/usr/local/sbin/pulse-sensors"` and `from="<pulse-ip>"`, and
rotate keys regularly.

#### Auditing Your Deployment

Review the existing deployment's SSH keys and mounts privately with its
administrator. Checking one filename, or a failed file lookup, does not prove
that no key is present. Do not share private keys, full service definitions or
container inspections; those can contain credentials.

For temperature collection, check the affected host on **Proxmox** or
**Machines**, keeping its agent version, last activity and sensor observation
time separate. A reporting agent does not prove that every reading is fresh.
Use the [existing Linux temperature checks](docs/TEMPERATURE_MONITORING.md#check-existing-linux-readings-safely)
only when collection is safe. During a backup, freeze/thaw or an unresponsive-host
incident, defer live checks and follow [backup safety](docs/VM_DISK_MONITORING.md#backup-safety).

**Documentation:** [Critical security notice for container deployments](#critical-security-notice-for-container-deployments)
**Issues:** <https://github.com/rcourtman/pulse/issues>
**Private disclosures:** <security@pulserelay.pro>

---

## Mandatory Authentication

Authentication setup is prompted for all new Pulse installations. This protects your Proxmox API credentials from unauthorized
access.

> **Service name note:** systemd deployments use `pulse.service`. If you're
> upgrading from an older install that still registers `pulse-backend.service`,
> substitute that name in the commands below.

### First-Run Security Setup
When you first access Pulse, you'll be guided through a mandatory security
setup:
- Create your admin username and password
- Automatic API token generation for automation
- Settings are applied immediately without restart
- **Your existing nodes and settings are preserved**

## Smart Security Context

### Public Access Detection
Pulse automatically detects when it's being accessed from public networks:
- **Private networks**: local/RFC1918 addresses (192.168.x.x, 10.x.x.x, etc.)
- **Public networks**: any non-private IP address
- **Stronger warnings**: red alerts when accessed from public IPs without
  authentication

### Trusted Networks Configuration (Deprecated)
**Note:** authentication is now mandatory regardless of network location.

Legacy configuration (no longer applicable):
```bash
# Environment variable (comma-separated CIDR blocks)
PULSE_TRUSTED_NETWORKS=192.168.1.0/24,10.0.0.0/24

# Or in systemd
sudo systemctl edit pulse
[Service]
Environment="PULSE_TRUSTED_NETWORKS=192.168.1.0/24,10.0.0.0/24"
```

When configured:
- Access still requires authentication (no bypass).
- The trusted list only influences security posture warnings and diagnostics.

## Security Warning System

Pulse includes a non-intrusive security warning system that helps you
understand your security posture.

### Security Score

**Settings → Security → Overview** summarises authentication methods,
API-token configuration, export protection, HTTPS and audit logging. These
indicators describe configuration checks, not a complete security audit. A
strong score does not establish that every file is encrypted, a backup is safe
to share, or stored credentials can be recovered without their encryption key.

### Dismissing Warnings
If you're comfortable with your security setup, you can dismiss warnings:
- **For 1 day** – reminder tomorrow
- **For 1 week** – reminder next week
- **Forever** – won't show again

## Credential Security

### Encrypted at Rest (AES-256-GCM)

Pulse encrypts configuration saved through its credential store. Examples in
the **active data directory** include:

| Stored configuration | File |
| --- | --- |
| Proxmox VE, PBS and PMG connection credentials | `nodes.enc` |
| SMTP settings, including passwords | `email.enc` |
| Webhook URLs and authentication headers | `webhooks.enc` |
| TrueNAS and vSphere connection credentials | `truenas.enc`, `vmware.enc` |
| AI provider credentials and SSO configuration | `ai.enc`, `sso.enc` |

The directory is normally `/etc/pulse` for systemd or `/data` inside Docker;
`PULSE_DATA_DIR` and deployment configuration can select another location.
Use the existing deployment's actual data path, not an assumed default.

### Encryption scope and recovery

The auto-generated **`.encryption.key`** is stored alongside these files. It
is key material, not an encrypted credential file. Anyone with the encrypted
files **and their matching key** can decrypt them. Protect the entire data
directory, mounted volume and recovery copies; encryption does not protect
against an account or process that can read both. Pulse protects its key file
with owner-only permissions (`0600`, with a `0700` parent directory); preserve
the intended service ownership rather than granting wider access to fix a read
failure.

This is not whole-directory or whole-disk encryption. Deployment environment
files, service definitions, agent token files, logs and history can still
contain secrets or identifying details. Quick Security Setup saves a bcrypt
password hash, but hashing an externally supplied plaintext `PULSE_AUTH_PASS`
for use does not rewrite its original deployment source. Keep hashes and raw
client credentials private too; see [private authentication configuration](docs/CONFIGURATION.md#private-docker-authentication-file).

Keep encrypted files and their matching key together in a **private recovery
backup**, not in a public attachment. Do not delete or regenerate the key, mix
files from different instances, or reset the data directory to repair a
decryption error. Preserve the existing files and reconcile the data path and
matching backup first. A file copy alone is not verified recovery; follow the
[server migration and restore guidance](docs/MIGRATION.md).

### Security Features
- **Logs**: targeted masking and redaction are not a guarantee that every
  output is safe to share. Free-text errors and historical records can contain
  secrets or private details. Review excerpts locally; follow
  [safe diagnostics collection](docs/TROUBLESHOOTING.md#collect-diagnostics-safely)
  and the [log-sharing precautions](docs/TROUBLESHOOTING.md#inspect-notification-logs).
- **API**: ordinary configuration reads redact stored credentials; newly issued
  tokens are deliberately revealed for their owner to save securely
- **Export**: requires authentication (session, proxy auth, or `X-API-Token`
  header) to extract credentials
- **Migration**: use passphrase-protected export/import (see
  [Migration Guide](docs/MIGRATION.md))
- **Auto-migration**: supported legacy credential files migrate when loaded;
  this does not scrub earlier backups or external deployment configuration

## Export/Import Protection

Configuration transfer requires management authority and a passphrase. Export
and import have different permissions; a successful public health check does
not establish either. Prefer the signed-in UI and keep the encrypted export and
its passphrase separate and private.

### Option 1: Create an API Token (Recommended)
Create a dedicated token in **API Access** with `settings:read` for export, or
`settings:write` for import. Use the [private-file export example](#usage) below;
do not paste a token or export passphrase into a command. An organization-bound
token can transfer only its selected organization, not the whole instance.

### Option 2: Allow Unprotected Export (Homelab)
```bash
# Using systemd
sudo systemctl edit pulse
# Add:
[Service]
Environment="ALLOW_UNPROTECTED_EXPORT=true"

# Docker: set ALLOW_UNPROTECTED_EXPORT=true in the existing deployment's
# environment, without changing its image, data volumes or other settings.
```

This exception applies only when Pulse has no configured authentication and
only to configuration export. It never enables import and never overrides
password, proxy, API-token, SSO, or hosted authentication. Without the
exception, unauthenticated export and import recovery is limited to a direct
loopback connection; private-network and forwarded requests are not loopback.

**Note:** for production, prefer Docker secrets or systemd environment files
for sensitive data.

## Security Features Summary

### Core Protection
- **Encryption**: credential-store files use AES-256-GCM, within the
  [scope and recovery limits](#encryption-scope-and-recovery) above
- **Export protection**: exports always encrypted with a passphrase
- **Minimum passphrase**: 12 characters required for exports
- **Security tab**: check status in *Settings → Security → Overview*

### Advanced Security (When Authentication Enabled)
- **Password security**
  - Bcrypt hashing with cost factor 12 (60‑character hash)
  - Quick Security Setup persists the hash, not the password; external
    deployment sources remain separately managed
  - Automatic hashing during security setup
  - **Critical**: bcrypt hashes must be exactly 60 characters
- **API token security**
  - 64‑character hex tokens (32 bytes entropy)
  - SHA3-256 hashed before storage (64‑character hash)
  - Raw token shown only once
  - Pulse-issued token records store verification hashes; raw client and agent
    token copies must remain private
  - Stored in `api_tokens.json` and managed via the UI
  - API-only mode supported (no password auth required)
- **CSRF protection**: session-authenticated state changes require CSRF tokens;
  API-token requests use their enforced scopes rather than a copied session cookie
- **Rate limiting**
  - Auth endpoints: 10 attempts/minute per IP
  - Config changes: 30 requests/minute per IP
  - Exports: 5 requests per 5 minutes per IP
  - Recovery operations: 3 requests per 10 minutes per IP
  - Update checks/actions: 60 requests/minute per IP
  - WebSocket connects: 30 requests/minute per IP
  - General API: 500 requests/minute per IP
  - Public endpoints: 1000 requests/minute per IP
  - 429 responses include rate limit headers:
    - `X-RateLimit-Limit`: Maximum requests per window
    - `X-RateLimit-Remaining`: Requests remaining in current window
    - `X-RateLimit-Reset`: Window reset timestamp
    - `Retry-After`: Seconds to wait before retrying (on 429 responses)
- **Account lockout**
  - Locks after 5 failed login attempts
  - 15-minute automatic lockout duration
  - Clear feedback showing remaining attempts
  - Time remaining displayed when locked
  - Manual reset available via API for admins
- **Session management**
  - Secure HttpOnly cookies
  - 24-hour session expiry (30 days when "Remember me" is enabled)
  - Session invalidation on password change
- **Security headers**
  - Content-Security-Policy
  - X-Frame-Options: `DENY` by default (adjusted when `allowEmbedding` is enabled in system settings)
  - X-Content-Type-Options: nosniff
  - X-XSS-Protection: 1; mode=block
  - Referrer-Policy: strict-origin-when-cross-origin
  - Permissions-Policy restricting sensitive APIs
- **Audit logging**
  - Authentication events include IP addresses
  - Rollback actions are logged with timestamps and metadata
  - Scheduler health escalations recorded in audit trail
  - Runtime logging configuration changes tracked
  - Security status reflects whether persistent audit logging is active (Pulse Pro)

### What's Encrypted in Exports
The entire configuration bundle is passphrase-encrypted, including node and
PBS credentials, email passwords, webhook authentication, hostnames, addresses,
thresholds, alert rules and schedules. The response's `status` wrapper is not
encrypted. Encryption is not redaction: anyone with the bundle and passphrase
can recover the included credentials and infrastructure details.

## Authentication Workflows

Pulse supports multiple authentication methods that can be used independently or
together.

> **Note**: `DISABLE_AUTH` is deprecated and no longer disables authentication. Remove it from your environment and restart if it's still present.

### SSO / Single Sign-On

Pulse supports **OIDC** and **SAML** SSO providers with multi-provider configuration:

- **OIDC**: Google, Authentik, Keycloak, Auth0, or any compliant provider.
- **SAML**: For enterprise IdPs that use SAML assertions.
- Multiple providers can be enabled simultaneously; the login page shows all available SSO buttons.
- Configure via **Settings → Security → Single Sign-On** (admin required).

See `docs/PROXY_AUTH.md` for proxy-based auth (Authentik, Authelia, Cloudflare).

### Password Authentication

#### Quick Security Setup (Recommended)
1. Navigate to *Settings → Security → Authentication*.
2. Click **Setup**.
3. Enter username and password.
4. Save the generated API token (shown only once!).
5. Security is enabled immediately (no restart needed).

This automatically:
- Hashes the password you chose
- Hashes it with bcrypt (cost factor 12)
- Creates secure API token (SHA3-256 hashed, raw token shown once)
- For systemd: Configures systemd with hashed credentials
- For Docker: Saves to `/data/.env` with hashed credentials (properly quoted to prevent shell expansion)
- Applies credentials immediately and persists them for future restarts

#### Manual Setup (Advanced)
Prefer Quick Security Setup so Pulse hashes and persists the password without
putting it in process arguments. For a deployment-managed headless instance,
edit a private environment file locally: set `PULSE_AUTH_USER` and a bcrypt hash
in `PULSE_AUTH_PASS`, using your deployment manager's file syntax. Keep the file
mode `0600` in a directory mode `0700`, outside repositories and diagnostics.

Have systemd read that file with `EnvironmentFile=`, or Docker Compose with
`env_file`, preserving the deployment's existing data volumes. Do not use
`docker run -e PULSE_AUTH_PASS=...` or a password-bearing shell assignment. An
environment file keeps the hash out of shell history and command arguments,
but does not hide it from the service or privileged inspection of its
environment. Treat hashes as sensitive too. Deployment environment values
override Pulse's generated authentication file; see
[password recovery](docs/TROUBLESHOOTING.md#i-forgot-my-password) before changing them.

#### Features
- Web UI login required when authentication enabled
- Change/remove password from Settings → Security → Authentication  
- Passwords ALWAYS hashed with bcrypt (cost 12)
- Session-based authentication with secure HttpOnly cookies
- 24-hour session expiry
- CSRF protection for session-authenticated state-changing operations
- Session invalidation on password change

### API Token Authentication  
For programmatic access and automation. API tokens are SHA3-256 hashed for security.

#### Token Setup via Quick Security
The Quick Security Setup automatically:
- Generates a cryptographically secure token
- Hashes it with SHA3-256
- Stores only the 64-character hash
- Adds the token to the managed token list

#### Manual Token Setup (Legacy Seeding)
Manage scoped tokens through **API Access**, not shared startup credentials.
Legacy `API_TOKEN` / `API_TOKENS` seeding is not the recommended setup route;
entries in Pulse's generated `.env` are ignored at runtime in v6. Hashing a
legacy token for `api_tokens.json` does not erase a raw value from a deployment
file, shell history or process environment. Rotate exposed credentials rather
than assuming a later hash removed those copies.

#### Token Management (Settings → API Tokens)
- Issue dedicated tokens for automation/agents without sharing a global credential
- View prefixes/suffixes and last-used timestamps for auditing
- Revoke tokens individually without downtime
- Regenerate tokens when rotating credentials (new value displayed once)
- All tokens stored as SHA3-256 hashes

#### Usage

For automation, follow the
[API guide's private header-file preparation and `pulse_api` helper](docs/API.md#api-token-recommended)
on the machine running curl. Define the helper in the **same Bash session**
before using these examples; it is local example code, not an installed Pulse
command. It supports either `X-API-Token` or Bearer authentication without
putting the token in arguments and requires curl 7.76 or later.

The helper makes **one request**, prints only its HTTP status and private
response-file path, and saves the body in a new owner-only file. Open that file
privately, including on failure; configuration, infrastructure and error bodies
can contain sensitive information. Share only a locally reviewed, redacted
excerpt, not the full response or header file.

Check protected access with a `monitoring:read` token:

```bash
pulse_api GET /api/state/summary
```

`/api/health` is public, so a successful response there does **not** validate a
token. The helper's loopback origin applies only on the Pulse host and explicitly
bypasses proxy environment settings, which `--disable` alone does not ignore.
For remote access, follow the API guide's **HTTPS origin and protocol changes
in the helper**, using a separately verified private CA where necessary, not
`--insecure`. Keep certificate verification, `--disable` first, `--globoff`,
the five-second connection and twenty-second total deadlines, and no redirects
or automatic retries. Do not add verbose/trace output.

For an encrypted configuration export, use a separate `settings:read` token in
the header file. Prepare the private request file without placing the
passphrase in a command:

```bash
(
  set -eu
  umask 077
  auth_dir="$HOME/.config/pulse"
  request_file="$auth_dir/export-request.json"
  if [ -L "$HOME/.config" ] || [ -L "$auth_dir" ] || [ -L "$request_file" ]; then
    printf 'Refusing a symlinked credential path.\n' >&2
    exit 1
  fi
  if [ -e "$request_file" ] && [ ! -f "$request_file" ]; then
    printf 'Request file must be a regular file.\n' >&2
    exit 1
  fi
  mkdir -p "$auth_dir"
  chmod 700 "$auth_dir"
  touch "$request_file"
  chmod 600 "$request_file"
  vi "$request_file"
)
```

In the editor, save this JSON with a strong, unique passphrase of at least
12 characters. The placeholder below is not a passphrase to reuse:

```json
{"passphrase":"replace-with-a-strong-unique-passphrase"}
```

Preparation preserves an existing file and stops on failure; do not continue
with a partial or unedited request. Then send the private file through the
same helper:

```bash
pulse_api POST /api/config/export < "$HOME/.config/pulse/export-request.json"
```

The successful response contains the encrypted bundle in `data`; it is not a
plain configuration file. A failed or partial response may contain an error
instead of a bundle: never import it or treat file creation or HTTP success
alone as a verified backup. Inspect the saved response privately before using
the [configuration-transfer procedure](docs/MIGRATION.md#transfer-the-included-configuration).
Keep the passphrase separate from the export. Remove the temporary request
file when no longer needed; do not include it in a diagnostics archive.

Configuration export accepts a token with `settings:read`; import accepts a
token with `settings:write`. Both `X-API-Token` and `Authorization: Bearer`
forms are supported, and organization-bound tokens can transfer only the
organization selected by the request.

### Scoped API Tokens

API tokens can be scoped to limit access. Available scopes:

| Scope | Purpose |
|---|---|
| `monitoring:read` | Read resource data, metrics, charts |
| `monitoring:write` | Update metadata, trigger discovery |
| `settings:read` | Read configuration, export |
| `settings:write` | Modify settings, import, manage nodes |
| `ai:chat` | Use the AI chat assistant |
| `ai:execute` | Run AI commands, view patrol findings |
| `docker:report` | Docker agent metric reporting |
| `kubernetes:report` | Kubernetes agent metric reporting |
| `agent:report` | Agent metric reporting |
| `docker:manage` | Docker host management actions |
| `kubernetes:manage` | Kubernetes cluster management actions |
| `agent:manage` | Agent configuration updates |

Endpoints enforce scope checks before processing. A token without the required scope receives `403 Forbidden`.

### Auto-Registration Security

#### Default Mode
- All access requires authentication
- Nodes can auto-register with the API token
- Setup scripts work without additional configuration

#### Secure Mode
- Require API token for all operations
- Protects auto-registration endpoint
- Enable by creating at least one API token (UI or legacy env seeding)

### Runtime Logging Configuration

Start with existing evidence at the current log level and the
[bounded Pulse log readers](docs/TROUBLESHOOTING.md#inspect-notification-logs).
**Debug is not a prerequisite for a report.** It can produce much more output,
including private names, paths and raw guest-filesystem responses. Do not enable
it, repeat the failed action or run Diagnostics just to fill an evidence gap.
If the interface stalls after enabling Debug, return to the previous working
level through the existing deployment's log-level control; do not reproduce the
stall. A responsive interface afterwards is not proof of underlying recovery.

The **Log Level** control in **Settings → Support → System Logs** changes the
server-wide threshold immediately, not just the browser's filter. Its saved
value is used at startup unless `LOG_LEVEL` overrides it. See
[log levels and display controls](docs/CONFIGURATION.md#log-levels).

#### Server logging environment settings

| Variable | Purpose | Default |
| --- | --- | --- |
| `LOG_LEVEL` | Startup threshold (`info`, `warn`, `error` or `debug`) | `info` |
| `LOG_FORMAT` | Output format (`auto`, `json` or `console`) | `auto` |
| `LOG_FILE` | Additional rotating file output | Unset |
| `LOG_MAX_SIZE` | File rotation size in MB | `100` |
| `LOG_MAX_AGE` | Rotated-file retention in positive days | `30` |
| `LOG_COMPRESS` | Compress rotated files | `true` |

File rotation and compression apply only when `LOG_FILE` is configured. These
settings do not bound the systemd journal, container logs or browser log buffer,
and do not configure the separate **Audit Log** retention. Zero or negative
`LOG_MAX_AGE` values do not disable cleanup: they are ignored with a startup
warning and the default applies. JSON formatting and compression do not redact
private information. Preserve incident evidence; do not delete logs or shorten
retention merely to complete a report.

Change only the intended settings in the existing deployment's managed source:

- **Systemd / Proxmox LXC:** edit the active service's environment file or
  drop-in privately. Reload the service manager if its unit or drop-in changed,
  then restart that service during a suitable maintenance window. For LXC,
  work inside the Pulse container, not on the Proxmox host.
- **Docker / Compose:** use the existing manager's recreate/redeploy operation
  for an environment change; `docker restart` does not apply it. Preserve the
  same image, persistent data mounts, credentials and other settings. Do not
  start a replacement `docker run` container just to change logging.

**Pause Stream** and **Clear Log Output** affect only the browser display; they
do not stop server logging, erase retained logs or remove them from a
**Support Bundle**. The bundle can include a complete log file, configuration
and environment information. Keep it private and share only relevant, locally
reviewed, redacted excerpts, not the whole archive. During backups, freeze/thaw
or an unresponsive-host incident, defer collection and follow
[safe diagnostics guidance](docs/TROUBLESHOOTING.md#collect-diagnostics-safely).

## CORS (Cross-Origin Resource Sharing)

By default, Pulse does **not** enable CORS (same-origin only). Configure allowed origins only when
you need cross-origin access (for example, a separate UI domain or external tooling).

### Configuring CORS for External Access

If you need to access the Pulse API from a different domain, configure **Settings → System → Network**
or use environment overrides:

```bash
# Docker
docker run -e ALLOWED_ORIGINS="https://app.example.com" rcourtman/pulse:latest

# systemd
sudo systemctl edit pulse
[Service]
Environment="ALLOWED_ORIGINS=https://app.example.com"

# Development (allow localhost)
ALLOWED_ORIGINS="http://localhost:5173"
```

Notes:

- `ALLOWED_ORIGINS` supports a single origin or `*` (it is written directly to `Access-Control-Allow-Origin`).
- In production, set a specific origin to avoid exposing the API to arbitrary sites.
- For local dev, Pulse auto-allows `http://localhost:5173` and `http://localhost:7655` when `NODE_ENV=development` or `PULSE_DEV=true`.

## Monitoring and Observability

### Scheduler Health API

#### Endpoint
```bash
pulse_api GET /api/monitoring/scheduler/health
```

Use the [private header-file and helper setup](#usage) with a `monitoring:read`
token. Read the saved response privately. The non-zero HTTP-error exit matters;
piping an unauthenticated error straight to `jq` can make a failed request look
successful.

#### Security Use Cases
1. **Anomaly Detection**
   - Watch for unusual queue depths (possible DoS)
   - Monitor circuit breaker trips (connectivity issues or attacks)
   - Track backoff patterns (rate limiting, potential probes)

2. **Performance Monitoring**
   - Identify performance degradation
   - Detect resource exhaustion
   - Track API response times

3. **Incident Response**
   - Real-time visibility into system health
   - Historical metrics for post-incident analysis
   - Circuit breaker status for failover decisions

#### Key Security Metrics
- **Queue Depth**: High values may indicate attack or overload
- **Circuit Breaker Status**: Half-open/open states suggest connectivity issues
- **Backoff Delays**: Increased backoff may indicate rate limiting or errors
- **Error Rates**: Track failed API calls and authentication attempts

Use existing status and incident evidence first. The endpoint above is a
read-only status check, not proof that every workload or collector is healthy.
Collect new evidence only when Pulse is running and collection is safe; follow
[safe diagnostics guidance](docs/TROUBLESHOOTING.md#collect-diagnostics-safely)
rather than running Diagnostics during backups, freeze/thaw or an
unresponsive-host incident. Review responses locally before sharing them.

### Existing Mobile Pairings (Retirement)

Pulse Mobile and Relay retire on **31 March 2027**. Existing paired phones keep
working until then; Relay is no longer sold, and existing Relay subscribers
receive Pro features at their current price. Relay connects the app, not the
web UI. For alerts afterwards, use an ntfy, Gotify or Pushover destination and
open Pulse in the phone's browser.

- **ECDH key exchange**: Existing app channels derive end-to-end encryption keys;
  the relay server does not see plaintext payloads.
- **Per-channel authentication**: Each mobile session authenticates independently.
- **Back-pressure**: Data limiters prevent channel flooding.
- **Existing access**: Paired-app access remains license-gated until retirement.

### Agent Command Security

**Agent commands are disabled by default.** This prevents the AI subsystem from executing arbitrary commands on monitored hosts.

- Operators must explicitly opt in with `--enable-commands` on the agent.
- Even when enabled, commands require `ai:execute` scope and admin privileges.
- All command executions are logged to the audit trail.
- Circuit breakers automatically halt execution when error thresholds are exceeded.

## Security Best Practices

### Credential Storage
- ✅ **DO**: Use Quick Security Setup for automatic hashing
- ✅ **DO**: Store only bcrypt hashes for passwords
- ✅ **DO**: Store only SHA3-256 hashes for API tokens
- ❌ **DON'T**: Put raw passwords or tokens in command arguments, URLs, logs or
  shared configuration files
- ✅ **DO**: Keep an agent's required raw credential and temporary API request
  files private; protect encrypted backups and keep their passphrases separate

### Authentication Setup
- ✅ **DO**: Use strong, unique passwords (16+ characters)
- ✅ **DO**: Rotate API tokens periodically
- ✅ **DO**: Use HTTPS in production environments
- ❌ **DON'T**: Share API tokens between users/services
- ❌ **DON'T**: Embed credentials in client-side code

### Verification Checklist

Review the existing deployment privately; a successful check is not a complete
security audit, and absence from one excerpt is not proof that no secret leaked.

- Protect the active credential source and its backups, including any
  deployment-supplied plaintext password. Follow
  [private authentication configuration](docs/CONFIGURATION.md#private-docker-authentication-file),
  rather than posting environment files or replacing a working installation.
- Server-side API token records contain SHA3-256 hashes; agents still require
  a private raw credential. Neither a token's shape nor a partial hint match
  proves that it is the credential used by a particular running process.
- Preserve the intended service ownership and owner-only access to sensitive
  files in the actual data directory. Review the
  [encryption and recovery limits](#encryption-scope-and-recovery); do not grant
  wider permissions or delete keys to repair a read failure.
- Review relevant existing logs, API responses and screenshots locally before
  sharing a redacted excerpt. Use the [bounded log readers](docs/TROUBLESHOOTING.md#inspect-notification-logs),
  not full logs, Support Bundles, service environments or network exports.
  Do not extract browser cookies or use **Copy as cURL** to create evidence.
- If private information was posted, use
  [disclosure containment](docs/TROUBLESHOOTING.md#if-you-already-posted-private-information).
  Removing the public copy does not revoke an exposed credential.

## Account Lockout and Recovery

### Lockout Behavior
- After **5 failed login attempts**, the account is locked for **15 minutes**
- Lockout applies to both username and IP address
- Login form shows remaining attempts after each failure
- Clear message when locked with time remaining

### Automatic Recovery
- Lockouts automatically expire after 15 minutes
- No action needed - just wait for the timer to expire
- Successful login clears all failed attempt counters

### Manual Recovery (Admin)
Administrators with API access can manually reset a temporary lockout; this does
not reset a password or bypass SSO. Use the [private header file and helper](#usage)
with a `settings:write` token. Session-authenticated API requests also require CSRF
protection; do not copy a session cookie into a command.

Choose only the identifier that needs resetting; these are separate examples,
not a batch. A timeout or lost response can occur **after the reset was applied**.
Check the existing lockout state before deciding whether another request is
needed; do not repeat a reset merely because its response is incomplete. A
saved file or a 2xx status does not prove that login has recovered.

```bash
# Reset lockout for a specific username
pulse_api POST /api/security/reset-lockout <<'JSON'
{"identifier":"username"}
JSON

# Reset lockout for an IP address
pulse_api POST /api/security/reset-lockout <<'JSON'
{"identifier":"198.51.100.100"}
JSON
```

## Troubleshooting

**Account locked?** Wait 15 minutes or contact admin for manual reset  
**Export blocked?** Authenticate with management authority, use a correctly scoped API token, connect directly over loopback on a no-auth installation, or deliberately set `ALLOW_UNPROTECTED_EXPORT=true` for export only<br>
**Rate limited?** Wait 1 minute and try again  
**Can't login?** Check `PULSE_AUTH_USER` and `PULSE_AUTH_PASS` environment variables  
**API access denied?** Verify the token you supplied matches one of the values created in *Settings → API Tokens* (use the original token, not the hash)  
**CORS errors?** Configure Allowed Origins in the UI or set `ALLOWED_ORIGINS` for your domain  
**Forgot password?** Follow the [deployment-specific recovery guide](docs/TROUBLESHOOTING.md#i-forgot-my-password). Deployment-managed credentials and identity-provider accounts need their own recovery path; deleting `.env` is not a universal reset.

---
