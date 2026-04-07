import { createSignal, createMemo, createRoot } from 'solid-js';
import {
  LicenseAPI,
  type EntitlementLegacyConnections,
  type LicenseCommercialPosture,
} from '@/api/license';
import { eventBus } from '@/stores/events';
import { logger } from '@/utils/logger';
import {
  getInProductPricingDestination,
  getUpgradeFallbackDestination,
  isSelfHostedPurchaseStartDestination,
} from '@/utils/pricingHandoff';
import {
  resolveUpgradeDestination,
  type UpgradeDestination,
} from '@/utils/upgradeNavigation';
import { loadRuntimeCapabilities } from '@/stores/license';
import { loadLicenseEntitlements } from '@/stores/licenseEntitlements';
import {
  presentationPolicyHidesCommercialSurfaces,
  sessionPresentationPolicyResolved,
} from '@/stores/sessionPresentationPolicy';

const FREE_COMMERCIAL_POSTURE_FALLBACK: LicenseCommercialPosture = {
  subscription_state: 'expired',
  upgrade_reasons: [],
  tier: 'free',
  trial_eligible: false,
  legacy_connections: {
    proxmox_nodes: 0,
    docker_hosts: 0,
    kubernetes_clusters: 0,
  },
  has_migration_gap: false,
  commercial_migration: undefined,
};

const [commercialPostureState, setCommercialPostureState] =
  createSignal<LicenseCommercialPosture | null>(null);
const [loading, setLoading] = createSignal(false);
const [loaded, setLoaded] = createSignal(false);
const [loadError, setLoadError] = createSignal<Error | null>(null);
let inFlightCommercialPostureLoad: Promise<void> | null = null;

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
 * Load the commercial posture payload used by non-billing trial and upgrade UI.
 * Initial bootstrap belongs to the authenticated app shell or first-run setup,
 * not to feature-local hooks.
 */
export async function loadCommercialPosture(force = false): Promise<void> {
  if (!sessionPresentationPolicyResolved()) {
    logger.debug(
      '[licenseCommercialStore] Commercial posture deferred until presentation policy resolves',
    );
    return;
  }

  if (presentationPolicyHidesCommercialSurfaces()) {
    setCommercialPostureState(FREE_COMMERCIAL_POSTURE_FALLBACK);
    setLoadError(null);
    setLoaded(true);
    setLoading(false);
    logger.debug('[licenseCommercialStore] Commercial posture suppressed by presentation policy');
    return;
  }

  if (inFlightCommercialPostureLoad) {
    if (!force) {
      return inFlightCommercialPostureLoad;
    }
    await inFlightCommercialPostureLoad;
  }

  if (loaded() && !force) return;

  setLoading(true);
  const request = (async () => {
    try {
      const next = await LicenseAPI.getCommercialPosture();
      setCommercialPostureState(next);
      setLoadError(null);
      setLoaded(true);
      logger.debug('[licenseCommercialStore] Commercial posture loaded', {
        tier: next.tier,
        sub_state: next.subscription_state,
      });
    } catch (err) {
      logger.error('[licenseCommercialStore] Failed to load commercial posture', err);
      setLoadError(err instanceof Error ? err : new Error(String(err)));
      setCommercialPostureState(FREE_COMMERCIAL_POSTURE_FALLBACK);
      setLoaded(true);
    } finally {
      setLoading(false);
    }
  })();

  inFlightCommercialPostureLoad = request;
  try {
    await request;
  } finally {
    if (inFlightCommercialPostureLoad === request) {
      inFlightCommercialPostureLoad = null;
    }
  }
}

/**
 * Start a Pro trial for the current org, then refresh both runtime and commercial state.
 */
export async function startProTrial(): Promise<StartProTrialResult> {
  if (!sessionPresentationPolicyResolved() || presentationPolicyHidesCommercialSurfaces()) {
    const err = new Error(
      'Trial activation unavailable under the current presentation policy',
    ) as TrialStartRequestError;
    err.status = 404;
    err.code = 'presentation_policy_unavailable';
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
  await Promise.all([
    loadCommercialPosture(true),
    loadRuntimeCapabilities(true),
    loadLicenseEntitlements(true),
  ]);
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
    const current = commercialPostureState();
    return Boolean(current && PRO_TIERS.has(current.tier));
  }),
);

export function getUpgradeReason(key: string) {
  const current = commercialPostureState();
  if (!current?.upgrade_reasons?.length) return undefined;
  return current.upgrade_reasons.find((reason) => reason.key === key);
}

export function getUpgradeActionUrl(key: string): string | undefined {
  return getUpgradeReason(key)?.action_url;
}

export function getFirstUpgradeActionUrl(): string | undefined {
  const current = commercialPostureState();
  return current?.upgrade_reasons?.[0]?.action_url;
}

export function getUpgradeActionDestination(key: string): UpgradeDestination {
  const productOwnedHref = key ? getUpgradeFallbackDestination(key) : undefined;
  const href =
    productOwnedHref || getUpgradeActionUrl(key) || getFirstUpgradeActionUrl() || getUpgradeFallbackDestination(key);

  if (isSelfHostedPurchaseStartDestination(href)) {
    return resolveUpgradeDestination(href, {
      hardNavigation: true,
      newTab: true,
      preserveOpener: true,
    });
  }

  return resolveUpgradeDestination(href);
}

export function getUpgradeActionUrlOrFallback(key: string): string {
  return getUpgradeActionDestination(key).href;
}

export function legacyConnections(): EntitlementLegacyConnections {
  return (
    commercialPostureState()?.legacy_connections ?? {
      proxmox_nodes: 0,
      docker_hosts: 0,
      kubernetes_clusters: 0,
    }
  );
}

export function hasMigrationGap(): boolean {
  return Boolean(commercialPostureState()?.has_migration_gap);
}

// Ensure org-scoped commercial posture does not leak across tenant switches.
eventBus.on('org_switched', () => {
  setCommercialPostureState(null);
  setLoaded(false);
  setLoadError(null);
  void loadCommercialPosture(true);
});

export {
  commercialPostureState as commercialPosture,
  loading as commercialPostureLoading,
  loaded as commercialPostureLoaded,
  loadError as commercialPostureLoadError,
};
