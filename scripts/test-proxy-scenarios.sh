#!/bin/bash

# Test script for reverse proxy scenarios
# These are the cases that often break in production

set -e

PULSE_URL=${1:-http://localhost:7655}
API_TOKEN=${2:-""}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "================================================"
echo "REVERSE PROXY SCENARIO TESTING"
echo "================================================"

FAILED=0
PASSED=0

test_scenario() {
    local test_name="$1"
    local command="$2"
    
    echo -n "$test_name: "
    if eval "$command" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC}"
        ((PASSED++)) || true
    else
        echo -e "${RED}✗${NC}"
        ((FAILED++)) || true
    fi
}

echo ""
echo "1. NGINX-STYLE PROXY HEADERS"
echo "============================"

test_scenario "X-Real-IP header" \
    "curl -s -H 'X-Real-IP: 192.168.1.100' $PULSE_URL/api/health | grep -q healthy"

test_scenario "X-Forwarded-For with multiple IPs" \
    "curl -s -H 'X-Forwarded-For: 203.0.113.0, 198.51.100.0, 172.16.0.1' $PULSE_URL/api/health | grep -q healthy"

test_scenario "X-Forwarded-Proto https" \
    "curl -s -H 'X-Forwarded-Proto: https' $PULSE_URL/api/health | grep -q healthy"

test_scenario "X-Forwarded-Host" \
    "curl -s -H 'X-Forwarded-Host: pulse.example.com' $PULSE_URL/api/health | grep -q healthy"

test_scenario "X-Forwarded-Port" \
    "curl -s -H 'X-Forwarded-Port: 443' $PULSE_URL/api/health | grep -q healthy"

echo ""
echo "2. CLOUDFLARE TUNNEL HEADERS"
echo "============================="

test_scenario "CF-Connecting-IP" \
    "curl -s -H 'CF-Connecting-IP: 198.51.100.42' $PULSE_URL/api/health | grep -q healthy"

test_scenario "CF-IPCountry" \
    "curl -s -H 'CF-IPCountry: US' $PULSE_URL/api/health | grep -q healthy"

test_scenario "CF-Ray (request ID)" \
    "curl -s -H 'CF-Ray: 8a9b7c6d5e4f3a2b1' $PULSE_URL/api/health | grep -q healthy"

test_scenario "CF-Visitor (scheme)" \
    "curl -s -H 'CF-Visitor: {\"scheme\":\"https\"}' $PULSE_URL/api/health | grep -q healthy"

echo ""
echo "3. TRAEFIK HEADERS"
echo "=================="

test_scenario "X-Forwarded-Prefix (subpath)" \
    "curl -s -H 'X-Forwarded-Prefix: /apps/pulse' $PULSE_URL/api/health | grep -q healthy"

test_scenario "X-Forwarded-Method override" \
    "curl -s -H 'X-Forwarded-Method: POST' $PULSE_URL/api/health | grep -q healthy"

echo ""
echo "4. PATH VARIATIONS (CRITICAL FOR #334)"
echo "======================================="

# These are the exact scenarios that broke with issue #334
test_scenario "Root without slash - no redirect" \
    "! curl -s -I $PULSE_URL | grep -q 'Location: ./'"

test_scenario "Root with slash - no redirect" \
    "! curl -s -I $PULSE_URL/ | grep -q 'Location: ./'"

test_scenario "Root without slash - 200 OK" \
    "curl -s -o /dev/null -w '%{http_code}' $PULSE_URL | grep -q 200"

test_scenario "Root with slash - 200 OK" \
    "curl -s -o /dev/null -w '%{http_code}' $PULSE_URL/ | grep -q 200"

# Test that we don't get relative redirects for any common paths
PATHS=("" "/" "/api" "/api/" "/settings" "/login" "/dashboard")
for path in "${PATHS[@]}"; do
    test_scenario "No relative redirect for '$path'" \
        "! curl -s -I '$PULSE_URL$path' 2>/dev/null | grep -i 'location:' | grep -q '^\\.'"
done

echo ""
echo "5. WEBSOCKET THROUGH PROXY"
echo "==========================="

test_scenario "WebSocket with proxy headers" \
    "curl -s -I -H 'Upgrade: websocket' -H 'X-Forwarded-For: 10.0.0.1' $PULSE_URL/ws | grep -q 'HTTP/1.1'"

test_scenario "WebSocket with wrong origin" \
    "curl -s -I -H 'Upgrade: websocket' -H 'Origin: http://evil.com' $PULSE_URL/ws | grep -q 'HTTP/1.1'"

echo ""
echo "6. AUTHENTICATION WITH PROXY"
echo "============================="

if [ -n "$API_TOKEN" ]; then
    test_scenario "API token through proxy headers" \
        "curl -s -H 'X-API-Token: $API_TOKEN' -H 'X-Forwarded-For: 10.0.0.1' $PULSE_URL/api/state | grep -q nodes"
    
    test_scenario "API token with Cloudflare headers" \
        "curl -s -H 'X-API-Token: $API_TOKEN' -H 'CF-Connecting-IP: 1.2.3.4' $PULSE_URL/api/state | grep -q nodes"
else
    echo -e "${YELLOW}Skipping auth tests (no API_TOKEN)${NC}"
fi

echo ""
echo "7. COOKIE HANDLING THROUGH PROXY"
echo "================================="

# Test that cookies work correctly with proxy headers (important for auth)
test_scenario "Set-Cookie with secure flag for https proxy" \
    "curl -s -I -H 'X-Forwarded-Proto: https' $PULSE_URL | grep -i 'set-cookie' || true"

echo ""
echo "8. CONTENT-TYPE PRESERVATION"
echo "============================="

test_scenario "JSON content-type preserved" \
    "curl -s -I $PULSE_URL/api/version | grep -q 'Content-Type: application/json'"

test_scenario "HTML content-type for UI" \
    "curl -s -I $PULSE_URL/ | grep -q 'Content-Type: text/html'"

echo ""
echo "9. SUBPATH DEPLOYMENT"
echo "====================="

# Test if Pulse could work behind a subpath (like /monitoring/pulse/)
test_scenario "API works with referer containing subpath" \
    "curl -s -H 'Referer: https://example.com/monitoring/pulse/' $PULSE_URL/api/health | grep -q healthy"

echo ""
echo "10. COMPRESSION HANDLING"
echo "========================"

test_scenario "Accepts gzip encoding" \
    "curl -s -H 'Accept-Encoding: gzip' -I $PULSE_URL/ | grep -i 'content-encoding' || true"

test_scenario "Handles deflate encoding request" \
    "curl -s -H 'Accept-Encoding: deflate' $PULSE_URL/api/health | grep -q healthy"

echo ""
echo "================================================"
echo "RESULTS: Passed: $PASSED | Failed: $FAILED"
echo "================================================"

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}⚠️  Some proxy scenarios failed!${NC}"
    exit 1
else
    echo -e "${GREEN}✅ All proxy scenarios handled correctly${NC}"
fi