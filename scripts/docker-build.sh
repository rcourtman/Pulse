#!/usr/bin/env bash
set -euo pipefail

# Simple wrapper that enables BuildKit and forwards all arguments.
# To skip building multi-arch agents set BUILD_AGENT=0 before invoking.

export DOCKER_BUILDKIT=1

docker build "$@"
