import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { SearchInput } from '@/components/shared/SearchInput';
import { StatusDot } from '@/components/shared/StatusDot';
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
import {
  filterPlatformResources,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';

// Proxmox Mail Gateway instances are mail-flow / quarantine appliances.
// The generic infrastructure table renders dashes for Disk I/O / Uptime
// / Temperature (PMG only exposes uptime, which we project now) and
// omits the queue / spam / virus / quarantine counts that are the
// operator columns. This bespoke table reuses canonical shared
// primitives and surfaces those PMG-native columns.

const STATUS_FILTER_OPTIONS: FilterOption<PlatformResourceStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'online', label: 'Healthy' },
  { value: 'degraded', label: 'Degraded' },
  { value: 'offline', label: 'Offline' },
];

const formatUptime = (seconds: number | undefined): string => {
  if (!seconds || seconds <= 0) return '—';
  const days = Math.floor(seconds / 86_400);
  if (days > 0) return `${days}d`;
  const hours = Math.floor(seconds / 3_600);
  if (hours > 0) return `${hours}h`;
  const mins = Math.floor(seconds / 60);
  return `${mins}m`;
};

const countCell = (value: number | undefined): JSX.Element => (
  <span class="tabular-nums">{typeof value === 'number' ? value.toLocaleString() : '—'}</span>
);

export const ProxmoxMailGatewayTable: Component<{
  resources: Resource[];
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<PlatformResourceStatusFilter>('all');

  const filtered = createMemo(() => filterPlatformResources(props.resources, search(), status()));
  const visible = createMemo(() => filtered().length);
  const total = createMemo(() => props.resources.length);

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <Card padding="lg">
          <EmptyState title={props.emptyTitle} description={props.emptyDescription} />
        </Card>
      }
    >
      <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2">
          <div class="min-w-[200px] flex-1 sm:max-w-xs">
            <SearchInput
              value={search}
              onChange={setSearch}
              placeholder="Search Mail Gateways"
            />
          </div>
          <FilterButtonGroup
            options={STATUS_FILTER_OPTIONS}
            value={status()}
            onChange={setStatus}
          />
          <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
            <Show when={visible() !== total()} fallback={<>{total()} instances</>}>
              {visible()} of {total()} instances
            </Show>
          </span>
        </div>

        <Show
          when={filtered().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                title="No instances match current filters"
                description="Adjust the search or status filter to see more instances."
              />
            </Card>
          }
        >
          <Card padding="none" tone="card" class="overflow-hidden">
            <Table class="w-full min-w-[1080px] border-collapse text-xs">
              <TableHeader class="bg-surface-alt text-muted border-b border-border">
                <TableRow class="text-left text-[10px] uppercase tracking-wide">
                  <TableHead class="px-3 py-2 font-medium">Instance</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Version</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Nodes</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Uptime</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Mail in</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Spam</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Virus</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Quarantine</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Queue</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Deferred</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border-subtle">
                <For each={filtered()}>
                  {(instance) => {
                    const pmg = () => instance.pmg;
                    const name = () => asTrimmedString(instance.name) || instance.id;
                    const version = () => asTrimmedString(pmg()?.version) || '—';
                    const indicator = () => getSimpleStatusIndicator(instance.status);
                    return (
                      <TableRow class="hover:bg-surface-hover">
                        <TableCell class="px-3 py-2">
                          <div class="flex items-center gap-2 min-w-0">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={instance.status || 'unknown'}
                              ariaHidden
                            />
                            <span class="font-semibold text-base-content truncate" title={name()}>
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                          {version()}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {countCell(pmg()?.nodeCount)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatUptime(instance.uptime ?? pmg()?.uptimeSeconds)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {countCell(pmg()?.mailCountTotal)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {countCell(pmg()?.spamIn)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {countCell(pmg()?.virusIn)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {countCell(pmg()?.quarantine)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {countCell(pmg()?.queueTotal ?? pmg()?.queueActive)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {countCell(pmg()?.queueDeferred)}
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

export default ProxmoxMailGatewayTable;
