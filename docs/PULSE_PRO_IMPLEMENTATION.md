# Pulse Pro Implementation Plan

**Goal**: Gate AI features behind a Pro license to create a sustainable income stream.

**Timeline**: ~1-2 weeks of focused work

---

## Phase 1: License System Architecture

### 1.1 License Format (Simple JWT)

```go
// internal/license/license.go
type LicenseData struct {
    LicenseID    string    `json:"lid"`        // Unique license ID
    Email        string    `json:"email"`      // Customer email
    Tier         string    `json:"tier"`       // "pro", "msp", "enterprise"
    IssuedAt     time.Time `json:"iat"`
    ExpiresAt    time.Time `json:"exp"`        // Empty = lifetime
    MaxNodes     int       `json:"max_nodes"`  // 0 = unlimited
    Features     []string  `json:"features"`   // ["ai_chat", "ai_patrol", "ai_alerts"]
}
```

**Why JWT?**
- Self-contained (no license server needed for validation)
- Signed with your private key, verified with public key embedded in binary
- Can be verified offline (important for air-gapped homelabs)
- Standard format, easy to generate from any payment processor webhook

### 1.2 License Validation Flow

```
┌────────────────────────────────────────────────────────────────┐
│  User purchases license on LemonSqueezy/Gumroad                │
│                         ↓                                      │
│  Webhook hits your simple license API (can be CloudFlare Worker)│
│                         ↓                                      │
│  Generate JWT signed with private key                          │
│                         ↓                                      │
│  Email license key to customer                                 │
│                         ↓                                      │
│  User pastes key in Pulse Settings → Pro tab                   │
│                         ↓                                      │
│  Pulse validates signature with embedded public key            │
│                         ↓                                      │
│  Store encrypted license in config dir (license.enc)           │
└────────────────────────────────────────────────────────────────┘
```

---

## Phase 2: Backend Implementation

### 2.1 New Files to Create

```
internal/license/
├── license.go       # License struct, validation, JWT parsing
├── license_test.go  # Tests
└── features.go      # Feature flags (what Pro includes)

internal/config/
└── license.go       # License persistence (store/load from disk)
```

### 2.2 License Service

```go
// internal/license/license.go
package license

import (
    "crypto/ed25519"
    "encoding/base64"
    "errors"
    "time"
    
    "github.com/golang-jwt/jwt/v5"
)

// Embedded public key (compiled into binary)
// Generate keypair: go run ./cmd/license-keygen
var publicKeyBase64 = "YOUR_PUBLIC_KEY_HERE"

type Service struct {
    license  *LicenseData
    loaded   bool
}

func NewService() *Service {
    return &Service{}
}

func (s *Service) LoadFromKey(licenseKey string) error {
    // Parse and validate JWT
    // Store in s.license
}

func (s *Service) IsValid() bool {
    if s.license == nil {
        return false
    }
    if !s.license.ExpiresAt.IsZero() && time.Now().After(s.license.ExpiresAt) {
        return false
    }
    return true
}

func (s *Service) HasFeature(feature string) bool {
    if !s.IsValid() {
        return false
    }
    for _, f := range s.license.Features {
        if f == feature || f == "all" {
            return true
        }
    }
    return false
}

// Feature constants
const (
    FeatureAIChat       = "ai_chat"
    FeatureAIPatrol     = "ai_patrol"
    FeatureAIAlerts     = "ai_alerts"
    FeatureOIDC         = "oidc"           // SSO/OIDC authentication
    FeatureKubernetes   = "kubernetes"     // K8s cluster monitoring
    FeatureMultiUser    = "multi_user"     // Multiple user accounts
    FeatureAPIAccess    = "api_access"     // Full API access for integrations
    FeatureWhiteLabel   = "white_label"    // Custom branding (MSP tier)
    FeatureAll          = "all"
)
```

### 2.3 Integration Points

Modify these files to check license:

| File | What to Gate |
|------|--------------|
| `internal/api/ai_handlers.go` | Chat endpoints, patrol endpoints |
| `internal/ai/patrol.go` | Patrol service start |
| `internal/ai/service.go` | AI chat service |
| `internal/ai/alert_triggered.go` | Alert analysis |
| `internal/api/oidc_handlers.go` | OIDC/SSO configuration |
| `internal/api/kubernetes_handlers.go` | K8s cluster endpoints |
| `internal/monitoring/kubernetes/` | K8s monitoring service |

