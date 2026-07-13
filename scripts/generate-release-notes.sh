#!/usr/bin/env bash

# Generate release notes with an agent that explores the actual repo history.
#
# Engine: headless Claude Code (`claude -p`, uses your Claude subscription —
# no API key), with Codex CLI (`codex exec`, OpenAI subscription) as fallback.
# The agent runs read-only git/gh commands itself instead of being fed
# pre-chewed diff fragments, so nothing user-visible is missed by grep luck.
#
# Usage:  ./scripts/generate-release-notes.sh <version> [previous-tag]
#
# Contract: the release notes markdown is written to STDOUT (trigger-release.sh
# captures it); all progress/diagnostics go to STDERR. SAVE_TO_FILE=1 also
# writes release-notes-v<version>.md.
#
# Env overrides:
#   RELEASE_NOTES_ENGINE=claude|codex   force an engine (default: claude, codex fallback)
#   RELEASE_NOTES_MODEL=<model>         model for the claude engine (default: sonnet)

set -euo pipefail

VERSION=${1:-}
PREVIOUS_TAG=${2:-}

if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version> [previous-tag]" >&2
    echo "Example: $0 6.1.0 v6.0.6" >&2
    exit 1
fi

cd "$(git rev-parse --show-toplevel)"

if [ -z "$PREVIOUS_TAG" ]; then
    PREVIOUS_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    if [ -z "$PREVIOUS_TAG" ]; then
        echo "No previous tag found, cannot generate diff-based release notes" >&2
        exit 1
    fi
fi

if ! git rev-parse -q --verify "${PREVIOUS_TAG}^{commit}" >/dev/null; then
    echo "Previous tag '${PREVIOUS_TAG}' does not exist" >&2
    exit 1
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

Focus on USER-VISIBLE changes only: features, fixes, and behavior users will
notice. Ignore internal refactors, test changes, CI/tooling, and docs.

Write the release notes in exactly this format:

## v${VERSION}

### Highlights
[2-4 short bullets, plain English, covering only what a typical user would
notice and care about. This section is rendered inside the Pulse UI itself
(the post-update "What's New" banner and the update-banner preview), so each
bullet must be self-contained and jargon-free. IMPORTANT: if this release has
nothing a typical user would notice (only internal fixes or minor patches),
OMIT this entire section — that deliberately keeps the in-app banner silent
for maintenance releases. Keep the heading at level 3 (###).]

### New Features
[Genuinely new user-facing capabilities. Be specific about what users can now do.]

### Bug Fixes
[Fixes for problems users would have encountered. Include issue refs like
(#1234) only when the fix verifiably addresses that issue.]

### Improvements
[Enhancements to existing features.]

Guidelines:
- Plain, factual, understated. No marketing language, no emojis.
- Omit any section that has no items.
- Do NOT write an Installation section or anything after Improvements — the
  release pipeline appends those.
- Highlights is the ONE exception to "boring": it is shown in-app to users who
  just updated, so pick the few changes they would actually notice — but still
  facts, no hype.

Your reply must be ONLY the release-notes markdown, starting with
"## v${VERSION}" — no preamble, no code fences, no commentary.
EOF

# Strip accidental markdown fences and anything before the "## v" heading.
clean_notes() {
    sed -e 's/^```[a-z]*$//' -e 's/^```$//' | awk '/^## v/{found=1} found{print}'
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
    echo "Error: release notes generation returned no '## v' section" >&2
    exit 1
fi

# Notes to stdout only — callers capture this.
printf '%s\n' "$RELEASE_NOTES"

if [ "${SAVE_TO_FILE:-}" = "1" ]; then
    OUTPUT_FILE="release-notes-v${VERSION}.md"
    printf '%s\n' "$RELEASE_NOTES" > "$OUTPUT_FILE"
    echo "Saved to ${OUTPUT_FILE}" >&2
fi
