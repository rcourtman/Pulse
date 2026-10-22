#!/usr/bin/env bash
# Complete, read-only inventories for the stable-publication continuity check.
# Only the observed HTTP/2 CANCEL signature permits another identical GET.
set -euo pipefail
umask 077

if [[ $# != 2 || ! "$1" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  echo 'Usage: read-github-release-inventory.sh <owner/repo> <evidence-directory>' >&2
  exit 2
fi
repository="$1"
evidence="$2"
mkdir -p -- "$evidence"
# A fresh read must never adopt a previous run's accepted inventory.
for name in stable-refs releases; do
  if [[ -e "$evidence/$name.json" || -L "$evidence/$name.json" ]]; then
    echo 'GitHub release inventory destination already exists; refusing stale evidence.' >&2
    exit 1
  fi
done

read_inventory() {
  local name="$1" endpoint="$2" attempt status output diagnostic
  for attempt in 1 2 3; do
    output="$evidence/$name-attempt-$attempt.json"
    diagnostic="$evidence/$name-attempt-$attempt.stderr"
    # Retain failed/truncated bytes separately. Neither another page nor a
    # later attempt can complete them into an apparently successful listing.
    if timeout -k 5 180 gh api --paginate --slurp \
        -H 'Accept: application/vnd.github+json' \
        -H 'X-GitHub-Api-Version: 2026-03-10' \
        "$endpoint" > "$output" 2> "$diagnostic"; then
      if [[ -s "$diagnostic" ]]; then
        echo "GitHub $name inventory returned an unexplained diagnostic; stopping." >&2
        return 1
      fi
      if ! python3 - "$output" <<'PY'
import json
from pathlib import Path
import sys

try:
    pages = json.loads(Path(sys.argv[1]).read_bytes())
    valid = (isinstance(pages, list) and bool(pages)
             and all(isinstance(page, list)
                     and all(isinstance(item, dict) for item in page)
                     for page in pages))
except (OSError, ValueError):
    valid = False
sys.exit(0 if valid else 1)
PY
      then
        echo "GitHub $name inventory is not a complete paginated JSON listing; stopping." >&2
        return 1
      fi
      mv -T -- "$output" "$evidence/$name.json"
      return 0
    else
      status=$?
    fi
    if [[ "$status" != 1 || "$attempt" == 3 ]] || ! LC_ALL=C awk '
        /^stream error: stream ID [0-9]+; CANCEL; received from peer$/ { seen = 1; next }
        { unknown = 1 }
        END { exit !(seen && !unknown) }
      ' "$diagnostic"; then
      echo "GitHub $name inventory read failed (exit $status); stopping. See retained attempt evidence." >&2
      return "$status"
    fi
    echo "GitHub $name inventory stream cancelled; retrying the same read in $((attempt * 2))s ($attempt/3)." >&2
    sleep "$((attempt * 2))"
  done
}

read_inventory stable-refs "repos/$repository/git/matching-refs/tags/v?per_page=100"
read_inventory releases "repos/$repository/releases?per_page=100"
