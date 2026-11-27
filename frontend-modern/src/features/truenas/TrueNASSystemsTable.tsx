import { Show, createMemo, type Component, type JSX } from 'solid-js';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { getAlertStyles, TRUENAS_ALERT_INSTANCES } from '@/utils/alerts';
import { useWebSocket } from '@/contexts/appRuntime';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { unifiedPlatformOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
import { asTrimmedString } from '@/utils/stringUtils';
import { getTemperatureSymbol } from '@/utils/temperature';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  PlatformWindowedRows,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformTableMetricFallback,
  PlatformTableNumberValue,
  PlatformTablePercentValue,
  PlatformTableTemperatureValue,
  PlatformResponsiveTableLabel,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  filterPlatformResources,
  formatPlatformTableBytesValue,
  formatPlatformTableUptimeValue,
  getPlatformTableFiniteMetric,
  getPlatformTableCellClassForKind,
  type PlatformResourceStatusFilter,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import {
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  TrueNASResourceDetailRow,
  TrueNASRowLead,
  TrueNASStaleBadge,
  TrueNASTableFrame,
  createTrueNASDetailState,
  getTrueNASMemoryDisplay,
  getTrueNASMemoryPercentSortValue,
} from './TrueNASTableFrame';
import {
  buildTrueNASSystemChildCounts,
  getTrueNASResourceDisplayStatus,
  getTrueNASStaleNotice,
  type TrueNASSystemChildCounts,
} from '@/features/truenas/truenasPageModel';
import type { Resource } from '@/types/resource';

// TrueNAS systems are storage appliances, not generic compute hosts.
// The generic infrastructure table's CPU / Memory / Disk columns are
// helpful (the agent payload carries them), but operators also want at-
// a-glance native inventory on the same row. Keep that inventory grouped
// instead of giving every count a separate column; the Overview page should
// scan as an appliance summary, while the dedicated tabs own full detail.
// This bespoke table reuses
// canonical shared primitives (Card, Table, SearchInput,
// FilterButtonGroup, StatusDot) and counts the per-system children
// client-side from the same TrueNAS resource scope already fetched by
// the page (no extra API calls).

const EMPTY_COUNTS: TrueNASSystemChildCounts = {
  pools: 0,
  datasets: 0,
  shares: 0,
  vms: 0,
  apps: 0,
  disks: 0,
  services: 0,
};

const plural = (count: number, singular: string): string =>
  `${count} ${count === 1 ? singular : `${singular}s`}`;

const systemCapacityPercent = (system: Resource): number | undefined => {
  const current = getPlatformTableFiniteMetric(system.disk?.current);
  if (current !== undefined) return current;
  const used = getPlatformTableFiniteMetric(system.disk?.used);
  const total = getPlatformTableFiniteMetric(system.disk?.total);
  return used !== undefined && total !== undefined && total > 0 ? (used / total) * 100 : undefined;
};

// Columns a user can sort by. The Inventory and VMs / Apps columns summarize
// several counts at once, so they carry no single scalar to order on.
const TRUENAS_SYSTEM_SORT_KEYS = [
  'system',
  'cpu',
  'memory',
  'capacity',
  'temp',
  'shares',
  'services',
] as const;

type TrueNASSystemSortKey = (typeof TRUENAS_SYSTEM_SORT_KEYS)[number];

const getTrueNASSystemSortValue = (
  system: Resource,
  counts: TrueNASSystemChildCounts,
  key: TrueNASSystemSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'system':
      return asTrimmedString(system.name) || system.id;
    case 'cpu':
      return getPlatformTableFiniteMetric(system.cpu?.current) ?? null;
    case 'memory':
      return getTrueNASMemoryPercentSortValue(system);
    case 'capacity':
      return systemCapacityPercent(system) ?? null;
    case 'temp':
      return getPlatformTableFiniteMetric(system.temperature) ?? null;
    case 'shares':
      return counts.shares;
    case 'services':
      return counts.services;
    default:
      key satisfies never;
      return null;
  }
};

