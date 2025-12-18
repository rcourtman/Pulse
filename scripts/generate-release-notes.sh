#!/usr/bin/env bash

# Generate release notes using LLM analysis of git commits
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
        echo "No previous tag found, using all commits"
        PREVIOUS_TAG=$(git rev-list --max-parents=0 HEAD)
    fi
fi

echo "Generating release notes for v${VERSION}..."
echo "Analyzing commits since ${PREVIOUS_TAG}..."

# Get commit log
COMMIT_LOG=$(git log ${PREVIOUS_TAG}..HEAD --pretty=format:"%h %s" --no-merges)

if [ -z "$COMMIT_LOG" ]; then
    echo "No commits found since ${PREVIOUS_TAG}"
    exit 1
fi

# Count commits
COMMIT_COUNT=$(echo "$COMMIT_LOG" | wc -l)
echo "Found ${COMMIT_COUNT} commits"

# Generate release notes using LLM API
# Supports both OpenAI and Anthropic Claude
# Set either OPENAI_API_KEY or ANTHROPIC_API_KEY

if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
    LLM_PROVIDER="anthropic"
elif [ -n "${OPENAI_API_KEY:-}" ]; then
    LLM_PROVIDER="openai"
else
    echo "No LLM API keys detected – falling back to deterministic release notes."
    LLM_PROVIDER="fallback"
fi

echo "Using LLM provider: ${LLM_PROVIDER}"

# Prepare prompt for LLM
read -r -d '' PROMPT <<EOF || true
You are generating release notes for Pulse v${VERSION}.

Pulse is a monitoring system for Proxmox VE and PBS with Docker container monitoring capabilities.

Analyze the following ${COMMIT_COUNT} git commits and generate professional release notes:

${COMMIT_LOG}

Generate release notes following this EXACT template format:

## What's Changed

### New Features
[List new features as bullet points, each starting with "**Feature name**:" followed by description. Do NOT include commit hashes or references.]

