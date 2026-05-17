#!/usr/bin/env bash
#
# Smoke tests for the top-level server installer.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INSTALL_SCRIPT="${ROOT_DIR}/install.sh"

if [[ ! -f "${INSTALL_SCRIPT}" ]]; then
  echo "install.sh not found at ${INSTALL_SCRIPT}" >&2
  exit 1
fi

failures=0

assert_success() {
  local desc="$1"
  shift
  if "$@"; then
    echo "[PASS] ${desc}"
    return 0
  else
    echo "[FAIL] ${desc}" >&2
    ((failures++))
    return 1
  fi
}

load_installer() {
  # shellcheck disable=SC1090
  source "${INSTALL_SCRIPT}"
  trap - EXIT
}

test_infer_release_from_archive_name_supports_prerelease() {
  (
    load_installer
    local version
    version="$(infer_release_from_archive_name "/tmp/pulse-v5.1.27-rc.1-linux-arm64.tar.gz")"
    [[ "${version}" == "v5.1.27-rc.1" ]]
  )
}

test_ensure_update_disk_headroom_fails_when_tmp_and_install_share_full_filesystem() {
  (
    load_installer

    UPDATE_MIN_TEMP_FREE_BYTES=$((100 * 1024))
    UPDATE_MIN_INSTALL_FREE_BYTES=$((80 * 1024))

    print_error() { :; }
    print_info() { :; }
    print_warn() { :; }
    df() {
      if [[ "$1" == "-Pk" ]]; then
        case "$2" in
          /tmp|/opt/pulse)
            printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
            printf '/dev/shared 1000 0 150 0%% /\n'
            return 0
            ;;
        esac
      fi
      command df "$@"
    }

    if ensure_update_disk_headroom /tmp /opt/pulse; then
      echo "ensure_update_disk_headroom unexpectedly passed on a shared full filesystem" >&2
      return 1
    fi
  )
}

test_ensure_update_disk_headroom_accepts_separate_filesystems_with_sufficient_space() {
  (
    load_installer

    UPDATE_MIN_TEMP_FREE_BYTES=$((100 * 1024))
    UPDATE_MIN_INSTALL_FREE_BYTES=$((80 * 1024))

    print_error() { :; }
    print_info() { :; }
    print_warn() { :; }
    df() {
      if [[ "$1" == "-Pk" ]]; then
        case "$2" in
          /tmp)
            printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
            printf '/dev/tmp 1000 0 120 0%% /tmp\n'
            return 0
            ;;
          /opt/pulse)
            printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
            printf '/dev/root 1000 0 90 0%% /\n'
            return 0
            ;;
        esac
      fi
      command df "$@"
    }

    ensure_update_disk_headroom /tmp /opt/pulse
  )
}

test_download_pulse_installs_from_local_archive_without_network() {
  (
    load_installer

    local tmpdir archive_root archive_path
    tmpdir="$(mktemp -d)"
    archive_root="${tmpdir}/archive"
    archive_path="${tmpdir}/pulse-v5.1.99-linux-amd64.tar.gz"

    mkdir -p "${archive_root}/bin"
    cat > "${archive_root}/bin/pulse" <<'EOF'
#!/usr/bin/env bash
echo "v5.1.99"
EOF
    chmod +x "${archive_root}/bin/pulse"
    printf 'v5.1.99\n' > "${archive_root}/VERSION"
    tar -czf "${archive_path}" -C "${archive_root}" .

    INSTALL_DIR="${tmpdir}/opt/pulse"
    CONFIG_DIR="${tmpdir}/etc/pulse"
    BUILD_FROM_SOURCE=false
    SKIP_DOWNLOAD=false
    ARCHIVE_OVERRIDE="${archive_path}"
    FORCE_VERSION=""
    FORCE_CHANNEL=""
    UPDATE_CHANNEL="stable"
    LATEST_RELEASE=""
    STOPPED_PULSE_SERVICE=""

    mkdir -p "${INSTALL_DIR}/bin" "${CONFIG_DIR}"

    detect_service_name() { echo "pulse"; }
    stop_pulse_service_for_update() { return 0; }
    restore_selinux_contexts() { :; }
    install_additional_agent_binaries() { return 0; }
    deploy_agent_scripts() { return 0; }
    validate_pulse_binary_architecture() { return 0; }
    chown() { :; }
    ln() { :; }
    curl() { echo "unexpected curl call" >&2; return 99; }
    wget() { echo "unexpected wget call" >&2; return 99; }

    download_pulse

    [[ -x "${INSTALL_DIR}/bin/pulse" ]]
    [[ "$("${INSTALL_DIR}/bin/pulse" --version)" == "v5.1.99" ]]
    [[ "${LATEST_RELEASE}" == "v5.1.99" ]]
  )
}