**Example gating in ai_handlers.go:**

```go
func (h *AISettingsHandler) HandleChat(w http.ResponseWriter, r *http.Request) {
    // Check Pro license
    if !h.licenseService.HasFeature(license.FeatureAIChat) {
        utils.WriteJSONError(w, http.StatusPaymentRequired, 
            "AI Chat requires Pulse Pro. Visit https://pulserelay.pro to upgrade.")
        return
    }
    // ... existing logic
}
```

---

## Phase 3: Frontend Implementation

### 3.1 New Settings Tab: "Pro License"

Location: `frontend-modern/src/routes/settings/+page.svelte` (or equivalent)

```
┌──────────────────────────────────────────────────────────────┐
│  ⚡ Pulse Pro                                                 │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  License Key                                           │  │
│  │  [________________________________________________]    │  │
│  │  [Activate License]                                    │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ✅ License Status: Active (Pro)                             │
│  📧 Licensed to: user@example.com                            │
│  📅 Expires: Never (Lifetime)                                │
│                                                              │
│  ─────────────────────────────────────────────────────────  │
│                                                              │
│  Included Features:                                          │
│  ✅ AI Chat Assistant                                        │
│  ✅ AI Patrol (Background Health Checks)                     │
│  ✅ AI Alert Analysis                                        │
│  ✅ Priority Support                                         │
│                                                              │
│  ─────────────────────────────────────────────────────────  │
│                                                              │
│  Don't have a license?                                       │
│  [Get Pulse Pro →] https://pulserelay.pro                   │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 3.2 Graceful Degradation for Unlicensed Users

When AI features are accessed without a license:

- **AI Settings Tab**: Show features but disabled with "Upgrade to Pro" message
- **Chat Button**: Show but with "Pro" badge, clicking prompts upgrade
- **Patrol Findings**: Hide or show "Enable with Pro" placeholder

**Don't be hostile.** The free version should still feel complete. Pro is an enhancement, not a hostage situation.

---

## Phase 4: Payment & License Generation

### 4.1 Payment Processor: LemonSqueezy

**Why LemonSqueezy over alternatives?**
- Handles global VAT/sales tax automatically
- Generates invoices (enterprises need this)
- Good webhook support for automation
- Reasonable fees (~5% + 50¢)
- Supports both subscription and one-time payments

### 4.2 Pricing Structure (Suggested)

| Tier | Price | Features | Target |
|------|-------|----------|--------|
| **Pro Monthly** | $12/month | AI features, OIDC/SSO, K8s monitoring | Individuals |
| **Pro Annual** | $99/year | Same as monthly, 2 months free | Power users |
| **Pro Lifetime** | $249 one-time | All Pro features, forever | Homelabbers who hate subscriptions |
| **MSP** | $49/month | All Pro + unlimited instances, white-label, multi-tenant | MSPs |
| **Enterprise** | Custom | All features + support SLA, on-prem license server | Large orgs |

### 4.3 Feature Matrix

| Feature | Free | Pro | MSP | Enterprise |
|---------|------|-----|-----|------------|
| Proxmox VE/PBS/PMG monitoring | ✅ | ✅ | ✅ | ✅ |
| Docker/Podman monitoring | ✅ | ✅ | ✅ | ✅ |
| Alerts (Discord, Slack, etc.) | ✅ | ✅ | ✅ | ✅ |
| Metrics history | ✅ | ✅ | ✅ | ✅ |
| Backup explorer | ✅ | ✅ | ✅ | ✅ |
| **AI Chat** | ❌ | ✅ | ✅ | ✅ |
| **AI Patrol** | ❌ | ✅ | ✅ | ✅ |
| **AI Alert Analysis** | ❌ | ✅ | ✅ | ✅ |
| **OIDC/SSO** | ❌ | ✅ | ✅ | ✅ |
| **Kubernetes monitoring** | ❌ | ✅ | ✅ | ✅ |
| Unlimited instances | ❌ | ❌ | ✅ | ✅ |
| White-label branding | ❌ | ❌ | ✅ | ✅ |
| Multi-tenant mode | ❌ | ❌ | ✅ | ✅ |
| Priority support | ❌ | Email | Email | Dedicated |
| SLA | ❌ | ❌ | ❌ | ✅ |

### 4.3 License Generation Service

A simple Cloudflare Worker or Vercel Edge Function:

```javascript
// Simplified license generator (LemonSqueezy webhook handler)
addEventListener('fetch', event => {
  event.respondWith(handleRequest(event.request))
})

