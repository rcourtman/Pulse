import { For, Show, createSignal, type Component, type JSX } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  filterPlatformResources,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformResourceStatusFilter,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import { ProxmoxMailGatewayDrawer } from './ProxmoxMailGatewayDrawer';

// Proxmox Mail Gateway instances are mail-flow / quarantine appliances.
// The generic infrastructure table renders dashes for Disk I/O / Uptime
// / Temperature (PMG only exposes uptime, which we project now) and
// omits the queue / spam / virus / quarantine counts that are the
// operator columns. This bespoke table reuses canonical shared
// primitives and surfaces those PMG-native columns.

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
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterPlatformResources,
  });
  const [selectedId, setSelectedId] = createSignal<string | null>(null);
  const toggleSelected = (id: string) => setSelectedId((current) => (current === id ? null : id));

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
        <PlatformTableToolbar
          search={tableState.search}
          onSearchChange={tableState.setSearch}
          searchPlaceholder="Search Mail Gateways"
          status={tableState.status()}
          onStatusChange={tableState.setStatus}
          statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
          visible={tableState.visible()}
          total={tableState.total()}
          rowNoun="instances"
        />

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                title="No instances match current filters"
                description="Adjust the search or status filter to see more instances."
              />
            </Card>
          }
        >
          <PlatformTableShell
            tableClass="min-w-[1080px] text-xs"
            header={
              <>
                <TableHead class={getPlatformTableHeadClassForKind('name')}>Instance</TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>Version</TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Nodes
                </TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Uptime
                </TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Mail in
                </TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Spam
                </TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Virus
                </TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Quarantine
                </TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Queue
                </TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Deferred
                </TableHead>
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
                  {(instance) => {
                    const pmg = () => instance.pmg;
                    const name = () => asTrimmedString(instance.name) || instance.id;
                    const version = () => asTrimmedString(pmg()?.version) || '—';
                    const indicator = () => getSimpleStatusIndicator(instance.status);
                    const isOpen = () => selectedId() === instance.id;
                    return (
                      <>
                        <TableRow
                          class={`cursor-pointer hover:bg-surface-hover ${
                            isOpen() ? 'bg-surface-hover' : ''
                          }`}
                          onClick={() => toggleSelected(instance.id)}
                          aria-expanded={isOpen()}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
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
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                          >
                            {version()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {countCell(pmg()?.nodeCount)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {formatUptime(instance.uptime ?? pmg()?.uptimeSeconds)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {countCell(pmg()?.mailCountTotal)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {countCell(pmg()?.spamIn)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {countCell(pmg()?.virusIn)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {countCell(pmg()?.quarantine)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {countCell(pmg()?.queueTotal ?? pmg()?.queueActive)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {countCell(pmg()?.queueDeferred)}
                          </TableCell>
                        </TableRow>
                        <Show when={isOpen()}>
                          <TableRow data-inline-detail-for={instance.id}>
                            <TableCell
                              colspan={10}
                              class="p-0 border-b border-border bg-surface-alt"
                            >
                              <div class="px-4 py-4" onClick={(event) => event.stopPropagation()}>
                                <ProxmoxMailGatewayDrawer
                                  instanceRow={instance}
                                  onClose={() => setSelectedId(null)}
                                />
                              </div>
                            </TableCell>
                          </TableRow>
                        </Show>
                      </>
                    );
                  }}
                </For>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default ProxmoxMailGatewayTable;
