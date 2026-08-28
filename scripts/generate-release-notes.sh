#!/usr/bin/env bash

# Generate release notes with an agent that explores the actual repo history.
#
# Engine: Codex CLI (`codex exec`, using the logged-in subscription).
# Independent research, drafting, and review passes use read-only repository
# tools. The models decide what matters from the evidence; the shell only owns
# the comparison range and the public release-note shape.
#
# Usage:  ./scripts/generate-release-notes.sh <version> [comparison-tag]
#         ./scripts/generate-release-notes.sh --resolve-base <version>
#         ./scripts/generate-release-notes.sh --visual-plan <version> <notes-file>
#
# Contract: the release notes markdown is written to STDOUT (trigger-release.sh
# captures it); all progress/diagnostics go to STDERR. SAVE_TO_FILE=1 also
# writes release-notes-v<version>.md.
#
# Env overrides:
#   RELEASE_NOTES_REASONING_EFFORT=<level> Codex effort (default: medium)
#   RELEASE_NOTES_TRACE_DIR=<directory> retain each pass for inspection
#   RELEASE_NOTE_VISUAL_PLAN_FILE=<path> write a validated optional visual plan

set -euo pipefail

MODE=generate
if [ "${1:-}" = "--resolve-base" ]; then
    MODE=resolve-base
    shift
elif [ "${1:-}" = "--visual-plan" ]; then
    MODE=visual-plan
    shift
fi

VERSION=${1:-}
VISUAL_NOTES_FILE=""
if [ "$MODE" = "visual-plan" ]; then
    VISUAL_NOTES_FILE=${2:-}
    REQUESTED_COMPARISON_TAG=${3:-}
else
    REQUESTED_COMPARISON_TAG=${2:-}
fi

if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version> [comparison-tag]" >&2
    echo "       $0 --resolve-base <version>" >&2
    echo "       $0 --visual-plan <version> <notes-file> [comparison-tag]" >&2
    echo "Example: $0 6.4.0-rc.6" >&2
    exit 1
fi

if [ "$MODE" = "visual-plan" ] && [ ! -s "$VISUAL_NOTES_FILE" ]; then
    echo "Visual planning requires a non-empty release-notes file" >&2
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
    RELEASE_RANGE_GUIDANCE="This RC covers ${PREVIOUS_TAG}..HEAD."
else
    RELEASE_RANGE_GUIDANCE="This stable release covers ${PREVIOUS_TAG}..HEAD. The same-version RC packets under docs/releases/ are available evidence."
fi

if [ "$MODE" = "generate" ]; then
    echo "Generating release notes for v${VERSION} (changes since ${PREVIOUS_TAG})..." >&2
fi

read -r -d '' RESEARCH_PROMPT <<EOF || true
Create a comprehensive factual account of the user-observable differences in
Pulse between ${PREVIOUS_TAG} and the current HEAD.

Pulse is a self-hosted monitoring dashboard for Proxmox VE, PBS, Docker, and
Kubernetes, used mostly by homelab and small-ops users.

${RELEASE_RANGE_GUIDANCE}

Use the available read-only repository and GitHub tools as you judge useful.
Take the time needed to understand the range and final source state. Return the
account with supporting evidence in Markdown. It has no public length or item
limit.
EOF

read -r -d '' NOTE_FORMAT <<EOF || true
# Pulse v${VERSION} Release Notes

[One short paragraph explaining the customer outcome of this release.]

## What's improved

- **[Short outcome]** - [Where users notice it and why it matters.]

[Use a concise set of meaningful bullets. Each full bullet, including Markdown
links, must be 260 characters or fewer. Use no semicolon or em dash characters.]

## Before you upgrade

[Only information users must act on or understand before upgrading. Omit this
section when there is none.]
EOF

# Strip accidental markdown fences and anything before the release title.
clean_notes() {
    sed -e 's/^```[a-z]*$//' -e 's/^```$//' | \
        awk -v title="# Pulse v${VERSION} Release Notes" '$0 == title {found=1} found{print}'
}

