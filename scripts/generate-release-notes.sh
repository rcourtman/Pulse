#!/usr/bin/env bash

# Generate release notes using LLM analysis of actual code diffs (not commit messages)
# Usage: ./scripts/generate-release-notes.sh <version> [previous-tag]

set -euo pipefail

VERSION=${1:-}
PREVIOUS_TAG=${2:-}

if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version> [previous-tag]"
    echo "Example: $0 4.29.0 v4.28.0"
    exit 1
fi

# Find previous tag if not specified
if [ -z "$PREVIOUS_TAG" ]; then
    PREVIOUS_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    if [ -z "$PREVIOUS_TAG" ]; then
        echo "No previous tag found, cannot generate diff-based release notes"
        exit 1
    fi
fi

echo "Generating release notes for v${VERSION}..."
echo "Comparing code changes from ${PREVIOUS_TAG} to HEAD..."

# Get diff stats (excluding non-user-facing files)
DIFF_STAT=$(git diff ${PREVIOUS_TAG}..HEAD --stat \
    -- ':!*.md' ':!*.test.go' ':!*_test.go' ':!*_test.tsx' ':!*_test.ts' \
    ':!.github/*' ':!tests/*' ':!docs/*' ':!*.txt' ':!*.json' ':!go.sum' \
    ':!frontend-modern/src/**/__tests__/*' \
    | tail -20)

# Get list of changed user-facing files
CHANGED_FILES=$(git diff ${PREVIOUS_TAG}..HEAD --name-only \
    -- ':!*.md' ':!*.test.go' ':!*_test.go' ':!*_test.tsx' ':!*_test.ts' \
    ':!.github/*' ':!tests/*' ':!docs/*' ':!*.txt' ':!go.sum' \
    ':!frontend-modern/src/**/__tests__/*' \
    | head -100)

# Get specific diffs for key user-facing areas (truncated for API limits)

# API routes/handlers - new endpoints
API_DIFF=$(git diff ${PREVIOUS_TAG}..HEAD -- 'internal/api/*.go' ':!*_test.go' \
    | grep -E '^\+.*func.*Handle|^\+.*router\.(GET|POST|PUT|DELETE|PATCH)|^\+.*\.Path\(' \
    | head -30 || echo "")

# Frontend pages and components - new features
FRONTEND_DIFF=$(git diff ${PREVIOUS_TAG}..HEAD -- 'frontend-modern/src/components/*.tsx' 'frontend-modern/src/pages/*.tsx' \
    ':!*_test.tsx' ':!*__tests__*' \
    | grep -E '^\+.*export|^\+.*function.*\(|^\+.*const.*=' \
    | head -40 || echo "")

# Config options - new settings users can configure
CONFIG_DIFF=$(git diff ${PREVIOUS_TAG}..HEAD -- 'internal/config/*.go' ':!*_test.go' \
    | grep -E '^\+.*`json:|^\+.*`yaml:' \
    | head -20 || echo "")

# Notifications/alerts - webhook changes, alert features
ALERT_DIFF=$(git diff ${PREVIOUS_TAG}..HEAD -- 'internal/notifications/*.go' 'internal/alerts/*.go' ':!*_test.go' \
    | grep -E '^\+' \
    | head -30 || echo "")

# Agent changes - host/docker agent features
AGENT_DIFF=$(git diff ${PREVIOUS_TAG}..HEAD -- 'cmd/pulse-agent/*.go' 'internal/agent/*.go' ':!*_test.go' \
    | grep -E '^\+' \
    | head -20 || echo "")

# Install script changes
INSTALL_DIFF=$(git diff ${PREVIOUS_TAG}..HEAD -- 'scripts/install.sh' 'install.sh' \
    | grep -E '^\+' \
    | head -20 || echo "")

# Models/types - new data structures
MODELS_DIFF=$(git diff ${PREVIOUS_TAG}..HEAD -- 'internal/models/*.go' ':!*_test.go' \
    | grep -E '^\+.*type.*struct|^\+.*`json:' \
    | head -20 || echo "")

# Bug fixes: Find commits referencing issues and verify fix is still in final code
echo "Checking for verified bug fixes..."
VERIFIED_BUG_FIXES=""

# Get commits that reference issues (pattern: #1234 or Related to #1234)
ISSUE_COMMITS=$(git log ${PREVIOUS_TAG}..HEAD --oneline --grep='#[0-9]' 2>/dev/null || echo "")

