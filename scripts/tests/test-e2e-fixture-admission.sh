#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
node --test "${root}/tests/integration/scripts/fixture-readiness.test.mjs"