test_prefetch_pulse_archive_for_container_sets_output_var() {
  (
    load_installer

    local archive_path=""
    LATEST_RELEASE="v5.1.42"

    resolve_target_release() { :; }
    download_release_archive() {
      printf 'test archive\n' > "$3"
      return 0
    }
    uname() { echo "x86_64"; }

    prefetch_pulse_archive_for_container archive_path

    [[ "${archive_path}" == /tmp/pulse-v5.1.42-amd64-lxc-*.tar.gz ]]
    [[ -f "${archive_path}" ]]
    rm -f "${archive_path}"
  )
}

test_resolve_target_release_ignores_host_configured_rc_channel_when_requested() {
  (
    load_installer

    local tmpdir
    tmpdir="$(mktemp -d)"
    CONFIG_DIR="${tmpdir}/etc/pulse"
    mkdir -p "${CONFIG_DIR}"
    cat > "${CONFIG_DIR}/system.json" <<'EOF'
{"updateChannel":"rc"}
EOF

    IGNORE_CONFIGURED_UPDATE_CHANNEL=true
    FORCE_VERSION=""
    FORCE_CHANNEL=""
    UPDATE_CHANNEL=""
    LATEST_RELEASE=""

    curl() {
      cat <<'EOF'
[
  {"tag_name":"v6.0.0-rc.2","draft":false,"prerelease":true},
  {"tag_name":"v5.1.28","draft":false,"prerelease":false}
]
EOF
    }

    resolve_target_release >/dev/null

    [[ "${UPDATE_CHANNEL}" == "stable" ]]
    [[ "${LATEST_RELEASE}" == "v5.1.28" ]]

    rm -rf "${tmpdir}"
  )
}

test_resolve_target_release_skips_prerelease_without_jq() {
  (
    load_installer

    FORCE_VERSION=""
    FORCE_CHANNEL=""
    UPDATE_CHANNEL=""
    LATEST_RELEASE=""

    command() {
      if [[ "$1" == "-v" && "${2:-}" == "jq" ]]; then
        return 1
      fi
      builtin command "$@"
    }

    curl() {
      cat <<'EOF'
[
  {
    "tag_name": "v6.0.0-rc.5",
    "draft": false,
    "prerelease": true
  },
  {
    "tag_name": "v5.1.30",
    "draft": false,
    "prerelease": false
  }
]
EOF
    }

    get_latest_release_from_redirect() {
      printf '%s\n' "v6.0.0-rc.5"
    }

    resolve_target_release >/dev/null

    [[ "${UPDATE_CHANNEL}" == "stable" ]]
    [[ "${LATEST_RELEASE}" == "v5.1.30" ]]
  )
}

test_install_pulse_archive_rejects_mismatched_arch_without_replacing_existing_binary() {
  (
    load_installer

    local tmpdir archive_root archive_path
    tmpdir="$(mktemp -d)"
    archive_root="${tmpdir}/archive"
    archive_path="${tmpdir}/pulse-v5.1.99-linux-arm64.tar.gz"

    mkdir -p "${archive_root}/bin"
    cat > "${archive_root}/bin/pulse" <<'EOF'
#!/usr/bin/env bash
echo "v5.1.99"
EOF
    chmod +x "${archive_root}/bin/pulse"
    tar -czf "${archive_path}" -C "${archive_root}" .

    INSTALL_DIR="${tmpdir}/opt/pulse"
    mkdir -p "${INSTALL_DIR}/bin"
    cat > "${INSTALL_DIR}/bin/pulse" <<'EOF'
#!/usr/bin/env bash
echo "v5.1.10"
EOF
    chmod +x "${INSTALL_DIR}/bin/pulse"

    validate_pulse_binary_architecture() { return 1; }

    if install_pulse_archive "${archive_path}" "v5.1.99"; then
      echo "install_pulse_archive unexpectedly succeeded on mismatched archive" >&2
      rm -rf "${tmpdir}"
      return 1
    fi

    [[ "$("${INSTALL_DIR}/bin/pulse" --version)" == "v5.1.10" ]]
    [[ ! -e "${INSTALL_DIR}/bin/pulse.old" ]]
    rm -rf "${tmpdir}"
  )
}

