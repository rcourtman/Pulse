import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import type { DockerHost } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { renderDockerStatusBadge } from './DockerStatusBadge';
import { formatPercent, formatUptime } from '@/utils/format';
import { ScrollableTable } from '@/components/shared/ScrollableTable';

export interface DockerHostSummary {
  host: DockerHost;
  cpuPercent: number;
  memoryPercent: number;
  memoryLabel?: string;
  diskPercent: number;
  diskLabel?: string;
  runningPercent: number;
  runningCount: number;
  totalCount: number;
  uptimeSeconds: number;
  lastSeenRelative: string;
  lastSeenAbsolute: string;
}

interface DockerHostSummaryTableProps {
  summaries: () => DockerHostSummary[];
  selectedHostId: () => string | null;
  onSelect: (hostId: string) => void;
}

type SortKey = 'name' | 'uptime' | 'cpu' | 'memory' | 'disk' | 'running' | 'lastSeen' | 'agent';

type SortDirection = 'asc' | 'desc';

const isHostOnline = (host: DockerHost) => {
  const status = host.status?.toLowerCase() ?? '';
  return status === 'online' || status === 'running' || status === 'healthy';
};

export const DockerHostSummaryTable: Component<DockerHostSummaryTableProps> = (props) => {
  const [sortKey, setSortKey] = createSignal<SortKey>('name');
  const [sortDirection, setSortDirection] = createSignal<SortDirection>('asc');

  const handleSort = (key: SortKey) => {
    if (sortKey() === key) {
      setSortDirection(sortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setSortKey(key);
      setSortDirection(key === 'name' ? 'asc' : 'desc');
    }
  };

  const formatLastSeenValue = (lastSeen?: number) => {
    if (!lastSeen) return 0;
    return lastSeen;
  };

  const runningStatusClass = (summary: DockerHostSummary) => {
    if (!summary.totalCount || summary.totalCount <= 0) {
      return 'text-gray-400 dark:text-gray-500';
    }
    if (summary.runningPercent >= 99) {
      return 'text-green-600 dark:text-green-400';
    }
    if (summary.runningPercent >= 70) {
      return 'text-yellow-600 dark:text-yellow-400';
    }
    return 'text-red-600 dark:text-red-400';
  };

  const sortedSummaries = createMemo(() => {
    const list = [...props.summaries()];
    const key = sortKey();
    const dir = sortDirection();

    list.sort((a, b) => {
      const hostA = a.host;
      const hostB = b.host;

      let value = 0;
      switch (key) {
        case 'name':
          value = hostA.displayName.localeCompare(hostB.displayName);
          break;
        case 'uptime':
          value = (a.uptimeSeconds || 0) - (b.uptimeSeconds || 0);
          break;
        case 'cpu':
          value = a.cpuPercent - b.cpuPercent;
          break;
        case 'memory':
          value = a.memoryPercent - b.memoryPercent;
          break;
        case 'disk':
          value = a.diskPercent - b.diskPercent;
          break;
        case 'running':
          value = a.runningPercent - b.runningPercent;
          break;
        case 'lastSeen':
          value = formatLastSeenValue(hostA.lastSeen) - formatLastSeenValue(hostB.lastSeen);
          break;
        case 'agent':
          // Sort by version, putting outdated versions first (for easy identification)
          const aOutdated = isAgentOutdated(hostA.agentVersion);
          const bOutdated = isAgentOutdated(hostB.agentVersion);
          if (aOutdated !== bOutdated) {
            value = aOutdated ? -1 : 1;
          } else {
            value = (hostA.agentVersion || '').localeCompare(hostB.agentVersion || '');
          }
          break;
      }

      if (value === 0) {
        value = hostA.displayName.localeCompare(hostB.displayName);
      }

      return dir === 'asc' ? value : -value;
    });

    return list;
  });

  const renderSortIndicator = (key: SortKey) => {
    if (sortKey() !== key) return null;
    return sortDirection() === 'asc' ? '▲' : '▼';
  };

  const isAgentOutdated = (version?: string) => {
    if (!version) return false;
    return version.includes('dev') || version.startsWith('0.1');
  };

  return (
    <Card padding="none" class="mb-4 overflow-hidden">
      <ScrollableTable minWidth="720px" persistKey="docker-host-summary">
        <table class="w-full border-collapse sm:whitespace-nowrap">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
              <th
                class="pl-3 pr-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-1/4 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500 whitespace-nowrap"
                onClick={() => handleSort('name')}
                onKeyDown={(e) => e.key === 'Enter' && handleSort('name')}
                tabIndex={0}
                role="button"
                aria-label={`Sort by host ${sortKey() === 'name' ? (sortDirection() === 'asc' ? 'ascending' : 'descending') : ''}`}
              >
                Host {renderSortIndicator('name')}
              </th>
              <th class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap">
                Status
              </th>
              <th
                class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-[140px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap"
                onClick={() => handleSort('cpu')}
              >
                CPU {renderSortIndicator('cpu')}
              </th>
              <th
                class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-[140px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap"
                onClick={() => handleSort('memory')}
              >
                Memory {renderSortIndicator('memory')}
              </th>
              <th
                class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-[140px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap"
                onClick={() => handleSort('disk')}
              >
                Disk {renderSortIndicator('disk')}
              </th>
              <th
                class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-24 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap"
                onClick={() => handleSort('running')}
              >
                Containers {renderSortIndicator('running')}
              </th>
              <th
                class="hidden px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-24 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap sm:table-cell"
                onClick={() => handleSort('uptime')}
              >
                Uptime {renderSortIndicator('uptime')}
              </th>
              <th
                class="hidden px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-32 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap sm:table-cell"
                onClick={() => handleSort('lastSeen')}
              >
                Last Update {renderSortIndicator('lastSeen')}
              </th>
              <th
                class="hidden px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-24 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap sm:table-cell"
                onClick={() => handleSort('agent')}
              >
                Agent {renderSortIndicator('agent')}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <For each={sortedSummaries()}>
              {(summary) => {
                const selected = props.selectedHostId() === summary.host.id;
                const online = isHostOnline(summary.host);
                const uptimeLabel = summary.uptimeSeconds ? formatUptime(summary.uptimeSeconds) : '—';
                const agentLabel = summary.host.agentVersion ? summary.host.agentVersion : '—';

                const rowStyle = () => {
                  const styles: Record<string, string> = {};
                  const shadows: string[] = [];

                  if (selected) {
                    shadows.push('0 0 0 1px rgba(59, 130, 246, 0.5)');
                    shadows.push('0 2px 4px -1px rgba(0, 0, 0, 0.1)');
                  }

                  if (shadows.length > 0) {
                    styles['box-shadow'] = shadows.join(', ');
                  }
                  return styles;
                };

                const rowClass = () => {
                  const baseHover = 'cursor-pointer transition-all duration-200 relative hover:bg-gray-50 dark:hover:bg-gray-700/50 hover:shadow-sm';

                  if (selected) {
                    return 'cursor-pointer transition-all duration-200 relative bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30 hover:shadow-sm z-10';
                  }

                  let className = baseHover;

                  if (!online) {
                    className += ' opacity-60';
                  }

                  return className;
                };

                const agentOutdated = isAgentOutdated(summary.host.agentVersion);

                return (
                  <tr
                    class={rowClass()}
                    style={rowStyle()}
                    onClick={() => props.onSelect(summary.host.id)}
                  >
                    <td class="pr-2 py-1 pl-3 align-middle">
                      <div class="flex flex-wrap items-center gap-1 sm:flex-nowrap sm:whitespace-nowrap sm:min-w-0">
                        <span class="font-medium text-[11px] text-gray-900 dark:text-gray-100 sm:truncate sm:max-w-[200px]">
                          {summary.host.displayName}
                        </span>
                        <Show when={summary.host.displayName !== summary.host.hostname}>
                          <span class="hidden sm:inline text-[9px] text-gray-500 dark:text-gray-400 sm:whitespace-nowrap">
                            ({summary.host.hostname})
                          </span>
                        </Show>
                        <span class="text-[9px] px-1 py-0 rounded text-[8px] font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400 whitespace-nowrap">
                          Docker
                        </span>
                        <Show when={summary.host.dockerVersion}>
                          <span class="text-[9px] text-gray-500 dark:text-gray-400 whitespace-nowrap">
                            v{summary.host.dockerVersion}
                          </span>
                        </Show>
                      </div>
                      <div class="mt-2 grid grid-cols-1 gap-1 text-[10px] text-gray-500 dark:text-gray-400 sm:hidden">
                        <div class="flex items-center gap-1">
                          <span class="font-semibold text-gray-600 dark:text-gray-300">Uptime:</span>
                          <span>{uptimeLabel}</span>
                        </div>
                        <div class="flex items-start gap-1">
                          <span class="font-semibold text-gray-600 dark:text-gray-300">Last:</span>
                          <span class="flex-1">
                            <span>{summary.lastSeenRelative}</span>
                            <Show when={summary.lastSeenAbsolute}>
                              <span class="block text-[9px] text-gray-400 dark:text-gray-500">
                                {summary.lastSeenAbsolute}
                              </span>
                            </Show>
                          </span>
                        </div>
                        <div class="flex flex-wrap items-center gap-1">
                          <span class="font-semibold text-gray-600 dark:text-gray-300">Agent:</span>
                          <span
                            class={
                              agentOutdated
                                ? 'rounded px-1 py-0.5 bg-yellow-100 dark:bg-yellow-900/20 text-yellow-700 dark:text-yellow-400 font-medium'
                                : 'rounded px-1 py-0.5 bg-green-100 dark:bg-green-900/20 text-green-700 dark:text-green-400 font-medium'
                            }
                          >
                            {agentLabel}
                          </span>
                          <Show when={agentOutdated}>
                            <span class="text-[9px] font-medium text-yellow-600 dark:text-yellow-500">
                              Update recommended
                            </span>
                          </Show>
                        </div>
                        <Show when={summary.host.intervalSeconds}>
                          <div class="flex items-center gap-1">
                            <span class="font-semibold text-gray-600 dark:text-gray-300">Interval:</span>
                            <span>{summary.host.intervalSeconds}s</span>
                          </div>
                        </Show>
                      </div>
                    </td>
                    <td class="px-2 py-1 align-middle">
                      <div class="flex justify-center items-center h-full w-full max-w-[180px] whitespace-nowrap">
                        {renderDockerStatusBadge(summary.host.status)}
                      </div>
                    </td>
                    <td class="px-2 py-1 align-middle">
                      <div class="flex justify-center items-center h-full w-full max-w-[180px] whitespace-nowrap">
                        <Show when={online} fallback={<span class="text-xs text-gray-400 dark:text-gray-500">—</span>}>
                          <MetricBar value={summary.cpuPercent} label={formatPercent(summary.cpuPercent)} type="cpu" />
                        </Show>
                      </div>
                    </td>
                    <td class="px-2 py-1 align-middle">
                      <div class="flex justify-center items-center h-full w-full max-w-[180px] whitespace-nowrap">
                        <Show when={online} fallback={<span class="text-xs text-gray-400 dark:text-gray-500">—</span>}>
                          <MetricBar
                            value={summary.memoryPercent}
                            label={formatPercent(summary.memoryPercent)}
                            sublabel={summary.memoryLabel}
                            type="memory"
                          />
                        </Show>
                      </div>
                    </td>
                    <td class="px-2 py-1 align-middle">
                      <div class="flex justify-center items-center h-full whitespace-nowrap">
                        <Show when={summary.diskLabel} fallback={<span class="text-xs text-gray-400 dark:text-gray-500">—</span>}>
                          <MetricBar
                            value={summary.diskPercent}
                            label={formatPercent(summary.diskPercent)}
                            sublabel={summary.diskLabel}
                            type="disk"
                          />
                        </Show>
                      </div>
                    </td>
                    <td class="px-2 py-1 align-middle">
                      <div class="flex justify-center items-center h-full whitespace-nowrap">
                        <Show
                          when={summary.totalCount > 0}
                          fallback={<span class="text-xs text-gray-400 dark:text-gray-500">—</span>}
                        >
                          <span
                            class={`text-xs font-medium whitespace-nowrap ${runningStatusClass(summary)}`}
                            title={`${formatPercent(summary.runningPercent)} running`}
                          >
                            {summary.runningCount}/{summary.totalCount}
                          </span>
                        </Show>
                      </div>
                    </td>
                    <td class="hidden px-2 py-1 align-middle whitespace-nowrap sm:table-cell">
                      <div class="flex justify-center items-center h-full">
                        <span class="text-xs text-gray-600 dark:text-gray-400">
                          {uptimeLabel}
                        </span>
                      </div>
                    </td>
                    <td class="hidden px-2 py-1 align-middle sm:table-cell">
                      <div class="flex justify-center items-center h-full">
                        <Show
                          when={summary.lastSeenRelative}
                          fallback={<span class="text-xs text-gray-400 dark:text-gray-500">—</span>}
                        >
                          {(relative) => (
                            <span
                              class="inline-flex items-center gap-1 text-xs text-gray-600 dark:text-gray-400 whitespace-nowrap"
                              title={summary.lastSeenAbsolute || undefined}
                            >
                              {relative()}
                            </span>
                          )}
                        </Show>
                      </div>
                    </td>
                    <td class="hidden px-2 py-1 align-middle sm:table-cell">
                      <div class="flex items-center justify-center gap-2 whitespace-nowrap text-[10px] h-full">
                        <Show
                          when={summary.host.agentVersion}
                          fallback={<span class="text-gray-400 dark:text-gray-500 text-xs">—</span>}
                        >
                          {(version) => (
                            <span
                              class={
                                agentOutdated
                                  ? 'px-1.5 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400 font-medium'
                                  : 'px-1.5 py-0.5 rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 font-medium'
                              }
                              title={`${
                                agentOutdated
                                  ? 'Agent is outdated on this host'
                                  : 'Agent is up to date'
                              }${summary.host.intervalSeconds ? `\nReporting interval: ${summary.host.intervalSeconds}s` : ''}`}
                            >
                              v{version()}
                            </span>
                          )}
                        </Show>
                        <Show when={agentOutdated}>
                          <span class="text-[10px] text-yellow-600 dark:text-yellow-500 font-medium" title="Update recommended">
                            ⚠
                          </span>
                        </Show>
                      </div>
                    </td>
                  </tr>
                );
              }}
            </For>
          </tbody>
        </table>
      </ScrollableTable>
    </Card>
  );
};
