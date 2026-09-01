#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 3 ]]; then
    echo "Usage: $0 <release-dir> <version> <output-dir>" >&2
    exit 64
fi

release_dir="$(cd "$1" && pwd -P)"
version="$2"
output_dir="$3"

if [[ -z "${output_dir}" || "${output_dir}" != /* ]]; then
    echo "Error: refusing unsafe release container context output: ${output_dir:-<empty>}" >&2
    exit 1
fi
output_dir="${output_dir%/}"
output_basename="$(basename "${output_dir}")"
output_parent="$(dirname "${output_dir}")"
if [[ -z "${output_dir}" || "${output_dir}" == "/" || "${output_basename}" == "." || "${output_basename}" == ".." ]]; then
    echo "Error: refusing unsafe release container context output: ${output_dir:-<empty>}" >&2
    exit 1
fi
mkdir -p "${output_parent}"
output_parent="$(cd "${output_parent}" && pwd -P)"
output_dir="${output_parent}/${output_basename}"
if [[ -L "${output_dir}" ]]; then
    echo "Error: refusing symlink release container context output: ${output_dir}" >&2
    exit 1
fi

validate_archive_entries() {
    local archive="$1"
    python3 - "${archive}" <<'PY'
import pathlib
import posixpath
import sys
import tarfile

archive = sys.argv[1]
seen: set[str] = set()
with tarfile.open(archive, "r:gz") as candidate:
    for member in candidate.getmembers():
        raw_name = member.name
        name = raw_name[2:] if raw_name.startswith("./") else raw_name
        if name in {"", "."} and member.isdir():
            continue
        parts = pathlib.PurePosixPath(name).parts
        unsafe_name = (
            not name
            or name.startswith("/")
            or "\\" in name
            or "//" in name
            or any(part in {"", ".", ".."} for part in parts)
        )
        unsafe_link = False
        if member.issym():
            target = member.linkname
            resolved = posixpath.normpath(posixpath.join(posixpath.dirname(name), target))
            unsafe_link = (
                not target
                or target.startswith("/")
                or "\\" in target
                or resolved == ".."
                or resolved.startswith("../")
            )
        if unsafe_name or unsafe_link or not (member.isfile() or member.isdir() or member.issym()):
            raise SystemExit(f"Error: release archive contains an unsafe member: {raw_name}")
        canonical = pathlib.PurePosixPath(*parts).as_posix()
        if canonical in seen:
            raise SystemExit(f"Error: release archive contains a duplicate member: {raw_name}")
        seen.add(canonical)
PY
}

rm -rf "${output_dir}"
mkdir -p "${output_dir}/amd64" "${output_dir}/arm64"

for arch in amd64 arm64; do
    archive="${release_dir}/pulse-v${version}-linux-${arch}.tar.gz"
    if [[ ! -f "${archive}" ]]; then
        echo "Error: immutable candidate is missing ${archive}." >&2
        exit 1
    fi
    validate_archive_entries "${archive}"
    tar --no-same-owner --no-same-permissions -xzf "${archive}" -C "${output_dir}/${arch}"
    for windows_arch in amd64 arm64 386; do
        for suffix in "" .sig .sshsig; do
            alias_path="${output_dir}/${arch}/bin/pulse-agent-windows-${windows_arch}${suffix}"
            expected_target="pulse-agent-windows-${windows_arch}.exe${suffix}"
            if [[ ! -L "${alias_path}" ]] || \
               [[ "$(readlink "${alias_path}")" != "${expected_target}" ]] || \
               [[ ! -f "${output_dir}/${arch}/bin/${expected_target}" ]]; then
                echo "Error: ${archive} has an invalid Windows agent alias ${alias_path}." >&2
                exit 1
            fi
            # runtime_prebuilt recreates these aliases after copying the
            # immutable .exe payload and sidecars. Keep the manifest-covered
            # context symlink-free rather than weakening its canonical-file rule.
            rm -f "${alias_path}"
        done
    done
    for required in \
        bin/pulse \
        bin/pulse.sig \
        bin/pulse.sshsig \
        bin/pulse-agent-linux-amd64 \
        bin/pulse-agent-linux-amd64.sig \
        bin/pulse-agent-linux-amd64.sshsig \
        scripts/install-container-agent.sh \
        scripts/install-docker.sh \
        scripts/install.sh \
        scripts/install.sh.sig \
        scripts/install.sh.sshsig \
        VERSION; do
        if [[ ! -f "${output_dir}/${arch}/${required}" ]]; then
            echo "Error: ${archive} is missing candidate container input ${required}." >&2
            exit 1
        fi
    done
    for helper_target in linux-amd64 linux-arm64 linux-armv7 linux-armv6 linux-386; do
        for suffix in "" .sig .sshsig; do
            required="bin/pulse-agent-helper-${helper_target}${suffix}"
            if [[ ! -f "${output_dir}/${arch}/${required}" ]]; then
                echo "Error: ${archive} is missing candidate container input ${required}." >&2
                exit 1
            fi
        done
    done
    for runner_target in linux-amd64 linux-arm64 linux-armv7 linux-armv6 linux-386; do
        for suffix in "" .sig .sshsig; do
            required="bin/pulse-agent-runner-${runner_target}${suffix}"
            if [[ ! -f "${output_dir}/${arch}/${required}" ]]; then
                echo "Error: ${archive} is missing candidate container input ${required}." >&2
                exit 1
            fi
        done
    done
done

if ! diff -qr \
    --exclude=pulse \
    --exclude=pulse.sig \
    --exclude=pulse.sshsig \
    "${output_dir}/amd64" "${output_dir}/arm64"; then
    echo "Error: candidate container inputs differ outside the target server binary and its signatures." >&2
    exit 1
fi

# The universal agent/script payload is identical in both server archives.
# Keep one copy plus the arm64 server and VERSION needed by the multi-arch
# runtime target so Actions transfers do not carry the same payload twice.
find "${output_dir}/arm64" -depth -mindepth 1 \
    ! -path "${output_dir}/arm64/bin" \
    ! -path "${output_dir}/arm64/bin/pulse" \
    ! -path "${output_dir}/arm64/VERSION" \
    -delete

echo "Prepared exact-candidate container context at ${output_dir}."
