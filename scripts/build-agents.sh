#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
source ./scripts/release_build_targets.sh
version="$(tr -d '\r\n' < VERSION)"
ldflags="$(./scripts/release_ldflags.sh agent --version "$version")"
mkdir -p bin
for target in "${PULSE_RELEASE_AGENT_TARGETS[@]}"; do
    read -r -a target_env <<< "$(pulse_release_target_env "$target")"
    env CGO_ENABLED=0 "${target_env[@]}" go build -trimpath -buildvcs=false \
        -ldflags="$ldflags" -o "bin/$(pulse_release_binary_filename agent "$target")" ./cmd/pulse-agent
done
