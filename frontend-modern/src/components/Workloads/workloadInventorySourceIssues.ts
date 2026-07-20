import type { Connection, ConnectionState, ConnectionType } from '@/api/connections';
import { formatConnectionErrorMessage } from '@/utils/connectionErrorPresentation';

export interface WorkloadInventorySourceIssue {
  id: string;
  name: string;
  type: ConnectionType;
  typeLabel: string;
  state: ConnectionState;
  stateLabel: string;
  coverageLabel: string;
  description: string;
  detail?: string;
}

const WORKLOAD_CAPABLE_TYPES: ReadonlySet<ConnectionType> = new Set([
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

const CONNECTION_TYPE_LABELS: Partial<Record<ConnectionType, string>> = {
  docker: 'Docker',
  kubernetes: 'Kubernetes',
  pve: 'Proxmox VE',
  vmware: 'VMware vCenter',
};

const STATE_RANK: Record<ConnectionState, number> = {
  active: 0,
  paused: 1,
  pending: 2,
  stale: 3,
  unauthorized: 4,
  unreachable: 5,
};

const BLOCKING_STATES: ReadonlySet<ConnectionState> = new Set([
  'paused',
  'pending',
  'stale',
  'unauthorized',
  'unreachable',
]);

const credentialInvalid = (connection: Connection): boolean =>
  connection.state === 'unauthorized' ||
  connection.fleet?.credentialStatus === 'invalid' ||
  connection.fleet?.credentialHealth?.status === 'invalid' ||
  connection.fleet?.credentialHealth?.status === 'expired';

const activeWorkloadSurfaces = (connection: Connection): string[] => {
  const scope = connection.scope ?? {};
  const scoped = Object.keys(scope).filter((surface) => scope[surface]);
  const surfaces = scoped.length > 0 ? scoped : (connection.surfaces ?? []);
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

const stateLabelFor = (connection: Connection): string => {
  if (credentialInvalid(connection)) return 'Credentials invalid';
  switch (connection.state) {
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
  connection: Connection,
  typeLabel: string,
  coverageLabel: string,
): string => {
  if (credentialInvalid(connection)) {
    return `Pulse has ${coverageLabel} enabled for ${connection.name}, but its ${typeLabel} API credentials are invalid.`;
  }
  switch (connection.state) {
    case 'paused':
      return `Pulse has ${coverageLabel} enabled for ${connection.name}, but collection is paused.`;
    case 'pending':
      return `Pulse has ${coverageLabel} enabled for ${connection.name}, but collection has not completed yet.`;
    case 'stale':
      return `Pulse has ${coverageLabel} enabled for ${connection.name}, but the last inventory data is stale.`;
    case 'unreachable':
      return `Pulse has ${coverageLabel} enabled for ${connection.name}, but the ${typeLabel} API is unreachable.`;
    default:
      return `Pulse has ${coverageLabel} enabled for ${connection.name}, but collection is blocked.`;
  }
};

const compactDetail = (raw?: string | null): string | undefined => {
  const formatted = formatConnectionErrorMessage(raw);
  if (!formatted) return undefined;
  return formatted.length > 220 ? `${formatted.slice(0, 217)}...` : formatted;
};

const connectionHasWorkloadCoverage = (connection: Connection): boolean =>
  WORKLOAD_CAPABLE_TYPES.has(connection.type) && activeWorkloadSurfaces(connection).length > 0;

export const buildWorkloadInventorySourceIssues = (
  connections: readonly Connection[],
): WorkloadInventorySourceIssue[] =>
  connections
    .filter((connection) => connection.enabled)
    .filter(connectionHasWorkloadCoverage)
    .filter((connection) => BLOCKING_STATES.has(connection.state) || credentialInvalid(connection))
    .map((connection) => {
      const coverageLabel = formatCoverage(activeWorkloadSurfaces(connection));
      const typeLabel = CONNECTION_TYPE_LABELS[connection.type] ?? connection.type;
      return {
        id: connection.id,
        name: connection.name,
        type: connection.type,
        typeLabel,
        state: connection.state,
        stateLabel: stateLabelFor(connection),
        coverageLabel,
        description: descriptionFor(connection, typeLabel, coverageLabel),
        detail: compactDetail(connection.lastError?.message ?? connection.stateReason),
      };
    })
    .sort((left, right) => {
      const stateDelta = STATE_RANK[right.state] - STATE_RANK[left.state];
      if (stateDelta !== 0) return stateDelta;
      return left.name.localeCompare(right.name);
    });