### Bug Fixes
[List bug fixes as bullet points, each starting with "**Component/area**:" followed by description. Include issue references like (#123) ONLY if the commit message explicitly mentions an issue number. Do NOT include commit hashes.]

### Improvements
[List improvements/enhancements as bullet points, each starting with "**Component/area**:" followed by description. Do NOT include commit hashes or references.]

### Breaking Changes
[List any breaking changes, or write "None" if there are none]

## Installation

**Quick Install (systemd / LXC / Proxmox VE):**
\`\`\`bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
\`\`\`

**Docker:**
\`\`\`bash
docker pull rcourtman/pulse:v${VERSION}
docker stop pulse && docker rm pulse
docker run -d --name pulse \\
  --restart unless-stopped \\
  -p 7655:7655 \\
  -v /opt/pulse/data:/data \\
  rcourtman/pulse:v${VERSION}
\`\`\`

**Manual Binary (amd64 example):**
\`\`\`bash
curl -LO https://github.com/rcourtman/Pulse/releases/download/v${VERSION}/pulse-v${VERSION}-linux-amd64.tar.gz
sudo systemctl stop pulse
sudo tar -xzf pulse-v${VERSION}-linux-amd64.tar.gz -C /usr/local/bin pulse
sudo systemctl start pulse
\`\`\`

**Helm:**
\`\`\`bash
helm upgrade --install pulse oci://ghcr.io/rcourtman/pulse-chart \\
  --version ${VERSION} \\
  --namespace pulse \\
  --create-namespace
\`\`\`

## Downloads
- Universal tarball (auto-detects architecture): \`pulse-v${VERSION}.tar.gz\`
- Architecture-specific: \`amd64\`, \`arm64\`, \`armv7\`, \`armv6\`, \`386\`
- Host agent packages: macOS (amd64/arm64), Windows (amd64/arm64/386), Linux (amd64/arm64/armv7/armv6/386)
- Sensor proxy: Linux (amd64/arm64/armv7/armv6/386)
- Helm chart: \`pulse-${VERSION}.tgz\`
- SHA256 checksums: \`checksums.txt\`

## Notes
[Add 2-4 bullet points highlighting the most important changes, configuration notes, or upgrade considerations. Keep this section concise and actionable.]

Guidelines:
- Match the exact format and style of the template above
- Use bold for feature/component names followed by colon
- Do NOT include commit hashes - they clutter the release notes
- Only include issue references like (#123) if explicitly mentioned in commit messages
- Focus ONLY on user-visible changes - exclude all development/infrastructure changes
- EXCLUDE: CI/CD changes, release workflow improvements, build process changes, development tooling, testing infrastructure
- EXCLUDE: Anything about "release notes generation", "automated release", "validation scripts", "GitHub workflows"
- INCLUDE ONLY: Features users interact with, bug fixes users experience, performance improvements users notice
- Use clear, non-technical language
- Group related changes together logically
- If there are no breaking changes, write "None" in that section
- Keep the Notes section practical and actionable for end users
EOF

# Helper to call Anthropic (returns notes via stdout)
generate_with_anthropic() {
    local response response_type content
    response=$(curl -s https://api.anthropic.com/v1/messages \
      -H "Content-Type: application/json" \
      -H "x-api-key: ${ANTHROPIC_API_KEY}" \
      -H "anthropic-version: 2023-06-01" \
      -d @- <<JSON
{
  "model": "claude-haiku-4-5-20251001",
  "max_tokens": 2000,
  "system": "You are a technical writer creating release notes. Be concise, clear, and focus on user-visible changes. Use proper markdown formatting.",
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

# Helper to call OpenAI (returns notes via stdout)
generate_with_openai() {
    local response content error_msg
    response=$(curl -s https://api.openai.com/v1/chat/completions \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer ${OPENAI_API_KEY}" \
      -d @- <<JSON
{
  "model": "gpt-4o-mini",
  "messages": [
    {
      "role": "system",
      "content": "You are a technical writer creating release notes. Be concise, clear, and focus on user-visible changes. Use proper markdown formatting."
    },
    {
      "role": "user",
      "content": $(echo "$PROMPT" | jq -Rs .)
    }
  ],
  "temperature": 0.7,
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

# Pretty-print a section of notes (expects array name)
print_section() {
    local ref="$1"
    declare -n arr_ref="$ref"
    if [ ${#arr_ref[@]} -eq 0 ]; then
        echo "None"
        return
    fi

    local item
    for item in "${arr_ref[@]}"; do
        echo "- ${item}"
    done
}

# Deterministic fallback when no LLM providers are available
generate_fallback_release_notes() {
    local raw_subjects=()
    mapfile -t raw_subjects < <(echo "$COMMIT_LOG" | sed 's/^[0-9a-f]\+ //')

    local subjects=()
    local subject lower
    for subject in "${raw_subjects[@]}"; do
        lower=$(echo "$subject" | tr '[:upper:]' '[:lower:]')
        if [[ "$lower" =~ (release[[:space:]-]?notes|release[[:space:]-]?workflow|workflow|github|ci|lint|docs|documentation|helm|auto-update|mock|dry[[:space:]-]?run|test|integration|build|validation|telemetry|release[[:space:]-]?assets|fallback|prepare) ]]; then
            continue
        fi
        subjects+=("$subject")
    done

    if [ ${#subjects[@]} -eq 0 ]; then
        subjects=("${raw_subjects[@]}")
    fi

    local features=()
    local bugs=()
    local improvements=()
    local breakings=()

    local subject lower
    for subject in "${subjects[@]}"; do
        [ -z "$subject" ] && continue
        lower=$(echo "$subject" | tr '[:upper:]' '[:lower:]')
        if [[ "$lower" =~ (feat|feature|add|introduc|support|new) ]]; then
            features+=("$subject")
        elif [[ "$lower" =~ (fix|bug|issue|patch|regress|correct) ]]; then
            bugs+=("$subject")
        elif [[ "$lower" =~ (breaking|deprecat|remove|drop|incompatib) ]]; then
            breakings+=("$subject")
        else
            improvements+=("$subject")
        fi
    done

    if [ ${#improvements[@]} -eq 0 ] && [ ${#features[@]} -eq 0 ] && [ ${#bugs[@]} -eq 0 ]; then
        improvements=("${subjects[@]}")
    fi

    local notes=()
    for subject in "${subjects[@]}"; do
        notes+=("$subject")
        [ ${#notes[@]} -ge 3 ] && break
    done
    if [ ${#notes[@]} -eq 0 ]; then
        notes+=("Routine maintenance and dependency updates.")
    fi

    {
        echo "## What's Changed"
        echo ""
        echo "### New Features"
        print_section features
        echo ""
        echo "### Bug Fixes"
        print_section bugs
        echo ""
        echo "### Improvements"
        print_section improvements
        echo ""
        echo "### Breaking Changes"
        print_section breakings
        echo ""
        echo "## Installation"
        echo ""
        echo "**Quick Install (systemd / LXC / Proxmox VE):**"
        cat <<'EOQI'
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
```
EOQI
        echo ""
        echo "**Docker:**"
        cat <<'EOQDock'
```bash
docker pull rcourtman/pulse:v${VERSION}
docker stop pulse && docker rm pulse
docker run -d --name pulse \
  --restart unless-stopped \
  -p 7655:7655 \
  -v /opt/pulse/data:/data \
  rcourtman/pulse:v${VERSION}
```
EOQDock
        echo ""
        echo "**Manual Binary (amd64 example):**"
        cat <<'EOQMan'
```bash
curl -LO https://github.com/rcourtman/Pulse/releases/download/v${VERSION}/pulse-v${VERSION}-linux-amd64.tar.gz
sudo systemctl stop pulse
sudo tar -xzf pulse-v${VERSION}-linux-amd64.tar.gz -C /usr/local/bin pulse
sudo systemctl start pulse
```
EOQMan
        echo ""
        echo "**Helm:**"
        cat <<'EOQHelm'
```bash
helm upgrade --install pulse oci://ghcr.io/rcourtman/pulse-chart \
  --version ${VERSION} \
  --namespace pulse \
  --create-namespace
```
EOQHelm
        echo ""
        echo "## Downloads"
        echo "- Universal tarball (auto-detects architecture): \`pulse-v${VERSION}.tar.gz\`"
        echo "- Architecture-specific: \`amd64\`, \`arm64\`, \`armv7\`, \`armv6\`, \`386\`"
        echo "- Host agent packages: macOS, Windows, Linux"
        echo "- Sensor proxy binaries: Linux (amd64/arm64/armv7/armv6/386)"
        echo "- Helm chart: \`pulse-${VERSION}.tgz\`"
        echo "- SHA256 checksums: \`checksums.txt\`"
        echo ""
        echo "## Notes"
        local item
        for item in "${notes[@]}"; do
            echo "- ${item}"
        done
    }
}

# Call LLM API based on provider with graceful fallback
RELEASE_NOTES=""
if [ "$LLM_PROVIDER" = "anthropic" ]; then
    if ! RELEASE_NOTES=$(generate_with_anthropic); then
        if [ -n "${OPENAI_API_KEY:-}" ]; then
            echo "Anthropic generation failed, falling back to OpenAI..." >&2
            if ! RELEASE_NOTES=$(generate_with_openai); then
                echo "OpenAI fallback failed; generating heuristic release notes." >&2
                RELEASE_NOTES=$(generate_fallback_release_notes)
            fi
            LLM_PROVIDER="openai"
        else
            echo "Anthropic generation failed and no OpenAI fallback is available; generating heuristic release notes." >&2
            RELEASE_NOTES=$(generate_fallback_release_notes)
        fi
    fi
else
    if ! RELEASE_NOTES=$(generate_with_openai); then
        echo "OpenAI generation failed; generating heuristic release notes." >&2
        RELEASE_NOTES=$(generate_fallback_release_notes)
    fi
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
