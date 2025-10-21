#!/bin/bash
# Heuristic router for selecting a Codex reasoning tier.
# Usage: codex-router.sh "Fix the typo" or echo "..." | codex-router.sh

set -euo pipefail

WORKDIR="/opt/pulse"

# Collect a prompt from stdin and/or arguments.
PROMPT_INPUT=""
if [[ ! -t 0 ]]; then
  PROMPT_INPUT="$(cat)"
fi

if [[ $# -gt 0 ]]; then
  if [[ -n "${PROMPT_INPUT}" ]]; then
    PROMPT="${PROMPT_INPUT}"$'\n'"$*"
  else
    PROMPT="$*"
  fi
else
  PROMPT="${PROMPT_INPUT}"
fi

PROMPT="$(printf '%s' "${PROMPT}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"

if [[ -z "${PROMPT}" ]]; then
  echo "Usage: $0 <prompt>" >&2
  exit 1
fi

reasoning_profile="low"

# Escalate when the task hints at analysis, design, or larger edits.
if [[ ${#PROMPT} -gt 400 ]] \
  || grep -qiE '(analysis|investigate|explain|design|architecture|strategy|refactor|spec|diagnos|trade[- ]?off|postmortem)' <<<"${PROMPT}"; then
  reasoning_profile="medium"
fi

case "${reasoning_profile}" in
  medium)
    profile_arg=(--profile medium)
    ;;
  *)
    profile_arg=(--profile low)
    ;;
esac

echo "Routing prompt to Codex profile '${reasoning_profile}'" >&2
exec codex exec "${profile_arg[@]}" -C "${WORKDIR}" "${PROMPT}"
