import type { LicenseStatus } from '@/api/license';
import type { MonitoredSystemContinuityStatus } from '@/api/license';
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
  monitoredSystemsSummary: string | number;
  capacityStatusSummary: string | number;
  maxMonitoredSystems: string | number;
  retailPlanDefinition?: Pick<
    SelfHostedPlanDefinition,
    'billingExtrasSummary' | 'metricHistoryDays'
  > | null;
  monitoredSystemContinuity?: MonitoredSystemContinuityStatus | null;
  continuityCapturedAt?: string;
}

export interface HostedCommercialModelInput {
  status?: Pick<
    LicenseStatus,
    'email' | 'is_lifetime' | 'expires_at' | 'max_monitored_systems' | 'max_guests'
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

const hasFiniteSelfHostedLimit = (value: string | number) =>
  typeof value === 'number'
    ? value > 0
    : !['unlimited', SELF_HOSTED_NOT_METERED_LABEL.toLowerCase()].includes(
        value.trim().toLowerCase(),
      );

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
    summary:
      !input.monitoredSystemContinuity && !hasFiniteSelfHostedLimit(input.maxMonitoredSystems)
        ? [
            {
              label: 'Core Monitoring',
              value: 'Included',
            },
            {
              label: 'Plan Status',
              value: input.statusLabel,
            },
          ]
        : [
            {
              label: 'Monitored Systems',
              value: input.monitoredSystemsSummary,
            },
            {
              label: 'Continuity Status',
              value: input.capacityStatusSummary,
            },
            {
              label: 'Plan Status',
              value: input.statusLabel,
            },
          ],
    details: [
      ...buildSelfHostedBaseDetails(input),
      ...(input.monitoredSystemContinuity
        ? [
            {
              label: 'Plan Baseline',
              value:
                input.monitoredSystemContinuity.plan_limit > 0
                  ? input.monitoredSystemContinuity.plan_limit
                  : SELF_HOSTED_NOT_METERED_LABEL,
            },
            {
              label: 'Current Baseline',
              value:
                input.monitoredSystemContinuity.effective_limit > 0
                  ? input.monitoredSystemContinuity.effective_limit
                  : SELF_HOSTED_NOT_METERED_LABEL,
            },
            ...(typeof input.monitoredSystemContinuity.grandfathered_floor === 'number' &&
            input.monitoredSystemContinuity.grandfathered_floor > 0
              ? [
                  {
                    label: 'Observed Legacy Estate',
                    value: input.monitoredSystemContinuity.grandfathered_floor,
                  },
                ]
              : []),
            {
              label: 'Continuity Verification',
              value: input.monitoredSystemContinuity.capture_pending
                ? 'Pending'
                : input.continuityCapturedAt || 'Captured',
            },
          ]
        : hasFiniteSelfHostedLimit(input.maxMonitoredSystems)
          ? [
              {
                label: 'Recorded Monitoring Baseline',
                value: input.maxMonitoredSystems,
              },
            ]
          : []),
    ],
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
