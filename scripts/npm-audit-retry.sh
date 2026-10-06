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
# Any vulnerability at any severity is still reported, no severity threshold
# exists, and suppression is never a valid closure. What depends on the change
# is only whether the verdict blocks it:
#
#   * a conclusive answer is acted on immediately;
#   * an unreachable endpoint is retried with backoff;
#   * a change that moves the dependency graph (NPM_AUDIT_REQUIRE_RESULT=true,
#     or any value other than exactly "false") fails on any finding and on an
#     endpoint that never answers;
#   * a change that leaves the graph identical to its base
#     (NPM_AUDIT_REQUIRE_RESULT=false) warns on both instead, naming every
#     finding, because any finding it sees is one the base commit already has.
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
# That split is the whole safety argument. When package.json and
# package-lock.json are untouched, the audit answer for this change is the one
# the base commit would produce, so this change can neither add a finding nor
# remove one. Failing it on an advisory the base already has blocks unrelated
# work without making anything safer: on 2026-10-03 GHSA-vfj7-8cjw-p6xm
# (braces, reached only through dev-only tailwindcss and jscpd, with no patched
# release) failed the required Frontend check on every open pull request,
# Go-only ones included. Advisories against unchanged dependencies are owned by
# the jobs that audit the graph as it stands: the scheduled npm-audit scan and
# the dependency advisory watch, which keep the strict default and fail, and
# Dependabot security updates. When the dependency graph does move, the
# change is answerable for the result, so any finding and any missing answer
# block it.
#
# Env:
#   NPM_AUDIT_ATTEMPTS        attempts before giving up (default 3)
#   NPM_AUDIT_RETRY_DELAY     seconds before the first retry, doubled each
#                             time (default 15)
#   NPM_AUDIT_ATTEMPT_TIMEOUT seconds one npm invocation may run (default 60)
#   NPM_AUDIT_MAX_SECONDS     total wall-clock budget for all attempts
#                             (default 240)
#   NPM_AUDIT_REQUIRE_RESULT  "false" only when this change leaves
#                             package.json and package-lock.json identical to
#                             its base; findings and a missing answer then
#                             warn. Any other value fails on both (default
#                             true — the safe default)
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
DELAY="${NPM_AUDIT_RETRY_DELAY:-15}"
ATTEMPT_TIMEOUT="${NPM_AUDIT_ATTEMPT_TIMEOUT:-60}"
MAX_SECONDS="${NPM_AUDIT_MAX_SECONDS:-240}"
REQUIRE_RESULT="${NPM_AUDIT_REQUIRE_RESULT:-true}"
NPM_BIN="${NPM_AUDIT_CMD:-npm}"

# Only an explicit "false" may relax anything. An empty or misspelled value,
# for instance from a missing workflow output, keeps the strict verdict.
graph_unchanged=false
if [ "${REQUIRE_RESULT}" = "false" ]; then
  graph_unchanged=true
fi

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
  "${NPM_BIN}" audit --json "${AUDIT_ARGS[@]}" "${SCOPE_ARGS[@]}" >"${out}" 2>/dev/null &
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

  # Keep the observed command status. Even a complete-looking zero summary
  # cannot establish a clean audit when npm failed or was stopped afterwards.
  # Positive findings in that same response still take precedence.
  return "${status}"
}

# Classify one audit run. Prints a verdict word, summary and (for a finding)
# allowlisted, JSON-escaped package/advisory details from that same response:
#   clean          — audit exited zero, complete report, no vulnerabilities
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
findings = report.get("vulnerabilities") if isinstance(report, dict) else None
count_names = ("total", "critical", "high", "moderate", "low", "info")
# Counts are numbers, not arbitrary registry strings. In particular, Python
# bools are ints too, but neither false nor null is evidence of zero findings.
counts = {}
if isinstance(vulns, dict):
    for name in count_names:
        value = vulns.get(name)
        if type(value) is int and value >= 0:
            counts[name] = value
