#!/bin/bash
# Compatibility entrypoint only. A resource name is not mock-data provenance.
# Never inspect or rewrite history, invoke privilege tools or stop monitoring.
set -euo pipefail

explain_retirement() {
    printf '%s\n' \
        'Automatic mock-alert cleanup has been retired.' \
        'A resource ID containing "mock" can belong to a real monitored resource.' \
        'This helper does not read or change alert history, create backups, or stop or restart monitoring.' \
        'Keep existing history intact for diagnosis. Use a separate disposable data directory for mock/test instances; never production data.' \
        'No cleanup was performed. There is no force or apply option.'
}

if [[ $# -eq 1 && ( $1 == --help || $1 == -h ) ]]; then
    explain_retirement
    exit 0
fi

# Old unattended callers must see a refusal, not an apparent successful cleanup.
# Do not echo arguments or legacy environment paths: they may contain private data.
explain_retirement >&2
exit 2
