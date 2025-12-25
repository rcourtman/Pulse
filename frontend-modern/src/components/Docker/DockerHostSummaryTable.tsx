import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import { StackedContainerBar } from './StackedContainerBar';
import type { DockerHost } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { renderDockerStatusBadge } from './DockerStatusBadge';
import { resolveHostRuntime } from './runtimeDisplay';
import { formatUptime } from '@/utils/format';
import { ScrollableTable } from '@/components/shared/ScrollableTable';
import { buildMetricKey } from '@/utils/metricsKeys';
import { StatusDot } from '@/components/shared/StatusDot';
import { getDockerHostStatusIndicator } from '@/utils/status';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { ResponsiveMetricCell, MetricText } from '@/components/shared/responsive';
import { EnhancedCPUBar } from '@/components/Dashboard/EnhancedCPUBar';
import { isAgentOutdated, getAgentVersionTooltip } from '@/utils/agentVersion';
import { DockerHostMetadataAPI, type DockerHostMetadata } from '@/api/dockerHostMetadata';
import { UrlEditPopover, createUrlEditState } from '@/components/shared/UrlEditPopover';
import { showSuccess, showError } from '@/utils/toast';
import { logger } from '@/utils/logger';

export interface DockerHostSummary {
  host: DockerHost;
  cpuPercent: number;
  memoryPercent: number;
  memoryLabel?: string;
  diskPercent: number;
  diskLabel?: string;
  runningPercent: number;
  runningCount: number;
  stoppedCount: number;
  errorCount: number;
  totalCount: number;
  uptimeSeconds: number;
  lastSeenRelative: string;
  lastSeenAbsolute: string;
}

interface DockerHostSummaryTableProps {
  summaries: () => DockerHostSummary[];
  selectedHostId: () => string | null;
  onSelect: (hostId: string) => void;
  dockerHostMetadata?: Record<string, DockerHostMetadata>;
  onHostCustomUrlUpdate?: (hostId: string, url: string) => void;
}

type SortKey = 'name' | 'uptime' | 'cpu' | 'memory' | 'disk' | 'running' | 'lastSeen' | 'agent';

type SortDirection = 'asc' | 'desc';

const isHostOnline = (host: DockerHost) => {
  const status = host.status?.toLowerCase() ?? '';
  return status === 'online' || status === 'running' || status === 'healthy' || status === 'degraded';
};

const getDisplayName = (host: DockerHost) => {
  return host.customDisplayName || host.displayName || host.hostname || host.id;
};

