# Iframe Embedding Guide

Pulse supports embedding in iframes (like Homepage dashboards) with flexible authentication options.

## Configuration

### Basic Iframe Settings

```bash
# Enable iframe embedding
ALLOW_EMBEDDING=true

# Specify allowed origins (comma-separated)
ALLOWED_EMBED_ORIGINS=http://homepage.lan:3000,http://192.168.0.171:3000
```

### Advanced Security Options

All settings below can be configured through the web UI (Settings > Advanced > Security > Advanced Security Options):

```bash
# Session timeout (hours) - how long users stay logged in
SESSION_TIMEOUT_HOURS=24

```

**Note:** Cookie SameSite policy is automatically configured based on your embedding settings. When embedding is enabled, Pulse will use the appropriate SameSite policy for your environment.
- `lax` - Good balance, works with iframes on same site
- `none` - Required for cross-origin iframes (HTTPS only)

### Authentication Options

#### Option 1: Manual Login (Default - Most Secure)
Users log in once within the iframe, and the session persists.

```bash
# No special configuration needed - this is the default behavior
```

**How it works:**
1. When the iframe loads, if not authenticated, it redirects to the login page
2. User enters credentials in the login form within the iframe
3. Session cookie is set with appropriate SameSite settings for cross-origin access
4. After login, user is redirected back to the dashboard
5. Session persists for 24 hours (default)

**Pros:**
- Most secure - full authentication required
- Session persists after login
- All features available
- Works across different origins/subdomains

**Cons:**
- User must log in at least once per session
- Shows login page initially

#### Option 2: Automatic Read-Only Access (Coming Soon)
Automatically grants read-only access from allowed origins.

```bash
IFRAME_AUTH_MODE=auto  # Grants viewer access from allowed origins
```

**Pros:**
- No login required from trusted origins
- Seamless iframe experience
- Still secure for write operations

**Cons:**
- Only read access from iframes
- Requires trusted network

## Homepage Integration

### For Manual Login Mode:
```yaml
# In Homepage services.yaml
- Pulse:
    icon: mdi-pulse
    href: http://192.168.0.122:7655
    description: Proxmox Monitoring
    widget:
      type: iframe
      name: Pulse Dashboard
      src: http://192.168.0.122:7655
      classes: h-96  # Adjust height
      refreshInterval: 30000  # 30 seconds
```

**First Time Setup:**
1. Click on the Pulse widget
2. Log in with your credentials
3. The session will persist (24 hours by default)

### Tips:
- Use a dedicated browser/profile for your dashboard to keep sessions
- Consider using a password manager for quick login
- Sessions persist across browser restarts

## Security Considerations

1. **Always use HTTPS in production** for secure session cookies
2. **Limit allowed origins** to only trusted sources
3. **Use strong admin passwords** since iframes have full access after login
4. **Monitor audit logs** for iframe access patterns

## Troubleshooting

**Iframe shows "refused to connect":**
- Check `ALLOW_EMBEDDING=true` is set
- Verify the origin is in `ALLOWED_EMBED_ORIGINS`
- Check browser console for CSP violations
- Restart Pulse service after changing settings

**401 Unauthorized errors in iframe:**
- This is normal on first load - the page should redirect to login
- If redirect doesn't happen, check browser console for errors
- Ensure JavaScript is enabled in the iframe

**Session doesn't persist:**
- Check cookie settings in browser (third-party cookies must be allowed)
- For HTTP environments, cookies use SameSite=Lax
- For HTTPS production, cookies use SameSite=None with Secure flag
- Browser privacy settings may block cross-origin cookies

**Login page doesn't show in iframe:**
- Clear browser cache
- Check if `/login.html` is accessible directly
- Look for JavaScript errors in browser console

**Can't see anything in iframe:**
- Check if you're logged in (should see login page if not)
- Verify Pulse is accessible at the URL
- Check for mixed content warnings (HTTP/HTTPS)
- Some browsers block HTTP iframes on HTTPS pages