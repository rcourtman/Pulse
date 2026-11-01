import { batch, createEffect, createMemo, createSignal, on, onCleanup } from 'solid-js';
import {
  BillingAdminAPI,
  type BillingState,
  type HostedOrganizationSummary,
} from '@/api/billingAdmin';
import { isHostedModeEnabled, isMultiTenantEnabled } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import {
  presentationPolicyHidesOrganizationSurfaces,
  presentationPolicyIsReadOnly,
} from '@/stores/sessionPresentationPolicy';
import { logger } from '@/utils/logger';
import { getBillingAdminStateUpdateSuccessMessage } from '@/utils/licensePresentation';
import { getOrganizationSettingsLoadErrorMessage } from '@/utils/organizationSettingsPresentation';

export type BillingStateCache = Record<string, BillingState | undefined>;
const emptyRecord = <T>(): Record<string, T> => Object.create(null);
const withEntry = <T>(entries: Record<string, T>, key: string, value: T): Record<string, T> =>
  Object.assign(emptyRecord<T>(), entries, { [key]: value });
export const BILLING_ADMIN_READ_ERROR =
  'Billing state is unavailable. Reload it before making changes.';
export const BILLING_ADMIN_UNCONFIRMED_CHANGE =
  'The subscription change could not be confirmed. Reload billing state before making another change.';

