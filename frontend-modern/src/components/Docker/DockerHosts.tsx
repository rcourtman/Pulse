import type { Component } from 'solid-js';
import { For, Show, createMemo, createSignal, createEffect, on } from 'solid-js';
import type { DockerHost, DockerContainer, Alert } from '@/types/api';
import { formatBytes, formatRelativeTime, formatUptime } from '@/utils/format';
import { Card } from '@/components/shared/Card';
import { ScrollableTable } from '@/components/shared/ScrollableTable';
import { EmptyState } from '@/components/shared/EmptyState';
import { CopyButton } from '@/components/shared/CopyButton';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { DockerFilter } from './DockerFilter';
import { getAlertStyles } from '@/utils/alerts';
// import type { DockerHostSummary } from './DockerHostSummaryTable';
import { renderDockerStatusBadge } from './DockerStatusBadge';
import { useWebSocket } from '@/App';

interface DockerHostsProps {
  hosts: DockerHost[];
  activeAlerts?: Record<string, Alert> | any; // Can be Store or plain object
}

interface ContainerEntry {
  host: DockerHost;
  container: DockerContainer;
}

// Drawer state storage
const drawerState = new Map<string, boolean>();

const buildContainerId = (container: DockerContainer, hostId: string) => {
  return `${hostId}-${container.id}`;
};

// Unused - kept for potential future use
// const formatContainerStatus = (container: DockerContainer) => {
//   const primary = container.state || container.status || 'unknown';
//   if (container.health) {
//     return `${primary} · ${container.health}`;
//   }
//   return primary;
// };

// Unused - kept for potential future use
// const DockerGroupHeader: Component<{
//   host: DockerHost;
//   colspan: number;
//   onToggle: (hostId: string) => void;
//   selected: boolean;
// }> = (props) => {
//   const lastSeenRelative = () =>
//     props.host.lastSeen ? formatRelativeTime(props.host.lastSeen) : '—';
//   const lastSeenAbsolute = () =>
//     props.host.lastSeen ? formatAbsoluteTime(props.host.lastSeen) : '—';
//
//   return (
//     <tr class="bg-gray-50 dark:bg-gray-800/60">
//       <td
//         colSpan={props.colspan}
//         class="py-1 pr-2 pl-3 text-[11px] sm:text-xs font-medium text-gray-700 dark:text-gray-200"
//       >
//         <div class="flex flex-wrap items-center justify-between gap-3">
//           <div class="flex flex-wrap items-center gap-2 sm:gap-3">
//             <button
//               type="button"
//               class={`text-sm font-semibold text-gray-900 dark:text-gray-100 hover:text-blue-600 dark:hover:text-blue-400 transition-colors ${
//                 props.selected ? 'underline' : ''
//               }`}
//               onClick={() => props.onToggle(props.host.id)}
//             >
//               {props.host.displayName}
//             </button>
//             <Show when={props.host.displayName !== props.host.hostname}>
//               <span class="text-xs text-gray-500 dark:text-gray-400">({props.host.hostname})</span>
//             </Show>
//             {renderDockerStatusBadge(props.host.status)}
//           </div>
//           <div class="flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
//             <span>
//               Last update {lastSeenRelative()} ({lastSeenAbsolute()})
//             </span>
//             <Show when={props.host.agentVersion}>
//               <span
//                 class={
//                   props.host.agentVersion?.includes('dev') || props.host.agentVersion?.startsWith('0.1')
//                     ? 'px-2 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400 font-medium'
//                     : 'px-2 py-0.5 rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 font-medium'
//                 }
//                 title={
//                   props.host.agentVersion?.includes('dev') || props.host.agentVersion?.startsWith('0.1')
//                     ? 'Outdated - Update recommended for auto-update feature'
//                     : 'Up to date - Auto-update enabled'
//                 }
//               >
//                 v{props.host.agentVersion}
//               </span>
//             </Show>
//             <Show when={props.host.intervalSeconds}>
//               <span>{props.host.intervalSeconds}s interval</span>
//             </Show>
//           </div>
//         </div>
//       </td>
//     </tr>
//   );
// };

