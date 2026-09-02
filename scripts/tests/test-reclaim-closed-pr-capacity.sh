#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
node --test "${root}/.github/scripts/reclaim-closed-pr-capacity.test.cjs"
