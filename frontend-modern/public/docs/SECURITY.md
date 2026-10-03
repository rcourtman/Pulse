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

```bash
# Detect vulnerable containers
ls /home/pulse/.ssh/id_ed25519* 2>/dev/null && echo "⚠️  SSH keys present"
```

Verify temperature collection is agent-based:

- UI: the Proxmox or Machines page shows the host as agent-backed after the agent reports.
- On each Proxmox host:
  ```bash
  systemctl status pulse-agent
  journalctl -u pulse-agent -n 200 --no-pager
  ```

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
Your instance receives a score from 0‑5 based on:
- ✅ Credentials encrypted at rest (always enabled)
- ✅ Export/import protection
- ⚠️ Authentication enabled
- ⚠️ HTTPS connection
- ⚠️ Audit logging

### Dismissing Warnings
If you're comfortable with your security setup, you can dismiss warnings:
- **For 1 day** – reminder tomorrow
- **For 1 week** – reminder next week
- **Forever** – won't show again

## Credential Security

### Encrypted at Rest (AES-256-GCM)
- **Node credentials**: passwords and API tokens (`/etc/pulse/nodes.enc`)
- **Email settings**: SMTP passwords (`/etc/pulse/email.enc`)
- **Webhook data**: URLs and auth headers (`/etc/pulse/webhooks.enc`)
- **Encryption key**: auto-generated (`/etc/pulse/.encryption.key`)

### Security Features
- **Logs**: token values masked with `***` in all outputs
- **API**: ordinary configuration reads redact stored credentials; newly issued
  tokens are deliberately revealed for their owner to save securely
- **Export**: requires authentication (session, proxy auth, or `X-API-Token`
  header) to extract credentials
- **Migration**: use passphrase-protected export/import (see
  [Migration Guide](docs/MIGRATION.md))
- **Auto-migration**: unencrypted configs automatically migrate to encrypted
  format

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
- **Encryption**: credentials encrypted at rest (AES-256-GCM)
- **Export protection**: exports always encrypted with a passphrase
- **Minimum passphrase**: 12 characters required for exports
- **Security tab**: check status in *Settings → Security → Overview*

### Advanced Security (When Authentication Enabled)
- **Password security**
  - Bcrypt hashing with cost factor 12 (60‑character hash)
  - Passwords never stored in plain text
  - Automatic hashing during security setup
  - **Critical**: bcrypt hashes must be exactly 60 characters
- **API token security**
  - 64‑character hex tokens (32 bytes entropy)
  - SHA3-256 hashed before storage (64‑character hash)
  - Raw token shown only once
  - Tokens never stored in plain text
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
- Configure via **Settings → Security → SSO Providers** (admin required).

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

For automation, first follow the
[API guide's private header-file preparation](docs/API.md#-authentication) on
the machine running curl. It supports either `X-API-Token` or Bearer
authentication without putting the token in arguments. Keep `--disable` first
to ignore local curl defaults that could enable credential-bearing trace
output; do not add verbose/trace options or follow redirects with credentials.
These requests need curl 7.76 or later.

Check protected access with a `monitoring:read` token:

```bash
curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  http://127.0.0.1:7655/api/state/summary
```

`/api/health` is public, so a successful response there does **not** validate a
token. The loopback URLs here apply only on the Pulse host. For remote access,
substitute your verified HTTPS address in each request; use a separately
verified private CA where necessary, not `--insecure`. Do not share unredacted
infrastructure responses, credential files or configuration exports.

For an encrypted configuration export, use a separate `settings:read` token in
the header file. Prepare the private request file without placing the
passphrase in a command:

```bash
umask 077
mkdir -p "$HOME/.config/pulse"
chmod 700 "$HOME/.config/pulse"
touch "$HOME/.config/pulse/export-request.json"
chmod 600 "$HOME/.config/pulse/export-request.json"
vi "$HOME/.config/pulse/export-request.json"
```

In the editor, save this JSON with a strong, unique passphrase of at least
12 characters. The placeholder below is not a passphrase to reuse:

```json
{"passphrase":"replace-with-a-strong-unique-passphrase"}
```

Then send the private file and save the response in a new private directory:

```bash
umask 077
export_dir=$(mktemp -d "$HOME/pulse-export.XXXXXX")
if curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  --request POST --header "Content-Type: application/json" \
  --data-binary "@$HOME/.config/pulse/export-request.json" \
  --output "$export_dir/config-export.json" \
  http://127.0.0.1:7655/api/config/export; then
  printf 'Export response saved to %s\n' "$export_dir/config-export.json"
else
  status=$?
  printf 'Export failed; do not import the response in %s\n' "$export_dir" >&2
  exit "$status"
fi
```

The successful response contains the encrypted bundle in `data`; it is not a
plain configuration file. A failed response may contain an error instead of a
bundle: never import it or treat file creation as success. Keep the passphrase
separate from the export. Remove the temporary request file when no longer
needed; do not include it in a diagnostics archive.

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

Pulse supports configurable logging (level, format, optional file output, rotation) via environment variables.

#### Security Benefits
- Enable debug logging temporarily for incident investigation
- Switch to JSON format for SIEM integration
- Adjust verbosity based on security posture
- Control file rotation to manage audit log retention

#### Configuration Options

**Via environment variables:**
```bash
# Systemd
sudo systemctl edit pulse
[Service]
Environment="LOG_LEVEL=info"
Environment="LOG_FORMAT=json"
Environment="LOG_MAX_SIZE=100"        # MB per log file
Environment="LOG_MAX_AGE=30"          # Days to retain logs
Environment="LOG_COMPRESS=true"       # Compress rotated logs

# Docker
docker run \
  -e LOG_LEVEL=info \
  -e LOG_FORMAT=json \
  -e LOG_MAX_SIZE=100 \
  -e LOG_MAX_AGE=30 \
  -e LOG_COMPRESS=true \
  rcourtman/pulse:latest
```

**Security Considerations:**
- Debug logs may contain sensitive data—enable only when needed
- JSON format recommended for security monitoring and SIEM
- Adjust retention based on compliance requirements
- Changes take effect on restart

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
curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  http://127.0.0.1:7655/api/monitoring/scheduler/health
```

Use the [private header-file preparation](#usage) and a `monitoring:read`
token. The non-zero HTTP-error exit matters; piping an unauthenticated error
straight to `jq` can make a failed request look successful.

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

Use the API endpoint above or export diagnostics from **Settings → Diagnostics** when troubleshooting.

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
Manually verify your deployment follows security best practices:
- No hardcoded credentials in environment files
- No credentials exposed in logs (check `docker logs pulse`)
- All passwords stored as bcrypt hashes (60 characters, starting with `$2a$` or `$2b$`)
- Server-side API token records stored as SHA3-256 hashes (64 characters);
  agents still need a private raw credential to authenticate
- Secure file permissions on `/etc/pulse/.env` (600)
- No credential leaks in API responses (test with `curl`)

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
not reset a password or bypass SSO. Use the [private header file](#usage) with a
`settings:write` token. Session-authenticated API requests also require CSRF
protection; do not copy a session cookie into a command.

```bash
# Reset lockout for a specific username
curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  --request POST --header "Content-Type: application/json" --data-binary @- \
  http://127.0.0.1:7655/api/security/reset-lockout <<'JSON'
{"identifier":"username"}
JSON

# Reset lockout for an IP address
curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  --request POST --header "Content-Type: application/json" --data-binary @- \
  http://127.0.0.1:7655/api/security/reset-lockout <<'JSON'
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