const DockerContainerRow: Component<{
  entry: ContainerEntry;
  onHostSelect: (hostId: string) => void;
  activeAlerts?: Record<string, Alert>;
}> = (props) => {
  const { container, host } = props.entry;
  const containerId = createMemo(() => buildContainerId(container, host.id));
  const [drawerOpen, setDrawerOpen] = createSignal(drawerState.get(containerId()) ?? false);

  // Get alert styles for this container
  const containerResourceId = createMemo(() => `docker:${host.id}/${container.id}`);
  const defaultAlertStyles = {
    hasUnacknowledgedAlert: false,
    hasAcknowledgedOnlyAlert: false,
    severity: null as 'critical' | 'warning' | null,
    hasAlert: false,
    alertCount: 0,
    unacknowledgedCount: 0,
    acknowledgedCount: 0,
    rowClass: '',
    indicatorClass: '',
    badgeClass: '',
  };

  const alertStyles = createMemo(() => {
    if (!props.activeAlerts) return defaultAlertStyles;
    try {
      // Convert Store to plain object if needed
      const alertsObj = typeof props.activeAlerts === 'object' ? { ...props.activeAlerts } : props.activeAlerts;
      return getAlertStyles(containerResourceId(), alertsObj) || defaultAlertStyles;
    } catch (e) {
      console.warn('Error getting alert styles for container:', containerResourceId(), e);
      return defaultAlertStyles;
    }
  });

  const cpuPercent = Math.max(0, Math.min(100, container.cpuPercent ?? 0));
  const memoryPercent = Math.max(0, Math.min(100, container.memoryPercent ?? 0));
  const memoryLabel = (() => {
    if (!container.memoryUsageBytes && !container.memoryLimitBytes) return undefined;
    if (!container.memoryLimitBytes) return `${formatBytes(container.memoryUsageBytes || 0)} used`;
    return `${formatBytes(container.memoryUsageBytes || 0)}/${formatBytes(container.memoryLimitBytes)}`;
  })();

  const uptime = () => (container.uptimeSeconds ? formatUptime(container.uptimeSeconds) : '—');
  // const startedAt = () => (container.startedAt ? formatAbsoluteTime(container.startedAt) : null);
  const isRunning = () => (container.state?.toLowerCase() === 'running');

  // Check if we have additional info to show in drawer
  const hasDrawerContent = createMemo(() => {
    return (
      (container.ports && container.ports.length > 0) ||
      (container.networks && container.networks.length > 0) ||
      container.createdAt ||
      container.image ||
      (isRunning() && container.uptimeSeconds)
    );
  });

  // const handleHostClick = (event: MouseEvent) => {
  //   event.preventDefault();
  //   props.onHostSelect(host.id);
  // };

  const toggleDrawer = (event: MouseEvent) => {
    if (!hasDrawerContent()) return;
    const target = event.target as HTMLElement;
    if (target.closest('a, button, [data-prevent-toggle]')) {
      return;
    }
    setDrawerOpen((prev) => !prev);
  };

  // Sync drawer state
  createEffect(on(containerId, (id) => {
    const stored = drawerState.get(id);
    if (stored !== undefined) {
      setDrawerOpen(stored);
    } else {
      setDrawerOpen(false);
    }
  }));

  createEffect(() => {
    drawerState.set(containerId(), drawerOpen());
  });

  // Match GuestRow styling with alert highlighting
  const showAlertHighlight = createMemo(() => alertStyles()?.hasUnacknowledgedAlert ?? false);
  const alertAccentColor = createMemo(() => {
    if (!showAlertHighlight()) return undefined;
    const severity = alertStyles()?.severity;
    return severity === 'critical' ? '#ef4444' : '#eab308';
  });

  const rowClass = () => {
    const base = 'transition-all duration-200 relative';
    const hover = 'hover:shadow-sm';
    const severity = alertStyles()?.severity;
    const alertBg = showAlertHighlight()
      ? severity === 'critical'
        ? 'bg-red-50 dark:bg-red-950/30'
        : 'bg-yellow-50 dark:bg-yellow-950/20'
      : '';
    const defaultHover = showAlertHighlight() ? '' : 'hover:bg-gray-50 dark:hover:bg-gray-700/30';
    const stoppedDimming = !isRunning() ? 'opacity-60' : '';
    const clickable = hasDrawerContent() ? 'cursor-pointer' : '';
    const expanded = drawerOpen() && !showAlertHighlight() ? 'bg-gray-50 dark:bg-gray-800/40' : '';
    return `${base} ${hover} ${defaultHover} ${alertBg} ${stoppedDimming} ${clickable} ${expanded}`;
  };

  const rowStyle = createMemo(() => {
    if (!showAlertHighlight()) return {};
    const color = alertAccentColor();
    if (!color) return {};
    return {
      'box-shadow': `inset 4px 0 0 0 ${color}`,
    };
  });

  return (
    <>
      <tr class={rowClass()} style={rowStyle()} onClick={toggleDrawer} aria-expanded={drawerOpen()}>
        {/* Container Name */}
        <td class="py-0.5 pr-2 pl-4 relative overflow-hidden">
          <div class="flex items-center gap-2 overflow-hidden">
            <span class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate" title={container.name || container.id.slice(0, 12)}>
              {container.name || container.id.slice(0, 12)}
            </span>
          </div>
        </td>

        {/* Status */}
        <td class="py-0.5 px-2 overflow-hidden">
          <div class="flex h-[24px] items-center gap-1 overflow-hidden">
            {renderDockerStatusBadge(container.state || container.status)}
            <Show when={container.health}>
              <span class="text-xs text-gray-500 dark:text-gray-400 truncate">({container.health})</span>
            </Show>
          </div>
        </td>

        {/* CPU */}
        <td class="py-0.5 px-2 overflow-hidden">
          <Show when={isRunning() && container.cpuPercent !== undefined} fallback={<span class="text-sm text-gray-400">-</span>}>
            <MetricBar value={cpuPercent} label={`${cpuPercent.toFixed(0)}%`} type="cpu" />
          </Show>
        </td>

        {/* Memory */}
        <td class="py-0.5 px-2 overflow-hidden">
          <Show when={isRunning() && container.memoryPercent !== undefined} fallback={<span class="text-sm text-gray-400">-</span>}>
            <MetricBar value={memoryPercent} label={`${memoryPercent.toFixed(0)}%`} sublabel={memoryLabel} type="memory" />
          </Show>
        </td>

        {/* Restarts */}
        <td class="py-0.5 px-2 text-sm text-gray-600 dark:text-gray-400 align-middle overflow-hidden">
          <span class="truncate">
            {container.restartCount ?? 0}
            <Show when={container.exitCode !== 0 && container.exitCode !== undefined}>
              <span class="text-xs text-gray-500 dark:text-gray-400"> (Exit {container.exitCode})</span>
            </Show>
          </span>
        </td>
      </tr>

      {/* Drawer - Additional Info */}
      <Show when={drawerOpen() && hasDrawerContent()}>
        <tr
          class={`text-[11px] ${
            isRunning()
              ? 'bg-gray-50/60 text-gray-600 dark:bg-gray-800/40 dark:text-gray-300'
              : 'bg-gray-100/70 text-gray-400 dark:bg-gray-900/30 dark:text-gray-500'
          }`}
        >
          <td class="px-4 py-2" colSpan={5}>
            <div class="grid w-full gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {/* Network & IPs */}
              <Show when={container.networks && container.networks.length > 0}>
                <div class="rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                  <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">Network</div>
                  <div class="mt-1 space-y-1">
                    <For each={container.networks}>
                      {(network) => (
                        <div class="flex flex-wrap items-center gap-1 text-gray-600 dark:text-gray-300">
                          <span class="font-medium">{network.name}</span>
                          <Show when={network.ipv4 || network.ipv6}>
                            <span class="text-gray-400 dark:text-gray-500">•</span>
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
                          </Show>
                        </div>
                      )}
                    </For>
                  </div>
                </div>
              </Show>

              {/* Ports */}
              <Show when={container.ports && container.ports.length > 0}>
                <div class="rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                  <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">Ports</div>
                  <div class="mt-1 flex flex-wrap gap-1">
                    <For each={container.ports}>
                      {(port) => (
                        <Show when={port.publicPort} fallback={
                          <span class="text-xs text-gray-600 dark:text-gray-300">{port.privatePort}/{port.protocol}</span>
                        }>
                          <span class="rounded bg-blue-100 px-1.5 py-0.5 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                            {port.publicPort}→{port.privatePort}
                          </span>
                        </Show>
                      )}
                    </For>
                  </div>
                </div>
              </Show>

              {/* Container Info */}
              <div class="rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">Info</div>
                <div class="mt-1 space-y-1 text-gray-600 dark:text-gray-300">
                  <Show when={container.image}>
                    <div class="flex items-start gap-1">
                      <span class="text-gray-500 dark:text-gray-400 min-w-[50px]">Image:</span>
                      <span class="break-all">{container.image}</span>
                    </div>
                  </Show>
                  <Show when={isRunning()}>
                    <div class="flex items-start gap-1">
                      <span class="text-gray-500 dark:text-gray-400 min-w-[50px]">Uptime:</span>
                      <span>{uptime()}</span>
                    </div>
                  </Show>
                  <Show when={container.createdAt}>
                    <div class="flex items-start gap-1">
                      <span class="text-gray-500 dark:text-gray-400 min-w-[50px]">Created:</span>
                      <span>{formatRelativeTime(container.createdAt!)}</span>
                    </div>
                  </Show>
                  <Show when={container.startedAt}>
                    <div class="flex items-start gap-1">
                      <span class="text-gray-500 dark:text-gray-400 min-w-[50px]">Started:</span>
                      <span>{formatRelativeTime(container.startedAt!)}</span>
                    </div>
                  </Show>
                </div>
              </div>
            </div>
          </td>
        </tr>
        </Show>
    </>
  );
};

