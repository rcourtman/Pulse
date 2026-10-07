#!/bin/sh
# Recover only the observed HTTP/2 proxy stream failure, never an access,
# checksum, manifest or unknown error. Keep the same Go proxy and sumdb.
set -eu

if [ "$#" -ne 0 ]; then
    echo 'Usage: go-mod-download.sh (no arguments)' >&2
    exit 2
fi

log=$(mktemp)
trap 'rm "$log"' 0
trap 'exit 130' INT
trap 'exit 143' TERM

attempt=1
while :; do
    # Both Alpine BusyBox and GNU timeout support this fixed invocation.
    # TERM at 180 seconds has a five-second KILL backstop. A timeout
    # is a terminal failure, not a reason to retry.
    if timeout -k 5 180 go mod download >"$log" 2>&1; then
        cat "$log"
        exit 0
    else
        status=$?
    fi
    cat "$log" >&2
    if [ "$status" -ne 1 ] || [ "$attempt" -eq 3 ]; then
        exit "$status"
    fi
    # Every diagnostic must be the specific transport failure. A refusal or
    # integrity error alongside it takes precedence and stops this attempt.
    if ! LC_ALL=C awk '
        /^go: [^[:space:]]+@[^[:space:]]+: read "https:\/\/[^"[:space:]]+": stream error: stream ID [0-9]+; INTERNAL_ERROR; received from peer$/ { seen = 1; next }
        { unknown = 1 }
        END { exit !(seen && !unknown) }
    ' "$log"; then
        exit "$status"
    fi
    echo "Go module proxy stream failed; retrying the same download ($attempt/3)." >&2
    sleep "$((attempt * 2))"
    attempt=$((attempt + 1))
done
