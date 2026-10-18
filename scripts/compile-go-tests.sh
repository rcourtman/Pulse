#!/usr/bin/env bash
# Compile the same Linux race-test binaries as CI, without running package
# initialisers, TestMain, tests or examples. This is not a test verdict.
set -euo pipefail

echo 'Compiling all Go race-test binaries; no test binary will execute.'
# -exec replaces execution of each successfully linked binary. In contrast,
# -run '^$' alone still runs init and TestMain. Vet and real test execution
# remain in the existing required backend shards. Disable result caching so
# the output cannot be mistaken for a previously executed test result.
go test -race -vet=off -count=1 -exec /bin/true ./...
echo 'All Go race-test binaries compiled; runtime tests remain required.'
