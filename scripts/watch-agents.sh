#!/bin/bash
# Automatic Agent Hot-Reload Watcher
# Watches for local code changes and pushes to all agents

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DEPLOY_SCRIPT="$SCRIPT_DIR/dev-deploy-agent.sh"

# Hosts to sync to (edit this list as needed)
HOSTS=("tower")

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[WATCHER]${NC} $1"
}

if ! command -v inotifywait &> /dev/null; then
    echo "Error: inotifywait not found. Install with: sudo apt install inotify-tools"
    exit 1
fi

log_info "${GREEN}Starting Agent Hot-Reload Watcher...${NC}"
log_info "Target hosts: ${HOSTS[*]}"
log_info "Watching internal/, pkg/, and cmd/pulse-agent/ for changes..."

# Debounce deployment to prevent multiple builds for rapid saves
LAST_DEPLOY=0
DEBOUNCE_SEC=2

cd "$PROJECT_ROOT"

# Use inotifywait to watch relevant directories
inotifywait -m -r -e modify,create,delete,move \
    --exclude ".*\.test\.go" \
    "$PROJECT_ROOT/internal" \
    "$PROJECT_ROOT/pkg" \
    "$PROJECT_ROOT/cmd/pulse-agent" | 
while read -r path action file; do
    # Only trigger for .go files
    if [[ "$file" == *.go ]]; then
        NOW=$(date +%s)
        if (( NOW - LAST_DEPLOY > DEBOUNCE_SEC )); then
            echo ""
            log_info "${YELLOW}Change detected in $path$file. Deploying...${NC}"
            
            # Execute deployment in background so we don't miss other events
            # but wait for it so we don't overlap builds
            "$DEPLOY_SCRIPT" "${HOSTS[@]}" || true
            
            LAST_DEPLOY=$(date +%s)
            log_info "${GREEN}Ready for next change.${NC}"
        fi
    fi
done
