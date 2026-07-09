import type { Resource } from '@/types/resource';
import { isPulseAgentPlatformResource } from '@/utils/agentResources';
import { getSimpleStatusIndicator } from '@/utils/status';

export interface StandalonePageModel {
  machines: Resource[];
  availabilityChecks: Resource[];
  resources: Resource[];
}

export interface StandalonePostureSummary {
  attention: number;
  critical: number;
  latestUpdateAt?: number;
  normal: number;
  total: number;
  unknown: number;
  warning: number;
}

export const isStandaloneMachineResource = (resource: Resource): boolean =>
  isPulseAgentPlatformResource(resource);

export const isAgentlessAvailabilityResource = (resource: Resource): boolean =>
  resource.type === 'network-endpoint' ||
  resource.platformType === 'availability' ||
  resource.sources?.includes('availability') === true;

const AGENT_REPORT_STALE_AFTER_MS = 5 * 60 * 1000;

export const getStandaloneResourceStatusIndicator = (resource: Resource, nowMs = Date.now()) => {
  const indicator = getSimpleStatusIndicator(resource.status);
  if (indicator.variant === 'danger') return indicator;
  if (
    resource.type === 'agent' &&
    (resource.agent?.stale === true ||
      (Number.isFinite(resource.lastSeen) &&
        resource.lastSeen > 0 &&
        nowMs - resource.lastSeen > AGENT_REPORT_STALE_AFTER_MS))
  ) {
    return { variant: 'warning' as const, label: 'Stale' };
  }
  return indicator;
};

export function buildStandalonePostureSummary(
  resources: readonly Resource[],
  nowMs = Date.now(),
): StandalonePostureSummary {
  const summary: StandalonePostureSummary = {
    attention: 0,
    critical: 0,
    normal: 0,
    total: resources.length,
    unknown: 0,
    warning: 0,
  };

  for (const resource of resources) {
    const variant = getStandaloneResourceStatusIndicator(resource, nowMs).variant;
    if (variant === 'success') {
      summary.normal += 1;
    } else if (variant === 'danger') {
      summary.critical += 1;
      summary.attention += 1;
    } else if (variant === 'warning') {
      summary.warning += 1;
      summary.attention += 1;
    } else {
      summary.unknown += 1;
    }

    if (
      Number.isFinite(resource.lastSeen) &&
      resource.lastSeen > 0 &&
      (!summary.latestUpdateAt || resource.lastSeen > summary.latestUpdateAt)
    ) {
      summary.latestUpdateAt = resource.lastSeen;
    }
  }

  return summary;
}

const ATTENTION_SORT_PRIORITY = {
  danger: 0,
  warning: 1,
  muted: 2,
  info: 2,
  success: 3,
} as const;

export function sortStandaloneResourcesByAttention(
  resources: readonly Resource[],
  nowMs = Date.now(),
): Resource[] {
  return [...resources].sort((left, right) => {
    const leftPriority =
      ATTENTION_SORT_PRIORITY[getStandaloneResourceStatusIndicator(left, nowMs).variant];
    const rightPriority =
      ATTENTION_SORT_PRIORITY[getStandaloneResourceStatusIndicator(right, nowMs).variant];
    if (leftPriority !== rightPriority) return leftPriority - rightPriority;
    return left.displayName.localeCompare(right.displayName, undefined, { numeric: true });
  });
}

export function buildStandalonePageModel(resources: readonly Resource[]): StandalonePageModel {
  const machines = resources.filter(isStandaloneMachineResource);
  const availabilityChecks = resources.filter(isAgentlessAvailabilityResource);
  return {
    machines,
    availabilityChecks,
    resources: machines,
  };
}
