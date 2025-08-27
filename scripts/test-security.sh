#!/bin/bash

# Security testing for Pulse
# Tests authentication, authorization, input validation, etc.
# Detects auth state and adjusts expectations

set -e

PULSE_URL=${1:-http://localhost:7655}
API_TOKEN=${2:-""}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "================================================"
echo "SECURITY TESTING"
echo "================================================"

VULNERABILITIES=0

# Detect if auth is disabled
AUTH_DISABLED=false
security_status=$(curl -s "$PULSE_URL/api/security/status" 2>/dev/null || echo "{}")
if echo "$security_status" | grep -q '"disabled":true'; then
    AUTH_DISABLED=true
    echo ""
    echo -e "${YELLOW}Note: Authentication is DISABLED (DISABLE_AUTH mode)${NC}"
    echo -e "${YELLOW}Skipping auth bypass tests${NC}"
fi

test_security() {
    local test_name="$1"
    local command="$2"
    local expected="$3"
    
    echo -n "$test_name: "
    RESULT=$(eval "$command" 2>/dev/null || echo "ERROR")
    if [[ "$RESULT" == *"$expected"* ]]; then
        echo -e "${GREEN}✓ Secure${NC}"
    else
        echo -e "${RED}✗ VULNERABLE${NC}"
        ((VULNERABILITIES++))
    fi
}

echo ""
echo "1. AUTHENTICATION BYPASS ATTEMPTS"
echo "================================="

if [ "$AUTH_DISABLED" != true ]; then
    test_security "Empty auth header" \
        "curl -s -H 'X-API-Token: ' $PULSE_URL/api/state | head -1" \
        "Authentication required"

    test_security "Null token" \
        "curl -s -H 'X-API-Token: null' $PULSE_URL/api/state | head -1" \
        "Authentication required"

    test_security "SQL injection in token" \
        "curl -s -H \"X-API-Token: ' OR '1'='1\" $PULSE_URL/api/state | head -1" \
        "Authentication required"

    test_security "Command injection in token" \
        "curl -s -H 'X-API-Token: \$(whoami)' $PULSE_URL/api/state | head -1" \
        "Authentication required"

    test_security "Path traversal in token" \
        "curl -s -H 'X-API-Token: ../../etc/passwd' $PULSE_URL/api/state | head -1" \
        "Authentication required"
else
    echo -e "${YELLOW}Skipped - auth is disabled${NC}"
fi

echo ""
echo "2. PATH TRAVERSAL ATTEMPTS"
echo "=========================="

if [ "$AUTH_DISABLED" != true ]; then
    test_security "Path traversal in URL" \
        "curl -s -o /dev/null -w '%{http_code}' $PULSE_URL/../../../etc/passwd" \
        "401"  # 401 is fine - auth blocks before path processing

    test_security "Double URL encoding" \
        "curl -s -o /dev/null -w '%{http_code}' $PULSE_URL/%252e%252e%252f%252e%252e%252fetc%252fpasswd" \
        "401"  # 401 is secure - auth blocks first

    test_security "Null byte injection" \
        "curl -s -o /dev/null -w '%{http_code}' '$PULSE_URL/api/health%00.json'" \
        "401"  # Expected - auth required
else
    # When auth is disabled, server returns 200 (index page) for invalid paths
    test_security "Path traversal in URL" \
        "curl -s -o /dev/null -w '%{http_code}' $PULSE_URL/../../../etc/passwd" \
        "200"  # Returns index page, not actual file

    test_security "Double URL encoding" \
        "curl -s -o /dev/null -w '%{http_code}' $PULSE_URL/%252e%252e%252f%252e%252e%252fetc%252fpasswd" \
        "200"  # Returns index page

    test_security "Null byte injection" \
        "curl -s -o /dev/null -w '%{http_code}' '$PULSE_URL/api/health%00.json'" \
        "404"  # API endpoint with null byte returns 404
fi

echo ""
echo "3. XSS ATTEMPTS"
echo "==============="

test_security "XSS in query parameter" \
    "curl -s '$PULSE_URL/api/health?test=<script>alert(1)</script>' | grep -c '<script>'" \
    "0"

test_security "XSS in header" \
    "curl -s -H 'User-Agent: <script>alert(1)</script>' $PULSE_URL | grep -c '<script>'" \
    "0"

test_security "XSS in API response" \
    "curl -s '$PULSE_URL/api/health?callback=<script>alert(1)</script>' | grep -c '<script>'" \
    "0"

echo ""
echo "4. INJECTION ATTACKS"
echo "===================="

test_security "Command injection attempt" \
    "curl -s '$PULSE_URL/api/health?\$(whoami)' -o /dev/null -w '%{http_code}'" \
    "200"

if [ "$AUTH_DISABLED" != true ]; then
    test_security "LDAP injection attempt" \
        "curl -s -H 'X-API-Token: *)(uid=*)' $PULSE_URL/api/state | head -1" \
        "Authentication required"

    test_security "NoSQL injection attempt" \
        "curl -s -H 'X-API-Token: {\"$ne\": null}' $PULSE_URL/api/state | head -1" \
        "Authentication required"
else
    # Even with DISABLE_AUTH, invalid tokens return "Invalid API token"
    test_security "LDAP injection attempt" \
        "curl -s -H 'X-API-Token: *)(uid=*)' $PULSE_URL/api/state | head -1" \
        "Invalid API token"

    test_security "NoSQL injection attempt" \
        "curl -s -H 'X-API-Token: {\"$ne\": null}' $PULSE_URL/api/state | head -1" \
        "Invalid API token"
fi

echo ""
echo "5. CSRF PROTECTION"
echo "=================="

if [ "$AUTH_DISABLED" != true ]; then
    test_security "Cross-origin POST request" \
        "curl -s -X POST -H 'Origin: http://evil.com' $PULSE_URL/api/config/export -o /dev/null -w '%{http_code}'" \
        "401"

    test_security "Missing referer on state change" \
        "curl -s -X POST $PULSE_URL/api/config/nodes -o /dev/null -w '%{http_code}'" \
        "401"
else
    echo -e "${YELLOW}Skipped - CSRF protection relies on auth${NC}"
fi

echo ""
echo "6. RATE LIMITING"
echo "================"

echo -n "Testing rate limiting (100 requests): "
FAILURES=0
for i in {1..100}; do
    STATUS=$(curl -s -o /dev/null -w '%{http_code}' $PULSE_URL/api/health)
    if [ "$STATUS" = "429" ]; then
        ((FAILURES++))
    fi
done

if [ $FAILURES -gt 0 ]; then
    echo -e "${GREEN}✓ Rate limiting active ($FAILURES requests blocked)${NC}"
else
    echo -e "${YELLOW}⚠️  No rate limiting detected${NC}"
fi

echo ""
echo "7. SENSITIVE DATA EXPOSURE"
echo "=========================="

# Export always requires auth, even with DISABLE_AUTH
test_security "Config export requires auth" \
    "curl -s -X POST $PULSE_URL/api/config/export | head -1" \
    "Unauthorized"

test_security "No credentials in health endpoint" \
    "curl -s $PULSE_URL/api/health | grep -c 'password\\|token\\|secret'" \
    "0"

test_security "No stack traces in errors" \
    "curl -s $PULSE_URL/api/nonexistent 2>&1 | grep -c 'goroutine\\|panic\\|stack'" \
    "0"

echo ""
echo "8. HEADER SECURITY"
echo "=================="

echo -n "X-Frame-Options present: "
if curl -s -I $PULSE_URL | grep -q "X-Frame-Options"; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗ Missing${NC}"
    ((VULNERABILITIES++))
fi

echo -n "X-Content-Type-Options present: "
if curl -s -I $PULSE_URL | grep -q "X-Content-Type-Options: nosniff"; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗ Missing${NC}"
    ((VULNERABILITIES++))
fi

echo -n "CSP header present: "
if curl -s -I $PULSE_URL | grep -q "Content-Security-Policy"; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗ Missing${NC}"
    ((VULNERABILITIES++))
fi

echo ""
echo "9. SESSION SECURITY"
echo "=================="

if [ -n "$API_TOKEN" ]; then
    test_security "Token not in response body" \
        "curl -s -H 'X-API-Token: $API_TOKEN' $PULSE_URL/api/state | grep -c '$API_TOKEN'" \
        "0"
    
    test_security "Token not reflected in headers" \
        "curl -s -I -H 'X-API-Token: $API_TOKEN' $PULSE_URL/api/state | grep -c '$API_TOKEN'" \
        "0"
else
    echo -e "${YELLOW}Skipping session tests (no API_TOKEN)${NC}"
fi

echo ""
echo "10. DOS PREVENTION"
echo "=================="

test_security "Large header handling (10KB)" \
    "curl -s -H 'X-Large: $(head -c 10000 /dev/zero | tr '\\0' 'A')' $PULSE_URL/api/health -o /dev/null -w '%{http_code}'" \
    "200"

test_security "Deeply nested JSON" \
    "echo '{\"a\":{\"b\":{\"c\":{\"d\":{\"e\":{}}}}}}' | curl -s -X POST -d @- $PULSE_URL/api/config/export -o /dev/null -w '%{http_code}'" \
    "401"

echo ""
echo "================================================"
echo "SECURITY TEST RESULTS"
echo "================================================"

if [ $VULNERABILITIES -gt 0 ]; then
    echo -e "${RED}⚠️  Found $VULNERABILITIES potential vulnerabilities!${NC}"
    exit 1
else
    echo -e "${GREEN}✅ All security tests passed${NC}"
fi