#!/usr/bin/env bash

# This file is sourced by run-secure-runtime-rootful-qualification.sh and kept
# separately so the fail-closed systemd readiness predicate can be exercised
# without entering the destructive qualification wrapper.

rootful_qualification_systemd_readiness() {
  local container_id="$1"
  local target_state failed_units

  target_state="$(docker exec "${container_id}" systemctl show \
    --property=ActiveState --value multi-user.target 2>/dev/null)" || return 1
  [[ "${target_state}" == "active" ]] || return 1

  failed_units="$(docker exec "${container_id}" systemctl list-units \
    --state=failed --no-legend --no-pager --plain 2>/dev/null)" || return 1
  if [[ -n "${failed_units//[[:space:]]/}" ]]; then
    printf 'ERROR: disposable systemd container has failed units:\n%s\n' "${failed_units}" >&2
    return 2
  fi
}
