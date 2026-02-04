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

## Alert System Reliability Audit

### Executive Summary
A focused audit of the Alert System was conducted to identify reliability issues such as stale alerts and incorrect clearing logic. Critical bugs causing "zombie interrupts" (stale alerts that never clear) were identified and fixed.

### Findings & Fixes

### 1. Stale Alerts on Node/Host Offline
- **Problem**: When a Proxmox Node or Pulse Host Agent went offline, the system correctly raised a connectivity alert but failed to clear existing resource alerts (High CPU/Memory/Disk). This resulted in contradictory states (e.g., "Node Offline" and "High CPU" simultaneously).
- **Fix**: Updated `CheckNode` and `HandleHostOffline` to explicitly clear all resource metric alerts when an offline state is confirmed.

### 2. Stale Alerts on Disabled Thresholds
- **Problem**: For optional metrics (Guest Disk I/O, Network I/O, Node/Host Temperature, Disk Usage with Overrides), the system skipped the evaluation logic entirely if the threshold was disabled or nil. This prevented the clean-up logic from running, causing existing alerts to persist indefinitely after a rule was disabled.
- **Fix**: Refactored the checking logic in `CheckGuest`, `CheckNode`, and `CheckHost` to execute unconditionally. The underlying `checkMetric` function now properly handles disabled thresholds by clearing any corresponding active alerts.

### 3. Missing Clear Logic for Global Disable
- **Problem**: Disabling global alert settings (e.g., "Disable all Host alerts") sometimes left specific metric alerts active if they were not explicitly cleared during the state transition.
- **Fix**: Verified and reinforced clearing mechanisms. Specifically, `CheckHost` disk monitoring was updated to ensure alerts are cleared even when specific disk overrides disable monitoring.

### Conclusion
The reliability of the alert system has been significantly improved. Alerts will now correctly reflect the current state of resources, and disabling rules will reliably clear associated alerts.
