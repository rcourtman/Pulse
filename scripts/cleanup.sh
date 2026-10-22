#!/usr/bin/env bash

set -euo pipefail

# Cleanup is for regenerable build output, not a reset of a local installation.
# Never scan arbitrary files by suffix: .bak, logs and empty directories can all
# be recovery evidence. Keep data, environment files and local settings intact.
ROOT="$(cd -P "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)"

BUILD_TARGETS=(
  "pulse"
  "frontend-modern/dist"
  "internal/api/frontend-modern"
)

EXTRA_TARGETS=(
  "bin"
  "build"
  "dist"
  "frontend-modern/node_modules"
  "frontend-modern/release"
  "frontend-modern/.vite"
  "frontend-modern/.cache"
  "frontend-modern/.eslintcache"
  "frontend"
  "frontend-modern/public/download"
  "frontend-modern/public/pulse-agent-windows-amd64.exe"
  "frontend-modern/public/pulse-agent-darwin-arm64.tar.gz"
  "frontend-modern/public/pulse-agent-darwin-amd64.tar.gz"
  "frontend-modern/public/pulse-agent-linux-amd64"
  "frontend-modern/public/pulse-agent-linux-arm64"
  "frontend-modern/public/pulse-agent-linux-armv7"
  "node_modules"
  "pulse-server"
  "pulse-test"
  "pulse-agent"
  "pulse-agent-linux-amd64"
  "pulse-agent-linux-arm64"
  "pulse-agent-linux-armv7"
  "pulse-agent-darwin-amd64"
  "pulse-agent-darwin-arm64"
  "pulse-agent-windows-amd64"
  "monitoring.test"
  "release"
  "scripts/macos/dist"
  "testing-tools/node_modules"
  ".parcel-cache"
  ".sass-cache"
  ".turbo"
  ".next"
  ".pytest_cache"
  ".gradle"
)

TARGETS=("${BUILD_TARGETS[@]}")
case "${1:-}" in
  "")
    [[ $# == 0 ]] || { echo "Usage: $0 [--build-only]" >&2; exit 2; }
    TARGETS+=("${EXTRA_TARGETS[@]}")
    ;;
  --build-only)
    [[ $# == 1 ]] || { echo "Usage: $0 [--build-only]" >&2; exit 2; }
    ;;
  *)
    echo "Usage: $0 [--build-only]" >&2
    exit 2
    ;;
esac

# An unavailable Git index is not evidence that a file is untracked. Verify the
# checkout identity and finish every index query before deleting the first file.
if ! git_root="$(git -C "${ROOT}" rev-parse --show-toplevel)" || [[ "${git_root}" != "${ROOT}" ]]; then
  echo "Error: cannot verify the cleanup checkout; no files removed." >&2
  exit 1
fi

has_linked_component() {
  local relative="$1" current="${ROOT}" component
  while :; do
    component="${relative%%/*}"
    current="${current}/${component}"
    [[ ! -L "${current}" ]] || return 0
    [[ "${relative}" == */* ]] || break
    relative="${relative#*/}"
  done
  return 1
}

removals=()
for target in "${TARGETS[@]}"; do
  # rm does not follow a final symlink, but it would follow a linked ancestor
  # such as frontend-modern -> another checkout before reaching dist.
  if has_linked_component "${target}"; then
    echo "Skipping linked ${target}"
    continue
  fi
  [[ -e "${ROOT}/${target}" ]] || continue
  if ! tracked="$(git -C "${ROOT}" ls-files -- "${target}")"; then
    echo "Error: cannot check tracked ${target}; no files removed." >&2
    exit 1
  fi
  if [[ -n "${tracked}" ]]; then
    echo "Skipping tracked ${target}"
    continue
  fi
  removals+=("${target}")
done

failed=0
for target in ${removals[@]+"${removals[@]}"}; do
  if rm -rf -- "${ROOT:?}/${target:?}"; then
    echo "Removed ${target}"
  else
    # Ownership failures must not turn a build cleanup into privileged removal.
    echo "Error: unable to remove ${target}; no privilege escalation attempted." >&2
    failed=1
  fi
done

if (( failed )); then
  echo "Cleanup incomplete; local data and settings were not targeted." >&2
  exit 1
fi

echo "Cleanup complete. Local data, settings, logs and recovery files outside build outputs were preserved."
