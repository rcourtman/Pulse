#!/bin/bash
# Auto-start script for pulse dev environment

PIDFILE="/tmp/pulse-dev.pid"
LOGFILE="/tmp/pulse-dev.log"

# Check if already running
if [ -f "$PIDFILE" ] && kill -0 $(cat "$PIDFILE") 2>/dev/null; then
    echo "Pulse dev server already running (PID: $(cat $PIDFILE))"
    exit 0
fi

# Start hot-dev.sh
cd /workspaces/pulse
nohup ./scripts/hot-dev.sh > "$LOGFILE" 2>&1 &
echo $! > "$PIDFILE"

echo "Pulse dev server starting... (PID: $(cat $PIDFILE))"
echo "Logs: tail -f $LOGFILE"
echo "Frontend will be available at http://localhost:7655"
