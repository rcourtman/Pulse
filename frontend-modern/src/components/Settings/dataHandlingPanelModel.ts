import type {
  ResourcePolicyPostureSummary,
  ResourceRedactionHint,
  ResourceRoutingScope,
  ResourceSensitivity,
} from '@/types/resource';
import {
  getResourceRedactionHintLabel,
  getResourceRoutingScopeLabel,
  getResourceSensitivityLabel,
  RESOURCE_POLICY_REDACTION_ORDER,
  RESOURCE_POLICY_ROUTING_ORDER,
  RESOURCE_POLICY_SENSITIVITY_ORDER,
} from '@/utils/resourcePolicyPresentation';

type DataHandlingTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger';

export interface DataHandlingPostureItem<T extends string = string> {
  key: T;
  label: string;
  count: number;
  percentage: number;
  tone: DataHandlingTone;
  description: string;
}

export interface DataHandlingPanelModel {
  totalResources: number;
  localOnlyResources: number;
  redactionHintCount: number;
  hasResources: boolean;
  hasRedactions: boolean;
  sensitivityItems: Array<DataHandlingPostureItem<ResourceSensitivity>>;
  routingItems: Array<DataHandlingPostureItem<ResourceRoutingScope>>;
  redactionItems: Array<DataHandlingPostureItem<ResourceRedactionHint>>;
}

const sensitivityDescriptions: Record<ResourceSensitivity, string> = {
  public: 'Safe to show broadly.',
  internal: 'Normal infrastructure detail.',
  sensitive: 'Handled with extra care.',
  restricted: 'Most guarded resource data.',
};

const routingDescriptions: Record<ResourceRoutingScope, string> = {
  'cloud-summary': 'Eligible for policy-limited summaries.',
  'local-first': 'Prefers local handling first.',
  'local-only': 'Kept inside this Pulse instance.',
};

const redactionDescriptions: Record<ResourceRedactionHint, string> = {
  hostname: 'Hostnames removed when needed.',
  'ip-address': 'IP addresses removed when needed.',
  'platform-id': 'Platform identifiers removed when needed.',
  alias: 'Aliases removed when needed.',
  path: 'Filesystem paths removed when needed.',
};

const sensitivityTone: Record<ResourceSensitivity, DataHandlingTone> = {
  public: 'success',
  internal: 'neutral',
  sensitive: 'warning',
  restricted: 'danger',
};

const routingTone: Record<ResourceRoutingScope, DataHandlingTone> = {
  'cloud-summary': 'info',
  'local-first': 'warning',
  'local-only': 'success',
};

const redactionTone: Record<ResourceRedactionHint, DataHandlingTone> = {
  hostname: 'neutral',
  'ip-address': 'neutral',
  'platform-id': 'neutral',
  alias: 'neutral',
  path: 'neutral',
};

const normalizeCount = (value: unknown): number => {
  const count = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(count)) {
    return 0;
  }
  return Math.max(0, Math.trunc(count));
};

const percentageOf = (count: number, total: number): number => {
  if (total <= 0 || count <= 0) {
    return 0;
  }
  return Math.min(100, Math.round((count / total) * 100));
};

const sumCounts = <T extends string>(counts: Partial<Record<T, number>> | undefined): number => {
  let total = 0;
  for (const value of Object.values(counts ?? {}) as Array<number | undefined>) {
    total += normalizeCount(value);
  }
  return total;
};

const buildItems = <T extends string>(
  order: readonly T[],
  counts: Partial<Record<T, number>> | undefined,
  total: number,
  labelFor: (key: T) => string,
  descriptionFor: Record<T, string>,
  toneFor: Record<T, DataHandlingTone>,
  includeZeroCounts: boolean,
): Array<DataHandlingPostureItem<T>> =>
  order.flatMap((key) => {
    const count = normalizeCount(counts?.[key]);
    if (!includeZeroCounts && count <= 0) {
      return [];
    }
    return [
      {
        key,
        label: labelFor(key),
        count,
        percentage: percentageOf(count, total),
        tone: toneFor[key],
        description: descriptionFor[key],
      },
    ];
  });

export function buildDataHandlingPanelModel(
  posture?: ResourcePolicyPostureSummary | null,
): DataHandlingPanelModel {
  const totalResources = normalizeCount(posture?.totalResources);
  const redactionHintCount = sumCounts<ResourceRedactionHint>(posture?.redactionCounts);

  return {
    totalResources,
    localOnlyResources: normalizeCount(posture?.routingCounts?.['local-only']),
    redactionHintCount,
    hasResources: totalResources > 0,
    hasRedactions: redactionHintCount > 0,
    sensitivityItems: buildItems<ResourceSensitivity>(
      RESOURCE_POLICY_SENSITIVITY_ORDER,
      posture?.sensitivityCounts,
      totalResources,
      getResourceSensitivityLabel,
      sensitivityDescriptions,
      sensitivityTone,
      true,
    ),
    routingItems: buildItems<ResourceRoutingScope>(
      RESOURCE_POLICY_ROUTING_ORDER,
      posture?.routingCounts,
      totalResources,
      getResourceRoutingScopeLabel,
      routingDescriptions,
      routingTone,
      true,
    ),
    redactionItems: buildItems<ResourceRedactionHint>(
      RESOURCE_POLICY_REDACTION_ORDER,
      posture?.redactionCounts,
      redactionHintCount,
      getResourceRedactionHintLabel,
      redactionDescriptions,
      redactionTone,
      false,
    ),
  };
}
