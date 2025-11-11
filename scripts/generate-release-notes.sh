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
    echo "Error: Either OPENAI_API_KEY or ANTHROPIC_API_KEY environment variable must be set"
    exit 1
fi

echo "Using LLM provider: ${LLM_PROVIDER}"

# Prepare prompt for LLM
read -r -d '' PROMPT <<EOF || true
You are generating release notes for Pulse v${VERSION}.

Pulse is a monitoring system for Proxmox VE and PBS with Docker container monitoring capabilities.

Analyze the following ${COMMIT_COUNT} git commits and generate professional release notes:

${COMMIT_LOG}

Generate release notes in this exact markdown format:

## What's Changed

[Organize changes into 3-5 categories like: New Features, Improvements, Bug Fixes, Docker/Agent Changes, Documentation, etc. Use bullet points with commit references.]

## Installation

Quick install on Linux:
\`\`\`bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | sudo bash
\`\`\`

Or download platform-specific archives below.

## Docker Images

Docker images are available for this release:
- \`rcourtman/pulse:v${VERSION}\`
- \`rcourtman/pulse-docker-agent:v${VERSION}\`

Guidelines:
- Group related commits together
- Focus on user-visible changes
- Use clear, non-technical language
- Skip internal refactoring unless it impacts users
- Mention breaking changes prominently if any
- Keep it concise but informative
EOF

# Call LLM API based on provider
if [ "$LLM_PROVIDER" = "anthropic" ]; then
    RESPONSE=$(curl -s https://api.anthropic.com/v1/messages \
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
)
    RELEASE_NOTES=$(echo "$RESPONSE" | jq -r '.content[0].text')
else
    RESPONSE=$(curl -s https://api.openai.com/v1/chat/completions \
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
)
    RELEASE_NOTES=$(echo "$RESPONSE" | jq -r '.choices[0].message.content')
fi

if [ -z "$RELEASE_NOTES" ] || [ "$RELEASE_NOTES" = "null" ]; then
    echo "Error: Failed to generate release notes"
    echo "API Response: $RESPONSE"
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
