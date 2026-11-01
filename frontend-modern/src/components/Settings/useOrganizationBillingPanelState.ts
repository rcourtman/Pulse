import { batch, createEffect, createMemo, createSignal, on, onCleanup } from 'solid-js';
import { LicenseAPI, type LicenseStatus } from '@/api/license';
import { OrgsAPI } from '@/api/orgs';
import { getOrgID } from '@/utils/apiClient';
import { normalizeOrgScope } from '@/utils/orgScope';
import { isMultiTenantEnabled } from '@/stores/license';
import { eventBus } from '@/stores/events';
import { presentationPolicyHidesOrganizationSurfaces } from '@/stores/sessionPresentationPolicy';
import { logger } from '@/utils/logger';
import {
  getLicenseTierLabel,
  getOrganizationBillingLicenseStatusLabel,
} from '@/utils/licensePresentation';
import { getOrganizationSettingsLoadErrorMessage } from '@/utils/organizationSettingsPresentation';
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
  const [loadError, setLoadError] = createSignal<string | null>(null);
  const [status, setStatus] = createSignal<LicenseStatus | null>(null);
  const [orgCount, setOrgCount] = createSignal(0);
  const [memberCount, setMemberCount] = createSignal(0);

  const activeOrgID = () => normalizeOrgScope(getOrgID());
  const [selectedOrgID, setSelectedOrgID] = createSignal(activeOrgID());
  let generation = 0;
  let disposed = false;
  let readController: AbortController | undefined;
  const isBillingAvailable = () =>
    isMultiTenantEnabled() && !presentationPolicyHidesOrganizationSurfaces();

  const selectionIsCurrent = () =>
    !disposed && isBillingAvailable() && selectedOrgID() === activeOrgID();

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
    if (disposed) return;
    const request = ++generation;
    readController?.abort();
    readController = undefined;
    const orgId = activeOrgID();
    const isCurrent = () => request === generation && selectionIsCurrent();
    batch(() => {
      setSelectedOrgID(orgId);
      setStatus(null);
      setOrgCount(0);
      setMemberCount(0);
      setLoadError(null);
      setLoading(isBillingAvailable());
    });
    if (!isBillingAvailable()) return;

    const controller = new AbortController();
    readController = controller;
    const options = {
      expectedOrgID: orgId,
      signal: controller.signal,
      retry: false as const,
      preferSessionAuth: true as const,
    };
    try {
      const [licenseStatus, orgs, members] = await Promise.all([
        LicenseAPI.getStatus(options),
        OrgsAPI.list(options),
        OrgsAPI.listMembers(orgId, options),
      ]);
      if (!isCurrent()) return;
      if (
        !licenseStatus ||
        typeof licenseStatus.valid !== 'boolean' ||
        typeof licenseStatus.tier !== 'string' ||
        !Array.isArray(orgs) ||
        !Array.isArray(members)
      ) {
        throw new Error('Organization billing returned invalid data');
      }
      batch(() => {
        setStatus(licenseStatus);
        setOrgCount(orgs.length);
        setMemberCount(members.length);
      });
    } catch (error) {
      if (!isCurrent()) return;
      logger.error('Failed to load billing data', error);
      setLoadError(getOrganizationSettingsLoadErrorMessage('', 'billing'));
    } finally {
      if (!disposed && request === generation) {
        // Stop sibling reads too when just one Promise.all member failed.
        controller.abort();
        readController = undefined;
        setLoading(false);
        if (!selectionIsCurrent()) void loadBillingData();
      }
    }
  };

  const reloadBillingData = () => {
    if (!loading()) void loadBillingData();
  };
  const unsubscribeOrg = eventBus.on('org_switched', () => void loadBillingData());
  const unsubscribeChanged = eventBus.on('organizations_changed', () => void loadBillingData());
  createEffect(on(isBillingAvailable, () => void loadBillingData()));
  onCleanup(() => {
    disposed = true;
    generation += 1;
    readController?.abort();
    unsubscribeOrg();
    unsubscribeChanged();
  });

  return {
    commercialPlanModel,
    commercialUsageModel,
    isBillingAvailable,
    loading,
    loadError,
    reloadBillingData,
    selectedOrgID,
  };
}
