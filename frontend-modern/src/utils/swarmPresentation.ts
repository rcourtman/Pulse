export const SWARM_DRAWER_TITLE = 'Swarm';
export const SWARM_DRAWER_SEARCH_PLACEHOLDER = 'Search services...';
export const SWARM_DRAWER_NO_CLUSTER_LABEL = 'No Swarm cluster detected';
export const SWARM_DRAWER_CLUSTER_PREFIX = 'Cluster:';
export const SWARM_DRAWER_CLUSTER_ID_PREFIX = 'Cluster ID:';
export const SWARM_DRAWER_ROLE_PREFIX = 'Role:';
export const SWARM_DRAWER_STATE_PREFIX = 'State:';
export const SWARM_DRAWER_CONTROL_PREFIX = 'Control:';
export const SWARM_DRAWER_CONTROL_AVAILABLE_LABEL = 'available';
export const SWARM_DRAWER_CONTROL_UNAVAILABLE_LABEL = 'unavailable';
export const SWARM_DRAWER_COLUMN_SERVICE_LABEL = 'Service';
export const SWARM_DRAWER_COLUMN_STACK_LABEL = 'Stack';
export const SWARM_DRAWER_COLUMN_IMAGE_LABEL = 'Image';
export const SWARM_DRAWER_COLUMN_MODE_LABEL = 'Mode';
export const SWARM_DRAWER_COLUMN_DESIRED_LABEL = 'Desired';
export const SWARM_DRAWER_COLUMN_RUNNING_LABEL = 'Running';
export const SWARM_DRAWER_COLUMN_UPDATE_LABEL = 'Update';
export const SWARM_DRAWER_COLUMN_PORTS_LABEL = 'Ports';

export function getSwarmDrawerPresentation() {
  return {
    title: SWARM_DRAWER_TITLE,
    searchPlaceholder: SWARM_DRAWER_SEARCH_PLACEHOLDER,
    noClusterLabel: SWARM_DRAWER_NO_CLUSTER_LABEL,
    clusterPrefix: SWARM_DRAWER_CLUSTER_PREFIX,
    clusterIdPrefix: SWARM_DRAWER_CLUSTER_ID_PREFIX,
    rolePrefix: SWARM_DRAWER_ROLE_PREFIX,
    statePrefix: SWARM_DRAWER_STATE_PREFIX,
    controlPrefix: SWARM_DRAWER_CONTROL_PREFIX,
    controlAvailableLabel: SWARM_DRAWER_CONTROL_AVAILABLE_LABEL,
    controlUnavailableLabel: SWARM_DRAWER_CONTROL_UNAVAILABLE_LABEL,
    serviceColumnLabel: SWARM_DRAWER_COLUMN_SERVICE_LABEL,
    stackColumnLabel: SWARM_DRAWER_COLUMN_STACK_LABEL,
    imageColumnLabel: SWARM_DRAWER_COLUMN_IMAGE_LABEL,
    modeColumnLabel: SWARM_DRAWER_COLUMN_MODE_LABEL,
    desiredColumnLabel: SWARM_DRAWER_COLUMN_DESIRED_LABEL,
    runningColumnLabel: SWARM_DRAWER_COLUMN_RUNNING_LABEL,
    updateColumnLabel: SWARM_DRAWER_COLUMN_UPDATE_LABEL,
    portsColumnLabel: SWARM_DRAWER_COLUMN_PORTS_LABEL,
  } as const;
}

export function formatSwarmClusterSummary(clusterName?: string | null) {
  const value = (clusterName || '').trim();
  return value ? `${SWARM_DRAWER_CLUSTER_PREFIX} ${value}` : SWARM_DRAWER_NO_CLUSTER_LABEL;
}

export function formatSwarmClusterId(clusterId?: string | null) {
  const value = (clusterId || '').trim();
  return value ? `${SWARM_DRAWER_CLUSTER_ID_PREFIX} ${value}` : '';
}

export function formatSwarmRoleLabel(role?: string | null) {
  const value = (role || '').trim();
  return value ? `${SWARM_DRAWER_ROLE_PREFIX} ${value}` : '';
}

export function formatSwarmStateLabel(state?: string | null) {
  const value = (state || '').trim();
  return value ? `${SWARM_DRAWER_STATE_PREFIX} ${value}` : '';
}

export function formatSwarmControlLabel(controlAvailable?: boolean | null) {
  if (typeof controlAvailable !== 'boolean') return '';
  return `${SWARM_DRAWER_CONTROL_PREFIX} ${controlAvailable ? SWARM_DRAWER_CONTROL_AVAILABLE_LABEL : SWARM_DRAWER_CONTROL_UNAVAILABLE_LABEL}`;
}

export function getSwarmServicesEmptyState(hasServices: boolean) {
  return hasServices
    ? {
        title: 'No services match your filters',
        description: 'Try clearing the search.',
      }
    : {
        title: 'No Swarm services found',
        description:
          'Enable Swarm service collection in the container runtime agent (includeServices) and wait for the next report.',
      };
}

export function getSwarmServicesLoadingState() {
  return {
    text: 'Loading Swarm services...',
  } as const;
}
