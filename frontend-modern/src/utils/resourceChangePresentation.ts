import type { ResourceChange } from '@/types/resource';
import type { ResourceChangeKind, ResourceFacetSourceAdapter } from '@/types/resource';
import { humanizeToken } from '@/utils/textPresentation';

export interface ResourceChangeLabelPresentation {
  label: string;
  plural: string;
  className: string;
}

export const RESOURCE_CHANGE_KIND_ORDER: ResourceChangeKind[] = [
  'state_transition',
  'restart',
  'config_update',
  'metric_anomaly',
  'relationship_change',
  'capability_change',
];

const RESOURCE_CHANGE_KIND_PRESENTATIONS: Record<
  ResourceChangeKind,
  ResourceChangeLabelPresentation
> = {
  state_transition: {
    label: 'State transition',
    plural: 'State transitions',
    className: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300',
  },
  restart: {
    label: 'Restart',
    plural: 'Restarts',
    className: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
  },
  config_update: {
    label: 'Config update',
    plural: 'Config updates',
    className: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
  },
  metric_anomaly: {
    label: 'Anomaly',
    plural: 'Anomalies',
    className: 'bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-300',
  },
  relationship_change: {
    label: 'Relationship change',
    plural: 'Relationship changes',
    className: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300',
  },
  capability_change: {
    label: 'Capability change',
    plural: 'Capability changes',
    className: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-300',
  },
};

type ResourceChangeSourceType =
  | 'platform_event'
  | 'pulse_diff'
  | 'heuristic'
  | 'user_action'
  | 'agent_action';

export const RESOURCE_CHANGE_SOURCE_TYPE_ORDER: ResourceChangeSourceType[] = [
  'platform_event',
  'pulse_diff',
  'heuristic',
  'user_action',
  'agent_action',
];

const RESOURCE_CHANGE_SOURCE_TYPE_PRESENTATIONS: Record<
  ResourceChangeSourceType,
  ResourceChangeLabelPresentation
> = {
  platform_event: {
    label: 'Platform event',
    plural: 'Platform events',
    className: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
  },
  pulse_diff: {
    label: 'Pulse diff',
    plural: 'Pulse diffs',
    className: 'bg-violet-100 text-violet-700 dark:bg-violet-900 dark:text-violet-300',
  },
  heuristic: {
    label: 'Heuristic',
    plural: 'Heuristics',
    className: 'bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900 dark:text-fuchsia-300',
  },
  user_action: {
    label: 'User action',
    plural: 'User actions',
    className: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
  },
  agent_action: {
    label: 'Agent action',
    plural: 'Agent actions',
    className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  },
};

export const RESOURCE_CHANGE_SOURCE_ADAPTER_ORDER: ResourceFacetSourceAdapter[] = [
  'docker_adapter',
  'proxmox_adapter',
  'truenas_adapter',
  'agent:ops-helper',
];

const RESOURCE_CHANGE_SOURCE_ADAPTER_PRESENTATIONS: Record<
  ResourceFacetSourceAdapter,
  ResourceChangeLabelPresentation
> = {
  docker_adapter: {
    label: 'Docker adapter',
    plural: 'Docker adapters',
    className: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300',
  },
  proxmox_adapter: {
    label: 'Proxmox adapter',
    plural: 'Proxmox adapters',
    className: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
  },
  truenas_adapter: {
    label: 'TrueNAS adapter',
    plural: 'TrueNAS adapters',
    className: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
  },
  'agent:ops-helper': {
    label: 'Ops helper',
    plural: 'Ops helpers',
    className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  },
};

const humanizeResourceChangeToken = (value: string): string => {
  return humanizeToken(value, { fallback: 'Unknown' });
};

const fallbackResourceChangePresentation = (value: string): ResourceChangeLabelPresentation => {
  const label = humanizeResourceChangeToken(value);
  return {
    label,
    plural: `${label}s`,
    className: 'bg-surface text-muted',
  };
};

export function getResourceChangeKindPresentation(
  kind: ResourceChangeKind | string,
): ResourceChangeLabelPresentation {
  return RESOURCE_CHANGE_KIND_PRESENTATIONS[kind as ResourceChangeKind] ?? fallbackResourceChangePresentation(String(kind));
}

export function getResourceChangeSourceTypePresentation(
  sourceType: ResourceChangeSourceType | string,
): ResourceChangeLabelPresentation {
  return (
    RESOURCE_CHANGE_SOURCE_TYPE_PRESENTATIONS[sourceType as ResourceChangeSourceType] ??
    fallbackResourceChangePresentation(String(sourceType))
  );
}

export function getResourceChangeSourceAdapterPresentation(
  sourceAdapter: ResourceFacetSourceAdapter | string,
): ResourceChangeLabelPresentation {
  return (
    RESOURCE_CHANGE_SOURCE_ADAPTER_PRESENTATIONS[sourceAdapter as ResourceFacetSourceAdapter] ??
    fallbackResourceChangePresentation(String(sourceAdapter))
  );
}

export function formatResourceChangeKind(kind: ResourceChange['kind']): string {
  switch (kind) {
    case 'state_transition':
      return 'State transition';
    case 'restart':
      return 'Restart';
    case 'config_update':
      return 'Config update';
    case 'metric_anomaly':
      return 'Metric anomaly';
    case 'relationship_change':
      return 'Relationship change';
    case 'capability_change':
      return 'Capability change';
    default:
      return humanizeToken(String(kind), { fallback: String(kind) });
  }
}

export function formatResourceChangeHeadline(change: ResourceChange): string {
  if (change.kind === 'state_transition' && change.from && change.to) {
    return `${formatResourceChangeKind(change.kind)}: ${change.from} → ${change.to}`;
  }
  if (change.kind === 'restart' && change.from && change.to) {
    return `${formatResourceChangeKind(change.kind)}: ${change.from} → ${change.to}`;
  }
  if (change.reason) {
    return `${formatResourceChangeKind(change.kind)}: ${change.reason}`;
  }
  return `${formatResourceChangeKind(change.kind)}: ${change.resourceId}`;
}

export function sortResourceChangesByObservedAt(
  changes: readonly ResourceChange[],
): ResourceChange[] {
  return [...changes].sort((left, right) => {
    const observedDelta =
      new Date(right.observedAt).getTime() - new Date(left.observedAt).getTime();
    if (observedDelta !== 0) {
      return observedDelta;
    }

    return right.id.localeCompare(left.id);
  });
}
