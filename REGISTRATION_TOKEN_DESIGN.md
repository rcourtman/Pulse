# Registration Token Design for Secure Auto-Registration

## Current Security Issues
- Auto-registration endpoint is open by default
- Tokens sent in plaintext over HTTP
- No way to limit who can register nodes

## Proposed Solution: Registration Tokens

### How it works:

1. **User generates registration token in UI**:
   - Click "Generate Registration Token" in Settings → Nodes
   - Options:
     - Validity: 15 min, 1 hour, 24 hours
     - Max uses: 1, 5, unlimited
     - Allowed node types: PVE, PBS, or both
   
2. **Token structure**:
   ```
   PULSE-REG-[random]-[checksum]
   Example: PULSE-REG-x7k9m2p4-a3
   ```

3. **Backend stores**:
   ```json
   {
     "token": "PULSE-REG-x7k9m2p4-a3",
     "created": "2025-01-09T10:00:00Z",
     "expires": "2025-01-09T10:15:00Z",
     "maxUses": 1,
     "usedCount": 0,
     "allowedTypes": ["pve", "pbs"],
     "createdBy": "admin-ip-address"
   }
   ```

4. **Setup script changes**:
   ```bash
   # User provides registration token
   read -p "Enter Pulse registration token: " REG_TOKEN
   
   # Send with registration
   curl -X POST "$PULSE_URL/api/auto-register" \
     -H "X-Registration-Token: $REG_TOKEN" \
     -H "Content-Type: application/json" \
     -d "$REGISTER_JSON"
   ```

5. **Security benefits**:
   - Time-limited access
   - Usage limits
   - Revocable
   - Audit trail
   - No permanent credentials needed
   - Can work alongside API tokens for extra security

## Migration Path

### Phase 1: Optional (current + tokens)
- Keep current open endpoint
- Add registration token as optional security
- Environment variable to require tokens

### Phase 2: Default secure (tokens required)
- Require registration tokens by default
- Add bypass flag for homelab users
- Clear migration documentation

### Phase 3: Enhanced security
- Token encryption
- Token signing with HMAC
- Token delegation (tokens that can create tokens)

## Comparison with Existing Models

### Kubernetes Join Tokens
- Similar time-limited approach
- We simplify by not requiring CA certs

### GitHub Personal Access Tokens
- Similar UI for generation
- We add usage limits

### Export/Import Model
- Uses API token + passphrase
- Registration tokens are simpler (single token)
- Both can coexist

## Implementation Priority
1. Basic token generation and validation
2. UI for token management
3. Audit logging
4. Token encryption (optional)

## Configuration Examples

### High Security Environment
```bash
# Require both API token AND registration token
API_TOKEN=secret-api-token
REQUIRE_REGISTRATION_TOKEN=true
REGISTRATION_TOKEN_DEFAULT_VALIDITY=900  # 15 minutes
REGISTRATION_TOKEN_DEFAULT_MAX_USES=1
```

### Homelab Environment
```bash
# Optional tokens, longer validity
REQUIRE_REGISTRATION_TOKEN=false
REGISTRATION_TOKEN_DEFAULT_VALIDITY=86400  # 24 hours
REGISTRATION_TOKEN_DEFAULT_MAX_USES=10
```

### Development Environment
```bash
# Bypass all security
ALLOW_UNPROTECTED_AUTO_REGISTER=true
```