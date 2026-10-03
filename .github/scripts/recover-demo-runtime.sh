#!/usr/bin/env bash
set -euo pipefail
# CI observer only; remote systemd owns mutation, restoration and both watches.
exec python3 .github/scripts/dispatch-demo-runtime.py recover
