#!/bin/bash

# Edge case testing for Pulse
# Tests the weird stuff that breaks in production

set -e

PULSE_URL=${1:-http://localhost:7655}

echo "================================================"
echo "EDGE CASE TESTING"
echo "================================================"

echo ""
echo "Testing URL variations that break reverse proxies..."
echo "----------------------------------------------------"

# Test all the ways users might access Pulse
URLS=(
    "$PULSE_URL"          # No trailing slash
    "$PULSE_URL/"         # With trailing slash  
    "$PULSE_URL//"        # Double slash (happens with bad proxy configs)
    "$PULSE_URL/./"       # Relative path (should not happen!)
    "$PULSE_URL/index.html" # Direct file access
)

for url in "${URLS[@]}"; do
    echo -n "Testing: $url ... "
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -I "$url" 2>/dev/null || echo "FAIL")
    LOCATION=$(curl -s -I "$url" 2>/dev/null | grep -i "^location:" | cut -d' ' -f2 | tr -d '\r\n' || echo "none")
    
    if [[ "$STATUS" == "200" ]]; then
        echo "✓ 200 OK"
    elif [[ "$STATUS" == "301" ]] || [[ "$STATUS" == "302" ]]; then
        echo "⚠️  Redirect to: $LOCATION"
        if [[ "$LOCATION" == "./" ]] || [[ "$LOCATION" == "../" ]]; then
            echo "   ❌ RELATIVE REDIRECT DETECTED - THIS BREAKS PROXIES!"
        fi
    else
        echo "❌ Status: $STATUS"
    fi
done

echo ""
echo "Testing problematic header combinations..."
echo "----------------------------------------------------"

# Headers that various proxies send
HEADER_TESTS=(
    "-H 'Host: example.com'"                                    # Different host
    "-H 'X-Forwarded-Host: proxy.local' -H 'X-Forwarded-Proto: https'"  # Proxy headers
    "-H 'X-Real-IP: 10.0.0.1' -H 'X-Forwarded-For: 10.0.0.1'"  # Multiple IPs
    "-H 'CF-Connecting-IP: 1.2.3.4'"                           # Cloudflare
    "-H 'X-Forwarded-Prefix: /pulse'"                          # Subpath proxy
)

for headers in "${HEADER_TESTS[@]}"; do
    echo -n "Testing with: $headers ... "
    if eval "curl -s $headers '$PULSE_URL' | grep -q '<title>Pulse</title>'" 2>/dev/null; then
        echo "✓"
    else
        echo "❌ Failed"
    fi
done

echo ""
echo "Testing authentication edge cases..."
echo "----------------------------------------------------"

# Test various auth header formats
echo -n "Empty API token header: "
curl -s -H "X-API-Token: " "$PULSE_URL/api/health" | grep -q "healthy" && echo "✓" || echo "❌"

echo -n "Malformed API token: "
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-Token: notavalidtoken" "$PULSE_URL/api/state")
[[ "$RESPONSE" == "401" ]] && echo "✓ Properly rejected" || echo "❌ Status: $RESPONSE"

echo -n "SQL injection in API token: "
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-Token: ' OR '1'='1" "$PULSE_URL/api/state")
[[ "$RESPONSE" == "401" ]] && echo "✓ Properly rejected" || echo "❌ Status: $RESPONSE"

echo ""
echo "Testing concurrent connections..."
echo "----------------------------------------------------"

echo "Sending 50 concurrent requests..."
for i in {1..50}; do
    curl -s "$PULSE_URL/api/health" > /dev/null &
done
wait
echo "✓ Handled concurrent load"

echo ""
echo "Testing large request handling..."
echo "----------------------------------------------------"

# Test with large headers
echo -n "Large header (10KB): "
LARGE_HEADER=$(head -c 10000 /dev/zero | tr '\0' 'A')
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "X-Large: $LARGE_HEADER" "$PULSE_URL/api/health" 2>/dev/null || echo "FAIL")
[[ "$RESPONSE" == "200" ]] && echo "✓" || echo "❌ Status: $RESPONSE"

echo ""
echo "Testing special characters in URLs..."
echo "----------------------------------------------------"

SPECIAL_PATHS=(
    "/api/health?test=<script>alert(1)</script>"  # XSS attempt
    "/api/health?test=';DROP TABLE--"              # SQL injection
    "/api/../../../etc/passwd"                     # Path traversal
    "/api/health%00.json"                          # Null byte
    "/api/health?test=%"                           # Invalid encoding
)

for path in "${SPECIAL_PATHS[@]}"; do
    echo -n "Testing: $path ... "
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" "$PULSE_URL$path" 2>/dev/null || echo "FAIL")
    if [[ "$RESPONSE" == "200" ]] || [[ "$RESPONSE" == "400" ]] || [[ "$RESPONSE" == "404" ]]; then
        echo "✓ Handled safely ($RESPONSE)"
    else
        echo "❌ Unexpected: $RESPONSE"
    fi
done

echo ""
echo "Testing WebSocket edge cases..."
echo "----------------------------------------------------"

echo -n "WebSocket with wrong protocol: "
curl -s -I -H "Upgrade: wrong" "$PULSE_URL/ws" | grep -q "HTTP/1.1" && echo "✓" || echo "❌"

echo -n "WebSocket with auth token: "
curl -s -I -H "Upgrade: websocket" -H "X-API-Token: test" "$PULSE_URL/ws" | grep -q "HTTP/1.1" && echo "✓" || echo "❌"

echo ""
echo "================================================"
echo "EDGE CASE TESTING COMPLETE"
echo "================================================"