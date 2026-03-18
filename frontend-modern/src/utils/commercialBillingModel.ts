import type { LicenseStatus } from '@/api/license';

export interface CommercialStatValue {
  label: string;
  value: string | number;
}

export interface CommercialUsageValue {
  label: string;
  current: number;
  limit?: number;
  accentClass: string;
}

export interface CommercialPlanViewModel {
  summary: CommercialStatValue[];
  details: CommercialStatValue[];
}

export interface CommercialUsageViewModel {
  meters: CommercialUsageValue[];
}

export interface SelfHostedCommercialModelInput {
  licensedEmail?: string;
  statusLabel: string;
  tierLabel: string;
  planTerms?: string;
  expires: string;
  daysRemaining: string | number;
  monitoredSystems: number;
  monitoredSystemLimit?: number;
  remainingSystemCapacity: string | number;
  maxMonitoredSystems: string | number;
  maxGuests: string | number;
}

export interface HostedCommercialModelInput {
  status?: Pick<LicenseStatus, 'email' | 'is_lifetime' | 'expires_at' | 'max_monitored_systems' | 'max_guests'> | null;
  tierLabel: string;
  licenseStatusLabel: string;
  organizationCount: number;
  memberCount: number;
  nodeUsage: number;
  guestUsage: number;
  renewsOrExpires: string;
}

const asUnlimitedLimit = (value?: number) =>
  typeof value === 'number' && value > 0 ? value : undefined;

export const buildSelfHostedCommercialPlanModel = (
  input: SelfHostedCommercialModelInput,
): CommercialPlanViewModel => ({
  summary: [
    {
      label: 'Monitored Systems',
      value:
        typeof input.monitoredSystemLimit === 'number' && input.monitoredSystemLimit > 0
          ? `${input.monitoredSystems} / ${input.monitoredSystemLimit}`
          : input.monitoredSystems,
    },
    {
      label: 'Remaining System Capacity',
      value: input.remainingSystemCapacity,
    },
    {
      label: 'Plan Status',
      value: input.statusLabel,
    },
  ],
  details: [
    {
      label: 'Tier',
      value: input.tierLabel,
    },
    {
      label: 'Licensed Email',
      value: input.licensedEmail || 'Not available',
    },
    ...(input.planTerms
      ? [
          {
            label: 'Plan Terms',
            value: input.planTerms,
          },
        ]
      : []),
    {
      label: 'Expires',
      value: input.expires,
    },
    {
      label: 'Days Remaining',
      value: input.daysRemaining,
    },
    {
      label: 'Included Monitored Systems',
      value: input.maxMonitoredSystems,
    },
    {
      label: 'Max Guests',
      value: input.maxGuests,
    },
  ],
});

export const buildHostedCommercialPlanModel = (
  input: HostedCommercialModelInput,
): CommercialPlanViewModel => ({
  summary: [
    {
      label: 'Plan Tier',
      value: input.tierLabel,
    },
    {
      label: 'License Status',
      value: input.licenseStatusLabel,
    },
    {
      label: 'Organizations',
      value: input.organizationCount,
    },
    {
      label: 'Members (Current Org)',
      value: input.memberCount,
    },
  ],
  details: [
    {
      label: 'Licensed Email',
      value: input.status?.email || 'Not configured',
    },
    {
      label: 'Renews / Expires',
      value: input.renewsOrExpires,
    },
  ],
});

export const buildHostedCommercialUsageModel = (
  input: HostedCommercialModelInput,
): CommercialUsageViewModel => ({
  meters: [
    {
      label: 'Monitored Systems',
      current: input.nodeUsage,
      limit: asUnlimitedLimit(input.status?.max_monitored_systems),
      accentClass: 'bg-blue-600 dark:bg-blue-500',
    },
    {
      label: 'Guests',
      current: input.guestUsage,
      limit: asUnlimitedLimit(input.status?.max_guests),
      accentClass: 'bg-emerald-600 dark:bg-emerald-500',
    },
  ],
});
