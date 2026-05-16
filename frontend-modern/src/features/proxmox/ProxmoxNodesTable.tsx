import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { SearchInput } from '@/components/shared/SearchInput';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { TemperatureGauge } from '@/components/shared/TemperatureGauge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { normalizeDiskArray } from '@/utils/format';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  filterPlatformResources,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Disk } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  getResourceClusterLabel,
  getResourceNodeName,
  getResourceVersion,
} from './proxmoxPageModel';

// Proxmox Overview mirrors the v5 Dashboard layout: a dedicated nodes table on
// top, the canonical Workloads filter + guest table below. The nodes table
// uses the canonical metric primitives (ResponsiveMetricCell / StackedMemoryBar
// / StackedDiskBar / TemperatureGauge) so the bars, severity coloring, and
// sparkline overlays match the rest of the platform-first surfaces.

const STATUS_FILTER_OPTIONS: FilterOption<PlatformResourceStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'online', label: 'Healthy' },
  { value: 'degraded', label: 'Degraded' },
  { value: 'offline', label: 'Offline' },
];

const formatUptime = (seconds: number | undefined): { label: string; warn: boolean } => {
  if (!seconds || seconds <= 0) return { label: '—', warn: false };
  const warn = seconds < 3_600; // <1h matches v5 "recently restarted" highlight
  const days = Math.floor(seconds / 86_400);
  if (days > 0) return { label: `${days}d`, warn };
  const hours = Math.floor(seconds / 3_600);
  if (hours > 0) return { label: `${hours}h`, warn };
  const mins = Math.floor(seconds / 60);
  return { label: `${mins}m`, warn };
};

type GuestCounts = { vms: number; containers: number };

const countGuestsForNode = (guests: Resource[], nodeName: string): GuestCounts => {
  const counts: GuestCounts = { vms: 0, containers: 0 };
  for (const guest of guests) {
    if (getResourceNodeName(guest) !== nodeName) continue;
    if (guest.type === 'vm') counts.vms += 1;
    else if (guest.type === 'system-container' || guest.type === 'oci-container') {
      counts.containers += 1;
    }
  }
  return counts;
};

const VMS_BADGE =
  'inline-flex min-w-[2rem] justify-center items-center rounded-md bg-sky-100 px-1.5 py-0.5 text-[11px] font-semibold tabular-nums text-sky-700 dark:bg-sky-900/40 dark:text-sky-300';
const CTS_BADGE =
  'inline-flex min-w-[2rem] justify-center items-center rounded-md bg-violet-100 px-1.5 py-0.5 text-[11px] font-semibold tabular-nums text-violet-700 dark:bg-violet-900/40 dark:text-violet-300';
const ZERO_BADGE =
  'inline-flex min-w-[2rem] justify-center items-center rounded-md bg-surface-alt px-1.5 py-0.5 text-[11px] font-medium tabular-nums text-muted';