export const DockerHostSummaryTable: Component<DockerHostSummaryTableProps> = (props) => {
  const [sortKey, setSortKey] = createSignal<SortKey>('name');
  const [sortDirection, setSortDirection] = createSignal<SortDirection>('asc');
  const { isMobile } = useBreakpoint();

  // URL editing state using shared hook
  const urlEdit = createUrlEditState();

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
          value = getDisplayName(hostA).localeCompare(getDisplayName(hostB));
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
        value = getDisplayName(hostA).localeCompare(getDisplayName(hostB));
      }

      return dir === 'asc' ? value : -value;
    });

    return list;
  });

  // URL editing functions
  const getHostCustomUrl = (hostId: string) => {
    return props.dockerHostMetadata?.[hostId]?.customUrl;
  };

  const handleStartEditingUrl = (hostId: string, event: MouseEvent) => {
    const currentUrl = getHostCustomUrl(hostId) || '';
    urlEdit.startEditing(hostId, currentUrl, event);
  };

  const handleSaveUrl = async () => {
    const hostId = urlEdit.editingId();
    if (!hostId) return;

    const newUrl = urlEdit.editingValue().trim();
    urlEdit.setIsSaving(true);

    try {
      await DockerHostMetadataAPI.updateMetadata(hostId, { customUrl: newUrl });

      if (props.onHostCustomUrlUpdate) {
        props.onHostCustomUrlUpdate(hostId, newUrl);
      }

      if (newUrl) {
        showSuccess('Host URL saved');
      } else {
        showSuccess('Host URL cleared');
      }

      urlEdit.finishEditing();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to save host URL';
      logger.error('Failed to save host URL:', err);
      showError(message);
      urlEdit.setIsSaving(false);
    }
  };

  const handleDeleteUrl = async () => {
    const hostId = urlEdit.editingId();
    if (!hostId) return;

    urlEdit.setIsSaving(true);

    try {
      await DockerHostMetadataAPI.updateMetadata(hostId, { customUrl: '' });

      if (props.onHostCustomUrlUpdate) {
        props.onHostCustomUrlUpdate(hostId, '');
      }

      showSuccess('Host URL removed');
      urlEdit.finishEditing();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to remove host URL';
      logger.error('Failed to remove host URL:', err);
      showError(message);
      urlEdit.setIsSaving(false);
    }
  };

  const renderSortIndicator = (key: SortKey) => {
    if (sortKey() !== key) return null;
    return sortDirection() === 'asc' ? '▲' : '▼';
  };

  // Agent version checking is now done via the shared utility that compares against server version

  return (
    <>
      <Card padding="none" tone="glass" class={`mb-4 ${urlEdit.isEditing() ? 'overflow-visible' : 'overflow-hidden'}`}>
        <ScrollableTable persistKey="docker-host-summary" minWidth={isMobile() ? '100%' : '800px'}>
          <table class="w-full border-collapse whitespace-nowrap">
            <thead>
              <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
                <th
                  class="pl-3 pr-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500 whitespace-nowrap"
                  onClick={() => handleSort('name')}
                  onKeyDown={(e) => e.key === 'Enter' && handleSort('name')}
                  tabIndex={0}
                  role="button"
                  aria-label={`Sort by host ${sortKey() === 'name' ? (sortDirection() === 'asc' ? 'ascending' : 'descending') : ''}`}
                >
                  Host {renderSortIndicator('name')}
                </th>
                <th class="px-2 py-1 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap">
                  Status
                </th>
                <th
                  class="px-2 py-1 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap"
                  style={isMobile() ? { "min-width": "60px" } : { width: "140px", "min-width": "140px", "max-width": "140px" }}
                  onClick={() => handleSort('cpu')}
                >
                  CPU {renderSortIndicator('cpu')}
                </th>
                <th
                  class="px-2 py-1 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap"
                  style={isMobile() ? { "min-width": "60px" } : { width: "140px", "min-width": "140px", "max-width": "140px" }}
                  onClick={() => handleSort('memory')}
                >
                  Memory {renderSortIndicator('memory')}
                </th>
                <th
                  class="px-2 py-1 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap"
                  style={isMobile() ? { "min-width": "60px" } : { width: "140px", "min-width": "140px", "max-width": "140px" }}
                  onClick={() => handleSort('disk')}
                >
                  Disk {renderSortIndicator('disk')}
                </th>
                <th
                  class="px-2 py-1 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap"
                  onClick={() => handleSort('running')}
                >
                  Containers {renderSortIndicator('running')}
                </th>
                <th
                  class="px-2 py-1 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap"
                  onClick={() => handleSort('uptime')}
                >
                  Uptime {renderSortIndicator('uptime')}
                </th>
                <th
                  class="px-2 py-1 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap"
                  onClick={() => handleSort('lastSeen')}
                >
                  Last Update {renderSortIndicator('lastSeen')}
                </th>
                <th
                  class="px-2 py-1 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap"
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
                    const baseHover = 'group cursor-pointer transition-all duration-200 relative hover:bg-gray-50 dark:hover:bg-gray-700/50 hover:shadow-sm';

                    if (selected) {
                      return 'group cursor-pointer transition-all duration-200 relative bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30 hover:shadow-sm z-10';
                    }

                    let className = baseHover;

                    if (!online) {
                      className += ' opacity-60';
                    }

                    return className;
                  };

                  const agentOutdated = isAgentOutdated(summary.host.agentVersion);
                  const runtimeInfo = resolveHostRuntime(summary.host);
                  const runtimeVersion = summary.host.runtimeVersion || summary.host.dockerVersion;
                  const metricsKey = buildMetricKey('dockerHost', summary.host.id);
                  const hostStatus = createMemo(() => getDockerHostStatusIndicator(summary.host));

                  return (
                    <tr
                      class={rowClass()}
                      style={rowStyle()}
                      onClick={() => props.onSelect(summary.host.id)}
                    >
                      <td class="pr-2 py-1 pl-3 align-middle relative">
                        <div class="flex items-center gap-1.5 min-w-0">
                          <StatusDot
                            variant={hostStatus().variant}
                            title={hostStatus().label}
                            ariaLabel={hostStatus().label}
                            size="xs"
                          />
                          <span class="font-medium text-[11px] text-gray-900 dark:text-gray-100 truncate" title={getDisplayName(summary.host)}>
                            {getDisplayName(summary.host)}
                          </span>
                          {/* URL icon and link */}
                          <Show when={getHostCustomUrl(summary.host.id)}>
                            <a
                              href={getHostCustomUrl(summary.host.id)}
                              target="_blank"
                              rel="noopener noreferrer"
                              class="flex-shrink-0 text-blue-500 hover:text-blue-600 dark:text-blue-400 dark:hover:text-blue-300 transition-colors"
                              title={`Open ${getHostCustomUrl(summary.host.id)}`}
                              onClick={(e) => e.stopPropagation()}
                            >
                              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                              </svg>
                            </a>
                          </Show>
                          <button
                            type="button"
                            class="flex-shrink-0 p-0.5 rounded text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 transition-colors opacity-0 group-hover:opacity-100 focus:opacity-100"
                            classList={{ 'opacity-100': urlEdit.editingId() === summary.host.id }}
                            title={getHostCustomUrl(summary.host.id) ? 'Edit URL' : 'Add URL'}
                            onClick={(e) => handleStartEditingUrl(summary.host.id, e)}
                          >
                            <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                              <Show when={getHostCustomUrl(summary.host.id)} fallback={
                                <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
                              }>
                                <path stroke-linecap="round" stroke-linejoin="round" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                              </Show>
                            </svg>
                          </button>
                          <Show when={getDisplayName(summary.host) !== summary.host.hostname}>
                            <span class="hidden sm:inline text-[9px] text-gray-500 dark:text-gray-400 whitespace-nowrap">
                              ({summary.host.hostname})
                            </span>
                          </Show>
                          <div class="hidden xl:flex items-center gap-1.5 ml-1">
                            <span
                              class={`text-[9px] px-1 py-0 rounded font-medium whitespace-nowrap ${runtimeInfo.badgeClass}`}
                              title={runtimeInfo.raw || runtimeInfo.label}
                            >
                              {runtimeInfo.label}
                            </span>
                            <Show when={runtimeVersion}>
                              <span class="text-[9px] text-gray-500 dark:text-gray-400 whitespace-nowrap">
                                v{runtimeVersion}
                              </span>
                            </Show>
                          </div>
                        </div>
                      </td>
                      <td class="px-2 py-1 align-middle">
                        <div class="flex justify-center items-center h-full w-full whitespace-nowrap">
                          {renderDockerStatusBadge(summary.host.status)}
                        </div>
                      </td>
                      <td class="px-2 py-1 align-middle" style={isMobile() ? { "min-width": "60px" } : { width: "140px", "min-width": "140px", "max-width": "140px" }}>
                        <div class="h-4 w-full">
                          <EnhancedCPUBar
                            usage={summary.cpuPercent}
                            loadAverage={summary.host.loadAverage?.[0]}
                            cores={isMobile() ? undefined : summary.host.cpus}
                            resourceId={metricsKey}
                          />
                        </div>
                      </td>
                      <td class="px-2 py-1 align-middle" style={isMobile() ? { "min-width": "60px" } : { width: "140px", "min-width": "140px", "max-width": "140px" }}>
                        <div class="h-4">
                          <ResponsiveMetricCell
                            value={summary.memoryPercent}
                            type="memory"
                            sublabel={summary.memoryLabel}
                            resourceId={metricsKey}
                            isRunning={online}
                            showMobile={false}
                          />
                        </div>
                      </td>
                      <td class="px-2 py-1 align-middle" style={isMobile() ? { "min-width": "60px" } : { width: "140px", "min-width": "140px", "max-width": "140px" }}>
                        <div class="h-4">
                          <ResponsiveMetricCell
                            value={summary.diskPercent}
                            type="disk"
                            sublabel={summary.diskLabel}
                            resourceId={metricsKey}
                            isRunning={!!summary.diskLabel}
                            showMobile={false}
                          />
                        </div>
                      </td>
                      <td class="px-2 py-1 align-middle">
                        <div class="flex justify-center items-center h-full whitespace-nowrap w-full">
                          <Show
                            when={summary.totalCount > 0}
                            fallback={<span class="text-xs text-gray-400 dark:text-gray-500">—</span>}
                          >
                            <StackedContainerBar
                              running={summary.runningCount}
                              stopped={summary.stoppedCount}
                              error={summary.errorCount}
                              total={summary.totalCount}
                            />
                          </Show>
                        </div>
                      </td>
                      <td class="px-2 py-1 align-middle whitespace-nowrap">
                        <div class="flex justify-center items-center h-full">
                          <span class="text-xs text-gray-600 dark:text-gray-400">
                            {uptimeLabel}
                          </span>
                        </div>
                      </td>
                      <td class="px-2 py-1 align-middle">
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
                      <td class="px-2 py-1 align-middle">
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
                                title={`${getAgentVersionTooltip(summary.host.agentVersion)}${summary.host.intervalSeconds ? `\nReporting interval: ${summary.host.intervalSeconds}s` : ''}`}
                              >
                                {version()}
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

      {/* URL editing popover - using shared component */}
      <UrlEditPopover
        isOpen={urlEdit.isEditing()}
        value={urlEdit.editingValue()}
        position={urlEdit.position()}
        isSaving={urlEdit.isSaving()}
        hasExistingUrl={!!getHostCustomUrl(urlEdit.editingId() || '')}
        placeholder="https://portainer.local:9000"
        helpText="Add a URL to quickly access this host's management interface (e.g., Portainer)"
        onValueChange={urlEdit.setEditingValue}
        onSave={handleSaveUrl}
        onCancel={urlEdit.cancelEditing}
        onDelete={handleDeleteUrl}
      />
    </>
  );
};
