import { For, Show, createMemo, type Component } from 'solid-js';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { getAlertsForResource } from '@/utils/alerts';
import { getResourceIdentityAliases } from '@/utils/resourceIdentity';
import { alertTypeDisplayLabel } from '@/features/alerts/helpers';
import { useWebSocket } from '@/contexts/appRuntime';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Alert } from '@/types/api';
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
  getPlatformTableWeightedColumnWidthStyle,
  PlatformResponsiveTableLabel,
  type PlatformResourceStatusFilter,
  PlatformTableEmptyState,
  PlatformTableShell,
  PlatformWindowedRows,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import {
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowInteractionProps,
  PlatformResourceDetailToggleButton,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import { ProxmoxMailGatewayDrawer } from './ProxmoxMailGatewayDrawer';

export type MailGatewayPhoneColumn =
  'instance' | 'nodes' | 'uptime' | 'mail' | 'queue' | 'deferred';

export type MailGatewayColumn =
  MailGatewayPhoneColumn | 'version' | 'spam' | 'virus' | 'quarantine';

// Phones carry five tracks. With node count as a sixth, gateway names that
// share a prefix ("mail-gateway-eu", "mail-gateway-us") truncated to the same
// text; the count stays in the row expansion.
export const MAIL_GATEWAY_PHONE_COLUMNS: readonly MailGatewayPhoneColumn[] = [
  'instance',
  'uptime',
  'mail',
  'queue',
  'deferred',
];

export const MAIL_GATEWAY_NARROW_PHONE_COLUMNS: readonly MailGatewayPhoneColumn[] = [
  'instance',
  'uptime',
  'mail',
  'queue',
  'deferred',
];

export const MAIL_GATEWAY_PHONE_COLUMN_WIDTHS: Readonly<Record<MailGatewayPhoneColumn, number>> = {
  instance: 40,
  nodes: 0,
  uptime: 15,
  mail: 15,
  queue: 15,
  deferred: 15,
};

export const MAIL_GATEWAY_NARROW_PHONE_COLUMN_WIDTHS: Readonly<
  Record<MailGatewayPhoneColumn, number>
> = {
  instance: 40,
  nodes: 0,
  uptime: 15,
  mail: 15,
  queue: 15,
  deferred: 15,
};

// Above the phone projection every visible column takes a weighted share of
// the row. Sizing only the always-on columns left Version, Spam, Virus, and
// Quarantine to split the remainder, which clipped their values to a single
// digit on a full-width desktop table.
export const MAIL_GATEWAY_COLUMN_WEIGHTS: Readonly<Record<MailGatewayColumn, number>> = {
  instance: 24,
  version: 11,
  nodes: 10,
  uptime: 12,
  mail: 12,
  spam: 10,
  virus: 10,
  quarantine: 14,
  queue: 12,
  deferred: 13,
};

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
  // A gateway the provider reports online can still be failing its job (mail
  // held in the queue past the configured age, a backlog). Its open alerts
  // decide the row's state, the status filter counts and the reason shown
  // under its name, so "is mail flowing" is answered here, not only on Alerts.
  const { activeAlerts } = useWebSocket();
  const alertsActivation = useAlertsActivation();
  // Alerts are keyed by the PMG instance id while the row carries the unified
  // id, so match across the resource's identity aliases (alerts contract).
  // There is deliberately no node-name fallback: PMG node alerts carry node
  // names, and matching on them could pin one gateway's alert on another.
  const alertIdsFor = (resource: Resource): string[] => [
    resource.id,
    ...getResourceIdentityAliases(resource),
  ];
  // One scan of the active-alert map per gateway, shared by the filter, the
  // status counts, the row and its drawer.
  const openAlertsById = createMemo(() => {
    const byId = new Map<string, Alert[]>();
    for (const resource of props.resources) {
      byId.set(
        resource.id,
        getAlertsForResource(
          alertIdsFor(resource),
          activeAlerts,
          alertsActivation.detectionEnabled(),
        )
          .filter((alert) => !alert.acknowledged)
          .sort((a, b) => Number(b.level === 'critical') - Number(a.level === 'critical')),
      );
    }
    return byId;
  });
  const openAlertsFor = (resource: Resource): Alert[] => openAlertsById().get(resource.id) ?? [];
  // Filtering keeps the provider's buckets: an alerted, reachable gateway
  // counts under attention (degraded), an unreachable one stays offline.
  const effectiveStatus = (resource: Resource): string | undefined => {
    if (openAlertsFor(resource).length === 0) return resource.status;
    return getSimpleStatusIndicator(resource.status).variant === 'danger'
      ? resource.status
      : 'warning';
  };
  // The dot follows the most severe open alert, so a critical one reads red.
  const indicatorFor = (resource: Resource) => {
    const base = getSimpleStatusIndicator(effectiveStatus(resource));
    return openAlertsFor(resource)[0]?.level === 'critical'
      ? { ...base, variant: 'danger' as const }
      : base;
  };
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: (resources, search, status) =>
      filterPlatformResources(resources, search, status, effectiveStatus),
  });
  const detail = createPlatformResourceDetailState({ idPrefix: 'proxmox-mail-gateway-detail' });
  const observedWidth = useObservedElementWidth();
  const layout = createMemo(() =>
    getPlatformTableContainerLayout(observedWidth.width() ?? 1920, [520, 720, 960, 1200]),
  );
  const isNarrowPhone = createMemo(() => {
    const width = observedWidth.width();
    return typeof width === 'number' && width > 0 && width < 360;
  });
  // Uptime and mail-flow counters remain in the phone projection; node count
  // moves into the existing instance detail expansion on phones.
  const showNodes = createMemo(() => layout() !== 'compact');
  const showUptime = createMemo(() => true);
  const showOperational = createMemo(() => ['operational', 'expanded', 'full'].includes(layout()));
  const showVirus = createMemo(() => ['expanded', 'full'].includes(layout()));
  const showVersion = createMemo(() => layout() === 'full');
  const visibleColumns = createMemo<readonly MailGatewayColumn[]>(() => {
    if (layout() === 'compact') {
      return isNarrowPhone() ? MAIL_GATEWAY_NARROW_PHONE_COLUMNS : MAIL_GATEWAY_PHONE_COLUMNS;
    }
    const columns: MailGatewayColumn[] = ['instance'];
    if (showVersion()) columns.push('version');
    if (showNodes()) columns.push('nodes');
    if (showUptime()) columns.push('uptime');
    columns.push('mail');
    if (showOperational()) columns.push('spam');
    if (showVirus()) columns.push('virus');
    if (showOperational()) columns.push('quarantine');
    columns.push('queue', 'deferred');
    return columns;
  });
  const visibleColumnCount = createMemo(() => visibleColumns().length);
  const columnWidthStyle = (column: MailGatewayColumn) => {
    if (layout() !== 'compact') {
      return getPlatformTableWeightedColumnWidthStyle(
        column,
        MAIL_GATEWAY_COLUMN_WEIGHTS,
        visibleColumns(),
      );
    }
    const phoneWidths: Partial<Record<MailGatewayColumn, number>> = isNarrowPhone()
      ? MAIL_GATEWAY_NARROW_PHONE_COLUMN_WIDTHS
      : MAIL_GATEWAY_PHONE_COLUMN_WIDTHS;
    return { width: `${phoneWidths[column] ?? 0}%` };
  };

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
          searchSuggestions={tableState.searchSuggestions}
          status={tableState.status()}
          onStatusChange={tableState.setStatus}
          statusOptions={withPlatformStatusCounts(
            PLATFORM_HEALTH_FILTER_OPTIONS,
            tableState.countForStatus,
          )}
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
            tableClass="min-w-0 table-fixed text-xs"
            colgroup={
              <colgroup>
                <For each={visibleColumns()}>
                  {(column) => (
                    <col style={columnWidthStyle(column)} data-proxmox-mail-column={column} />
                  )}
                </For>
              </colgroup>
            }
            header={
              <>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30`}
                >
                  Instance
                </TableHead>
                <Show when={showVersion()}>
                  <TableHead class={getPlatformTableHeadClassForKind('text')}>Version</TableHead>
                </Show>
                <Show when={showNodes()}>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15`}
                  >
                    Nodes
                  </TableHead>
                </Show>
                <Show when={showUptime()}>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15`}
                  >
                    {layout() === 'compact' ? 'Age' : 'Uptime'}
                  </TableHead>
                </Show>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15`}
                >
                  <PlatformResponsiveTableLabel compact="In" full="Mail in" />
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
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15`}
                >
                  <PlatformResponsiveTableLabel compact="Q" full="Queue" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-10`}
                >
                  <PlatformResponsiveTableLabel compact="Def" full="Deferred" />
                </TableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={tableState.filtered} estimatedRowHeight={32}>
                  {(instance) => {
                    const pmg = () => instance.pmg;
                    const name = () => asTrimmedString(instance.name) || instance.id;
                    const version = () => asTrimmedString(pmg()?.version) || '—';
                    const rowAlerts = createMemo(() => openAlertsFor(instance));
                    const indicator = () => indicatorFor(instance);
                    // Same tint as the other platform tables, from the cached alerts.
                    const rowAlertBg = () => {
                      const top = rowAlerts()[0];
                      if (!top) return '';
                      return top.level === 'critical'
                        ? 'bg-red-50 dark:bg-red-950/25'
                        : 'bg-yellow-50 dark:bg-yellow-950/25';
                    };
                    const isOpen = () => detail.isExpanded(instance);
                    const detailRowId = () => detail.detailRowId(instance);
                    return (
                      <>
                        <TableRow
                          {...getPlatformResourceDetailRowInteractionProps({
                            expanded: isOpen(),
                            onToggle: () => detail.toggle(instance),
                            class: rowAlertBg(),
                          })}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex items-center gap-2 min-w-0">
                              <PlatformResourceDetailToggleButton
                                expanded={isOpen()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => detail.toggle(instance)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={rowAlerts()[0]?.message || instance.status || 'unknown'}
                                ariaHidden
                              />
                              <div class="min-w-0">
                                <span
                                  class="block font-semibold text-base-content truncate"
                                  title={name()}
                                >
                                  {name()}
                                </span>
                                <Show when={rowAlerts()[0]}>
                                  {(alert) => (
                                    <span
                                      class={`block truncate text-[11px] font-medium ${
                                        alert().level === 'critical'
                                          ? 'text-red-600 dark:text-red-300'
                                          : 'text-amber-700 dark:text-amber-300'
                                      }`}
                                      title={rowAlerts()
                                        .map((open) => open.message)
                                        .join('\n')}
                                      data-mail-gateway-alert-reason
                                    >
                                      {alertTypeDisplayLabel(alert().type)}
                                      {rowAlerts().length > 1 ? ` +${rowAlerts().length - 1}` : ''}
                                    </span>
                                  )}
                                </Show>
                              </div>
                            </div>
                          </TableCell>
                          <Show when={showVersion()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                            >
                              {version()}
                            </TableCell>
                          </Show>
                          <Show when={showNodes()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              <PlatformTableNumberValue
                                value={pmg()?.nodeCount}
                                format={formatPlatformTableIntegerValue}
                              />
                            </TableCell>
                          </Show>
                          <Show when={showUptime()}>
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
                            cellClass="whitespace-normal"
                            contentClass="min-w-0 whitespace-normal px-4 py-4"
                            data-inline-detail-for={instance.id}
                          >
                            <ProxmoxMailGatewayDrawer
                              instanceRow={instance}
                              onClose={() => detail.close(instance)}
                              alerts={rowAlerts()}
                            />
                          </InlineDetailTableRow>
                        </Show>
                      </>
                    );
                  }}
                </PlatformWindowedRows>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default ProxmoxMailGatewayTable;
