import { createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { LicenseAPI, type LicenseStatus } from '@/api/license';
import { OrgsAPI } from '@/api/orgs';
import { getOrgID } from '@/utils/apiClient';
import { normalizeOrgScope } from '@/utils/orgScope';
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
} from '@/utils/organizationSettingsPresentation';
import {
  buildHostedCommercialPlanModel,
  buildHostedCommercialUsageModel,
} from '@/utils/commercialBillingModel';

export interface OrganizationBillingPanelProps {
  nodeUsage: number;
  guestUsage: number;
}

const formatBillingDate = (value?: string | null) => {
  if (!value) return 'Never';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString();
};

export function useOrganizationBillingPanelState(props: OrganizationBillingPanelProps) {
  const [loading, setLoading] = createSignal(true);
  const [status, setStatus] = createSignal<LicenseStatus | null>(null);
  const [orgCount, setOrgCount] = createSignal(0);
  const [memberCount, setMemberCount] = createSignal(0);

  const activeOrgID = () => normalizeOrgScope(getOrgID());
  const isBillingAvailable = () => isMultiTenantEnabled();

  const commercialPlanModel = createMemo(() =>
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
        : formatBillingDate(status()?.expires_at),
    }),
  );

  const commercialUsageModel = createMemo(() =>
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
        : formatBillingDate(status()?.expires_at),
    }),
  );

  const loadBillingData = async () => {
    setLoading(true);
    try {
      const [licenseStatus, orgs, members] = await Promise.all([
        LicenseAPI.getStatus(),
        OrgsAPI.list(),
        OrgsAPI.listMembers(activeOrgID()),
      ]);
      setStatus(licenseStatus);
      setOrgCount(orgs.length);
      setMemberCount(members.length);
    } catch (error) {
      logger.error('Failed to load billing data', error);
      const message = error instanceof Error ? error.message : '';
      notificationStore.error(getOrganizationSettingsLoadErrorMessage(message, 'billing'));
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    if (!isBillingAvailable()) {
      setLoading(false);
      return;
    }

    void loadBillingData();

    const unsubscribe = eventBus.on('org_switched', () => {
      void loadBillingData();
    });
    onCleanup(unsubscribe);
  });

  return {
    commercialPlanModel,
    commercialUsageModel,
    isBillingAvailable,
    loading,
  };
}