test_parse_args_rejects_archive_with_source() {
  local tmpdir output_file
  tmpdir="$(mktemp -d)"
  output_file="${tmpdir}/output.txt"

  if bash "${INSTALL_SCRIPT}" --source --archive /tmp/pulse.tar.gz >"${output_file}" 2>&1; then
    echo "installer unexpectedly accepted --archive with --source" >&2
    rm -rf "${tmpdir}"
    return 1
  fi

  if ! grep -q -- "--archive cannot be used with --source" "${output_file}"; then
    echo "expected archive/source validation message" >&2
    cat "${output_file}" >&2
    rm -rf "${tmpdir}"
    return 1
  fi

  rm -rf "${tmpdir}"
  return 0
}

test_installer_runs_when_streamed_over_stdin() {
  local tmpdir output_file
  tmpdir="$(mktemp -d)"
  output_file="${tmpdir}/output.txt"

  if ! cat "${INSTALL_SCRIPT}" | bash -s -- --help >"${output_file}" 2>&1; then
    echo "installer failed when streamed to bash" >&2
    cat "${output_file}" >&2
    rm -rf "${tmpdir}"
    return 1
  fi

  if grep -q "BASH_SOURCE\\[0\\]: unbound variable" "${output_file}"; then
    echo "installer still hit BASH_SOURCE unbound variable when streamed to bash" >&2
    cat "${output_file}" >&2
    rm -rf "${tmpdir}"
    return 1
  fi

  if ! grep -q "Usage: install.sh \\[OPTIONS\\]" "${output_file}"; then
    echo "expected streamed installer help to show install.sh usage" >&2
    cat "${output_file}" >&2
    rm -rf "${tmpdir}"
    return 1
  fi

  rm -rf "${tmpdir}"
  return 0
}

test_install_additional_agent_binaries_copies_local_binaries_without_network() {
  (
    load_installer

    local tmpdir source_dir
    tmpdir="$(mktemp -d)"
    source_dir="${tmpdir}/source"
    INSTALL_DIR="${tmpdir}/opt/pulse"

    mkdir -p "${source_dir}/bin" "${INSTALL_DIR}/bin"
    printf 'host-agent\n' > "${source_dir}/bin/pulse-host-agent-linux-arm64"
    printf 'unified-agent\n' > "${source_dir}/bin/pulse-agent-linux-arm64"
    chmod +x "${source_dir}/bin/pulse-host-agent-linux-arm64" "${source_dir}/bin/pulse-agent-linux-arm64"

    local curl_calls=0
    local wget_calls=0

    chown() { :; }
    curl() { curl_calls=$((curl_calls + 1)); return 99; }
    wget() { wget_calls=$((wget_calls + 1)); return 99; }

    install_additional_agent_binaries "v5.1.99" "${source_dir}"

    [[ -x "${INSTALL_DIR}/bin/pulse-host-agent-linux-arm64" ]]
    [[ -x "${INSTALL_DIR}/bin/pulse-agent-linux-arm64" ]]
    [[ "${curl_calls}" -eq 0 ]]
    [[ "${wget_calls}" -eq 0 ]]

    rm -rf "${tmpdir}"
  )
}

test_install_additional_agent_binaries_skips_network_when_local_extras_are_missing() {
  (
    load_installer

    local tmpdir source_dir
    tmpdir="$(mktemp -d)"
    source_dir="${tmpdir}/source"
    INSTALL_DIR="${tmpdir}/opt/pulse"

    mkdir -p "${source_dir}/bin" "${INSTALL_DIR}/bin"

    local curl_calls=0
    local wget_calls=0

    chown() { :; }
    curl() { curl_calls=$((curl_calls + 1)); return 99; }
    wget() { wget_calls=$((wget_calls + 1)); return 99; }

    install_additional_agent_binaries "v5.1.99" "${source_dir}"

    [[ "${curl_calls}" -eq 0 ]]
    [[ "${wget_calls}" -eq 0 ]]

    rm -rf "${tmpdir}"
  )
}

