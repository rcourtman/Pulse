import type {
  RuntimeInventorySource,
  RuntimeInventorySourceState,
  RuntimeInventorySourceType,
} from '@/api/runtimeInventorySources';

export interface WorkloadInventorySourceIssue {
  name: string;
  type: RuntimeInventorySourceType;
  typeLabel: string;
  state: RuntimeInventorySourceState;
  stateLabel: string;
  coverageLabel: string;
  description: string;
}

const WORKLOAD_CAPABLE_TYPES: ReadonlySet<RuntimeInventorySourceType> = new Set([
  'pve',
  'vmware',
  'docker',
  'kubernetes',
]);

const WORKLOAD_SURFACE_LABELS: Record<string, string> = {
  containers: 'containers',
  docker: 'containers',
  kubernetes: 'Kubernetes workloads',
  pods: 'pods',
  vms: 'VMs',
};
const WORKLOAD_SURFACE_ORDER = ['vms', 'containers', 'docker', 'pods', 'kubernetes'];

const CONNECTION_TYPE_LABELS: Record<RuntimeInventorySourceType, string> = {
  docker: 'Docker',
  kubernetes: 'Kubernetes',
  pve: 'Proxmox VE',
  vmware: 'VMware vCenter',
};

const STATE_RANK: Record<RuntimeInventorySourceState, number> = {
  paused: 1,
  pending: 2,
  stale: 3,
  unauthorized: 4,
  unreachable: 5,
};

const BLOCKING_STATES: ReadonlySet<RuntimeInventorySourceState> = new Set([
  'paused',
  'pending',
  'stale',
  'unauthorized',
  'unreachable',
]);

const activeWorkloadSurfaces = (source: RuntimeInventorySource): string[] => {
  const surfaces = source.surfaces ?? [];
  const seen = new Set<string>();
  const labels: string[] = [];
  const orderedSurfaces = [...surfaces].sort((left, right) => {
    const leftRank = WORKLOAD_SURFACE_ORDER.indexOf(left);
    const rightRank = WORKLOAD_SURFACE_ORDER.indexOf(right);
    const normalizedLeftRank = leftRank === -1 ? WORKLOAD_SURFACE_ORDER.length : leftRank;
    const normalizedRightRank = rightRank === -1 ? WORKLOAD_SURFACE_ORDER.length : rightRank;
    if (normalizedLeftRank !== normalizedRightRank) return normalizedLeftRank - normalizedRightRank;
    return left.localeCompare(right);
  });
  for (const surface of orderedSurfaces) {
    const label = WORKLOAD_SURFACE_LABELS[surface];
    if (!label || seen.has(label)) continue;
    seen.add(label);
    labels.push(label);
  }
  return labels;
};

const formatCoverage = (labels: readonly string[]): string => {
  if (labels.length === 0) return 'workload inventory';
  if (labels.length === 1) return labels[0] ?? 'workload inventory';
  if (labels.length === 2) return `${labels[0]} and ${labels[1]}`;
  return `${labels.slice(0, -1).join(', ')}, and ${labels[labels.length - 1]}`;
};

const stateLabelFor = (source: RuntimeInventorySource): string => {
  switch (source.state) {
    case 'paused':
      return 'Collection paused';
    case 'pending':
      return 'Collection pending';
    case 'stale':
      return 'Collection stale';
    case 'unreachable':
      return 'Source unreachable';
    case 'unauthorized':
      return 'Credentials invalid';
    default:
      return 'Collection blocked';
  }
};

const descriptionFor = (
  source: RuntimeInventorySource,
  typeLabel: string,
  coverageLabel: string,
): string => {
  if (source.state === 'unauthorized') {
    return `Pulse has ${coverageLabel} enabled for ${source.name}, but its ${typeLabel} API credentials are invalid.`;
  }
  switch (source.state) {
    case 'paused':
      return `Pulse has ${coverageLabel} enabled for ${source.name}, but collection is paused.`;
    case 'pending':
      return `Pulse has ${coverageLabel} enabled for ${source.name}, but collection has not completed yet.`;
    case 'stale':
      return `Pulse has ${coverageLabel} enabled for ${source.name}, but the last inventory data is stale.`;
    case 'unreachable':
      return `Pulse has ${coverageLabel} enabled for ${source.name}, but the ${typeLabel} API is unreachable.`;
    default:
      return `Pulse has ${coverageLabel} enabled for ${source.name}, but collection is blocked.`;
  }
};

const sourceHasWorkloadCoverage = (source: RuntimeInventorySource): boolean =>
  WORKLOAD_CAPABLE_TYPES.has(source.type) && activeWorkloadSurfaces(source).length > 0;

export const buildWorkloadInventorySourceIssues = (
  sources: readonly RuntimeInventorySource[],
): WorkloadInventorySourceIssue[] =>
  sources
    .filter(sourceHasWorkloadCoverage)
    .filter((source) => BLOCKING_STATES.has(source.state))
    .map((source) => {
      const coverageLabel = formatCoverage(activeWorkloadSurfaces(source));
      const typeLabel = CONNECTION_TYPE_LABELS[source.type];
      return {
        name: source.name,
        type: source.type,
        typeLabel,
        state: source.state,
        stateLabel: stateLabelFor(source),
        coverageLabel,
        description: descriptionFor(source, typeLabel, coverageLabel),
      };
    })
    .sort((left, right) => {
      const stateDelta = STATE_RANK[right.state] - STATE_RANK[left.state];
      if (stateDelta !== 0) return stateDelta;
      return left.name.localeCompare(right.name);
    });
