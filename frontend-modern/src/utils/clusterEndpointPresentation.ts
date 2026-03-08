import type { ClusterEndpoint } from '@/types/nodes';

export type ClusterEndpointPulseStatus = 'reachable' | 'unreachable' | 'checking';

export interface ClusterEndpointPresentation {
  panelClass: string;
  proxmoxLabel: 'Online' | 'Offline';
  pulseLabel: 'Reachable' | 'Unreachable' | 'Checking...';
  pulseStatus: ClusterEndpointPulseStatus;
}

export function getClusterEndpointPulseStatus(
  endpoint: Pick<ClusterEndpoint, 'pulseReachable'>,
): ClusterEndpointPulseStatus {
  if (endpoint.pulseReachable === null || endpoint.pulseReachable === undefined) return 'checking';
  return endpoint.pulseReachable ? 'reachable' : 'unreachable';
}

export function getClusterEndpointPresentation(
  endpoint: Pick<ClusterEndpoint, 'online' | 'pulseReachable'>,
): ClusterEndpointPresentation {
  const pulseStatus = getClusterEndpointPulseStatus(endpoint);

  if (endpoint.online && pulseStatus === 'reachable') {
    return {
      panelClass:
        'border-green-200 bg-green-50 text-green-700 dark:border-green-700 dark:bg-green-900 dark:text-green-300',
      proxmoxLabel: 'Online',
      pulseLabel: 'Reachable',
      pulseStatus,
    };
  }

  if (pulseStatus === 'unreachable') {
    return {
      panelClass:
        'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-300',
      proxmoxLabel: endpoint.online ? 'Online' : 'Offline',
      pulseLabel: 'Unreachable',
      pulseStatus,
    };
  }

  if (endpoint.online) {
    return {
      panelClass:
        'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-300',
      proxmoxLabel: 'Online',
      pulseLabel: 'Checking...',
      pulseStatus,
    };
  }

  return {
    panelClass: 'border-border bg-surface-alt text-muted',
    proxmoxLabel: 'Offline',
    pulseLabel: pulseStatus === 'reachable' ? 'Reachable' : 'Checking...',
    pulseStatus,
  };
}
