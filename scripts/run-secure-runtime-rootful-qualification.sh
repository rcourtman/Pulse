#!/usr/bin/env bash
set -euo pipefail

readonly REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly UBUNTU_IMAGE="${PULSE_ROOTFUL_UBUNTU_IMAGE:?set PULSE_ROOTFUL_UBUNTU_IMAGE to an immutable ubuntu@sha256:... Ubuntu 24.04 image}"
readonly OUTPUT_PARENT="${PULSE_ROOTFUL_QUALIFICATION_OUTPUT_DIR:?set PULSE_ROOTFUL_QUALIFICATION_OUTPUT_DIR to an existing absolute private directory}"
readonly CONFIRM="${PULSE_ROOTFUL_QUALIFICATION_CONFIRM:-}"

portable_mode() {
  stat -c '%a' "$1" 2>/dev/null || stat -f '%Lp' "$1"
}

portable_uid() {
  stat -c '%u' "$1" 2>/dev/null || stat -f '%u' "$1"
}

if [[ ! "${UBUNTU_IMAGE}" =~ ^ubuntu@sha256:[0-9a-f]{64}$ ]]; then
  echo "ERROR: PULSE_ROOTFUL_UBUNTU_IMAGE must be an exact ubuntu@sha256 digest" >&2
  exit 2
