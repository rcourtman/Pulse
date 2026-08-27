#!/usr/bin/env bash

# Generate release notes with an agent that explores the actual repo history.
#
# Engine: headless Claude Code (`claude -p`, uses your Claude subscription —
# no API key), with Codex CLI (`codex exec`, OpenAI subscription) as fallback.
# The agent runs read-only git/gh commands itself instead of being fed
# pre-chewed diff fragments, so nothing user-visible is missed by grep luck.
#
# Usage:  ./scripts/generate-release-notes.sh <version> [comparison-tag]
#         ./scripts/generate-release-notes.sh --resolve-base <version>
#
# Contract: the release notes markdown is written to STDOUT (trigger-release.sh
# captures it); all progress/diagnostics go to STDERR. SAVE_TO_FILE=1 also
# writes release-notes-v<version>.md.
#
# Env overrides:
#   RELEASE_NOTES_ENGINE=claude|codex   force an engine (default: claude, codex fallback)
#   RELEASE_NOTES_MODEL=<model>         model for the claude engine (default: sonnet)

set -euo pipefail

MODE=generate
if [ "${1:-}" = "--resolve-base" ]; then
    MODE=resolve-base
    shift
fi

VERSION=${1:-}
REQUESTED_COMPARISON_TAG=${2:-}

if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version> [comparison-tag]" >&2
    echo "       $0 --resolve-base <version>" >&2
    echo "Example: $0 6.4.0-rc.6" >&2
    exit 1
fi

cd "$(git rev-parse --show-toplevel)"