if [ -n "$ISSUE_COMMITS" ]; then
    while IFS= read -r commit_line; do
        [ -z "$commit_line" ] && continue
        
        # Extract commit hash and message
        COMMIT_HASH=$(echo "$commit_line" | awk '{print $1}')
        COMMIT_MSG=$(echo "$commit_line" | cut -d' ' -f2-)
        
        # Get the files this commit touched
        COMMIT_FILES=$(git diff-tree --no-commit-id --name-only -r "$COMMIT_HASH" 2>/dev/null | head -5)
        
        # Check if any of those files have changes in the final diff
        CHANGE_STILL_EXISTS=false
        for file in $COMMIT_FILES; do
            if git diff ${PREVIOUS_TAG}..HEAD --name-only | grep -q "^${file}$"; then
                # File is still modified in final diff - verify the commit's changes exist
                COMMIT_ADDITIONS=$(git show "$COMMIT_HASH" --pretty="" --unified=0 -- "$file" 2>/dev/null | grep '^+[^+]' | head -3 || echo "")
                if [ -n "$COMMIT_ADDITIONS" ]; then
                    # Check if at least one added line still exists in final diff
                    FIRST_ADDITION=$(echo "$COMMIT_ADDITIONS" | head -1 | sed 's/^+//' | head -c 40)
                    if [ -n "$FIRST_ADDITION" ] && git diff ${PREVIOUS_TAG}..HEAD -- "$file" | grep -qF "$FIRST_ADDITION"; then
                        CHANGE_STILL_EXISTS=true
                        break
                    fi
                fi
            fi
        done
        
        if [ "$CHANGE_STILL_EXISTS" = true ]; then
            VERIFIED_BUG_FIXES="${VERIFIED_BUG_FIXES}${commit_line}
"
        fi
    done <<< "$ISSUE_COMMITS"
fi

# Clean up the bug fixes list
VERIFIED_BUG_FIXES=$(echo "$VERIFIED_BUG_FIXES" | sed '/^$/d' | head -15)

echo "Collected diffs from key areas"

# Auto-load API keys from local secrets if not already set
PULSE_SECRETS_DIR="${PULSE_SECRETS_DIR:-$HOME/Development/pulse/secrets}"
if [ -z "${ANTHROPIC_API_KEY:-}" ] && [ -f "${PULSE_SECRETS_DIR}/anthropic/api_key" ]; then
    ANTHROPIC_API_KEY=$(cat "${PULSE_SECRETS_DIR}/anthropic/api_key")
    export ANTHROPIC_API_KEY
fi

# Check for LLM API keys
if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
    LLM_PROVIDER="anthropic"
elif [ -n "${OPENAI_API_KEY:-}" ]; then
    LLM_PROVIDER="openai"
else
    echo "No LLM API keys detected – cannot generate diff-based notes."
    echo "Set ANTHROPIC_API_KEY or OPENAI_API_KEY"
    exit 1
fi

echo "Using LLM provider: ${LLM_PROVIDER}"

# Build the prompt with actual code changes
read -r -d '' PROMPT <<EOF || true
You are generating release notes for Pulse v${VERSION}.
Pulse is a monitoring dashboard for Proxmox VE, PBS, and Docker containers.

IMPORTANT: You are analyzing ACTUAL CODE DIFFS, not commit messages. This means you see what is truly different between v${PREVIOUS_TAG} and v${VERSION}. Focus only on user-visible changes.

Here are the files that changed (excluding tests/docs):
${CHANGED_FILES}

Summary of changes:
${DIFF_STAT}

NEW API ENDPOINTS/HANDLERS (lines starting with + are additions):
${API_DIFF:-No significant API changes detected}

NEW FRONTEND FEATURES:
${FRONTEND_DIFF:-No significant frontend changes detected}

NEW CONFIG OPTIONS:
${CONFIG_DIFF:-No significant config changes detected}

ALERT/NOTIFICATION CHANGES:
${ALERT_DIFF:-No significant alert changes detected}

AGENT CHANGES:
${AGENT_DIFF:-No significant agent changes detected}

INSTALL SCRIPT CHANGES:
${INSTALL_DIFF:-No significant install changes detected}

NEW DATA MODELS:
${MODELS_DIFF:-No significant model changes detected}

VERIFIED BUG FIXES (commits referencing issues, verified still in final code):
${VERIFIED_BUG_FIXES:-No verified bug fix commits found}

Generate release notes following this format:

## v${VERSION}

### New Features
[List genuinely new user-facing features. Be specific about what users can now do.]

