import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import type { DockerHost } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { renderDockerStatusBadge } from './DockerStatusBadge';
import { formatUptime } from '@/utils/format';

export interface DockerHostSummary {
  host: DockerHost;
  cpuPercent: number;
  memoryPercent: number;
  memoryLabel?: string;
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

type SortKey = 'name' | 'uptime' | 'cpu' | 'memory' | 'running' | 'lastSeen' | 'agent';

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
      <div class="overflow-x-auto">
        <table class="w-full min-w-[720px] border-collapse">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
              <th
                class="pl-3 pr-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-1/4 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500"
                onClick={() => handleSort('name')}
                onKeyDown={(e) => e.key === 'Enter' && handleSort('name')}
                tabindex="0"
                role="button"
                aria-label={`Sort by host ${sortKey() === 'name' ? (sortDirection() === 'asc' ? 'ascending' : 'descending') : ''}`}
              >
                Host {renderSortIndicator('name')}
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                Status
              </th>
              <th
                class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-32 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                onClick={() => handleSort('cpu')}
              >
                CPU {renderSortIndicator('cpu')}
              </th>
              <th
                class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-32 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                onClick={() => handleSort('memory')}
              >
                Memory {renderSortIndicator('memory')}
              </th>
              <th
                class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-24 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                onClick={() => handleSort('running')}
              >
                Containers {renderSortIndicator('running')}
              </th>
              <th
                class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-24 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                onClick={() => handleSort('uptime')}
              >
                Uptime {renderSortIndicator('uptime')}
              </th>
              <th
                class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-32 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                onClick={() => handleSort('lastSeen')}
              >
                Last Update {renderSortIndicator('lastSeen')}
              </th>
              <th
                class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-24 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
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

                return (
                  <tr
                    class={rowClass()}
                    style={rowStyle()}
                    onClick={() => props.onSelect(summary.host.id)}
                  >
                    <td class="pr-2 py-0.5 pl-3 whitespace-nowrap">
                      <div class="flex items-center gap-1">
                        <span class="font-medium text-[11px] text-gray-900 dark:text-gray-100">
                          {summary.host.displayName}
                        </span>
                        <Show when={summary.host.displayName !== summary.host.hostname}>
                          <span class="text-[9px] text-gray-500 dark:text-gray-400">
                            ({summary.host.hostname})
                          </span>
                        </Show>
                        <Show when={summary.host.dockerVersion}>
                          <span class="text-[9px] px-1 py-0 rounded text-[8px] font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                            Docker {summary.host.dockerVersion}
                          </span>
                        </Show>
                      </div>
                    </td>
                    <td class="px-2 py-0.5">
                      {renderDockerStatusBadge(summary.host.status)}
                    </td>
                    <td class="px-2 py-0.5">
                      <Show when={online} fallback={<span class="text-xs text-gray-400 dark:text-gray-500">—</span>}>
                        <MetricBar
                          value={summary.cpuPercent}
                          label={`${summary.cpuPercent.toFixed(1)}%`}
                          type="cpu"
                        />
                      </Show>
                    </td>
                    <td class="px-2 py-0.5">
                      <Show when={online} fallback={<span class="text-xs text-gray-400 dark:text-gray-500">—</span>}>
                        <MetricBar
                          value={summary.memoryPercent}
                          label={`${summary.memoryPercent.toFixed(1)}%`}
                          sublabel={summary.memoryLabel}
                          type="memory"
                        />
                      </Show>
                    </td>
                    <td class="px-2 py-0.5">
                      <div class="flex justify-center">
                        <MetricBar
                          value={summary.runningPercent}
                          label={`${summary.runningCount}/${summary.totalCount}`}
                          type="generic"
                        />
                      </div>
                    </td>
                    <td class="px-2 py-0.5 whitespace-nowrap">
                      <span class="text-xs text-gray-600 dark:text-gray-400">
                        {summary.uptimeSeconds ? formatUptime(summary.uptimeSeconds) : '—'}
                      </span>
                    </td>
                    <td class="px-2 py-0.5">
                      <div class="text-sm text-gray-900 dark:text-gray-100">
                        {summary.lastSeenRelative}
                      </div>
                      <div class="text-xs text-gray-500 dark:text-gray-400">
                        {summary.lastSeenAbsolute}
                      </div>
                    </td>
                    <td class="px-2 py-0.5">
                      <div class="flex flex-col gap-0.5">
                        <Show when={summary.host.agentVersion}>
                          <div class="flex flex-col gap-0.5">
                            <span
                              class={
                                isAgentOutdated(summary.host.agentVersion)
                                  ? 'text-[10px] px-1.5 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400 font-medium inline-block w-fit'
                                  : 'text-[10px] px-1.5 py-0.5 rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 font-medium inline-block w-fit'
                              }
                              title={
                                isAgentOutdated(summary.host.agentVersion)
                                  ? 'Outdated - Update recommended'
                                  : 'Up to date - Auto-update enabled'
                              }
                            >
                              v{summary.host.agentVersion}
                            </span>
                            <Show when={isAgentOutdated(summary.host.agentVersion)}>
                              <span class="text-[9px] text-yellow-600 dark:text-yellow-500 font-medium">
                                ⚠ Update available
                              </span>
                            </Show>
                          </div>
                        </Show>
                        <Show when={summary.host.intervalSeconds}>
                          <span class="text-[10px] text-gray-500 dark:text-gray-400">
                            {summary.host.intervalSeconds}s
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
      </div>
    </Card>
  );
};
