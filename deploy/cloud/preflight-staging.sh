#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"

export DOMAIN="${DOMAIN:-cloud-staging.pulserelay.pro}"
export SSH_TARGET="${SSH_TARGET:-root@cloud-staging-host}"
export EXPECT_CP_ENV="${EXPECT_CP_ENV:-staging}"
export EXPECT_STRIPE_MODE="${EXPECT_STRIPE_MODE:-test}"
export CHECK_STRIPE_CHECKOUT_PROBE="${CHECK_STRIPE_CHECKOUT_PROBE:-true}"

exec "${SCRIPT_DIR}/preflight-live.sh" "$@"
