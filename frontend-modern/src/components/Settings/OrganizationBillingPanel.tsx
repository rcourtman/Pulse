import { Component, Show, createSignal, onCleanup, onMount } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { LicenseAPI, type LicenseStatus } from '@/api/license';
import { OrgsAPI } from '@/api/orgs';
import { getOrgID } from '@/utils/apiClient';
import { isMultiTenantEnabled } from '@/stores/license';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import CreditCard from 'lucide-solid/icons/credit-card';

interface OrganizationBillingPanelProps {
  nodeUsage: number;
  guestUsage: number;
}

const tierLabel: Record<string, string> = {
  free: 'Free',
  pro: 'Pro',
  pro_annual: 'Pro Annual',
  lifetime: 'Lifetime',
  msp: 'MSP',
  enterprise: 'Enterprise',
};

const ratio = (current: number, limit?: number) => {
  if (!limit || limit <= 0) return 0;
  if (current <= 0) return 0;
  return Math.min(100, Math.round((current / limit) * 100));
};

const formatDate = (value?: string | null) => {
  if (!value) return 'Never';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString();
};

export const OrganizationBillingPanel: Component<OrganizationBillingPanelProps> = (props) => {
  const [loading, setLoading] = createSignal(true);
  const [status, setStatus] = createSignal<LicenseStatus | null>(null);
  const [orgCount, setOrgCount] = createSignal(0);
  const [memberCount, setMemberCount] = createSignal(0);
  const nodeLimit = () => {
    const value = status()?.max_nodes;
    return typeof value === 'number' && value > 0 ? value : undefined;
  };
  const guestLimit = () => {
    const value = status()?.max_guests;
    return typeof value === 'number' && value > 0 ? value : undefined;
  };

  const loadBillingData = async () => {
    setLoading(true);
    try {
      const activeOrgID = getOrgID() || 'default';
      const [licenseStatus, orgs, members] = await Promise.all([
        LicenseAPI.getStatus(),
        OrgsAPI.list(),
        OrgsAPI.listMembers(activeOrgID),
      ]);
      setStatus(licenseStatus);
      setOrgCount(orgs.length);
      setMemberCount(members.length);
    } catch (error) {
      logger.error('Failed to load billing data', error);
      const msg = error instanceof Error ? error.message : '';
      if (msg.includes('402')) {
        notificationStore.error('Multi-tenant requires an Enterprise license');
      } else if (msg.includes('501')) {
        notificationStore.error('Multi-tenant is not enabled on this server');
      } else {
        notificationStore.error('Failed to load billing and plan details');
      }
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    if (!isMultiTenantEnabled()) return;
    void loadBillingData();

    const unsubscribe = eventBus.on('org_switched', () => {
      void loadBillingData();
    });
    onCleanup(unsubscribe);
  });

  return (
    <Show when={isMultiTenantEnabled()} fallback={<div class="p-4 text-sm text-slate-500">This feature is not available.</div>}>
      <div class="space-y-6">
        <SettingsPanel
          title="Billing & Plan"
          description="Review your current plan tier, usage against limits, and available upgrade paths."
          icon={<CreditCard class="w-5 h-5" />}
          bodyClass="space-y-5"
        >
          <Show
            when={!loading()}
            fallback={
              <div class="space-y-5">
                <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                  {Array.from({ length: 4 }).map(() => (
                    <div class="rounded-md border border-border p-3 space-y-2">
                      <div class="h-3 w-20 animate-pulse rounded bg-surface-hover" />
                      <div class="h-5 w-24 animate-pulse rounded bg-surface-hover" />
                    </div>
                  ))}
                </div>

                <div class="space-y-3 rounded-md border border-border p-4">
                  <div class="h-4 w-36 animate-pulse rounded bg-surface-hover" />
                  {Array.from({ length: 2 }).map(() => (
                    <div class="space-y-2">
                      <div class="flex items-center justify-between">
                        <div class="h-3 w-14 animate-pulse rounded bg-surface-hover" />
                        <div class="h-3 w-20 animate-pulse rounded bg-surface-hover" />
                      </div>
                      <div class="h-2 w-full animate-pulse rounded bg-surface-hover" />
                    </div>
                  ))}
                </div>

                <div class="grid gap-3 sm:grid-cols-2">
                  {Array.from({ length: 2 }).map(() => (
                    <div class="rounded-md border border-border p-3 space-y-2">
                      <div class="h-3 w-24 animate-pulse rounded bg-surface-hover" />
                      <div class="h-5 w-40 animate-pulse rounded bg-surface-hover" />
                    </div>
                  ))}
                </div>

                <div class="flex flex-wrap items-center gap-2">
                  <div class="h-10 w-40 animate-pulse rounded bg-surface-hover" />
                  <div class="h-10 w-40 animate-pulse rounded bg-surface-hover" />
                </div>
              </div>
            }
          >
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div class="rounded-md border border-border p-3">
              <p class="text-xs uppercase tracking-wide text-muted">Plan Tier</p>
              <p class="mt-1 text-sm font-medium text-base-content">
                {tierLabel[status()?.tier || 'free'] || status()?.tier || 'Free'}
              </p>
            </div>
            <div class="rounded-md border border-border p-3">
              <p class="text-xs uppercase tracking-wide text-muted">License Status</p>
              <p class="mt-1 text-sm font-medium text-base-content">
                {status()?.valid ? (status()?.in_grace_period ? 'Grace Period' : 'Active') : 'No License'}
              </p>
            </div>
            <div class="rounded-md border border-border p-3">
              <p class="text-xs uppercase tracking-wide text-muted">Organizations</p>
              <p class="mt-1 text-sm font-medium text-base-content">{orgCount()}</p>
            </div>
            <div class="rounded-md border border-border p-3">
              <p class="text-xs uppercase tracking-wide text-muted">Members (Current Org)</p>
              <p class="mt-1 text-sm font-medium text-base-content">{memberCount()}</p>
            </div>
          </div>

          <div class="space-y-3 rounded-md border border-border p-4">
            <h4 class="text-sm font-semibold text-base-content">Usage vs Plan Limits</h4>

            <div class="space-y-1">
              <div class="flex items-center justify-between text-xs text-muted">
                <span>Nodes</span>
                <span>
                  {props.nodeUsage}
                  {typeof nodeLimit() === 'number' ? ` / ${nodeLimit()}` : ' / Unlimited'}
                </span>
              </div>
              <Show when={typeof nodeLimit() === 'number'}>
                <div class="h-2 w-full rounded bg-surface-hover">
                  <div
                    class="h-2 rounded bg-blue-600 dark:bg-blue-500"
                    style={{ width: `${ratio(props.nodeUsage, nodeLimit())}%` }}
                  />
                </div>
              </Show>
            </div>

            <div class="space-y-1">
              <div class="flex items-center justify-between text-xs text-muted">
                <span>Guests</span>
                <span>
                  {props.guestUsage}
                  {typeof guestLimit() === 'number' ? ` / ${guestLimit()}` : ' / Unlimited'}
                </span>
              </div>
              <Show when={typeof guestLimit() === 'number'}>
                <div class="h-2 w-full rounded bg-surface-hover">
                  <div
                    class="h-2 rounded bg-emerald-600 dark:bg-emerald-500"
                    style={{ width: `${ratio(props.guestUsage, guestLimit())}%` }}
                  />
                </div>
              </Show>
            </div>
          </div>

          <div class="grid gap-3 sm:grid-cols-2">
            <div class="rounded-md border border-border p-3">
              <p class="text-xs uppercase tracking-wide text-muted">Licensed Email</p>
              <p class="mt-1 text-sm font-medium text-base-content">{status()?.email || 'Not configured'}</p>
            </div>
            <div class="rounded-md border border-border p-3">
              <p class="text-xs uppercase tracking-wide text-muted">Renews / Expires</p>
              <p class="mt-1 text-sm font-medium text-base-content">
                {status()?.is_lifetime ? 'Never (Lifetime)' : formatDate(status()?.expires_at)}
              </p>
            </div>
          </div>

          </Show>
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationBillingPanel;
