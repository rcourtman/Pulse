#!/bin/bash

echo "=== Security Fixes Test Script ==="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test 1: Check that logs don't contain sensitive data
echo "Test 1: Checking for sensitive data in logs..."
if grep -E "(api_token_first_chars|token.*:.*[a-zA-Z0-9]{10,})" /opt/pulse/pulse.log 2>/dev/null | tail -5; then
    echo -e "${RED}✗ Found potential sensitive data in logs${NC}"
else
    echo -e "${GREEN}✓ No sensitive token data found in recent logs${NC}"
fi
echo

# Test 2: Test password complexity validation
echo "Test 2: Testing password complexity validation..."
echo "Testing weak passwords that should be rejected:"

# Create a test Go program to test password validation
cat > /tmp/test-password.go << 'EOF'
package main

import (
    "fmt"
    "github.com/rcourtman/pulse-go-rewrite/internal/auth"
)

func main() {
    tests := []struct {
        password string
        shouldFail bool
        description string
    }{
        {"weak", true, "Too short"},
        {"password123", true, "Contains weak pattern"},
        {"Admin123!", false, "Strong password"},
        {"P@ssw0rd!", true, "Contains weak pattern"},
        {"MyStr0ng!Pass", false, "Strong password"},
        {"12345678", true, "Only numbers"},
        {"abcdefgh", true, "Only lowercase"},
        {"ABCDEFGH", true, "Only uppercase"},
        {"Ab1!Cd2@", false, "Strong password"},
    }

    for _, test := range tests {
        err := auth.ValidatePasswordComplexity(test.password)
        if test.shouldFail && err == nil {
            fmt.Printf("✗ FAIL: '%s' (%s) - Expected rejection but was accepted\n", test.password, test.description)
        } else if !test.shouldFail && err != nil {
            fmt.Printf("✗ FAIL: '%s' (%s) - Expected acceptance but was rejected: %v\n", test.password, test.description, err)
        } else if test.shouldFail && err != nil {
            fmt.Printf("✓ PASS: '%s' (%s) - Correctly rejected: %v\n", test.password, test.description, err)
        } else {
            fmt.Printf("✓ PASS: '%s' (%s) - Correctly accepted\n", test.password, test.description)
        }
    }
}
EOF

cd /opt/pulse && go run /tmp/test-password.go
echo

# Test 3: Check file permissions
echo "Test 3: Checking file permissions for sensitive files..."
FILES_TO_CHECK=(
    "/etc/pulse/nodes.json"
    "/etc/pulse/system.json"
    "/etc/pulse/email.json"
    "/etc/pulse/webhooks.json"
    "/etc/pulse/.encryption.key"
)

for file in "${FILES_TO_CHECK[@]}"; do
    if [ -f "$file" ]; then
        perms=$(stat -c %a "$file")
        if [ "$perms" = "600" ] || [ "$perms" = "400" ]; then
            echo -e "${GREEN}✓ $file has secure permissions ($perms)${NC}"
        else
            echo -e "${YELLOW}⚠ $file has permissions $perms (should be 600)${NC}"
        fi
    fi
done
echo

# Test 4: Check WebSocket origin restrictions
echo "Test 4: Testing WebSocket origin restrictions..."
# Try to connect with an invalid origin
response=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Origin: http://evil.com" \
    -H "Connection: Upgrade" \
    -H "Upgrade: websocket" \
    -H "Sec-WebSocket-Version: 13" \
    -H "Sec-WebSocket-Key: x3JJHMbDL1EzLkh9GBhXDw==" \
    http://localhost:7655/ws)

if [ "$response" = "403" ] || [ "$response" = "401" ]; then
    echo -e "${GREEN}✓ WebSocket correctly rejects invalid origins (HTTP $response)${NC}"
else
    echo -e "${YELLOW}⚠ WebSocket returned HTTP $response for invalid origin (expected 403/401)${NC}"
fi
echo

# Test 5: Check rate limiting
echo "Test 5: Testing rate limiting on security endpoints..."
echo "Attempting 15 rapid requests to trigger rate limiting..."
for i in {1..15}; do
    response=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:7655/api/security/setup \
        -H "Content-Type: application/json" \
        -d '{"username":"test","password":"test","apiToken":"test"}')
    if [ "$response" = "429" ]; then
        echo -e "${GREEN}✓ Rate limiting triggered after $i attempts (HTTP 429)${NC}"
        break
    elif [ "$i" = "15" ]; then
        echo -e "${YELLOW}⚠ Rate limiting not triggered after 15 attempts${NC}"
    fi
done
echo

# Test 6: Check security headers
echo "Test 6: Checking security headers..."
headers=$(curl -s -I http://localhost:7655/api/health)
security_headers=(
    "X-Frame-Options: DENY"
    "X-Content-Type-Options: nosniff"
    "Content-Security-Policy:"
)

for header in "${security_headers[@]}"; do
    if echo "$headers" | grep -q "$header"; then
        echo -e "${GREEN}✓ Header present: $header${NC}"
    else
        echo -e "${RED}✗ Missing header: $header${NC}"
    fi
done
echo

echo "=== Security Test Complete ==="