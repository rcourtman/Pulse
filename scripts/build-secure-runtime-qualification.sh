#!/usr/bin/env bash

# Build the exact Linux/amd64 secure-runtime qualification subjects on a
# GitHub-hosted runner. Candidate assembly publishes the three predecessor
# collectors and requires the current collector/helper/runner subjects to be
# byte-identical to the ordinary release payload before signing the packet.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

VERSION="${1:-}"
OUTPUT_DIR="${2:-}"
TARGET_ARCH="${3:-amd64}"

if [[ -z "${VERSION}" || -z "${OUTPUT_DIR}" || -n "${4:-}" ]]; then
    echo "Usage: $0 <release-version> <output-directory> [amd64|arm64]" >&2
    exit 2
fi
case "${TARGET_ARCH}" in
    amd64|arm64) ;;
    *)
        echo "Error: unsupported secure-runtime qualification architecture: ${TARGET_ARCH}" >&2
        exit 2
        ;;
esac
if [[ "$(tr -d '\n\r[:space:]' < VERSION)" != "${VERSION}" ]]; then
    echo "Error: requested version ${VERSION} does not match VERSION." >&2
    exit 3
fi
if [[ -z "${PULSE_UPDATE_SIGNING_PUBLIC_KEY:-}" ]]; then
    echo "Error: PULSE_UPDATE_SIGNING_PUBLIC_KEY is required." >&2
    exit 3
fi

required_go="go1.26.8"
go_version="$(go env GOVERSION 2>/dev/null || true)"
if [[ "${go_version}" != "${required_go}" ]]; then
    echo "Error: Go toolchain must be ${required_go} (got ${go_version:-unknown})." >&2
    exit 3
fi

OUTPUT_DIR="$(python3 -c 'import os, sys; print(os.path.abspath(sys.argv[1]))' "${OUTPUT_DIR}")"
case "${OUTPUT_DIR}" in
    /|"${REPO_ROOT}"|"${REPO_ROOT}/scripts")
        echo "Error: refusing unsafe secure-runtime qualification output: ${OUTPUT_DIR}" >&2
        exit 2
        ;;
esac
if [[ -e "${OUTPUT_DIR}" ]] && find "${OUTPUT_DIR}" -mindepth 1 -print -quit | grep -q .; then
    echo "Error: secure-runtime qualification output must be absent or empty: ${OUTPUT_DIR}" >&2
    exit 2
fi
mkdir -p "${OUTPUT_DIR}"

source_sha="$(git rev-parse HEAD)"
if [[ ! "${source_sha}" =~ ^[0-9a-f]{40}$ ]]; then
    echo "Error: secure-runtime qualification requires an exact Git commit." >&2
    exit 3
fi

update_public_key="$(printf '%s' "${PULSE_UPDATE_SIGNING_PUBLIC_KEY}" | tr -d '\r\n[:space:]')"
update_key_fingerprint="$(go run ./scripts/release_update_key.go fingerprint --public-key "${update_public_key}")"
release_tag="v${VERSION}"
predecessor_base="${VERSION%%-*}"
collector_v1_version="${predecessor_base}-0.secure.v6.1"
collector_v2_version="${predecessor_base}-0.secure.v6.2"
collector_v3_version="${predecessor_base}-0.secure.v6.3"

collector_v1_ldflags="$(./scripts/release_ldflags.sh agent --version "${collector_v1_version}" --update-public-keys "${update_public_key}")"
collector_v2_ldflags="$(./scripts/release_ldflags.sh agent --version "${collector_v2_version}" --update-public-keys "${update_public_key}")"
collector_v3_ldflags="$(./scripts/release_ldflags.sh agent --version "${collector_v3_version}" --update-public-keys "${update_public_key}")"
release_agent_ldflags="$(./scripts/release_ldflags.sh agent --version "${VERSION}" --update-public-keys "${update_public_key}")"

artifact_asset() {
    case "$1" in
        collector_v1) printf 'pulse-secure-runtime-collector-v1-linux-%s\n' "${TARGET_ARCH}" ;;
        collector_v2) printf 'pulse-secure-runtime-collector-v2-linux-%s\n' "${TARGET_ARCH}" ;;
        collector_v3) printf 'pulse-secure-runtime-collector-v3-linux-%s\n' "${TARGET_ARCH}" ;;
        collector_v4) printf 'pulse-agent-linux-%s\n' "${TARGET_ARCH}" ;;
        helper) printf 'pulse-agent-helper-linux-%s\n' "${TARGET_ARCH}" ;;
        runner) printf 'pulse-agent-runner-linux-%s\n' "${TARGET_ARCH}" ;;
    esac
}

artifact_package() {
    case "$1" in
        collector_v1|collector_v2|collector_v3|collector_v4)
            printf '%s\n' 'github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent'
            ;;
        helper) printf '%s\n' 'github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent-helper' ;;
        runner) printf '%s\n' 'github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent-runner' ;;
    esac
}

artifact_version() {
    case "$1" in
        collector_v1) printf '%s\n' "${collector_v1_version}" ;;
        collector_v2) printf '%s\n' "${collector_v2_version}" ;;
        collector_v3) printf '%s\n' "${collector_v3_version}" ;;
        collector_v4|helper|runner) printf '%s\n' "${VERSION}" ;;
    esac
}

