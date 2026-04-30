export interface UpgradeMetricEvent {
  type: string;
  capability?: string;
  surface: string;
  tenant_mode?: string;
  limit_key?: string;
  current_value?: number;
  limit_value?: number;
  timestamp: number;
  idempotency_key: string;
}

export interface TrackUpgradeMetricEventInput {
  type: string;
  surface: string;
  capability?: string;
  tenant_mode?: string;
  limit_key?: string;
  current_value?: number;
  limit_value?: number;
  idempotencyKey?: string;
}

export const UPGRADE_METRIC_EVENTS = {
  PRICING_VIEWED: 'pricing_viewed',
  PAYWALL_VIEWED: 'paywall_viewed',
  TRIAL_STARTED: 'trial_started',
  LICENSE_ACTIVATED: 'license_activated',
  UPGRADE_CLICKED: 'upgrade_clicked',
  CHECKOUT_CLICKED: 'checkout_clicked',
  LIMIT_WARNING_SHOWN: 'limit_warning_shown',
  LIMIT_BLOCKED: 'limit_blocked',
  AGENT_INSTALL_TOKEN_GENERATED: 'agent_install_token_generated',
  AGENT_INSTALL_COMMAND_COPIED: 'agent_install_command_copied',
  AGENT_INSTALL_PROFILE_SELECTED: 'agent_install_profile_selected',
  AGENT_FIRST_CONNECTED: 'agent_first_connected',
  INFRASTRUCTURE_ONBOARDING_OPENED: 'infrastructure_onboarding_opened',
  INFRASTRUCTURE_ONBOARDING_PATH_SELECTED: 'infrastructure_onboarding_path_selected',
  INFRASTRUCTURE_ONBOARDING_PROBE_RESULT: 'infrastructure_onboarding_probe_result',
  INFRASTRUCTURE_ONBOARDING_CATALOG_SELECTED: 'infrastructure_onboarding_catalog_selected',
  INFRASTRUCTURE_ONBOARDING_CREDENTIALS_OPENED: 'infrastructure_onboarding_credentials_opened',
} as const;

export function trackUpgradeMetricEvent(
  _event: TrackUpgradeMetricEventInput,
): void {
  // Compatibility no-op: customer frontend surfaces must not emit maintainer
  // commercial, funnel, or onboarding analytics.
}

export function trackPaywallViewed(_capability: string, _surface: string): void {
  // Compatibility no-op.
}

export function trackPricingViewed(_surface: string, _capability?: string): void {
  // Compatibility no-op.
}

export function trackUpgradeClicked(_surface: string, _capability?: string): void {
  // Compatibility no-op.
}

export function trackCheckoutClicked(_surface: string, _capability?: string): void {
  // Compatibility no-op.
}

export function trackAgentInstallTokenGenerated(_surface: string, _capability?: string): void {
  // Compatibility no-op.
}

export function trackAgentInstallCommandCopied(_surface: string, _capability?: string): void {
  // Compatibility no-op.
}

export function trackAgentInstallProfileSelected(_surface: string, _profile: string): void {
  // Compatibility no-op.
}

export function trackAgentFirstConnected(_surface: string, _capability?: string): void {
  // Compatibility no-op.
}