# Run on the logged-in subscription, never on a metered API key. A stray
# environment variable can silently override subscription authentication.
scrub_env() {
    env -u OPENAI_API_KEY "$@"
}

generate_with_codex() {
    local prompt=$1
    command -v codex >/dev/null || return 1
    echo "Engine: codex" >&2
    local out
    out=$(mktemp)
    # Session log goes to stderr; only the agent's final message is kept.
    if ! scrub_env codex exec --ephemeral --sandbox read-only \
        -c "model_reasoning_effort=\"${RELEASE_NOTES_REASONING_EFFORT:-medium}\"" \
        -o "$out" "$prompt" >&2 </dev/null; then
        rm -f "$out"
        return 1
    fi
    cat "$out"
    rm -f "$out"
}

generate_notes() {
    local prompt=$1
    generate_with_codex "$prompt" || {
        echo "Codex generation failed (the Codex CLI must be installed and logged in)" >&2
        return 1
    }
}

generate_structured_notes() {
    local prompt=$1
    local schema=$2
    command -v codex >/dev/null || return 1
    local out schema_file
    out=$(mktemp)
    schema_file=$(mktemp)
    printf '%s\n' "$schema" > "$schema_file"
    echo "Engine: codex (structured output)" >&2
    if ! scrub_env codex exec --ephemeral --sandbox read-only \
        -c "model_reasoning_effort=\"${RELEASE_NOTES_REASONING_EFFORT:-medium}\"" \
        --output-schema "$schema_file" -o "$out" "$prompt" >&2 </dev/null; then
        rm -f "$out" "$schema_file"
        echo "Codex structured generation failed" >&2
        return 1
    fi
    cat "$out"
    rm -f "$out" "$schema_file"
}

save_trace() {
    local name=$1
    local content=$2
    [ -n "${RELEASE_NOTES_TRACE_DIR:-}" ] || return 0
    mkdir -p "$RELEASE_NOTES_TRACE_DIR"
    printf '%s\n' "$content" > "$RELEASE_NOTES_TRACE_DIR/$name"
}

clean_visual_plan() {
    sed -e 's/^```json$//' -e 's/^```$//'
}

