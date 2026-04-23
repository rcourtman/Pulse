#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SMOKE="${ROOT_DIR}/scripts/run_cloud_public_signup_smoke.sh"

bash -n "${SMOKE}"

grep -q 'EXPECT_PUBLIC_SIGNUP_ENABLED' "${SMOKE}"
grep -q 'PUBLIC_SIGNUP_CLOSED_REDIRECT_URL' "${SMOKE}"
grep -q 'CHECK_MAGIC_LINK_VALID_PROBE' "${SMOKE}"
grep -q 'check_closed_signup_contract' "${SMOKE}"
grep -q 'check_open_signup_contract' "${SMOKE}"

echo "cloud public signup smoke checks passed"
