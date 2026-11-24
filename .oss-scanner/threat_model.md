# Pulse threat model for scanner reports

## What Pulse does

Pulse is a self-hosted monitoring dashboard for Proxmox VE, PBS, Docker, Kubernetes, TrueNAS and vSphere. It holds API tokens and other credentials for the systems it monitors, stores their state and sends alerts. Optional agents run on monitored hosts and can act on them when an operator turns that on. An optional AI feature, Patrol, investigates problems and can propose or run fixes using the operator's own model. It is often run for a homelab or small team, and can be exposed through a reverse proxy.

The biggest risk is that a flaw in Pulse reaches the user's whole environment, so treat its credentials and its ability to act on hosts as the things most worth protecting.

## Where untrusted input enters

- **Unauthenticated HTTP.** The public route list is `publicPaths` in `internal/api/router.go`. It includes login, first-run setup and bootstrap token checks, recovery, installers and agent downloads, the agent WebSocket, OAuth and hosted-mode callbacks, and signup.
- **Authenticated users below instance administrator.** Sessions, proxy-authenticated users, OIDC and SAML users, organization members and owners, and scoped API tokens. Routes are in `internal/api/router_routes_*.go`, authorization in `internal/api/auth.go` and `internal/api/authorization.go`, and scopes in `pkg/auth/scopes.go`.
- **Agents and agent tokens.** A token leaked from one monitored host should not give control of Pulse or of other hosts. See `internal/api/agent_*.go`, `internal/agentexec`, `internal/actionrunner`, `cmd/pulse-agent`, `cmd/pulse-agent-helper` and `cmd/pulse-agent-runner`.
- **Data from monitored systems.** Resource names, labels, logs, API responses and command output can be influenced by whoever controls a guest or container. It reaches the server, the browser and AI prompts, so check output encoding and whether it can steer the AI.
- **Outbound requests.** Webhooks, notification targets, AI provider endpoints and SSO endpoints can be pointed at internal addresses. See `internal/notifications` and `internal/securityutil`.

## Components that matter most

- Authentication, sessions, first-run setup, recovery and API token handling.
- Authorization at every route, especially administrator-only actions and organization boundaries when `PULSE_MULTI_TENANT_ENABLED=true`.
- Anything that runs or approves actions: `internal/agentexec`, `internal/actionrunner`, `internal/ai/approval`, and read-only command classification in `internal/ai/safety/readonly.go`. AI tools must not exceed the Patrol autonomy mode the operator chose (`internal/config/ai.go`), even when monitored data tries to talk the model into it.
- Storage and exports for credentials (`internal/crypto` and the config export and import paths).
- Agent install and self-update: `scripts/install.sh`, `scripts/install.ps1`, `internal/agentupdate`, `internal/updatesignature`.
- `cmd/pulse-mcp`.
- Hosted mode (`cmd/pulse-control-plane`, `internal/cloudcp`), where tenant separation matters.

## How to exercise it

The image keeps `/src`, the release binaries in `/src/bin`, Go modules and frontend dependencies.

- Run the server with `PULSE_DATA_DIR=/tmp/pulse FRONTEND_PORT=7655 /src/bin/pulse`. First-run setup then starts on port 7655.
- Run focused Go tests with `go test ./internal/api/ -run <name>`. The whole of `internal/api` is slow, so narrow it.
- Run frontend tests with `cd frontend-modern && npx vitest run <file>`.

## Severity ratings

- **Critical:** without authentication, or from a single agent token, gaining instance administrator rights, reading credentials for monitored systems, or running code on the Pulse server or on monitored hosts. Also forging an update that agents would accept.
- **High:** a user below instance administrator performing administrator actions or obtaining administrator credentials. Writing across organization boundaries. Getting past command classification, approvals or the chosen autonomy mode. Unauthenticated disclosure of sensitive configuration, or an unauthenticated way to restart or stop the service.
- **Medium:** a low-privilege user reading a limited secret or forging reports. Reading across organization boundaries where the data is limited. Outbound address checks that can be bypassed but need administrator-level configuration. Tokens leaking through redirects.
- **Low:** hardening with no demonstrated path to harm, and denial of service that needs an authenticated administrator.

Please state the lowest role needed, the settings and deployment type involved, and a reproducer against a build using the `release` tag.

## Anything to leave alone

- Behaviour that needs a build without the `release` tag, such as mock fixtures or the admin bypass, which shipped images fail closed on.
- Behaviour that only appears when an operator sets a development option such as `PULSE_DEV` or `PULSE_DEV_ALLOW_CONTAINER_SSH`.
- An instance administrator, or anyone who can already read the data directory. `SECURITY.md` documents that the encryption key sits beside the encrypted files, and that this encryption doesn't protect against an account that can read both.
- Passing a paid-feature check on an instance you administer. Licence bypass is only in scope if it crosses a tenant boundary in hosted mode.
- Pulse Mobile and the mobile relay path (`internal/relay`), which are being sunsetted. Only report an `internal/relay` issue if it reaches the server unauthenticated or crosses tenant boundaries when relay is not configured.
- `dev/`, `testing-tools/`, `tests/`, `testdata/`, `cmd/eval`, `cmd/patrol-qualify`, `cmd/agent-probe`, and scripts other than the installers.