generate_visual_plan() {
    local notes=$1
    local prior_research=${2:-}
    local visual_research plan validation_error visual_schema
    read -r -d '' VISUAL_RESEARCH_PROMPT <<EOF || true
Investigate the visible differences in Pulse between ${PREVIOUS_TAG} and the
current HEAD that could be communicated more clearly with screenshots than
with text alone.

Use the available read-only repository and GitHub tools as you judge useful.
Identify truthful candidate views in the previous and current source states,
including the route, visible state, and accessible UI labels needed to reach
and verify each view. You may conclude that no visual adds meaningful value.
Return a factual Markdown brief with no public length or item limit. Do not
return JSON yet.

Customer release notes:

${notes}

Prior factual release investigation, when available:

${prior_research}
EOF

    echo "Researching visual release-note evidence..." >&2
    visual_research=$(generate_notes "$VISUAL_RESEARCH_PROMPT") || return 1
    if [ -z "$visual_research" ]; then
        echo "Visual release-note research returned an empty brief" >&2
        return 1
    fi
    save_trace visual-research.md "$visual_research"
    visual_schema=$(python3 scripts/release_control/release_note_visuals.py schema)

    read -r -d '' VISUAL_PROMPT <<EOF || true
Decide whether screenshots would materially improve the customer release notes
for Pulse v${VERSION}. The comparison range is ${PREVIOUS_TAG}..HEAD.

Use the factual visual investigation below. Select no views when screenshots
would add little. The capture system can open same-origin routes and use
accessible click or wait steps. It will render the exact comparison tag and
current HEAD with identical generated demo data.

Return only JSON in this shape, with at most three captures:
{
  "schema_version": 1,
  "captures": [
    {
      "id": "lower-case-hyphenated-id",
      "title": "Short customer-facing title",
      "description": "Why this visible difference matters",
      "viewport": {"width": 1440, "height": 900},
      "before": {
        "route": "/same-origin-path",
        "steps": [
          {
            "action": "click",
            "locator": {
              "kind": "role",
              "role": "button",
              "name": "Accessible name",
              "exact": true,
              "nth": 0
            }
          }
        ],
        "ready": {"kind": "text", "value": "Visible text", "exact": true, "nth": 0}
      },
      "after": {
        "route": "/same-origin-path",
        "steps": [],
        "ready": {"kind": "text", "value": "Visible text", "exact": true, "nth": 0}
      }
    }
  ]
}

The before state may be null when a truthful comparison is unavailable and a
current-state image is still useful. Locator kind may be role, text, label, or
testid. Actions may be click or wait. Role locators use role and name. Other
locators use value. Every state needs a ready locator for content that must be
visible in the finished image. Locator names and values are literal accessible
text, not regular expressions. Use labels verified against the deterministic
generated demo data. Use no semicolon or em dash characters in public text.

Customer release notes:

${notes}

Factual visual investigation:

${visual_research}
EOF

    echo "Selecting useful release-note visuals..." >&2
    plan=$(generate_structured_notes "$VISUAL_PROMPT" "$visual_schema") || return 1
    plan=$(printf '%s\n' "$plan" | clean_visual_plan)
    if ! validation_error=$(printf '%s\n' "$plan" | \
        python3 scripts/release_control/release_note_visuals.py validate --plan - 2>&1); then
        echo "Visual plan failed validation; requesting one constrained revision..." >&2
        echo "$validation_error" >&2
        read -r -d '' VISUAL_REPAIR_PROMPT <<EOF || true
Produce a fresh JSON visual plan that passes this exact validation error:

${validation_error}

Use the factual investigation below to reconstruct the decision. An invalid or
empty prior response is not evidence that screenshots add no value. Select zero
views only if that is your judgment from the evidence. Reply only with JSON in
the required schema.

Factual visual investigation:

${visual_research}

Customer release notes:

${notes}

Invalid prior response:

${plan}
EOF
        plan=$(generate_structured_notes "$VISUAL_REPAIR_PROMPT" "$visual_schema") || return 1
        plan=$(printf '%s\n' "$plan" | clean_visual_plan)
    fi
    printf '%s\n' "$plan" | \
        python3 scripts/release_control/release_note_visuals.py validate --plan -
}

if [ "$MODE" = "visual-plan" ]; then
    VISUAL_NOTES=$(cat "$VISUAL_NOTES_FILE")
    generate_visual_plan "$VISUAL_NOTES"
    exit 0
fi

echo "Researching the complete release range..." >&2
RESEARCH_BRIEF=$(generate_notes "$RESEARCH_PROMPT") || exit 1

if [ -z "$RESEARCH_BRIEF" ]; then
    echo "Error: release research returned an empty brief" >&2
    exit 1
fi
save_trace research.md "$RESEARCH_BRIEF"

read -r -d '' DRAFT_PROMPT <<EOF || true
Write the most useful customer release notes for Pulse v${VERSION}.

${RELEASE_RANGE_GUIDANCE}

Use your judgment to select and explain what matters most to users. The
read-only repository and GitHub tools are available as you judge useful.

Use exactly this public shape:

${NOTE_FORMAT}

Reply only with the release-notes Markdown beginning exactly
"# Pulse v${VERSION} Release Notes".
EOF

echo "Drafting an independent customer release story..." >&2
RELEASE_NOTES=$(generate_notes "$DRAFT_PROMPT") || exit 1

RELEASE_NOTES=$(printf '%s\n' "$RELEASE_NOTES" | clean_notes)

if [ -z "$RELEASE_NOTES" ]; then
    echo "Error: release notes generation returned no canonical release title" >&2
    exit 1
fi
save_trace independent-draft.md "$RELEASE_NOTES"

echo "Running an independent improvement pass..." >&2
read -r -d '' REVIEW_PROMPT <<EOF || true
Independently improve the Pulse v${VERSION} customer release notes below.

