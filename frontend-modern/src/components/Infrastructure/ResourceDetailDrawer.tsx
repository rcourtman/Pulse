import { Component, Show, createMemo, For } from 'solid-js';
import type { Resource, ResourceMetric } from '@/types/resource';
import { getDisplayName, getCpuPercent, getMemoryPercent, getDiskPercent } from '@/types/resource';
import { formatBytes, formatUptime, formatRelativeTime, formatAbsoluteTime, formatPercent } from '@/utils/format';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { StatusDot } from '@/components/shared/StatusDot';
import { TagBadges } from '@/components/Dashboard/TagBadges';
import { buildMetricKey } from '@/utils/metricsKeys';
import { getHostStatusIndicator } from '@/utils/status';
import { getPlatformBadge, getSourceBadge, getTypeBadge, getUnifiedSourceBadges } from './resourceBadges';

interface ResourceDetailDrawerProps {
  resource: Resource;
  onClose?: () => void;
}

const metricSublabel = (metric?: ResourceMetric) => {
  if (!metric || typeof metric.used !== 'number' || typeof metric.total !== 'number') return undefined;
  return `${formatBytes(metric.used)}/${formatBytes(metric.total)}`;
};

export const ResourceDetailDrawer: Component<ResourceDetailDrawerProps> = (props) => {
  const displayName = createMemo(() => getDisplayName(props.resource));
  const statusIndicator = createMemo(() => getHostStatusIndicator({ status: props.resource.status }));
  const lastSeen = createMemo(() => formatRelativeTime(props.resource.lastSeen));
  const lastSeenAbsolute = createMemo(() => formatAbsoluteTime(props.resource.lastSeen));
  const metricKey = createMemo(() => buildMetricKey('host', props.resource.id));

  const cpuPercent = createMemo(() => (props.resource.cpu ? Math.round(getCpuPercent(props.resource)) : null));
  const memoryPercent = createMemo(() => (props.resource.memory ? Math.round(getMemoryPercent(props.resource)) : null));
  const diskPercent = createMemo(() => (props.resource.disk ? Math.round(getDiskPercent(props.resource)) : null));

  const platformBadge = createMemo(() => getPlatformBadge(props.resource.platformType));
  const sourceBadge = createMemo(() => getSourceBadge(props.resource.sourceType));
  const typeBadge = createMemo(() => getTypeBadge(props.resource.type));
  const unifiedSourceBadges = createMemo(() => {
    const platformData = props.resource.platformData as { sources?: string[] } | undefined;
    return getUnifiedSourceBadges(platformData?.sources ?? []);
  });
  const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);

  return (
    <div class="space-y-3">
      <div class="flex items-start justify-between gap-4">
        <div class="space-y-1 min-w-0">
          <div class="flex items-center gap-2">
            <StatusDot
              variant={statusIndicator().variant}
              title={statusIndicator().label}
              ariaLabel={statusIndicator().label}
              size="sm"
            />
            <div class="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate" title={displayName()}>
              {displayName()}
            </div>
          </div>
          <div class="text-[11px] text-gray-500 dark:text-gray-400 truncate" title={props.resource.id}>
            {props.resource.id}
          </div>
          <div class="flex flex-wrap gap-1.5">
            <Show when={typeBadge()}>
              {(badge) => (
                <span class={badge().classes} title={badge().title}>
                  {badge().label}
                </span>
              )}
            </Show>
            <Show
              when={hasUnifiedSources()}
              fallback={
                <>
                  <Show when={platformBadge()}>
                    {(badge) => (
                      <span class={badge().classes} title={badge().title}>
                        {badge().label}
                      </span>
                    )}
                  </Show>
                  <Show when={sourceBadge()}>
                    {(badge) => (
                      <span class={badge().classes} title={badge().title}>
                        {badge().label}
                      </span>
                    )}
                  </Show>
                </>
              }
            >
              <For each={unifiedSourceBadges()}>
                {(badge) => (
                  <span class={badge.classes} title={badge.title}>
                    {badge.label}
                  </span>
                )}
              </For>
            </Show>
          </div>
        </div>

        <Show when={props.onClose}>
          <button
            type="button"
            onClick={() => props.onClose?.()}
            class="text-xs font-medium text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
          >
            Close
          </button>
        </Show>
      </div>

      <div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
          <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Metrics</div>
          <div class="space-y-2">
            <div class="space-y-1">
              <div class="text-[10px] text-gray-500 dark:text-gray-400">CPU</div>
              <Show when={cpuPercent() !== null} fallback={<div class="text-xs text-gray-400">—</div>}>
                <MetricBar
                  value={cpuPercent() ?? 0}
                  label={formatPercent(cpuPercent() ?? 0)}
                  type="cpu"
                  resourceId={metricKey()}
                />
              </Show>
            </div>
            <div class="space-y-1">
              <div class="text-[10px] text-gray-500 dark:text-gray-400">Memory</div>
              <Show when={memoryPercent() !== null} fallback={<div class="text-xs text-gray-400">—</div>}>
                <MetricBar
                  value={memoryPercent() ?? 0}
                  label={formatPercent(memoryPercent() ?? 0)}
                  sublabel={metricSublabel(props.resource.memory)}
                  type="memory"
                  resourceId={metricKey()}
                />
              </Show>
            </div>
            <div class="space-y-1">
              <div class="text-[10px] text-gray-500 dark:text-gray-400">Disk</div>
              <Show when={diskPercent() !== null} fallback={<div class="text-xs text-gray-400">—</div>}>
                <MetricBar
                  value={diskPercent() ?? 0}
                  label={formatPercent(diskPercent() ?? 0)}
                  sublabel={metricSublabel(props.resource.disk)}
                  type="disk"
                  resourceId={metricKey()}
                />
              </Show>
            </div>
          </div>
        </div>

        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
          <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Status</div>
          <div class="space-y-1.5 text-[11px]">
            <div class="flex items-center justify-between gap-2">
              <span class="text-gray-500 dark:text-gray-400">State</span>
              <span class="font-medium text-gray-700 dark:text-gray-200 capitalize">{props.resource.status || 'unknown'}</span>
            </div>
            <Show when={props.resource.uptime}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-gray-500 dark:text-gray-400">Uptime</span>
                <span class="font-medium text-gray-700 dark:text-gray-200">{formatUptime(props.resource.uptime ?? 0)}</span>
              </div>
            </Show>
            <Show when={props.resource.lastSeen}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-gray-500 dark:text-gray-400">Last Seen</span>
                <span
                  class="font-medium text-gray-700 dark:text-gray-200"
                  title={lastSeenAbsolute()}
                >
                  {lastSeen() || '—'}
                </span>
              </div>
            </Show>
            <Show when={props.resource.platformId}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-gray-500 dark:text-gray-400">Platform ID</span>
                <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.resource.platformId}>
                  {props.resource.platformId}
                </span>
              </div>
            </Show>
            <Show when={props.resource.clusterId}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-gray-500 dark:text-gray-400">Cluster</span>
                <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.resource.clusterId}>
                  {props.resource.clusterId}
                </span>
              </div>
            </Show>
            <Show when={props.resource.parentId}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-gray-500 dark:text-gray-400">Parent</span>
                <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.resource.parentId}>
                  {props.resource.parentId}
                </span>
              </div>
            </Show>
          </div>
        </div>

        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
          <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Identity</div>
          <div class="space-y-1.5 text-[11px]">
            <Show when={props.resource.identity?.hostname}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-gray-500 dark:text-gray-400">Hostname</span>
                <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.resource.identity?.hostname}>
                  {props.resource.identity?.hostname}
                </span>
              </div>
            </Show>
            <Show when={props.resource.identity?.machineId}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-gray-500 dark:text-gray-400">Machine ID</span>
                <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.resource.identity?.machineId}>
                  {props.resource.identity?.machineId}
                </span>
              </div>
            </Show>
            <Show when={props.resource.identity?.ips && props.resource.identity.ips.length > 0}>
              <div class="flex flex-col gap-1">
                <span class="text-gray-500 dark:text-gray-400">IP Addresses</span>
                <div class="flex flex-wrap gap-1">
                  <For each={props.resource.identity?.ips ?? []}>
                    {(ip) => (
                      <span
                        class="inline-flex items-center rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
                        title={ip}
                      >
                        {ip}
                      </span>
                    )}
                  </For>
                </div>
              </div>
            </Show>
            <Show when={props.resource.tags && props.resource.tags.length > 0}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-gray-500 dark:text-gray-400">Tags</span>
                <TagBadges tags={props.resource.tags} maxVisible={6} />
              </div>
            </Show>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ResourceDetailDrawer;
