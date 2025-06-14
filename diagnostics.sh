#!/bin/bash

# Pulse Diagnostics Script
# This script collects diagnostic information to help troubleshoot issues

echo "================================"
echo "Pulse Diagnostics Report"
echo "================================"
echo ""

# Check if Pulse is running
PORT=${PORT:-7655}
HOST="localhost"

# Function to check if Pulse is accessible
check_pulse_running() {
    echo "Checking if Pulse is running on port $PORT..."
    if curl -s -o /dev/null -w "%{http_code}" "http://$HOST:$PORT/api/health" | grep -q "200"; then
        echo "✓ Pulse is running and accessible"
        return 0
    else
        echo "✗ Pulse is not accessible on http://$HOST:$PORT"
        echo ""
        echo "Please ensure Pulse is running before running diagnostics."
        echo "If using Docker: docker logs pulse"
        echo "If using systemd: sudo journalctl -u pulse-monitor.service -n 50"
        return 1
    fi
}

# Function to fetch and display diagnostics
run_diagnostics() {
    echo ""
    echo "Fetching diagnostic information..."
    echo ""
    
    # Fetch diagnostics from the API
    RESPONSE=$(curl -s "http://$HOST:$PORT/api/diagnostics")
    
    if [ -z "$RESPONSE" ]; then
        echo "Failed to fetch diagnostics from Pulse API"
        return 1
    fi
    
    # Save full report to file
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    REPORT_FILE="pulse_diagnostics_${TIMESTAMP}.json"
    echo "$RESPONSE" > "$REPORT_FILE"
    echo "Full diagnostic report saved to: $REPORT_FILE"
    echo ""
    
    # Parse and display key information using jq if available
    if command -v jq &> /dev/null; then
        echo "=== SUMMARY ==="
        echo "$RESPONSE" | jq -r '.summary | "Critical Issues: \(.criticalIssues)\nWarnings: \(.warnings)"'
        echo ""
        
        echo "=== CONFIGURATION ==="
        echo "Proxmox Instances:"
        echo "$RESPONSE" | jq -r '.configuration.proxmox[] | "  Instance \(.index): \(.host) (Node: \(.node_name), Auth: \(.auth_type), Self-signed: \(.self_signed_certs))"'
        echo ""
        echo "PBS Instances:"
        echo "$RESPONSE" | jq -r '.configuration.pbs[] | "  Instance \(.index): \(.host) (Node: \(.node_name), Auth: \(.auth_type), Self-signed: \(.self_signed_certs))"'
        echo ""
        
        echo "=== CONNECTIVITY ==="
        echo "Proxmox Connections:"
        echo "$RESPONSE" | jq -r '.connectivity.proxmox[] | "  Instance \(.index): \(if .reachable then "✓ Reachable" else "✗ Unreachable" end) \(if .authValid then "(Auth: ✓)" else "(Auth: ✗)" end) \(if .error then "- Error: \(.error)" else "" end)"'
        echo ""
        echo "PBS Connections:"
        echo "$RESPONSE" | jq -r '.connectivity.pbs[] | "  Instance \(.index): \(if .reachable then "✓ Reachable" else "✗ Unreachable" end) \(if .authValid then "(Auth: ✓)" else "(Auth: ✗)" end) \(if .error then "- Error: \(.error)" else "" end)"'
        echo ""
        
        echo "=== DATA FLOW ==="
        echo "$RESPONSE" | jq -r '.dataFlow | "PVE Guests: \(.pve.guests_count) (\(.pve.vms_count) VMs, \(.pve.containers_count) Containers)\nPBS Instances: \(.pbs.instances_count)\nTotal Backups: \(.pbs.backups_total)"'
        echo ""
        
        # Show PBS backup matching details
        echo "PBS Backup Matching:"
        echo "$RESPONSE" | jq -r '.dataFlow.pbs.backup_matching[] | "  Instance \(.index): \(.backups_count) backups, \(.matching_backups) matching current guests"'
        echo ""
        
        echo "=== RECOMMENDATIONS ==="
        RECOMMENDATIONS=$(echo "$RESPONSE" | jq -r '.recommendations[] | "[\(.severity | ascii_upcase)] \(.category): \(.message)"')
        if [ -z "$RECOMMENDATIONS" ]; then
            echo "No issues found - everything looks good!"
        else
            echo "$RECOMMENDATIONS"
        fi
        echo ""
        
        echo "=== SHARING THIS REPORT ==="
        echo "To share this diagnostic report:"
        echo "1. Open the file: $REPORT_FILE"
        echo "2. Remove any sensitive information (tokens, IPs if needed)"
        echo "3. Share the file content when reporting issues"
        
    else
        echo "Note: Install 'jq' for formatted output (apt-get install jq or brew install jq)"
        echo ""
        echo "Raw diagnostic data saved to: $REPORT_FILE"
        echo "Please share this file when reporting issues (after removing sensitive data)"
    fi
}

# Main execution
if check_pulse_running; then
    run_diagnostics
fi

echo ""
echo "================================"
echo "End of Diagnostic Report"
echo "================================"