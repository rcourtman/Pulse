import { Component, For, Show, batch, createSignal, createMemo, createEffect, onCleanup } from 'solid-js';
import type { DockerHost, DockerContainer, DockerService, DockerTask } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { ScrollableTable } from '@/components/shared/ScrollableTable';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import {
  DockerTree,
  type DockerTreeHostEntry,
  type DockerTreeSelection,
  type DockerTreeServiceEntry,
} from './DockerTree';

interface DockerUnifiedTableProps {
  hosts: DockerHost[];
  searchTerm?: string;
  statsFilter?: { type: 'host-status' | 'container-state' | 'service-health'; value: string } | null;
}

const findContainerForTask = (containers: DockerContainer[], task: DockerTask) => {
  if (!containers.length) return undefined;

  const taskId = task.containerId?.toLowerCase() ?? '';
  const taskName = task.containerName?.toLowerCase() ?? '';
  const taskNameBase = taskName.split('.')[0] || taskName;

  return containers.find((container) => {
    const id = container.id?.toLowerCase() ?? '';
    const name = container.name?.toLowerCase() ?? '';

    const idMatch =
      !!taskId &&
      (id === taskId || id.includes(taskId) || taskId.includes(id));

    const nameMatch =
      !!taskName &&
      (name === taskName ||
        name.includes(taskName) ||
        taskName.includes(name) ||
        (!!taskNameBase && (name === taskNameBase || name.includes(taskNameBase))));

    return idMatch || nameMatch;
  });
};

const OFFLINE_HOST_STATUSES = new Set(['offline', 'error', 'unreachable', 'down', 'disconnected']);
const DEGRADED_HOST_STATUSES = new Set(['degraded', 'warning', 'maintenance', 'partial', 'initializing', 'unknown']);

const STOPPED_CONTAINER_STATES = new Set(['exited', 'stopped', 'created', 'paused']);
const ERROR_CONTAINER_STATES = new Set(['restarting', 'dead', 'removing', 'failed', 'error', 'oomkilled', 'unhealthy']);

// Persistent state for expanded hosts and services
const hostExpandState = new Map<string, boolean>();
const hostExpandSignals = new Map<string, ReturnType<typeof createSignal<boolean>>>();
const serviceExpandState = new Map<string, boolean>();
const serviceExpandSignals = new Map<string, ReturnType<typeof createSignal<boolean>>>();

const getTaskNodeId = (serviceKey: string, task: DockerTask, index: number) => {
  if (task.id) {
    return `${serviceKey}:task:${task.id}`;
  }
  if (task.containerId) {
    return `${serviceKey}:task:${task.containerId}`;
  }
  if (task.containerName) {
    return `${serviceKey}:task:${task.containerName}`;
  }
  if (task.slot !== undefined && task.slot !== null) {
    return `${serviceKey}:task:slot-${task.slot}`;
  }
  return `${serviceKey}:task:${index}`;
};

// Docker Host Group Header Component (matches NodeGroupHeader style)
interface DockerHostHeaderProps {
  host: DockerHost;
  colspan: number;
  isExpanded: boolean;
  onToggle: () => void;
  isActive?: boolean;
}

