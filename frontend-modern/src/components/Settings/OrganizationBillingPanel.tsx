import { Component, Show, createSignal, onMount } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { LicenseAPI, type LicenseStatus } from '@/api/license';
import { OrgsAPI } from '@/api/orgs';
import { getOrgID } from '@/utils/apiClient';
import { isMultiTenantEnabled } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import CreditCard from 'lucide-solid/icons/credit-card';

interface OrganizationBillingPanelProps {
  nodeUsage: number;
  guestUsage: number;
}

const PULSE_PRO_URL = 'https://pulserelay.pro/';
const PULSE_PRO_MANAGE_URL = 'https://pulserelay.pro/manage';

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
      notificationStore.error('Failed to load billing and plan details');
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    if (!isMultiTenantEnabled()) return;
    void loadBillingData();
  });

  return (
    <Show when={isMultiTenantEnabled()} fallback={<div class="p-4 text-sm text-gray-500">This feature is not available.</div>}>
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
                    <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3 space-y-2">
                      <div class="h-3 w-20 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
                      <div class="h-5 w-24 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
                    </div>
                  ))}
                </div>

                <div class="space-y-3 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
                  <div class="h-4 w-36 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
                  {Array.from({ length: 2 }).map(() => (
                    <div class="space-y-2">
                      <div class="flex items-center justify-between">
                        <div class="h-3 w-14 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
                        <div class="h-3 w-20 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
                      </div>
                      <div class="h-2 w-full animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
                    </div>
                  ))}
                </div>

                <div class="grid gap-3 sm:grid-cols-2">
                  {Array.from({ length: 2 }).map(() => (
                    <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3 space-y-2">
                      <div class="h-3 w-24 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
                      <div class="h-5 w-40 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
                    </div>
                  ))}
                </div>

                <div class="flex flex-wrap items-center gap-2">
                  <div class="h-10 w-40 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
                  <div class="h-10 w-40 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
                </div>
              </div>
            }
          >
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Plan Tier</p>
              <p class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">
                {tierLabel[status()?.tier || 'free'] || status()?.tier || 'Free'}
              </p>
            </div>
            <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">License Status</p>
              <p class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">
                {status()?.valid ? (status()?.in_grace_period ? 'Grace Period' : 'Active') : 'No License'}
              </p>
            </div>
            <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Organizations</p>
              <p class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{orgCount()}</p>
            </div>
            <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Members (Current Org)</p>
              <p class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{memberCount()}</p>
            </div>
          </div>

          <div class="space-y-3 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Usage vs Plan Limits</h4>

            <div class="space-y-1">
              <div class="flex items-center justify-between text-xs text-gray-600 dark:text-gray-400">
                <span>Nodes</span>
                <span>
                  {props.nodeUsage}
                  {typeof nodeLimit() === 'number' ? ` / ${nodeLimit()}` : ' / Unlimited'}
                </span>
              </div>
              <Show when={typeof nodeLimit() === 'number'}>
                <div class="h-2 w-full rounded bg-gray-200 dark:bg-gray-700">
                  <div
                    class="h-2 rounded bg-blue-600 dark:bg-blue-500"
                    style={{ width: `${ratio(props.nodeUsage, nodeLimit())}%` }}
                  />
                </div>
              </Show>
            </div>

            <div class="space-y-1">
              <div class="flex items-center justify-between text-xs text-gray-600 dark:text-gray-400">
                <span>Guests</span>
                <span>
                  {props.guestUsage}
                  {typeof guestLimit() === 'number' ? ` / ${guestLimit()}` : ' / Unlimited'}
                </span>
              </div>
              <Show when={typeof guestLimit() === 'number'}>
                <div class="h-2 w-full rounded bg-gray-200 dark:bg-gray-700">
                  <div
                    class="h-2 rounded bg-emerald-600 dark:bg-emerald-500"
                    style={{ width: `${ratio(props.guestUsage, guestLimit())}%` }}
                  />
                </div>
              </Show>
            </div>
          </div>

          <div class="grid gap-3 sm:grid-cols-2">
            <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Licensed Email</p>
              <p class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{status()?.email || 'Not configured'}</p>
            </div>
            <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Renews / Expires</p>
              <p class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">
                {status()?.is_lifetime ? 'Never (Lifetime)' : formatDate(status()?.expires_at)}
              </p>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <a
              href={PULSE_PRO_URL}
              target="_blank"
              rel="noreferrer"
              class="inline-flex items-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700"
            >
              View Upgrade Options
            </a>
            <Show when={status()?.valid && status()?.tier !== 'free' && !status()?.is_lifetime}>
              <a
                href={`${PULSE_PRO_MANAGE_URL}?email=${encodeURIComponent(status()?.email || '')}`}
                target="_blank"
                rel="noreferrer"
                class="inline-flex items-center rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:border-gray-600 dark:text-gray-200 dark:hover:bg-gray-800"
              >
                Manage Subscription
              </a>
            </Show>
          </div>
          </Show>
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationBillingPanel;
