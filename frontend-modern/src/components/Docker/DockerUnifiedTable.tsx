import { Component, For, Show, createMemo, createSignal, createEffect, Accessor } from 'solid-js';
import type { DockerHost, DockerContainer, DockerService, DockerTask } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { formatBytes, formatPercent, formatUptime, formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import type { DockerMetadata } from '@/api/dockerMetadata';
import { DockerMetadataAPI } from '@/api/dockerMetadata';
import { resolveHostRuntime } from './runtimeDisplay';
import { showSuccess, showError } from '@/utils/toast';
import { logger } from '@/utils/logger';
import { buildMetricKey } from '@/utils/metricsKeys';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  DEGRADED_HEALTH_STATUSES,
  ERROR_CONTAINER_STATES,
  OFFLINE_HEALTH_STATUSES,
  STOPPED_CONTAINER_STATES,
  getDockerContainerStatusIndicator,
  getDockerHostStatusIndicator,
  getDockerServiceStatusIndicator,
} from '@/utils/status';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { StackedMemoryBar } from '@/components/Dashboard/StackedMemoryBar';
import type { ColumnConfig } from '@/types/responsive';

const typeBadgeClass = (type: 'container' | 'service' | 'task' | 'unknown') => {
  switch (type) {
    case 'container':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200';
    case 'service':
      return 'bg-purple-50 text-purple-700 dark:bg-purple-900/30 dark:text-purple-200';
    case 'task':
      return 'bg-slate-200 text-slate-600 dark:bg-slate-700 dark:text-slate-200';
    default:
      return 'bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-300';
  }
};

type StatsFilter =
  | { type: 'host-status'; value: string }
  | { type: 'container-state'; value: string }
  | { type: 'service-health'; value: string }
  | null;

type SearchToken = { key?: string; value: string };

type DockerRow =
  | {
    kind: 'container';
    id: string;
    host: DockerHost;
    container: DockerContainer;
  }
  | {
    kind: 'service';
    id: string;
    host: DockerHost;
    service: DockerService;
    tasks: DockerTask[];
  };

interface DockerUnifiedTableProps {
  hosts: DockerHost[];
  searchTerm?: string;
  statsFilter?: StatsFilter;
  selectedHostId?: () => string | null;
  dockerMetadata?: Record<string, DockerMetadata>;
  onCustomUrlUpdate?: (resourceId: string, url: string) => void;
}

type SortKey =
  | 'host'
  | 'resource'
  | 'type'
  | 'image'
  | 'status'
  | 'cpu'
  | 'memory'
  | 'disk'
  | 'tasks'
  | 'updated';

type SortDirection = 'asc' | 'desc';

const SORT_KEYS: SortKey[] = [
  'host',
  'resource',
  'type',
  'image',
  'status',
  'cpu',
  'memory',
  'disk',
  'tasks',
  'updated',
];

const SORT_DEFAULT_DIRECTION: Record<SortKey, SortDirection> = {
  host: 'asc',
  resource: 'asc',
  type: 'asc',
  image: 'asc',
  status: 'desc',
  cpu: 'desc',
  memory: 'desc',
  disk: 'desc',
  tasks: 'desc',
  updated: 'desc',
};

// Column configuration using the priority system (matching Proxmox overview pattern)
// Extends ColumnConfig for type compatibility with useGridTemplate
interface DockerColumnDef extends ColumnConfig {
  shortLabel?: string; // Short label for narrow viewports
}

// Column definitions with responsive priorities:
// - essential: Always visible (xs and up)
// - primary: Visible on small screens and up (sm: 640px+)
// - secondary: Visible on medium screens and up (md: 768px+)
// - supplementary: Visible on large screens and up (lg: 1024px+)
// - detailed: Visible on extra large screens and up (xl: 1280px+)
export const DOCKER_COLUMNS: DockerColumnDef[] = [
  { id: 'resource', label: 'Resource', priority: 'essential', minWidth: 'auto', flex: 1, sortKey: 'resource' },
  { id: 'type', label: 'Type', priority: 'essential', minWidth: 'auto', maxWidth: 'auto', sortKey: 'type' },
  { id: 'image', label: 'Image / Stack', priority: 'essential', minWidth: 'auto', maxWidth: 'auto', sortKey: 'image' },
  { id: 'status', label: 'Status', priority: 'essential', minWidth: 'auto', maxWidth: 'auto', sortKey: 'status' },
  // Metric columns - need fixed width to match progress bar max-width (140px + padding)
  // Note: Disk column removed - Docker API rarely provides this data
  { id: 'cpu', label: 'CPU', priority: 'essential', minWidth: '55px', maxWidth: '156px', sortKey: 'cpu' },
  { id: 'memory', label: 'Memory', priority: 'essential', minWidth: '75px', maxWidth: '156px', sortKey: 'memory' },
  { id: 'tasks', label: 'Tasks', priority: 'essential', minWidth: 'auto', maxWidth: 'auto', sortKey: 'tasks' },
  { id: 'updated', label: 'Uptime', priority: 'essential', minWidth: 'auto', maxWidth: 'auto', sortKey: 'updated' },
];

// Global state for currently expanded drawer (only one drawer open at a time)
const [currentlyExpandedRowId, setCurrentlyExpandedRowId] = createSignal<string | null>(null);

// Global editing state for Docker resource URLs
const [currentlyEditingDockerResourceId, setCurrentlyEditingDockerResourceId] = createSignal<string | null>(null);
const dockerEditingValues = new Map<string, string>();
const [dockerEditingValuesVersion, setDockerEditingValuesVersion] = createSignal(0);

const toLower = (value?: string | null) => value?.toLowerCase() ?? '';

const ensureMs = (value?: number | string | null): number | null => {
  if (!value) return null;
  if (typeof value === 'number') {
    return value > 1e12 ? value : value * 1000;
  }
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? null : parsed;
};

const parseSearchTerm = (term?: string): SearchToken[] => {
  if (!term) return [];
  return term
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .map((token) => {
      const [rawKey, ...rest] = token.split(':');
      if (rest.length === 0) {
        return { value: token.toLowerCase() };
      }
      return { key: rawKey.toLowerCase(), value: rest.join(':').toLowerCase() };
    });
};

const getHostDisplayName = (host: DockerHost): string =>
  host.customDisplayName || host.displayName || host.hostname || host.id || '';

const compareStrings = (a: string, b: string) =>
  a.localeCompare(b, undefined, { sensitivity: 'base' });

const STATUS_SEVERITY: Record<string, number> = {
  error: 3,
  critical: 3,
  danger: 3,
  warning: 2,
  degraded: 2,
  offline: 2,
  alert: 2,
  info: 1,
  success: 1,
  ok: 1,
  default: 0,
};

const getResourceName = (row: DockerRow) =>
  row.kind === 'container'
    ? row.container.name || row.container.id || ''
    : row.service.name || row.service.id || '';

const getImageKey = (row: DockerRow) =>
  row.kind === 'container'
    ? row.container.image || ''
    : row.service.image || row.service.stack || '';

const getTypeSortValue = (row: DockerRow) => (row.kind === 'container' ? 0 : 1);

const getStatusSortValue = (row: DockerRow) => {
  const indicator =
    row.kind === 'container'
      ? getDockerContainerStatusIndicator(row.container)
      : getDockerServiceStatusIndicator(row.service);
  return STATUS_SEVERITY[toLower(indicator.variant)] ?? 0;
};

const getContainerCpuSortValue = (container: DockerContainer) => {
  const running = toLower(container.state) === 'running';
  const value = Number.isFinite(container.cpuPercent) ? container.cpuPercent : Number.NEGATIVE_INFINITY;
  if (!running || value <= 0) return Number.NEGATIVE_INFINITY;
  return value;
};

const getContainerMemorySortValue = (container: DockerContainer) => {
  const running = toLower(container.state) === 'running';
  const value = Number.isFinite(container.memoryPercent)
    ? container.memoryPercent
    : Number.NEGATIVE_INFINITY;
  if (!running || !container.memoryUsageBytes) return Number.NEGATIVE_INFINITY;
  return value;
};

const getContainerDiskSortValue = (container: DockerContainer) => {
  const total = container.rootFilesystemBytes ?? 0;
  const used = container.writableLayerBytes ?? 0;
  if (total <= 0 || used <= 0) return Number.NEGATIVE_INFINITY;
  return Math.min(100, (used / total) * 100);
};

const getCpuSortValue = (row: DockerRow) =>
  row.kind === 'container' ? getContainerCpuSortValue(row.container) : Number.NEGATIVE_INFINITY;

const getMemorySortValue = (row: DockerRow) =>
  row.kind === 'container' ? getContainerMemorySortValue(row.container) : Number.NEGATIVE_INFINITY;

const getDiskSortValue = (row: DockerRow) =>
  row.kind === 'container' ? getContainerDiskSortValue(row.container) : Number.NEGATIVE_INFINITY;

const getTasksSortValue = (row: DockerRow) => {
  if (row.kind === 'container') {
    const restarts = Number.isFinite(row.container.restartCount)
      ? row.container.restartCount
      : 0;
    return -restarts;
  }

  const desired = row.service.desiredTasks ?? 0;
  const running = row.service.runningTasks ?? 0;
  if (desired > 0) {
    return running / desired;
  }
  if (running > 0) return 1;
  return 0;
};

const getUpdatedSortValue = (row: DockerRow) => {
  if (row.kind === 'container') {
    const uptime = row.container.uptimeSeconds;
    if (!Number.isFinite(uptime)) return Number.NEGATIVE_INFINITY;
    return Date.now() - uptime * 1000;
  }
  const timestamp = ensureMs(row.service.updatedAt ?? row.service.createdAt);
  return timestamp ?? Number.NEGATIVE_INFINITY;
};

const compareRowsByKey = (a: DockerRow, b: DockerRow, key: SortKey) => {
  switch (key) {
    case 'host':
      return compareStrings(toLower(getHostDisplayName(a.host)), toLower(getHostDisplayName(b.host)));
    case 'resource':
      return compareStrings(toLower(getResourceName(a)), toLower(getResourceName(b)));
    case 'type':
      return getTypeSortValue(a) - getTypeSortValue(b);
    case 'image':
      return compareStrings(toLower(getImageKey(a)), toLower(getImageKey(b)));
    case 'status':
      return getStatusSortValue(a) - getStatusSortValue(b);
    case 'cpu':
      return getCpuSortValue(a) - getCpuSortValue(b);
    case 'memory':
      return getMemorySortValue(a) - getMemorySortValue(b);
    case 'disk':
      return getDiskSortValue(a) - getDiskSortValue(b);
    case 'tasks':
      return getTasksSortValue(a) - getTasksSortValue(b);
    case 'updated':
      return getUpdatedSortValue(a) - getUpdatedSortValue(b);
    default:
      return compareStrings(toLower(getResourceName(a)), toLower(getResourceName(b)));
  }
};

interface PodmanMetadataItem {
  label: string;
  value?: string;
}

interface PodmanMetadataSection {
  title: string;
  items: PodmanMetadataItem[];
}

const PODMAN_METADATA_GROUPS: Array<{
  title: string;
  prefixes?: string[];
  keys?: string[];
}> = [
    {
      title: 'Pod',
      prefixes: ['io.podman.annotations.pod.', 'io.podman.pod.', 'net.containers.podman.pod.'],
    },
    {
      title: 'Compose',
      prefixes: ['io.podman.compose.'],
    },
    {
      title: 'Auto Update',
      prefixes: ['io.containers.autoupdate.'],
      keys: ['io.containers.autoupdate'],
    },
    {
      title: 'User Namespace',
      keys: ['io.podman.annotations.userns', 'io.containers.userns'],
    },
    {
      title: 'Capabilities',
      keys: ['io.containers.capabilities', 'io.containers.selinux', 'io.containers.seccomp'],
    },
    {
      title: 'Podman Annotations',
      prefixes: ['io.podman.annotations.'],
    },
    {
      title: 'Container Settings',
      prefixes: ['io.containers.'],
    },
  ];

