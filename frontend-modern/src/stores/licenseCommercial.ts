import { createSignal, createMemo, createRoot } from 'solid-js';
import {
  LicenseAPI,
  type EntitlementLegacyConnections,
  type LicenseCommercialEntitlements,
} from '@/api/license';
import { demoModeEnabled } from '@/stores/demoMode';
import { eventBus } from '@/stores/events';
import { logger } from '@/utils/logger';
import {
  getInProductPricingDestination,
  getUpgradeFallbackDestination,
} from '@/utils/pricingHandoff';
import {
  resolveUpgradeDestination,
  type UpgradeDestination,
} from '@/utils/upgradeNavigation';
import { loadLicenseStatus as loadRuntimeLicenseStatus } from '@/stores/license';

const FREE_COMMERCIAL_ENTITLEMENTS_FALLBACK: LicenseCommercialEntitlements = {
  capabilities: ['update_alerts', 'sso', 'ai_patrol'],
  limits: [],
  subscription_state: 'expired',
  upgrade_reasons: [],
  tier: 'free',
  hosted_mode: false,
  trial_eligible: false,
  legacy_connections: {
    proxmox_nodes: 0,
    docker_hosts: 0,
    kubernetes_clusters: 0,
  },
  has_migration_gap: false,
  commercial_migration: undefined,
};

const [commercialEntitlements, setCommercialEntitlements] =
  createSignal<LicenseCommercialEntitlements | null>(null);
const [loading, setLoading] = createSignal(false);
const [loaded, setLoaded] = createSignal(false);
const [loadError, setLoadError] = createSignal<Error | null>(null);

type TrialStartErrorPayload = {
  error?: string;
  code?: string;
  details?: Record<string, string>;
};

type TrialStartRequestError = Error & {
  status: number;
  code?: string;
  details?: Record<string, string>;
  retryAfterSeconds?: number;
};

export type StartProTrialResult =
  | { outcome: 'activated' }
  | { outcome: 'redirect'; actionUrl: string };

function parseRetryAfterSeconds(value: string | null | undefined): number | undefined {
  const normalized = value?.trim();
  if (!normalized) return undefined;

  const parsed = Number(normalized);
  if (Number.isFinite(parsed) && parsed > 0) {
    return Math.ceil(parsed);
  }

  const retryAt = Date.parse(normalized);
  if (Number.isNaN(retryAt)) return undefined;

  const waitMs = retryAt - Date.now();
  if (waitMs <= 0) return 1;
  return Math.ceil(waitMs / 1000);
}

/**
 * Load the commercial entitlement payload used by billing, trial, and upgrade UI.
 */
export async function loadLicenseStatus(force = false): Promise<void> {
  if (demoModeEnabled()) {
    setCommercialEntitlements(FREE_COMMERCIAL_ENTITLEMENTS_FALLBACK);
    setLoadError(null);
    setLoaded(true);
    setLoading(false);
    logger.debug('[licenseCommercialStore] Commercial entitlements suppressed in demo mode');
    return;
  }

  if (loaded() && !force) return;

  setLoading(true);
  try {
    const next = await LicenseAPI.getCommercialEntitlements();
    setCommercialEntitlements(next);
    setLoadError(null);
    setLoaded(true);
    logger.debug('[licenseCommercialStore] Commercial entitlements loaded', {
      tier: next.tier,
      sub_state: next.subscription_state,
    });
  } catch (err) {
    logger.error('[licenseCommercialStore] Failed to load commercial entitlements', err);
    setLoadError(err instanceof Error ? err : new Error(String(err)));
    setCommercialEntitlements(FREE_COMMERCIAL_ENTITLEMENTS_FALLBACK);
    setLoaded(true);
  } finally {
    setLoading(false);
  }
}

/**
 * Start a Pro trial for the current org, then refresh both runtime and commercial state.
 */
export async function startProTrial(): Promise<StartProTrialResult> {
  if (demoModeEnabled()) {
    const err = new Error('Trial activation unavailable in demo mode') as TrialStartRequestError;
    err.status = 404;
    err.code = 'demo_mode_unavailable';
    throw err;
  }

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

    const err = new Error(
      payload?.error?.trim() || `Failed to start trial (status ${res.status})`,
    ) as TrialStartRequestError;
    const retryAfterSeconds =
      parseRetryAfterSeconds(res.headers.get('Retry-After')) ??
      parseRetryAfterSeconds(payload?.details?.retry_after_seconds);
    err.status = res.status;
    err.code = payload?.code;
    err.details = payload?.details;
    err.retryAfterSeconds = retryAfterSeconds;
    throw err;
  }
  await Promise.all([loadLicenseStatus(true), loadRuntimeLicenseStatus(true)]);
  return { outcome: 'activated' };
}

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
 * Helper to check if the current commercial entitlement qualifies as Pro.
 * Relay is a paid tier but does not count as Pro.
 */
export const isPro = createRoot(() =>
  createMemo(() => {
    const current = commercialEntitlements();
    return Boolean(current && PRO_TIERS.has(current.tier));
  }),
);

export function getUpgradeReason(key: string) {
  const current = commercialEntitlements();
  if (!current?.upgrade_reasons?.length) return undefined;
  return current.upgrade_reasons.find((reason) => reason.key === key);
}

export function getUpgradeActionUrl(key: string): string | undefined {
  return getUpgradeReason(key)?.action_url;
}

export function getFirstUpgradeActionUrl(): string | undefined {
  const current = commercialEntitlements();
  return current?.upgrade_reasons?.[0]?.action_url;
}

export function getUpgradeActionDestination(key: string): UpgradeDestination {
  return resolveUpgradeDestination(
    getUpgradeActionUrl(key) ||
      getInProductPricingDestination(key) ||
      getFirstUpgradeActionUrl() ||
      getUpgradeFallbackDestination(key),
  );
}

export function getUpgradeActionUrlOrFallback(key: string): string {
  return getUpgradeActionDestination(key).href;
}

export function getLimit(key: string) {
  const current = commercialEntitlements();
  if (!current?.limits?.length) return undefined;
  return current.limits.find((limit) => limit.key === key);
}

export function legacyConnections(): EntitlementLegacyConnections {
  return (
    commercialEntitlements()?.legacy_connections ?? {
      proxmox_nodes: 0,
      docker_hosts: 0,
      kubernetes_clusters: 0,
    }
  );
}

export function hasMigrationGap(): boolean {
  return Boolean(commercialEntitlements()?.has_migration_gap);
}

// Ensure org-scoped commercial entitlements do not leak across tenant switches.
eventBus.on('org_switched', () => {
  setCommercialEntitlements(null);
  setLoaded(false);
  setLoadError(null);
  void loadLicenseStatus(true);
});

export {
  commercialEntitlements,
  commercialEntitlements as entitlements,
  commercialEntitlements as licenseStatus,
  loading as licenseLoading,
  loaded as licenseLoaded,
  loadError as licenseLoadError,
};
