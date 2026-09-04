#!/usr/bin/env bash
# npm-audit-retry.sh — Run npm audit, separating a real advisory from an
# unreachable advisory endpoint.
#
# Usage: scripts/npm-audit-retry.sh <all|production>
#
# `npm audit` exits 1 both when it finds vulnerabilities and when it cannot
# reach registry.npmjs.org. Treating those the same made a required check
# depend on npm's availability: on 2026-09-03 the advisory bulk endpoint
# returned 503s and timeouts for over an hour and no pull request could land,
# including Go-only ones. Four consecutive failures, zero advisories.
#
# This keeps the gate exactly as strict about advisories — any vulnerability at
# any severity still fails, and suppression is never a valid closure — and
# changes only what happens when npm cannot answer:
#
#   * a conclusive answer is acted on immediately, pass or fail;
#   * an unreachable endpoint is retried with backoff;
#   * if it is still unreachable after every attempt, the run fails when this
#     change touches the dependency graph (NPM_AUDIT_REQUIRE_RESULT=true) and
#     warns without failing when it does not.
#
# The retry budget is wall-clock, not just an attempt count, because attempt
# count alone does not bound anything: npm's own `fetch-timeout` defaults to
# five minutes and it retries internally, so a single `npm audit` against a
# hanging endpoint can sit for minutes before this script sees a verdict. On
# 2026-09-04 that produced a 10m56s audit step (two 5m00s attempts, then a
# 9s success) and cancelled the Frontend job at its 25m limit with every test
# already passing — a green run reported as a failed required check. So each
# attempt is bounded, npm's internal retry loop is disabled in favour of this
# one, and the whole sequence stops at a deadline.
#
# That last split is the whole safety argument. When package.json and
# package-lock.json are untouched, the audit answer for this change is the one
# the base commit already produced, so skipping it adds no risk from this
# change; advisories published later against unchanged dependencies are caught
# by Dependabot security updates, not by a per-pull-request audit. When the
# dependency graph does move, the answer is unknown and only then does an
# unreachable endpoint have to block.
#
# Env:
#   NPM_AUDIT_ATTEMPTS        attempts before giving up (default 3)
#   NPM_AUDIT_RETRY_DELAY     seconds before the first retry, doubled each
#                             time (default 15)
#   NPM_AUDIT_ATTEMPT_TIMEOUT seconds one npm invocation may run (default 60)
#   NPM_AUDIT_MAX_SECONDS     total wall-clock budget for all attempts
#                             (default 240)
#   NPM_AUDIT_REQUIRE_RESULT  "true" to fail when no answer was obtained
#                             (default true — the safe default)
#   NPM_AUDIT_CMD             npm executable to invoke (test seam)

set -uo pipefail

SCOPE="${1:-}"
case "${SCOPE}" in
  all)        SCOPE_ARGS=() ;;
  production) SCOPE_ARGS=(--omit=dev) ;;
  *)
    echo "Usage: $0 <all|production>" >&2
    exit 2
    ;;
esac

ATTEMPTS="${NPM_AUDIT_ATTEMPTS:-3}"
DELAY="${NPM_AUDIT_RETRY_DELAY:-15}"
ATTEMPT_TIMEOUT="${NPM_AUDIT_ATTEMPT_TIMEOUT:-60}"
MAX_SECONDS="${NPM_AUDIT_MAX_SECONDS:-240}"
REQUIRE_RESULT="${NPM_AUDIT_REQUIRE_RESULT:-true}"
NPM_BIN="${NPM_AUDIT_CMD:-npm}"

# This script is the retry layer. npm's own fetch retry loop would multiply
# every attempt by an unbounded amount of hidden waiting, which is exactly
# what made a bounded-looking three attempts run for eleven minutes.
export npm_config_fetch_retries=0
export npm_config_fetch_timeout=$((ATTEMPT_TIMEOUT * 1000))

DEADLINE=$(( $(date +%s) + MAX_SECONDS ))

# Run one audit under a hard wall-clock bound, portably: `timeout` is not
# present on every developer machine, so a watchdog subshell kills the npm
# process if it outlives the limit. Blocking on `wait` for the real child
# avoids the zombie-liveness race that a `kill -0` poll would hit.
run_audit() {
  local limit="$1" out="$2"

  : >"${out}"
  "${NPM_BIN}" audit --json "${SCOPE_ARGS[@]}" >"${out}" 2>/dev/null &
  local npm_pid=$!

  (
    sleep "${limit}"
    kill -TERM "${npm_pid}" 2>/dev/null
    sleep 2
    kill -KILL "${npm_pid}" 2>/dev/null
  ) >/dev/null 2>&1 &
  local killer_pid=$!

  wait "${npm_pid}" 2>/dev/null
  local status=$?

  kill -TERM "${killer_pid}" 2>/dev/null
  wait "${killer_pid}" 2>/dev/null

  # 143 = SIGTERM, 137 = SIGKILL: the watchdog fired. The report is then
  # empty or truncated, which classify_report already reads as unreachable.
  if [ "${status}" -eq 143 ] || [ "${status}" -eq 137 ]; then
    return 124
  fi
  return 0
}

