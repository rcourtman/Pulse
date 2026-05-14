#!/usr/bin/env bash

# Shared helpers for the managed local Pulse development runtime.

hot_dev_pulse_process_count() {
    local pattern="${1:-^\./pulse$}"
    local count=0
    local pid

    if ! command -v pgrep >/dev/null 2>&1; then
        printf '0\n'
        return 0
    fi

    while IFS= read -r pid; do
        [[ -n "${pid}" ]] || continue
        count=$((count + 1))
    done < <(pgrep -f "${pattern}" 2>/dev/null || true)

    printf '%s\n' "${count}"
}
