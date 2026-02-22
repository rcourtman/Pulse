# Repo Boundary (Pulse v6)

This document defines where code should live as Pulse v6 is finalized.

## Ownership map

1. `pulse` (public): community/core product runtime and OSS-safe docs.
2. `pulse-enterprise` (private): paid in-app implementations (enterprise modules behind interfaces).
3. `pulse-pro` (private): backend/business infrastructure (`license-server`, `relay-server`, billing/support ops).

## Critical migration constraint

`pulse-enterprise` cannot import `pulse/internal/...` packages because of Go `internal` package visibility rules.

To move paid in-app logic out of this public repo, shared contracts must be exposed through `pkg/...` interfaces first, then implemented privately.

Current promoted contract surface:

- `pkg/licensing` (feature/tier constants, entitlement state enums, upgrade URL resolver, upgrade reason matrix, feature-gate interfaces, shared request/response types)
- `pkg/licensing` evaluator + entitlement source contracts (for capability/limit checks outside `internal/...`)
- `pkg/licensing` core license model types (`Claims`, `License`, `LicenseState`, `LicenseStatus`) for cross-repo contract stability

## Current boundary audit

Run:

```bash
./scripts/audit-private-boundary.sh
```

Enforce full boundary (non-zero exit on production paid-domain files):

```bash
./scripts/audit-private-boundary.sh --enforce
```

Enforce API import boundary only (non-zero exit if any non-test `internal/api/*.go` imports `internal/license/*`):

```bash
./scripts/audit-private-boundary.sh --enforce-api-imports
```

Enforce API root import boundary (non-zero exit if any non-test `internal/api/*.go` imports `internal/license`):

```bash
./scripts/audit-private-boundary.sh --enforce-api-root-imports
```

Enforce non-API runtime import boundary (non-zero exit if non-test runtime files import `internal/license`):

```bash
./scripts/audit-private-boundary.sh --enforce-nonapi-imports
```

The script currently reports paid-domain files in:

- `internal/license/...`
- paid-focused handlers in `internal/api/...` (`license`, `entitlement`, `billing`, `stripe`, `hosted`, `rbac`, `audit`, `reporting`, `sso`, `conversion`)

This is expected until extraction phases complete.

Current milestone:

- Non-test `internal/api/*.go` imports of `internal/license/*`: **0**
- API root imports of `internal/license`: **0**
- Non-API runtime imports of `internal/license`: **0**

## Safety requirements during extraction

1. Keep API contracts stable for current v5/v6 users.
2. Do not break `license.pulserelay.pro` behavior.
3. Keep JWT claim schema compatibility (`lid`, `email`, `tier`, `iat`, `exp` when applicable).
4. Move code in phases with tests green at every step.
