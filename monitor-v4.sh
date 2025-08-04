#!/bin/bash

# Monitor v4 Pulse instances

echo "=== Pulse v4 Monitoring ==="
echo "Time: $(date)"
echo

# Check test container (delly)
echo "Test Container (delly - 192.168.0.152):"
if curl -s http://192.168.0.152:7655/api/version >/dev/null 2>&1; then
    echo "  ✓ API responding"
    version=$(curl -s http://192.168.0.152:7655/api/version | python3 -c "import json,sys; d=json.load(sys.stdin); print(f\"  Version: {d['version']} ({d['channel']} channel)\")")
    echo "$version"
else
    echo "  ✗ API not responding"
fi

# Check production container (pimox)
echo
echo "Production Container (pimox - 192.168.0.150):"
if curl -s http://192.168.0.150:7655/api/version >/dev/null 2>&1; then
    echo "  ✓ API responding"
    version=$(curl -s http://192.168.0.150:7655/api/version | python3 -c "import json,sys; d=json.load(sys.stdin); print(f\"  Version: {d['version']} ({d['channel']} channel)\")")
    echo "$version"
else
    echo "  ✗ API not responding"
fi

# Check memory usage
echo
echo "Memory Usage:"
ssh root@delly.lan "pct exec 130 -- ps aux | grep pulse-linux | grep -v grep" 2>/dev/null | awk '{print "  Test container: " $6/1024 " MB"}'
ssh root@pimox.lan "pct exec 140 -- ps aux | grep pulse-linux | grep -v grep" 2>/dev/null | awk '{print "  Prod container: " $6/1024 " MB"}'

echo
echo "==========================="