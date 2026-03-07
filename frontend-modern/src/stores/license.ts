import { createSignal, createMemo, createRoot } from 'solid-js';
import {
  LicenseAPI,
  type EntitlementLegacyConnections,
  type LicenseEntitlements,
} from '@/api/license';
import { eventBus } from '@/stores/events';
import { logger } from '@/utils/logger';

// Reactive signals for entitlements (canonical gating source).
const [entitlements, setEntitlements] = createSignal<LicenseEntitlements | null>(null);
const [loading, setLoading] = createSignal(false);
const [loaded, setLoaded] = createSignal(false);
const [loadError, setLoadError] = createSignal<Error | null>(null);

type TrialStartErrorPayload = {
  error?: string;
  code?: string;
  details?: Record<string, string>;
};

export type StartProTrialResult =
  | { outcome: 'activated' }
  | { outcome: 'redirect'; actionUrl: string };

/**
 * Load the entitlements payload from the server.
 *
 * This is the canonical feature-gating source (do not gate on tier strings).
 */
export async function loadLicenseStatus(force = false): Promise<void> {
  if (loaded() && !force) return;

  setLoading(true);
  try {
    const next = await LicenseAPI.getEntitlements();
    setEntitlements(next);
    setLoadError(null);
    setLoaded(true);
    logger.debug('[licenseStore] Entitlements loaded', {
      tier: next.tier,
      sub_state: next.subscription_state,
    });
  } catch (err) {
    logger.error('[licenseStore] Failed to load entitlements', err);
    setLoadError(err instanceof Error ? err : new Error(String(err)));
    // Best-effort fallback to avoid breaking the UI.
    setEntitlements({
      capabilities: ['update_alerts', 'sso', 'ai_patrol'],
      limits: [],
      // Match backend behavior when no license is present.
      subscription_state: 'expired',
      upgrade_reasons: [],
      tier: 'free',
      hosted_mode: false,
      trial_eligible: true,
      legacy_connections: {
        proxmox_nodes: 0,
        docker_hosts: 0,
        kubernetes_clusters: 0,
      },
      has_migration_gap: false,
      commercial_migration: undefined,
    });
    setLoaded(true);
  } finally {
    setLoading(false);
  }
}

/**
 * Start a Pro trial for the current org, then refresh entitlements.
 */
export async function startProTrial(): Promise<StartProTrialResult> {
  const res = await LicenseAPI.startTrial();
  if (!res.ok) {
    let payload: TrialStartErrorPayload | null = null;
    try {
      payload = (await res.json()) as TrialStartErrorPayload;
    } catch {
      payload = null;
    }

    if (res.status === 409 && payload?.code === 'trial_signup_required') {
      const actionUrl =
        payload.details?.action_url ||
        getFirstUpgradeActionUrl() ||
        getUpgradeActionUrlOrFallback('relay');
      return { outcome: 'redirect', actionUrl };
    }

    const err = new Error(`Failed to start trial (status ${res.status})`) as Error & {
      status: number;
      code?: string;
    };
    err.status = res.status;
    err.code = payload?.code;
    throw err;
  }
  await loadLicenseStatus(true);
  return { outcome: 'activated' };
}

/** Tiers that qualify as "Pro" for UI gating (support link, Pro badges, etc.). */
const PRO_TIERS = new Set([
  'pro',
  'pro_annual',
  'pro_plus',
  'lifetime',
  'cloud',
  'msp',
  'enterprise',
]);

/**
 * Helper to check if the current license is Pulse Pro (Pro tier or above).
 * Relay is a paid tier but does NOT count as Pro.
 */
export const isPro = createRoot(() =>
  createMemo(() => {
    const current = entitlements();
    return Boolean(current && PRO_TIERS.has(current.tier));
  }),
);

/**
 * Check if a specific feature is enabled by the current license.
 */
export function hasFeature(feature: string): boolean {
  const current = entitlements();
  if (!current) return false;
  return current.capabilities.includes(feature);
}

export function isMultiTenantEnabled(): boolean {
  return hasFeature('multi_tenant');
}

export function isHostedModeEnabled(): boolean {
  return Boolean(entitlements()?.hosted_mode);
}

export function getUpgradeReason(key: string) {
  const current = entitlements();
  if (!current?.upgrade_reasons?.length) return undefined;
  return current.upgrade_reasons.find((reason) => reason.key === key);
}

export function getUpgradeActionUrl(key: string): string | undefined {
  return getUpgradeReason(key)?.action_url;
}

export function getFirstUpgradeActionUrl(): string | undefined {
  const current = entitlements();
  return current?.upgrade_reasons?.[0]?.action_url;
}

export function getUpgradeActionUrlOrFallback(key: string): string {
  return (
    getUpgradeActionUrl(key) ||
    getFirstUpgradeActionUrl() ||
    `/pricing?feature=${encodeURIComponent(key)}`
  );
}

export function getLimit(key: string) {
  const current = entitlements();
  if (!current?.limits?.length) return undefined;
  return current.limits.find((limit) => limit.key === key);
}

export function legacyConnections(): EntitlementLegacyConnections {
  return (
    entitlements()?.legacy_connections ?? {
      proxmox_nodes: 0,
      docker_hosts: 0,
      kubernetes_clusters: 0,
    }
  );
}

export function hasMigrationGap(): boolean {
  return Boolean(entitlements()?.has_migration_gap);
}

/** Default max history days when entitlements aren't loaded yet. */
const DEFAULT_MAX_HISTORY_DAYS = 7;

function parseRangeDays(range: string): number {
  const match = range.match(/^(\d+)(h|d)$/);
  if (!match) return 0;
  const val = parseInt(match[1], 10);
  return match[2] === 'd' ? val : val / 24;
}

/**
 * Return the tier-specific max history days from the entitlements payload.
 * Falls back to DEFAULT_MAX_HISTORY_DAYS when entitlements aren't loaded.
 */
export function maxHistoryDays(): number {
  return entitlements()?.max_history_days ?? DEFAULT_MAX_HISTORY_DAYS;
}

/**
 * Check if a time range exceeds the current tier's history limit.
 * The limit is tier-aware (Free=7d, Relay=14d, Pro/Pro+=90d) via `max_history_days`.
 */
export function isRangeLocked(range: string): boolean {
  return parseRangeDays(range) > maxHistoryDays();
}

// Ensure org-scoped entitlements do not leak across tenant switches.
eventBus.on('org_switched', () => {
  setEntitlements(null);
  setLoaded(false);
  setLoadError(null);
  void loadLicenseStatus(true);
});

/**
 * Get the full license status.
 */
export {
  entitlements,
  entitlements as licenseStatus,
  loading as licenseLoading,
  loaded as licenseLoaded,
  loadError as licenseLoadError,
};
