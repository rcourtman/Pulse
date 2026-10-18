import type { ResourceType } from '@/types/resource';
import { formatRelativeTime } from '@/utils/format';

export type PlatformAlertProvider = 'docker' | 'kubernetes' | 'truenas' | 'vmware';

const CODE_PREFIX_BY_PROVIDER: Record<PlatformAlertProvider, string> = {
  docker: 'docker_',
  kubernetes: 'k8s_',
  truenas: 'truenas_',
  vmware: 'vmware_',
};

const RESOURCE_TYPE_LABELS: Record<PlatformAlertProvider, Partial<Record<ResourceType, string>>> = {
  docker: {
    agent: 'Host',
    'app-container': 'Container',
    'docker-service': 'Service',
    'docker-task': 'Task',
    'docker-swarm-node': 'Swarm Node',
    'docker-image': 'Image',
    'docker-volume': 'Volume',
    'docker-network': 'Network',
    'docker-secret': 'Secret',
    'docker-config': 'Config',
  },
  kubernetes: {
    agent: 'Node',
    'k8s-cluster': 'Cluster',
    'k8s-node': 'Node',
    pod: 'Pod',
    'k8s-deployment': 'Deployment',
    'k8s-replicaset': 'ReplicaSet',
    'k8s-statefulset': 'StatefulSet',
    'k8s-daemonset': 'DaemonSet',
    'k8s-job': 'Job',
    'k8s-cronjob': 'CronJob',
    'k8s-service': 'Service',
    'k8s-ingress': 'Ingress',
    'k8s-namespace': 'Namespace',
    'k8s-event': 'Event',
    'k8s-persistent-volume': 'PersistentVolume',
    'k8s-persistent-volume-claim': 'PVC',
  },
  truenas: {
    agent: 'System',
    storage: 'Pool',
    pool: 'Pool',
    dataset: 'Dataset',
    physical_disk: 'Disk',
    'network-share': 'Share',
    vm: 'VM',
    'app-container': 'App',
  },
  vmware: {
    agent: 'Host',
    vm: 'VM',
    storage: 'Datastore',
  },
};

const titleCaseToken = (part: string): string =>
  part.charAt(0).toUpperCase() + part.slice(1).toLowerCase();

const isUnsetDate = (date: Date): boolean =>
  Number.isNaN(date.getTime()) || date.getUTCFullYear() < 2000;

export function formatPlatformAlertCode(code: string, provider?: PlatformAlertProvider): string {
  const trimmed = code.trim();
  const prefix = provider ? CODE_PREFIX_BY_PROVIDER[provider] : undefined;
  const normalized =
    prefix && trimmed.startsWith(prefix)
      ? trimmed.slice(prefix.length)
      : trimmed.replace(/^(docker|k8s|truenas|vmware)_/, '');

  if (!normalized) return '-';

  return normalized
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map(titleCaseToken)
    .join(' ');
}

export function formatPlatformAlertResourceType(
  type: ResourceType,
  provider: PlatformAlertProvider,
): string {
  return RESOURCE_TYPE_LABELS[provider][type] ?? type;
}

export function formatPlatformAlertEntityType(value: string): string {
  const normalized = value.trim().toLowerCase();
  if (normalized === 'host') return 'Host';
  if (normalized === 'vm') return 'VM';
  if (normalized === 'datastore') return 'Datastore';
  return normalized ? titleCaseToken(normalized) : '-';
}

// An open alert's Started cell answers how long it has been going on, so it
// reads as an age ("2h ago"), as alert cards do. The exact time is the cell's
// hover text and the drawer's Started row (formatPlatformAlertDetailDateTime).
// Tables pass the shared relative-time clock as `now` so the age keeps moving.
export function formatPlatformAlertStartedAt(value: string | undefined, now?: number): string {
  if (!value) return '-';

  const parsed = new Date(value);
  if (isUnsetDate(parsed)) return '-';

  return formatRelativeTime(parsed, { compact: true, emptyText: '-', now }) || '-';
}

export function formatPlatformAlertDetailDateTime(value?: string): string {
  if (!value) return '-';

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  if (parsed.getUTCFullYear() < 2000) return '-';

  return parsed.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}