artifact_ldflags() {
    case "$1" in
        collector_v1) printf '%s\n' "${collector_v1_ldflags}" ;;
        collector_v2) printf '%s\n' "${collector_v2_ldflags}" ;;
        collector_v3) printf '%s\n' "${collector_v3_ldflags}" ;;
        collector_v4|helper) printf '%s\n' "${release_agent_ldflags}" ;;
        runner) printf '\n' ;;
    esac
}

build_subject() {
    local name="$1"
    local package="./cmd/pulse-agent"
    local output="${OUTPUT_DIR}/$(artifact_asset "${name}")"
    local -a command=(go build -buildvcs=false -trimpath)
    case "${name}" in
        helper) package="./cmd/pulse-agent-helper" ;;
        runner) package="./cmd/pulse-agent-runner" ;;
    esac
    local ldflags="$(artifact_ldflags "${name}")"
    if [[ -n "${ldflags}" ]]; then
        command+=("-ldflags=${ldflags}")
    fi
    command+=(-o "${output}" "${package}")
    env CGO_ENABLED=0 GOOS=linux GOARCH="${TARGET_ARCH}" "${command[@]}"
}

for name in collector_v1 collector_v2 collector_v3 collector_v4 helper runner; do
    build_subject "${name}"
done

export SECURE_RUNTIME_OUTPUT_DIR="${OUTPUT_DIR}"
export SECURE_RUNTIME_VERSION="${VERSION}"
export SECURE_RUNTIME_TAG="${release_tag}"
export SECURE_RUNTIME_SOURCE_SHA="${source_sha}"
export SECURE_RUNTIME_TARGET_ARCH="${TARGET_ARCH}"
export SECURE_RUNTIME_GO_VERSION="${go_version}"
export SECURE_RUNTIME_UPDATE_PUBLIC_KEY="${update_public_key}"
export SECURE_RUNTIME_UPDATE_KEY_FINGERPRINT="${update_key_fingerprint}"
for name in collector_v1 collector_v2 collector_v3 collector_v4 helper runner; do
    upper_name="$(printf '%s' "${name}" | tr '[:lower:]' '[:upper:]')"
    export "SECURE_RUNTIME_${upper_name}_ASSET=$(artifact_asset "${name}")"
    export "SECURE_RUNTIME_${upper_name}_PACKAGE=$(artifact_package "${name}")"
    export "SECURE_RUNTIME_${upper_name}_VERSION=$(artifact_version "${name}")"
    export "SECURE_RUNTIME_${upper_name}_LDFLAGS=$(artifact_ldflags "${name}")"
done

python3 - <<'PY'
import hashlib
import json
import os
from pathlib import Path

root = Path(os.environ["SECURE_RUNTIME_OUTPUT_DIR"])
names = ("collector_v1", "collector_v2", "collector_v3", "collector_v4", "helper", "runner")
artifacts = {}
subject_lines = []
for name in names:
    prefix = f"SECURE_RUNTIME_{name.upper()}_"
    asset = os.environ[prefix + "ASSET"]
    path = root / asset
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    ldflags = os.environ[prefix + "LDFLAGS"]
    artifacts[name] = {
        "release_asset": asset,
        "sha256": digest,
        "build": {
            "tool": "go build",
            "package": os.environ[prefix + "PACKAGE"],
            "target_os": "linux",
            "target_arch": os.environ["SECURE_RUNTIME_TARGET_ARCH"],
            "cgo_enabled": 0,
            "go_version": os.environ["SECURE_RUNTIME_GO_VERSION"],
            "trimpath": True,
            "buildvcs": False,
            "build_args": ["-buildvcs=false", "-trimpath"],
            "ldflags": ldflags,
            "ldflags_sha256": hashlib.sha256(ldflags.encode()).hexdigest(),
            "version": os.environ[prefix + "VERSION"],
            "update_key_fingerprint": os.environ["SECURE_RUNTIME_UPDATE_KEY_FINGERPRINT"],
        },
    }
    subject_lines.append(f"{digest}  {asset}")

contract = {
    "schema_version": 1,
    "repository": "rcourtman/Pulse",
    "assembly_signer_workflow": "github.com/rcourtman/Pulse/.github/workflows/build-release-candidate.yml",
    "compiler_signer_workflow": "github.com/rcourtman/Pulse/.github/workflows/compile-release-payload.yml",
    "compiler_runner_trust": "github-hosted-deny-self-hosted",
    "tag": os.environ["SECURE_RUNTIME_TAG"],
    "version": os.environ["SECURE_RUNTIME_VERSION"],
    "source_sha": os.environ["SECURE_RUNTIME_SOURCE_SHA"],
    "update_public_keys": os.environ["SECURE_RUNTIME_UPDATE_PUBLIC_KEY"],
    "update_key_fingerprint": os.environ["SECURE_RUNTIME_UPDATE_KEY_FINGERPRINT"],
    "artifacts": artifacts,
}
(root / "secure-runtime-build-contract-v1.json").write_text(
    json.dumps(contract, indent=2, sort_keys=True) + "\n", encoding="utf-8"
)
contract_digest = hashlib.sha256(
    (root / "secure-runtime-build-contract-v1.json").read_bytes()
).hexdigest()
subject_lines.append(f"{contract_digest}  secure-runtime-build-contract-v1.json")
(root / "secure-runtime-compiler-subjects.sha256").write_text(
    "\n".join(subject_lines) + "\n", encoding="utf-8"
)
PY

echo "Built hosted secure-runtime qualification subjects in ${OUTPUT_DIR}."
