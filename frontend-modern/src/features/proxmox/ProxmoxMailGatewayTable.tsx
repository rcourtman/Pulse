import { For, Show, createMemo, createSignal, type Component } from 'solid-js';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableNumberValue,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  filterPlatformResources,
  formatPlatformTableIntegerValue,
  formatPlatformTableUptimeValue,
  getPlatformTableCellClassForKind,
  getPlatformTableContainerLayout,
  getPlatformTableHeadClassForKind,
  type PlatformResourceStatusFilter,
  PlatformTableEmptyState,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import { PlatformResourceDetailToggleButton } from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import { ProxmoxMailGatewayDrawer } from './ProxmoxMailGatewayDrawer';

// Proxmox Mail Gateway instances are mail-flow / quarantine appliances.
// The generic infrastructure table renders dashes for Disk I/O / Uptime
// / Temperature (PMG only exposes uptime, which we project now) and
// omits the queue / spam / virus / quarantine counts that are the
// operator columns. This bespoke table reuses canonical shared
// primitives and surfaces those PMG-native columns.

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
  const observedWidth = useObservedElementWidth();
  const layout = createMemo(() =>
    getPlatformTableContainerLayout(observedWidth.width() ?? 1920, [520, 720, 960, 1200]),
  );
  const showBasic = createMemo(() => layout() !== 'compact');
  const showOperational = createMemo(() => ['operational', 'expanded', 'full'].includes(layout()));
  const showVirus = createMemo(() => ['expanded', 'full'].includes(layout()));
  const showVersion = createMemo(() => layout() === 'full');
  const visibleColumnCount = createMemo(
    () =>
      4 +
      Number(showBasic()) * 2 +
      Number(showOperational()) * 2 +
      Number(showVirus()) +
      Number(showVersion()),
  );

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState title={props.emptyTitle} description={props.emptyDescription} />
      }
    >
      <div ref={observedWidth.setElement} class="space-y-3" data-proxmox-mail-layout={layout()}>
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
            <PlatformTableEmptyState
              title="No instances match current filters"
              description="Adjust the search or status filter to see more instances."
            />
          }
        >
          <PlatformTableShell
            tableClass="min-w-[0px] table-fixed text-xs"
            header={
              <>
                <TableHead class={getPlatformTableHeadClassForKind('name')}>Instance</TableHead>
                <Show when={showVersion()}>
                  <TableHead class={getPlatformTableHeadClassForKind('text')}>Version</TableHead>
                </Show>
                <Show when={showBasic()}>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Nodes
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Uptime
                  </TableHead>
                </Show>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Mail in
                </TableHead>
                <Show when={showOperational()}>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Spam
                  </TableHead>
                </Show>
                <Show when={showVirus()}>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Virus
                  </TableHead>
                </Show>
                <Show when={showOperational()}>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Quarantine
                  </TableHead>
                </Show>
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
                    const detailRowId = () => `proxmox-mail-gateway-detail-${instance.id}`;
                    return (
                      <>
                        <TableRow
                          class={`cursor-pointer hover:bg-surface-hover ${
                            isOpen() ? 'bg-surface-hover' : ''
                          }`}
                          onClick={() => toggleSelected(instance.id)}
                          aria-controls={isOpen() ? detailRowId() : undefined}
                          aria-expanded={isOpen()}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex items-center gap-2 min-w-0">
                              <PlatformResourceDetailToggleButton
                                expanded={isOpen()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => toggleSelected(instance.id)}
                              />
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
                          <Show when={showVersion()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                            >
                              {version()}
                            </TableCell>
                          </Show>
                          <Show when={showBasic()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              <PlatformTableNumberValue
                                value={pmg()?.nodeCount}
                                format={formatPlatformTableIntegerValue}
                              />
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              {formatPlatformTableUptimeValue(
                                instance.uptime ?? pmg()?.uptimeSeconds,
                              )}
                            </TableCell>
                          </Show>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={pmg()?.mailCountTotal}
                              format={formatPlatformTableIntegerValue}
                            />
                          </TableCell>
                          <Show when={showOperational()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              <PlatformTableNumberValue
                                value={pmg()?.spamIn}
                                format={formatPlatformTableIntegerValue}
                              />
                            </TableCell>
                          </Show>
                          <Show when={showVirus()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              <PlatformTableNumberValue
                                value={pmg()?.virusIn}
                                format={formatPlatformTableIntegerValue}
                              />
                            </TableCell>
                          </Show>
                          <Show when={showOperational()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              <PlatformTableNumberValue
                                value={pmg()?.quarantine}
                                format={formatPlatformTableIntegerValue}
                              />
                            </TableCell>
                          </Show>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={pmg()?.queueTotal ?? pmg()?.queueActive}
                              format={formatPlatformTableIntegerValue}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={pmg()?.queueDeferred}
                              format={formatPlatformTableIntegerValue}
                            />
                          </TableCell>
                        </TableRow>
                        <Show when={isOpen()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={visibleColumnCount()}
                            contentClass="px-4 py-4"
                            data-inline-detail-for={instance.id}
                          >
                            <ProxmoxMailGatewayDrawer
                              instanceRow={instance}
                              onClose={() => setSelectedId(null)}
                            />
                          </InlineDetailTableRow>
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
