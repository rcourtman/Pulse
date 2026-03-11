export type CloudTierKey = 'starter' | 'power' | 'max';

export interface CloudPlanDefinition {
  tier: CloudTierKey;
  planVersion: string;
  name: string;
  price: string;
  subline: string;
  agents: number;
  support: 'Community' | 'Priority';
  foundingPrice?: string;
  foundingLabel?: string;
  highlighted?: boolean;
}

export const DEFAULT_CLOUD_TIER: CloudTierKey = 'starter';

export const CLOUD_PLAN_DEFINITIONS: readonly CloudPlanDefinition[] = [
  {
    tier: 'starter',
    planVersion: 'cloud_starter',
    name: 'Starter',
    price: '$29/month',
    subline: 'or $249/year (save 29%)',
    agents: 10,
    support: 'Community',
    foundingPrice: '$19/month',
    foundingLabel: '$19/mo - Founding Member rate (first 100 signups)',
    highlighted: true,
  },
  {
    tier: 'power',
    planVersion: 'cloud_power',
    name: 'Power',
    price: '$49/month',
    subline: 'or $449/year (save 24%)',
    agents: 30,
    support: 'Priority',
  },
  {
    tier: 'max',
    planVersion: 'cloud_max',
    name: 'Max',
    price: '$79/month',
    subline: 'or $699/year (save 26%)',
    agents: 75,
    support: 'Priority',
  },
] as const;

export const CLOUD_PLAN_BY_TIER: Record<CloudTierKey, CloudPlanDefinition> = {
  starter: CLOUD_PLAN_DEFINITIONS[0],
  power: CLOUD_PLAN_DEFINITIONS[1],
  max: CLOUD_PLAN_DEFINITIONS[2],
};

export const CLOUD_PLAN_LABELS: Record<string, string> = {
  cloud_starter: 'Cloud Starter',
  cloud_founding: 'Cloud Starter (Founding)',
  cloud_power: 'Cloud Power',
  cloud_max: 'Cloud Max',
  msp_starter: 'MSP Starter',
  msp_growth: 'MSP Growth',
  msp_scale: 'MSP Scale',
};

export function parseCloudTier(value?: string | null): CloudTierKey {
  switch ((value || '').trim().toLowerCase()) {
    case 'power':
      return 'power';
    case 'max':
      return 'max';
    case 'starter':
    default:
      return DEFAULT_CLOUD_TIER;
  }
}

export function getCloudPlanForTier(value?: string | null): CloudPlanDefinition {
  return CLOUD_PLAN_BY_TIER[parseCloudTier(value)];
}
