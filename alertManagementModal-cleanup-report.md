# Alert Management Modal - Duplicate & Unused Code Report

## 1. Duplicate Functions Found

### Email Test Functions
- **`testEmailConfiguration()`** (line 1288) - Uses `/api/alerts/test-email` endpoint
- **`testEmailConnection()`** (line 2775) - Uses `/api/test-email` endpoint
- Both functions do the same thing but use different API endpoints
- `setupEmailTestButton()` at line 1281 sets up listener for `testEmailConfiguration`
- Direct event listener at line 1858 sets up listener for `testEmailConnection`

### Webhook Save Functions  
- **`saveWebhookConfiguration()`** at line 2566 - Stub function with TODO comment
- **`saveWebhookConfiguration()`** at line 2891 - Full implementation
- First one is never used, second one is the actual implementation

## 2. Unused Code

### Functions That Reference Non-Existent Elements
- `handleWebhookPreset()` (line 2545) references `webhook-url-input` element
- `saveWebhookConfiguration()` (line 2567) references `webhook-url-input` element  
- `testWebhookConnection()` (line 2947) references `webhook-url-input` element
- **But the actual input has `name="WEBHOOK_URL"` not `id="webhook-url-input"`**

## 3. Duplicate Event Handler Registration
- Email test button gets event listener added twice:
  - Once in `setupEmailTestButton()` (line 1284)
  - Once directly in `initializeConfigureTab()` flow (line 1858)

## 4. Functions Exposed But May Not Need Global Access
These functions are exposed globally but might only need local scope:
- `deleteCustomAlert`
- `openCustomAlertModal`
- `showAlertDetails`
- `toggleAlert`
- `addWebhookEndpoint`
- `removeWebhookEndpoint`
- `handleEmailProviderSelection`
- `switchTab`

## 5. Recommendations

### Remove Duplicates
1. Remove the stub `saveWebhookConfiguration()` at line 2566
2. Choose one email test function and remove the other
3. Fix webhook functions to use correct selector: `input[name="WEBHOOK_URL"]`

### Fix Element References
1. Update all references from `webhook-url-input` to use the correct selector
2. Remove duplicate event listener registrations

### Code Organization
1. Consider if all exposed functions really need global access
2. Consolidate similar functionality into single functions