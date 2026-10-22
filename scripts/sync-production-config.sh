#!/usr/bin/env bash
# Explicit lab-copy helper, not production recovery or an isolated reproduction.
# Read installed files only. A backup filename does not identify a matching key.
set -euo pipefail

PROD_DIR="/etc/pulse"
DEV_DIR=${DEV_DIR:-"/opt/pulse/tmp/dev-config"}

fail() {
    printf '%s\n' "Configuration import refused: $*" >&2
    exit 1
}

# A missing installation remains a no-op, including the destination directory.
if [[ ! -d "$PROD_DIR" ]]; then
    printf '%s\n' 'No installed configuration directory; development files unchanged.'
    exit 0
fi

# Resolve aliases before creating directories or changing permissions. Neither
# an installed subtree nor its ancestor is a development import destination.
command -v realpath >/dev/null 2>&1 || fail 'GNU realpath is required; no files changed.'
IFS= read -r -d '' source_dir < <(realpath -e -z -- "$PROD_DIR" 2>/dev/null) ||
    fail 'cannot resolve installed directory; no files changed.'
IFS= read -r -d '' destination_dir < <(realpath -m -z -- "$DEV_DIR" 2>/dev/null) ||
    fail 'cannot resolve development directory; no files changed.'
if [[ "${destination_dir%/}/" == "${source_dir%/}/"* ||
      "${source_dir%/}/" == "${destination_dir%/}/"* ]]; then
    fail 'installed and development directories must be separate, non-overlapping trees; no files changed.'
fi

# Preserve the existing import scope. In particular, never copy installed .env,
# OIDC, sessions, CSRF state, history or alert data, and never clear dev versions.
names=(.encryption.key nodes.enc nodes.json system.json guest_metadata.json email.enc webhooks.enc api_tokens.json ai.enc)
selected=()
for name in "${names[@]}"; do
    for path in "$PROD_DIR/$name" "$DEV_DIR/$name"; do
        if [[ -e "$path" || -L "$path" ]]; then
            [[ -f "$path" && ! -L "$path" && -r "$path" ]] ||
                fail 'an import file is linked, unreadable or not regular; preserve it for manual recovery; no files changed.'
        fi
    done
    if [[ -f "$PROD_DIR/$name" ]]; then
        # Retain encrypted-node precedence when both formats are present.
        if [[ "$name" == nodes.json && -f "$PROD_DIR/nodes.enc" ]]; then continue; fi
        selected+=("$name")
    fi
done
for path in "$PROD_DIR/.encryption.key" "$DEV_DIR/.encryption.key"; do
    if [[ -e "$path" && ! -s "$path" ]]; then
        fail 'an encryption key is empty; identify the matching key independently; no files changed.'
    fi
done

# Include encrypted backups in orphan detection, not just the current imports.
# They are recovery evidence, not permission to generate or guess another key.
for directory in "$PROD_DIR" "$DEV_DIR"; do
    if [[ ! -f "$directory/.encryption.key" ]]; then
        for path in "$directory"/{nodes,email,webhooks,oidc,ai}.enc*; do
            if [[ -s "$path" || -L "$path" ]]; then
                fail 'encryption key is missing but encrypted data exists; preserve files and identify the matching key independently; no files changed.'
            fi
        done
    fi
done
if [[ -f "$PROD_DIR/nodes.json" && ! -f "$PROD_DIR/nodes.enc" && -f "$DEV_DIR/nodes.enc" ]]; then
    fail 'plaintext installed nodes conflict with retained encrypted development nodes; preserve both data sets and use a separate development copy; no files changed.'
fi
if [[ -f "$PROD_DIR/.encryption.key" && -f "$DEV_DIR/.encryption.key" ]] &&
    ! cmp -s "$PROD_DIR/.encryption.key" "$DEV_DIR/.encryption.key"; then
    fail 'development and installed keys differ; preserve both data sets and use a separate development copy; no files changed.'
fi

changed=()
for name in "${selected[@]}"; do
    if [[ ! -f "$DEV_DIR/$name" ]] || ! cmp -s "$PROD_DIR/$name" "$DEV_DIR/$name"; then
        changed+=("$name")
    fi
done
if (( ${#changed[@]} == 0 )); then
    printf '%s\n' 'Selected configuration already matches; development files unchanged.'
    exit 0
fi

# Stage all reads and previous destination files privately before replacing any
# selected file. Renames avoid following existing destination hard links. The
# previous snapshot also keeps overwritten dev settings recoverable.
umask 077
mkdir -p -- "$DEV_DIR"
staging=$(mktemp -d "$DEV_DIR/.config-import.XXXXXXXX")
publishing=false
cleanup() {
    local code=$? name
    trap - EXIT
    if [[ "$publishing" == true ]]; then
        printf '%s\n' 'Import incomplete; preserve .config-import.* in the development directory for manual recovery. No services were started.' >&2
    else
        # Only this invocation's fixed-name staging files are eligible, never
        # installed data, retained snapshots or arbitrary directory contents.
        for name in "${selected[@]}"; do
            for path in "$staging/new/$name" "$staging/previous/$name"; do
                if [[ -f "$path" ]]; then rm -- "$path" || code=1; fi
            done
        done
        rmdir -- "$staging/new" "$staging/previous" "$staging" || code=1
    fi
    exit "$code"
}
# Arm cleanup before the first copy; handled interruption before publication is
# not permission to leave staged credentials in an unknown temporary location.
mkdir -- "$staging/new" "$staging/previous"
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
for name in "${selected[@]}"; do
    cp -- "$PROD_DIR/$name" "$staging/new/$name"
    chmod 600 -- "$staging/new/$name"
done
for name in "${changed[@]}"; do
    if [[ -f "$DEV_DIR/$name" ]]; then
        cp -- "$DEV_DIR/$name" "$staging/previous/$name"
        chmod 600 -- "$staging/previous/$name"
    fi
done
# Reject an observed source change during staging. This is not a lock on live
# configuration, an authenticated key/ciphertext match or a consistent snapshot.
for name in "${selected[@]}"; do
    cmp -s "$PROD_DIR/$name" "$staging/new/$name" ||
        fail 'installed configuration changed or could not be rechecked during staging; existing development files unchanged.'
done
chmod 700 -- "$DEV_DIR"
publishing=true
for name in "${changed[@]}"; do
    mv -T -- "$staging/new/$name" "$DEV_DIR/$name"
done
# Unchanged staged reads are no longer needed. Preserve only the previous files
# in this private snapshot; never sweep older snapshots or runtime data.
for name in "${selected[@]}"; do
    if [[ -f "$staging/new/$name" ]]; then rm -- "$staging/new/$name"; fi
done
rmdir -- "$staging/new"
if ! rmdir -- "$staging/previous" 2>/dev/null; then
    printf '%s\n' 'Previous development files retained privately under .config-import.*/previous; preserve the matching key and snapshot together.'
else
    rmdir -- "$staging"
fi
trap - EXIT HUP INT TERM
printf '%s\n' 'Selected configuration imported. Installed and development runtime files were not changed; no services were started.'
