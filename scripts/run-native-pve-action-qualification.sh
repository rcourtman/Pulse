#!/usr/bin/env bash
set -euo pipefail

readonly REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PVE_HOST="${PVE_HOST:?set PVE_HOST to the dedicated disposable PVE node}"
PVE_USER="${PVE_USER:-root}"
PVE_VMID="${PVE_VMID:?set PVE_VMID to the disposable VM ID}"
PVE_CTID="${PVE_CTID:?set PVE_CTID to the disposable container ID}"
PVE_KNOWN_HOSTS_FILE="${PVE_KNOWN_HOSTS_FILE:?set PVE_KNOWN_HOSTS_FILE to an absolute pinned known_hosts file}"
PULSE_PVE_QUALIFICATION_CONFIRM="${PULSE_PVE_QUALIFICATION_CONFIRM:-}"
PULSE_PVE_QUALIFICATION_OUTPUT_DIR="${PULSE_PVE_QUALIFICATION_OUTPUT_DIR:?set PULSE_PVE_QUALIFICATION_OUTPUT_DIR to an absolute local evidence directory}"

if [[ "${PVE_USER}" != "root" ]]; then
  echo "ERROR: native PVE qualification requires PVE_USER=root" >&2
  exit 2
fi
if [[ ! "${PVE_HOST}" =~ ^[A-Za-z0-9._:-]+$ || "${PVE_HOST}" == -* ]]; then
  echo "ERROR: PVE_HOST contains unsupported characters" >&2
  exit 2
fi
if [[ ! "${PVE_VMID}" =~ ^[1-9][0-9]{0,8}$ || ! "${PVE_CTID}" =~ ^[1-9][0-9]{0,8}$ || "${PVE_VMID}" == "${PVE_CTID}" ]]; then
  echo "ERROR: PVE_VMID and PVE_CTID must be distinct positive decimal guest IDs" >&2
  exit 2
