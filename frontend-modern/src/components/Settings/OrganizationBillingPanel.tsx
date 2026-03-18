import { Component, For, Show, createSignal, onCleanup, onMount } from 'solid-js';
import { LicenseAPI, type LicenseStatus } from '@/api/license';
import { OrgsAPI } from '@/api/orgs';
import { getOrgID } from '@/utils/apiClient';
import { isMultiTenantEnabled } from '@/stores/license';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import {
  getLicenseTierLabel,
  getOrganizationBillingLicenseStatusLabel,
} from '@/utils/licensePresentation';
import {
  getOrganizationSettingsLoadErrorMessage,
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import CreditCard from 'lucide-solid/icons/credit-card';
import {
  CommercialBillingShell,
  CommercialSection,
  CommercialStatGrid,
  CommercialUsageMeters,
} from './CommercialBillingSections';
import {
  buildHostedCommercialPlanModel,
  buildHostedCommercialUsageModel,
} from '@/utils/commercialBillingModel';

export interface OrganizationBillingPanelProps {
  nodeUsage: number;
  guestUsage: number;
}

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
  const commercialPlanModel = () =>
    buildHostedCommercialPlanModel({
      status: status(),
      tierLabel: getLicenseTierLabel(status()?.tier || 'free'),
      licenseStatusLabel: getOrganizationBillingLicenseStatusLabel(status()),
      organizationCount: orgCount(),
      memberCount: memberCount(),
      nodeUsage: props.nodeUsage,
      guestUsage: props.guestUsage,
      renewsOrExpires: status()?.is_lifetime
        ? 'Never (Lifetime)'
        : formatDate(status()?.expires_at),
    });
  const commercialUsageModel = () =>
    buildHostedCommercialUsageModel({
      status: status(),
      tierLabel: getLicenseTierLabel(status()?.tier || 'free'),
      licenseStatusLabel: getOrganizationBillingLicenseStatusLabel(status()),
      organizationCount: orgCount(),
      memberCount: memberCount(),
      nodeUsage: props.nodeUsage,
      guestUsage: props.guestUsage,
      renewsOrExpires: status()?.is_lifetime
        ? 'Never (Lifetime)'
        : formatDate(status()?.expires_at),
    });

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
      notificationStore.error(getOrganizationSettingsLoadErrorMessage(msg, 'billing'));
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
    <Show
      when={isMultiTenantEnabled()}
      fallback={
        <div class={ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS}>
          {ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE}
        </div>
      }
    >
      <div class="space-y-6">
        <CommercialBillingShell
          title="Billing & Usage"
          description="Review your organization plan, usage against limits, and available upgrade paths."
          icon={<CreditCard class="w-5 h-5" />}
          loading={loading()}
          loadingFallback={
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
            </div>
          }
        >
          <CommercialSection
            title="Plan"
            description="Review the active organization plan, subscription state, and tenant footprint tied to this billing record."
          >
              <CommercialStatGrid
                columns="four"
                items={commercialPlanModel().summary}
              />

              <div class="grid gap-3 sm:grid-cols-2">
                <For each={commercialPlanModel().details}>
                  {(item) => (
                    <div class="rounded-md border border-border p-3">
                      <p class="text-xs uppercase tracking-wide text-muted">{item.label}</p>
                      <p class="mt-1 text-sm font-medium text-base-content">
                        {item.label === 'Renews / Expires' && status()?.expires_at && !status()?.is_lifetime
                          ? formatDate(status()?.expires_at)
                          : item.value}
                      </p>
                    </div>
                  )}
                </For>
              </div>
            </CommercialSection>

          <CommercialSection
            title="Usage"
            description="Compare current agent and guest usage against the active organization allocation."
          >
              <CommercialUsageMeters
                title="Usage vs Plan Limits"
                items={commercialUsageModel().meters}
              />
            </CommercialSection>
        </CommercialBillingShell>
      </div>
    </Show>
  );
};

export default OrganizationBillingPanel;