export const DockerHosts: Component<DockerHostsProps> = (props) => {
  const { initialDataReceived, reconnecting, connected } = useWebSocket();
  const isLoading = createMemo(() => {
    if (typeof initialDataReceived === 'function') {
      const hostCount = Array.isArray(props.hosts) ? props.hosts.length : 0;
      return !initialDataReceived() && hostCount === 0;
    }
    return false;
  });
  const sortedHosts = createMemo(() => {
    const hosts = props.hosts || [];
    return [...hosts].sort((a, b) => a.displayName.localeCompare(b.displayName));
  });

  const [selectedHostId, setSelectedHostId] = createSignal<string | null>(null);
  const [search, setSearch] = createSignal('');

  // Unused - kept for potential future use
  // const hostSummaries = createMemo<DockerHostSummary[]>(() => {
  //   return sortedHosts().map((host) => {
  //     const containers = host.containers || [];
  //     const runningCount = containers.filter((ct) => ct.state?.toLowerCase() === 'running').length;
  //     const totalCount = containers.length;
  //
  //     const cpuUsage = containers.reduce((acc, ct) => acc + (ct.cpuPercent || 0), 0);
  //     const cpuPercent = Number.isFinite(cpuUsage)
  //       ? Math.min(100, Math.max(0, Number(cpuUsage.toFixed(1))))
  //       : 0;
  //
  //     const memoryUsed = containers.reduce((acc, ct) => acc + (ct.memoryUsageBytes || 0), 0);
  //     const memoryPercent = host.totalMemoryBytes
  //       ? Math.min(100, Math.max(0, Number(((memoryUsed / host.totalMemoryBytes) * 100).toFixed(1))))
  //       : 0;
  //
  //     const runningPercent = totalCount > 0
  //       ? Math.min(100, Math.max(0, Number(((runningCount / totalCount) * 100).toFixed(1))))
  //       : 0;
  //
  //     const memoryLabel = host.totalMemoryBytes
  //       ? `${formatBytes(memoryUsed)} / ${formatBytes(host.totalMemoryBytes)}`
  //       : undefined;
  //
  //     return {
  //       host,
  //       cpuPercent,
  //       memoryPercent,
  //       memoryLabel,
  //       runningPercent,
  //       runningCount,
  //       totalCount,
  //       uptimeSeconds: host.uptimeSeconds,
  //       lastSeenRelative: host.lastSeen ? formatRelativeTime(host.lastSeen) : '—',
  //       lastSeenAbsolute: host.lastSeen ? formatAbsoluteTime(host.lastSeen) : '—',
  //     } satisfies DockerHostSummary;
  //   });
  // });

  const searchTerms = createMemo(() =>
    search()
      .toLowerCase()
      .split(/[\s,]+/)
      .map((term) => term.trim())
      .filter(Boolean),
  );

  const matchesTerm = (host: DockerHost, container: DockerContainer, term: string) => {
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
      host.status,
    ];

    if (container.labels) {
      Object.entries(container.labels).forEach(([key, val]) => {
        tokens.push(`${key}:${val}`);
      });
    }

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
        case 'label':
          if (!container.labels) return false;
          return Object.entries(container.labels).some(([key, val]) =>
            `${key}:${val}`.toLowerCase().includes(target),
          );
        default:
          return hasToken(tokens);
      }
    }

    return hasToken(tokens);
  };

  const matchesSearch = (host: DockerHost, container: DockerContainer) => {
    const terms = searchTerms();
    if (terms.length === 0) return true;
    return terms.every((term) => matchesTerm(host, container, term));
  };

  const groupedContainers = createMemo(() => {
    const selectedHost = selectedHostId();
    const groups = new Map<string, ContainerEntry[]>();

    sortedHosts().forEach((host) => {
      if (selectedHost && host.id !== selectedHost) {
        return;
      }

      (host.containers || []).forEach((container) => {
        if (!matchesSearch(host, container)) return;

        const entry = { host, container };
        const existing = groups.get(host.id);
        if (existing) {
          existing.push(entry);
        } else {
          groups.set(host.id, [entry]);
        }
      });
    });

    return sortedHosts()
      .filter((host) => !selectedHost || host.id === selectedHost)
      .map((host) => ({ host, containers: groups.get(host.id) ?? [] }))
      .filter((group) => group.containers.length > 0);
  });

  const hasContainers = createMemo(() => {
    const groups = groupedContainers();
    return groups && groups.length > 0 && groups.some(g => g.containers.length > 0);
  });

  const toggleHostSelection = (hostId: string) => {
    setSelectedHostId((current) => (current === hostId ? null : hostId));
  };

  const activeHostName = createMemo(() => {
    const id = selectedHostId();
    if (!id) return undefined;
    const host = sortedHosts().find((item) => item.id === id);
    return host?.displayName;
  });

  // Get containers for selected host
  const selectedHostContainers = createMemo(() => {
    const hostId = selectedHostId();
    if (!hostId) return [];

    const host = sortedHosts().find(h => h.id === hostId);
    if (!host) return [];

    return (host.containers || [])
      .filter(container => matchesSearch(host, container))
      .map(container => ({ host, container }));
  });

  const selectedHost = createMemo(() => {
    const hostId = selectedHostId();
    return hostId ? sortedHosts().find(h => h.id === hostId) : null;
  });

  return (
    <div class="space-y-0">
      <Show when={isLoading()}>
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 animate-spin text-blue-500"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
            }
            title={reconnecting() ? 'Reconnecting to Docker agents...' : 'Loading Docker data...'}
            description={
              reconnecting()
                ? 'Re-establishing metrics from the monitoring service.'
                : connected()
                  ? 'Waiting for the first Docker update.'
                  : 'Connecting to the monitoring service.'
            }
          />
        </Card>
      </Show>

      <Show when={!isLoading()}>
        <Show
          when={sortedHosts().length === 0}
          fallback={
            <>
              {/* Filters */}
              <DockerFilter
                search={search}
                setSearch={setSearch}
                activeHostName={activeHostName()}
                onClearHost={() => setSelectedHostId(null)}
                onReset={() => setSelectedHostId(null)}
              />

              {/* Master-Detail Layout */}
              <div class="flex gap-4">
                {/* Left: Host List */}
                <Card padding="none" class="w-80 flex-shrink-0 overflow-hidden">
                  <div class="bg-gray-50 dark:bg-gray-800 px-4 py-2 border-b border-gray-200 dark:border-gray-700">
                    <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Docker Hosts</h3>
                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{sortedHosts().length} {sortedHosts().length === 1 ? 'host' : 'hosts'}</p>
                  </div>
                  <div class="divide-y divide-gray-200 dark:divide-gray-700">
                    <For each={sortedHosts()}>
                      {(host) => {
                        const isSelected = () => selectedHostId() === host.id;
                        const containerCount = (host.containers || []).length;
                        const runningCount = (host.containers || []).filter(c => c.state?.toLowerCase() === 'running').length;

                        return (
                          <button
                            type="button"
                            onClick={() => toggleHostSelection(host.id)}
                            class={`w-full text-left px-4 py-3 transition-colors ${
                              isSelected()
                                ? 'bg-blue-100 dark:bg-blue-900/40'
                                : 'hover:bg-blue-50 dark:hover:bg-blue-900/20'
                            }`}
                          >
                            <div class="flex items-center justify-between mb-1">
                              <div class="flex items-center gap-2">
                                <span class={`text-sm font-medium ${isSelected() ? 'text-blue-900 dark:text-blue-100' : 'text-gray-900 dark:text-gray-100'}`}>
                                  {host.displayName}
                                </span>
                                {renderDockerStatusBadge(host.status)}
                              </div>
                            </div>
                            <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                              <span>{runningCount}/{containerCount} running</span>
                              <Show when={host.lastSeen}>
                                <span>{formatRelativeTime(host.lastSeen!)}</span>
                              </Show>
                            </div>
                          </button>
                        );
                      }}
                    </For>
                  </div>
                </Card>

                {/* Right: Container List */}
                <div class="flex-1 min-w-0">
                  <Show
                    when={selectedHost()}
                    fallback={
                      <Show
                        when={hasContainers()}
                        fallback={
                          <Card padding="lg">
                            <EmptyState
                              title="No containers found"
                              description={
                                search().trim()
                                  ? 'No containers match your search.'
                                  : 'No containers on any host'
                              }
                            />
                          </Card>
                        }
                      >
                        <Card padding="none" class="overflow-hidden">
                          <ScrollableTable>
                            <table class="w-full border-collapse table-fixed">
                              <colgroup>
                                <col style="width: 25%" />
                                <col style="width: 15%" />
                                <col style="width: 20%" />
                                <col style="width: 20%" />
                                <col style="width: 20%" />
                              </colgroup>
                              <thead>
                                <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                                  <th class="pl-4 pr-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                    Container
                                  </th>
                                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                    Status
                                  </th>
                                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                    CPU
                                  </th>
                                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                    Memory
                                  </th>
                                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                    Restarts
                                  </th>
                                </tr>
                              </thead>
                              <tbody>
                                <For each={groupedContainers()}>
                                  {(group) => (
                                    <>
                                      {/* Host Section Header */}
                                      <tr class="bg-gray-50 dark:bg-gray-900/40">
                                        <td colspan="5" class="px-4 py-2 border-t-2 border-b border-gray-200 dark:border-gray-700">
                                          <div class="flex items-center justify-between">
                                            <div class="flex items-center gap-3">
                                              <h3 class="text-sm font-bold text-gray-900 dark:text-gray-100">{group.host.displayName}</h3>
                                              <Show when={group.host.displayName !== group.host.hostname}>
                                                <span class="text-xs text-gray-500 dark:text-gray-400">({group.host.hostname})</span>
                                              </Show>
                                              {renderDockerStatusBadge(group.host.status)}
                                              <span class="text-xs text-gray-600 dark:text-gray-400">
                                                {group.containers.length} {group.containers.length === 1 ? 'container' : 'containers'}
                                              </span>
                                            </div>
                                            <div class="flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
                                              <Show when={group.host.lastSeen}>
                                                <span>Updated {formatRelativeTime(group.host.lastSeen!)}</span>
                                              </Show>
                                              <Show when={group.host.agentVersion}>
                                                <span>Agent {group.host.agentVersion}</span>
                                              </Show>
                                            </div>
                                          </div>
                                        </td>
                                      </tr>
                                      {/* Host Containers */}
                                      <For each={group.containers}>
                                        {(entry) => <DockerContainerRow entry={entry} onHostSelect={toggleHostSelection} activeAlerts={props.activeAlerts} />}
                                      </For>
                                    </>
                                  )}
                                </For>
                              </tbody>
                            </table>
                          </ScrollableTable>
                        </Card>
                      </Show>
                    }
                  >
                    {(host) => (
                      <Show
                        when={selectedHostContainers().length > 0}
                        fallback={
                          <Card padding="lg">
                            <EmptyState
                              title="No containers found"
                              description={
                                search().trim()
                                  ? 'No containers match your search.'
                                  : `No containers on ${host().displayName}`
                              }
                            />
                          </Card>
                        }
                      >
                        <Card padding="none" class="overflow-hidden">
                          {/* Host Info Header */}
                          <div class="bg-gray-50 dark:bg-gray-900/40 border-b-2 border-gray-200 dark:border-gray-700 px-4 py-3">
                            <div class="flex items-center justify-between">
                              <div class="flex items-center gap-3">
                                <h3 class="text-base font-bold text-gray-900 dark:text-gray-100">{host().displayName}</h3>
                                <Show when={host().displayName !== host().hostname}>
                                  <span class="text-sm text-gray-500 dark:text-gray-400">({host().hostname})</span>
                                </Show>
                                {renderDockerStatusBadge(host().status)}
                                <span class="text-sm text-gray-600 dark:text-gray-400">
                                  {selectedHostContainers().length} {selectedHostContainers().length === 1 ? 'container' : 'containers'}
                                </span>
                              </div>
                              <div class="flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
                                <Show when={host().lastSeen}>
                                  <span>Updated {formatRelativeTime(host().lastSeen!)}</span>
                                </Show>
                                <Show when={host().agentVersion}>
                                  <span>Agent {host().agentVersion}</span>
                                </Show>
                              </div>
                            </div>
                          </div>

                          {/* Containers Table */}
                          <ScrollableTable>
                            <table class="w-full border-collapse table-fixed">
                              <colgroup>
                                <col style="width: 25%" />
                                <col style="width: 15%" />
                                <col style="width: 20%" />
                                <col style="width: 20%" />
                                <col style="width: 20%" />
                              </colgroup>
                              <thead>
                                <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                                  <th class="pl-4 pr-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                    Container
                                  </th>
                                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                    Status
                                  </th>
                                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                    CPU
                                  </th>
                                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                    Memory
                                  </th>
                                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                                    Restarts
                                  </th>
                                </tr>
                              </thead>
                              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                                <For each={selectedHostContainers()}>
                                  {(entry) => (
                                    <DockerContainerRow
                                      entry={entry}
                                      onHostSelect={toggleHostSelection}
                                      activeAlerts={props.activeAlerts}
                                    />
                                  )}
                                </For>
                              </tbody>
                            </table>
                          </ScrollableTable>
                        </Card>
                      </Show>
                    )}
                  </Show>
                </div>
              </div>
            </>
          }
        >
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                />
              </svg>
            }
            title="No Docker hosts configured"
            description={
              <span>
                Deploy the Pulse Docker agent on at least one Docker host to light up this tab. As soon as an agent reports in, container metrics appear automatically.
              </span>
            }
            actions={
              <>
                <CopyButton
                  text={`docker run -d \ \
  --name pulse-docker-agent \ \
  -e PULSE_URL="http://<pulse-server>:8080" \ \
  -e PULSE_TOKEN="<your-api-token>" \ \
  -v /var/run/docker.sock:/var/run/docker.sock \ \
  ghcr.io/rcourtman/pulse-docker-agent:latest`}
                  class="w-full sm:w-auto"
                >
                  Copy install command
                </CopyButton>
                <a
                  href="https://github.com/rcourtman/Pulse/blob/main/docs/DOCKER_MONITORING.md"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-sm text-blue-600 dark:text-blue-400 hover:underline"
                >
                  Read the Docker monitoring guide
                </a>
              </>
            }
          />
        </Card>
        </Show>
      </Show>
    </div>
  );
};
