#!/usr/bin/env bash
# npm-audit-retry.sh — Run npm audit, separating a real advisory from an
# unreachable advisory endpoint.
#
# Usage: scripts/npm-audit-retry.sh <all|production> [npm-audit-args...]
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
#   NPM_AUDIT_FETCH_TIMEOUT_MS per-attempt npm fetch timeout (default 60000)
#   NPM_AUDIT_RETRY_DELAY     seconds before the first retry, doubled each
#                             time (default 15)
#   NPM_AUDIT_REQUIRE_RESULT  "true" to fail when no answer was obtained
#                             (default true — the safe default)
#   NPM_AUDIT_CMD             npm executable to invoke (test seam)

set -uo pipefail

SCOPE="${1:-}"
case "${SCOPE}" in
  all)        SCOPE_ARGS=() ;;
  production) SCOPE_ARGS=(--omit=dev) ;;
  *)
    echo "Usage: $0 <all|production> [npm-audit-args...]" >&2
    exit 2
    ;;
esac
shift
AUDIT_ARGS=("$@")

ATTEMPTS="${NPM_AUDIT_ATTEMPTS:-3}"
FETCH_TIMEOUT_MS="${NPM_AUDIT_FETCH_TIMEOUT_MS:-60000}"
DELAY="${NPM_AUDIT_RETRY_DELAY:-15}"
REQUIRE_RESULT="${NPM_AUDIT_REQUIRE_RESULT:-true}"
NPM_BIN="${NPM_AUDIT_CMD:-npm}"

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

meta = report.get("metadata") if isinstance(report, dict) else None
vulns = meta.get("vulnerabilities") if isinstance(meta, dict) else None
if isinstance(vulns, dict) and "total" in vulns:
    # A usable advisory verdict takes precedence even if npm also includes a
    # transport error. Never turn a real finding into a retryable outage.
    total = vulns.get("total", 0)
    detail = " ".join(
        f"{name}={vulns.get(name, 0)}"
        for name in ("critical", "high", "moderate", "low", "info")
    )
    print("vulnerable" if total else "clean")
    print(f"total={total} {detail}")
    sys.exit(0)

if isinstance(report, dict) and report.get("error"):
    # npm reports an unusable endpoint as an error object, ENOAUDIT being the
    # code it uses for 5xx, timeouts and offline runs alike.
    print("unreachable")
    sys.exit(0)

if not isinstance(vulns, dict) or "total" not in vulns:
    # No usable verdict in the payload: treat as unreachable rather than
    # silently passing on a shape we do not understand.
    print("unreachable")
    sys.exit(0)

'
}

report_file="$(mktemp)"
trap 'rm -f "${report_file}"' EXIT

attempt=1
delay="${DELAY}"
while [ "${attempt}" -le "${ATTEMPTS}" ]; do
  echo "npm audit (${SCOPE}) attempt ${attempt}/${ATTEMPTS}"
  "${NPM_BIN}" audit --json --fetch-timeout="${FETCH_TIMEOUT_MS}" "${AUDIT_ARGS[@]}" "${SCOPE_ARGS[@]}" >"${report_file}" 2>/dev/null
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
      "${NPM_BIN}" audit --fetch-timeout="${FETCH_TIMEOUT_MS}" "${AUDIT_ARGS[@]}" "${SCOPE_ARGS[@]}" || true
      exit 1
      ;;
    *)
      echo "npm audit (${SCOPE}): advisory endpoint did not return a usable result"
      ;;
  esac

  if [ "${attempt}" -lt "${ATTEMPTS}" ]; then
    echo "retrying in ${delay}s"
    sleep "${delay}"
    delay=$((delay * 2))
  fi
  attempt=$((attempt + 1))
done

if [ "${REQUIRE_RESULT}" = "true" ]; then
  echo "::error::npm audit (${SCOPE}) could not reach the advisory endpoint after ${ATTEMPTS} attempts, and this change touches the dependency graph, so the result cannot be assumed."
  exit 1
fi

echo "::warning::npm audit (${SCOPE}) could not reach the advisory endpoint after ${ATTEMPTS} attempts. This change does not touch package.json or package-lock.json, so the dependency graph is identical to the base commit that already passed; continuing without a fresh result."
exit 0