export const TrueNASSystemsTable: Component<{
  systems: Resource[];
  // Full TrueNAS resource scope so we can count pools / datasets / apps
  // per system without spawning additional fetches.
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const { activeAlerts } = useWebSocket();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = alertsActivation.detectionEnabled;
  const tableState = createPlatformTableFilterState({
    resources: () => props.systems,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: (resources, search, status) =>
      filterPlatformResources(resources, search, status, getTrueNASResourceDisplayStatus),
  });
  const drawer = createTrueNASDetailState('truenas-system-drawer');
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);
  const countsBySystem = createMemo(() =>
    buildTrueNASSystemChildCounts(props.scope, props.systems),
  );
  const sort = createPlatformTableSortState({
    storageKey: 'truenasSystems',
    sortKeys: TRUENAS_SYSTEM_SORT_KEYS,
    descendingFirst: ['cpu', 'memory', 'capacity', 'temp', 'shares', 'services'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), (system, key) =>
      getTrueNASSystemSortValue(system, countsBySystem().get(system.id) ?? EMPTY_COUNTS, key),
    ),
  );

  return (
    <TrueNASTableFrame
      hasItems={props.systems.length > 0}
      tableState={tableState}
      common={props}
      filteredEmptyDescription="Adjust the search or status filter to see more systems."
      searchPlaceholder="Search TrueNAS systems"
      statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
      rowNoun="systems"
      title={props.title ?? 'Systems'}
      tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
      header={
        <>
          <PlatformSortableTableHead
            kind="name"
            sort={sort}
            sortKey="system"
            class="platform-table-mobile-w-30 md:w-[17%]"
          >
            System
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="metric-bar"
            sort={sort}
            sortKey="cpu"
            class="platform-table-mobile-w-15 md:w-[10%]"
          >
            CPU
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="metric-bar"
            sort={sort}
            sortKey="memory"
            class="platform-table-mobile-w-15 md:w-[10%]"
          >
            <PlatformResponsiveTableLabel compact="Mem" full="Memory" />
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="metric-bar"
            sort={sort}
            sortKey="capacity"
            class="platform-table-mobile-w-20 md:w-[13%]"
          >
            <PlatformResponsiveTableLabel compact="Cap" full="Capacity" />
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="numeric-value"
            sort={sort}
            sortKey="temp"
            class="platform-table-mobile-w-20 md:w-[6%]"
          >
            <PlatformResponsiveTableLabel compact={getTemperatureSymbol()} full="Temp" />
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="text"
            sort={sort}
            class="hidden lg:table-cell lg:w-[15%]"
          >
            Inventory
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="numeric-value"
            sort={sort}
            sortKey="shares"
            class="hidden lg:table-cell lg:w-[8%]"
          >
            Shares
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="text"
            sort={sort}
            class="hidden lg:table-cell lg:w-[10%]"
          >
            VMs / Apps
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="numeric-value"
            sort={sort}
            sortKey="services"
            class="hidden lg:table-cell lg:w-[10%]"
          >
            Services
          </PlatformSortableTableHead>
        </>
      }
      body={
        <>
          <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
            {(system) => {
              const name = () => asTrimmedString(system.name) || system.id;
              const version = () => asTrimmedString(system.agent?.osVersion) || '—';
              const displayStatus = () => getTrueNASResourceDisplayStatus(system);
              const indicator = () => getSimpleStatusIndicator(displayStatus());
              const staleNotice = () => getTrueNASStaleNotice(system);
              const storagePercent = () => systemCapacityPercent(system);
              const storageFullLabel = () =>
                typeof system.disk?.used === 'number' && typeof system.disk?.total === 'number'
                  ? `${formatPlatformTableBytesValue(system.disk.used)} / ${formatPlatformTableBytesValue(system.disk.total)}`
                  : undefined;
              const c = () => countsBySystem().get(system.id) ?? EMPTY_COUNTS;
              const storageInventory = () =>
                `${plural(c().pools, 'pool')} / ${plural(c().datasets, 'dataset')} · ${plural(c().disks, 'disk')}`;
              const workloadInventory = () =>
                `${plural(c().vms, 'VM')} · ${plural(c().apps, 'app')}`;
              const uptimeLabel = () => formatPlatformTableUptimeValue(system.uptime);
              const systemMeta = () =>
                [
                  version() !== '—' ? version() : '',
                  uptimeLabel() !== '—' ? `up ${uptimeLabel()}` : '',
                ]
                  .filter(Boolean)
                  .join(' · ');
              const metricsKey = () => buildMetricKeyForUnifiedResource(system);
              const alertResourceIds = () => unifiedPlatformOverrideIdCandidates(system);
              const cpuThresholds = () =>
                alertsActivation.getMetricThresholds('truenas', 'cpu', alertResourceIds());
              const memoryThresholds = () =>
                alertsActivation.getMetricThresholds('truenas', 'memory', alertResourceIds());
              const cpuPercent = () => getPlatformTableFiniteMetric(system.cpu?.current);
              const memory = () => getTrueNASMemoryDisplay(system);
              const memoryCache = () =>
                getPlatformTableFiniteMetric(system.agent?.memory?.cache) ?? 0;
              const canRenderMetrics = () => indicator().variant !== 'muted';
              const isExpanded = () => drawer.isExpanded(system);
              const sysAlertStyles = createMemo(() =>
                getAlertStyles(system.id, activeAlerts, alertsEnabled(), {
                  node: name(),
                  instances: TRUENAS_ALERT_INSTANCES,
                }),
              );
              const sysAlertBg = () => {
                const s = sysAlertStyles();
                if (!s.hasUnacknowledgedAlert) return '';
                return s.severity === 'critical'
                  ? 'bg-red-50 dark:bg-red-950/25'
                  : 'bg-yellow-50 dark:bg-yellow-950/25';
              };
              return (
                <>
                  <TableRow
                    class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs ${sysAlertBg()}`}
                    data-truenas-system-row={system.id}
                    onClick={() => drawer.toggle(system)}
                  >
                    <TableCell class={getPlatformTableCellClassForKind('name')}>
                      <TrueNASRowLead
                        item={system}
                        drawer={drawer}
                        label={name()}
                        variant={indicator().variant}
                        dotTitle={displayStatus() || 'unknown'}
                        dotAriaHidden
                      >
                        <div class="min-w-0">
                          <div
                            class="truncate font-semibold text-base-content"
                            title={[name(), systemMeta()].filter(Boolean).join(' · ')}
                          >
                            {name()}
                          </div>
                          <TrueNASStaleBadge notice={staleNotice()} attr="system" />
                        </div>
                      </TrueNASRowLead>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('metric-bar')} w-[20%] md:w-auto`}
                    >
                      <ResponsiveMetricCell
                        class="w-full"
                        value={cpuPercent() ?? 0}
                        type="cpu"
                        resourceId={metricsKey()}
                        isRunning={canRenderMetrics() && cpuPercent() !== undefined}
                        showMobile={false}
                        thresholds={cpuThresholds()}
                      />
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('metric-bar')} w-[20%] md:w-auto`}
                    >
                      <Show
                        when={canRenderMetrics() && memory().hasMetric}
                        fallback={<PlatformTableMetricFallback />}
                      >
                        <StackedMemoryBar
                          used={memory().used}
                          total={memory().total}
                          cache={memoryCache()}
                          cacheInclusiveLabel="Used with ARC cache"
                          detailsLabel={`Memory composition for ${name()}`}
                          percentOnly={memory().percentOnly}
                          thresholds={memoryThresholds()}
                        />
                      </Show>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}
                    >
                      <span class="md:hidden">
                        <PlatformTablePercentValue value={storagePercent()} />
                      </span>
                      <span class="hidden md:inline">
                        <Show
                          when={storageFullLabel()}
                          fallback={<PlatformTablePercentValue value={storagePercent()} />}
                        >
                          {(label) => label()}
                        </Show>
                      </span>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                    >
                      <PlatformTableTemperatureValue value={system.temperature} />
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                      title={storageInventory()}
                    >
                      <div class="truncate">{storageInventory()}</div>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content lg:table-cell`}
                    >
                      <PlatformTableNumberValue value={c().shares} />
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                      title={workloadInventory()}
                    >
                      <div class="truncate">{workloadInventory()}</div>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content lg:table-cell`}
                    >
                      <PlatformTableNumberValue value={c().services} />
                    </TableCell>
                  </TableRow>
                  <TrueNASResourceDetailRow
                    resource={system}
                    indicator={indicator()}
                    drawer={drawer}
                    colSpan={9}
                    resolveResourceLabel={resolveResourceLabel}
                  />
                </>
              );
            }}
          </PlatformWindowedRows>
        </>
      }
    />
  );
};

export default TrueNASSystemsTable;
