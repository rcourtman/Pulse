import type { LicenseStatus } from '@/api/license';
import type { SelfHostedPlanDefinition } from '@/utils/selfHostedPlans';

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
  retailPlanDefinition?: Pick<
    SelfHostedPlanDefinition,
    'billingExtrasSummary' | 'metricHistoryDays'
  > | null;
}

export interface HostedCommercialModelInput {
  status?: Pick<
    LicenseStatus,
    'email' | 'is_lifetime' | 'expires_at' | 'max_guests'
  > | null;
  tierLabel: string;
  licenseStatusLabel: string;
  organizationCount: number;
  memberCount: number;
  nodeUsage: number;
  guestUsage: number;
  renewsOrExpires: string;
}

export const SELF_HOSTED_NOT_METERED_LABEL = 'Not metered';
export const LIFETIME_DAYS_REMAINING_LABEL = 'Permanent';

const asUnlimitedLimit = (value?: number) =>
  typeof value === 'number' && value > 0 ? value : undefined;

const buildSelfHostedBaseDetails = (
  input: SelfHostedCommercialModelInput,
): CommercialPlanViewModel['details'] => [
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
];

export const buildSelfHostedCommercialPlanModel = (
  input: SelfHostedCommercialModelInput,
): CommercialPlanViewModel => {
  if (input.retailPlanDefinition) {
    return {
      summary: [
        {
          label: 'Core Monitoring',
          value: 'Included',
        },
        {
          label: 'Metric History',
          value: `${input.retailPlanDefinition.metricHistoryDays} days`,
        },
        {
          label: 'Included Extras',
          value: input.retailPlanDefinition.billingExtrasSummary,
        },
      ],
      details: buildSelfHostedBaseDetails(input),
    };
  }

  return {
    summary: [
      {
        label: 'Core Monitoring',
        value: 'Included',
      },
      {
        label: 'Plan Status',
        value: input.statusLabel,
      },
    ],
    details: buildSelfHostedBaseDetails(input),
  };
};

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