VERSION=${VERSION#v}

latest_stable_before() {
    local target_tag="v$1"
    local candidate

    while IFS= read -r candidate; do
        [ "$candidate" = "$target_tag" ] && continue
        if [ "$(printf '%s\n%s\n' "$candidate" "$target_tag" | sort -V | head -n 1)" = "$candidate" ]; then
            printf '%s\n' "$candidate"
        fi
    done < <(git tag --list 'v*' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V)
}

resolve_comparison_tag() {
    local version=$1
    local base rc expected

    if [[ "$version" =~ ^([0-9]+\.[0-9]+\.[0-9]+)-rc\.([0-9]+)$ ]]; then
        base=${BASH_REMATCH[1]}
        rc=${BASH_REMATCH[2]}
        if (( rc > 1 )); then
            expected="v${base}-rc.$((rc - 1))"
            if ! git merge-base --is-ancestor "$expected" HEAD 2>/dev/null; then
                echo "Expected immediately preceding RC tag '$expected' is not an ancestor of HEAD" >&2
                return 1
            fi
            printf '%s\n' "$expected"
            return
        fi
    elif [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "Unsupported release version '$version'; expected X.Y.Z or X.Y.Z-rc.N" >&2
        return 1
    fi

    latest_stable_before "${base:-$version}" | tail -n 1
}

EXPECTED_COMPARISON_TAG=$(resolve_comparison_tag "$VERSION")
if [ -z "$EXPECTED_COMPARISON_TAG" ]; then
    echo "No valid comparison tag found for v${VERSION}" >&2
    exit 1
fi

if [ -n "$REQUESTED_COMPARISON_TAG" ] && [ "$REQUESTED_COMPARISON_TAG" != "$EXPECTED_COMPARISON_TAG" ]; then
    echo "Comparison tag '$REQUESTED_COMPARISON_TAG' violates the release-note range for v${VERSION}; expected '$EXPECTED_COMPARISON_TAG'" >&2
    exit 1
fi

PREVIOUS_TAG=$EXPECTED_COMPARISON_TAG

if ! git rev-parse -q --verify "${PREVIOUS_TAG}^{commit}" >/dev/null; then
    echo "Comparison tag '${PREVIOUS_TAG}' does not exist" >&2
    exit 1
fi

if [ "$MODE" = "resolve-base" ]; then
    printf '%s\n' "$PREVIOUS_TAG"
    exit 0
fi

if [[ "$VERSION" == *-rc.* ]]; then
    RELEASE_RANGE_GUIDANCE="This is an RC release. Cover only the user-visible changes since the immediately preceding RC, ${PREVIOUS_TAG}. Do not repeat changes already announced in an earlier RC."
else
    RELEASE_RANGE_GUIDANCE="This is a stable GA release. Cover the complete release train since the previous stable release, ${PREVIOUS_TAG}. Synthesize that potentially large commit range into a few user-relevant themes; do not concatenate RC notes or enumerate every commit."
fi

echo "Generating release notes for v${VERSION} (changes since ${PREVIOUS_TAG})..." >&2

read -r -d '' PROMPT <<EOF || true
You are generating the release notes for Pulse v${VERSION}.

Pulse is a self-hosted monitoring dashboard for Proxmox VE, PBS, Docker, and
Kubernetes, used mostly by homelab and small-ops users.

The repo is checked out at the commit that will become v${VERSION}. The
previous release tag is ${PREVIOUS_TAG}. Investigate the changes yourself —
start with \`git log ${PREVIOUS_TAG}..HEAD --oneline\` and
\`git diff ${PREVIOUS_TAG}..HEAD --stat\`, then use targeted \`git diff\` /
\`git show\` and read source files where the diff alone is unclear. For
commits that reference GitHub issues (#1234), you may use \`gh issue view\` to
understand the user-facing symptom. Only describe changes that exist in the
final code state; verify anything you are unsure about before writing it.

${RELEASE_RANGE_GUIDANCE}

Focus on USER-VISIBLE changes only: features, fixes, and behavior users will
notice. Ignore internal refactors, test changes, CI/tooling, and docs. Group
related commits into the outcome a user would recognize, such as a rewritten
alert system, rather than listing the implementation steps separately.

Write the release notes in exactly this format:

# Pulse v${VERSION} Release Notes

[One short paragraph explaining the customer outcome of this release. Lead
with what feels better or works now, not how it was implemented.]

## What's improved

- **[Short outcome]** — [Where users notice it and why it matters.]

[Use 4-6 meaningful bullets for a normal RC or minor release. A narrow patch
may use fewer. Keep every bullet concrete and independently useful.]

## Before you upgrade

[Only user-relevant compatibility, migration, signing, companion-app, or known
risk information. Omit this section when there is nothing users need to do or
understand before upgrading.]

Guidelines:
- Plain, factual, understated. No marketing language, no emojis.
- Omit any section that has no items.
- Every bullet must stand on its own. A reader should understand where they
  would notice the change and what is different without knowing Pulse's
  implementation.
- Put features and fixes together under What's improved. Describe each
  user-visible outcome exactly once; do not repeat it under another heading or
  split one outcome into separate feature and fix bullets.
- Avoid internal release and architecture vocabulary such as canonical,
  governed, schema, provider transport, preflight, convergence, or runtime
  boundary unless that exact term is visible to the user in the product.
- Do not use vague entries such as "improved agent handling" or "various UI
  fixes". Split unrelated changes and name the behavior that changed.
- Do NOT write Install, Roll back, Promotion Metadata, Release Qualification,
  validation, gate, workflow, or governance sections. The release pipeline
  appends concise install and rollback instructions, while machine promotion
  records stay outside the customer changelog.

Your reply must be ONLY the release-notes markdown, starting with
"# Pulse v${VERSION} Release Notes" — no preamble, no code fences, no
commentary.
EOF

# Strip accidental markdown fences and anything before the release title.
clean_notes() {
    sed -e 's/^```[a-z]*$//' -e 's/^```$//' | \
        awk -v title="# Pulse v${VERSION} Release Notes" '$0 == title {found=1} found{print}'
}

# Both engines must run on the logged-in subscription (Claude Max / OpenAI
# plan), never on metered API keys. Stray env vars silently override
# subscription auth — scrub them before invoking either CLI.
scrub_env() {
    env -u ANTHROPIC_API_KEY -u ANTHROPIC_AUTH_TOKEN -u ANTHROPIC_BASE_URL \
        -u ANTHROPIC_PROFILE -u OPENAI_API_KEY "$@"
}

generate_with_claude() {
    command -v claude >/dev/null || return 1
    echo "Engine: claude (model: ${RELEASE_NOTES_MODEL:-sonnet})" >&2
    scrub_env claude -p "$PROMPT" \
        --model "${RELEASE_NOTES_MODEL:-sonnet}" \
        --allowedTools \
            "Bash(git log:*)" "Bash(git diff:*)" "Bash(git show:*)" \
            "Bash(git describe:*)" "Bash(git tag:*)" "Bash(git rev-parse:*)" \
            "Bash(gh issue view:*)" "Bash(gh pr view:*)" \
            "Read" "Grep" "Glob" \
        </dev/null
}

generate_with_codex() {
    command -v codex >/dev/null || return 1
    echo "Engine: codex" >&2
    local out
    out=$(mktemp)
    # Session log goes to stderr; only the agent's final message is kept.
    if ! scrub_env codex exec --sandbox read-only -o "$out" "$PROMPT" >&2 </dev/null; then
        rm -f "$out"
        return 1
    fi
    cat "$out"
    rm -f "$out"
}

RELEASE_NOTES=""
case "${RELEASE_NOTES_ENGINE:-claude}" in
    codex)
        RELEASE_NOTES=$(generate_with_codex) || {
            echo "Codex generation failed" >&2
            exit 1
        }
        ;;
    *)
        if ! RELEASE_NOTES=$(generate_with_claude); then
            echo "Claude generation failed, trying Codex fallback..." >&2
            RELEASE_NOTES=$(generate_with_codex) || {
                echo "Both engines failed (need 'claude' or 'codex' CLI, logged in)" >&2
                exit 1
            }
        fi
        ;;
esac

RELEASE_NOTES=$(printf '%s\n' "$RELEASE_NOTES" | clean_notes)

if [ -z "$RELEASE_NOTES" ]; then
    echo "Error: release notes generation returned no canonical release title" >&2
    exit 1
fi

# Notes to stdout only — callers capture this.
printf '%s\n' "$RELEASE_NOTES"

if [ "${SAVE_TO_FILE:-}" = "1" ]; then
    OUTPUT_FILE="release-notes-v${VERSION}.md"
    printf '%s\n' "$RELEASE_NOTES" > "$OUTPUT_FILE"
    echo "Saved to ${OUTPUT_FILE}" >&2
fi
