#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname -- "$SCRIPT_DIR")"

cd "$REPO_ROOT"

printf 'Pulse authentication and credential review baseline\n'
printf 'Commit: %s\n' "$(git rev-parse HEAD)"
go version

printf '\nRunning cryptography, authentication, and configuration package tests\n'
go test ./internal/crypto ./pkg/auth ./internal/config -count=1

printf '\nRunning API authorization and configuration transfer tests\n'
./scripts/ensure_test_assets.sh
go test ./internal/api \
  -run '^(TestBearerAPITokenScopesDenyReadWriteAndExecRoutes|TestTenantMiddleware_.*|TestConfigTransfer.*|TestAllowUnprotectedExport.*|TestSecurityStatusMatchesConfigTransferPolicy|TestDeniedConfigTransferDoesNotMutateOrReload)$' \
  -count=1

printf '\nChecking public documentation links and mirrors\n'
python3 scripts/check_public_docs.py

printf '\nAuthentication and credential review baseline passed\n'