const humanizePodmanKey = (raw: string): string => {
  if (!raw) return 'Value';
  const cleaned = raw.replace(/[_\-.]+/g, ' ').trim();
  if (!cleaned) return 'Value';
  return cleaned
    .split(' ')
    .map((segment) => {
      if (!segment) return segment;
      if (segment.toUpperCase() === segment) return segment;
      return segment.charAt(0).toUpperCase() + segment.slice(1);
    })
    .join(' ')
    .replace(/\bId\b/g, 'ID')
    .replace(/\bUrl\b/g, 'URL');
};

const stripPrefix = (key: string, prefixes: string[] = []): string => {
  for (const prefix of prefixes) {
    if (prefix && key.startsWith(prefix)) {
      const stripped = key.slice(prefix.length);
      if (stripped) {
        return stripped;
      }
    }
  }
  const lastDot = key.lastIndexOf('.');
  if (lastDot >= 0 && lastDot < key.length - 1) {
    return key.slice(lastDot + 1);
  }
  return key;
};

const buildPodmanMetadataSections = (
  metadata?: DockerContainer['podman'],
  labels?: Record<string, string>,
): PodmanMetadataSection[] => {
  const sections: PodmanMetadataSection[] = [];
  const consumed = new Set<string>();
  const markConsumed = (...keys: (string | undefined)[]) => {
    keys.forEach((key) => {
      if (key) consumed.add(key);
    });
  };

  const pushSection = (title: string, items: PodmanMetadataItem[]) => {
    if (items.length > 0) {
      sections.push({ title, items });
    }
  };

  if (metadata) {
    const podItems: PodmanMetadataItem[] = [];
    if (metadata.podName) {
      podItems.push({ label: 'Pod Name', value: metadata.podName });
      markConsumed('io.podman.annotations.pod.name');
    }
    if (metadata.podId) {
      podItems.push({ label: 'Pod ID', value: metadata.podId });
      markConsumed('io.podman.annotations.pod.id');
    }
    if (metadata.infra !== undefined) {
      podItems.push({ label: 'Infra Container', value: metadata.infra ? 'true' : 'false' });
      markConsumed('io.podman.annotations.pod.infra');
    }
    pushSection('Pod', podItems);

    const composeItems: PodmanMetadataItem[] = [];
    if (metadata.composeProject) {
      composeItems.push({ label: 'Project', value: metadata.composeProject });
      markConsumed('io.podman.compose.project');
    }
    if (metadata.composeService) {
      composeItems.push({ label: 'Service', value: metadata.composeService });
      markConsumed('io.podman.compose.service');
    }
    if (metadata.composeWorkdir) {
      composeItems.push({ label: 'Working Dir', value: metadata.composeWorkdir });
      markConsumed('io.podman.compose.working_dir');
    }
    if (metadata.composeConfigHash) {
      composeItems.push({ label: 'Config Hash', value: metadata.composeConfigHash });
      markConsumed('io.podman.compose.config-hash');
    }
    pushSection('Compose', composeItems);

    const autoUpdateItems: PodmanMetadataItem[] = [];
    if (metadata.autoUpdatePolicy) {
      autoUpdateItems.push({ label: 'Policy', value: metadata.autoUpdatePolicy });
      markConsumed('io.containers.autoupdate');
    }
    if (metadata.autoUpdateRestart) {
      autoUpdateItems.push({ label: 'Restart', value: metadata.autoUpdateRestart });
      markConsumed('io.containers.autoupdate.restart');
    }
    pushSection('Auto Update', autoUpdateItems);

    const namespaceItems: PodmanMetadataItem[] = [];
    if (metadata.userNamespace) {
      namespaceItems.push({ label: 'User Namespace', value: metadata.userNamespace });
      markConsumed('io.podman.annotations.userns', 'io.containers.userns');
    }
    pushSection('Security', namespaceItems);
  }

  if (!labels || Object.keys(labels).length === 0) {
    return sections;
  }

  const entries = Object.entries(labels);
  const remaining = entries.filter(
    ([key]) =>
      !consumed.has(key) && (key.includes('podman') || key.startsWith('io.containers.')),
  );
  if (remaining.length === 0) {
    return sections;
  }

  const used = new Set<string>();
  const addSection = (title: string, prefixes: string[] = [], keys: string[] = []) => {
    const items: Array<[string, string]> = [];

    for (const [key, value] of remaining) {
      if (used.has(key)) continue;

      const matchesPrefix = prefixes.some((prefix) => prefix && key.startsWith(prefix));
      const matchesKey = keys.includes(key);

      if (!matchesPrefix && !matchesKey) continue;

      items.push([key, value]);
      used.add(key);
    }

    if (items.length === 0) return;

    sections.push({
      title,
      items: items.map(([key, value]) => ({
        label: humanizePodmanKey(stripPrefix(key, prefixes)),
        value: value || undefined,
      })),
    });
  };

  for (const group of PODMAN_METADATA_GROUPS) {
    addSection(group.title, group.prefixes ?? [], group.keys ?? []);
  }

  const leftovers = remaining.filter(([key]) => !used.has(key));
  if (leftovers.length > 0) {
    sections.push({
      title: 'Additional Podman Labels',
      items: leftovers.map(([key, value]) => ({
        label: humanizePodmanKey(stripPrefix(key)),
        value: value || undefined,
      })),
    });
  }

  return sections;
};

const findContainerForTask = (containers: DockerContainer[], task: DockerTask) => {
  if (!containers.length) return undefined;

  const taskId = task.containerId?.toLowerCase() ?? '';
  const taskName = task.containerName?.toLowerCase() ?? '';
  const taskNameBase = taskName.split('.')[0] || taskName;

  return containers.find((container) => {
    const id = container.id?.toLowerCase() ?? '';
    const name = container.name?.toLowerCase() ?? '';

    const idMatch = !!taskId && (id === taskId || id.includes(taskId) || taskId.includes(id));
    const nameMatch =
      !!taskName &&
      (name === taskName ||
        name.includes(taskName) ||
        taskName.includes(name) ||
        (!!taskNameBase && (name === taskNameBase || name.includes(taskNameBase))));

    return idMatch || nameMatch;
  });
};

const hostMatchesFilter = (filter: StatsFilter, host: DockerHost) => {
  if (!filter || filter.type !== 'host-status') return true;
  const status = toLower(host.status);
  if (filter.value === 'offline') {
    return OFFLINE_HEALTH_STATUSES.has(status);
  }
  if (filter.value === 'degraded') {
    return DEGRADED_HEALTH_STATUSES.has(status);
  }
  if (filter.value === 'online') {
    return status === 'online';
  }
  return true;
};

const containerMatchesStateFilter = (filter: StatsFilter, container: DockerContainer) => {
  if (!filter || filter.type !== 'container-state') return true;
  const state = toLower(container.state);
  if (filter.value === 'running') return state === 'running';
  if (filter.value === 'stopped') return STOPPED_CONTAINER_STATES.has(state);
  if (filter.value === 'error') {
    return ERROR_CONTAINER_STATES.has(state) || toLower(container.health) === 'unhealthy';
  }
  return true;
};

const serviceMatchesHealthFilter = (filter: StatsFilter, service: DockerService) => {
  if (!filter || filter.type !== 'service-health') return true;
  const desired = service.desiredTasks ?? 0;
  const running = service.runningTasks ?? 0;
  if (filter.value === 'degraded') {
    return desired > 0 && running < desired;
  }
  if (filter.value === 'healthy') {
    return desired > 0 && running >= desired;
  }
  return true;
};

const containerMatchesToken = (
  token: SearchToken,
  host: DockerHost,
  container: DockerContainer,
) => {
  const state = toLower(container.state);
  const health = toLower(container.health);
  const hostName = toLower(host.customDisplayName ?? host.displayName ?? host.hostname ?? host.id);

  if (token.key === 'name') {
    return (
      toLower(container.name).includes(token.value) ||
      toLower(container.id).includes(token.value)
    );
  }

  if (token.key === 'image') {
    return toLower(container.image).includes(token.value);
  }

  if (token.key === 'host') {
    return hostName.includes(token.value);
  }

  if (token.key === 'pod') {
    const pod = container.podman?.podName?.toLowerCase() ?? '';
    return pod.includes(token.value);
  }

  if (token.key === 'compose') {
    const project = container.podman?.composeProject?.toLowerCase() ?? '';
    const service = container.podman?.composeService?.toLowerCase() ?? '';
    return project.includes(token.value) || service.includes(token.value);
  }

  if (token.key === 'state') {
    return state.includes(token.value) || health.includes(token.value);
  }

  const fields: string[] = [
    container.name,
    container.id,
    container.image,
    container.status,
    container.state,
    container.health,
    host.displayName,
    host.hostname,
    host.id,
  ]
    .filter(Boolean)
    .map((value) => value!.toLowerCase());

  if (container.podman) {
    [
      container.podman.podName,
      container.podman.podId,
      container.podman.composeProject,
      container.podman.composeService,
      container.podman.autoUpdatePolicy,
      container.podman.userNamespace,
    ]
      .filter(Boolean)
      .forEach((value) => fields.push(value!.toLowerCase()));
  }

  if (container.labels) {
    Object.entries(container.labels).forEach(([key, value]) => {
      fields.push(key.toLowerCase());
      if (value) fields.push(value.toLowerCase());
    });
  }

  if (container.ports) {
    container.ports.forEach((port) => {
      const parts = [port.privatePort, port.publicPort, port.protocol, port.ip]
        .filter(Boolean)
        .map(String)
        .join(':')
        .toLowerCase();
      if (parts) fields.push(parts);
    });
  }

  return fields.some((field) => field.includes(token.value));
};

const serviceMatchesToken = (token: SearchToken, host: DockerHost, service: DockerService) => {
  const hostName = toLower(host.customDisplayName ?? host.displayName ?? host.hostname ?? host.id);
  const serviceName = toLower(service.name ?? service.id);
  const image = toLower(service.image);

  if (token.key === 'name') {
    return serviceName.includes(token.value);
  }

  if (token.key === 'image') {
    return image.includes(token.value);
  }

  if (token.key === 'host') {
    return hostName.includes(token.value);
  }

  if (token.key === 'state') {
    const desired = service.desiredTasks ?? 0;
    const running = service.runningTasks ?? 0;
    const status = desired > 0 && running >= desired ? 'healthy' : 'degraded';
    return status.includes(token.value);
  }

  const fields: string[] = [
    service.name,
    service.id,
    service.image,
    service.stack,
    service.mode,
    host.displayName,
    host.hostname,
    host.id,
  ]
    .filter(Boolean)
    .map((value) => value!.toLowerCase());

  if (service.labels) {
    Object.entries(service.labels).forEach(([key, value]) => {
      fields.push(key.toLowerCase());
      if (value) fields.push(value.toLowerCase());
    });
  }

  return fields.some((field) => field.includes(token.value));
};

const serviceHealthBadge = (service: DockerService) => {
  const desired = service.desiredTasks ?? 0;
  const running = service.runningTasks ?? 0;
  if (desired === 0) {
    return {
      label: 'No tasks',
      class: 'bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-300',
    };
  }
  if (running >= desired) {
    return {
      label: 'Healthy',
      class: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
    };
  }
  return {
    label: `Degraded (${running}/${desired})`,
    class: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
  };
};

