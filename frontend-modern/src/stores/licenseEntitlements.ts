import { createSignal } from 'solid-js';
import { LicenseAPI, type LicenseCommercialEntitlements } from '@/api/license';
import { eventBus } from '@/stores/events';
import { logger } from '@/utils/logger';
import {
  presentationPolicyHidesCommercialSurfaces,
  sessionPresentationPolicyResolved,
} from '@/stores/sessionPresentationPolicy';

const FREE_LICENSE_ENTITLEMENTS_FALLBACK: LicenseCommercialEntitlements = {
  capabilities: ['update_alerts', 'sso', 'ai_patrol'],
  limits: [],
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

const [licenseEntitlementsState, setLicenseEntitlementsState] =
  createSignal<LicenseCommercialEntitlements | null>(null);
const [loading, setLoading] = createSignal(false);
const [loaded, setLoaded] = createSignal(false);
const [loadError, setLoadError] = createSignal<Error | null>(null);

/**
 * Load the full commercial entitlement payload used only by billing/identity surfaces.
 */
export async function loadLicenseEntitlements(force = false): Promise<void> {
  if (!sessionPresentationPolicyResolved()) {
    logger.debug(
      '[licenseEntitlementsStore] Full entitlements deferred until presentation policy resolves',
    );
    return;
  }

  if (presentationPolicyHidesCommercialSurfaces()) {
    setLicenseEntitlementsState(FREE_LICENSE_ENTITLEMENTS_FALLBACK);
    setLoadError(null);
    setLoaded(true);
    setLoading(false);
    logger.debug(
      '[licenseEntitlementsStore] Full entitlements suppressed by presentation policy',
    );
    return;
  }

  if (loaded() && !force) return;

  setLoading(true);
  try {
    const next = await LicenseAPI.getCommercialEntitlements();
    setLicenseEntitlementsState(next);
    setLoadError(null);
    setLoaded(true);
    logger.debug('[licenseEntitlementsStore] Full entitlements loaded', {
      tier: next.tier,
      sub_state: next.subscription_state,
    });
  } catch (err) {
    logger.error('[licenseEntitlementsStore] Failed to load full entitlements', err);
    setLoadError(err instanceof Error ? err : new Error(String(err)));
    setLicenseEntitlementsState(FREE_LICENSE_ENTITLEMENTS_FALLBACK);
    setLoaded(true);
  } finally {
    setLoading(false);
  }
}

// Ensure org-scoped billing entitlements do not leak across tenant switches.
eventBus.on('org_switched', () => {
  setLicenseEntitlementsState(null);
  setLoaded(false);
  setLoadError(null);
  void loadLicenseEntitlements(true);
});

export {
  licenseEntitlementsState as licenseEntitlements,
  loading as licenseEntitlementsLoading,
  loaded as licenseEntitlementsLoaded,
  loadError as licenseEntitlementsLoadError,
};
