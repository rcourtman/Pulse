import { createEffect, createMemo, createSignal } from 'solid-js';
import {
  BillingAdminAPI,
  type BillingState,
  type HostedOrganizationSummary,
} from '@/api/billingAdmin';
import { isHostedModeEnabled, isMultiTenantEnabled } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import { presentationPolicyHidesOrganizationSurfaces } from '@/stores/sessionPresentationPolicy';
import { logger } from '@/utils/logger';
import {
  getBillingAdminStateUpdateSuccessMessage,
} from '@/utils/licensePresentation';
import {
  getOrganizationSettingsLoadErrorMessage as getOrganizationSettingsPanelLoadErrorMessage,
} from '@/utils/organizationSettingsPresentation';

export type BillingStateCache = Record<string, BillingState | undefined>;

async function promisePool<T>(items: T[], concurrency: number, fn: (item: T) => Promise<void>) {
  if (items.length === 0) return;
  const limit = Math.max(1, Math.min(concurrency, items.length));
  let idx = 0;
  const workers = Array.from({ length: limit }).map(async () => {
    for (;;) {
      const current = idx;
      idx += 1;
      if (current >= items.length) return;
      await fn(items[current]);
    }
  });
  await Promise.all(workers);
}

export function useBillingAdminPanelState() {
  const [orgs, setOrgs] = createSignal<HostedOrganizationSummary[]>([]);
  const [loadingOrgs, setLoadingOrgs] = createSignal(false);
  const [orgsError, setOrgsError] = createSignal<string | null>(null);
  const [billingByOrgID, setBillingByOrgID] = createSignal<BillingStateCache>({});
  const [billingLoadingByOrgID, setBillingLoadingByOrgID] = createSignal<Record<string, boolean>>(
    {},
  );
  const [savingByOrgID, setSavingByOrgID] = createSignal<Record<string, boolean>>({});
  const [expandedOrgID, setExpandedOrgID] = createSignal<string | null>(null);

  const hostedEnabled = createMemo(
    () =>
      isMultiTenantEnabled() &&
      isHostedModeEnabled() &&
      !presentationPolicyHidesOrganizationSurfaces(),
  );

  const setBillingLoading = (orgID: string, value: boolean) => {
    setBillingLoadingByOrgID((prev) => ({ ...prev, [orgID]: value }));
  };

  const setSaving = (orgID: string, value: boolean) => {
    setSavingByOrgID((prev) => ({ ...prev, [orgID]: value }));
  };

  const loadOrganizations = async () => {
    setLoadingOrgs(true);
    setOrgsError(null);
    try {
      const next = await BillingAdminAPI.listOrganizations();
      setOrgs(next ?? []);
    } catch (err) {
      logger.error('Failed to list hosted organizations', err);
      const msg = err instanceof Error ? err.message : '';
      const errorMessage = getOrganizationSettingsPanelLoadErrorMessage(msg, 'billing-admin');
      setOrgsError(errorMessage);
      notificationStore.error(errorMessage);
    } finally {
      setLoadingOrgs(false);
    }
  };

  const ensureBillingState = async (orgID: string): Promise<BillingState | null> => {
    const cached = billingByOrgID()[orgID];
    if (cached) return cached;

    if (billingLoadingByOrgID()[orgID]) {
      return null;
    }

    setBillingLoading(orgID, true);
    try {
      const state = await BillingAdminAPI.getBillingState(orgID);
      setBillingByOrgID((prev) => ({ ...prev, [orgID]: state }));
      return state;
    } catch (err) {
      logger.error('Failed to fetch billing state', { orgID, err });
      return null;
    } finally {
      setBillingLoading(orgID, false);
    }
  };

  const updateSubscriptionState = async (orgID: string, nextState: 'suspended' | 'active') => {
    setSaving(orgID, true);
    try {
      const current =
        (await ensureBillingState(orgID)) ?? (await BillingAdminAPI.getBillingState(orgID));
      const planVersion = current.plan_version?.trim() || undefined;
      const payload: BillingState = {
        ...current,
        subscription_state: nextState,
        capabilities: Array.isArray(current.capabilities) ? current.capabilities : [],
        limits: current.limits ?? {},
        meters_enabled: Array.isArray(current.meters_enabled) ? current.meters_enabled : [],
        ...(planVersion ? { plan_version: planVersion } : {}),
      };

      const saved = await BillingAdminAPI.putBillingState(orgID, payload);
      setBillingByOrgID((prev) => ({ ...prev, [orgID]: saved }));
      notificationStore.success(getBillingAdminStateUpdateSuccessMessage(nextState), 2500);
    } catch (err) {
      logger.error('Failed to update billing state', { orgID, err });
      const msg = err instanceof Error ? err.message : 'Failed to update billing state';
      notificationStore.error(msg);
    } finally {
      setSaving(orgID, false);
    }
  };

  const refreshOrganizations = async () => {
    setBillingByOrgID({});
    setExpandedOrgID(null);
    await loadOrganizations();
  };

  const toggleExpandedOrganization = async (orgID: string) => {
    if (!orgID) return;
    setExpandedOrgID((prev) => (prev === orgID ? null : orgID));
    await ensureBillingState(orgID);
  };

  const reloadOrganization = async (orgID: string) => {
    if (!orgID) return;
    setBillingByOrgID((prev) => ({ ...prev, [orgID]: undefined }));
    await ensureBillingState(orgID);
  };

  createEffect(() => {
    if (!hostedEnabled()) return;
    void loadOrganizations();
  });

  createEffect(() => {
    const list = orgs();
    if (!hostedEnabled() || loadingOrgs() || list.length === 0) return;

    const missing = list.map((o) => o.org_id).filter((id) => id && !billingByOrgID()[id]);
    if (missing.length === 0) return;

    void promisePool(missing, 6, async (orgID) => {
      await ensureBillingState(orgID);
    });
  });

  return {
    billingByOrgID,
    billingLoadingByOrgID,
    expandedOrgID,
    hostedEnabled,
    loadingOrgs,
    orgs,
    orgsError,
    refreshOrganizations,
    reloadOrganization,
    savingByOrgID,
    toggleExpandedOrganization,
    updateSubscriptionState,
  };
}
