# Security Improvement Plan - Balanced Approach

## Philosophy: "Secure by Default, Easy to Disable"

### Phase 1: Non-Breaking Improvements (No User Impact)

#### 1. Add Security Warning Banner
On first setup or when no auth is configured:
```
⚠️ Pulse is running without authentication. Your Proxmox credentials are accessible to anyone on your network.
→ Enable security in Settings → Security (Dismiss | Learn More | Enable Now)
```

#### 2. Audit Logging (Silent)
- Add audit.log file (off by default initially)
- Log sensitive operations when enabled
- No UI changes required
- Power users can enable via env var

#### 3. Security Score Dashboard
Add a small widget showing security posture:
```
Security Score: 2/5 ⚠️
✅ Credentials encrypted
✅ Export requires passphrase
❌ No authentication enabled
❌ No HTTPS configured  
❌ No audit logging
```

### Phase 2: Opt-in Security (User Choice)

#### Quick Security Setup Wizard
One-click security hardening:
```
"Secure My Pulse Instance" button that:
1. Generates a random password
2. Enables basic auth
3. Shows the credentials ONCE
4. Enables audit logging
5. Sets secure headers
```

#### Environment Variable Defaults
```bash
# For new installations via script:
PULSE_FIRST_RUN_SECURITY=prompt  # Ask user during install
PULSE_FIRST_RUN_SECURITY=auto    # Auto-enable with generated password
PULSE_FIRST_RUN_SECURITY=skip    # Current behavior (no auth)
```

### Phase 3: Smart Security (Context-Aware)

#### Auto-Detection
```javascript
if (accessedFromPublicIP || !isRFC1918Address) {
  showWarning("Pulse accessed from public network - authentication strongly recommended");
  offerQuickSetup();
}
```

#### Trusted Networks Option
```yaml
security:
  trusted_networks:
    - 192.168.1.0/24  # No auth required
    - 10.0.0.0/24     # No auth required
  require_auth_outside: true
```

### Implementation Priority

#### 1. Start with Warnings (v4.3.2)
- [ ] Add security warning banner
- [ ] Add security score widget
- [ ] Add "Learn More" documentation

#### 2. Add Easy Opt-in (v4.4.0)
- [ ] Quick security wizard
- [ ] One-click hardening
- [ ] Generated passwords with QR codes

#### 3. Smart Defaults (v5.0.0)
- [ ] Prompt during installation
- [ ] Context-aware warnings
- [ ] Trusted network configuration

## User Communication Strategy

### For Existing Users (Upgrades)
```
What's New in v4.3.2:
- Security improvements (all optional!)
- New security score indicator
- Quick setup wizard for those who want it
- YOUR SETUP UNCHANGED - No action required
```

### For New Users (Fresh Install)
```
Welcome to Pulse!

How would you like to configure security?
[ ] Recommended - Enable authentication (generates password)
[ ] Homelab - Warning only (can enable later)
[ ] Skip - I'll configure it myself

You can change this anytime in Settings → Security
```

### Documentation Approach

#### Three Tiers of Users

**1. "Just Make It Work" Users**
- No changes to current experience
- Can dismiss warnings
- Everything still works

**2. "Security Conscious" Users**
- One-click security button
- Clear documentation
- Reasonable defaults

**3. "Enterprise/MSP" Users**
- Full security features
- Audit logs
- Token management
- RBAC (future)

## Specific Non-Breaking Improvements

### 1. Add Security Headers (No User Impact)
```go
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("Content-Security-Policy", "default-src 'self'")
```

### 2. Add Rate Limiting (Invisible)
```go
// Generous limits that won't affect normal use
rateLimit := 1000  // requests per minute
burstLimit := 100  // burst capacity
```

### 3. Add HTTPS Detection
```javascript
if (window.location.protocol === 'http:' && !isLocalNetwork()) {
  showWarning("Pulse is running over HTTP. Consider using HTTPS for better security.");
}
```

### 4. Session Timeout (When Auth Enabled)
```javascript
// Only if user has enabled auth
if (authEnabled && idleTime > 30 * 60 * 1000) {
  showMessage("Session expired for your security. Please log in again.");
}
```

## The Key: Make Security EASY

### Bad (Current)
1. Read documentation
2. Set environment variables
3. Restart service
4. Configure tokens
5. Test everything

### Good (Proposed)
1. Click "Secure My Instance"
2. Save generated password
3. Done

## Migration Messages

### For Users Who Don't Want Auth
```
# In .env or systemd
PULSE_DISABLE_SECURITY_WARNINGS=true
```

### For Power Users
```
# Full control
PULSE_AUTH_REQUIRED=true
PULSE_AUDIT_LOG=true
PULSE_SESSION_TIMEOUT=1800
PULSE_RATE_LIMIT=100
```

## Success Metrics

- **No GitHub issues** about "forced authentication"
- **Increased security score** adoption over time
- **Some users** actually enable security
- **No breaking changes** for existing setups

## The Bottom Line

1. **Never force security** on existing users
2. **Make security trivially easy** to enable
3. **Educate without nagging**
4. **Reward security** (green checkmarks, score)
5. **Default secure** for new installs (with skip option)