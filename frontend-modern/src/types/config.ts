/**
 * Configuration Type Definitions
 *
 * This file defines the types for Pulse's configuration structure.
 * Configuration is split into three files:
 *
 * 1. .env - Authentication credentials (AuthConfig)
 * 2. system.json - Application settings (SystemConfig)
 * 3. nodes.enc - Encrypted node credentials (NodesConfig)
 */

/**
 * Authentication configuration from .env file
 * These are environment variables for authentication ONLY
 */
export interface AuthConfig {
  PULSE_AUTH_USER: string; // Admin username
  PULSE_AUTH_PASS: string; // Bcrypt hashed password
}

/**
 * System settings from system.json file
 * These are application behavior settings
 */
export type UpdateChannel = 'stable' | 'rc';

export interface SystemConfig {
  pvePollingInterval?: number; // PVE polling interval in seconds
  pbsPollingInterval?: number; // PBS polling interval in seconds
  pmgPollingInterval?: number; // PMG polling interval in seconds
  connectionTimeout?: number; // Seconds before timeout (default: 10)
  autoUpdateEnabled: boolean; // Enable auto-updates
  updateChannel?: UpdateChannel; // Update channel: 'stable' | 'rc'
  autoUpdateCheckInterval?: number; // Hours between update checks
  autoUpdateTime?: string; // Time for updates (HH:MM format)
  backupPollingInterval?: number; // Backup polling interval in seconds (0 = default cadence)
  backupPollingEnabled?: boolean; // Enable backup polling of PVE/PBS data
  temperatureMonitoringEnabled?: boolean; // Collect CPU/NVMe temperatures via SSH
  sshPort?: number; // SSH port for temperature monitoring (default: 22)
  allowedOrigins?: string; // CORS allowed origins
  frontendPort?: number; // Frontend UI port (default: 7655)
  theme?: string; // Theme preference: 'light' | 'dark' | undefined (system default)
  fullWidthMode?: boolean; // Full-width layout mode preference
  discoveryEnabled?: boolean; // Enable/disable network discovery
  discoverySubnet?: string; // Subnet to scan for discovery (default: 'auto')
  allowEmbedding?: boolean; // Allow iframe embedding
  allowedEmbedOrigins?: string; // Comma-separated list of allowed origins for embedding
  webhookAllowedPrivateCIDRs?: string; // Comma-separated list of private CIDR ranges allowed for webhooks (e.g., "192.168.1.0/24,10.0.0.0/8")
  hideLocalLogin?: boolean; // Hide local login form (username/password)
  publicURL?: string; // Public URL for email notifications (e.g., http://198.51.100.100:8080)
  disableDockerUpdateActions?: boolean; // Hide Docker update buttons while still detecting updates (server-wide)
  reduceProUpsellNoise?: boolean; // Legacy compatibility preference for proactive commercial prompts
  telemetryEnabled?: boolean; // Outbound usage telemetry, enabled by default unless disabled
}

/**
 * Node instance configuration (stored encrypted in nodes.enc)
 */
export interface NodeInstance {
  name: string;
  url: string;
  username: string;
  password?: string; // Encrypted at rest
  token?: string; // Optional API token
  fingerprint?: string; // TLS certificate fingerprint
}

/**
 * PVE-specific node configuration
 */
export interface PVENodeConfig extends NodeInstance {
  realm?: string; // Authentication realm (pam, pve, etc.)
}

/**
 * PBS-specific node configuration
 */
export interface PBSNodeConfig extends NodeInstance {
  datastore?: string; // Default datastore
}

/**
 * Nodes configuration from nodes.enc file
 */
export interface NodesConfig {
  pveInstances: PVENodeConfig[];
  pbsInstances: PBSNodeConfig[];
}

/**
 * API response for security status
 */
export interface SecurityStatusSettingsCapabilities {
  apiAccessRead: boolean;
  apiAccessWrite: boolean;
  authenticationRead: boolean;
  authenticationWrite: boolean;
  singleSignOnRead: boolean;
  singleSignOnWrite: boolean;
  roles: boolean;
  users: boolean;
  auditLog: boolean;
  auditWebhooksRead: boolean;
  auditWebhooksWrite: boolean;
  relayRead: boolean;
  relayWrite: boolean;
  billingAdmin: boolean;
}

export interface SecurityStatusSessionCapabilities {
  demoMode: boolean;
  assistantEnabled?: boolean;
}

export interface SecurityStatusPresentationPolicy {
  demoMode: boolean;
  readOnly: boolean;
  hideCommercial: boolean;
  hideUpgrade: boolean;
}

export interface SecurityStatus {
  detailLevel?: 'public' | 'authenticated' | 'privileged';
  hasAuthentication: boolean;
  requiresAuth: boolean;
  ssoEnabled?: boolean;
  hideLocalLogin?: boolean; // Login discovery metadata
  ssoProviders?: SSOProviderInfo[];
  presentationPolicy?: SecurityStatusPresentationPolicy;
  apiTokenConfigured?: boolean;
  apiTokenHint?: string;
  credentialsEncrypted?: boolean;
  exportProtected?: boolean;
  hasAuditLogging?: boolean;
  configuredButPendingRestart?: boolean;
  unprotectedExportAllowed?: boolean;
  hasHTTPS?: boolean;
  publicAccess?: boolean;
  isPrivateNetwork?: boolean;
  isTrustedNetwork?: boolean;
  clientIP?: string;
  hasProxyAuth?: boolean;
  proxyAuthUsername?: string;
  proxyAuthIsAdmin?: boolean;
  proxyAuthLogoutURL?: string;
  authUsername?: string;
  authLastModified?: string;
  message?: string;
  ssoSessionUsername?: string;
  ssoSessionDisplayName?: string;
  ssoLogoutURL?: string;
  agentUrl?: string; // URL for agent install commands (from PULSE_PUBLIC_URL or auto-detected)
  // Token auth scopes (for kiosk/limited-access mode)
  tokenScopes?: string[];
  sessionCapabilities?: SecurityStatusSessionCapabilities;
  settingsCapabilities?: SecurityStatusSettingsCapabilities;
}

/**
 * SSO provider info for login page
 */
export interface SSOProviderInfo {
  id: string;
  name: string;
  type: 'oidc' | 'saml';
  displayName: string;
  iconUrl?: string;
  loginUrl: string;
}