The comparison range is ${PREVIOUS_TAG}..HEAD. Use the available read-only
repository and GitHub tools as you judge useful, along with the internal brief.
Decide whether the draft selected, weighted, and explained the changes in the
way most useful to users, and make any changes you judge warranted.

Use this public shape:

${NOTE_FORMAT}

Reply only with the complete corrected Markdown beginning exactly
"# Pulse v${VERSION} Release Notes".

Factual account from a separate investigation:

${RESEARCH_BRIEF}

Candidate release notes:

${RELEASE_NOTES}
EOF
RELEASE_NOTES=$(generate_notes "$REVIEW_PROMPT") || exit 1
RELEASE_NOTES=$(printf '%s\n' "$RELEASE_NOTES" | clean_notes)
save_trace reviewed-notes.md "$RELEASE_NOTES"

echo "Auditing the customer story for material omissions..." >&2
read -r -d '' OMISSION_PROMPT <<EOF || true
Perform a final independent omission audit of the Pulse v${VERSION} customer
release notes below.

The comparison range is ${PREVIOUS_TAG}..HEAD. Use the available read-only
repository and GitHub tools as you judge useful, along with the factual account.
Look for distinct user-observable changes that are absent or materially
underweighted. Decide whether any omission warrants changing the customer
story. Keep the notes concise by combining related work around customer
outcomes rather than listing implementation details.

Use this public shape:

${NOTE_FORMAT}

Reply only with the complete corrected Markdown beginning exactly
"# Pulse v${VERSION} Release Notes".

Factual account from the separate investigation:

${RESEARCH_BRIEF}

Candidate release notes:

${RELEASE_NOTES}
EOF
RELEASE_NOTES=$(generate_notes "$OMISSION_PROMPT") || exit 1
RELEASE_NOTES=$(printf '%s\n' "$RELEASE_NOTES" | clean_notes)
save_trace omission-audited-notes.md "$RELEASE_NOTES"

validate_notes() {
    printf '%s\n' "$1" | python3 scripts/release_control/render_release_body.py \
        --version "$VERSION" \
        --validate-notes-file /dev/stdin
}

if ! VALIDATION_ERROR=$(validate_notes "$RELEASE_NOTES" 2>&1); then
    echo "Generated notes failed release-body validation; requesting one constrained revision..." >&2
    echo "$VALIDATION_ERROR" >&2
    read -r -d '' REPAIR_PROMPT <<EOF || true
Correct the Pulse v${VERSION} release notes below so they pass this exact
validation error while preserving their meaning:

${VALIDATION_ERROR}

Requirements:
- Output only Markdown beginning with # Pulse v${VERSION} Release Notes.
- Preserve the factual meaning selected by the independent review.
- Satisfy the reported format constraint.

Release notes to repair:

${RELEASE_NOTES}
EOF
    RELEASE_NOTES=$(generate_notes "$REPAIR_PROMPT") || exit 1
    RELEASE_NOTES=$(printf '%s\n' "$RELEASE_NOTES" | clean_notes)
    if ! validate_notes "$RELEASE_NOTES"; then
        echo "Error: revised release notes still fail release-body validation" >&2
        exit 1
    fi
fi

# Notes to stdout only — callers capture this.
printf '%s\n' "$RELEASE_NOTES"

if [ "${SAVE_TO_FILE:-}" = "1" ]; then
    OUTPUT_FILE="release-notes-v${VERSION}.md"
    printf '%s\n' "$RELEASE_NOTES" > "$OUTPUT_FILE"
    echo "Saved to ${OUTPUT_FILE}" >&2
fi

if [ -n "${RELEASE_NOTE_VISUAL_PLAN_FILE:-}" ]; then
    generate_visual_plan "$RELEASE_NOTES" "$RESEARCH_BRIEF" > "$RELEASE_NOTE_VISUAL_PLAN_FILE"
    echo "Visual plan saved to ${RELEASE_NOTE_VISUAL_PLAN_FILE}" >&2
fi
