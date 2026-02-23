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
  API_TOKEN: string; // Legacy API authentication token (hashed)
  API_TOKENS?: string; // Optional comma-separated list of hashed tokens
}

/**
 * System settings from system.json file
 * These are application behavior settings
 */
export interface SystemConfig {
  pvePollingInterval?: number; // PVE polling interval in seconds
  pbsPollingInterval?: number; // PBS polling interval in seconds
  pmgPollingInterval?: number; // PMG polling interval in seconds
  connectionTimeout?: number; // Seconds before timeout (default: 10)
  autoUpdateEnabled: boolean; // Enable auto-updates
  updateChannel?: string; // Update channel: 'stable' | 'rc' | 'beta'
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
  publicURL?: string; // Public URL for email notifications (e.g., http://192.168.1.100:8080)
  disableDockerUpdateActions?: boolean; // Hide Docker update buttons while still detecting updates (server-wide)
  disableLegacyRouteRedirects?: boolean; // Disable legacy frontend URL redirects globally
  reduceProUpsellNoise?: boolean; // Hide proactive Pro prompts; paywalls still appear when accessing gated features
  disableLocalUpgradeMetrics?: boolean; // Disable local-only upgrade UX metrics collection
  telemetryEnabled?: boolean; // Opt-in anonymous usage telemetry
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
 * Complete configuration structure
 */
export interface PulseConfig {
  auth: Partial<AuthConfig>; // From .env
  system: SystemConfig; // From system.json
  nodes: NodesConfig; // From nodes.enc
}

/**
 * API response for security status
 */
export interface SecurityStatus {
  hasAuthentication: boolean;
  apiTokenConfigured: boolean;
  apiTokenHint: string;
  requiresAuth: boolean;
  credentialsEncrypted: boolean;
  exportProtected: boolean;
  hasAuditLogging: boolean;
  configuredButPendingRestart: boolean;
  unprotectedExportAllowed?: boolean;
  hasHTTPS?: boolean;
  oidcEnabled?: boolean;
  publicAccess?: boolean;
  isPrivateNetwork?: boolean;
  clientIP?: string;
  hasProxyAuth?: boolean;
  proxyAuthUsername?: string;
  proxyAuthIsAdmin?: boolean;
  proxyAuthLogoutURL?: string;
  authUsername?: string;
  authLastModified?: string;
  disabled?: boolean; // legacy field (removed backend support)
  deprecatedDisableAuth?: boolean;
  message?: string;
  hideLocalLogin?: boolean; // Hide local login form
  agentUrl?: string; // URL for agent install commands (from PULSE_PUBLIC_URL or auto-detected)
  // First run setup fields
  bootstrapTokenPath?: string;
  isDocker?: boolean;
  inContainer?: boolean;
  lxcCtid?: string;
  dockerContainerName?: string;
  // OIDC fields
  oidcIssuer?: string;
  oidcClientId?: string;
  oidcEnvOverrides?: Record<string, boolean>;
  oidcLogoutURL?: string;
  // Multi-provider SSO
  ssoProviders?: SSOProviderInfo[];
  // Token auth scopes (for kiosk/limited-access mode)
  tokenScopes?: string[];
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

/**
 * First-run setup request
 */
export interface SetupRequest {
  username: string;
  password: string;
  apiToken?: string;
  enableNotifications?: boolean;
  darkMode?: boolean;
}

/**
 * Type guards for configuration validation
 */
export const isValidUpdateChannel = (value: string): value is 'stable' | 'rc' | 'beta' => {
  return ['stable', 'rc', 'beta'].includes(value);
};

export const isValidTimeFormat = (value: string): boolean => {
  return /^([01]\d|2[0-3]):([0-5]\d)$/.test(value);
};

/**
 * Default values for configuration
 */
export const DEFAULT_CONFIG: {
  system: SystemConfig;
} = {
  system: {
    connectionTimeout: 60,
    autoUpdateEnabled: false,
    updateChannel: 'stable',
    autoUpdateCheckInterval: 24,
    autoUpdateTime: '03:00',
    backupPollingEnabled: true,
    backupPollingInterval: 0,
    temperatureMonitoringEnabled: true,
    telemetryEnabled: true,
    sshPort: 22,
    allowedOrigins: '',
    frontendPort: 7655,
  },
};