export const ProxmoxNodesTable: Component<{
  nodes: Resource[];
  guests: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<PlatformResourceStatusFilter>('all');

  const filtered = createMemo(() => filterPlatformResources(props.nodes, search(), status()));
  const visible = createMemo(() => filtered().length);
  const total = createMemo(() => props.nodes.length);

  return (
    <Show
      when={props.nodes.length > 0}
      fallback={
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title={props.emptyTitle}
            description={props.emptyDescription}
          />
        </Card>
      }
    >
      <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2">
          <div class="min-w-[200px] flex-1 sm:max-w-xs">
            <SearchInput value={search} onChange={setSearch} placeholder="Search Proxmox nodes" />
          </div>
          <FilterButtonGroup
            options={STATUS_FILTER_OPTIONS}
            value={status()}
            onChange={setStatus}
          />
          <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
            <Show when={visible() !== total()} fallback={<>{total()} nodes</>}>
              {visible()} of {total()} nodes
            </Show>
          </span>
        </div>

        <Show
          when={filtered().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={props.emptyIcon}
                title="No nodes match current filters"
                description="Adjust the search or status filter to see more nodes."
              />
            </Card>
          }
        >
          <Card padding="none" tone="card" class="overflow-hidden">
            <Table class="w-full min-w-[1080px] border-collapse text-xs">
              <TableHeader class="bg-surface-alt text-muted border-b border-border">
                <TableRow class="text-left text-[10px] uppercase tracking-wide">
                  <TableHead class="px-3 py-2 font-medium">Node</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Version</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Uptime</TableHead>
                  <TableHead class="px-3 py-2 font-medium" style={{ width: '180px' }}>CPU</TableHead>
                  <TableHead class="px-3 py-2 font-medium" style={{ width: '180px' }}>Memory</TableHead>
                  <TableHead class="px-3 py-2 font-medium" style={{ width: '180px' }}>Disk</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Temp</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-center">VMs</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-center">CTs</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Cluster</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border-subtle">
                <For each={filtered()}>
                  {(node) => {
                    const name = () => asTrimmedString(node.name) || node.id;
                    const version = () => asTrimmedString(getResourceVersion(node));
                    const cluster = () => getResourceClusterLabel(node);
                    const counts = () =>
                      countGuestsForNode(props.guests, getResourceNodeName(node));
                    const indicator = () => getSimpleStatusIndicator(node.status);
                    const isOnline = () => indicator().variant === 'success';
                    const uptime = () => formatUptime(node.uptime);
                    const metricsKey = () => buildMetricKeyForUnifiedResource(node);
                    const temperature = () => node.temperature;
                    const cpuPercent = () => node.cpu?.current ?? 0;
                    const memoryUsed = () => node.memory?.used ?? 0;
                    const memoryTotal = () => node.memory?.total ?? 0;
                    const memoryPercentOnly = () =>
                      !memoryTotal() && typeof node.memory?.current === 'number'
                        ? node.memory.current
                        : undefined;
                    const aggregateDisk = (): Disk | undefined =>
                      node.disk
                        ? ({
                            total: node.disk.total ?? 0,
                            used: node.disk.used ?? 0,
                            free: node.disk.free ?? 0,
                            usage: node.disk.current ?? 0,
                          } as Disk)
                        : undefined;
                    return (
                      <TableRow class="hover:bg-surface-hover">
                        <TableCell class="px-3 py-2">
                          <div class="flex items-center gap-2 min-w-0">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={node.status || 'unknown'}
                              ariaHidden
                            />
                            <span class="font-semibold text-base-content truncate" title={name()}>
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell class="px-3 py-2">
                          <Show
                            when={version()}
                            fallback={<span class="text-muted">—</span>}
                          >
                            <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 font-mono text-[10px] text-base-content">
                              {version()}
                            </span>
                          </Show>
                        </TableCell>
                        <TableCell
                          class={`px-3 py-2 text-right tabular-nums ${
                            uptime().warn
                              ? 'text-orange-600 dark:text-orange-400'
                              : 'text-base-content'
                          }`}
                        >
                          {uptime().label}
                        </TableCell>
                        <TableCell class="px-3 py-2">
                          <ResponsiveMetricCell
                            class="w-full"
                            value={cpuPercent()}
                            type="cpu"
                            resourceId={metricsKey()}
                            isRunning={isOnline()}
                            showMobile={false}
                          />
                        </TableCell>
                        <TableCell class="px-3 py-2">
                          <Show
                            when={isOnline() && (memoryTotal() > 0 || memoryPercentOnly() != null)}
                            fallback={
                              <div class="flex justify-center">
                                <span class="text-xs text-muted" aria-hidden="true">
                                  —
                                </span>
                              </div>
                            }
                          >
                            <StackedMemoryBar
                              used={memoryUsed()}
                              total={memoryTotal()}
                              percentOnly={memoryPercentOnly()}
                            />
                          </Show>
                        </TableCell>
                        <TableCell class="px-3 py-2">
                          <Show
                            when={isOnline() && (aggregateDisk() || node.agent?.disks?.length)}
                            fallback={
                              <div class="flex justify-center">
                                <span class="text-xs text-muted" aria-hidden="true">
                                  —
                                </span>
                              </div>
                            }
                          >
                            <StackedDiskBar
                              disks={normalizeDiskArray(node.agent?.disks)}
                              aggregateDisk={aggregateDisk()}
                            />
                          </Show>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right">
                          <Show
                            when={typeof temperature() === 'number' && (temperature() as number) > 0}
                            fallback={<span class="text-xs text-muted">—</span>}
                          >
                            <TemperatureGauge value={temperature() as number} />
                          </Show>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-center">
                          <span class={counts().vms > 0 ? VMS_BADGE : ZERO_BADGE}>
                            {counts().vms}
                          </span>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-center">
                          <span class={counts().containers > 0 ? CTS_BADGE : ZERO_BADGE}>
                            {counts().containers}
                          </span>
                        </TableCell>
                        <TableCell class="px-3 py-2">
                          <span class="inline-flex items-center rounded-md bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-base-content">
                            {cluster()}
                          </span>
                        </TableCell>
                      </TableRow>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </Card>
        </Show>
      </div>
    </Show>
  );
};

export default ProxmoxNodesTable;
