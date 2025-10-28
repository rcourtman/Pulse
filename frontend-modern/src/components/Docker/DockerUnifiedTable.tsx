import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import type { DockerHost, DockerContainer, DockerService, DockerTask } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { ScrollableTable } from '@/components/shared/ScrollableTable';
import { EmptyState } from '@/components/shared/EmptyState';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { formatBytes, formatPercent, formatUptime, formatRelativeTime } from '@/utils/format';

const OFFLINE_HOST_STATUSES = new Set(['offline', 'error', 'unreachable', 'down', 'disconnected']);
const DEGRADED_HOST_STATUSES = new Set([
  'degraded',
  'warning',
  'maintenance',
  'partial',
  'initializing',
  'unknown',
]);

const STOPPED_CONTAINER_STATES = new Set(['exited', 'stopped', 'created', 'paused']);
const ERROR_CONTAINER_STATES = new Set([
  'restarting',
  'dead',
  'removing',
  'failed',
  'error',
  'oomkilled',
  'unhealthy',
]);

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
}

const rowExpandState = new Map<string, boolean>();

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
    return OFFLINE_HOST_STATUSES.has(status);
  }
  if (filter.value === 'degraded') {
    return DEGRADED_HOST_STATUSES.has(status) || status === 'degraded';
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
  const hostName = toLower(host.displayName ?? host.hostname ?? host.id);

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
  const hostName = toLower(host.displayName ?? host.hostname ?? host.id);
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

const DockerHostGroupHeader: Component<{ host: DockerHost; colspan: number }> = (props) => {
  const displayName = props.host.displayName || props.host.hostname || props.host.id;

  return (
    <tr class="bg-gray-50 dark:bg-gray-900/40">
      <td colSpan={props.colspan} class="py-1 pr-2 pl-4">
        <div class="flex flex-nowrap items-center gap-2 whitespace-nowrap text-sm font-semibold text-slate-700 dark:text-slate-100">
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

const DockerContainerRow: Component<{ row: Extract<DockerRow, { kind: 'container' }>; columns: number }> = (props) => {
  const { host, container } = props.row;
  const rowId = buildRowId(host, props.row);
  const [expanded, setExpanded] = createSignal(rowExpandState.get(rowId) ?? false);
  const hasDrawerContent = createMemo(() => {
    return (
      (container.ports && container.ports.length > 0) ||
      (container.labels && Object.keys(container.labels).length > 0) ||
      (container.networks && container.networks.length > 0)
    );
  });

  const toggle = (event: MouseEvent) => {
    if (!hasDrawerContent()) return;
    const target = event.target as HTMLElement;
    if (target.closest('a, button, [data-prevent-toggle]')) return;
    setExpanded((prev) => {
      const next = !prev;
      rowExpandState.set(rowId, next);
      return next;
    });
  };

  const cpuPercent = () => Math.max(0, Math.min(100, container.cpuPercent ?? 0));
  const memPercent = () => Math.max(0, Math.min(100, container.memoryPercent ?? 0));
  const memUsageLabel = () => {
    if (!container.memoryUsageBytes) return undefined;
    const used = formatBytes(container.memoryUsageBytes);
    const limit = container.memoryLimitBytes
      ? formatBytes(container.memoryLimitBytes)
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

  return (
    <>
      <tr
        class={`border-b border-gray-200 dark:border-gray-700 transition-all duration-200 ${
          hasDrawerContent() ? 'cursor-pointer' : ''
        } ${expanded() ? 'bg-gray-50 dark:bg-gray-800/40' : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'} ${!isRunning() ? 'opacity-60' : ''}`}
        onClick={toggle}
        aria-expanded={expanded()}
      >
        <td class="pl-4 pr-2 py-1">
          <div class="flex items-center gap-1.5 min-w-0">
            <div class="flex items-center gap-1.5 min-w-0 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">
              <span class="truncate font-semibold" title={containerTitle()}>
                {container.name || container.id}
              </span>
            </div>
          </div>
        </td>
        <td class="px-2 py-1">
          <span
            class={`inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${typeBadgeClass(
              'container',
            )}`}
          >
            Container
          </span>
        </td>
        <td class="px-2 py-1 text-xs text-gray-700 dark:text-gray-300">
          <span class="truncate" title={container.image}>
            {container.image || '—'}
          </span>
        </td>
        <td class="px-2 py-1 text-xs">
          <span class={`rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${statusBadgeClass()}`}>
            {statusLabel()}
          </span>
        </td>
        <td class="px-2 py-1">
          <Show
            when={isRunning() && container.cpuPercent && container.cpuPercent > 0}
            fallback={<span class="text-xs text-gray-400">—</span>}
          >
            <MetricBar value={cpuPercent()} label={formatPercent(cpuPercent())} type="cpu" />
          </Show>
        </td>
        <td class="px-2 py-1">
          <Show
            when={isRunning() && container.memoryUsageBytes && container.memoryUsageBytes > 0}
            fallback={<span class="text-xs text-gray-400">—</span>}
          >
            <MetricBar
              value={memPercent()}
              label={formatPercent(memPercent())}
              type="memory"
              sublabel={memUsageLabel()}
            />
          </Show>
        </td>
        <td class="px-2 py-1 text-xs text-gray-700 dark:text-gray-300">
          <Show when={isRunning()} fallback={<span class="text-gray-400">—</span>}>
            {restarts()}
            <span class="text-[10px] text-gray-500 dark:text-gray-400 ml-1">restarts</span>
          </Show>
        </td>
        <td class="px-2 py-1 text-xs text-gray-700 dark:text-gray-300">
          <Show when={isRunning()} fallback={<span class="text-gray-400">—</span>}>
            {uptime()}
          </Show>
        </td>
      </tr>

      <Show when={expanded() && hasDrawerContent()}>
        <tr class="bg-gray-50 dark:bg-gray-900/50">
          <td colSpan={props.columns} class="px-4 py-3">
            <div class="grid gap-3 md:grid-cols-2">
              <Show when={container.ports && container.ports.length > 0}>
                <div>
                  <div class="text-[11px] font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-400">
                    Ports
                  </div>
                  <div class="mt-1 flex flex-wrap gap-1">
                    {container.ports!.map((port) => {
                      const label = port.publicPort
                        ? `${port.publicPort}:${port.privatePort}/${port.protocol}`
                        : `${port.privatePort}/${port.protocol}`;
                      return (
                        <span class="rounded bg-blue-100 px-1.5 py-0.5 text-[11px] text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                          {label}
                        </span>
                      );
                    })}
                  </div>
                </div>
              </Show>

              <Show when={container.networks && container.networks.length > 0}>
                <div>
                  <div class="text-[11px] font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-400">
                    Networks
                  </div>
                  <div class="mt-1 space-y-1 text-xs text-gray-700 dark:text-gray-300">
                    {container.networks!.map((network) => (
                      <div>
                        <span class="font-medium">{network.name}</span>
                        <Show when={network.ipv4}>
                          <span class="ml-2 text-gray-500 dark:text-gray-400">IPv4: {network.ipv4}</span>
                        </Show>
                        <Show when={network.ipv6}>
                          <span class="ml-2 text-gray-500 dark:text-gray-400">IPv6: {network.ipv6}</span>
                        </Show>
                      </div>
                    ))}
                  </div>
                </div>
              </Show>

              <Show when={container.labels && Object.keys(container.labels).length > 0}>
                <div class="md:col-span-2">
                  <div class="text-[11px] font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-400">
                    Labels
                  </div>
                  <div class="mt-1 flex flex-wrap gap-1">
                    {Object.entries(container.labels!).map(([key, value]) => (
                      <span class="rounded bg-gray-200 px-1.5 py-0.5 text-[11px] text-gray-700 dark:bg-gray-700/60 dark:text-gray-200">
                        {key}
                        <Show when={value}>: {value}</Show>
                      </span>
                    ))}
                  </div>
                </div>
              </Show>
            </div>
          </td>
        </tr>
      </Show>
    </>
  );
};

const DockerServiceRow: Component<{ row: Extract<DockerRow, { kind: 'service' }>; columns: number }> = (props) => {
  const { host, service, tasks } = props.row;
  const rowId = buildRowId(host, props.row);
  const [expanded, setExpanded] = createSignal(rowExpandState.get(rowId) ?? false);
  const hasTasks = () => tasks.length > 0;

  const toggle = (event: MouseEvent) => {
    if (!hasTasks()) return;
    const target = event.target as HTMLElement;
    if (target.closest('a, button, [data-prevent-toggle]')) return;
    setExpanded((prev) => {
      const next = !prev;
      rowExpandState.set(rowId, next);
      return next;
    });
  };

  const badge = serviceHealthBadge(service);
  const updatedAt = ensureMs(service.updatedAt ?? service.createdAt);
  const isHealthy = () => {
    const desired = service.desiredTasks ?? 0;
    const running = service.runningTasks ?? 0;
    return desired > 0 && running >= desired;
  };

  const serviceTitle = () => {
    const primary = service.name || service.id || 'Service';
    const identifier = service.id && service.name && service.id !== service.name ? service.id : '';
    return identifier ? `${primary} \u2014 ${identifier}` : primary;
  };

  return (
    <>
      <tr
        class={`border-b border-gray-200 dark:border-gray-700 transition-all duration-200 ${
          hasTasks() ? 'cursor-pointer' : ''
        } ${
          expanded()
            ? 'bg-gray-50 dark:bg-gray-800/40'
            : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'
        } ${!isHealthy() ? 'opacity-60' : ''}`}
        onClick={toggle}
        aria-expanded={expanded()}
      >
        <td class="pl-4 pr-2 py-1">
          <div class="flex items-center gap-1.5 min-w-0">
            <div class="flex items-center gap-1.5 min-w-0 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">
              <span class="truncate font-semibold" title={serviceTitle()}>
                {service.name || service.id || 'Service'}
              </span>
            </div>
            <Show when={service.stack}>
              <span class="text-[10px] text-gray-500 dark:text-gray-400 truncate" title={`Stack: ${service.stack}`}>
                Stack: {service.stack}
              </span>
            </Show>
          </div>
        </td>
        <td class="px-2 py-1">
          <span class={`inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${typeBadgeClass('service')}`}>
            Service
          </span>
        </td>
        <td class="px-2 py-1 text-xs text-gray-700 dark:text-gray-300">
          <span class="truncate" title={service.image}>
            {service.image || '—'}
          </span>
        </td>
        <td class="px-2 py-1 text-xs">
          <span class={`rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${badge.class}`}>
            {badge.label}
          </span>
        </td>
        <td class="px-2 py-1 text-xs text-gray-400 dark:text-gray-500">—</td>
        <td class="px-2 py-1 text-xs text-gray-400 dark:text-gray-500">—</td>
        <td class="px-2 py-1 text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
          <span class="font-semibold text-gray-900 dark:text-gray-100">
            {(service.runningTasks ?? 0)}/{service.desiredTasks ?? 0}
          </span>
          <span class="ml-1 text-gray-500 dark:text-gray-400">tasks</span>
        </td>
        <td class="px-2 py-1 text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
          <Show when={updatedAt} fallback="—">
            {(timestamp) => (
              <span title={new Date(timestamp()).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })}>
                {formatRelativeTime(timestamp())}
              </span>
            )}
          </Show>
        </td>
      </tr>

      <Show when={expanded() && hasTasks()}>
        <tr class="bg-gray-50 dark:bg-gray-900/60">
          <td colSpan={props.columns} class="px-4 py-3">
            <div class="space-y-2 border-l-4 border-gray-200 dark:border-gray-700 pl-3">
              <div class="flex items-center justify-between text-[11px] font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-300">
                <span>Tasks</span>
                <span class="text-[10px] font-normal text-gray-500 dark:text-gray-400">
                  {tasks.length} {tasks.length === 1 ? 'entry' : 'entries'}
                </span>
              </div>
              <div class="overflow-x-auto border border-gray-200 dark:border-gray-700/60 rounded-lg bg-white dark:bg-gray-900/60">
                <table class="min-w-full divide-y divide-gray-100 dark:divide-gray-800/60 text-xs">
                  <thead class="bg-gray-100 dark:bg-gray-900/40 text-[10px] uppercase tracking-wide text-gray-600 dark:text-gray-200">
                    <tr>
                      <th class="py-1 pr-2 text-left font-medium">Task</th>
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
                              <div class="flex items-center gap-2 text-sm text-gray-900 dark:text-gray-100">
                                <span class="truncate font-medium" title={taskTitle()}>
                                  {taskLabel()}
                                </span>
                                <span
                                  class={`hidden sm:inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${typeBadgeClass(
                                    'task',
                                  )}`}
                                >
                                  Task
                                </span>
                              </div>
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
                                <MetricBar value={Math.min(100, cpu)} label={formatPercent(cpu)} type="cpu" />
                              </Show>
                            </td>
                            <td class="py-1 px-2 w-[140px]">
                              <Show when={mem > 0} fallback={<span class="text-gray-400">—</span>}>
                                <MetricBar value={Math.min(100, mem)} label={formatPercent(mem)} type="memory" />
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
          </td>
        </tr>
      </Show>
    </>
  );
};

const DockerUnifiedTable: Component<DockerUnifiedTableProps> = (props) => {
  const tokens = createMemo(() => parseSearchTerm(props.searchTerm));

  const sortedHosts = createMemo(() => {
    const hosts = props.hosts || [];
    return [...hosts].sort((a, b) => {
      const aName = a.displayName || a.hostname || a.id;
      const bName = b.displayName || b.hostname || b.id;
      return aName.localeCompare(bName);
    });
  });

  const groupedRows = createMemo(() => {
    const groups: Array<{ host: DockerHost; rows: DockerRow[] }> = [];
    const filter = props.statsFilter ?? null;
    const searchTokens = tokens();
    const selectedHostId = props.selectedHostId ? props.selectedHostId() : null;

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

        containerRows.push({
          kind: 'container',
          id: container.id || `${host.id}-container-${container.name}`,
          host,
          container,
        });
      });

      services.forEach((service) => {
        if (!serviceMatchesHealthFilter(filter, service)) return;
        const matchesSearch = searchTokens.every((token) => serviceMatchesToken(token, host, service));
        if (!matchesSearch) return;

        const associatedTasks = tasks.filter((task) => {
          if (service.id && task.serviceId) {
            return task.serviceId === service.id;
          }
          if (service.name && task.serviceName) {
            return task.serviceName === service.name;
          }
          return false;
        });

        associatedTasks.forEach((task) => {
          if (task.containerId) serviceOwnedContainers.add(task.containerId.toLowerCase());
          if (task.containerName) serviceOwnedContainers.add(task.containerName.toLowerCase());
        });

        serviceRows.push({
          kind: 'service',
          id: service.id || `${host.id}-service-${service.name}`,
          host,
          service,
          tasks: associatedTasks,
        });
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

    return groups;
  });

  const totalRows = createMemo(() =>
    groupedRows().reduce((acc, group) => acc + group.rows.length, 0),
  );

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

  return (
    <div class="space-y-4">
      <Show
        when={totalRows() > 0}
        fallback={
          <Card padding="lg">
            <EmptyState
              title="No Docker workloads found"
              description={
                totalContainers() === 0 && totalServices() === 0
                  ? 'Add a Docker agent in Settings to start gathering container and service metrics.'
                  : props.searchTerm || props.statsFilter
                    ? 'No Docker containers or services match your current filters.'
                    : 'Docker data is currently unavailable.'
              }
            />
          </Card>
        }
      >
        <Card padding="none" class="overflow-hidden">
          <ScrollableTable minWidth="900px">
            <table class="w-full min-w-[900px] table-fixed border-collapse whitespace-nowrap">
              <thead>
                <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                  <th class="pl-4 pr-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[24%]">
                    Resource
                  </th>
                  <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[11%]">
                    Type
                  </th>
                  <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[17%]">
                    Image / Stack
                  </th>
                  <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[15%]">
                    Status
                  </th>
                  <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[10%]">
                    CPU
                  </th>
                  <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[11%]">
                    Memory
                  </th>
                  <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[10%]">
                    Tasks / Restarts
                  </th>
                  <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[10%]">
                    Updated / Uptime
                  </th>
                </tr>
              </thead>
              <tbody>
                <For each={groupedRows()}>
                  {(group) => (
                    <>
                      <DockerHostGroupHeader host={group.host} colspan={8} />
                      <For each={group.rows}>
                        {(row) =>
                          row.kind === 'container' ? (
                            <DockerContainerRow row={row} columns={8} />
                          ) : (
                            <DockerServiceRow row={row} columns={8} />
                          )
                        }
                      </For>
                    </>
                  )}
                </For>
              </tbody>
            </table>
          </ScrollableTable>
        </Card>

        <div class="flex items-center gap-2 rounded border border-gray-200 bg-gray-50 p-2 text-xs text-gray-600 dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-300">
          <span class="flex items-center gap-1">
            <span class="h-2 w-2 rounded-full bg-green-500" aria-hidden="true" />
            {runningContainers()} running
          </span>
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
