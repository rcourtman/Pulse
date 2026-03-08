import type {
  Node,
  VM,
  Container,
  PBSInstance,
  Agent,
  DockerRuntime,
  DockerContainer,
  DockerService,
  ReplicationJob,
} from '@/types/api';

const ONLINE_STATUS = 'online';
const RUNNING_STATUS = 'running';
const STATUS_LABELS: Record<string, string> = {
  online: 'Online',
  offline: 'Offline',
  degraded: 'Degraded',
  paused: 'Paused',
  unknown: 'Unknown',
  running: 'Running',
  stopped: 'Stopped',
};

export const STATUS_SORT_ORDER = [
  'online',
  'degraded',
  'paused',
  'offline',
  'stopped',
  'unknown',
  'running',
] as const;

const normalize = (value?: string | null): string => (value || '').trim().toLowerCase();

export const formatStatusLabel = (value?: string | null, fallback = 'Unknown'): string => {
  if (!value) return fallback;
  const normalized = value.trim();
  if (!normalized) return fallback;
  return normalized.charAt(0).toUpperCase() + normalized.slice(1);
};

export const getCanonicalStatusLabel = (value?: string | null, fallback = 'Unknown'): string => {
  const raw = (value || '').trim();
  const normalized = normalize(raw);
  if (!normalized) return fallback;
  return STATUS_LABELS[normalized] || raw;
};

export type StatusIndicatorVariant = 'success' | 'warning' | 'danger' | 'muted';

export interface StatusIndicator {
  variant: StatusIndicatorVariant;
  label: string;
}

const defaultIndicator: StatusIndicator = { variant: 'muted', label: 'Unknown' };

const STATUS_INDICATOR_BADGE_TONE_CLASSES: Record<StatusIndicatorVariant, string> = {
  success: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
  warning: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  danger: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
  muted: 'bg-surface-alt text-base-content',
};

export const OFFLINE_HEALTH_STATUSES = new Set([
  'offline',
  'error',
  'failed',
  'down',
  'unreachable',
  'disconnected',
  'timeout',
  'stopped',
  'inactive',
]);

export const DEGRADED_HEALTH_STATUSES = new Set([
  'degraded',
  'warning',
  'maintenance',
  'syncing',
  'initializing',
  'starting',
  'pending',
  'partial',
  'unknown',
  'recovering',
  'pausing',
  'restarting',
]);

export const STOPPED_CONTAINER_STATES = new Set(['exited', 'stopped', 'created', 'paused']);
export const ERROR_CONTAINER_STATES = new Set([
  'restarting',
  'dead',
  'removing',
  'failed',
  'error',
  'oomkilled',
  'unhealthy',
]);

export function getSimpleStatusIndicator(value?: string | null): StatusIndicator {
  const normalized = normalize(value);
  if (!normalized) return defaultIndicator;

  if (OFFLINE_HEALTH_STATUSES.has(normalized)) {
    return { variant: 'danger', label: formatStatusLabel(value, 'Offline') };
  }

  if (DEGRADED_HEALTH_STATUSES.has(normalized)) {
    return { variant: 'warning', label: formatStatusLabel(value, 'Warning') };
  }

  if (normalized === ONLINE_STATUS || normalized === RUNNING_STATUS || normalized === 'healthy') {
    return {
      variant: 'success',
      label:
        normalized === 'healthy'
          ? 'Healthy'
          : normalized === RUNNING_STATUS
            ? 'Running'
            : 'Online',
    };
  }

  return { variant: 'muted', label: formatStatusLabel(value, 'Unknown') };
}

export function getStatusIndicatorBadgeToneClasses(variant: StatusIndicatorVariant): string {
  return STATUS_INDICATOR_BADGE_TONE_CLASSES[variant] || STATUS_INDICATOR_BADGE_TONE_CLASSES.muted;
}

export function isConnectedHealthStatus(value?: string | null): boolean {
  const normalized = normalize(value);
  return normalized === ONLINE_STATUS || normalized === RUNNING_STATUS || normalized === 'healthy';
}

export function isNodeOnline(node: Partial<Node> | undefined | null): boolean {
  if (!node) return false;
  if (node.status !== ONLINE_STATUS) return false;
  if ((node.uptime ?? 0) <= 0) return false;
  const connection = (node as Node).connectionHealth;
  const normalizedConnection = normalize(connection);
  if (normalizedConnection === 'offline' || normalizedConnection === 'error') return false;
  return true;
}

export function isGuestRunning(
  guest: Partial<VM | Container> | undefined | null,
  parentNodeOnline = true,
): boolean {
  if (!guest) return false;
  if (!parentNodeOnline) return false;
  return guest.status === RUNNING_STATUS;
}

export function getNodeStatusIndicator(node: Partial<Node> | undefined | null): StatusIndicator {
  if (!node) return defaultIndicator;

  const connection = normalize((node as Node).connectionHealth);
  const status = normalize(node.status);
  const uptime = node.uptime ?? 0;

  if (
    OFFLINE_HEALTH_STATUSES.has(connection) ||
    OFFLINE_HEALTH_STATUSES.has(status) ||
    uptime <= 0
  ) {
    return { variant: 'danger', label: formatStatusLabel(connection || status, 'Offline') };
  }

  if (DEGRADED_HEALTH_STATUSES.has(connection) || DEGRADED_HEALTH_STATUSES.has(status)) {
    return { variant: 'warning', label: formatStatusLabel(connection || status, 'Degraded') };
  }

  if (isNodeOnline(node)) {
    return { variant: 'success', label: 'Online' };
  }

  return defaultIndicator;
}