const buildRowId = (host: DockerHost, row: DockerRow) => {
  if (row.kind === 'container') {
    return `container:${host.id}:${row.container.id ?? row.container.name}`;
  }
  return `service:${host.id}:${row.service.id ?? row.service.name}`;
};

const GROUPED_RESOURCE_INDENT = 'pl-5 sm:pl-6 lg:pl-8';
const UNGROUPED_RESOURCE_INDENT = 'pl-4 sm:pl-5 lg:pl-6';

const DockerHostGroupHeader: Component<{
  host: DockerHost;
  columnCount: number;
}> = (props) => {
  const displayName = getHostDisplayName(props.host);
  const hostStatus = () => getDockerHostStatusIndicator(props.host);
  const isOnline = () => hostStatus().variant === 'success';
  return (
    <tr class="bg-gray-50 dark:bg-gray-900/40">
      <td colspan={props.columnCount} class="py-0.5 pr-2 pl-4">
        <div
          class={`flex flex-nowrap items-center gap-2 whitespace-nowrap text-sm font-semibold text-slate-700 dark:text-slate-100 ${isOnline() ? '' : 'opacity-60'}`}
          title={hostStatus().label}
        >
          <StatusDot
            variant={hostStatus().variant}
            title={hostStatus().label}
            ariaLabel={hostStatus().label}
            size="xs"
          />
          <span>{displayName}</span>
          <Show when={props.host.displayName && props.host.displayName !== props.host.hostname}>
            <span class="text-[10px] font-medium text-slate-500 dark:text-slate-400">
              ({props.host.hostname})
            </span>
          </Show>
        </div>
      </td>
    </tr>
  );
};

