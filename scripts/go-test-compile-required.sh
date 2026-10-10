#!/usr/bin/env bash
# Read the entire changed-file list. Source/manifests, embedded backend files
# and this admission path can affect the root Go module; frontend-only changes
# cannot invalidate its test API. An unavailable diff is handled as true by CI.
set -euo pipefail

if grep -E '^go\.(mod|sum)$|^(internal|cmd|pkg)/|^scripts/(compile-go-tests|go-test-compile-required|ensure_test_assets)\.sh$|^\.github/workflows/build-and-test\.yml$|\.(go|c|cc|cpp|h|s|S|syso)$' >/dev/null; then
    printf 'true\n'
else
    printf 'false\n'
fi