const isBillingState = (value: unknown): value is BillingState => {
  if (!value || typeof value !== 'object') return false;
  const state = value as BillingState;
  const isStringArray = (items: unknown) =>
    Array.isArray(items) && items.every((item) => typeof item === 'string');
  return (
    typeof state.subscription_state === 'string' &&
    state.subscription_state.trim().length > 0 &&
    isStringArray(state.capabilities) &&
    !!state.limits &&
    typeof state.limits === 'object' &&
    !Array.isArray(state.limits) &&
    Object.values(state.limits).every(
      (limit) => typeof limit === 'number' && Number.isFinite(limit),
    ) &&
    (state.meters_enabled === undefined || isStringArray(state.meters_enabled)) &&
    ['plan_version', 'stripe_customer_id', 'stripe_subscription_id', 'stripe_price_id'].every(
      (key) =>
        state[key as keyof BillingState] === undefined ||
        typeof state[key as keyof BillingState] === 'string',
    ) &&
    [state.trial_started_at, state.trial_ends_at].every(
      (time) => time == null || (typeof time === 'number' && Number.isFinite(time)),
    )
  );
};

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
  const [billingByOrgID, setBillingByOrgID] = createSignal<BillingStateCache>(emptyRecord());
  const [billingLoadingByOrgID, setBillingLoadingByOrgID] =
    createSignal<Record<string, boolean>>(emptyRecord());
  const [billingErrorByOrgID, setBillingErrorByOrgID] =
    createSignal<Record<string, string>>(emptyRecord());
  const [savingByOrgID, setSavingByOrgID] = createSignal<Record<string, boolean>>(emptyRecord());
  const [expandedOrgID, setExpandedOrgID] = createSignal<string | null>(null);
  let generation = 0;
  let disposed = false;
  let listController: AbortController | undefined;
  const billingReads = new Map<string, AbortController>();
  const billingWrites = new Map<string, AbortController>();

  const hostedEnabled = createMemo(
    () =>
      isMultiTenantEnabled() &&
      isHostedModeEnabled() &&
      !presentationPolicyHidesOrganizationSurfaces(),
  );
  const canManageBilling = () => hostedEnabled() && !presentationPolicyIsReadOnly();
  const isCurrent = (request: number) => !disposed && request === generation && hostedEnabled();
  const hasCurrentOrganization = (orgID: string) =>
    !loadingOrgs() && !orgsError() && orgs().some((org) => org.org_id === orgID);

  const invalidateRequests = () => {
    generation += 1;
    listController?.abort();
    listController = undefined;
    for (const controller of billingReads.values()) controller.abort();
    for (const controller of billingWrites.values()) controller.abort();
    billingReads.clear();
    billingWrites.clear();
  };

  const clearSnapshot = () => {
    setOrgs([]);
    setOrgsError(null);
    setBillingByOrgID(emptyRecord());
    setBillingLoadingByOrgID(emptyRecord());
    setBillingErrorByOrgID(emptyRecord());
    setSavingByOrgID(emptyRecord());
    setExpandedOrgID(null);
  };

  const ensureBillingState = async (orgID: string, request = generation) => {
    if (
      !isCurrent(request) ||
      !hasCurrentOrganization(orgID) ||
      billingByOrgID()[orgID] ||
      billingReads.has(orgID) ||
      billingWrites.has(orgID) ||
      billingErrorByOrgID()[orgID]
    )
      return;

    const controller = new AbortController();
    billingReads.set(orgID, controller);
    const readIsCurrent = () =>
      isCurrent(request) && hasCurrentOrganization(orgID) && billingReads.get(orgID) === controller;
    setBillingLoadingByOrgID((prev) => withEntry(prev, orgID, true));
    try {
      const state = await BillingAdminAPI.getBillingState(orgID, { signal: controller.signal });
      if (!readIsCurrent()) return;
      if (!isBillingState(state)) throw new Error('Billing state returned invalid data');
      setBillingByOrgID((prev) => withEntry(prev, orgID, state));
    } catch (err) {
      if (!readIsCurrent()) return;
      logger.error('Failed to fetch billing state', { orgID, err });
      setBillingErrorByOrgID((prev) => withEntry(prev, orgID, BILLING_ADMIN_READ_ERROR));
    } finally {
      if (readIsCurrent()) {
        billingReads.delete(orgID);
        setBillingLoadingByOrgID((prev) => withEntry(prev, orgID, false));
      }
    }
  };

  const refreshOrganizations = async () => {
    if (disposed) return;
    invalidateRequests();
    const request = generation;
    batch(() => {
      clearSnapshot();
      setLoadingOrgs(hostedEnabled());
    });
    if (!hostedEnabled()) return;

    const controller = new AbortController();
    listController = controller;
    let next: HostedOrganizationSummary[];
    try {
      next = await BillingAdminAPI.listOrganizations({ signal: controller.signal });
      if (!isCurrent(request)) return;
      if (
        !Array.isArray(next) ||
        next.some(
          (org) =>
            !org ||
            typeof org.org_id !== 'string' ||
            !org.org_id.trim() ||
            org.org_id !== org.org_id.trim(),
        ) ||
        new Set(next.map((org) => org.org_id)).size !== next.length
      )
        throw new Error('Hosted organizations returned invalid data');
      setOrgs(next);
    } catch (err) {
      if (!isCurrent(request)) return;
      logger.error('Failed to list hosted organizations', err);
      setOrgsError(getOrganizationSettingsLoadErrorMessage('', 'billing-admin'));
      return;
    } finally {
      if (isCurrent(request)) {
        listController = undefined;
        setLoadingOrgs(false);
      }
    }

    // One bounded prefetch per accepted inventory, not a reactive retry loop.
    // Invalidation also prevents the old pool from admitting its queued rows.
    await promisePool(next, 6, async (org) => {
      await ensureBillingState(org.org_id, request);
    });
  };

  const updateSubscriptionState = async (orgID: string, nextState: 'suspended' | 'active') => {
    const request = generation;
    const current = billingByOrgID()[orgID];
    if (
      !isCurrent(request) ||
      !canManageBilling() ||
      !hasCurrentOrganization(orgID) ||
      !current ||
      billingReads.has(orgID) ||
      billingWrites.has(orgID) ||
      billingErrorByOrgID()[orgID] ||
      current.subscription_state.toLowerCase() === nextState
    )
      return;

    const controller = new AbortController();
    billingWrites.set(orgID, controller);
    const writeIsCurrent = () =>
      isCurrent(request) &&
      canManageBilling() &&
      hasCurrentOrganization(orgID) &&
      billingWrites.get(orgID) === controller;
    setSavingByOrgID((prev) => withEntry(prev, orgID, true));
    try {
      const planVersion = current.plan_version?.trim() || undefined;
      const payload: BillingState = {
        ...current,
        subscription_state: nextState,
        ...(planVersion ? { plan_version: planVersion } : {}),
      };
      const saved = await BillingAdminAPI.putBillingState(orgID, payload, {
        signal: controller.signal,
      });
      if (!writeIsCurrent()) return;
      if (!isBillingState(saved) || saved.subscription_state.toLowerCase() !== nextState) {
        throw new Error('Billing update did not confirm the requested subscription state');
      }
      setBillingByOrgID((prev) => withEntry(prev, orgID, saved));
      notificationStore.success(getBillingAdminStateUpdateSuccessMessage(nextState), 2500);
    } catch (err) {
      if (!writeIsCurrent()) return;
      logger.error('Failed to update billing state', { orgID, err });
      batch(() => {
        setBillingByOrgID((prev) => withEntry(prev, orgID, undefined));
        setBillingErrorByOrgID((prev) => withEntry(prev, orgID, BILLING_ADMIN_UNCONFIRMED_CHANGE));
      });
    } finally {
      if (writeIsCurrent()) {
        billingWrites.delete(orgID);
        setSavingByOrgID((prev) => withEntry(prev, orgID, false));
      }
    }
  };

  const toggleExpandedOrganization = async (orgID: string) => {
    if (!isCurrent(generation) || !hasCurrentOrganization(orgID)) return;
    const opening = expandedOrgID() !== orgID;
    setExpandedOrgID(opening ? orgID : null);
    if (opening) await ensureBillingState(orgID);
  };

  const reloadOrganization = async (orgID: string) => {
    if (!isCurrent(generation) || !hasCurrentOrganization(orgID) || billingWrites.has(orgID))
      return;
    billingReads.get(orgID)?.abort();
    billingReads.delete(orgID);
    batch(() => {
      setBillingByOrgID((prev) => withEntry(prev, orgID, undefined));
      setBillingErrorByOrgID((prev) => {
        const next = Object.assign(emptyRecord<string>(), prev);
        delete next[orgID];
        return next;
      });
    });
    await ensureBillingState(orgID);
  };

  createEffect(
    on([hostedEnabled, presentationPolicyIsReadOnly], () => void refreshOrganizations()),
  );
  onCleanup(() => {
    disposed = true;
    invalidateRequests();
    batch(() => {
      clearSnapshot();
      setLoadingOrgs(false);
    });
  });

  return {
    billingByOrgID,
    billingErrorByOrgID,
    billingLoadingByOrgID,
    canManageBilling,
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