test_setup_update_command_renders_self_contained_update_helper() {
  (
    load_installer

    local tmpdir update_path profile_path bashrc_path fakebin curl_log bash_log real_bash
    tmpdir="$(mktemp -d)"
    update_path="${tmpdir}/update"
    profile_path="${tmpdir}/profile"
    bashrc_path="${tmpdir}/bashrc"
    fakebin="${tmpdir}/bin"
    curl_log="${tmpdir}/curl.log"
    bash_log="${tmpdir}/bash.log"
    real_bash="$(command -v bash)"

    mkdir -p "${fakebin}" "${tmpdir}/opt/pulse"
    : > "${profile_path}"
    : > "${bashrc_path}"

    GITHUB_REPO="example/pulse"
    SOURCE_BRANCH="release/5.1"
    INSTALL_DIR="${tmpdir}/opt/pulse"
    UPDATE_HELPER_PATH="${update_path}"
    PULSE_PROFILE_PATH="${profile_path}"
    PULSE_BASHRC_PATH="${bashrc_path}"

    setup_update_command

    if [[ ! -x "${update_path}" ]]; then
      echo "update helper was not created executable" >&2
      rm -rf "${tmpdir}"
      return 1
    fi
    if grep -Fq 'maintenance_raw_url' "${update_path}"; then
      echo "update helper still depends on installer-only maintenance_raw_url" >&2
      cat "${update_path}" >&2
      rm -rf "${tmpdir}"
      return 1
    fi
    if ! grep -Fq 'set -euo pipefail' "${update_path}"; then
      echo "update helper does not fail closed for curl pipe failures" >&2
      cat "${update_path}" >&2
      rm -rf "${tmpdir}"
      return 1
    fi
    if ! grep -Fq 'INSTALLER_URL=https://raw.githubusercontent.com/example/pulse/release/5.1/install.sh' "${update_path}"; then
      echo "update helper did not render the maintenance installer URL" >&2
      cat "${update_path}" >&2
      rm -rf "${tmpdir}"
      return 1
    fi

    printf 'feature-branch\n' > "${INSTALL_DIR}/BUILD_FROM_SOURCE"

    cat > "${fakebin}/curl" <<EOF
#!${real_bash}
printf '%s\n' "\$*" > "\$PULSE_TEST_CURL_LOG"
printf 'echo installer ok\n'
EOF
    cat > "${fakebin}/bash" <<EOF
#!${real_bash}
cat >/dev/null
printf '%s\n' "\$*" > "\$PULSE_TEST_BASH_LOG"
EOF
    chmod +x "${fakebin}/curl" "${fakebin}/bash"

    PULSE_TEST_CURL_LOG="${curl_log}" \
      PULSE_TEST_BASH_LOG="${bash_log}" \
      PATH="${fakebin}:${PATH}" \
      "${real_bash}" "${update_path}" >/dev/null

    if ! grep -Fq 'https://raw.githubusercontent.com/example/pulse/release/5.1/install.sh' "${curl_log}"; then
      echo "update helper did not call curl with the maintenance installer URL" >&2
      cat "${curl_log}" >&2
      rm -rf "${tmpdir}"
      return 1
    fi
    if ! grep -Fq -- '-s -- --source feature-branch' "${bash_log}"; then
      echo "update helper did not preserve source-build update arguments" >&2
      cat "${bash_log}" >&2
      rm -rf "${tmpdir}"
      return 1
    fi

    rm -rf "${tmpdir}"
  )
}

main() {
  assert_success "infer_release_from_archive_name parses prerelease tarballs" test_infer_release_from_archive_name_supports_prerelease
  assert_success "update disk preflight fails on shared low-space filesystems" test_ensure_update_disk_headroom_fails_when_tmp_and_install_share_full_filesystem
  assert_success "update disk preflight passes on separate filesystems with enough headroom" test_ensure_update_disk_headroom_accepts_separate_filesystems_with_sufficient_space
  assert_success "download_pulse installs from local archive without network" test_download_pulse_installs_from_local_archive_without_network
  assert_success "prefetch helper writes archive path via output variable" test_prefetch_pulse_archive_for_container_sets_output_var
  assert_success "resolve_target_release ignores host-configured rc during fresh LXC bootstrap" test_resolve_target_release_ignores_host_configured_rc_channel_when_requested
  assert_success "resolve_target_release skips prereleases without jq" test_resolve_target_release_skips_prerelease_without_jq
  assert_success "wrong-arch archives fail before replacing the installed binary" test_install_pulse_archive_rejects_mismatched_arch_without_replacing_existing_binary
  assert_success "parse_args rejects archive with source builds" test_parse_args_rejects_archive_with_source
  assert_success "installer supports curl-pipe execution via bash stdin" test_installer_runs_when_streamed_over_stdin
  assert_success "install_additional_agent_binaries copies local extras without network" test_install_additional_agent_binaries_copies_local_binaries_without_network
  assert_success "install_additional_agent_binaries skips network when extras are missing" test_install_additional_agent_binaries_skips_network_when_local_extras_are_missing
  assert_success "setup_update_command renders a self-contained update helper" test_setup_update_command_renders_self_contained_update_helper

  if (( failures > 0 )); then
    echo "Total failures: ${failures}" >&2
    return 1
  fi

  echo "All server installer smoke tests passed."
}

main "$@"
