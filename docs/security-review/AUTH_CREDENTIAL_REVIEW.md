# Authentication and Credential Storage Review Packet

This packet defines the first focused external review boundary for Pulse. It is
an invitation to inspect and challenge the implementation. It is not a claim
that an independent review has already happened.

## Review objective

Assess whether Pulse keeps authentication, authorization, tenant isolation,
credential storage, and configuration transfer within their documented trust
boundaries. Findings should identify a concrete path from attacker capability
to security impact.

Test against a named commit so that every observation can be reproduced.

## In scope

- Browser session authentication and logout behavior
- Password hashing and password policy enforcement
- API token generation, hashing, comparison, scope enforcement, and revocation
- Organization binding and cross-tenant denial behavior
- Proxy authentication and forwarded request boundaries
- Authorization for configuration export and import
- Encryption key creation, permissions, migration, and validation
- AES-GCM encryption and decryption behavior
- Failure behavior when encrypted data exists without usable key material
- Secret redaction and API responses that represent credential presence

## Out of scope

- Security of an external identity provider itself
- Security of the operating system, reverse proxy, or container runtime
- Agent command authority, update delivery, network discovery, and SSRF
- Availability testing against a production Pulse installation
- Customer data or credentials from any real deployment

Those boundaries remain valid review targets, but they should be handled as
separate packets so that evidence and conclusions stay precise.

## Attacker capabilities to consider

1. An unauthenticated network client can reach the Pulse HTTP service.
2. An authenticated user has the least privileged available role.
3. A caller holds a valid API token with one narrow scope.
4. A tenant user knows or guesses another organization identifier.
5. A reverse proxy supplies forwarded identity and network headers.
6. A local user can read a copied data directory but not the original key.
7. An operator restores encrypted configuration without the matching key.
8. A caller sends a malformed, oversized, or partial import request.

Root access to the running Pulse host is not treated as a boundary that
application-level encryption can defeat. The review should still report any
unnecessary secret exposure that would increase the impact of host compromise.

## Security properties to verify

- Protected routes deny access when identity is missing or invalid.
- A credential cannot gain authority outside its assigned scopes.
- Organization selection cannot bypass tenant authorization.
- Management operations require management authority.
- Export and import authorization happens before request bodies are consumed
  or state can change.
- Failed import or export requests do not mutate configuration or trigger a
  reload.
- Stored API tokens and passwords are one-way hashed where the runtime only
  needs comparison.
- Reversible secrets use authenticated encryption with unique nonces.
- Encryption key files reject unsafe permissions, symlinks, invalid material,
  and unexpected replacement.
- Existing encrypted data without its key fails closed instead of creating a
  replacement key.
- API responses expose credential presence or identifiers without returning
  stored secret values.

## Source map

Authentication and authorization:

- `internal/api/auth.go`
- `internal/api/authorization.go`
- `internal/api/router.go`
- `internal/api/router_routes_auth_security.go`
- `internal/api/security.go`
- `internal/api/security_tokens.go`
- `internal/api/session_store.go`
- `internal/api/middleware.go`
- `internal/api/middleware_tenant.go`
- `internal/config/api_tokens.go`
- `internal/api/oidc_handlers.go`
- `internal/api/oidc_service.go`
- `internal/api/saml_handlers.go`
- `internal/api/saml_service.go`
- `internal/config/sso.go`
- `internal/api/api_token_scope_transport_integration_test.go`
- `internal/api/oidc_legacy_callback_recovery_test.go`
- `internal/api/router_csrf_middleware_test.go`
- `internal/api/security_tokens_lifecycle_test.go`
- `internal/api/session_store_test.go`
- `internal/api/saml_service_test.go`
- `internal/api/middleware_tenant_authorization_test.go`
- `pkg/auth/`

Credential storage and configuration transfer:

- `internal/crypto/crypto.go`
- `internal/crypto/crypto_test.go`
- `internal/config/persistence.go`
- `internal/config/persistence_fail_test.go`
- `internal/api/config_transfer_authorization.go`
- `internal/api/config_transfer_authorization_test.go`

## Reproducible baseline

From a clean checkout of the commit under review, run:

```bash
./scripts/security_review_auth_credentials.sh
```

The script prints the tested commit, records the Go toolchain version in its
output, runs the focused package and authorization tests, verifies that every
named API security regression still exists before running it, and validates
the public documentation mirrors. The explicit inventory prevents a renamed
or deleted test from becoming a silent successful no-op. A passing result is
regression evidence only. It does not prove that the implementation is free
of vulnerabilities.

## Manual review procedure

1. Trace each authentication method from request parsing to the final user and
   token context.
2. Build a route matrix for anonymous sessions, user sessions, proxy identity,
   unrestricted tokens, and narrowly scoped tokens.
3. Trace organization selection before and after authorization decisions.
4. Trace export and import requests from authorization through body reads,
   persistence, reload, and error handling.
5. Trace every stored secret from input through hashing or encryption to API
   serialization and logging.
6. Exercise malformed credentials, stale sessions, revoked tokens, conflicting
   organization identifiers, forwarded loopback claims, and missing key files.
7. Record any assumption that depends on deployment configuration rather than
   application enforcement.

Use only disposable local data. Do not test against a community member or
customer installation without the operator's explicit permission.

## Requested output

Use the [finding template](FINDING_TEMPLATE.md) for each confirmed issue or
material hardening gap. A useful report contains:

- The exact tested commit and deployment mode
- Required attacker access and privileges
- Minimal reproduction steps
- Expected and observed behavior
- Security impact and affected boundary
- Relevant source locations and test evidence
- A practical containment or remediation suggestion when known

Send suspected vulnerabilities privately to <security@pulserelay.pro>. General
hardening suggestions without an exploitable path can use the public issue
tracker after confirming that publication does not expose a weakness.
