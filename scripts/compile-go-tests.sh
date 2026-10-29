#!/usr/bin/env bash
# Compile the same Linux race-test binaries as CI, without running package
# initialisers, TestMain, tests or examples. This is not a test verdict.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
selection="$(python3 "$ROOT_DIR/scripts/select-go-test-compile-packages.py" \
    --base "${PULSE_GO_COMPILE_BASE:-}" --head "${PULSE_GO_COMPILE_HEAD:-}")"
# An unavailable Git diff selects ./...; a broken selector must not turn the
# gate into a successful no-op. Paths are literal package arguments, not flags.
if [[ -z "$selection" ]]; then
    echo 'Go precompilation returned no package selection.' >&2
    exit 1
fi
mapfile -t packages <<< "$selection"
for package in "${packages[@]}"; do
    if [[ "$package" != . && "$package" != ./... && ! "$package" =~ ^\./[A-Za-z0-9][A-Za-z0-9_.-]*(/[A-Za-z0-9][A-Za-z0-9_.-]*)*$ ]]; then
        echo 'Go precompilation returned an invalid package selection.' >&2
        exit 1
    fi
done
# -exec replaces execution of each successfully linked binary. In contrast,
# -run '^$' alone still runs init and TestMain. Vet and real test execution
# remain in the existing required backend shards. Disable result caching so
# the output cannot be mistaken for a previously executed test result.
if [[ "$selection" == ./... ]]; then
    echo 'Compiling all Go race-test binaries; no test binary will execute.'
    go test -race -vet=off -count=1 -exec /bin/true ./...
else
    printf 'Compiling %s changed-test package(s) and their imports; no test binary will execute.\n' "${#packages[@]}"
    printf '  %s\n' "${packages[@]}"
    go test -race -vet=off -count=1 -exec /bin/true "${packages[@]}"
fi
echo 'Selected Go race-test binaries compiled; runtime tests remain required.'