### Bug Fixes  
[List fixes for issues users would have encountered. Use the VERIFIED BUG FIXES section above - these reference actual GitHub issues. Include the issue number in format (#1234).]

### Improvements
[List enhancements to existing features.]

---

## Installation

**Quick Install (LXC / Proxmox VE):**
\`\`\`bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
\`\`\`

**Docker:**
\`\`\`bash
docker pull rcourtman/pulse:${VERSION}
\`\`\`

**Helm:**
\`\`\`bash
helm upgrade --install pulse oci://ghcr.io/rcourtman/pulse-chart --version ${VERSION}
\`\`\`

See the [Installation Guide](https://github.com/rcourtman/Pulse#installation) for details.

GUIDELINES:
- Write plain, factual release notes. No marketing language or excitement.
- Only mention features that exist in the FINAL code state
- Do not mention internal refactors, test changes, or CI/CD improvements
- Do not mention AI features prominently - these are optional features
- Keep it concise and boring - users want facts, not hype
- If a section has no items, omit the section entirely
- No emojis
EOF

# Helper to call Anthropic
generate_with_anthropic() {
    local response content
    response=$(curl -s https://api.anthropic.com/v1/messages \
      -H "Content-Type: application/json" \
      -H "x-api-key: ${ANTHROPIC_API_KEY}" \
      -H "anthropic-version: 2023-06-01" \
      -d @- <<JSON
{
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 2000,
  "system": "You are a technical writer creating factual, understated release notes. Be concise and avoid marketing language.",
  "messages": [
    {
      "role": "user",
      "content": $(echo "$PROMPT" | jq -Rs .)
    }
  ]
}
JSON
) || {
        echo "Anthropic API request failed" >&2
        return 1
    }

    local response_type
    response_type=$(echo "$response" | jq -r '.type // empty')
    if [ "$response_type" = "error" ]; then
        local message
        message=$(echo "$response" | jq -r '.error.message // "Unknown error"')
        echo "Anthropic API error: $message" >&2
        return 1
    fi

    content=$(echo "$response" | jq -r '.content[0].text // empty')
    if [ -z "$content" ] || [ "$content" = "null" ]; then
        echo "Anthropic API returned empty content: $response" >&2
        return 1
    fi

    printf '%s' "$content"
}

# Helper to call OpenAI
generate_with_openai() {
    local response content error_msg
    response=$(curl -s https://api.openai.com/v1/chat/completions \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer ${OPENAI_API_KEY}" \
      -d @- <<JSON
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "system",
      "content": "You are a technical writer creating factual, understated release notes. Be concise and avoid marketing language."
    },
    {
      "role": "user",
      "content": $(echo "$PROMPT" | jq -Rs .)
    }
  ],
  "temperature": 0.3,
  "max_tokens": 2000
}
JSON
) || {
        echo "OpenAI API request failed" >&2
        return 1
    }

    error_msg=$(echo "$response" | jq -r '.error.message? // empty')
    if [ -n "$error_msg" ]; then
        echo "OpenAI API error: $error_msg" >&2
        return 1
    fi

    content=$(echo "$response" | jq -r '.choices[0].message.content // empty')
    if [ -z "$content" ] || [ "$content" = "null" ]; then
        echo "OpenAI API returned empty content: $response" >&2
        return 1
    fi

    printf '%s' "$content"
}

# Call LLM API based on provider
RELEASE_NOTES=""
if [ "$LLM_PROVIDER" = "anthropic" ]; then
    if ! RELEASE_NOTES=$(generate_with_anthropic); then
        if [ -n "${OPENAI_API_KEY:-}" ]; then
            echo "Anthropic generation failed, falling back to OpenAI..." >&2
            RELEASE_NOTES=$(generate_with_openai) || {
                echo "Both LLM providers failed" >&2
                exit 1
            }
        else
            echo "Anthropic generation failed and no OpenAI fallback" >&2
            exit 1
        fi
    fi
else
    RELEASE_NOTES=$(generate_with_openai) || {
        echo "OpenAI generation failed" >&2
        exit 1
    }
fi

if [ -z "$RELEASE_NOTES" ] || [ "$RELEASE_NOTES" = "null" ]; then
    echo "Error: Release notes generation returned empty content" >&2
    exit 1
fi

# Output release notes
echo ""
echo "Generated release notes:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "$RELEASE_NOTES"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Optionally save to file
if [ "${SAVE_TO_FILE:-}" = "1" ]; then
    OUTPUT_FILE="release-notes-v${VERSION}.md"
    echo "$RELEASE_NOTES" > "$OUTPUT_FILE"
    echo ""
    echo "Saved to: $OUTPUT_FILE"
fi
