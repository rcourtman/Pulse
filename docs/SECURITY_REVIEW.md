# Security Review Scope

Pulse does not currently claim an independent security certification or a
third-party penetration-test attestation. This page gives reviewers a concrete,
reproducible starting point and makes the intended trust boundaries explicit.

Before reviewing the code, read:

- [Production Deployment and Security](PRODUCTION_SECURITY.md) for the
  least-privilege deployment model and operational checklist.
- The canonical [Security Policy](../SECURITY.md) for reporting and hardening.
- [Agent Security](AGENT_SECURITY.md) for root privilege, command execution,
  guest access, and update verification.
- [Installation](INSTALL.md) for the signed, version-pinned server installer.

## First focused review packet

The [Authentication and Credential Storage Review Packet](security-review/AUTH_CREDENTIAL_REVIEW.md)
defines a narrow first assessment with explicit attacker capabilities, security
properties, source boundaries, manual review steps, and a reusable finding
template. Its baseline can be run with:

```bash
./scripts/security_review_auth_credentials.sh
```

## Suggested review boundaries

### 1. Authentication, authorization, and tenant isolation

Review browser sessions, API-token scopes, organization binding, proxy
authentication, and denial behavior. Confirm that a credential valid for one
route or organization cannot acquire management, agent, or cross-tenant
authority elsewhere.

Starting points:

- `internal/api/auth.go`
- `internal/api/authorization.go`
- `internal/api/middleware.go`
- `internal/api/api_token_scope_transport_integration_test.go`
- `internal/api/middleware_tenant_authorization_test.go`
- `pkg/auth/`
- `docs/security-review/AUTH_CREDENTIAL_REVIEW.md`

### 2. Credential storage and configuration transfer

Review encryption-key creation and permissions, AES-GCM use, encrypted
configuration persistence, backup/restore behavior, and export/import
authorization. Confirm that missing or unsafe key material fails closed rather
than silently replacing a key and orphaning encrypted data.

Starting points:

- `internal/crypto/crypto.go`
- `internal/crypto/crypto_test.go`
- `internal/config/persistence.go`
- `internal/config/persistence_fail_test.go`
- `internal/api/config_transfer_authorization.go`
- `internal/api/config_transfer_authorization_test.go`

### 3. Agent privilege and command authority

Review the Linux agent's default root profile as a privileged monitoring
boundary. Confirm that command execution remains disabled by default, enabling
it does not bypass policy and approval, report-only credentials cannot acquire
execution authority, and agent health endpoints remain loopback-bound unless
explicitly changed.

Starting points:

- `cmd/pulse-agent/main.go`
- `internal/agentexec/`
- `internal/api/agent_command_authorization.go`
- `internal/api/agent_command_authorization_test.go`
- `scripts/install.sh`
- `docs/AGENT_SECURITY.md`

### 4. Installer and update supply chain

Review the server installer's release pinning and Ed25519 verification, agent
checksum and signature enforcement, executable validation, self-test, and
atomic replacement behavior. Include the documented first automatic v5-to-v6
agent update limitation in the threat model.

Starting points:

- `install.sh`
- `docs/INSTALL.md`
- `internal/agentupdate/update.go`
- `internal/agentupdate/update_http_test.go`
- `.github/workflows/create-release.yml`
- `docs/CODE_SIGNING_POLICY.md`

### 5. Discovery and outbound network boundaries

Confirm that network discovery is disabled by default, probes only the
documented Proxmox ports, honors subnet allowlists and blocklists, and applies
host, concurrency, and timeout bounds. Review outbound URL validation and SSRF
protections for integrations and probes.

Starting points:

- `internal/config/config.go`
- `internal/discovery/`
- `pkg/discovery/`
- `internal/securityutil/outbound_http.go`
- `pkg/securityutil/outbound_http.go`

### 6. Container and host separation

Review the block on storing host SSH private keys inside Pulse containers,
Docker/Podman socket handling, Proxmox guest inventory opt-ins, and data-volume
permissions. Confirm that API-only Proxmox monitoring does not require Pulse
software on the hypervisor.

Starting points:

- `SECURITY.md`
- `docs/AGENT_SECURITY.md`
- `internal/hostagent/proxmox_lxc_filesystems.go`
- `internal/monitoring/docker_detection.go`
- `internal/api/contract_test.go`
- `internal/dockeragent/`

### 7. Production-scale failure behavior

Review behavior under concurrent reads and writes, stale data, partial platform
responses, retention, and large resource sets. Pulse includes simulated
500-node API and metrics-store regression coverage. Treat that as reproducible
engineering evidence, not as a production certification.

Starting points:

- `internal/api/load_test.go`
- `pkg/metrics/store_loadtest_test.go`
- `pkg/metrics/store_slo_test.go`

## Reproducible baseline

From a clean checkout with the documented Go and Node versions installed:

```bash
go test ./internal/crypto ./pkg/auth ./internal/agentupdate \
  ./internal/agentexec ./internal/discovery ./pkg/discovery \
  ./internal/securityutil ./pkg/securityutil

go test ./internal/api \
  -run '^TestLoad_500Node_(ConcurrentResources|ConcurrentMetricsHistory|MixedEndpoints)$' \
  -count=1

go test ./pkg/metrics \
  -run '^TestLoadTest500Nodes_WriteAndQueryIntegration$' \
  -count=1

python3 scripts/check_public_docs.py
```

These commands are a baseline, not a substitute for manual review, dependency
analysis, fuzzing, deployment-specific testing, or an independent threat model.

## Reporting findings

Please report suspected vulnerabilities privately to
<security@pulserelay.pro>. Include the affected version or commit, deployment
model, required privileges, reproduction steps, expected impact, and any
suggested containment. Do not include live credentials or customer data.

General defects and hardening suggestions that do not disclose an exploitable
weakness can use the public [issue tracker](https://github.com/rcourtman/Pulse/issues).
The canonical disclosure contact and current security guidance remain in the
[Security Policy](../SECURITY.md).