# Classify one audit run. Prints a verdict word on stdout:
#   clean          — audit completed, no vulnerabilities
#   vulnerable     — audit completed, vulnerabilities present
#   unreachable    — npm could not get an answer from the advisory endpoint
classify_report() {
  python3 -c '
import json, sys

raw = sys.stdin.read().strip()
if not raw:
    print("unreachable")
    sys.exit(0)
try:
    report = json.loads(raw)
except ValueError:
    print("unreachable")
    sys.exit(0)

# npm reports an unusable audit endpoint as an error object, ENOAUDIT being
# the code it uses for 5xx, timeouts and offline runs alike.
if isinstance(report, dict) and report.get("error"):
    print("unreachable")
    sys.exit(0)

meta = report.get("metadata") if isinstance(report, dict) else None
vulns = meta.get("vulnerabilities") if isinstance(meta, dict) else None
if not isinstance(vulns, dict) or "total" not in vulns:
    # No usable verdict in the payload: treat as unreachable rather than
    # silently passing on a shape we do not understand.
    print("unreachable")
    sys.exit(0)

total = vulns.get("total", 0)
detail = " ".join(
    f"{name}={vulns.get(name, 0)}"
    for name in ("critical", "high", "moderate", "low", "info")
)
print("vulnerable" if total else "clean")
print(f"total={total} {detail}")
'
}

report_file="$(mktemp)"
trap 'rm -f "${report_file}"' EXIT

attempt=1
delay="${DELAY}"
budget_exhausted=false
while [ "${attempt}" -le "${ATTEMPTS}" ]; do
  remaining=$(( DEADLINE - $(date +%s) ))
  if [ "${remaining}" -le 0 ]; then
    echo "npm audit (${SCOPE}): ${MAX_SECONDS}s retry budget exhausted before attempt ${attempt}"
    budget_exhausted=true
    break
  fi

  # Never let one attempt outlive the overall budget.
  attempt_limit="${ATTEMPT_TIMEOUT}"
  if [ "${attempt_limit}" -gt "${remaining}" ]; then
    attempt_limit="${remaining}"
  fi

  echo "npm audit (${SCOPE}) attempt ${attempt}/${ATTEMPTS} (limit ${attempt_limit}s, ${remaining}s of budget left)"
  if ! run_audit "${attempt_limit}" "${report_file}"; then
    echo "npm audit (${SCOPE}): attempt ${attempt} exceeded ${attempt_limit}s and was stopped"
  fi
  verdict_output="$(classify_report <"${report_file}")"
  verdict="$(printf '%s\n' "${verdict_output}" | head -1)"
  summary="$(printf '%s\n' "${verdict_output}" | sed -n '2p')"

  case "${verdict}" in
    clean)
      echo "npm audit (${SCOPE}): no vulnerabilities (${summary})"
      exit 0
      ;;
    vulnerable)
      echo "npm audit (${SCOPE}): vulnerabilities present (${summary})"
      echo "::error::npm audit (${SCOPE}) found vulnerabilities: ${summary}"
      # Re-run without --json so the log carries the human-readable advisory
      # detail a maintainer needs to act on.
      "${NPM_BIN}" audit "${SCOPE_ARGS[@]}" || true
      exit 1
      ;;
    *)
      echo "npm audit (${SCOPE}): advisory endpoint did not return a usable result"
      ;;
  esac

  if [ "${attempt}" -lt "${ATTEMPTS}" ]; then
    remaining=$(( DEADLINE - $(date +%s) ))
    if [ "${delay}" -ge "${remaining}" ]; then
      echo "npm audit (${SCOPE}): ${MAX_SECONDS}s retry budget exhausted"
      budget_exhausted=true
      break
    fi
    echo "retrying in ${delay}s"
    sleep "${delay}"
    delay=$((delay * 2))
  fi
  attempt=$((attempt + 1))
done

if [ "${budget_exhausted}" = "true" ]; then
  gave_up="within its ${MAX_SECONDS}s retry budget"
else
  gave_up="after ${ATTEMPTS} attempts"
fi

if [ "${REQUIRE_RESULT}" = "true" ]; then
  echo "::error::npm audit (${SCOPE}) could not reach the advisory endpoint ${gave_up}, and this change touches the dependency graph, so the result cannot be assumed."
  exit 1
fi

echo "::warning::npm audit (${SCOPE}) could not reach the advisory endpoint ${gave_up}. This change does not touch package.json or package-lock.json, so the dependency graph is identical to the base commit that already passed; continuing without a fresh result."
exit 0