fi
if [[ "${PVE_KNOWN_HOSTS_FILE}" != /* || ! -f "${PVE_KNOWN_HOSTS_FILE}" ]]; then
  echo "ERROR: PVE_KNOWN_HOSTS_FILE must be an existing absolute path" >&2
  exit 2
fi
if [[ "${PULSE_PVE_QUALIFICATION_OUTPUT_DIR}" != /* || ! -d "${PULSE_PVE_QUALIFICATION_OUTPUT_DIR}" ]]; then
  echo "ERROR: PULSE_PVE_QUALIFICATION_OUTPUT_DIR must be absolute" >&2
  exit 2
fi
if [[ -n "$(git -C "${REPO_ROOT}" status --porcelain)" ]]; then
  echo "ERROR: qualification builds require a clean exact source checkout" >&2
  exit 2
fi

readonly PVE_TARGET="${PVE_USER}@${PVE_HOST}"
readonly SOURCE_COMMIT="$(git -C "${REPO_ROOT}" rev-parse HEAD)"
readonly RUN_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
readonly OUTPUT_DIR="${PULSE_PVE_QUALIFICATION_OUTPUT_DIR}/${RUN_STAMP}-${SOURCE_COMMIT:0:12}"
LOCAL_BUILD_DIR=""
REMOTE_DIR=""
REMOVE_REMOTE_DIR=false

ssh_cmd() {
  ssh -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
    -o "UserKnownHostsFile=${PVE_KNOWN_HOSTS_FILE}" -- "${PVE_TARGET}" "$@"
}

cleanup() {
  if [[ "${REMOVE_REMOTE_DIR}" == "true" && -n "${REMOTE_DIR}" && "${REMOTE_DIR}" == /run/pulse-pve-qualification.* ]]; then
    ssh_cmd "rm -f '${REMOTE_DIR}/pulse-agent-runner' '${REMOTE_DIR}/hostagent.test' '${REMOTE_DIR}/state/manifest.json' '${REMOTE_DIR}/state/receipt.json' '${REMOTE_DIR}/state/cleanup.json'; rmdir '${REMOTE_DIR}/state/typed-actions' >/dev/null 2>&1 || true; rmdir '${REMOTE_DIR}/state' '${REMOTE_DIR}'" >/dev/null 2>&1 || true
  elif [[ -n "${REMOTE_DIR}" && "${REMOTE_DIR}" == /run/pulse-pve-qualification.* ]]; then
    echo "WARNING: incomplete qualification state retained for inspection at ${PVE_TARGET}:${REMOTE_DIR}" >&2
  fi
  if [[ -n "${LOCAL_BUILD_DIR}" && "${LOCAL_BUILD_DIR}" == /tmp/pulse-pve-qualification.* ]]; then
    rm -f "${LOCAL_BUILD_DIR}/pulse-agent-runner" "${LOCAL_BUILD_DIR}/hostagent.test" >/dev/null 2>&1 || true
    rmdir "${LOCAL_BUILD_DIR}" >/dev/null 2>&1 || true
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

if ! ssh-keygen -F "${PVE_HOST}" -f "${PVE_KNOWN_HOSTS_FILE}" >/dev/null; then
  echo "ERROR: PVE_HOST is not pinned in PVE_KNOWN_HOSTS_FILE" >&2
  exit 2
fi
if [[ "$(ssh_cmd "id -u")" != "0" ]]; then
  echo "ERROR: the PVE SSH session must run as root" >&2
  exit 2
fi

remote_machine_id="$(ssh_cmd "tr -d '\\r\\n' </etc/machine-id")"
remote_node="$(ssh_cmd "basename \"\$(readlink -f /etc/pve/local)\"")"
if [[ ! "${remote_machine_id}" =~ ^[0-9a-f]{32}$ || ! "${remote_node}" =~ ^[A-Za-z0-9][A-Za-z0-9.-]*$ ]]; then
  echo "ERROR: target did not return a valid PVE machine/node identity" >&2
  exit 2
fi
remote_cluster_id="$(ssh_cmd "if test -f /etc/pve/corosync.conf; then printf 'corosync-sha256:'; sha256sum /etc/pve/corosync.conf | cut -d ' ' -f 1; else printf 'standalone:${remote_node}'; fi")"
cluster_confirmation_hash="$(printf '%s' "${remote_cluster_id}" | sha256_files | awk '{print substr($1,1,12)}')"
expected_confirmation="I_HAVE_VERIFIED_THIS_DISPOSABLE_PVE_TARGET_MACHINE_${remote_machine_id:0:12}_NODE_${remote_node}_CLUSTER_${cluster_confirmation_hash}_COMMIT_${SOURCE_COMMIT}_VM_${PVE_VMID}_CT_${PVE_CTID}"
if [[ "${PULSE_PVE_QUALIFICATION_CONFIRM}" != "${expected_confirmation}" ]]; then
  echo "ERROR: exact destructive confirmation required:" >&2
  echo "export PULSE_PVE_QUALIFICATION_CONFIRM=${expected_confirmation}" >&2
  exit 2
fi

remote_runner_load_state="$(ssh_cmd "/usr/bin/systemctl show --property=LoadState --value pulse-agent-runner.service")"
if [[ "${remote_runner_load_state}" != "not-found" ]]; then
  echo "ERROR: target already has a loaded pulse-agent-runner.service; use a dedicated disposable PVE node" >&2
  exit 2
fi
if [[ -n "$(ssh_cmd "/usr/bin/systemctl list-units --all --full --plain --no-legend 'pulse-agent-action-*.service'")" ]]; then
  echo "ERROR: target already has Pulse typed-action units" >&2
  exit 2
fi

remote_arch="$(ssh_cmd "uname -m")"
case "${remote_arch}" in
  x86_64) go_arch=amd64 ;;
  aarch64|arm64) go_arch=arm64 ;;
  *)
    echo "ERROR: unsupported PVE architecture: ${remote_arch}" >&2
    exit 2
    ;;
esac

mkdir "${OUTPUT_DIR}"
chmod 0700 "${OUTPUT_DIR}"
LOCAL_BUILD_DIR="$(mktemp -d /tmp/pulse-pve-qualification.XXXXXX)"
echo "Building exact ${SOURCE_COMMIT} qualification binaries for linux/${go_arch}"
(
  cd "${REPO_ROOT}"
  CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" GOFLAGS= GOWORK=off go build -trimpath -o "${LOCAL_BUILD_DIR}/pulse-agent-runner" ./cmd/pulse-agent-runner
  CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" GOFLAGS= GOWORK=off go test -c -trimpath -o "${LOCAL_BUILD_DIR}/hostagent.test" ./internal/hostagent
)
chmod 0700 "${LOCAL_BUILD_DIR}/pulse-agent-runner" "${LOCAL_BUILD_DIR}/hostagent.test"
sha256_files "${LOCAL_BUILD_DIR}/pulse-agent-runner" "${LOCAL_BUILD_DIR}/hostagent.test" >"${OUTPUT_DIR}/artifact-sha256.txt"

REMOTE_DIR="$(ssh_cmd "mktemp -d /run/pulse-pve-qualification.XXXXXX")"
if [[ "${REMOTE_DIR}" != /run/pulse-pve-qualification.* ]]; then
  echo "ERROR: remote qualification directory was outside the expected boundary" >&2
  exit 2
fi
scp -q -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
  -o "UserKnownHostsFile=${PVE_KNOWN_HOSTS_FILE}" -- \
  "${LOCAL_BUILD_DIR}/pulse-agent-runner" "${LOCAL_BUILD_DIR}/hostagent.test" \
  "${PVE_TARGET}:${REMOTE_DIR}/"
ssh_cmd "chown root:root '${REMOTE_DIR}/pulse-agent-runner' '${REMOTE_DIR}/hostagent.test' && chmod 0700 '${REMOTE_DIR}/pulse-agent-runner' '${REMOTE_DIR}/hostagent.test' && mkdir -m 0700 '${REMOTE_DIR}/state'"

remote_hashes="$(ssh_cmd "sha256sum '${REMOTE_DIR}/pulse-agent-runner' '${REMOTE_DIR}/hostagent.test'")"
local_runner_hash="$(sha256_files "${LOCAL_BUILD_DIR}/pulse-agent-runner" | awk '{print $1}')"
local_test_hash="$(sha256_files "${LOCAL_BUILD_DIR}/hostagent.test" | awk '{print $1}')"
if ! grep -q "^${local_runner_hash} " <<<"${remote_hashes}" || ! grep -q "^${local_test_hash} " <<<"${remote_hashes}"; then
  echo "ERROR: copied qualification artifact hashes did not match" >&2
  exit 2
fi

echo "Running native PVE qualification against disposable VM ${PVE_VMID} and CT ${PVE_CTID}"
qualification_unit="pulse-pve-qualification-${SOURCE_COMMIT:0:8}-${PVE_VMID}-${PVE_CTID}.service"
evidence_unit="${qualification_unit%.service}-evidence.service"
if [[ "$(ssh_cmd "/usr/bin/systemctl show --property=LoadState --value '${qualification_unit}'")" != "not-found" ]]; then
  echo "ERROR: qualification supervisor unit already exists: ${qualification_unit}" >&2
  exit 2
fi
if [[ "$(ssh_cmd "/usr/bin/systemctl show --property=LoadState --value '${evidence_unit}'")" != "not-found" ]]; then
  echo "ERROR: qualification evidence-retention unit already exists: ${evidence_unit}" >&2
  exit 2
fi
cleanup_exec="/usr/bin/env PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin LC_ALL=C PULSE_AGENT_RUNNER_STATE_DIR=${REMOTE_DIR}/state PULSE_TEST_PVE_CLEANUP_MANIFEST=${REMOTE_DIR}/state/manifest.json PULSE_TEST_PVE_CLEANUP_RECEIPT=${REMOTE_DIR}/state/cleanup.json ${REMOTE_DIR}/hostagent.test -test.v -test.run TestNativePVEQualificationSupervisorCleanup -test.timeout=15m"
ssh_cmd "/usr/bin/systemd-run --no-ask-password --quiet --remain-after-exit --service-type=exec --unit='${qualification_unit}' --property=User=root --property=Group=root --property=UMask=0077 --property=WorkingDirectory=/ --property=RuntimeMaxSec=20m --property=TimeoutStopSec=20m --property='ExecStopPost=${cleanup_exec}' -- /usr/bin/env PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin LC_ALL=C PULSE_AGENT_RUNNER_STATE_DIR='${REMOTE_DIR}/state' PULSE_TEST_PVE_SUPERVISOR_UNIT='${qualification_unit}' PULSE_TEST_PVE_CONFIRM='${expected_confirmation}' PULSE_TEST_PVE_VM_ID='${PVE_VMID}' PULSE_TEST_PVE_CT_ID='${PVE_CTID}' PULSE_TEST_ACTION_RUNNER_BINARY='${REMOTE_DIR}/pulse-agent-runner' PULSE_TEST_ACTION_RUNNER_SHA256='${local_runner_hash}' PULSE_TEST_HOSTAGENT_SHA256='${local_test_hash}' PULSE_TEST_PVE_RECEIPT_PATH='${REMOTE_DIR}/state/receipt.json' PULSE_TEST_SOURCE_COMMIT='${SOURCE_COMMIT}' '${REMOTE_DIR}/hostagent.test' -test.v -test.run TestNativePVEProxmoxGuestLifecycleQualification -test.timeout=0"

supervisor_properties="$(ssh_cmd "/usr/bin/systemctl show --property=InvocationID --property=ActiveState --property=ExecStopPost '${qualification_unit}'")"
supervisor_invocation_id="$(sed -n 's/^InvocationID=//p' <<<"${supervisor_properties}")"
if [[ ! "${supervisor_invocation_id}" =~ ^[0-9a-f]{32}$ ]] ||
   ! grep -qx 'ActiveState=active' <<<"${supervisor_properties}" ||
   ! grep '^ExecStopPost=' <<<"${supervisor_properties}" | grep -Fq "PULSE_AGENT_RUNNER_STATE_DIR=${REMOTE_DIR}/state"; then
  echo "ERROR: qualification supervisor did not start with an exact active InvocationID" >&2
  exit 1
fi
ssh_cmd "/usr/bin/systemd-run --no-ask-password --quiet --collect --service-type=exec --unit='${evidence_unit}' --property='Wants=${qualification_unit}' --property='After=${qualification_unit}' --property=RuntimeMaxSec=45m -- /usr/bin/sleep infinity"
evidence_unit_properties="$(ssh_cmd "/usr/bin/systemctl show --property=InvocationID --property=ActiveState --property=Wants --property=After '${evidence_unit}'")"
evidence_unit_invocation_id="$(sed -n 's/^InvocationID=//p' <<<"${evidence_unit_properties}")"
if [[ ! "${evidence_unit_invocation_id}" =~ ^[0-9a-f]{32}$ ]] ||
   ! grep -qx 'ActiveState=active' <<<"${evidence_unit_properties}" ||
   ! sed -n 's/^Wants=//p' <<<"${evidence_unit_properties}" | tr ' ' '\n' | grep -Fxq "${qualification_unit}" ||
   ! sed -n 's/^After=//p' <<<"${evidence_unit_properties}" | tr ' ' '\n' | grep -Fxq "${qualification_unit}"; then
  echo "ERROR: supervisor evidence-retention unit was not bound exactly" >&2
  exit 1
fi

deadline=$((SECONDS + 2400))
stop_requested=false
supervisor_stop_status=not-requested
pre_stop_supervisor_properties=""
while (( SECONDS < deadline )); do
  supervisor_active_state="$(ssh_cmd "/usr/bin/systemctl show --property=ActiveState --value '${qualification_unit}'" 2>/dev/null || true)"
  if [[ "${stop_requested}" == "false" && "${supervisor_active_state}" == "active" ]] && ssh_cmd "test -s '${REMOTE_DIR}/state/receipt.json'" >/dev/null 2>&1; then
    stop_requested=true
    pre_stop_supervisor_properties="$(ssh_cmd "/usr/bin/systemctl show --property=InvocationID --property=ActiveState --property=Result --property=ExecMainStatus '${qualification_unit}'")"
    if ! grep -qx "InvocationID=${supervisor_invocation_id}" <<<"${pre_stop_supervisor_properties}" ||
       ! grep -qx 'ActiveState=active' <<<"${pre_stop_supervisor_properties}" ||
       ! grep -qx 'Result=success' <<<"${pre_stop_supervisor_properties}" ||
       ! grep -qx 'ExecMainStatus=0' <<<"${pre_stop_supervisor_properties}"; then
      echo "ERROR: qualification main process did not reach the exact successful terminal state" >&2
      exit 1
    fi
    if ssh_cmd "/usr/bin/timeout 21m /usr/bin/systemctl --no-ask-password stop '${qualification_unit}'"; then
      supervisor_stop_status=0
    else
      supervisor_stop_status=failed
      echo "ERROR: qualification supervisor stop/ExecStopPost job failed" >&2
      exit 1
    fi
    supervisor_active_state="$(ssh_cmd "/usr/bin/systemctl show --property=ActiveState --value '${qualification_unit}'" 2>/dev/null || true)"
  fi
  if ssh_cmd "test -s '${REMOTE_DIR}/state/cleanup.json'" >/dev/null 2>&1 && [[ -z "${supervisor_active_state}" || "${supervisor_active_state}" == "inactive" || "${supervisor_active_state}" == "failed" ]]; then
    break
  fi
  sleep 5
done
if ! ssh_cmd "test -s '${REMOTE_DIR}/state/cleanup.json'" >/dev/null 2>&1; then
  ssh_cmd "/usr/bin/timeout 21m /usr/bin/systemctl stop '${qualification_unit}'" >/dev/null 2>&1 || true
  echo "ERROR: qualification supervisor did not produce bounded cleanup evidence" >&2
  exit 1
fi

final_supervisor_properties="$(ssh_cmd "/usr/bin/systemctl show --property=InvocationID --property=ActiveState --property=Result --property=ExecMainStatus --property=ExecStopPost '${qualification_unit}'" 2>/dev/null || true)"
if [[ "${supervisor_stop_status}" != "0" ]] ||
   ! grep -qx "InvocationID=${supervisor_invocation_id}" <<<"${final_supervisor_properties}" ||
   ! grep -qx 'ActiveState=inactive' <<<"${final_supervisor_properties}" ||
   ! grep -qx 'Result=success' <<<"${final_supervisor_properties}" ||
   ! grep -qx 'ExecMainStatus=0' <<<"${final_supervisor_properties}" ||
   ! grep '^ExecStopPost=' <<<"${final_supervisor_properties}" | grep -Eq 'code=exited.*status=0([ /;}]|$)'; then
  echo "ERROR: qualification supervisor did not complete with the exact successful main and ExecStopPost result" >&2
  exit 1
fi
if ! ssh_cmd "/usr/bin/timeout 30s /usr/bin/systemctl --no-ask-password stop '${evidence_unit}'"; then
  echo "ERROR: qualification evidence-retention unit did not stop" >&2
  exit 1
fi

ssh_cmd "/usr/bin/journalctl --no-pager --output=short-iso-precise --unit='${qualification_unit}'" >"${OUTPUT_DIR}/transcript.txt"
printf '%s\n' "started:" "${supervisor_properties}" "before_stop:" "${pre_stop_supervisor_properties}" "stop_status=${supervisor_stop_status}" "after_stop:" "${final_supervisor_properties}" >"${OUTPUT_DIR}/supervisor-properties.txt"
for name in manifest.json cleanup.json; do
  scp -q -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
    -o "UserKnownHostsFile=${PVE_KNOWN_HOSTS_FILE}" -- \
    "${PVE_TARGET}:${REMOTE_DIR}/state/${name}" "${OUTPUT_DIR}/${name}"
done
scp -q -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
  -o "UserKnownHostsFile=${PVE_KNOWN_HOSTS_FILE}" -- \
  "${PVE_TARGET}:${REMOTE_DIR}/state/receipt.json" "${OUTPUT_DIR}/receipt.json"
sha256_files "${OUTPUT_DIR}/transcript.txt" >"${OUTPUT_DIR}/transcript-sha256.txt"
printf '%s\n' "source_commit=${SOURCE_COMMIT}" "pve_node=${remote_node}" "pve_machine_id=${remote_machine_id}" "pve_cluster_id=${remote_cluster_id}" "pve_vmid=${PVE_VMID}" "pve_ctid=${PVE_CTID}" "supervisor_unit=${qualification_unit}" "supervisor_invocation_id=${supervisor_invocation_id}" "evidence_unit=${evidence_unit}" "evidence_unit_invocation_id=${evidence_unit_invocation_id}" "completed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"${OUTPUT_DIR}/run-metadata.txt"
chmod 0600 "${OUTPUT_DIR}"/*

if ! python3 -I - "${OUTPUT_DIR}/manifest.json" "${OUTPUT_DIR}/cleanup.json" "${OUTPUT_DIR}/receipt.json" "${SOURCE_COMMIT}" "${remote_machine_id}" "${remote_node}" "${remote_cluster_id}" "${local_runner_hash}" "${local_test_hash}" "${qualification_unit}" "${supervisor_invocation_id}" "${PVE_VMID}" "${PVE_CTID}" <<'PY'
import hashlib
import json
import pathlib
import sys
manifest_path, cleanup_path, receipt_path = map(pathlib.Path, sys.argv[1:4])
source_commit, machine_id, node, cluster_id, runner_hash, test_hash, supervisor_unit, supervisor_invocation_id, vmid, ctid = sys.argv[4:]
manifest_bytes = manifest_path.read_bytes()
manifest = json.loads(manifest_bytes)
cleanup = json.loads(cleanup_path.read_text())
receipt = json.loads(receipt_path.read_text())
manifest_hash = hashlib.sha256(manifest_bytes).hexdigest()

def require(condition, message):
    if not condition:
        raise SystemExit(f"invalid native PVE qualification evidence: {message}")

expected_common = {
    "source_commit": source_commit,
    "machine_id": machine_id,
    "node": node,
    "cluster_id": cluster_id,
    "supervisor_unit": supervisor_unit,
    "supervisor_invocation_id": supervisor_invocation_id,
    "runner_sha256": runner_hash,
    "test_sha256": test_hash,
}
require(manifest.get("schema_version") == 1, "manifest schema")
require(all(manifest.get(key) == value for key, value in expected_common.items()), "manifest run binding")
require(len(manifest.get("guests", [])) == 2, "manifest guest count")
require({(item.get("kind"), item.get("vmid")) for item in manifest["guests"]} == {("vm", int(vmid)), ("ct", int(ctid))}, "manifest guest identities")
require(all(item.get("node") == node and item.get("config_digest") and item.get("bridges") and item.get("networks") for item in manifest["guests"]), "manifest guest evidence")
require(cleanup.get("schema_version") == 1 and cleanup.get("result") == "passed", "cleanup result")
require(all(cleanup.get(key) == value for key, value in expected_common.items()), "cleanup run binding")
require(cleanup.get("manifest_sha256") == manifest_hash, "cleanup manifest hash")
require(cleanup.get("runner_anchor_invocation_id") == manifest.get("runner_anchor_invocation_id"), "cleanup runner anchor")
require(cleanup.get("anchor_stopped") is True, "cleanup anchor state")
require(cleanup.get("action_units_gone") is True and cleanup.get("sideband_empty") is True, "cleanup containment")
require(len(cleanup.get("guests", [])) == 2 and all(item.get("stopped") is True and not item.get("error") for item in cleanup["guests"]), "cleanup guest states")
require([item.get("identity") for item in cleanup["guests"]] == manifest["guests"], "cleanup guest binding")
require(receipt.get("schema_version") == 1 and receipt.get("result") == "passed", "qualification result")
require(all(receipt.get(key) == value for key, value in expected_common.items()), "qualification run binding")
require(receipt.get("manifest_sha256") == manifest_hash, "qualification manifest hash")
require(receipt.get("runner_anchor_invocation_id") == manifest.get("runner_anchor_invocation_id"), "qualification runner anchor")
require(len(receipt.get("guests", [])) == 2 and all(item.get("final_state") == "stopped" and item.get("emergency_cleanup") is False for item in receipt["guests"]), "qualification guest states")
require([item.get("identity") for item in receipt["guests"]] == manifest["guests"], "qualification guest binding")
expected_operations = ["start", "reboot", "shutdown", "start", "stop"]
expected_before = ["stopped", "running", "running", "stopped", "running"]
expected_after = ["running", "running", "stopped", "running", "stopped"]
for guest in receipt["guests"]:
    operations = guest.get("operations", [])
    identity = guest["identity"]
    require([item.get("operation") for item in operations] == expected_operations, "operation sequence")
    require([item.get("result", {}).get("before", {}).get("status") for item in operations] == expected_before, "operation before states")
    require([item.get("result", {}).get("after", {}).get("status") for item in operations] == expected_after, "operation after states")
    require(all(item.get("action_units_gone") is True and item.get("sideband_empty") is True for item in operations), "per-operation containment")
    for index, item in enumerate(operations):
        result = item.get("result", {})
        require(result.get("operation") == expected_operations[index] and result.get("guest_kind") == identity["kind"] and result.get("vmid") == identity["vmid"], "production result identity")
        require(result.get("execution_phase") == "complete" and result.get("mutation_started") is True and result.get("mutation_completed") is True and result.get("readback_ran") is True, "production result completion")
        require(not result.get("error") and not result.get("reason_code"), "production result error fields")
        if expected_after[index] == "running":
            cgroup = item.get("cgroup", "")
            vmid_text = str(identity["vmid"])
            cgroup_markers = [f"qemu-{vmid_text}.scope", f"/{vmid_text}.scope"] if identity["kind"] == "vm" else [f"lxc.payload.{vmid_text}", f"/lxc/{vmid_text}/", f"machine-lxc\\x2d{vmid_text}.scope"]
            require(any(marker in cgroup for marker in cgroup_markers), "VMID-specific cgroup")
            link_paths = item.get("link_paths", {})
            require(set(link_paths) == set(identity["networks"]), "per-NIC link-path keys")
            require(all(isinstance(link_paths[network], list) and len(link_paths[network]) >= 2 and link_paths[network][-1] == bridge for network, bridge in identity["networks"].items()), "per-NIC live bridge paths")
PY
then
  echo "ERROR: qualification or supervisor cleanup receipt was not structurally passing" >&2
  exit 1
fi
if [[ "$(ssh_cmd "/usr/bin/systemctl show --property=LoadState --value pulse-agent-runner.service")" != "not-found" || -n "$(ssh_cmd "/usr/bin/systemctl list-units --all --full --plain --no-legend 'pulse-agent-action-*.service'")" ]]; then
  echo "ERROR: qualification cleanup left Pulse runtime units behind" >&2
  exit 1
fi
for evidence_file in "${OUTPUT_DIR}"/*; do
  [[ "${evidence_file}" == "${OUTPUT_DIR}/evidence-sha256.txt" ]] && continue
  sha256_files "${evidence_file}"
done >"${OUTPUT_DIR}/evidence-sha256.txt"
chmod 0600 "${OUTPUT_DIR}/evidence-sha256.txt"
ssh_cmd "/usr/bin/systemctl reset-failed '${qualification_unit}'" >/dev/null 2>&1 || true
REMOVE_REMOTE_DIR=true
echo "Native PVE qualification passed; evidence retained at ${OUTPUT_DIR}"