has_findings = isinstance(findings, dict) and bool(findings)
if has_findings or any(count > 0 for count in counts.values()):
    # Any positive package or severity evidence wins, even if the summary is
    # missing/inconsistent or npm also reports a transport error. Retrying must
    # never replace an already observed finding with a later clean response.
    detail = " ".join(
        "{}={}".format(name, counts.get(name, "unknown")) for name in count_names
    )
    if has_findings:
        detail += f" package_records={len(findings)}"
    print("vulnerable")
    print(detail)
    # Do not query npm again for human-readable detail. A second request
    # can hang outside the watchdog or return different advisory evidence.
    # JSON encoding keeps registry text from becoming terminal escapes or
    # GitHub workflow commands; transport errors and unknown fields stay out.
    emitted = False
    if isinstance(findings, dict):
        for name, finding in sorted(findings.items()):
            if not isinstance(finding, dict):
                continue
            projected = {"name": name}
            for key in ("name", "severity", "isDirect", "range", "nodes"):
                if key in finding:
                    projected[key] = finding[key]
            via = finding.get("via")
            if isinstance(via, list):
                projected["via"] = [
                    {key: advisory[key] for key in
                     ("source", "name", "dependency", "title", "url", "severity", "range")
                     if key in advisory}
                    if isinstance(advisory, dict) else advisory
                    for advisory in via if isinstance(advisory, (dict, str))
                ]
            fix = finding.get("fixAvailable")
            if isinstance(fix, bool):
                projected["fixAvailable"] = fix
            elif isinstance(fix, dict):
                projected["fixAvailable"] = {
                    key: fix[key] for key in ("name", "version", "isSemVerMajor")
                    if key in fix
                }
            print("audit finding " + json.dumps(projected, ensure_ascii=True, sort_keys=True))
            emitted = True
    if not emitted:
        print("npm audit: package-level detail unavailable in captured verdict")
    sys.exit(0)

if (
    sys.argv[1] == "0"
    and isinstance(report, dict) and not report.get("error")
    and len(counts) == len(count_names) and all(count == 0 for count in counts.values())
    and ("vulnerabilities" not in report or isinstance(findings, dict))
):
    print("clean")
    print(" ".join(f"{name}=0" for name in count_names))
else:
    # A failed/stopped command, partial zero summary or endpoint error is not
    # a clean verdict, even if npm wrote zero counts before it ended.
    # With no positive finding, retain the existing bounded outage policy.
    print("unreachable")

' "$1"
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
  audit_status=0
  run_audit "${attempt_limit}" "${report_file}" || audit_status=$?
  case "${audit_status}" in
    0) ;;
    143|137)
      echo "npm audit (${SCOPE}): command was stopped (status ${audit_status}, attempt limit ${attempt_limit}s)"
      ;;
    *)
      echo "npm audit (${SCOPE}): command exited with status ${audit_status}"
      ;;
  esac
  verdict_output="$(classify_report "${audit_status}" <"${report_file}")"
  verdict="$(printf '%s\n' "${verdict_output}" | head -1)"
  summary="$(printf '%s\n' "${verdict_output}" | sed -n '2p')"

  case "${verdict}" in
    clean)
      echo "npm audit (${SCOPE}): no vulnerabilities (${summary})"
      exit 0
      ;;
    vulnerable)
      echo "npm audit (${SCOPE}): vulnerabilities present (${summary})"
      # Keep diagnostics bound to the conclusive response, with no extra
      # registry request outside the attempt/total wall-clock limits.
      printf '%s\n' "${verdict_output}" | tail -n +3
      if [ "${graph_unchanged}" = "true" ]; then
        echo "::warning::npm audit (${SCOPE}) found vulnerabilities the base commit already has: ${summary}. This change does not touch package.json or package-lock.json, so it neither introduced them nor can remove them; the scheduled audit and the dependency advisory watch keep failing until the dependency graph is fixed."
        exit 0
      fi
      echo "::error::npm audit (${SCOPE}) found vulnerabilities: ${summary}"
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

if [ "${graph_unchanged}" != "true" ]; then
  echo "::error::npm audit (${SCOPE}) could not reach the advisory endpoint ${gave_up}, and this change touches the dependency graph, so the result cannot be assumed."
  exit 1
fi

echo "::warning::npm audit (${SCOPE}) could not reach the advisory endpoint ${gave_up}. This change does not touch package.json or package-lock.json, so the dependency graph is identical to its base commit; continuing without a fresh result."
exit 0
