import { Component, Show, createEffect, createMemo, createSignal } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { BillingAdminAPI, type BillingState, type HostedOrganizationSummary } from '@/api/billingAdmin';
import { isHostedModeEnabled, isMultiTenantEnabled } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import CreditCard from 'lucide-solid/icons/credit-card';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';

type BillingStateCache = Record<string, BillingState | undefined>;

function formatUnixSeconds(value?: number | null): string {
  if (!value || value <= 0) return 'N/A';
  const date = new Date(value * 1000);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString();
}

function trialStatus(state?: BillingState): string {
  if (!state) return 'Loading...';

  const sub = (state.subscription_state || '').toLowerCase();
  if (sub !== 'trial' && !state.trial_ends_at && !state.trial_started_at) {
    return 'No trial';
  }

  const started = formatUnixSeconds(state.trial_started_at);
  const ends = formatUnixSeconds(state.trial_ends_at);
  if (sub === 'trial') {
    return `Trial (ends ${ends})`;
  }
  return `Trial (started ${started}, ends ${ends})`;
}

async function promisePool<T>(items: T[], concurrency: number, fn: (item: T) => Promise<void>) {
  if (items.length === 0) return;
  const limit = Math.max(1, Math.min(concurrency, items.length));
  let idx = 0;
  const workers = Array.from({ length: limit }).map(async () => {
    for (; ;) {
      const current = idx;
      idx += 1;
      if (current >= items.length) return;
      await fn(items[current]);
    }
  });
  await Promise.all(workers);
}

