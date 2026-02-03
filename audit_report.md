# Security Audit Report

## Executive Summary
A comprehensive security audit of the Pulse codebase was conducted, focusing on Remote Code Execution (RCE), Server-Side Request Forgery (SSRF), and Authentication mechanisms. Critical vulnerabilities identified in previous scans (Apprise RCE, Webhook SSRF) have been verified as fixed. No new critical vulnerabilities were found during this final review.

## Audit Findings

### 1. Apprise RCE (Fixed)
- **Problem**: The `CLIPath` parameter in Apprise configuration was user-controllable and passed directly to `exec.Command`, allowing arbitrary command execution.
- **Fix**: The `CLIPath` is now hardcoded to `"apprise"` in `NormalizeAppriseConfig` (`internal/notifications/notifications.go`), preventing users from injecting malicious paths.
- **Verification**: Code review confirms `CLIPath` is sanitized before use.

### 2. Webhook SSRF & DNS Rebinding (Fixed)
- **Problem**: Webhook URL validation checked the initial IP but was vulnerable to Time-of-Check Time-of-Use (TOCTOU) DNS rebinding attacks.
- **Fix**: A custom `http.Transport` was implemented in `createSecureWebhookClient` (`internal/notifications/notifications.go`) that pins the DNS resolution to the validated IP address. Redirects are also re-validated.
- **Verification**: The `DialContext` ensures the connection is made to the specific IP address that passed validation.

### 3. Agent Execution Security
- **Analysis**: The `agentexec` WebSocket endpoint (`/api/agent/ws`) allows connections from any origin (`CheckOrigin: true`).
- **Risk Assessment**: **Low**. Authentication is performed via the initial `agent_register` message payload containing the token. Browser-based attacks (CSWSH) are not effective because they cannot inject the token into the WebSocket message payload.
- **Recommendations**: No immediate action required.

### 4. Admin Test Connection SSRF
- **Analysis**: The `HandleTestConnection` endpoint allows admins to connect to arbitrary hosts (Proxmox/PBS/PMG) to verify configuration.
- **Risk Assessment**: **Accepted Risk**. This is an intended feature for administrators. While it allows an authenticated admin to probe internal network ports, it is necessary for the application's function.
- **Recommendations**: Ensure `PULSE_TRUSTED_PROXY_CIDRS` is configured if running in a sensitive environment to prevent IP spoofing, although unrelated to this specific feature.

### 5. Debug Endpoints
- **Analysis**: Checked for exposure of `pprof` or other debug handlers.
- **Findings**: No debug endpoints are exposed in the production router.

### 6. Authentication Bypass
- **Analysis**: Reviewed `adminBypassEnabled` logic.
- **Findings**: Bypass is strictly limited to development mode (`PULSE_DEV=true` or `NODE_ENV=development`) AND explicit opt-in (`ALLOW_ADMIN_BYPASS=1`). It cannot be accidentally enabled in production.

## Conclusion
The application security posture has been significantly improved with the remediation of the RCE and SSRF vulnerabilities. The remaining identified risks are low or accepted features. The application is ready for release from a security perspective.
