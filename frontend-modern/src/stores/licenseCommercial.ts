import { createSignal, createMemo, createRoot } from 'solid-js';
import {
  LicenseAPI,
  type EntitlementLegacyConnections,
  type LicenseCommercialPosture,
} from '@/api/license';
import { eventBus } from '@/stores/events';
import { logger } from '@/utils/logger';
import {
  getUpgradeFallbackDestination,
  isSelfHostedPurchaseStartDestination,
} from '@/utils/pricingHandoff';
import { resolveUpgradeDestination, type UpgradeDestination } from '@/utils/upgradeNavigation';
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

/**
 * Load the commercial posture payload used by non-billing upgrade and activation UI.
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

export function commercialOverflowDaysRemaining(): number | null {
  const current = commercialPostureState()?.overflow_days_remaining;
  return typeof current === 'number' && Number.isFinite(current) ? current : null;
}

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
    productOwnedHref ||
    getUpgradeActionUrl(key) ||
    getFirstUpgradeActionUrl() ||
    getUpgradeFallbackDestination(key);

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
