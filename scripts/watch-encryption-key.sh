#!/bin/bash
# Watch the encryption key file for any modifications or deletions
# Logs to journald with full context about what process did it

LOG_TAG="encryption-key-watcher"
WATCH_DIR="/etc/pulse"
WATCH_FILE=".encryption.key"

log_event() {
    local event="$1"
    local file="$2"
    
    # Get current process info
    echo "[$LOG_TAG] EVENT: $event on $file at $(date '+%Y-%m-%d %H:%M:%S')"
    
    # Try to capture what processes are accessing /etc/pulse
    echo "[$LOG_TAG] Processes with open files in /etc/pulse:"
    lsof +D /etc/pulse 2>/dev/null | head -20 || echo "  (lsof failed)"
    
    # Log the current state of the file
    if [[ -f "$WATCH_DIR/$WATCH_FILE" ]]; then
        echo "[$LOG_TAG] File still exists: $(ls -la "$WATCH_DIR/$WATCH_FILE")"
    else
        echo "[$LOG_TAG] *** FILE IS MISSING! ***"
        echo "[$LOG_TAG] Contents of $WATCH_DIR:"
        ls -la "$WATCH_DIR" | grep -i enc
    fi
    
    # Log recent sudo commands
    echo "[$LOG_TAG] Recent sudo activity:"
    journalctl -u sudo --since "2 minutes ago" --no-pager 2>/dev/null | tail -10 || true
}

echo "[$LOG_TAG] Starting encryption key watcher..."
echo "[$LOG_TAG] Monitoring: $WATCH_DIR/$WATCH_FILE"

# Watch for all relevant events on the directory
inotifywait -m -e delete,move,modify,attrib,create "$WATCH_DIR" --format '%e %f' 2>/dev/null | while read event file; do
    # Only log events related to encryption key
    if [[ "$file" == ".encryption.key"* ]]; then
        log_event "$event" "$file"
    fi
done
