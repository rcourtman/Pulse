#!/bin/bash

# Test script for proxy auth admin permissions
# This simulates what Authentik/Caddy would send

BASE_URL="http://localhost:7656"
PROXY_SECRET="test-secret-123"

echo "=== Testing Proxy Auth Admin Permissions ==="
echo

# First, update the config to enable proxy auth
echo "1. Setting up proxy auth configuration..."
cat > /tmp/proxy-test.env << EOF
PROXY_AUTH_SECRET=$PROXY_SECRET
PROXY_AUTH_USER_HEADER=X-Authentik-Username
PROXY_AUTH_ROLE_HEADER=X-Authentik-Groups
PROXY_AUTH_ADMIN_ROLE=admin
PROXY_AUTH_ROLE_SEPARATOR=|
EOF

# Copy current env and add proxy auth settings
cp /etc/pulse/.env /tmp/backup.env
cat /tmp/proxy-test.env >> /etc/pulse/.env

# Restart service to pick up new config
echo "2. Restarting service with proxy auth enabled..."
sudo systemctl restart pulse-dev
sleep 5

echo "3. Testing API endpoints with different user roles..."
echo

# Test as non-admin user (should be blocked from write operations)
echo "=== Testing as non-admin user (alice) ==="
echo "Groups: users|staff (no admin role)"
echo

echo -n "GET /api/security/status (should work): "
curl -s -X GET "$BASE_URL/api/security/status" \
  -H "X-Proxy-Secret: $PROXY_SECRET" \
  -H "X-Authentik-Username: alice" \
  -H "X-Authentik-Groups: users|staff" \
  | jq -r '.proxyAuthIsAdmin // "ERROR"'

echo -n "GET /api/config/nodes (should work): "
curl -s -X GET "$BASE_URL/api/config/nodes" \
  -H "X-Proxy-Secret: $PROXY_SECRET" \
  -H "X-Authentik-Username: alice" \
  -H "X-Authentik-Groups: users|staff" \
  -o /dev/null -w "%{http_code}\n"

echo -n "POST /api/config/nodes (should be 403 Forbidden): "
curl -s -X POST "$BASE_URL/api/config/nodes" \
  -H "X-Proxy-Secret: $PROXY_SECRET" \
  -H "X-Authentik-Username: alice" \
  -H "X-Authentik-Groups: users|staff" \
  -H "Content-Type: application/json" \
  -d '{"name":"test","host":"192.168.1.1","type":"pve"}' \
  -o /dev/null -w "%{http_code}\n"

echo -n "POST /api/system/settings/update (should be 403 Forbidden): "
curl -s -X POST "$BASE_URL/api/system/settings/update" \
  -H "X-Proxy-Secret: $PROXY_SECRET" \
  -H "X-Authentik-Username: alice" \
  -H "X-Authentik-Groups: users|staff" \
  -H "Content-Type: application/json" \
  -d '{"pollingInterval":30}' \
  -o /dev/null -w "%{http_code}\n"

echo -n "POST /api/config/export (should be 403 Forbidden): "
curl -s -X POST "$BASE_URL/api/config/export" \
  -H "X-Proxy-Secret: $PROXY_SECRET" \
  -H "X-Authentik-Username: alice" \
  -H "X-Authentik-Groups: users|staff" \
  -H "Content-Type: application/json" \
  -d '{"passphrase":"test123456789"}' \
  -o /dev/null -w "%{http_code}\n"

echo
echo "=== Testing as admin user (bob) ==="
echo "Groups: users|staff|admin (has admin role)"
echo

echo -n "GET /api/security/status (should work): "
curl -s -X GET "$BASE_URL/api/security/status" \
  -H "X-Proxy-Secret: $PROXY_SECRET" \
  -H "X-Authentik-Username: bob" \
  -H "X-Authentik-Groups: users|staff|admin" \
  | jq -r '.proxyAuthIsAdmin // "ERROR"'

echo -n "POST /api/config/nodes (should work - 400 due to incomplete data): "
curl -s -X POST "$BASE_URL/api/config/nodes" \
  -H "X-Proxy-Secret: $PROXY_SECRET" \
  -H "X-Authentik-Username: bob" \
  -H "X-Authentik-Groups: users|staff|admin" \
  -H "Content-Type: application/json" \
  -d '{"name":"test","host":"192.168.1.1","type":"pve"}' \
  -o /dev/null -w "%{http_code}\n"

echo -n "POST /api/system/settings/update (should work - 200): "
curl -s -X POST "$BASE_URL/api/system/settings/update" \
  -H "X-Proxy-Secret: $PROXY_SECRET" \
  -H "X-Authentik-Username: bob" \
  -H "X-Authentik-Groups: users|staff|admin" \
  -H "Content-Type: application/json" \
  -d '{"darkMode":true}' \
  -o /dev/null -w "%{http_code}\n"

echo -n "POST /api/config/export (should work - 200): "
curl -s -X POST "$BASE_URL/api/config/export" \
  -H "X-Proxy-Secret: $PROXY_SECRET" \
  -H "X-Authentik-Username: bob" \
  -H "X-Authentik-Groups: users|staff|admin" \
  -H "Content-Type: application/json" \
  -d '{"passphrase":"test123456789"}' \
  -o /dev/null -w "%{http_code}\n"

echo
echo "=== Testing without proxy auth (should fail) ==="
echo

echo -n "POST /api/config/nodes (should be 401 Unauthorized): "
curl -s -X POST "$BASE_URL/api/config/nodes" \
  -H "Content-Type: application/json" \
  -d '{"name":"test","host":"192.168.1.1","type":"pve"}' \
  -o /dev/null -w "%{http_code}\n"

# Restore original config
echo
echo "4. Restoring original configuration..."
mv /tmp/backup.env /etc/pulse/.env
sudo systemctl restart pulse-dev

echo
echo "=== Test Complete ==="
echo "Summary:"
echo "- Non-admin users should get 403 Forbidden on write operations"
echo "- Admin users should be able to perform all operations"
echo "- Users without proxy auth should get 401 Unauthorized"