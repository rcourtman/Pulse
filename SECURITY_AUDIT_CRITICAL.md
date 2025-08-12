# Pulse Security Audit - Critical Assessment
Date: August 12, 2025

## Critical Context
**Pulse stores Proxmox API tokens with WRITE permissions** - These credentials can destroy entire infrastructures if compromised.

## Current Security Posture

### What's Protected Well ✅
1. **Credentials at rest** - AES-256-GCM encryption
2. **Export/Import** - Requires API token + passphrase (PBKDF2 100k iterations)
3. **Frontend** - Never receives actual credentials, only `hasToken: true`
4. **Logs** - Credentials masked with `***`

### Critical Vulnerabilities 🔴

#### 1. **NO AUTHENTICATION BY DEFAULT**
- **Risk**: CRITICAL
- **Issue**: Anyone with network access can view all metrics and node status
- **Impact**: Information disclosure, attack surface mapping
- **Fix**: Require authentication by default

#### 2. **API Token in Plain Text (in memory)**
- **Risk**: HIGH
- **Issue**: While encrypted at rest, the API token is decrypted and stored in memory
- **Impact**: Memory dumps could expose token
- **Fix**: Use proper session management

#### 3. **No Audit Logging**
- **Risk**: HIGH
- **Issue**: No record of who accessed credentials or made changes
- **Impact**: Cannot detect or investigate breaches
- **Fix**: Implement comprehensive audit logging

#### 4. **Registration Tokens - Poorly Secured**
- **Risk**: MEDIUM
- **Issue**: If someone gets a registration token, they can add rogue nodes
- **Impact**: Unauthorized node registration
- **Current Mitigation**: Tokens expire and have use limits

#### 5. **No Rate Limiting**
- **Risk**: MEDIUM
- **Issue**: Brute force attacks possible on API endpoints
- **Impact**: Potential credential stuffing/brute force
- **Fix**: Implement rate limiting

#### 6. **Single Master Key**
- **Risk**: MEDIUM
- **Issue**: One key encrypts everything
- **Impact**: Single point of failure
- **Mitigation**: Export/import uses separate passphrase

## Immediate Security Recommendations

### 1. Add Default Authentication 🔴 CRITICAL
```bash
# Should be DEFAULT behavior:
Environment="REQUIRE_AUTH=true"
Environment="DEFAULT_USER=admin"
Environment="DEFAULT_PASSWORD=<generated-on-install>"
```

### 2. Implement Audit Logging 🔴 CRITICAL
```json
{
  "timestamp": "2025-08-12T20:00:00Z",
  "user": "admin",
  "action": "VIEW_CREDENTIALS",
  "resource": "pve-node-1",
  "ip": "192.168.1.100"
}
```

### 3. Add Session Management 🟡 HIGH
- Replace persistent API tokens with sessions
- Implement timeout/expiry
- Require re-authentication for sensitive operations

### 4. Credential Rotation Reminders 🟡 HIGH
- Track credential age
- Remind users to rotate Proxmox tokens
- Support multiple tokens per node

### 5. Network Segmentation Guide 🟡 HIGH
Document best practices:
- Run Pulse on management VLAN only
- Use firewall rules
- Restrict access to trusted IPs

## Security Features That Should Be DEFAULT

1. **Authentication Required** - Not optional
2. **HTTPS Only** - Warn loudly on HTTP
3. **Audit Logs** - Always on
4. **Session Timeout** - 30 minutes default
5. **Failed Login Lockout** - After 5 attempts

## Comparison to Industry Standards

| Feature | Pulse | Industry Standard | Risk |
|---------|-------|------------------|------|
| Default Auth | ❌ No | ✅ Yes | CRITICAL |
| Audit Logs | ❌ No | ✅ Yes | HIGH |
| Rate Limiting | ❌ No | ✅ Yes | MEDIUM |
| Session Mgmt | ❌ No | ✅ Yes | HIGH |
| RBAC | ❌ No | ✅ Yes | MEDIUM |
| MFA | ❌ No | ✅ Yes | LOW (for homelab) |

## The Registration Token Problem

The feature exists but is poorly integrated:
- **Purpose**: Allow nodes to self-register securely
- **Problem**: No actual auto-registration endpoint/workflow
- **Result**: Feature exists but serves no purpose

Either:
1. **Complete the feature** - Add auto-registration endpoint
2. **Remove it** - Reduce complexity

## Recommended Security Defaults

```yaml
# This should be the DEFAULT configuration:
security:
  authentication:
    enabled: true          # Not optional
    default_user: admin    
    require_password_change: true
  
  api:
    require_https: true    # Warn if not HTTPS
    rate_limit: 100/min
    session_timeout: 30m
  
  audit:
    enabled: true          # Always on
    retention: 90d
    include_reads: false   # Only writes by default
  
  credentials:
    rotation_reminder: 90d
    encryption: AES-256-GCM
    export_requires_token: true
```

## Conclusion

Pulse handles **extremely sensitive credentials** but treats security as optional. For an app managing infrastructure credentials:

1. **Authentication should be mandatory**, not optional
2. **Audit logging is essential**, not a nice-to-have  
3. **Session management is standard**, not complex

The current security is adequate for a completely trusted, isolated homelab network. But given that Pulse stores credentials that can **destroy entire Proxmox clusters**, security should be taken much more seriously.

## Priority Actions

1. 🔴 **CRITICAL**: Make authentication mandatory by default
2. 🔴 **CRITICAL**: Add audit logging for all credential access
3. 🟡 **HIGH**: Implement proper session management
4. 🟡 **HIGH**: Add rate limiting
5. 🟢 **MEDIUM**: Complete or remove registration tokens feature

The question isn't "does it work?" but "is it secure enough for infrastructure credentials?" Currently: **Not by default**.