async function handleRequest(request) {
  const webhook = await request.json()
  
  if (webhook.meta.event_name === 'order_created') {
    const license = generateLicense({
      email: webhook.data.attributes.user_email,
      tier: 'pro',
      expiresAt: null, // lifetime for now
    })
    
    // Send license via email
    await sendLicenseEmail(webhook.data.attributes.user_email, license)
  }
  
  return new Response('OK')
}

function generateLicense(data) {
  // Sign JWT with private key
  // Return base64-encoded license key
}
```

---

## Phase 5: Launch Communication

### 5.1 Changelog Entry

```markdown
## v5.0.0 - The AI Update

### 🚀 Major Changes

**Introducing Pulse Pro**

Pulse 5.0 includes powerful AI features that require a Pro license:
- **AI Chat**: Natural language interface to your infrastructure
- **AI Patrol**: Background health monitoring and insights
- **AI Alert Analysis**: Smart analysis when alerts fire

Core monitoring features remain **completely free and open source**.

Pro licenses support ongoing development and enable me to work on Pulse full-time.

[Get Pulse Pro →](https://pulserelay.pro)

---

*Pulse has grown from a weekend project to something used by thousands. 
To keep improving it, I need to make it sustainable. Thank you for your support!*

— Richard
```

### 5.2 Preemptive FAQ

Add to README or docs:

**Q: Why are AI features paid?**
A: AI features require significant development effort and ongoing maintenance. Pro licenses let me work on Pulse sustainably while keeping core monitoring free.

**Q: Will monitoring features become paid?**
A: No. Proxmox/Docker/K8s monitoring, alerts, history, and all current free features will remain free forever.

**Q: What if I'm already using AI in the RC?**
A: Thank you for testing! RC users were beta testers helping shape these features. The final release requires a Pro license.

**Q: I can't afford Pro.**
A: Email me (richard@pulserelay.pro). I offer discounts for students, hobbyists in financial hardship, and open source contributors.

**Q: Can I self-host without Pro?**
A: Absolutely. Pulse works great without AI features. Pro is optional.

---

## Implementation Order

1. **Week 1: Backend**
   - [ ] Create `internal/license/` package
   - [ ] Implement JWT validation with embedded public key
   - [ ] Add license persistence (encrypted storage)
   - [ ] Gate AI endpoints with license checks
   - [ ] Add `/api/license` endpoints (check, activate)

2. **Week 2: Frontend + Payment**
   - [ ] Add Pro License settings tab
   - [ ] Update AI settings to show Pro-gated state
   - [ ] Set up LemonSqueezy product
   - [ ] Create license generation webhook
   - [ ] Set up pulserelay.pro landing page (can be simple)
   - [ ] Write announcement blog post

3. **Launch**
   - [ ] Release v5.0.0 stable
   - [ ] Post to Reddit (/r/homelab, /r/Proxmox, /r/selfhosted)
   - [ ] Post to GitHub Discussions
   - [ ] Email mailing list (if you have one)

---

## Security Considerations

1. **Private key**: Never commit to repo. Store in password manager + secure backup.
2. **License validation**: Always verify signature, never trust claims without verification.
3. **Obfuscation**: Consider light obfuscation of license check code (not for security, but to discourage trivial patching).
4. **Grace period**: If validation fails, maybe grant 7-day grace period before disabling (better UX).

---

## What NOT to Do

- ❌ Phone-home license validation (breaks air-gapped installs)
- ❌ Aggressive license enforcement (pisses off users)
- ❌ Remove free features to "encourage" upgrades
- ❌ Make the free version feel crippled
- ❌ Hide that it's paid (be upfront in README)

---

## Success Metrics (First 90 Days)

| Metric | Target |
|--------|--------|
| Pro licenses sold | 50-100 |
| Monthly revenue | $500-$1000 |
| Churn rate | <5% |
| Negative community reactions | <10 vocal complaints |
| GitHub stars lost | <50 |

If you hit these numbers, you've validated the model. Then you can expand to MSP tier, add features, etc.

---

*This plan can be adjusted based on your preferences. Want me to help implement any specific part?*