fi
if [[ "${OUTPUT_PARENT}" != /* || ! -d "${OUTPUT_PARENT}" || -L "${OUTPUT_PARENT}" ]]; then
  echo "ERROR: PULSE_ROOTFUL_QUALIFICATION_OUTPUT_DIR must be an existing absolute non-symlink directory" >&2
  exit 2
fi
if [[ "$(portable_mode "${OUTPUT_PARENT}")" != "700" ]]; then
  echo "ERROR: qualification output directory must have exact mode 0700" >&2
  exit 2
fi
if [[ "$(portable_uid "${OUTPUT_PARENT}")" != "$(id -u)" ]]; then
  echo "ERROR: qualification output directory must be owned by the invoking user" >&2
  exit 2
fi
if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
  echo "ERROR: a working Docker CLI/daemon is required to create disposable qualification containers" >&2
  exit 2
fi
if [[ "$(git -C "${REPO_ROOT}" branch --show-current)" != "main" ]]; then
  echo "ERROR: qualification builds are allowed only from main" >&2
  exit 2
fi
if [[ -n "$(git -C "${REPO_ROOT}" status --porcelain)" ]]; then
  echo "ERROR: qualification requires a clean exact source checkout" >&2
  exit 2
fi

readonly SOURCE_COMMIT="$(git -C "${REPO_ROOT}" rev-parse HEAD)"
readonly EXPECTED_CONFIRM="I_HAVE_VERIFIED_THESE_ARE_DISPOSABLE_ROOTFUL_SYSTEMD_CONTAINERS_COMMIT_${SOURCE_COMMIT}"
if [[ "${CONFIRM}" != "${EXPECTED_CONFIRM}" ]]; then
  echo "ERROR: exact destructive opt-in required:" >&2
  echo "export PULSE_ROOTFUL_QUALIFICATION_CONFIRM=${EXPECTED_CONFIRM}" >&2
  exit 2
fi

readonly RUN_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
readonly OUTPUT_DIR="${OUTPUT_PARENT}/${RUN_STAMP}-${SOURCE_COMMIT:0:12}"
readonly IMAGE_TAG="pulse-rootful-qualification:${SOURCE_COMMIT:0:12}"
readonly CONTAINER_RUN_LABEL="org.pulse.rootful-qualification.run"
readonly CONTAINER_RUN_NONCE="$(openssl rand -hex 16)"
if docker image inspect "${IMAGE_TAG}" >/dev/null 2>&1; then
  echo "ERROR: qualification image tag already exists; refusing to overwrite ${IMAGE_TAG}" >&2
  exit 2
fi
PACKET_DIR="$(mktemp -d /tmp/pulse-rootful-packet.XXXXXX)"
CONTAINER_IDS=()
IMAGE_CREATED=false

inspect_container_nonce() {
  docker inspect --format '{{ index .Config.Labels "org.pulse.rootful-qualification.run" }}' "$1"
}

remove_container_strict() {
  local container_id="$1"
  local observed_nonce remaining
  observed_nonce="$(inspect_container_nonce "${container_id}")" || {
    echo "ERROR: unable to verify ownership label for qualification container ${container_id}" >&2
    return 1
  }
  if [[ "${observed_nonce}" != "${CONTAINER_RUN_NONCE}" ]]; then
    echo "ERROR: qualification container ${container_id} ownership label changed" >&2
    return 1
  fi
  docker rm -f "${container_id}" >/dev/null || return 1
  remaining="$(docker ps -aq --no-trunc --filter "id=${container_id}")" || return 1
  if printf '%s\n' "${remaining}" | grep -Fxq "${container_id}"; then
    echo "ERROR: qualification container ${container_id} remains after removal" >&2
    return 1
  fi
}

forget_container() {
  local removed_id="$1" candidate
  local retained=()
  for candidate in "${CONTAINER_IDS[@]}"; do
    [[ "${candidate}" == "${removed_id}" ]] || retained+=("${candidate}")
  done
  CONTAINER_IDS=("${retained[@]}")
}

remove_container_best_effort() {
  local container_id="$1" observed_nonce
  observed_nonce="$(inspect_container_nonce "${container_id}" 2>/dev/null)" || {
    echo "WARNING: unable to inspect qualification container ${container_id}; manual cleanup may be required" >&2
    return
  }
  if [[ "${observed_nonce}" != "${CONTAINER_RUN_NONCE}" ]]; then
    echo "WARNING: refusing to remove qualification container after ownership-label mismatch: ${container_id}" >&2
    return
  fi
  docker rm -f "${container_id}" >/dev/null 2>&1 || \
    echo "WARNING: failed to remove qualification container ${container_id}" >&2
}

remove_image_strict() {
  local observed_nonce
  [[ "${IMAGE_CREATED}" == true ]] || return 0
  observed_nonce="$(docker image inspect --format '{{ index .Config.Labels "org.pulse.rootful-qualification.run" }}' "${IMAGE_TAG}")" || return 1
  if [[ "${observed_nonce}" != "${CONTAINER_RUN_NONCE}" ]]; then
    echo "ERROR: qualification image ownership label changed" >&2
    return 1
  fi
  docker image rm "${IMAGE_TAG}" >/dev/null || return 1
  IMAGE_CREATED=false
}

cleanup() {
  local container_id discovered discovered_ids=""
  discovered_ids="$(docker ps -aq --no-trunc --filter "label=${CONTAINER_RUN_LABEL}=${CONTAINER_RUN_NONCE}" 2>/dev/null)" || true
  for container_id in "${CONTAINER_IDS[@]}"; do
    remove_container_best_effort "${container_id}"
  done
  while IFS= read -r discovered; do
    [[ -n "${discovered}" ]] || continue
    if [[ " ${CONTAINER_IDS[*]} " != *" ${discovered} "* ]]; then
      remove_container_best_effort "${discovered}"
    fi
  done <<<"${discovered_ids}"
  if [[ "${IMAGE_CREATED}" == true ]]; then
    remove_image_strict >/dev/null 2>&1 || echo "WARNING: failed to remove qualification image ${IMAGE_TAG}" >&2
  fi
  if [[ "${PACKET_DIR}" == /tmp/pulse-rootful-packet.* && -d "${PACKET_DIR}" ]]; then
    find "${PACKET_DIR}" -type f -exec chmod u+w {} + 2>/dev/null || true
    rm -rf -- "${PACKET_DIR}"
  fi
}
trap cleanup EXIT INT TERM

sha256_files() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$@"
  else
    shasum -a 256 "$@"
  fi
}

mkdir "${OUTPUT_DIR}"
chmod 0700 "${OUTPUT_DIR}"

openssl genpkey -algorithm ED25519 -out "${PACKET_DIR}/update-private.pem"
chmod 0600 "${PACKET_DIR}/update-private.pem"
openssl pkey -in "${PACKET_DIR}/update-private.pem" -pubout -outform DER -out "${PACKET_DIR}/update-public.der"
chmod 0600 "${PACKET_DIR}/update-public.der"
update_public_key="$(python3 -I - "${PACKET_DIR}/update-public.der" <<'PY'
import base64
import pathlib
import sys

spki = pathlib.Path(sys.argv[1]).read_bytes()
prefix = bytes.fromhex("302a300506032b6570032100")
if len(spki) != len(prefix) + 32 or not spki.startswith(prefix):
    raise SystemExit("unexpected Ed25519 SubjectPublicKeyInfo encoding")
print(base64.b64encode(spki[len(prefix):]).decode("ascii"), end="")
PY
)"
qualification_version="rootful-v1.${SOURCE_COMMIT:0:12}"
agent_ldflags="$(cd "${REPO_ROOT}" && ./scripts/release_ldflags.sh agent --version "${qualification_version}" --update-public-keys "${update_public_key}")"
helper_ldflags="$(cd "${REPO_ROOT}" && ./scripts/release_ldflags.sh agent --version "${qualification_version}" --update-public-keys "${update_public_key}")"

(
  cd "${REPO_ROOT}"
  CGO_ENABLED=0 GOOS=linux GOARCH="$(go env GOARCH)" GOFLAGS= GOWORK=off go build -trimpath -buildvcs=true -ldflags "${agent_ldflags}" -o "${PACKET_DIR}/pulse-agent" ./cmd/pulse-agent
  CGO_ENABLED=0 GOOS=linux GOARCH="$(go env GOARCH)" GOFLAGS= GOWORK=off go build -trimpath -buildvcs=true -ldflags "${helper_ldflags}" -o "${PACKET_DIR}/pulse-agent-helper" ./cmd/pulse-agent-helper
  CGO_ENABLED=0 GOOS=linux GOARCH="$(go env GOARCH)" GOFLAGS= GOWORK=off go test -c -trimpath -buildvcs=true -o "${PACKET_DIR}/dockeragent.test" ./scripts/installtests
)
openssl pkeyutl -sign -rawin -inkey "${PACKET_DIR}/update-private.pem" -in "${PACKET_DIR}/pulse-agent" | openssl base64 -A >"${PACKET_DIR}/pulse-agent.sig"
printf '\n' >>"${PACKET_DIR}/pulse-agent.sig"
rm -f "${PACKET_DIR}/update-private.pem" "${PACKET_DIR}/update-public.der"
unset update_public_key agent_ldflags helper_ldflags
install -m 0700 "${REPO_ROOT}/scripts/install.sh" "${PACKET_DIR}/install.sh"
chmod 0700 "${PACKET_DIR}/pulse-agent" "${PACKET_DIR}/pulse-agent-helper" "${PACKET_DIR}/dockeragent.test"

verify_vcs_artifact() {
  local artifact="$1" expected_package="$2" metadata
  metadata="$(go version -m "${artifact}")" || {
    echo "ERROR: unable to inspect Go build metadata for ${artifact}" >&2
    return 1
  }
  grep -Fq $'path\t'"${expected_package}" <<<"${metadata}" || {
    echo "ERROR: artifact package identity mismatch for ${artifact}" >&2
    return 1
  }
  grep -Fq "vcs.revision=${SOURCE_COMMIT}" <<<"${metadata}" || {
    echo "ERROR: artifact vcs.revision does not match source commit: ${artifact}" >&2
    return 1
  }
  grep -Fq 'vcs.modified=false' <<<"${metadata}" || {
    echo "ERROR: artifact vcs.modified identity is not clean: ${artifact}" >&2
    return 1
  }
}

verify_vcs_artifact "${PACKET_DIR}/pulse-agent" "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent"
verify_vcs_artifact "${PACKET_DIR}/pulse-agent-helper" "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent-helper"
verify_vcs_artifact "${PACKET_DIR}/dockeragent.test" "github.com/rcourtman/pulse-go-rewrite/scripts/installtests.test"

python3 -I - "${REPO_ROOT}" "${PACKET_DIR}/source-hashes.json" <<'PY'
import importlib.util
import json
import pathlib
import sys

checkout = pathlib.Path(sys.argv[1]).resolve(strict=True)
destination = pathlib.Path(sys.argv[2])
validator_path = checkout / "scripts/release_control/secure_runtime_rootful_attestation_v1.py"
spec = importlib.util.spec_from_file_location("rootful_validator", validator_path)
if spec is None or spec.loader is None:
    raise SystemExit("unable to load rootful receipt validator")
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
_, hashes = module.load_source_manifest(checkout, checkout / module.SOURCE_MANIFEST_PATH)
destination.write_text(json.dumps(hashes, sort_keys=True, separators=(",", ":")) + "\n")
PY
chmod 0600 "${PACKET_DIR}/source-hashes.json" "${PACKET_DIR}/pulse-agent.sig"

cat >"${PACKET_DIR}/Dockerfile" <<EOF
FROM ${UBUNTU_IMAGE}
ENV container=docker DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      bash busybox-static ca-certificates curl dbus gnupg iproute2 jq kmod openssl passwd procps python3 \
      software-properties-common systemd systemd-sysv podman && \
    install -d -m 0755 /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc && \
    chmod 0644 /etc/apt/keyrings/docker.asc && \
    printf 'deb [arch=%s signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu noble stable\n' "\$(dpkg --print-architecture)" >/etc/apt/sources.list.d/docker.list && \
    apt-get update && apt-get install -y --no-install-recommends docker-ce docker-ce-cli containerd.io && \
    apt-get clean && rm -rf /var/lib/apt/lists/* && \
    ln -sf /dev/null /etc/systemd/system/docker.service && \
    ln -sf /dev/null /etc/systemd/system/docker.socket && \
    ln -sf /dev/null /etc/systemd/system/podman.service && \
    ln -sf /dev/null /etc/systemd/system/podman.socket && \
    install -d -m 0700 /opt/pulse/packet /opt/pulse/result && \
    printf '%s\n' disposable-v1 >/etc/pulse-secure-runtime-rootful-qualification && \
    rm -f /etc/machine-id && touch /etc/machine-id && \
    systemctl set-default multi-user.target
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
EOF

docker build --pull --no-cache --network default \
  --label "${CONTAINER_RUN_LABEL}=${CONTAINER_RUN_NONCE}" \
  -t "${IMAGE_TAG}" -f "${PACKET_DIR}/Dockerfile" "${PACKET_DIR}" | tee "${OUTPUT_DIR}/image-build.log"
IMAGE_CREATED=true
docker image inspect "${IMAGE_TAG}" >"${OUTPUT_DIR}/qualification-image-inspect.json"
chmod 0600 "${OUTPUT_DIR}/image-build.log" "${OUTPUT_DIR}/qualification-image-inspect.json"

capture_qualification_container_diagnostics() {
  local runtime_name="$1" container_id="$2"
  docker logs "${container_id}" >"${OUTPUT_DIR}/${runtime_name}-container.log" 2>&1 || true
  docker exec "${container_id}" journalctl --no-pager -n 2000 >"${OUTPUT_DIR}/${runtime_name}-journal.log" 2>&1 || true
  chmod 0600 "${OUTPUT_DIR}/${runtime_name}-container.log" "${OUTPUT_DIR}/${runtime_name}-journal.log"
}

run_runtime() {
  local runtime_name="$1"
  local container_name="pulse-rootful-qual-${runtime_name}-${SOURCE_COMMIT:0:8}-$$"
  local container_id local_receipt machine_id_file machine_id deadline mounts
  local_receipt="${OUTPUT_DIR}/${runtime_name}-receipt.json"
  machine_id_file="${PACKET_DIR}/.machine-id-${runtime_name}"
  machine_id="$(openssl rand -hex 16)"
  if [[ ! "${machine_id}" =~ ^[0-9a-f]{32}$ || "${machine_id}" == "00000000000000000000000000000000" ]]; then
    echo "ERROR: unable to generate a valid machine ID for ${runtime_name}" >&2
    return 1
  fi
  printf '%s\n' "${machine_id}" >"${machine_id_file}"
  chmod 0444 "${machine_id_file}"

  container_id="$(docker create --name "${container_name}" --hostname "pulse-rootful-${runtime_name}" \
    --label "${CONTAINER_RUN_LABEL}=${CONTAINER_RUN_NONCE}" \
    --privileged --network none --cgroupns=private \
    --tmpfs /run:rw,nosuid,nodev,mode=755 --tmpfs /run/lock:rw,nosuid,nodev,mode=755 \
    "${IMAGE_TAG}")"
  CONTAINER_IDS+=("${container_id}")
  docker cp "${machine_id_file}" "${container_id}:/etc/machine-id"
  rm -f -- "${machine_id_file}"
  docker cp "${PACKET_DIR}/." "${container_id}:/opt/pulse/packet"
  docker start "${container_id}" >/dev/null

  deadline=$((SECONDS + 60))
  until docker exec "${container_id}" systemctl is-system-running --wait >/dev/null 2>&1; do
    if (( SECONDS >= deadline )); then
      capture_qualification_container_diagnostics "${runtime_name}" "${container_id}"
      echo "ERROR: ${runtime_name} disposable systemd container did not become ready" >&2
      return 1
    fi
    sleep 1
  done
  if docker exec "${container_id}" sh -c 'ip route | grep -q "^default "'; then
    echo "ERROR: ${runtime_name} qualification container unexpectedly has a default route" >&2
    return 1
  fi
  mounts="$(docker inspect "${container_id}" --format '{{range .Mounts}}{{println .Source "->" .Destination}}{{end}}')"
  if grep -E '/(var/)?run/(docker|podman)(\.sock)?' <<<"${mounts}"; then
    echo "ERROR: host runtime socket was mounted into ${runtime_name} qualification container" >&2
    return 1
  fi

  if ! docker exec \
      -e PULSE_SECURE_RUNTIME_ROOTFUL_QUALIFICATION=disposable-v1 \
      -e "PULSE_ROOTFUL_RUNTIME=${runtime_name}" \
      -e PULSE_ROOTFUL_RECEIPT=/opt/pulse/result/rootful-receipt.json \
      -e PULSE_ROOTFUL_SOURCE_HASHES=/opt/pulse/packet/source-hashes.json \
      -e "PULSE_ROOTFUL_SOURCE_COMMIT=${SOURCE_COMMIT}" \
      -e PULSE_SECURE_RUNTIME_COLLECTOR=/opt/pulse/packet/pulse-agent \
      -e PULSE_SECURE_RUNTIME_COLLECTOR_SIGNATURE=/opt/pulse/packet/pulse-agent.sig \
      -e PULSE_SECURE_RUNTIME_HELPER=/opt/pulse/packet/pulse-agent-helper \
      -e PULSE_SECURE_RUNTIME_INSTALLER=/opt/pulse/packet/install.sh \
      "${container_id}" /opt/pulse/packet/dockeragent.test \
        -test.run '^TestSecureRuntimeRootfulQualification$' -test.count=1 -test.v -test.timeout=45m \
        | tee "${OUTPUT_DIR}/${runtime_name}-test.log"; then
    capture_qualification_container_diagnostics "${runtime_name}" "${container_id}"
    chmod 0600 "${OUTPUT_DIR}/${runtime_name}-test.log"
    return 1
  fi
  docker exec "${container_id}" test -f /opt/pulse/result/rootful-receipt.json || {
    capture_qualification_container_diagnostics "${runtime_name}" "${container_id}"
    echo "ERROR: ${runtime_name} qualification did not retain its receipt" >&2
    return 1
  }
  docker cp "${container_id}:/opt/pulse/result/rootful-receipt.json" "${local_receipt}"
  capture_qualification_container_diagnostics "${runtime_name}" "${container_id}"
  chmod 0600 "${local_receipt}" "${OUTPUT_DIR}/${runtime_name}-test.log"
  remove_container_strict "${container_id}"
  forget_container "${container_id}"
}

run_runtime docker
run_runtime podman

python3 -I - "${OUTPUT_DIR}/docker-receipt.json" "${OUTPUT_DIR}/podman-receipt.json" "${OUTPUT_DIR}/receipt.json" <<'PY'
import json
import pathlib
import sys

docker_path, podman_path, output_path = map(pathlib.Path, sys.argv[1:])
docker = json.loads(docker_path.read_text())
podman = json.loads(podman_path.read_text())
if docker.get("result") != "passed" or podman.get("result") != "passed":
    raise SystemExit('per-runtime qualification result != "passed"')
for field in ("schema_version", "kind", "source_commit", "source_hashes", "artifacts"):
    if docker.get(field) != podman.get(field):
        raise SystemExit(f"per-runtime qualification field differs: {field}")
runs = docker.get("runs", []) + podman.get("runs", [])
if [run.get("runtime", {}).get("runtime") for run in runs] != ["docker", "podman"]:
    raise SystemExit("per-runtime receipts are not exact Docker then Podman runs")
machine_ids = [run.get("host", {}).get("machine_id") for run in runs]
daemon_ids = [run.get("runtime", {}).get("daemon_id") for run in runs]
if len(set(machine_ids)) != 2 or len(set(daemon_ids)) != 2:
    raise SystemExit("Docker and Podman qualification hosts/daemons must have distinct identities")
combined = {
    "schema_version": docker["schema_version"], "kind": docker["kind"], "result": "passed",
    "source_commit": docker["source_commit"],
    "started_at": min(docker["started_at"], podman["started_at"]),
    "completed_at": max(docker["completed_at"], podman["completed_at"]),
    "source_hashes": docker["source_hashes"], "artifacts": docker["artifacts"], "runs": runs,
}
output_path.write_text(json.dumps(combined, indent=2, sort_keys=True) + "\n")
PY
chmod 0600 "${OUTPUT_DIR}/receipt.json"

python3 -I "${REPO_ROOT}/scripts/release_control/secure_runtime_rootful_attestation_v1.py" \
  "${OUTPUT_DIR}/receipt.json" \
  --qualification-test "${PACKET_DIR}/dockeragent.test" \
  --collector "${PACKET_DIR}/pulse-agent" \
  --helper "${PACKET_DIR}/pulse-agent-helper" \
  --installer "${PACKET_DIR}/install.sh" \
  >"${OUTPUT_DIR}/attestation.json"
chmod 0600 "${OUTPUT_DIR}/attestation.json"

sha256_files "${OUTPUT_DIR}/receipt.json" "${OUTPUT_DIR}/attestation.json" >"${OUTPUT_DIR}/sha256.txt"
chmod 0600 "${OUTPUT_DIR}/sha256.txt"

if [[ -n "$(docker ps -aq --no-trunc --filter "label=${CONTAINER_RUN_LABEL}=${CONTAINER_RUN_NONCE}")" ]]; then
  echo "ERROR: labeled qualification containers remain after strict cleanup" >&2
  exit 1
fi
remove_image_strict
if [[ -n "$(docker images -q --filter "label=${CONTAINER_RUN_LABEL}=${CONTAINER_RUN_NONCE}")" ]]; then
  echo "ERROR: labeled qualification images remain after strict cleanup" >&2
  exit 1
fi
while IFS= read -r retained; do
  [[ -z "${retained}" ]] && continue
  if [[ "$(portable_uid "${retained}")" != "$(id -u)" ]]; then
    echo "ERROR: retained output is not owned by the invoking user: ${retained}" >&2
    exit 1
  fi
  chmod go-rwx "${retained}"
done < <(find "${OUTPUT_DIR}" -mindepth 1 -maxdepth 1 -type f -print)

echo "Rootful qualification passed: ${OUTPUT_DIR}"
