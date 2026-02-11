import { apiFetch } from '@/utils/apiClient';

export interface ConversionEvent {
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

export const CONVERSION_EVENTS = {
  PAYWALL_VIEWED: 'paywall_viewed',
  TRIAL_STARTED: 'trial_started',
  LICENSE_ACTIVATED: 'license_activated',
  UPGRADE_CLICKED: 'upgrade_clicked',
  LIMIT_WARNING_SHOWN: 'limit_warning_shown',
  LIMIT_BLOCKED: 'limit_blocked',
  AGENT_INSTALL_TOKEN_GENERATED: 'agent_install_token_generated',
  AGENT_INSTALL_COMMAND_COPIED: 'agent_install_command_copied',
  AGENT_INSTALL_PROFILE_SELECTED: 'agent_install_profile_selected',
  AGENT_FIRST_CONNECTED: 'agent_first_connected',
} as const;

const ONE_MINUTE_MS = 60_000;
const recentlySentKeys = new Set<string>();
const sentAtByKey = new Map<string, number>();

function pruneExpiredKeys(now: number): void {
  for (const [key, sentAt] of sentAtByKey.entries()) {
    if (now - sentAt <= ONE_MINUTE_MS) continue;
    sentAtByKey.delete(key);
    recentlySentKeys.delete(key);
  }
}

export function trackConversionEvent(
  event: Partial<ConversionEvent> & { type: string; surface: string },
): void {
  const now = Date.now();
  const idempotencyKey = `${event.type}:${event.surface}:${event.capability || ''}:${Math.floor(now / ONE_MINUTE_MS)}`;

  pruneExpiredKeys(now);
  if (recentlySentKeys.has(idempotencyKey)) {
    return;
  }

  recentlySentKeys.add(idempotencyKey);
  sentAtByKey.set(idempotencyKey, now);

  const payload: ConversionEvent = {
    type: event.type,
    capability: event.capability,
    surface: event.surface,
    tenant_mode: event.tenant_mode,
    limit_key: event.limit_key,
    current_value: event.current_value,
    limit_value: event.limit_value,
    timestamp: now,
    idempotency_key: idempotencyKey,
  };

  try {
    void apiFetch('/api/conversion/events', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    }).catch(() => {
      // Conversion tracking should never break user interactions.
    });
  } catch {
    // Fire-and-forget, swallow sync failures.
  }
}

export function trackPaywallViewed(capability: string, surface: string): void {
  trackConversionEvent({
    type: CONVERSION_EVENTS.PAYWALL_VIEWED,
    capability,
    surface,
  });
}

export function trackUpgradeClicked(surface: string, capability?: string): void {
  trackConversionEvent({
    type: CONVERSION_EVENTS.UPGRADE_CLICKED,
    surface,
    capability,
  });
}

export function trackAgentInstallTokenGenerated(surface: string, capability?: string): void {
  trackConversionEvent({
    type: CONVERSION_EVENTS.AGENT_INSTALL_TOKEN_GENERATED,
    surface,
    capability,
  });
}

export function trackAgentInstallCommandCopied(surface: string, capability?: string): void {
  trackConversionEvent({
    type: CONVERSION_EVENTS.AGENT_INSTALL_COMMAND_COPIED,
    surface,
    capability,
  });
}

export function trackAgentInstallProfileSelected(surface: string, profile: string): void {
  trackConversionEvent({
    type: CONVERSION_EVENTS.AGENT_INSTALL_PROFILE_SELECTED,
    surface,
    capability: profile,
  });
}

export function trackAgentFirstConnected(surface: string, capability?: string): void {
  trackConversionEvent({
    type: CONVERSION_EVENTS.AGENT_FIRST_CONNECTED,
    surface,
    capability,
  });
}