const DockerContainerRow: Component<{
  row: Extract<DockerRow, { kind: 'container' }>;
  isMobile: Accessor<boolean>;
  customUrl?: string;
  onCustomUrlUpdate?: (resourceId: string, url: string) => void;
  showHostContext?: boolean;
  resourceIndentClass?: string;
}> = (props) => {
  const { host, container } = props.row;
  const runtimeInfo = resolveHostRuntime(host);
  const runtimeVersion = () => host.runtimeVersion || host.dockerVersion || null;
  const hostStatus = createMemo(() => getDockerHostStatusIndicator(host));
  const hostDisplayName = () => getHostDisplayName(host);
  const rowId = buildRowId(host, props.row);
  const resourceId = () => `${host.id}:container:${container.id || container.name}`;
  const isEditingUrl = createMemo(() => currentlyEditingDockerResourceId() === resourceId());
  const resourceIndent = () => props.resourceIndentClass ?? GROUPED_RESOURCE_INDENT;

  const [customUrl, setCustomUrl] = createSignal<string | undefined>(props.customUrl);
  const [shouldAnimateIcon, setShouldAnimateIcon] = createSignal(false);
  const expanded = createMemo(() => currentlyExpandedRowId() === rowId);
  const editingUrlValue = createMemo(() => {
    dockerEditingValuesVersion(); // Subscribe to changes
    return dockerEditingValues.get(resourceId()) || '';
  });
  let urlInputRef: HTMLInputElement | undefined;

  const writableLayerBytes = createMemo(() => container.writableLayerBytes ?? 0);
  const rootFilesystemBytes = createMemo(() => container.rootFilesystemBytes ?? 0);
  const hasDiskStats = createMemo(() => writableLayerBytes() > 0 || rootFilesystemBytes() > 0);
  const diskPercent = createMemo<number | null>(() => {
    const total = rootFilesystemBytes();
    if (!total || total <= 0) return null;
    const used = writableLayerBytes();
    if (used <= 0) return 0;
    return Math.min(100, (used / total) * 100);
  });
  const diskUsageLabel = createMemo(() => {
    const used = writableLayerBytes();
    if (used <= 0) return '0 B';
    return formatBytes(used, 0);
  });
  const diskSublabel = createMemo<string | undefined>(() => {
    const total = rootFilesystemBytes();
    if (!total || total <= 0) return undefined;
    return `${diskUsageLabel()} / ${formatBytes(total, 0)}`;
  });
  const createdRelative = createMemo(() => (container.createdAt ? formatRelativeTime(container.createdAt) : null));
  const createdAbsolute = createMemo(() => (container.createdAt ? formatAbsoluteTime(container.createdAt) : null));
  const startedRelative = createMemo(() =>
    container.startedAt ? formatRelativeTime(container.startedAt) : null,
  );
  const startedAbsolute = createMemo(() =>
    container.startedAt ? formatAbsoluteTime(container.startedAt) : null,
  );
  const mounts = createMemo(() => container.mounts || []);
  const hasMounts = createMemo(() => mounts().length > 0);
  const blockIo = createMemo(() => container.blockIo);
  const blockIoReadBytes = createMemo(() => blockIo()?.readBytes ?? 0);
  const blockIoWriteBytes = createMemo(() => blockIo()?.writeBytes ?? 0);
  const blockIoReadRate = createMemo(() => blockIo()?.readRateBytesPerSecond ?? null);
  const blockIoWriteRate = createMemo(() => blockIo()?.writeRateBytesPerSecond ?? null);
  const formatIoRate = (value?: number | null) => {
    if (value === undefined || value === null) return undefined;
    if (value <= 0) return undefined;
    const decimals = value >= 1024 * 1024 ? 1 : value >= 1024 ? 1 : 0;
    return `${formatBytes(value, decimals)}/s`;
  };
  const blockIoReadRateLabel = createMemo(() => formatIoRate(blockIoReadRate()));
  const blockIoWriteRateLabel = createMemo(() => formatIoRate(blockIoWriteRate()));
  const podmanMetadata = createMemo(() => container.podman);
  const podName = createMemo(() => podmanMetadata()?.podName?.trim() || undefined);
  const isPodInfra = createMemo(() => podmanMetadata()?.infra ?? false);
  const podmanMetadataSections = createMemo(() =>
    buildPodmanMetadataSections(podmanMetadata(), container.labels),
  );
  const hasPodmanMetadata = createMemo(
    () => !!podmanMetadata() || podmanMetadataSections().length > 0,
  );
  const hasBlockIo = createMemo(() => {
    const stats = blockIo();
    if (!stats) return false;
    const read = stats.readBytes ?? 0;
    const write = stats.writeBytes ?? 0;
    const readRate = stats.readRateBytesPerSecond ?? 0;
    const writeRate = stats.writeRateBytesPerSecond ?? 0;
    return read > 0 || write > 0 || readRate > 0 || writeRate > 0;
  });
  const hasDrawerContent = createMemo(() => {
    return (
      (container.ports && container.ports.length > 0) ||
      (container.labels && Object.keys(container.labels).length > 0) ||
      (container.networks && container.networks.length > 0) ||
      hasMounts() ||
      hasBlockIo() ||
      hasPodmanMetadata()
    );
  });

  // Update custom URL when prop changes, but only if we're not currently editing
  createEffect(() => {
    if (currentlyEditingDockerResourceId() !== resourceId()) {
      const prevUrl = customUrl();
      const newUrl = props.customUrl;

      // Only animate when URL transitions from empty to having a value
      if (!prevUrl && newUrl) {
        setShouldAnimateIcon(true);
        setTimeout(() => setShouldAnimateIcon(false), 200);
      }

      setCustomUrl(newUrl);
    }
  });

  // Auto-focus the input when editing starts
  createEffect(() => {
    if (isEditingUrl() && urlInputRef) {
      urlInputRef.focus();
      urlInputRef.select();
    }
  });

  const toggle = (event: MouseEvent) => {
    if (!hasDrawerContent()) return;
    const target = event.target as HTMLElement;
    if (target.closest('a, button, input, [data-prevent-toggle]')) return;
    // Toggle: if this row is currently expanded, close it; otherwise open it (closing any other)
    setCurrentlyExpandedRowId(prev => prev === rowId ? null : rowId);
  };

  const startEditingUrl = (event: MouseEvent) => {
    event.stopPropagation();

    // If another resource is being edited, save it first
    const currentEditing = currentlyEditingDockerResourceId();
    if (currentEditing !== null && currentEditing !== resourceId()) {
      const currentInput = document.querySelector(`input[data-resource-id="${currentEditing}"]`) as HTMLInputElement;
      if (currentInput) {
        currentInput.blur();
      }
    }

    dockerEditingValues.set(resourceId(), customUrl() || '');
    setDockerEditingValuesVersion(v => v + 1);
    setCurrentlyEditingDockerResourceId(resourceId());
  };

  // Add global click handler to close editor
  createEffect(() => {
    if (isEditingUrl()) {
      const handleGlobalClick = (e: MouseEvent) => {
        if (currentlyEditingDockerResourceId() !== resourceId()) return;

        const target = e.target as HTMLElement;
        const isClickingResourceName = target.closest('[data-resource-name-editable]');

        if (!target.closest('[data-url-editor]') && !isClickingResourceName) {
          e.preventDefault();
          e.stopPropagation();
          e.stopImmediatePropagation();
          cancelEditingUrl();
        }
      };

      const handleGlobalMouseDown = (e: MouseEvent) => {
        if (currentlyEditingDockerResourceId() !== resourceId()) return;

        const target = e.target as HTMLElement;
        const isClickingResourceName = target.closest('[data-resource-name-editable]');

        if (!target.closest('[data-url-editor]') && !isClickingResourceName) {
          e.preventDefault();
          e.stopPropagation();
          e.stopImmediatePropagation();
        }
      };

      document.addEventListener('mousedown', handleGlobalMouseDown, true);
      document.addEventListener('click', handleGlobalClick, true);
      return () => {
        document.removeEventListener('mousedown', handleGlobalMouseDown, true);
        document.removeEventListener('click', handleGlobalClick, true);
      };
    }
  });

  const saveUrl = async () => {
    if (currentlyEditingDockerResourceId() !== resourceId()) return;

    const newUrl = (dockerEditingValues.get(resourceId()) || '').trim();

    // Clear global editing state
    dockerEditingValues.delete(resourceId());
    setDockerEditingValuesVersion(v => v + 1);
    setCurrentlyEditingDockerResourceId(null);

    // If URL hasn't changed, don't save
    if (newUrl === (customUrl() || '')) return;

    // Animate if transitioning from no URL to having a URL
    const hadUrl = !!customUrl();
    if (!hadUrl && newUrl) {
      setShouldAnimateIcon(true);
      setTimeout(() => setShouldAnimateIcon(false), 200);
    }

    // Optimistically update local and parent state immediately
    setCustomUrl(newUrl || undefined);
    if (props.onCustomUrlUpdate) {
      props.onCustomUrlUpdate(resourceId(), newUrl);
    }

    try {
      await DockerMetadataAPI.updateMetadata(resourceId(), { customUrl: newUrl });

      if (newUrl) {
        showSuccess('Container URL saved');
      } else {
        showSuccess('Container URL cleared');
      }
    } catch (err: any) {
      logger.error('Failed to save container URL:', err);
      showError(err.message || 'Failed to save container URL');
      // Revert on error
      setCustomUrl(hadUrl ? customUrl() : undefined);
      if (props.onCustomUrlUpdate) {
        props.onCustomUrlUpdate(resourceId(), hadUrl ? customUrl() || '' : '');
      }
    }
  };

  const deleteUrl = async () => {
    if (currentlyEditingDockerResourceId() !== resourceId()) return;

    // Clear global editing state
    dockerEditingValues.delete(resourceId());
    setDockerEditingValuesVersion(v => v + 1);
    setCurrentlyEditingDockerResourceId(null);

    // If there was a URL set, delete it
    if (customUrl()) {
      try {
        await DockerMetadataAPI.updateMetadata(resourceId(), { customUrl: '' });
        setCustomUrl(undefined);

        // Notify parent to update metadata
        if (props.onCustomUrlUpdate) {
          props.onCustomUrlUpdate(resourceId(), '');
        }

        showSuccess('Container URL removed');
      } catch (err: any) {
        logger.error('Failed to remove container URL:', err);
        showError(err.message || 'Failed to remove container URL');
      }
    }
  };

  const cancelEditingUrl = () => {
    if (currentlyEditingDockerResourceId() !== resourceId()) return;

    // Just close without saving
    dockerEditingValues.delete(resourceId());
    setDockerEditingValuesVersion(v => v + 1);
    setCurrentlyEditingDockerResourceId(null);
  };

  const cpuPercent = () => Math.max(0, Math.min(100, container.cpuPercent ?? 0));
  const memPercent = () => Math.max(0, Math.min(100, container.memoryPercent ?? 0));
  const metricsKey = buildMetricKey('dockerContainer', container.id);
  const memUsageLabel = () => {
    if (!container.memoryUsageBytes) return undefined;
    const used = formatBytes(container.memoryUsageBytes, 0);
    const limit = container.memoryLimitBytes
      ? formatBytes(container.memoryLimitBytes, 0)
      : undefined;
    return limit ? `${used} / ${limit}` : used;
  };

  const uptime = () => (container.uptimeSeconds ? formatUptime(container.uptimeSeconds) : '—');
  const restarts = () => container.restartCount ?? 0;

  const state = () => toLower(container.state);
  const health = () => toLower(container.health);
  const isRunning = () => state() === 'running';

  const statusBadgeClass = () => {
    if (state() === 'running' && (!health() || health() === 'healthy')) {
      return 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300';
    }
    if (ERROR_CONTAINER_STATES.has(state()) || health() === 'unhealthy') {
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300';
    }
    if (STOPPED_CONTAINER_STATES.has(state())) {
      return 'bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-300';
    }
    return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300';
  };
  const containerStatusIndicator = createMemo(() => getDockerContainerStatusIndicator(container));

  const statusLabel = () => {
    if (health()) {
      return `${container.state ?? 'Unknown'} (${container.health})`;
    }
    return container.status || container.state || 'Unknown';
  };

  const containerTitle = () => {
    const primary = container.name || container.id || 'Container';
    const identifier = container.id && container.name && container.id !== container.name ? container.id : '';
    return identifier ? `${primary} \u2014 ${identifier}` : primary;
  };

  // Render cell content based on column type
  const renderCell = (column: ColumnConfig) => {
    switch (column.id) {
      case 'resource':
        return (
          <div class={`${resourceIndent()} pr-2 py-0.5 overflow-hidden`}>
            <div class="flex items-center gap-1.5 min-w-0">
              <StatusDot
                variant={containerStatusIndicator().variant}
                title={statusLabel()}
                ariaLabel={containerStatusIndicator().label}
                size="xs"
              />
              <div class="flex-1 min-w-0 truncate">
                <Show
                  when={isEditingUrl()}
                  fallback={
                    <div class="flex items-center gap-1.5 flex-1 min-w-0">
                      <span
                        class="text-sm font-semibold text-gray-900 dark:text-gray-100 cursor-text select-none truncate"
                        style="cursor: text;"
                        title={`${containerTitle()}${customUrl() ? ' - Click to edit URL' : ' - Click to add URL'}`}
                        onClick={startEditingUrl}
                        data-resource-name-editable
                      >
                        {container.name || container.id}
                      </span>
                      <Show when={podName()}>
                        {(name) => (
                          <span class="inline-flex items-center gap-1 rounded bg-purple-100 px-1.5 py-0.5 text-[10px] font-medium text-purple-700 dark:bg-purple-900/40 dark:text-purple-200 flex-shrink-0">
                            Pod: {name()}
                            <Show when={isPodInfra()}>
                              <span class="rounded bg-purple-200 px-1 py-0.5 text-[9px] uppercase text-purple-800 dark:bg-purple-800/50 dark:text-purple-200 ml-1">
                                infra
                              </span>
                            </Show>
                          </span>
                        )}
                      </Show>
                      <Show when={customUrl()}>
                        <a
                          href={customUrl()}
                          target="_blank"
                          rel="noopener noreferrer"
                          class={`flex-shrink-0 text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors ${shouldAnimateIcon() ? 'animate-fadeIn' : ''}`}
                          title="Open in new tab"
                          onClick={(event) => event.stopPropagation()}
                        >
                          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                          </svg>
                        </a>
                      </Show>
                      <Show when={props.showHostContext}>
                        <span
                          class="inline-flex items-center gap-1 rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300 flex-shrink-0 max-w-[120px]"
                          title={`Host: ${hostDisplayName()}`}
                        >
                          <StatusDot variant={hostStatus().variant} title={hostStatus().label} ariaLabel={hostStatus().label} size="xs" />
                          <span class="truncate">{hostDisplayName()}</span>
                        </span>
                      </Show>
                    </div>
                  }
                >
                  <div class="flex-1 flex items-center gap-1 min-w-0" data-url-editor>
                    <input
                      ref={urlInputRef}
                      type="text"
                      value={editingUrlValue()}
                      data-resource-id={resourceId()}
                      onInput={(e) => { dockerEditingValues.set(resourceId(), e.currentTarget.value); setDockerEditingValuesVersion(v => v + 1); }}
                      onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); saveUrl(); } else if (e.key === 'Escape') { e.preventDefault(); cancelEditingUrl(); } }}
                      onClick={(e) => e.stopPropagation()}
                      placeholder="https://example.com:8080"
                      class="flex-1 min-w-0 px-2 py-0.5 text-sm border border-blue-500 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                    />
                    <button type="button" data-url-editor-button onClick={(e) => { e.stopPropagation(); saveUrl(); }} class="flex-shrink-0 w-6 h-6 flex items-center justify-center text-xs bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors" title="Save (or press Enter)">✓</button>
                    <button type="button" data-url-editor-button onClick={(e) => { e.stopPropagation(); deleteUrl(); }} class="flex-shrink-0 w-6 h-6 flex items-center justify-center text-xs bg-red-600 text-white rounded hover:bg-red-700 transition-colors" title="Delete URL">✕</button>
                  </div>
                </Show>
              </div>
            </div>
          </div>
        );
      case 'type':
        return (
          <div class="px-2 py-0.5 flex items-center overflow-hidden">
            <span class={`inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${runtimeInfo.badgeClass}`} title={runtimeVersion() ? `${runtimeInfo.label} ${runtimeVersion()}` : runtimeInfo.raw || runtimeInfo.label}>
              {runtimeInfo.label}
            </span>
          </div>
        );
      case 'image':
        return (
          <div class="px-2 py-0.5 text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
            {container.image || '—'}
          </div>
        );
      case 'status':
        return (
          <div class="px-2 py-0.5 text-xs whitespace-nowrap">
            <span class={`rounded px-2 py-0.5 text-[10px] font-medium ${statusBadgeClass()}`}>{statusLabel()}</span>
          </div>
        );
      case 'cpu':
        return (
          <div class="px-2 py-0.5 flex items-center overflow-hidden">
            <ResponsiveMetricCell
              value={cpuPercent()}
              type="cpu"
              resourceId={metricsKey}
              isRunning={isRunning() && (container.cpuPercent ?? 0) > 0}
              showMobile={props.isMobile()}
              class="w-full"
            />
          </div>
        );
      case 'memory':
        const memoryTotal = () => container.memoryLimitBytes && container.memoryLimitBytes > 0
          ? container.memoryLimitBytes
          : host.totalMemoryBytes;

        return (
          <div class="px-2 py-0.5 flex items-center overflow-hidden">
            <Show when={props.isMobile()}>
              <div class="md:hidden w-full">
                <ResponsiveMetricCell
                  value={memPercent()}
                  type="memory"
                  sublabel={memUsageLabel()}
                  resourceId={metricsKey}
                  isRunning={isRunning() && (container.memoryUsageBytes ?? 0) > 0}
                  showMobile={true}
                  class="w-full"
                />
              </div>
            </Show>
            <div class="hidden md:block w-full">
              <StackedMemoryBar
                used={container.memoryUsageBytes || 0}
                total={memoryTotal()}
                balloon={0}
                swapUsed={0}
                swapTotal={0}
              />
            </div>
          </div>
        );
      case 'disk':
        return (
          <div class="px-2 py-0.5 flex items-center overflow-hidden">
            <Show when={hasDiskStats()} fallback={<span class="text-xs text-gray-400">—</span>}>
              <Show when={diskPercent() !== null} fallback={<span class="text-xs text-gray-700 dark:text-gray-300">{diskUsageLabel()}</span>}>
                <ResponsiveMetricCell
                  value={diskPercent() ?? 0}
                  type="disk"
                  resourceId={metricsKey}
                  sublabel={diskSublabel() ?? diskUsageLabel()}
                  isRunning={true}
                  showMobile={props.isMobile()}
                  class="w-full"
                />
              </Show>
            </Show>
          </div>
        );
      case 'tasks':
        return (
          <div class="px-2 py-0.5 text-xs text-gray-700 dark:text-gray-300 overflow-hidden whitespace-nowrap">
            <Show when={isRunning()} fallback={<span class="text-gray-400">—</span>}>
              <span class={restarts() > 5 ? 'text-red-600 dark:text-red-400 font-medium' : ''}>
                {restarts()}
              </span>
              <span class="text-[10px] text-gray-500 dark:text-gray-400 ml-1">restarts</span>
            </Show>
          </div>
        );
      case 'updated':
        return (
          <div class="px-2 py-0.5 text-xs text-gray-700 dark:text-gray-300 overflow-hidden whitespace-nowrap">
            <Show when={isRunning()} fallback={<span class="text-gray-400">—</span>}>
              <Show when={props.isMobile()} fallback={uptime()}>
                {formatUptime(container.uptimeSeconds || 0, true)}
              </Show>
            </Show>
          </div>
        );
      default:
        return null;
    }
  };

  return (
    <>
      <tr
        class={`transition-all duration-200 ${hasDrawerContent() ? 'cursor-pointer' : ''} ${expanded() ? 'bg-gray-50 dark:bg-gray-800/40' : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'} ${!isRunning() ? 'opacity-60' : ''}`}
        onClick={toggle}
        aria-expanded={expanded()}
      >
        <For each={DOCKER_COLUMNS}>
          {(column) => (
            <td
              class="py-0.5 align-middle whitespace-nowrap"
              style={{
                "min-width": column.id === 'cpu' || column.id === 'memory' ? '140px' : undefined,
                // Force resource column to respect truncation for very long names (issue #789)
                "max-width": column.id === 'resource' ? '0' : undefined,
                "width": column.id === 'resource' ? '40%' : undefined,
              }}
            >
              {renderCell(column)}
            </td>
          )}
        </For>
      </tr>

      <Show when={expanded() && hasDrawerContent()}>
        <tr>
          <td colspan={DOCKER_COLUMNS.length} class="p-0">
            <div class="w-0 min-w-full bg-gray-50 dark:bg-gray-900/50 px-4 py-3 overflow-hidden">
              <div class="flex flex-wrap justify-start gap-3">
                <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                  <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">
                    Summary
                  </div>
              <div class="mt-2 space-y-1 text-[11px] text-gray-600 dark:text-gray-300">
                <div class="flex items-center justify-between gap-2">
                  <span class="font-medium text-gray-700 dark:text-gray-200">Runtime</span>
                  <span
                    class={`inline-flex items-center gap-2 rounded-full px-2 py-0.5 text-[10px] font-semibold ${runtimeInfo.badgeClass}`}
                    title={runtimeInfo.raw || runtimeInfo.label}
                  >
                    {runtimeInfo.label}
                    <Show when={runtimeVersion()}>
                      {(version) => (
                        <span class="text-[10px] text-gray-500 dark:text-gray-400">{version()}</span>
                      )}
                    </Show>
                  </span>
                </div>
                <div class="flex items-start justify-between gap-2">
                  <span class="font-medium text-gray-700 dark:text-gray-200">Image</span>
                  <span class="flex-1 truncate text-right text-gray-600 dark:text-gray-300" title={container.image}>
                    {container.image || '—'}
                  </span>
                </div>
                <Show when={podName()}>
                  {(name) => (
                    <div class="flex items-center justify-between gap-2">
                      <span class="font-medium text-gray-700 dark:text-gray-200">Pod</span>
                      <span class="text-right text-gray-600 dark:text-gray-300">
                        {name()}
                        <Show when={isPodInfra()}>
                          <span class="ml-2 rounded bg-purple-100 px-1.5 py-0.5 text-[10px] font-semibold text-purple-700 dark:bg-purple-900/40 dark:text-purple-200">
                            infra
                          </span>
                        </Show>
                      </span>
                    </div>
                  )}
                </Show>
                <Show when={podmanMetadata()?.composeProject}>
                  {(project) => (
                    <div class="flex items-center justify-between gap-2">
                      <span class="font-medium text-gray-700 dark:text-gray-200">Compose Project</span>
                      <span class="text-right text-gray-600 dark:text-gray-300">{project()}</span>
                    </div>
                  )}
                </Show>
                <Show when={podmanMetadata()?.composeService}>
                  {(service) => (
                    <div class="flex items-center justify-between gap-2">
                      <span class="font-medium text-gray-700 dark:text-gray-200">Compose Service</span>
                      <span class="text-right text-gray-600 dark:text-gray-300">{service()}</span>
                    </div>
                  )}
                </Show>
                <Show when={podmanMetadata()?.autoUpdatePolicy}>
                  {(policy) => (
                    <div class="flex items-center justify-between gap-2">
                      <span class="font-medium text-gray-700 dark:text-gray-200">Auto Update</span>
                      <span class="text-right text-gray-600 dark:text-gray-300">
                        {policy()}
                        <Show when={podmanMetadata()?.autoUpdateRestart}>
                          {(restart) => (
                            <span class="ml-2 text-[10px] text-gray-500 dark:text-gray-400">restart: {restart()}</span>
                          )}
                        </Show>
                      </span>
                    </div>
                  )}
                </Show>
                <Show when={podmanMetadata()?.userNamespace}>
                  {(userns) => (
                    <div class="flex items-center justify-between gap-2">
                      <span class="font-medium text-gray-700 dark:text-gray-200">User Namespace</span>
                      <span class="text-right text-gray-600 dark:text-gray-300">{userns()}</span>
                    </div>
                  )}
                </Show>
                <div class="flex items-center justify-between gap-2">
                  <span class="font-medium text-gray-700 dark:text-gray-200">State</span>
                  <span class="text-right text-gray-600 dark:text-gray-300">{statusLabel()}</span>
                </div>
                <div class="flex items-center justify-between gap-2">
                  <span class="font-medium text-gray-700 dark:text-gray-200">Restarts</span>
                  <span class="text-right text-gray-600 dark:text-gray-300">{restarts()}</span>
                </div>
                <Show when={createdRelative()}>
                  {(created) => (
                    <div class="flex flex-col gap-0.5">
                      <span class="font-medium text-gray-700 dark:text-gray-200">Created</span>
                      <div class="text-right text-gray-600 dark:text-gray-300">
                        {created()}
                        <Show when={createdAbsolute()}>
                          {(abs) => (
                            <div class="text-[10px] text-gray-500 dark:text-gray-400">{abs()}</div>
                          )}
                        </Show>
                      </div>
                    </div>
                  )}
                </Show>
                <Show when={startedRelative()}>
                  {(started) => (
                    <div class="flex flex-col gap-0.5">
                      <span class="font-medium text-gray-700 dark:text-gray-200">Started</span>
                      <div class="text-right text-gray-600 dark:text-gray-300">
                        {started()}
                        <Show when={startedAbsolute()}>
                          {(abs) => (
                            <div class="text-[10px] text-gray-500 dark:text-gray-400">{abs()}</div>
                          )}
                        </Show>
                      </div>
                    </div>
                  )}
                </Show>
                <div class="flex items-center justify-between gap-2">
                  <span class="font-medium text-gray-700 dark:text-gray-200">Uptime</span>
                  <span class="text-right text-gray-600 dark:text-gray-300">{uptime()}</span>
                </div>
              </div>
              <Show when={runtimeInfo.id === 'podman'}>
                <div class="mt-3 rounded border border-dashed border-purple-200 px-2 py-1 text-[10px] text-purple-700 dark:border-purple-700/60 dark:text-purple-200">
                  Podman hosts report container metrics, but Swarm services and tasks are unavailable. Runtime annotations and compose metadata appear below when present.
                </div>
              </Show>
            </div>
            <Show when={container.ports && container.ports.length > 0}>
              <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">
                  Ports
                </div>
                <div class="mt-1 flex flex-wrap gap-1 text-[11px] text-gray-600 dark:text-gray-300">
                  {container.ports!.map((port) => {
                    const label = port.publicPort
                      ? `${port.publicPort}:${port.privatePort}/${port.protocol}`
                      : `${port.privatePort}/${port.protocol}`;
                    return (
                      <span class="rounded bg-blue-100 px-1.5 py-0.5 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                        {label}
                      </span>
                    );
                  })}
                </div>
              </div>
            </Show>

            <Show when={container.networks && container.networks.length > 0}>
              <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">
                  Networks
                </div>
                <div class="mt-1 space-y-1 text-[11px] text-gray-600 dark:text-gray-300">
                  {container.networks!.map((network) => (
                    <div class="rounded border border-dashed border-gray-200 p-2 last:mb-0 dark:border-gray-700/70">
                      <div class="font-medium text-gray-700 dark:text-gray-200">{network.name}</div>
                      <div class="mt-0.5 flex flex-wrap gap-1 text-[10px] text-gray-500 dark:text-gray-400">
                        <Show when={network.ipv4}>
                          <span class="rounded bg-blue-100 px-1.5 py-0.5 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                            {network.ipv4}
                          </span>
                        </Show>
                        <Show when={network.ipv6}>
                          <span class="rounded bg-purple-100 px-1.5 py-0.5 text-purple-700 dark:bg-purple-900/40 dark:text-purple-200">
                            {network.ipv6}
                          </span>
                        </Show>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </Show>

            <Show when={hasPodmanMetadata()}>
              <div class="min-w-[220px] flex-1 rounded border border-purple-200 bg-white/70 p-2 shadow-sm dark:border-purple-700/60 dark:bg-purple-950/20">
                <div class="text-[11px] font-medium uppercase tracking-wide text-purple-700 dark:text-purple-200">
                  Podman Metadata
                </div>
                <div class="mt-1 space-y-2 text-[11px] text-gray-600 dark:text-gray-300">
                  <For each={podmanMetadataSections()}>
                    {(section) => (
                      <div class="space-y-1 border-b border-purple-100 pb-1 last:border-b-0 last:pb-0 dark:border-purple-800/30">
                        <div class="text-[10px] font-semibold uppercase tracking-wide text-purple-600 dark:text-purple-300">
                          {section.title}
                        </div>
                        <div class="space-y-1">
                          <For each={section.items}>
                            {(item) => (
                              <div class="flex items-start justify-between gap-2">
                                <span class="font-medium text-gray-700 dark:text-gray-200">{item.label}</span>
                                <span
                                  class="max-w-[220px] break-all text-right text-gray-600 dark:text-gray-300"
                                  title={item.value || '—'}
                                >
                                  {item.value || '—'}
                                </span>
                              </div>
                            )}
                          </For>
                        </div>
                      </div>
                    )}
                  </For>
                </div>
              </div>
            </Show>

            <Show when={hasBlockIo()}>
              <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">
                  Block I/O
                </div>
                <div class="mt-1 space-y-1 text-[11px] text-gray-600 dark:text-gray-300">
                  <div class="flex items-center justify-between">
                    <span>Read</span>
                    <div class="text-right">
                      <div class="font-semibold text-gray-900 dark:text-gray-100">
                        {formatBytes(blockIoReadBytes())}
                      </div>
                      <Show when={blockIoReadRateLabel()}>
                        <div class="text-[10px] text-gray-500 dark:text-gray-400">
                          {blockIoReadRateLabel()}
                        </div>
                      </Show>
                    </div>
                  </div>
                  <div class="flex items-center justify-between">
                    <span>Write</span>
                    <div class="text-right">
                      <div class="font-semibold text-gray-900 dark:text-gray-100">
                        {formatBytes(blockIoWriteBytes())}
                      </div>
                      <Show when={blockIoWriteRateLabel()}>
                        <div class="text-[10px] text-gray-500 dark:text-gray-400">
                          {blockIoWriteRateLabel()}
                        </div>
                      </Show>
                    </div>
                  </div>
                </div>
              </div>
            </Show>

            <Show when={hasMounts()}>
              <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">
                  Mounts
                </div>
                <div class="mt-1 space-y-1 text-[11px] text-gray-600 dark:text-gray-300">
                  <For each={mounts()}>
                    {(mount) => {
                      const destination = mount.destination || mount.source || mount.name || 'mount';
                      const rw = mount.rw === false ? 'read-only' : 'read-write';
                      return (
                        <div class="rounded border border-dashed border-gray-200 p-2 last:mb-0 dark:border-gray-700/70">
                          <div class="flex items-center justify-between gap-2">
                            <span class="truncate font-medium text-gray-700 dark:text-gray-200" title={destination}>
                              {destination}
                            </span>
                            <Show when={mount.type}>
                              <span class="text-[10px] uppercase tracking-wide text-gray-500 dark:text-gray-400">
                                {mount.type}
                              </span>
                            </Show>
                          </div>
                          <Show when={mount.source}>
                            <div class="mt-1 truncate text-[11px] text-gray-600 dark:text-gray-300" title={mount.source}>
                              {mount.source}
                            </div>
                          </Show>
                          <div class="mt-1 flex flex-wrap gap-1 text-[10px] text-gray-500 dark:text-gray-400">
                            <span
                              class={`rounded px-1.5 py-0.5 ${mount.rw === false
                                ? 'bg-gray-200 text-gray-700 dark:bg-gray-700/60 dark:text-gray-200'
                                : 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                }`}
                            >
                              {rw}
                            </span>
                            <Show when={mount.mode}>
                              <span class="rounded bg-gray-200 px-1.5 py-0.5 text-gray-700 dark:bg-gray-700/60 dark:text-gray-200">
                                mode: {mount.mode}
                              </span>
                            </Show>
                            <Show when={mount.driver}>
                              <span class="rounded bg-blue-100 px-1.5 py-0.5 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                                {mount.driver}
                              </span>
                            </Show>
                            <Show when={mount.name}>
                              <span class="rounded bg-purple-100 px-1.5 py-0.5 text-purple-700 dark:bg-purple-900/40 dark:text-purple-200">
                                {mount.name}
                              </span>
                            </Show>
                            <Show when={mount.propagation}>
                              <span class="rounded bg-gray-100 px-1.5 py-0.5 text-gray-600 dark:bg-gray-800/40 dark:text-gray-300">
                                {mount.propagation}
                              </span>
                            </Show>
                          </div>
                        </div>
                      );
                    }}
                  </For>
                </div>
              </div>
            </Show>

            <Show when={container.labels && Object.keys(container.labels).length > 0}>
              <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">
                  Labels
                </div>
                <div class="mt-1 flex flex-wrap gap-1 text-[11px] text-gray-600 dark:text-gray-300">
                  {Object.entries(container.labels!).map(([key, value]) => {
                    const fullLabel = value ? `${key}: ${value}` : key;
                    return (
                      <span
                        class="max-w-full truncate rounded bg-gray-200 px-1.5 py-0.5 text-gray-700 dark:bg-gray-700/60 dark:text-gray-200"
                        title={fullLabel}
                      >
                        {key}
                        <Show when={value}>: {value}</Show>
                      </span>
                    );
                  })}
                </div>
              </div>
                </Show>
              </div>
            </div>
          </td>
        </tr>
      </Show>
    </>
  );
};

const DockerServiceRow: Component<{
  row: Extract<DockerRow, { kind: 'service' }>;
  isMobile: Accessor<boolean>;
  customUrl?: string;
  onCustomUrlUpdate?: (resourceId: string, url: string) => void;
  showHostContext?: boolean;
  resourceIndentClass?: string;
}> = (props) => {
  const { host, service, tasks } = props.row;
  const rowId = buildRowId(host, props.row);
  const resourceId = () => `${host.id}:service:${service.id || service.name}`;
  const isEditingUrl = createMemo(() => currentlyEditingDockerResourceId() === resourceId());
  const hostStatus = createMemo(() => getDockerHostStatusIndicator(host));
  const hostDisplayName = () => getHostDisplayName(host);
  const resourceIndent = () => props.resourceIndentClass ?? GROUPED_RESOURCE_INDENT;

  const [customUrl, setCustomUrl] = createSignal<string | undefined>(props.customUrl);
  const [shouldAnimateIcon, setShouldAnimateIcon] = createSignal(false);
  const expanded = createMemo(() => currentlyExpandedRowId() === rowId);
  const editingUrlValue = createMemo(() => {
    dockerEditingValuesVersion(); // Subscribe to changes
    return dockerEditingValues.get(resourceId()) || '';
  });
  let urlInputRef: HTMLInputElement | undefined;

  const hasTasks = () => tasks.length > 0;

  // Update custom URL when prop changes, but only if we're not currently editing
  createEffect(() => {
    if (currentlyEditingDockerResourceId() !== resourceId()) {
      const prevUrl = customUrl();
      const newUrl = props.customUrl;

      // Only animate when URL transitions from empty to having a value
      if (!prevUrl && newUrl) {
        setShouldAnimateIcon(true);
        setTimeout(() => setShouldAnimateIcon(false), 200);
      }

      setCustomUrl(newUrl);
    }
  });

  // Auto-focus the input when editing starts
  createEffect(() => {
    if (isEditingUrl() && urlInputRef) {
      urlInputRef.focus();
      urlInputRef.select();
    }
  });

  const toggle = (event: MouseEvent) => {
    if (!hasTasks()) return;
    const target = event.target as HTMLElement;
    if (target.closest('a, button, input, [data-prevent-toggle]')) return;
    // Toggle: if this row is currently expanded, close it; otherwise open it (closing any other)
    setCurrentlyExpandedRowId(prev => prev === rowId ? null : rowId);
  };

  const startEditingUrl = (event: MouseEvent) => {
    event.stopPropagation();

    // If another resource is being edited, save it first
    const currentEditing = currentlyEditingDockerResourceId();
    if (currentEditing !== null && currentEditing !== resourceId()) {
      const currentInput = document.querySelector(`input[data-resource-id="${currentEditing}"]`) as HTMLInputElement;
      if (currentInput) {
        currentInput.blur();
      }
    }

    dockerEditingValues.set(resourceId(), customUrl() || '');
    setDockerEditingValuesVersion(v => v + 1);
    setCurrentlyEditingDockerResourceId(resourceId());
  };

  // Add global click handler to close editor
  createEffect(() => {
    if (isEditingUrl()) {
      const handleGlobalClick = (e: MouseEvent) => {
        if (currentlyEditingDockerResourceId() !== resourceId()) return;

        const target = e.target as HTMLElement;
        const isClickingResourceName = target.closest('[data-resource-name-editable]');

        if (!target.closest('[data-url-editor]') && !isClickingResourceName) {
          e.preventDefault();
          e.stopPropagation();
          e.stopImmediatePropagation();
          cancelEditingUrl();
        }
      };

      const handleGlobalMouseDown = (e: MouseEvent) => {
        if (currentlyEditingDockerResourceId() !== resourceId()) return;

        const target = e.target as HTMLElement;
        const isClickingResourceName = target.closest('[data-resource-name-editable]');

        if (!target.closest('[data-url-editor]') && !isClickingResourceName) {
          e.preventDefault();
          e.stopPropagation();
          e.stopImmediatePropagation();
        }
      };

      document.addEventListener('mousedown', handleGlobalMouseDown, true);
      document.addEventListener('click', handleGlobalClick, true);
      return () => {
        document.removeEventListener('mousedown', handleGlobalMouseDown, true);
        document.removeEventListener('click', handleGlobalClick, true);
      };
    }
  });

  const saveUrl = async () => {
    if (currentlyEditingDockerResourceId() !== resourceId()) return;

    const newUrl = (dockerEditingValues.get(resourceId()) || '').trim();

    // Clear global editing state
    dockerEditingValues.delete(resourceId());
    setDockerEditingValuesVersion(v => v + 1);
    setCurrentlyEditingDockerResourceId(null);

    // If URL hasn't changed, don't save
    if (newUrl === (customUrl() || '')) return;

    // Animate if transitioning from no URL to having a URL
    const hadUrl = !!customUrl();
    if (!hadUrl && newUrl) {
      setShouldAnimateIcon(true);
      setTimeout(() => setShouldAnimateIcon(false), 200);
    }

    // Optimistically update local and parent state immediately
    setCustomUrl(newUrl || undefined);
    if (props.onCustomUrlUpdate) {
      props.onCustomUrlUpdate(resourceId(), newUrl);
    }

    try {
      await DockerMetadataAPI.updateMetadata(resourceId(), { customUrl: newUrl });

      if (newUrl) {
        showSuccess('Service URL saved');
      } else {
        showSuccess('Service URL cleared');
      }
    } catch (err: any) {
      logger.error('Failed to save service URL:', err);
      showError(err.message || 'Failed to save service URL');
      // Revert on error
      setCustomUrl(hadUrl ? customUrl() : undefined);
      if (props.onCustomUrlUpdate) {
        props.onCustomUrlUpdate(resourceId(), hadUrl ? customUrl() || '' : '');
      }
    }
  };

  const deleteUrl = async () => {
    if (currentlyEditingDockerResourceId() !== resourceId()) return;

    // Clear global editing state
    dockerEditingValues.delete(resourceId());
    setDockerEditingValuesVersion(v => v + 1);
    setCurrentlyEditingDockerResourceId(null);

    // If there was a URL set, delete it
    if (customUrl()) {
      try {
        await DockerMetadataAPI.updateMetadata(resourceId(), { customUrl: '' });
        setCustomUrl(undefined);

        // Notify parent to update metadata
        if (props.onCustomUrlUpdate) {
          props.onCustomUrlUpdate(resourceId(), '');
        }

        showSuccess('Service URL removed');
      } catch (err: any) {
        logger.error('Failed to remove service URL:', err);
        showError(err.message || 'Failed to remove service URL');
      }
    }
  };

  const cancelEditingUrl = () => {
    if (currentlyEditingDockerResourceId() !== resourceId()) return;

    // Just close without saving
    dockerEditingValues.delete(resourceId());
    setDockerEditingValuesVersion(v => v + 1);
    setCurrentlyEditingDockerResourceId(null);
  };

  const badge = serviceHealthBadge(service);
  const updatedAt = ensureMs(service.updatedAt ?? service.createdAt);
  const isHealthy = () => {
    const desired = service.desiredTasks ?? 0;
    const running = service.runningTasks ?? 0;
    return desired > 0 && running >= desired;
  };
  const serviceStatusIndicator = createMemo(() => getDockerServiceStatusIndicator(service));

  const serviceTitle = () => {
    const primary = service.name || service.id || 'Service';
    const identifier = service.id && service.name && service.id !== service.name ? service.id : '';
    return identifier ? `${primary} \u2014 ${identifier}` : primary;
  };

  // Render cell content based on column type
  const renderCell = (column: ColumnConfig) => {
    switch (column.id) {
      case 'resource':
        return (
          <div class={`${resourceIndent()} pr-2 py-0.5`}>
            <div class="flex items-center gap-1.5 min-w-0">
              <StatusDot
                variant={serviceStatusIndicator().variant}
                title={badge.label}
                ariaLabel={serviceStatusIndicator().label}
                size="xs"
              />
              <div class="flex-1 min-w-0">
                <Show
                  when={isEditingUrl()}
                  fallback={
                    <div class="flex items-center gap-1.5 flex-1 min-w-0">
                      <span
                        class="text-sm font-semibold text-gray-900 dark:text-gray-100 cursor-text select-none"
                        style="cursor: text;"
                        title={`${serviceTitle()}${customUrl() ? ' - Click to edit URL' : ' - Click to add URL'}`}
                        onClick={startEditingUrl}
                        data-resource-name-editable
                      >
                        {service.name || service.id || 'Service'}
                      </span>
                      <Show when={customUrl()}>
                        <a
                          href={customUrl()}
                          target="_blank"
                          rel="noopener noreferrer"
                          class={`flex-shrink-0 text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors ${shouldAnimateIcon() ? 'animate-fadeIn' : ''}`}
                          title="Open in new tab"
                          onClick={(event) => event.stopPropagation()}
                        >
                          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                          </svg>
                        </a>
                      </Show>
                      <Show when={service.stack && !isEditingUrl()}>
                        <span class="text-[10px] text-gray-500 dark:text-gray-400 truncate" title={`Stack: ${service.stack}`}>
                          Stack: {service.stack}
                        </span>
                      </Show>
                      <Show when={props.showHostContext}>
                        <span
                          class="inline-flex items-center gap-1 rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300"
                          title={`Host: ${hostDisplayName()}`}
                        >
                          <StatusDot variant={hostStatus().variant} title={hostStatus().label} ariaLabel={hostStatus().label} size="xs" />
                          <span class="max-w-[160px] truncate">{hostDisplayName()}</span>
                        </span>
                      </Show>
                    </div>
                  }
                >
                  <div class="flex-1 flex items-center gap-1 min-w-0" data-url-editor>
                    <input
                      ref={urlInputRef}
                      type="text"
                      value={editingUrlValue()}
                      data-resource-id={resourceId()}
                      onInput={(e) => { dockerEditingValues.set(resourceId(), e.currentTarget.value); setDockerEditingValuesVersion(v => v + 1); }}
                      onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); saveUrl(); } else if (e.key === 'Escape') { e.preventDefault(); cancelEditingUrl(); } }}
                      onClick={(e) => e.stopPropagation()}
                      placeholder="https://example.com:8080"
                      class="flex-1 min-w-0 px-2 py-0.5 text-sm border border-blue-500 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                    />
                    <button type="button" data-url-editor-button onClick={(e) => { e.stopPropagation(); saveUrl(); }} class="flex-shrink-0 w-6 h-6 flex items-center justify-center text-xs bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors" title="Save (or press Enter)">✓</button>
                    <button type="button" data-url-editor-button onClick={(e) => { e.stopPropagation(); deleteUrl(); }} class="flex-shrink-0 w-6 h-6 flex items-center justify-center text-xs bg-red-600 text-white rounded hover:bg-red-700 transition-colors" title="Delete URL">✕</button>
                  </div>
                </Show>
              </div>
            </div>
          </div>
        );
      case 'type':
        return (
          <div class="px-2 py-0.5 flex items-center">
            <span class={`inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${typeBadgeClass('service')}`}>
              Service
            </span>
          </div>
        );
      case 'image':
        return (
          <div class="px-2 py-0.5 text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
            {service.image || '—'}
          </div>
        );
      case 'status':
        return (
          <div class="px-2 py-0.5 text-xs">
            <span class={`rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${badge.class}`}>{badge.label}</span>
          </div>
        );
      case 'cpu':
        return <div class="px-2 py-0.5 text-xs text-gray-400 dark:text-gray-500">—</div>;
      case 'memory':
        return <div class="px-2 py-0.5 text-xs text-gray-400 dark:text-gray-500">—</div>;
      case 'disk':
        return <div class="px-2 py-0.5 text-xs text-gray-400 dark:text-gray-500">—</div>;
      case 'tasks':
        return (
          <div class="px-2 py-0.5 text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
            <span class="font-semibold text-gray-900 dark:text-gray-100">
              {(service.runningTasks ?? 0)}/{service.desiredTasks ?? 0}
            </span>
            <span class="ml-1 text-gray-500 dark:text-gray-400">tasks</span>
          </div>
        );
      case 'updated':
        return (
          <div class="px-2 py-0.5 text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
            <Show when={updatedAt} fallback="—">
              {(timestamp) => (
                <span title={new Date(timestamp()).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })}>
                  {formatRelativeTime(timestamp())}
                </span>
              )}
            </Show>
          </div>
        );
      default:
        return null;
    }
  };

  return (
    <>
      <tr
        class={`transition-all duration-200 ${hasTasks() ? 'cursor-pointer' : ''} ${expanded() ? 'bg-gray-50 dark:bg-gray-800/40' : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'} ${!isHealthy() ? 'opacity-60' : ''}`}
        onClick={toggle}
        aria-expanded={expanded()}
      >
        <For each={DOCKER_COLUMNS}>
          {(column) => (
            <td
              class="py-0.5 align-middle whitespace-nowrap"
              style={{
                "min-width": column.id === 'cpu' || column.id === 'memory' ? '140px' : undefined,
                // Force resource column to respect truncation for very long names (issue #789)
                "max-width": column.id === 'resource' ? '0' : undefined,
                "width": column.id === 'resource' ? '40%' : undefined,
              }}
            >
              {renderCell(column)}
            </td>
          )}
        </For>
      </tr>

      <Show when={expanded() && hasTasks()}>
        <tr>
          <td colspan={DOCKER_COLUMNS.length} class="p-0">
            <div class="w-0 min-w-full bg-gray-50 dark:bg-gray-900/60 px-4 py-3 overflow-hidden">
              <div class="flex flex-wrap justify-start gap-3">
                <div class="min-w-[320px] flex-1 rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                  <div class="flex items-center justify-between text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">
                    <span>Tasks</span>
                    <span class="text-[10px] font-normal text-gray-500 dark:text-gray-400">
                      {tasks.length} {tasks.length === 1 ? 'entry' : 'entries'}
                    </span>
                  </div>
                  <div class="mt-2 overflow-x-auto">
                    <table class="min-w-full divide-y divide-gray-100 dark:divide-gray-800/60 text-xs">
                  <thead class="bg-gray-100 dark:bg-gray-900/40 text-[10px] uppercase tracking-wide text-gray-600 dark:text-gray-200">
                    <tr>
                      <th class="py-1 pr-2 text-left font-medium">Task</th>
                      <th class="py-1 px-2 text-left font-medium w-[80px]">Type</th>
                      <th class="py-1 px-2 text-left font-medium">Node</th>
                      <th class="py-1 px-2 text-left font-medium">State</th>
                      <th class="py-1 px-2 text-left font-medium w-[120px]">CPU</th>
                      <th class="py-1 px-2 text-left font-medium w-[140px]">Memory</th>
                      <th class="py-1 px-2 text-left font-medium">Updated</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-gray-800/50">
                    <For each={tasks}>
                      {(task) => {
                        const container = findContainerForTask(host.containers || [], task);
                        const cpu = container?.cpuPercent ?? 0;
                        const mem = container?.memoryPercent ?? 0;
                        const updated = ensureMs(task.updatedAt ?? task.createdAt ?? task.startedAt);
                        const taskLabel = () => {
                          if (task.containerName) return task.containerName;
                          if (task.containerId) return task.containerId.slice(0, 12);
                          if (task.slot !== undefined) return `slot-${task.slot}`;
                          return task.id ?? 'Task';
                        };
                        const taskTitle = () => {
                          const label = taskLabel();
                          if (task.containerId && task.containerId !== label) {
                            return `${label} \u2014 ${task.containerId}`;
                          }
                          if (task.id && task.id !== label) {
                            return `${label} \u2014 ${task.id}`;
                          }
                          return label;
                        };
                        const state = toLower(task.currentState ?? task.desiredState ?? 'unknown');
                        const taskMetricsKey = container?.id ? buildMetricKey('dockerContainer', container.id) : undefined;
                        const stateClass = () => {
                          if (state === 'running') {
                            return 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300';
                          }
                          if (state === 'failed' || state === 'error') {
                            return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300';
                          }
                          return 'bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-300';
                        };
                        return (
                          <tr class="hover:bg-gray-100 dark:hover:bg-gray-800/40">
                            <td class="py-1 pr-2">
                              <div class="flex items-center gap-1 text-sm text-gray-900 dark:text-gray-100">
                                <span class="truncate font-medium" title={taskTitle()}>
                                  {taskLabel()}
                                </span>
                              </div>
                            </td>
                            <td class="py-1 px-2">
                              <span
                                class={`inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${typeBadgeClass(
                                  'task',
                                )}`}
                              >
                                Task
                              </span>
                            </td>
                            <td class="py-1 px-2 text-gray-600 dark:text-gray-400">
                              {task.nodeName || task.nodeId || '—'}
                            </td>
                            <td class="py-1 px-2">
                              <span class={`rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${stateClass()}`}>
                                {task.currentState || task.desiredState || 'Unknown'}
                              </span>
                            </td>
                            <td class="py-1 px-2 w-[120px]">
                              <Show when={cpu > 0} fallback={<span class="text-gray-400">—</span>}>
                                <MetricBar
                                  value={Math.min(100, cpu)}
                                  label={formatPercent(cpu)}
                                  type="cpu"
                                  resourceId={taskMetricsKey}
                                />
                              </Show>
                            </td>
                            <td class="py-1 px-2 w-[140px]">
                              <Show when={mem > 0} fallback={<span class="text-gray-400">—</span>}>
                                <MetricBar
                                  value={Math.min(100, mem)}
                                  label={formatPercent(mem)}
                                  type="memory"
                                  resourceId={taskMetricsKey}
                                />
                              </Show>
                            </td>
                            <td class="py-1 px-2 text-gray-600 dark:text-gray-400 whitespace-nowrap">
                              <Show when={updated} fallback="—">
                                {(timestamp) => (
                                  <span title={new Date(timestamp()).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })}>
                                    {formatRelativeTime(timestamp())}
                                  </span>
                                )}
                              </Show>
                            </td>
                          </tr>
                        );
                      }}
                    </For>
                  </tbody>
                    </table>
                  </div>
                </div>
              </div>
            </div>
          </td>
        </tr>
      </Show>
    </>
  );
};

