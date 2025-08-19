#!/bin/bash

# Test different installation methods
# Catches issues with install script, Docker, systemd, etc.

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "================================================"
echo "INSTALLATION METHOD TESTING"
echo "================================================"

VERSION=${1:-$(cat VERSION)}

echo ""
echo "1. INSTALL SCRIPT VALIDATION"
echo "============================"

echo -n "Install script exists on GitHub: "
if curl -s -f https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh > /dev/null; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
fi

echo -n "Install script is executable: "
SCRIPT=$(curl -s https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh)
if echo "$SCRIPT" | head -1 | grep -q "^#!/bin/bash"; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
fi

echo -n "Install script has version detection: "
if echo "$SCRIPT" | grep -q "VERSION="; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
fi

echo ""
echo "2. GITHUB RELEASE ARTIFACTS"
echo "==========================="

ARTIFACTS=(
    "pulse-v${VERSION}-linux-amd64.tar.gz"
    "pulse-v${VERSION}-linux-arm64.tar.gz"
    "pulse-v${VERSION}-linux-armv7.tar.gz"
    "pulse-v${VERSION}.tar.gz"
    "checksums.txt"
)

for artifact in "${ARTIFACTS[@]}"; do
    echo -n "Checking $artifact: "
    URL="https://github.com/rcourtman/Pulse/releases/download/v${VERSION}/${artifact}"
    if curl -s -f -I "$URL" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${YELLOW}Not yet uploaded${NC}"
    fi
done

echo ""
echo "3. DOCKER IMAGE AVAILABILITY"
echo "============================"

echo -n "Docker Hub image exists: "
if curl -s "https://hub.docker.com/v2/repositories/rcourtman/pulse/tags/v${VERSION}" | grep -q "v${VERSION}"; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${YELLOW}Not yet pushed${NC}"
fi

echo ""
echo "4. SYSTEMD SERVICE FILE"
echo "======================="

echo -n "Service file exists locally: "
if [ -f /etc/systemd/system/pulse.service ] || [ -f /etc/systemd/system/pulse-backend.service ]; then
    echo -e "${GREEN}✓${NC}"
    
    # Check service file contents
    SERVICE_FILE=$(ls /etc/systemd/system/pulse*.service 2>/dev/null | head -1)
    
    echo -n "Service has Restart=always: "
    if grep -q "Restart=always" "$SERVICE_FILE"; then
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${RED}✗${NC}"
    fi
    
    echo -n "Service runs as pulse user: "
    if grep -q "User=pulse" "$SERVICE_FILE"; then
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${YELLOW}⚠️  Running as different user${NC}"
    fi
else
    echo -e "${YELLOW}Not installed via systemd${NC}"
fi

echo ""
echo "5. BINARY COMPATIBILITY TESTS"
echo "============================="

echo -n "Current binary runs: "
if /opt/pulse/pulse help > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
fi

echo -n "Binary has embedded frontend: "
if strings /opt/pulse/pulse 2>/dev/null | grep -q "<!DOCTYPE html>"; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗ Frontend not embedded!${NC}"
fi

echo ""
echo "6. CONFIGURATION VALIDATION"
echo "==========================="

echo -n "Config directory exists: "
if [ -d /etc/pulse ]; then
    echo -e "${GREEN}✓${NC}"
    
    echo -n ".env file exists: "
    if [ -f /etc/pulse/.env ]; then
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${YELLOW}Using defaults${NC}"
    fi
    
    echo -n "nodes.json exists: "
    if [ -f /etc/pulse/nodes.json ]; then
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${YELLOW}No nodes configured${NC}"
    fi
else
    echo -e "${RED}✗${NC}"
fi

echo ""
echo "7. PERMISSION CHECKS"
echo "==================="

echo -n "Pulse user exists: "
if id pulse > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC}"
    
    echo -n "Config owned by pulse user: "
    if [ -d /etc/pulse ] && [ "$(stat -c %U /etc/pulse)" = "pulse" ]; then
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${YELLOW}⚠️  Ownership issue${NC}"
    fi
else
    echo -e "${YELLOW}Running as different user${NC}"
fi

echo ""
echo "8. PORT AVAILABILITY"
echo "==================="

echo -n "Port 7655 is listening: "
if netstat -tln 2>/dev/null | grep -q :7655 || ss -tln 2>/dev/null | grep -q :7655; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗ Not listening${NC}"
fi

echo ""
echo "9. UPDATE MECHANISM"
echo "=================="

echo -n "Can detect current version: "
CURRENT=$(curl -s http://localhost:7655/api/version 2>/dev/null | jq -r .version 2>/dev/null)
if [ -n "$CURRENT" ]; then
    echo -e "${GREEN}✓ ($CURRENT)${NC}"
else
    echo -e "${RED}✗${NC}"
fi

echo -n "Can check for updates: "
if curl -s http://localhost:7655/api/updates/check 2>/dev/null | grep -q "currentVersion"; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${YELLOW}Requires auth${NC}"
fi

echo ""
echo "================================================"
echo "Installation tests complete"
echo "================================================"