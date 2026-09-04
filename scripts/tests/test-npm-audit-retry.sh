#!/usr/bin/env bash
#
# Smoke tests for scripts/npm-audit-retry.sh — the gate must stay exactly as
# strict about advisories and only tolerate an unreachable endpoint.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="${ROOT_DIR}/scripts/npm-audit-retry.sh"

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

failures=0

# Build a fake npm that emits a canned payload per invocation. Each line of
# the mode list is used for one successive call, so retry behaviour is
# observable.
make_fake_npm() {
  local name="$1"
  shift
  local path="${WORK_DIR}/${name}"
  {
    printf '#!/usr/bin/env bash\n'
    printf 'count_file="%s/${0##*/}.count"\n' "${WORK_DIR}"
    printf 'n=$(cat "$count_file" 2>/dev/null || echo 0)\n'
    printf 'n=$((n + 1))\n'
    printf 'printf "%%s" "$n" > "$count_file"\n'
    printf 'case "$n" in\n'
    local i=1
    for payload in "$@"; do
      printf '  %d) cat <<'"'"'JSON'"'"'\n%s\nJSON\n  ;;\n' "${i}" "${payload}"
      i=$((i + 1))
    done
    printf '  *) cat <<'"'"'JSON'"'"'\n%s\nJSON\n  ;;\n' "${!#}"
    printf 'esac\n'
    printf 'exit 1\n'
  } > "${path}"
  chmod +x "${path}"
  printf '%s' "${path}"
}

CLEAN='{"metadata":{"vulnerabilities":{"info":0,"low":0,"moderate":0,"high":0,"critical":0,"total":0}}}'
VULN='{"metadata":{"vulnerabilities":{"info":0,"low":0,"moderate":1,"high":0,"critical":0,"total":1}}}'
ENOAUDIT='{"error":{"code":"ENOAUDIT","summary":"503 Service Unavailable","detail":""}}'

run_case() {
  local desc="$1" expected_status="$2" npm_bin="$3" require="$4"
  shift 4
  local out status
  set +e
  out="$(NPM_AUDIT_CMD="${npm_bin}" \
         NPM_AUDIT_RETRY_DELAY=0 \
         NPM_AUDIT_ATTEMPTS="${NPM_AUDIT_ATTEMPTS:-3}" \
         NPM_AUDIT_REQUIRE_RESULT="${require}" \
         bash "${SCRIPT}" all 2>&1)"
  status=$?
  set -e
  if [ "${status}" != "${expected_status}" ]; then
    echo "FAIL: ${desc} — exit ${status}, want ${expected_status}"
    printf '%s\n' "${out}" | sed 's/^/    /'
    failures=$((failures + 1))
    return
  fi
  for needle in "$@"; do
    if ! printf '%s' "${out}" | grep -qF -- "${needle}"; then
      echo "FAIL: ${desc} — output missing: ${needle}"
      printf '%s\n' "${out}" | sed 's/^/    /'
      failures=$((failures + 1))
      return
    fi
  done
  echo "ok: ${desc}"
}

# A clean audit passes on the first attempt.
run_case "clean audit passes" 0 "$(make_fake_npm npm-clean "${CLEAN}")" true \
  "no vulnerabilities"

# A vulnerability fails, and must fail even when the dependency graph is
# untouched — the outage tolerance must never soften a real finding.
run_case "vulnerability fails" 1 "$(make_fake_npm npm-vuln "${VULN}")" false \
  "vulnerabilities present"

# A transient endpoint failure that clears on retry passes.
run_case "retry recovers from a transient outage" 0 \
  "$(make_fake_npm npm-recover "${ENOAUDIT}" "${CLEAN}")" true \
  "did not return a usable result" "no vulnerabilities"

# A vulnerability found only after a transient failure still fails.
run_case "retry surfacing a vulnerability fails" 1 \
  "$(make_fake_npm npm-late-vuln "${ENOAUDIT}" "${VULN}")" true \
  "vulnerabilities present"

# A sustained outage fails when this change touches the dependency graph.
run_case "sustained outage fails when dependencies changed" 1 \
  "$(make_fake_npm npm-out-required "${ENOAUDIT}")" true \
  "could not reach the advisory endpoint"

# A sustained outage warns but passes when the dependency graph is unchanged.
run_case "sustained outage warns when dependencies unchanged" 0 \
  "$(make_fake_npm npm-out-optional "${ENOAUDIT}")" false \
  "::warning::" "does not touch package.json"

# Unparseable output is treated as unreachable, never as a pass.
run_case "garbage output is not treated as clean" 1 \
  "$(make_fake_npm npm-garbage "not json at all")" true \
  "could not reach the advisory endpoint"

# An empty report is likewise not a pass.
run_case "empty output is not treated as clean" 1 \
  "$(make_fake_npm npm-empty "")" true \
  "could not reach the advisory endpoint"

# An unknown payload shape must not pass silently.
run_case "unknown payload shape is not treated as clean" 1 \
  "$(make_fake_npm npm-shape '{"metadata":{}}')" true \
  "could not reach the advisory endpoint"

if [ "${failures}" -ne 0 ]; then
  echo "${failures} test(s) failed"
  exit 1
fi
echo "all npm-audit-retry tests passed"