export function getPBSStatusIndicator(
  instance: Partial<PBSInstance> | undefined | null,
): StatusIndicator {
  if (!instance) return defaultIndicator;

  const connection = normalize(instance.connectionHealth);
  const status = normalize(instance.status);

  if (OFFLINE_HEALTH_STATUSES.has(connection) || OFFLINE_HEALTH_STATUSES.has(status)) {
    return { variant: 'danger', label: formatStatusLabel(connection || status, 'Offline') };
  }

  if (status === 'healthy' || status === ONLINE_STATUS) {
    return { variant: 'success', label: formatStatusLabel(status, 'Online') };
  }

  if (DEGRADED_HEALTH_STATUSES.has(connection) || DEGRADED_HEALTH_STATUSES.has(status)) {
    return { variant: 'warning', label: formatStatusLabel(connection || status, 'Degraded') };
  }

  return defaultIndicator;
}

export function getGuestPowerIndicator(
  guest: Partial<VM | Container> | undefined | null,
  parentNodeOnline = true,
): StatusIndicator {
  if (!guest) return defaultIndicator;
  if (!parentNodeOnline) {
    return { variant: 'danger', label: 'Node offline' };
  }
  return isGuestRunning(guest, parentNodeOnline)
    ? { variant: 'success', label: 'Running' }
    : { variant: 'danger', label: 'Stopped' };
}

export function getAgentStatusIndicator(agent: Partial<Agent> | undefined | null): StatusIndicator {
  if (!agent) return defaultIndicator;
  const status = normalize(agent.status);

  if (OFFLINE_HEALTH_STATUSES.has(status)) {
    return { variant: 'danger', label: formatStatusLabel(status, 'Offline') };
  }

  if (DEGRADED_HEALTH_STATUSES.has(status)) {
    return { variant: 'warning', label: formatStatusLabel(status, 'Degraded') };
  }

  if (status === ONLINE_STATUS || status === RUNNING_STATUS) {
    return { variant: 'success', label: 'Online' };
  }

  return status
    ? { variant: 'muted', label: formatStatusLabel(status, 'Unknown') }
    : defaultIndicator;
}

export function getDockerHostStatusIndicator(
  host: Partial<DockerRuntime> | string | undefined | null,
): StatusIndicator {
  const rawStatus = typeof host === 'string' ? host : host?.status;
  const status = normalize(rawStatus);

  if (OFFLINE_HEALTH_STATUSES.has(status)) {
    return { variant: 'danger', label: formatStatusLabel(status, 'Offline') };
  }

  if (DEGRADED_HEALTH_STATUSES.has(status)) {
    return { variant: 'warning', label: formatStatusLabel(status, 'Degraded') };
  }

  if (isConnectedHealthStatus(status)) {
    return { variant: 'success', label: formatStatusLabel(status, 'Online') };
  }

  return status
    ? { variant: 'muted', label: formatStatusLabel(status, 'Unknown') }
    : defaultIndicator;
}

export function getDockerContainerStatusIndicator(
  container: Partial<DockerContainer> | undefined | null,
): StatusIndicator {
  if (!container) return defaultIndicator;
  const state = normalize(container.state);
  const health = normalize(container.health);

  if (state === RUNNING_STATUS && (!health || health === 'healthy')) {
    return { variant: 'success', label: 'Running' };
  }

  if (ERROR_CONTAINER_STATES.has(state) || health === 'unhealthy') {
    const label = health === 'unhealthy' ? 'Unhealthy' : formatStatusLabel(state, 'Error');
    return { variant: 'danger', label };
  }

  if (STOPPED_CONTAINER_STATES.has(state)) {
    return { variant: 'danger', label: formatStatusLabel(state, 'Stopped') };
  }

  if (!state && health) {
    return { variant: 'warning', label: formatStatusLabel(health, 'Unknown') };
  }

  if (state) {
    return { variant: 'warning', label: formatStatusLabel(state, 'Unknown') };
  }

  return defaultIndicator;
}

export function getDockerServiceStatusIndicator(
  service: Partial<DockerService> | undefined | null,
): StatusIndicator {
  if (!service) return defaultIndicator;
  const desired = service.desiredTasks ?? 0;
  const running = service.runningTasks ?? 0;

  if (desired <= 0) {
    if (running > 0) {
      return { variant: 'warning', label: `Running ${running} task${running === 1 ? '' : 's'}` };
    }
    return { variant: 'muted', label: 'No tasks' };
  }

  if (running >= desired) {
    return { variant: 'success', label: 'Healthy' };
  }

  if (running === 0) {
    return { variant: 'danger', label: `Stopped (${running}/${desired})` };
  }

  return { variant: 'warning', label: `Degraded (${running}/${desired})` };
}

export function getReplicationJobStatusIndicator(
  job: Partial<ReplicationJob> | undefined | null,
): StatusIndicator {
  if (!job) return defaultIndicator;
  const status = normalize(job.status || job.state);
  const lastStatus = normalize(job.lastSyncStatus);

  if (status.includes('error') || lastStatus.includes('error')) {
    return { variant: 'danger', label: formatStatusLabel(status || lastStatus, 'Error') };
  }

  if (status.includes('sync')) {
    return { variant: 'warning', label: formatStatusLabel(status, 'Syncing') };
  }

  return { variant: 'success', label: formatStatusLabel(status, 'Idle') };
}