const DockerHostHeader: Component<DockerHostHeaderProps> = (props) => {
  const status = () => props.host.status?.toLowerCase() ?? 'unknown';
  const isOnline = () => status() === 'online';
  const isOffline = () => OFFLINE_HOST_STATUSES.has(status());
  const displayName = () => props.host.displayName || props.host.hostname || props.host.id;

  const totalContainers = () => (props.host.containers?.length || 0);
  const runningContainers = () =>
    (props.host.containers?.filter((c) => c.state?.toLowerCase() === 'running').length || 0);
  const totalServices = () => (props.host.services?.length || 0);

  return (
    <tr class="bg-gray-50 dark:bg-gray-900/40">
      <td
        colspan={props.colspan}
        class={`py-1 pr-2 pl-4 text-[12px] sm:text-sm font-semibold text-slate-700 dark:text-slate-100 ${
          props.isActive ? 'bg-sky-50 dark:bg-sky-900/30' : ''
        }`}
      >
        <button
          type="button"
          onClick={props.onToggle}
          class={`flex flex-wrap items-center gap-3 w-full text-left transition-colors duration-150 hover:text-sky-600 dark:hover:text-sky-400 ${
            isOffline() ? 'opacity-60' : ''
          } ${props.isActive ? 'text-sky-700 dark:text-sky-300' : ''}`}
          title={isOnline() ? 'Online' : isOffline() ? 'Offline' : 'Degraded'}
        >
          {/* Expand/collapse arrow */}
          <svg
            class={`h-4 w-4 transition-transform duration-200 ${props.isExpanded ? 'rotate-90' : ''}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>

          <span>{displayName()}</span>

          {/* Status badge */}
          {(() => {
            if (isOnline()) {
              return (
                <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300">
                  Online
                </span>
              );
            }
            if (isOffline()) {
              return (
                <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300">
                  Offline
                </span>
              );
            }
            return (
              <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300">
                Degraded
              </span>
            );
          })()}

          {/* Container count */}
          <Show when={totalContainers() > 0}>
            <span class="text-[10px] text-slate-500 dark:text-slate-400">
              {runningContainers()}/{totalContainers()} containers running
            </span>
          </Show>

          {/* Service count */}
          <Show when={totalServices() > 0}>
            <span class="text-[10px] text-slate-500 dark:text-slate-400">
              {totalServices()} services
            </span>
          </Show>
        </button>
      </td>
    </tr>
  );
};

// Service Row Component (expandable for task containers)
interface ServiceRowProps {
  service: DockerService;
  hostId: string;
  tasks: DockerTreeServiceEntry['tasks'];
  containers: DockerContainer[];
  isExpanded: boolean;
  onToggle: () => void;
  isSelected?: boolean;
  rowRef?: (row: HTMLTableRowElement | null) => void;
  selectedTaskId?: string | null;
  onTaskMount?: (taskNodeId: string, row: HTMLTableRowElement) => void;
  onTaskUnmount?: (taskNodeId: string) => void;
}

const ServiceRow: Component<ServiceRowProps> = (props) => {
  const desiredTasks = () => props.service.desiredTasks ?? 0;
  const runningTasks = () => props.service.runningTasks ?? 0;
  const isHealthy = () => desiredTasks() > 0 && runningTasks() >= desiredTasks();
  const hasTasks = () => props.tasks.length > 0;

  onCleanup(() => {
    props.rowRef?.(null);
  });

  const healthBadge = () => {
    if (desiredTasks() === 0) {
      return (
        <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300">
          No tasks
        </span>
      );
    }
    if (isHealthy()) {
      return (
        <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300">
          Healthy
        </span>
      );
    }
    return (
      <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300">
        Degraded ({runningTasks()}/{desiredTasks()})
      </span>
    );
  };

  return (
    <>
      <tr
        ref={(row) => row && props.rowRef?.(row)}
        class={`border-t border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50 ${
          props.isSelected ? 'bg-sky-50 dark:bg-sky-900/30' : ''
        }`}
      >
        <td class="pl-8 pr-2 py-2 text-sm text-gray-900 dark:text-gray-100">
          <button
            type="button"
            onClick={props.onToggle}
            class={`flex items-center gap-2 w-full text-left ${
              props.isSelected ? 'text-sky-700 dark:text-sky-300' : ''
            }`}
            disabled={!hasTasks()}
          >
            {/* Expand arrow - only show if there are tasks */}
            <Show when={hasTasks()}>
              <svg
                class={`h-3 w-3 transition-transform duration-200 flex-shrink-0 ${props.isExpanded ? 'rotate-90' : ''}`}
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 5l7 7-7 7"
                />
              </svg>
            </Show>
            <span class="font-medium truncate">{props.service.name || props.service.id}</span>
          </button>
        </td>
        <td class="px-2 py-2 text-xs text-gray-600 dark:text-gray-400">
          <div class="truncate" title={props.service.image || undefined}>
            {props.service.image || 'Image not specified'}
          </div>
          <Show when={props.service.mode}>
            <div class="mt-1 text-[10px] text-gray-500 dark:text-gray-400 whitespace-nowrap">
              {props.service.mode}
            </div>
          </Show>
        </td>
        <td class="px-2 py-2 text-xs">{healthBadge()}</td>
      </tr>

      {/* Task Containers Drawer */}
      <Show when={props.isExpanded && hasTasks()}>
        <tr class="bg-gray-50 dark:bg-gray-900/60">
          <td colSpan={3} class="px-4 py-3 pl-12">
            <div class="space-y-2">
              <h4 class="text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wide">
                Task Containers ({props.tasks.length})
              </h4>
              <div class="overflow-x-auto">
                <table class="min-w-full text-xs">
                  <thead>
                    <tr class="text-gray-600 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700">
                      <th class="text-left py-1 pr-2 font-medium">Task/Container</th>
                      <th class="text-left py-1 px-2 font-medium">Node</th>
                      <th class="text-left py-1 px-2 font-medium">State</th>
                      <th class="text-left py-1 px-2 font-medium">CPU</th>
                      <th class="text-left py-1 px-2 font-medium">Memory</th>
                      <th class="text-left py-1 px-2 font-medium">Started</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-gray-800">
                    <For each={props.tasks}>
                      {(taskEntry) => {
                        const task = taskEntry.task;
                        const currentState = task.currentState?.toLowerCase() || 'unknown';
                        const stateBadge = () => {
                          if (currentState === 'running') {
                            return 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300';
                          }
                          if (currentState === 'failed') {
                            return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300';
                          }
                          return 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300';
                        };

                        // Find the corresponding container for this task
                        const container = findContainerForTask(props.containers, task);

                        const hasCpuData = () =>
                          typeof container?.cpuPercent === 'number' && !Number.isNaN(container.cpuPercent);
                        const hasMemData = () =>
                          typeof container?.memoryPercent === 'number' && !Number.isNaN(container.memoryPercent);
                        const cpuPercent = () => (hasCpuData() ? container!.cpuPercent : 0);
                        const memPercent = () => (hasMemData() ? container!.memoryPercent : 0);

                        const taskLabel = () => {
                          const name = task.containerName || task.containerId?.slice(0, 12) || '—';
                          if (task.slot !== undefined && task.slot !== null) {
                            return `${name}.${task.slot}`;
                          }
                          return name;
                        };

                        onCleanup(() => {
                          props.onTaskUnmount?.(taskEntry.nodeId);
                        });

                        return (
                          <tr
                            ref={(row) => row && props.onTaskMount?.(taskEntry.nodeId, row)}
                            class={`hover:bg-gray-100 dark:hover:bg-gray-800/70 ${
                              props.selectedTaskId === taskEntry.nodeId ? 'bg-sky-50 dark:bg-sky-900/30' : ''
                            }`}
                          >
                            <td class="py-0.5 pr-2 text-gray-900 dark:text-gray-100" title={task.id}>
                              {taskLabel()}
                            </td>
                            <td class="py-0.5 px-2 text-gray-600 dark:text-gray-400">
                              {task.nodeName || task.nodeId || '—'}
                            </td>
                            <td class="py-0.5 px-2">
                              <span
                                class={`rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${stateBadge()}`}
                              >
                                {task.currentState || 'Unknown'}
                              </span>
                            </td>
                            <td class="py-0.5 px-2">
                              <Show when={hasCpuData()} fallback={<span class="text-xs text-gray-400">—</span>}>
                                <MetricBar value={cpuPercent()} label={`${cpuPercent().toFixed(1)}%`} />
                              </Show>
                            </td>
                            <td class="py-0.5 px-2">
                              <Show when={hasMemData()} fallback={<span class="text-xs text-gray-400">—</span>}>
                                <MetricBar value={memPercent()} label={`${memPercent().toFixed(1)}%`} type="memory" />
                              </Show>
                            </td>
                            <td class="py-0.5 px-2 text-gray-600 dark:text-gray-400 text-[10px]">
                              {(() => {
                                const timestamp = task.startedAt || task.createdAt;
                                if (!timestamp) return '—';
                                // Handle both Unix timestamps (number) and ISO strings (from backend time.Time)
                                const date = typeof timestamp === 'number'
                                  ? new Date(timestamp * 1000)
                                  : new Date(timestamp);
                                return date.toLocaleString();
                              })()}
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

// Container Row Component
interface ContainerRowProps {
  container: DockerContainer;
  indent?: boolean;
  isSelected?: boolean;
  rowRef?: (row: HTMLTableRowElement | null) => void;
}

const ContainerRow: Component<ContainerRowProps> = (props) => {
  const formatPorts = () => {
    if (!props.container.ports || props.container.ports.length === 0) return '—';
    return props.container.ports
      .map((p) => {
        if (p.publicPort) {
          return `${p.publicPort}:${p.privatePort}/${p.protocol}`;
        }
        return `${p.privatePort}/${p.protocol}`;
      })
      .join(', ');
  };

  const stateBadge = () => {
    const state = props.container.state?.toLowerCase() || 'unknown';
    if (state === 'running') {
      return (
        <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300">
          Running
        </span>
      );
    }
    if (state === 'exited' || state === 'stopped') {
      return (
        <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300">
          Stopped
        </span>
      );
    }
    return (
      <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300">
        {state}
      </span>
    );
  };

  const hasCpuData = () =>
    typeof props.container.cpuPercent === 'number' && !Number.isNaN(props.container.cpuPercent);
  const hasMemData = () =>
    typeof props.container.memoryPercent === 'number' && !Number.isNaN(props.container.memoryPercent);
  const cpuPercent = () => (hasCpuData() ? props.container.cpuPercent! : 0);
  const memPercent = () => (hasMemData() ? props.container.memoryPercent! : 0);

  onCleanup(() => {
    props.rowRef?.(null);
  });

  return (
    <tr
      ref={(row) => row && props.rowRef?.(row)}
      class={`border-t border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50 ${
        props.isSelected ? 'bg-sky-50 dark:bg-sky-900/30' : ''
      }`}
    >
      <td
        class={`${props.indent ? 'pl-8' : 'pl-4'} pr-2 py-0.5 text-sm text-gray-900 dark:text-gray-100 truncate`}
        title={props.container.name || props.container.id}
      >
        {props.container.name || props.container.id?.slice(0, 12)}
      </td>
      <td class="px-2 py-0.5 text-xs text-gray-600 dark:text-gray-400">
        <span class="truncate max-w-[10rem]" title={props.container.image || undefined}>
          {props.container.image || 'Image not specified'}
        </span>
      </td>
      <td class="px-2 py-0.5 text-xs">{stateBadge()}</td>
      <td class="px-2 py-0.5">
        <Show when={hasCpuData()} fallback={<span class="text-xs text-gray-400">—</span>}>
          <MetricBar value={cpuPercent()} label={`${cpuPercent().toFixed(1)}%`} />
        </Show>
      </td>
      <td class="px-2 py-0.5 w-[100px]">
        <Show when={hasMemData()} fallback={<span class="text-xs text-gray-400">—</span>}>
          <MetricBar value={memPercent()} label={`${memPercent().toFixed(1)}%`} type="memory" />
        </Show>
      </td>
      <td class="px-2 py-0.5 text-xs text-gray-600 dark:text-gray-400 truncate max-w-[10rem]" title={formatPorts()}>
        <span class="truncate">{formatPorts()}</span>
      </td>
    </tr>
  );
};

export const DockerUnifiedTable: Component<DockerUnifiedTableProps> = (props) => {
  // Track expanded state for each host
  const getHostExpandState = (hostId: string) => {
    if (!hostExpandState.has(hostId)) {
      hostExpandState.set(hostId, true);
    }
    if (!hostExpandSignals.has(hostId)) {
      hostExpandSignals.set(
        hostId,
        createSignal(hostExpandState.get(hostId) ?? true),
      );
    }

    const [isExpanded, setIsExpanded] = hostExpandSignals.get(hostId)!;

    const setExpanded = (value: boolean) =>
      setIsExpanded(() => {
        hostExpandState.set(hostId, value);
        return value;
      });

    return {
      isExpanded,
      toggle: () =>
        setIsExpanded((prev) => {
          const next = !prev;
          hostExpandState.set(hostId, next);
          return next;
        }),
      setExpanded,
    };
  };

  // Track expanded state for each service
  const getServiceExpandState = (serviceKey: string) => {
    if (!serviceExpandSignals.has(serviceKey)) {
      serviceExpandSignals.set(
        serviceKey,
        createSignal(serviceExpandState.get(serviceKey) ?? false),
      );
    }

    const [isExpanded, setIsExpanded] = serviceExpandSignals.get(serviceKey)!;

    const setExpanded = (value: boolean) =>
      setIsExpanded(() => {
        serviceExpandState.set(serviceKey, value);
        return value;
      });

    return {
      isExpanded,
      toggle: () =>
        setIsExpanded((prev) => {
          const next = !prev;
          serviceExpandState.set(serviceKey, next);
          return next;
        }),
      setExpanded,
    };
  };

  const normalizeContainerState = (state?: string | null, status?: string | null) => {
    const lowerState = state?.toLowerCase().trim();
    if (lowerState) return lowerState;
    const lowerStatus = status?.toLowerCase().trim();
    if (!lowerStatus) return '';
    if (lowerStatus.startsWith('up')) return 'running';
    if (lowerStatus.startsWith('exited')) return 'exited';
    if (lowerStatus.startsWith('created')) return 'created';
    if (lowerStatus.startsWith('paused')) return 'paused';
    if (lowerStatus.includes('restarting')) return 'restarting';
    if (lowerStatus.includes('unhealthy')) return 'unhealthy';
    if (lowerStatus.includes('dead')) return 'dead';
    if (lowerStatus.includes('removing')) return 'removing';
    return lowerStatus;
  };

  const matchesContainerStateFilter = (filterValue: string, container?: DockerContainer | null) => {
    if (!container) return false;
    const state = normalizeContainerState(container.state, container.status);
    if (filterValue === 'running') {
      return state === 'running';
    }
    if (filterValue === 'stopped') {
      return STOPPED_CONTAINER_STATES.has(state);
    }
    if (filterValue === 'error') {
      return ERROR_CONTAINER_STATES.has(state);
    }
    return true;
  };

  const matchesTaskStateFilter = (filterValue: string, task: DockerTask, container?: DockerContainer | null) => {
    if (!filterValue) return true;
    if (container && matchesContainerStateFilter(filterValue, container)) {
      return true;
    }

    const current = task.currentState?.toLowerCase() ?? '';
    if (filterValue === 'running') {
      return current === 'running';
    }
    if (filterValue === 'stopped') {
      return current === 'complete' || current === 'shutdown' || current === 'stopped';
    }
    if (filterValue === 'error') {
      return current === 'failed' || current === 'error';
    }
    return true;
  };

  const hostMatchesFilter = (host: DockerHost) => {
    const filter = props.statsFilter;
    if (!filter || filter.type !== 'host-status') {
      return true;
    }
    const status = host.status?.toLowerCase() ?? 'unknown';
    switch (filter.value) {
      case 'offline':
        return OFFLINE_HOST_STATUSES.has(status);
      case 'degraded':
        return DEGRADED_HOST_STATUSES.has(status);
      case 'online':
        return status === 'online';
      default:
        return true;
    }
  };

  // Parse search terms
  const searchTerms = createMemo(() => {
    const term = props.searchTerm || '';
    return term
      .toLowerCase()
      .split(/[\s,]+/)
      .map((t) => t.trim())
      .filter(Boolean);
  });

  // Check if a container matches search terms
  const containerMatchesSearch = (host: DockerHost, container: DockerContainer) => {
    const terms = searchTerms();
    if (terms.length === 0) return true;

    return terms.every((term) => {
      const [prefix, value] = term.includes(':') ? term.split(/:(.+)/) : [null, term];
      const target = value || term;

      const tokens = [
        container.name,
        container.image,
        container.id,
        container.state,
        container.status,
        host.displayName,
        host.hostname,
      ];

      const hasToken = (list: (string | undefined)[]) =>
        list
          .filter(Boolean)
          .some((entry) => entry!.toLowerCase().includes(target));

      if (prefix) {
        switch (prefix) {
          case 'host':
            return hasToken([host.displayName, host.hostname]);
          case 'name':
            return hasToken([container.name]);
          case 'image':
            return hasToken([container.image]);
          case 'state':
            return hasToken([container.state, container.status]);
          case 'id':
            return hasToken([container.id]);
          default:
            return hasToken(tokens);
        }
      }

      return hasToken(tokens);
    });
  };

  // Check if a service matches search terms
  const serviceMatchesSearch = (host: DockerHost, service: DockerService) => {
    const terms = searchTerms();
    if (terms.length === 0) return true;

    return terms.every((term) => {
      const [prefix, value] = term.includes(':') ? term.split(/:(.+)/) : [null, term];
      const target = value || term;

      const tokens = [
        service.name,
        service.id,
        service.image,
        host.displayName,
        host.hostname,
      ];

      const hasToken = (list: (string | undefined)[]) =>
        list
          .filter(Boolean)
          .some((entry) => entry!.toLowerCase().includes(target));

      if (prefix) {
        switch (prefix) {
          case 'host':
            return hasToken([host.displayName, host.hostname]);
          case 'name':
          case 'service':
            return hasToken([service.name, service.id]);
          case 'image':
            return hasToken([service.image]);
          default:
            return hasToken(tokens);
        }
      }

      return hasToken(tokens);
    });
  };

  // Sort hosts alphabetically
  const sortedHosts = createMemo(() => {
    const hosts = props.hosts || [];
    return [...hosts].sort((a, b) => {
      const aName = a.displayName || a.hostname || a.id || '';
      const bName = b.displayName || b.hostname || b.id || '';
      return aName.localeCompare(bName);
    });
  });

  const containerFilterValue = () =>
    props.statsFilter?.type === 'container-state' ? props.statsFilter.value : null;

const visibleHosts = createMemo<DockerTreeHostEntry[]>(() => {
    const results: DockerTreeHostEntry[] = [];
    const hosts = sortedHosts();
    const filterValue = containerFilterValue();

    hosts.forEach((host, index) => {
      if (!hostMatchesFilter(host)) {
        return;
      }

      const hostId =
        host.id ||
        host.hostname ||
        host.displayName ||
        `host-${index}`;

      const hostContainers = host.containers || [];
      const hostTasks = host.tasks || [];

      const referencedContainers = new Set<string>();
      hostTasks.forEach((task) => {
        if (task.containerId) referencedContainers.add(task.containerId);
        if (task.containerName) referencedContainers.add(task.containerName);
      });

      const standalone = hostContainers
        .filter((container) => {
          const id = container.id || '';
          const name = container.name || '';

          if (referencedContainers.has(id) || referencedContainers.has(name)) {
            return false;
          }

          if (!containerMatchesSearch(host, container)) {
            return false;
          }

          if (filterValue && !matchesContainerStateFilter(filterValue, container)) {
            return false;
          }

          return true;
        })
        .map((container, idx) => ({
          container,
          nodeId: container.id
            ? `${hostId}:container:${container.id}`
            : container.name
            ? `${hostId}:container:${container.name}`
            : `${hostId}:container:${idx}`,
        }));

      const services = (host.services || []).reduce<DockerTreeServiceEntry[]>((rows, service) => {
        if (!serviceMatchesSearch(host, service)) {
          return rows;
        }

        const filteredTasks = hostTasks.filter((task) => {
          const matchesService =
            (task.serviceId && task.serviceId === service.id) ||
            (!task.serviceId && task.serviceName && task.serviceName === service.name);
          if (!matchesService) return false;

          if (!filterValue) return true;
          const container = findContainerForTask(hostContainers, task);
          return matchesTaskStateFilter(filterValue, task, container);
        });

        if (filterValue && filteredTasks.length === 0) {
          return rows;
        }

        const identifier = service.id || service.name || 'service';
        const serviceKey = `${hostId}:${identifier}`;

        const tasks = filteredTasks.map((task, idx) => ({
          task,
          nodeId: getTaskNodeId(serviceKey, task, idx),
        }));

        rows.push({
          key: serviceKey,
          service,
          tasks,
        });

        return rows;
      }, []);

      if (services.length === 0 && standalone.length === 0) {
        return;
      }

      results.push({
        host,
        hostId,
        containers: hostContainers,
        services,
        standaloneContainers: standalone,
      });
    });

    return results;
});

  const [selectedNode, setSelectedNode] = createSignal<DockerTreeSelection | null>(null);
  const [isMobileTreeOpen, setIsMobileTreeOpen] = createSignal(false);

  const displayedHosts = createMemo(() => {
    const hosts = visibleHosts();
    const selection = selectedNode();
    if (!selection) return hosts;
    if (selection.type === 'host') {
      const match = hosts.find((host) => host.hostId === selection.hostId);
      return match ? [match] : hosts;
    }
    return hosts.filter((host) => host.hostId === selection.hostId);
  });

  const hostRefs = new Map<string, HTMLElement>();
  const serviceRefs = new Map<string, HTMLTableRowElement>();
  const taskRefs = new Map<string, HTMLTableRowElement>();
  const containerRefs = new Map<string, HTMLTableRowElement>();

  const assignHostRef = (hostId: string, el: HTMLElement | null | undefined) => {
    if (el) {
      hostRefs.set(hostId, el);
    } else {
      hostRefs.delete(hostId);
    }
  };

  const assignServiceRef = (serviceKey: string, el: HTMLTableRowElement | null | undefined) => {
    if (el) {
      serviceRefs.set(serviceKey, el);
    } else {
      serviceRefs.delete(serviceKey);
    }
  };

  const registerTaskRef = (nodeId: string, row: HTMLTableRowElement) => {
    taskRefs.set(nodeId, row);
  };

  const unregisterTaskRef = (nodeId: string) => {
    taskRefs.delete(nodeId);
  };

  const assignContainerRef = (nodeId: string, el: HTMLTableRowElement | null | undefined) => {
    if (el) {
      containerRefs.set(nodeId, el);
    } else {
      containerRefs.delete(nodeId);
    }
  };

  const scrollToSelection = (selection: DockerTreeSelection, smooth = true) => {
    if (typeof window === 'undefined') return false;

    const verticalScroll = (
      element?: Element | null,
      block: ScrollLogicalPosition = 'nearest',
    ) => {
      if (!element) return false;
      const rect = element.getBoundingClientRect();
      const topAllowance = 96;
      const bottomAllowance = window.innerHeight - 32;

      if (rect.top >= topAllowance && rect.bottom <= bottomAllowance) {
        return true;
      }

      element.scrollIntoView({
        behavior: smooth ? 'smooth' : 'auto',
        block,
        inline: 'nearest',
      });
      return true;
    };

    if (selection.type === 'host') {
      return verticalScroll(hostRefs.get(selection.hostId), 'start');
    }

    if (selection.type === 'service') {
      return verticalScroll(serviceRefs.get(selection.id), 'nearest');
    }

    if (selection.type === 'task') {
      return verticalScroll(taskRefs.get(selection.id), 'center');
    }

    if (selection.type === 'container') {
      return verticalScroll(containerRefs.get(selection.id), 'nearest');
    }

    return false;
  };

  const handleTreeSelect = (selection: DockerTreeSelection) => {
    batch(() => {
      setSelectedNode(selection);

      const hostState = getHostExpandState(selection.hostId);
      hostState.setExpanded(true);

      if (selection.type === 'task') {
        const serviceState = getServiceExpandState(selection.serviceKey);
        serviceState.setExpanded(true);
      }
    });

    setIsMobileTreeOpen(false);
  };

  const buildSelectionFingerprint = (selection: DockerTreeSelection) => {
    switch (selection.type) {
      case 'host':
        return `host:${selection.hostId}`;
      case 'service':
        return `service:${selection.hostId}:${selection.id}`;
      case 'task':
        return `task:${selection.hostId}:${selection.serviceKey}:${selection.id}`;
      case 'container':
        return `container:${selection.hostId}:${selection.id}`;
      default:
        return '';
    }
  };

  let lastScrollFingerprint = '';

  const attemptScrollToSelection = (
    selection: DockerTreeSelection,
    fingerprint: string,
    remainingAttempts = 8,
  ) => {
    if (remainingAttempts <= 0) return;
    const didScroll = scrollToSelection(selection, remainingAttempts === 8);
    if (didScroll) {
      lastScrollFingerprint = fingerprint;
    } else if (typeof window !== 'undefined') {
      requestAnimationFrame(() =>
        attemptScrollToSelection(selection, fingerprint, remainingAttempts - 1),
      );
    }
  };

  createEffect(() => {
    const selection = selectedNode();
    if (!selection) return;
    const hosts = visibleHosts();
    const hostEntry = hosts.find((entry) => entry.hostId === selection.hostId);
    if (!hostEntry) {
      setSelectedNode(null);
      return;
    }

    if (selection.type === 'service') {
      const exists = hostEntry.services.some((service) => service.key === selection.id);
      if (!exists) {
        setSelectedNode({ type: 'host', hostId: hostEntry.hostId, id: hostEntry.hostId });
        return;
      }
    } else if (selection.type === 'task') {
      const serviceEntry = hostEntry.services.find((service) => service.key === selection.serviceKey);
      if (!serviceEntry) {
        setSelectedNode({ type: 'host', hostId: hostEntry.hostId, id: hostEntry.hostId });
        return;
      }
      const taskExists = serviceEntry.tasks.some((task) => task.nodeId === selection.id);
      if (!taskExists) {
        setSelectedNode({ type: 'service', hostId: hostEntry.hostId, id: serviceEntry.key });
        return;
      }
    } else if (selection.type === 'container') {
      const exists = hostEntry.standaloneContainers.some((container) => container.nodeId === selection.id);
      if (!exists) {
        setSelectedNode({ type: 'host', hostId: hostEntry.hostId, id: hostEntry.hostId });
        return;
      }
    }

    const fingerprint = buildSelectionFingerprint(selection);
    if (!fingerprint || fingerprint === lastScrollFingerprint) return;

    attemptScrollToSelection(selection, fingerprint);
  });

  const hasVisibleHosts = createMemo(() => visibleHosts().length > 0);

  return (
    <div class="flex flex-col gap-4 lg:flex-row">
      <Show when={hasVisibleHosts()}>
        <div class="flex justify-end lg:hidden">
          <button
            type="button"
            onClick={() => setIsMobileTreeOpen(true)}
            class="inline-flex items-center gap-2 rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-600 shadow-sm transition hover:border-slate-400 hover:text-slate-700 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-200 dark:hover:border-slate-500"
            aria-label="Browse Docker hosts"
          >
            <svg class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
              <path d="M4 4h12v2H4V4zm0 5h12v2H4V9zm0 5h12v2H4v-2z" />
            </svg>
            Browse Hosts
          </button>
        </div>
      </Show>

      <Show when={hasVisibleHosts()}>
        <aside class="hidden lg:block lg:w-72 lg:flex-shrink-0 lg:sticky lg:top-20 lg:self-start">
          <DockerTree
            hosts={visibleHosts()}
            selected={selectedNode()}
            onSelect={handleTreeSelect}
            getHostState={getHostExpandState}
            getServiceState={getServiceExpandState}
          />
        </aside>
      </Show>

      <Show when={hasVisibleHosts() && isMobileTreeOpen()}>
        <div class="lg:hidden">
          <div
            class="fixed inset-0 z-40 bg-slate-900/40 backdrop-blur-sm"
            onClick={() => setIsMobileTreeOpen(false)}
          />
          <aside class="fixed inset-y-0 left-0 z-50 flex w-72 max-w-full flex-col bg-white shadow-xl dark:bg-slate-900">
            <div class="flex items-center justify-between border-b border-slate-200 px-4 py-3 dark:border-slate-700">
              <h2 class="text-sm font-semibold text-slate-700 dark:text-slate-200">Docker Hosts</h2>
              <button
                type="button"
                class="rounded-md p-1 text-slate-500 transition hover:text-slate-700 dark:text-slate-300 dark:hover:text-slate-100"
                onClick={() => setIsMobileTreeOpen(false)}
                aria-label="Close host navigation"
              >
                <svg class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                  <path
                    fill-rule="evenodd"
                    d="M10 8.586l4.95-4.95 1.414 1.414L11.414 10l4.95 4.95-1.414 1.414L10 11.414l-4.95 4.95-1.414-1.414L8.586 10l-4.95-4.95L5.05 3.636 10 8.586z"
                    clip-rule="evenodd"
                  />
                </svg>
              </button>
            </div>
            <div class="flex-1 overflow-y-auto px-2 pb-6">
              <DockerTree
                hosts={visibleHosts()}
                selected={selectedNode()}
                onSelect={handleTreeSelect}
                getHostState={getHostExpandState}
                getServiceState={getServiceExpandState}
              />
            </div>
          </aside>
        </div>
      </Show>

      <div class="flex-1 min-w-0 space-y-6">
        <For each={displayedHosts()}>
          {(entry) => {
            const hostState = getHostExpandState(entry.hostId);
            const currentSelection = () => selectedNode();
            const isHostActive = () => currentSelection()?.hostId === entry.hostId;

            const displayedServices = createMemo(() => {
              const selection = currentSelection();
              if (!selection || selection.type === 'host') return entry.services;
              if (selection.type === 'service') {
                return entry.services.filter((service) => service.key === selection.id);
              }
              if (selection.type === 'task') {
                return entry.services.filter((service) => service.key === selection.serviceKey);
              }
              return [];
            });

            const displayedStandaloneContainers = createMemo(() => {
              const selection = currentSelection();
              if (!selection || selection.type === 'host') return entry.standaloneContainers;
              if (selection.type === 'container') {
                return entry.standaloneContainers.filter((container) => container.nodeId === selection.id);
              }
              return [];
            });

            const shouldShowServices = createMemo(() => displayedServices().length > 0);
            const shouldShowContainers = createMemo(() => displayedStandaloneContainers().length > 0);

            return (
              <section
                id={`host-${entry.hostId}`}
                ref={(el) => assignHostRef(entry.hostId, el)}
                class="space-y-4 scroll-mt-28 min-w-0"
              >
                <Card
                  padding="none"
                  class={`overflow-hidden ${isHostActive() ? 'ring-1 ring-sky-400' : ''}`}
                >
                  <table class="w-full border-collapse">
                    <tbody>
                      <DockerHostHeader
                        host={entry.host}
                        colspan={6}
                        isExpanded={hostState.isExpanded()}
                        onToggle={hostState.toggle}
                        isActive={isHostActive()}
                      />
                    </tbody>
                  </table>
                </Card>

                <Show when={hostState.isExpanded()}>
                  <div class="space-y-4">
                    <Show when={shouldShowServices()}>
                      <Card padding="none" class="overflow-hidden">
                        <ScrollableTable persistKey={`services-${entry.hostId}`} minWidth="680px">
                          <table class="w-full border-collapse">
                            <thead>
                              <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                                <th class="pl-4 pr-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                  Service
                                </th>
                                <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                  Image
                                </th>
                                <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                  Status
                                </th>
                              </tr>
                            </thead>
                          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                              <For each={displayedServices()}>
                                {(serviceEntry) => {
                                  const serviceState = getServiceExpandState(serviceEntry.key);
                                  const serviceSelected = () => {
                                    const selection = selectedNode();
                                    if (!selection) return false;
                                    if (selection.type === 'service') {
                                      return selection.id === serviceEntry.key;
                                    }
                                    if (selection.type === 'task') {
                                      return selection.serviceKey === serviceEntry.key;
                                    }
                                    return false;
                                  };
                              const selectedTaskId = () => {
                                const selection = selectedNode();
                                if (selection?.type === 'task' && selection.serviceKey === serviceEntry.key) {
                                  return selection.id;
                                }
                                return null;
                              };

                              onCleanup(() => {
                                serviceRefs.delete(serviceEntry.key);
                              });

                              return (
                                <ServiceRow
                                  service={serviceEntry.service}
                                  hostId={entry.hostId}
                                  tasks={serviceEntry.tasks}
                                  containers={entry.containers}
                                  isExpanded={serviceState.isExpanded()}
                                  onToggle={serviceState.toggle}
                                  isSelected={serviceSelected()}
                                  rowRef={(row) => assignServiceRef(serviceEntry.key, row)}
                                  selectedTaskId={selectedTaskId()}
                                  onTaskMount={registerTaskRef}
                                  onTaskUnmount={unregisterTaskRef}
                                />
                              );
                            }}
                          </For>
                            </tbody>
                          </table>
                        </ScrollableTable>
                      </Card>
                    </Show>

                    <Show when={shouldShowContainers()}>
                      <Card padding="none" class="overflow-hidden">
                        <ScrollableTable persistKey={`containers-${entry.hostId}`}>
                          <table class="w-full border-collapse">
                            <thead>
                              <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                                <th class="pl-4 pr-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                  Container
                                </th>
                                <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                  Image
                                </th>
                                <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                  Status
                                </th>
                                <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                  CPU
                                </th>
                                <th class="px-2 py-1 text-left text-[11px] sm:text-xs.font-medium uppercase tracking-wider">
                                  Memory
                                </th>
                                <th class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                  Ports
                                </th>
                              </tr>
                            </thead>
                          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                              <For each={displayedStandaloneContainers()}>
                                {(containerEntry) => {
                                  const isContainerSelected = () =>
                                    selectedNode()?.type === 'container' &&
                                    selectedNode()?.id === containerEntry.nodeId;

                                  return (
                                    <ContainerRow
                                      container={containerEntry.container}
                                      indent={false}
                                      isSelected={isContainerSelected()}
                                      rowRef={(row) => assignContainerRef(containerEntry.nodeId, row)}
                                    />
                                  );
                                }}
                              </For>
                            </tbody>
                          </table>
                        </ScrollableTable>
                      </Card>
                    </Show>
                  </div>
                </Show>
              </section>
            );
          }}
        </For>
      </div>
    </div>
  );
};