export const BillingAdminPanel: Component = () => {
  const [orgs, setOrgs] = createSignal<HostedOrganizationSummary[]>([]);
  const [loadingOrgs, setLoadingOrgs] = createSignal(false);
  const [orgsError, setOrgsError] = createSignal<string | null>(null);

  const [billingByOrgID, setBillingByOrgID] = createSignal<BillingStateCache>({});
  const [billingLoadingByOrgID, setBillingLoadingByOrgID] = createSignal<Record<string, boolean>>(
    {},
  );
  const [savingByOrgID, setSavingByOrgID] = createSignal<Record<string, boolean>>({});

  const [expandedOrgID, setExpandedOrgID] = createSignal<string | null>(null);

  const loadOrganizations = async () => {
    setLoadingOrgs(true);
    setOrgsError(null);
    try {
      const next = await BillingAdminAPI.listOrganizations();
      setOrgs(next ?? []);
    } catch (err) {
      logger.error('Failed to list hosted organizations', err);
      const msg = err instanceof Error ? err.message : 'Failed to list organizations';
      setOrgsError(msg);
      notificationStore.error(msg);
    } finally {
      setLoadingOrgs(false);
    }
  };

  const setBillingLoading = (orgID: string, value: boolean) => {
    setBillingLoadingByOrgID((prev) => ({ ...prev, [orgID]: value }));
  };

  const setSaving = (orgID: string, value: boolean) => {
    setSavingByOrgID((prev) => ({ ...prev, [orgID]: value }));
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
      const current = (await ensureBillingState(orgID)) ?? (await BillingAdminAPI.getBillingState(orgID));
      const payload: BillingState = {
        ...current,
        subscription_state: nextState,
        plan_version: current.plan_version || current.subscription_state || nextState,
        capabilities: Array.isArray(current.capabilities) ? current.capabilities : [],
        limits: current.limits ?? {},
        meters_enabled: Array.isArray(current.meters_enabled) ? current.meters_enabled : [],
      };

      const saved = await BillingAdminAPI.putBillingState(orgID, payload);
      setBillingByOrgID((prev) => ({ ...prev, [orgID]: saved }));
      notificationStore.success(
        nextState === 'suspended' ? 'Organization billing suspended' : 'Organization billing activated',
        2500,
      );
    } catch (err) {
      logger.error('Failed to update billing state', { orgID, err });
      const msg = err instanceof Error ? err.message : 'Failed to update billing state';
      notificationStore.error(msg);
    } finally {
      setSaving(orgID, false);
    }
  };

  const hostedEnabled = createMemo(() => isMultiTenantEnabled() && isHostedModeEnabled());

  createEffect(() => {
    if (!hostedEnabled()) return;
    void loadOrganizations();
  });

  // Preload billing state for the visible table so key columns can render without per-row clicks.
  createEffect(() => {
    const list = orgs();
    if (!hostedEnabled() || loadingOrgs() || list.length === 0) return;

    const missing = list.map((o) => o.org_id).filter((id) => id && !billingByOrgID()[id]);
    if (missing.length === 0) return;

    void promisePool(missing, 6, async (orgID) => {
      await ensureBillingState(orgID);
    });
  });

  const stripeCustomerCell = (state?: BillingState) => {
    const value = (state?.stripe_customer_id || '').trim();
    if (!value) return 'N/A';
    return value;
  };

  return (
    <Show
      when={hostedEnabled()}
      fallback={<div class="p-4 text-sm text-slate-500">This feature is not available.</div>}
    >
      <SettingsPanel
        title="Billing Admin"
        description="View and manage billing state across all tenants (hosted mode only)."
        icon={<CreditCard class="w-5 h-5" />}
        action={
          <button
            type="button"
            onClick={() => {
              setBillingByOrgID({});
              setExpandedOrgID(null);
              void loadOrganizations();
            }}
            disabled={loadingOrgs()}
            class="w-full sm:w-auto px-3 py-1.5 text-xs font-medium rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50"
          >
            Refresh
          </button>
        }
        bodyClass="space-y-4"
      >
        <Show when={orgsError()}>
          <div class="rounded-md border border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 p-3 text-sm text-red-800 dark:text-red-200">
            {orgsError()}
          </div>
        </Show>

        <div class="mt-4">
          <PulseDataGrid
            data={orgs()}
            isLoading={loadingOrgs()}
            emptyState="No organizations found."
            desktopMinWidth="920px"
            columns={[
              {
                key: 'organization',
                label: 'Organization',
                render: (org) => {
                  const orgID = () => (org.org_id || '').trim();
                  return (
                    <button
                      type="button"
                      class="text-left w-full"
                      onClick={() => {
                        const id = orgID();
                        if (!id) return;
                        setExpandedOrgID((prev) => (prev === id ? null : id));
                        void ensureBillingState(id);
                      }}
                    >
                      <div class="font-medium text-slate-900 dark:text-slate-100">
                        {org.display_name || org.org_id}
                      </div>
                      <div class="text-xs text-slate-500 dark:text-slate-400">
                        <span class="font-mono">{org.org_id}</span>
                        <Show when={org.soft_deleted}>
                          <span class="ml-2 rounded px-1.5 py-0.5 bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200">
                            soft-deleted
                          </span>
                        </Show>
                        <Show when={org.suspended && !org.soft_deleted}>
                          <span class="ml-2 rounded px-1.5 py-0.5 bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200">
                            suspended
                          </span>
                        </Show>
                      </div>
                    </button>
                  );
                }
              },
              {
                key: 'owner',
                label: 'Owner',
                render: (org) => <span class="font-mono text-xs text-slate-700 dark:text-slate-200">{org.owner_user_id || 'N/A'}</span>
              },
              {
                key: 'subscription',
                label: 'Subscription',
                render: (org) => {
                  const orgID = () => (org.org_id || '').trim();
                  const billing = () => billingByOrgID()[orgID()];
                  const currentSubState = () => (billing()?.subscription_state || '').toLowerCase() || 'unknown';
                  return <span class="font-mono text-xs text-slate-700 dark:text-slate-200">{currentSubState()}</span>;
                }
              },
              {
                key: 'trial',
                label: 'Trial',
                render: (org) => {
                  const orgID = () => (org.org_id || '').trim();
                  const billing = () => billingByOrgID()[orgID()];
                  return <span class="text-xs text-slate-700 dark:text-slate-200">{trialStatus(billing())}</span>;
                }
              },
              {
                key: 'stripeCustomer',
                label: 'Stripe Customer',
                render: (org) => {
                  const orgID = () => (org.org_id || '').trim();
                  const billing = () => billingByOrgID()[orgID()];
                  return (
                    <span class="font-mono text-xs text-slate-700 dark:text-slate-200" title={stripeCustomerCell(billing())}>
                      {stripeCustomerCell(billing())}
                    </span>
                  );
                }
              },
              {
                key: 'actions',
                label: 'Actions',
                align: 'right',
                render: (org) => {
                  const orgID = () => (org.org_id || '').trim();
                  const billing = () => billingByOrgID()[orgID()];
                  const currentSubState = () => (billing()?.subscription_state || '').toLowerCase() || 'unknown';
                  return (
                    <div class="inline-flex flex-col sm:flex-row sm:items-center gap-2">
                      <button
                        type="button"
                        onClick={() => {
                          const id = orgID();
                          if (!id) return;
                          void updateSubscriptionState(id, 'suspended');
                        }}
                        disabled={
                          savingByOrgID()[orgID()] ||
                          billingLoadingByOrgID()[orgID()] ||
                          currentSubState() === 'suspended'
                        }
                        class="px-2.5 py-1.5 text-xs font-medium rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50"
                      >
                        Suspend Org
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          const id = orgID();
                          if (!id) return;
                          void updateSubscriptionState(id, 'active');
                        }}
                        disabled={
                          savingByOrgID()[orgID()] ||
                          billingLoadingByOrgID()[orgID()] ||
                          currentSubState() === 'active'
                        }
                        class="px-2.5 py-1.5 text-xs font-medium rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50"
                      >
                        Activate Org
                      </button>
                    </div>
                  );
                }
              }
            ]}
            keyExtractor={(org) => org.org_id}
            isRowExpanded={(org) => expandedOrgID() === (org.org_id || '').trim()}
            expandedRender={(org) => {
              const orgID = () => (org.org_id || '').trim();
              const billing = () => billingByOrgID()[orgID()];
              return (
                <div class="px-3 pb-3 bg-slate-50/50 dark:bg-slate-800/50">
                  <div class="rounded-md border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 p-3">
                    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-2">
                      <div class="text-xs font-semibold text-slate-600 dark:text-slate-300">
                        Billing state JSON
                      </div>
                      <button
                        type="button"
                        onClick={() => {
                          const id = orgID();
                          if (!id) return;
                          setBillingByOrgID((prev) => ({ ...prev, [id]: undefined }));
                          void ensureBillingState(id);
                        }}
                        class="px-2 py-1 text-xs rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-50 dark:hover:bg-slate-800"
                      >
                        Reload
                      </button>
                    </div>
                    <pre class="text-xs overflow-x-auto whitespace-pre-wrap font-mono text-slate-800 dark:text-slate-100">
                      {JSON.stringify(billing() ?? { loading: true }, null, 2)}
                    </pre>
                  </div>
                </div>
              );
            }}
          />
        </div>
      </SettingsPanel>
    </Show>
  );
};

export default BillingAdminPanel;
