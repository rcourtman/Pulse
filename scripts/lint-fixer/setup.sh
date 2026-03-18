#!/usr/bin/env bash
set -euo pipefail

# Quick setup and validation for lint-fixer

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Lint Fixer Setup & Validation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

MISSING=0

# 1. Check dependencies
echo "1. Checking dependencies..."

if ! command -v aider >/dev/null 2>&1; then
  echo "  ✗ aider not found"
  echo "    Install: pip install aider-chat"
  MISSING=1
else
  AIDER_VERSION=$(aider --version 2>&1 | head -1 || echo "unknown")
  echo "  ✓ aider $AIDER_VERSION"
fi

if ! command -v golangci-lint >/dev/null 2>&1; then
  if [ -f "$HOME/go/bin/golangci-lint" ]; then
    GOLANGCI_LINT="$HOME/go/bin/golangci-lint"
    echo "  ✓ golangci-lint found at ~/go/bin/golangci-lint"
  else
    echo "  ✗ golangci-lint not found"
    echo "    Install: https://golangci-lint.run/welcome/install/"
    MISSING=1
  fi
else
  GOLANGCI_LINT="golangci-lint"
  GOLANGCI_VERSION=$($GOLANGCI_LINT --version 2>&1 | head -1 || echo "unknown")
  echo "  ✓ golangci-lint $GOLANGCI_VERSION"
fi

if ! command -v go >/dev/null 2>&1; then
  echo "  ✗ go not found"
  MISSING=1
else
  GO_VERSION=$(go version | cut -d' ' -f3)
  echo "  ✓ go $GO_VERSION"
fi

echo ""

# 2. Check API key
echo "2. Checking OpenRouter API key..."

if [ -z "${OPENROUTER_API_KEY:-}" ]; then
  echo "  ✗ OPENROUTER_API_KEY not set"
  echo ""
  echo "  Get one at: https://openrouter.ai/keys"
  echo "  Then add to your shell:"
  echo "    export OPENROUTER_API_KEY='sk-or-v1-...'"
  echo ""
  echo "  Or add to ~/.zshrc or ~/.bashrc:"
  echo "    echo 'export OPENROUTER_API_KEY=\"sk-or-v1-...\"' >> ~/.zshrc"
  echo ""
  MISSING=1
else
  KEY_PREVIEW="${OPENROUTER_API_KEY:0:12}...${OPENROUTER_API_KEY: -4}"
  echo "  ✓ OPENROUTER_API_KEY set ($KEY_PREVIEW)"
fi

echo ""

# 3. Check aider configuration
echo "3. Checking aider configuration..."

if [ -f ".aider.model.settings.yml" ]; then
  echo "  ✓ .aider.model.settings.yml found"

  # Validate it has the right settings
  if grep -q "minimax/minimax-m2.5" ".aider.model.settings.yml"; then
    echo "  ✓ MiniMax M2.5 configuration present"
  else
    echo "  ⚠ MiniMax M2.5 not configured"
  fi

  if grep -q "temperature: 1.0" ".aider.model.settings.yml"; then
    echo "  ✓ Optimal temperature setting (1.0)"
  else
    echo "  ⚠ Temperature not set to 1.0"
  fi

  if grep -q "reasoning_split: true" ".aider.model.settings.yml"; then
    echo "  ✓ Thinking mode enabled (reasoning_split: true)"
  else
    echo "  ⚠ Thinking mode not enabled"
  fi
else
  echo "  ✗ .aider.model.settings.yml not found"
  echo "    This file configures MiniMax M2.5 optimizations"
  MISSING=1
fi

echo ""

if [ $MISSING -eq 1 ]; then
  echo "❌ Setup incomplete — fix issues above first"
  exit 1
fi

# 4. Test lint detection
echo "4. Testing lint detection..."

ERRCHECK_COUNT=$($GOLANGCI_LINT run --enable errcheck --disable-all ./internal/... 2>&1 | grep -c errcheck || echo "0")
DUPL_COUNT=$($GOLANGCI_LINT run --enable dupl --disable-all ./internal/... 2>&1 | grep -c dupl || echo "0")

# Clean up newlines
ERRCHECK_COUNT=$(echo "$ERRCHECK_COUNT" | tr -d '\n')
DUPL_COUNT=$(echo "$DUPL_COUNT" | tr -d '\n')

echo "  Found $ERRCHECK_COUNT errcheck warnings"
echo "  Found $DUPL_COUNT dupl warnings"
echo "  Total: $((ERRCHECK_COUNT + DUPL_COUNT)) warnings to fix"

if [ "$ERRCHECK_COUNT" -eq 0 ] && [ "$DUPL_COUNT" -eq 0 ]; then
  echo "  ✓ No warnings to fix! Codebase is clean."
  exit 0
fi

echo ""

# 5. Show top packages
echo "5. Top packages by warning count:"
echo ""

declare -A PKG_WARNINGS
for linter in errcheck dupl; do
  while IFS= read -r line; do
    if [[ "$line" =~ ^internal/([^/]+)/ ]]; then
      pkg="${BASH_REMATCH[1]}"
      PKG_WARNINGS["$pkg"]=$((${PKG_WARNINGS[$pkg]:-0} + 1))
    fi
  done < <($GOLANGCI_LINT run --enable "$linter" --disable-all ./internal/... 2>&1 | grep "$linter" || true)
done

for pkg in "${!PKG_WARNINGS[@]}"; do
  echo "${PKG_WARNINGS[$pkg]} $pkg"
done | sort -rn | head -10 | while read -r count pkg; do
  printf "  %-30s %3d warnings\n" "internal/$pkg" "$count"
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  ✅ Setup complete! Ready to run."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Run the fixer:"
echo "  ./scripts/lint-fixer/run.sh"
echo ""
echo "Monitor progress:"
echo "  ./scripts/lint-fixer/watch.sh"
echo ""
echo "Overnight execution:"
echo "  nohup ./scripts/lint-fixer/run.sh > /tmp/lint-fixer.log 2>&1 &"
echo ""
echo "Stop gracefully:"
echo "  touch scripts/lint-fixer/.stop"
echo ""