const areTasksEqual = (a: DockerTask[], b: DockerTask[]) => {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    if (a[i] !== b[i]) return false;
  }
  return true;
};

const DockerUnifiedTable: Component<DockerUnifiedTableProps> = (props) => {
  // Use the breakpoint hook for responsive behavior
  const { isMobile } = useBreakpoint();

  // Caches for stable object references to prevent re-animations
  const rowCache = new Map<string, DockerRow>();
  const tasksCache = new Map<string, DockerTask[]>();

  const tokens = createMemo(() => parseSearchTerm(props.searchTerm));
  const [sortKey, setSortKey] = usePersistentSignal<SortKey>('dockerUnifiedSortKey', 'host', {
    deserialize: (value) => (SORT_KEYS.includes(value as SortKey) ? (value as SortKey) : 'host'),
  });
  const [sortDirection, setSortDirection] = usePersistentSignal<SortDirection>(
    'dockerUnifiedSortDirection',
    'asc',
    {
      deserialize: (value) => (value === 'asc' || value === 'desc' ? value : 'asc'),
    },
  );

  const isGroupedView = createMemo(() => sortKey() === 'host');

  const handleSort = (key: SortKey) => {
    if (sortKey() === key) {
      setSortDirection(sortDirection() === 'asc' ? 'desc' : 'asc');
      return;
    }
    setSortKey(key);
    setSortDirection(SORT_DEFAULT_DIRECTION[key]);
  };

  const renderSortIndicator = (key: SortKey) => {
    if (sortKey() !== key) return null;
    return sortDirection() === 'asc' ? '▲' : '▼';
  };

  const resetHostGrouping = () => {
    setSortKey('host');
    setSortDirection(SORT_DEFAULT_DIRECTION.host);
  };

  const ariaSort = (key: SortKey) => {
    if (sortKey() !== key) {
      if (sortKey() === 'host' && key === 'resource') return 'other';
      return 'none';
    }
    return sortDirection() === 'asc' ? 'ascending' : 'descending';
  };

  const sortedHosts = createMemo(() => {
    const hosts = props.hosts || [];
    return [...hosts].sort((a, b) => {
      const aName = getHostDisplayName(a);
      const bName = getHostDisplayName(b);
      return aName.localeCompare(bName);
    });
  });

  const groupedRows = createMemo(() => {
    const groups: Array<{ host: DockerHost; rows: DockerRow[] }> = [];
    const filter = props.statsFilter ?? null;
    const searchTokens = tokens();
    const selectedHostId = props.selectedHostId ? props.selectedHostId() : null;
    const usedCacheKeys = new Set<string>();
    const usedTaskCacheKeys = new Set<string>();

    sortedHosts().forEach((host) => {
      if (!hostMatchesFilter(filter, host)) {
        return;
      }

      if (selectedHostId && host.id !== selectedHostId) {
        return;
      }

      const containerRows: Array<Extract<DockerRow, { kind: 'container' }>> = [];
      const serviceRows: Array<Extract<DockerRow, { kind: 'service' }>> = [];

      const containers = host.containers || [];
      const services = host.services || [];
      const tasks = host.tasks || [];

      const serviceNames = new Set<string>();
      const serviceIds = new Set<string>();
      services.forEach((service) => {
        if (service.name) serviceNames.add(service.name.toLowerCase());
        if (service.id) serviceIds.add(service.id.toLowerCase());
      });

      const serviceOwnedContainers = new Set<string>();

      containers.forEach((container) => {
        if (!containerMatchesStateFilter(filter, container)) return;
        const matchesSearch = searchTokens.every((token) => containerMatchesToken(token, host, container));
        if (!matchesSearch) return;

        const rowId = container.id || `${host.id}-container-${container.name}`;
        const cacheKey = `c:${host.id}:${rowId}`;
        usedCacheKeys.add(cacheKey);

        let row = rowCache.get(cacheKey);
        if (!row || row.kind !== 'container' || row.host !== host || row.container !== container) {
          row = {
            kind: 'container',
            id: rowId,
            host,
            container,
          };
          rowCache.set(cacheKey, row);
        }

        containerRows.push(row as Extract<DockerRow, { kind: 'container' }>);
      });

      services.forEach((service) => {
        if (!serviceMatchesHealthFilter(filter, service)) return;
        const matchesSearch = searchTokens.every((token) => serviceMatchesToken(token, host, service));
        if (!matchesSearch) return;

        let associatedTasks = tasks.filter((task) => {
          if (service.id && task.serviceId) {
            return task.serviceId === service.id;
          }
          if (service.name && task.serviceName) {
            return task.serviceName === service.name;
          }
          return false;
        });

        // Use stable array reference for tasks if content matches
        const taskCacheKey = `s:${host.id}:${service.id || service.name}`;
        usedTaskCacheKeys.add(taskCacheKey);
        const cachedTasks = tasksCache.get(taskCacheKey);
        if (cachedTasks && areTasksEqual(cachedTasks, associatedTasks)) {
          associatedTasks = cachedTasks;
        } else {
          tasksCache.set(taskCacheKey, associatedTasks);
        }

        associatedTasks.forEach((task) => {
          if (task.containerId) serviceOwnedContainers.add(task.containerId.toLowerCase());
          if (task.containerName) serviceOwnedContainers.add(task.containerName.toLowerCase());
        });

        const rowId = service.id || `${host.id}-service-${service.name}`;
        const cacheKey = `s:${host.id}:${rowId}`;
        usedCacheKeys.add(cacheKey);

        let row = rowCache.get(cacheKey);
        // Check if row needs update (host/service changed, or tasks array changed)
        if (!row || row.kind !== 'service' || row.host !== host || row.service !== service || row.tasks !== associatedTasks) {
          row = {
            kind: 'service',
            id: rowId,
            host,
            service,
            tasks: associatedTasks,
          };
          rowCache.set(cacheKey, row);
        }

        serviceRows.push(row as Extract<DockerRow, { kind: 'service' }>);
      });

      if (serviceRows.length > 0) {
        serviceRows.sort((a, b) => {
          const nameA = a.service.name || a.service.id || '';
          const nameB = b.service.name || b.service.id || '';
          return nameA.localeCompare(nameB);
        });
      }

      if (containerRows.length > 0) {
        containerRows.sort((a, b) => {
          const nameA = a.container.name || a.container.id || '';
          const nameB = b.container.name || b.container.id || '';
          return nameA.localeCompare(nameB);
        });
        const filtered = containerRows.filter((row) => {
          const idKey = (row.container.id || '').toLowerCase();
          const nameKey = (row.container.name || '').toLowerCase();
          const shortNameKey = nameKey.split('.')[0];

          const labelServiceName =
            row.container.labels?.['com.docker.swarm.service.name']?.toLowerCase() ?? '';
          const labelServiceID =
            row.container.labels?.['com.docker.swarm.service.id']?.toLowerCase() ?? '';

          const belongsToService =
            (idKey && serviceOwnedContainers.has(idKey)) ||
            (nameKey && serviceOwnedContainers.has(nameKey)) ||
            (shortNameKey && serviceOwnedContainers.has(shortNameKey)) ||
            (labelServiceName && serviceNames.has(labelServiceName)) ||
            (labelServiceID && serviceIds.has(labelServiceID)) ||
            (serviceNames.size > 0 && nameKey && [...serviceNames].some((svc) => nameKey.startsWith(`${svc}.`)));

          return !belongsToService;
        });

        containerRows.length = 0;
        containerRows.push(...filtered);
      }

      const hostRows = [...serviceRows, ...containerRows];

      if (hostRows.length > 0) {
        groups.push({ host, rows: hostRows });
      }
    });

    // Prune caches
    for (const key of rowCache.keys()) {
      if (!usedCacheKeys.has(key)) {
        rowCache.delete(key);
      }
    }
    for (const key of tasksCache.keys()) {
      if (!usedTaskCacheKeys.has(key)) {
        tasksCache.delete(key);
      }
    }

    return groups;
  });

  const flatRows = createMemo(() => groupedRows().flatMap((group) => group.rows));

  const orderedGroups = createMemo(() => {
    if (sortKey() !== 'host') {
      return groupedRows();
    }
    if (sortDirection() === 'asc') return groupedRows();
    const reversed = [...groupedRows()];
    reversed.reverse();
    return reversed;
  });

  const sortedRows = createMemo(() => {
    if (sortKey() === 'host') {
      return flatRows();
    }

    const rows = [...flatRows()];
    const key = sortKey();
    const dir = sortDirection();

    rows.sort((a, b) => {
      const primary = compareRowsByKey(a, b, key);
      if (primary !== 0) {
        return dir === 'asc' ? primary : -primary;
      }

      const byResource = compareRowsByKey(a, b, 'resource');
      if (byResource !== 0) {
        return byResource;
      }

      return compareRowsByKey(a, b, 'host');
    });

    return rows;
  });

  const totalRows = createMemo(() => flatRows().length);

  const totalContainers = createMemo(() =>
    (props.hosts || []).reduce((acc, host) => acc + (host.containers?.length ?? 0), 0),
  );
  const totalServices = createMemo(() =>
    (props.hosts || []).reduce((acc, host) => acc + (host.services?.length ?? 0), 0),
  );

  const runningContainers = createMemo(() =>
    groupedRows().reduce((acc, group) => {
      return (
        acc +
        group.rows
          .filter((row): row is Extract<typeof row, { kind: 'container' }> => row.kind === 'container')
          .filter((row) => toLower(row.container.state) === 'running').length
      );
    }, 0),
  );

  const stoppedContainers = createMemo(() =>
    groupedRows().reduce((acc, group) => {
      return (
        acc +
        group.rows
          .filter((row): row is Extract<typeof row, { kind: 'container' }> => row.kind === 'container')
          .filter((row) => STOPPED_CONTAINER_STATES.has(toLower(row.container.state))).length
      );
    }, 0),
  );

  const degradedContainers = createMemo(() =>
    groupedRows().reduce((acc, group) => {
      return (
        acc +
        group.rows
          .filter((row): row is Extract<typeof row, { kind: 'container' }> => row.kind === 'container')
          .filter((row) => {
            const state = toLower(row.container.state);
            const health = toLower(row.container.health);

            // Explicitly degraded/error states
            if (ERROR_CONTAINER_STATES.has(state)) return true;

            // Running but unhealthy
            if (state === 'running' && health === 'unhealthy') return true;

            // Any other state that is NOT running and NOT stopped
            if (state !== 'running' && !STOPPED_CONTAINER_STATES.has(state)) return true;

            return false;
          }).length
      );
    }, 0),
  );

  const renderRow = (row: DockerRow, grouped: boolean) => {
    const resourceId =
      row.kind === 'container'
        ? `${row.host.id}:container:${row.container.id || row.container.name}`
        : `${row.host.id}:service:${row.service.id || row.service.name}`;
    const metadata = props.dockerMetadata?.[resourceId];

    return row.kind === 'container' ? (
      <DockerContainerRow
        row={row}
        isMobile={isMobile}
        customUrl={metadata?.customUrl}
        onCustomUrlUpdate={props.onCustomUrlUpdate}
        showHostContext={!grouped}
        resourceIndentClass={grouped ? GROUPED_RESOURCE_INDENT : UNGROUPED_RESOURCE_INDENT}
      />
    ) : (
      <DockerServiceRow
        row={row}
        isMobile={isMobile}
        customUrl={metadata?.customUrl}
        onCustomUrlUpdate={props.onCustomUrlUpdate}
        showHostContext={!grouped}
        resourceIndentClass={grouped ? GROUPED_RESOURCE_INDENT : UNGROUPED_RESOURCE_INDENT}
      />
    );
  };

  return (
    <div class="space-y-4">
      <Show
        when={totalRows() > 0}
        fallback={
          <Card padding="lg">
            <EmptyState
              title="No container workloads found"
              description={
                totalContainers() === 0 && totalServices() === 0
                  ? 'Add a container agent in Settings to start gathering container and service metrics.'
                  : props.searchTerm || props.statsFilter
                    ? 'No containers or services match your current filters.'
                    : 'Container runtime data is currently unavailable.'
              }
            />
          </Card>
        }
      >
        <Card padding="none" tone="glass" class="overflow-hidden">
          <div class="overflow-x-auto">
            <table class="w-full border-collapse whitespace-nowrap">
              <thead>
                <tr class="border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 text-[11px] sm:text-xs font-medium uppercase tracking-wider sticky top-0 z-20">
                  <For each={DOCKER_COLUMNS}>
                    {(column) => {
                      const col = column as DockerColumnDef;
                      const colSortKey = col.sortKey as SortKey | undefined;
                      const isResource = col.id === 'resource';
                      return (
                        <th
                          class={`${isResource ? 'pl-4 sm:pl-5 lg:pl-6 pr-2' : 'px-2'} py-1 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 text-left font-medium whitespace-nowrap`}
                          style={{ "min-width": col.id === 'cpu' || col.id === 'memory' ? '140px' : undefined }}
                          onClick={() => colSortKey && handleSort(colSortKey)}
                          onKeyDown={(e) => e.key === 'Enter' && colSortKey && handleSort(colSortKey)}
                          tabIndex={0}
                          role="button"
                          aria-label={`Sort by ${col.label} ${colSortKey && sortKey() === colSortKey ? (sortDirection() === 'asc' ? 'ascending' : 'descending') : ''}`}
                          aria-sort={colSortKey ? ariaSort(colSortKey) : 'none'}
                        >
                          <Show when={isResource}>
                            <div class="flex flex-wrap items-center gap-2">
                              <span>{col.label}</span>
                              {colSortKey && renderSortIndicator(colSortKey)}
                              <Show when={sortKey() === 'host'}>
                                <span class="text-[10px] font-medium text-gray-500 dark:text-gray-400">Grouped by host</span>
                              </Show>
                              <Show when={sortKey() !== 'host'}>
                                <button
                                  type="button"
                                  class="ml-auto rounded bg-gray-200 px-2 py-0.5 text-[10px] font-medium text-gray-700 transition hover:bg-gray-300 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    resetHostGrouping();
                                  }}
                                >
                                  Group by host
                                </button>
                              </Show>
                            </div>
                          </Show>
                          <Show when={!isResource}>
                            <div class="flex items-center gap-1">
                              <span>{col.label}</span>
                              {colSortKey && renderSortIndicator(colSortKey)}
                            </div>
                          </Show>
                        </th>
                      );
                    }}
                  </For>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                <Show
                  when={isGroupedView()}
                  fallback={
                    <For each={sortedRows()}>
                      {(row) => renderRow(row, false)}
                    </For>
                  }
                >
                  <For each={orderedGroups()}>
                    {(group) => (
                      <>
                        <DockerHostGroupHeader host={group.host} columnCount={DOCKER_COLUMNS.length} />
                        <For each={group.rows}>{(row) => renderRow(row, true)}</For>
                      </>
                    )}
                  </For>
                </Show>
              </tbody>
            </table>
          </div>
        </Card>

        <div class="flex items-center gap-2 rounded border border-gray-200 bg-gray-50 p-2 text-xs text-gray-600 dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-300">
          <span class="flex items-center gap-1">
            <span class="h-2 w-2 rounded-full bg-green-500" aria-hidden="true" />
            {runningContainers()} running
          </span>
          <Show when={degradedContainers() > 0}>
            <span class="text-gray-400">|</span>
            <span class="flex items-center gap-1">
              <span class="h-2 w-2 rounded-full bg-orange-500" aria-hidden="true" />
              {degradedContainers()} degraded
            </span>
          </Show>
          <span class="text-gray-400">|</span>
          <span class="flex items-center gap-1">
            <span class="h-2 w-2 rounded-full bg-gray-400" aria-hidden="true" />
            {stoppedContainers()} stopped
          </span>
        </div>
      </Show>
    </div>
  );
};

export { DockerUnifiedTable };
