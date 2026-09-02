import { Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { ResourceNameWithWebInterfaceLink } from '@/components/shared/WebInterfaceLink';
import { StatusDot } from '@/components/shared/StatusDot';
import { DockerHostDrawer } from './DockerHostDrawer';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { getAlertStyles } from '@/utils/alerts';
import { useWebSocket } from '@/contexts/appRuntime';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { hostOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
import { asTrimmedString } from '@/utils/stringUtils';
import { normalizeDiskArray } from '@/utils/format';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  PlatformWindowedRows,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformResponsiveTableLabel,
  PlatformTableMetricFallback,
  PlatformTableEmptyState,
  PlatformTableTemperatureValue,
  PlatformTableShell,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableUptimeValue,
  getPlatformTableFiniteMetric,
  getPlatformTableCellClassForKind,
  type PlatformTableSortValue,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import { PlatformResourceDetailToggleButton } from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Disk } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  filterDockerResources,
  getDockerHostSystemBadge,
  hasDockerSwarmEvidence,
  type DockerResourceStatusFilter,
} from './dockerPageModel';

// Docker / Podman hosts are container hosts, not generic Pulse Agents.
// The operator columns that matter are runtime version, container count,
// and Swarm role, alongside the
// usual CPU / Memory / Disk / Uptime / Temperature from the agent
// telemetry. The generic infrastructure table renders the metrics fine
// but omits the runtime context that distinguishes a Docker host from
// any other agent. This bespoke table reuses canonical shared
// primitives and surfaces the Docker-native columns.

const percentFromMetric = (metric: Resource['cpu'] | undefined): number | undefined =>
  getPlatformTableFiniteMetric(metric?.current);

const memoryTotalFor = (host: Resource): number =>
  getPlatformTableFiniteMetric(host.memory?.total) ??
  getPlatformTableFiniteMetric(host.agent?.memory?.total) ??
  getPlatformTableFiniteMetric(host.docker?.memory?.total) ??
  0;

const memoryUsedFor = (host: Resource): number =>
  getPlatformTableFiniteMetric(host.memory?.used) ??
  getPlatformTableFiniteMetric(host.agent?.memory?.used) ??
  getPlatformTableFiniteMetric(host.docker?.memory?.used) ??
  0;

const memoryPercentOnlyFor = (host: Resource): number | undefined => {
  if (memoryTotalFor(host) > 0) return undefined;
  return (
    getPlatformTableFiniteMetric(host.memory?.current) ??
    getPlatformTableFiniteMetric(host.agent?.memory?.usage) ??
    getPlatformTableFiniteMetric(host.docker?.memory?.usage)
  );
};

const memoryUnavailableFor = (host: Resource): boolean =>
  !host.memory &&
  (host.agent?.memory?.usageUnavailable === true || host.docker?.memory?.usageUnavailable === true);

// Host telemetry the Docker agent reports beyond the typed Resource docker
// block. One cast site shared by the row renderer and the sort accessor.
type DockerHostDockerMeta = NonNullable<Resource['docker']> & {
  runtime?: string;
  runtimeVersion?: string;
  containerCount?: number;
  uptimeSeconds?: number;
  temperature?: number;
  swarm?: { nodeRole?: string };
};

const dockerHostMeta = (host: Resource): DockerHostDockerMeta | undefined =>
  host.docker as DockerHostDockerMeta | undefined;

const aggregateDiskFor = (host: Resource): Disk | undefined => {
  if (!host.disk) return undefined;
  const total = getPlatformTableFiniteMetric(host.disk.total) ?? 0;
  const used = getPlatformTableFiniteMetric(host.disk.used) ?? 0;
  const free =
    getPlatformTableFiniteMetric(host.disk.free) ?? (total > 0 ? Math.max(0, total - used) : 0);
  const usage =
    total > 0 && used > 0
      ? (used / total) * 100
      : (getPlatformTableFiniteMetric(host.disk.current) ?? 0);
  if (total <= 0 && usage <= 0) return undefined;
  return { total, used, free, usage };
};

const DOCKER_HOST_SORT_KEYS = [
  'host',
  'system',
  'version',
  'containers',
  'cpu',
  'memory',
  'disk',
  'uptime',
  'temp',
  'swarm',
] as const;

type DockerHostSortKey = (typeof DOCKER_HOST_SORT_KEYS)[number];

const getDockerHostSortValue = (host: Resource, key: DockerHostSortKey): PlatformTableSortValue => {
  switch (key) {
    case 'host':
      return asTrimmedString(host.name) || host.id;
    case 'system':
      return getDockerHostSystemBadge(host)?.label ?? null;
    case 'version':
      return asTrimmedString(dockerHostMeta(host)?.runtimeVersion) || null;
    case 'containers':
      return dockerHostMeta(host)?.containerCount ?? null;
    case 'cpu':
      return percentFromMetric(host.cpu) ?? null;
    case 'memory': {
      if (memoryUnavailableFor(host)) return null;
      const total = memoryTotalFor(host);
      if (total > 0) return (memoryUsedFor(host) / total) * 100;
      return memoryPercentOnlyFor(host) ?? null;
    }
    case 'disk':
      return aggregateDiskFor(host)?.usage ?? null;
    case 'uptime':
      return host.uptime ?? dockerHostMeta(host)?.uptimeSeconds ?? null;
    case 'temp': {
      const temp = host.temperature ?? dockerHostMeta(host)?.temperature;
      return typeof temp === 'number' && Number.isFinite(temp) && temp > 0 ? temp : null;
    }
    case 'swarm': {
      if (!hasDockerSwarmEvidence(host)) return null;
      return asTrimmedString(dockerHostMeta(host)?.swarm?.nodeRole) || null;
    }
    default:
      key satisfies never;
      return null;
  }
};

export const DockerHostsTable: Component<{
  resources: Resource[];
  sourceCount?: number;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: JSX.Element;
  actions?: JSX.Element;
  showToolbar?: boolean;
}> = (props) => {
  const { activeAlerts } = useWebSocket();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = alertsActivation.detectionEnabled;
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
  });
  const showSwarmColumn = createMemo(() => props.resources.some(hasDockerSwarmEvidence));
  const [selectedHostId, setSelectedHostId] = createSignal<string | null>(null);
  const drawerColspan = createMemo(() => (showSwarmColumn() ? 10 : 9));
  const sort = createPlatformTableSortState({
    storageKey: 'dockerHosts',
    sortKeys: DOCKER_HOST_SORT_KEYS,
    descendingFirst: ['containers', 'cpu', 'memory', 'disk', 'uptime', 'temp'],
  });
  const sortedHosts = createMemo(() =>
    sort.sortRows(tableState.filtered(), getDockerHostSortValue),
  );

  const hasFilteredSourceRows = () => (props.sourceCount ?? props.resources.length) > 0;

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={hasFilteredSourceRows() ? 'No hosts match current filters' : props.emptyTitle}
          description={
            hasFilteredSourceRows()
              ? 'Adjust the shared Docker page filters to see more hosts.'
              : props.emptyDescription
          }
        />
      }
    >
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search Docker hosts"
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              PLATFORM_HEALTH_FILTER_OPTIONS,
              tableState.countForStatus,
            )}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="hosts"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No hosts match current filters"
              description="Adjust the search or status filter to see more hosts."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Docker hosts'}
            actions={props.actions}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1240px]"
            header={
              <>
                {/*
                    Desktop widths balance the three bar-metric columns (CPU /
                    Memory / Disk) against the short-content columns so the
                    bars aren't squeezed by table-fixed's equal split. Mobile
                    widths keep host identity at roughly one-third while
                    reserving room for all six operational phone fields.
                  */}
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="host"
                  class="platform-table-mobile-w-30 md:w-[13%]"
                >
                  Host
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="system"
                  class="hidden md:table-cell md:w-[7%]"
                >
                  System
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="version"
                  class="hidden md:table-cell md:w-[7%]"
                >
                  Version
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="containers"
                  class="hidden md:table-cell md:w-[9%]"
                >
                  Containers
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="cpu"
                  class="platform-table-mobile-w-15 w-[15%] min-[360px]:w-[13%] md:w-[14%]"
                >
                  CPU
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="memory"
                  class="platform-table-mobile-w-15 w-[15%] min-[360px]:w-[13%] md:w-[14%]"
                >
                  <span class="md:hidden">Mem</span>
                  <span class="hidden md:inline">Memory</span>
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="disk"
                  class="platform-table-mobile-w-15 w-[15%] min-[360px]:w-[13%] md:w-[14%]"
                >
                  Disk
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="uptime"
                  class="platform-table-mobile-w-20 w-[20%] min-[360px]:w-[11%] sm:hidden md:table-cell md:w-[6%]"
                >
                  <PlatformResponsiveTableLabel compact="Up" full="Uptime" />
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="temp"
                  class="platform-table-narrow-hidden hidden min-[360px]:table-cell min-[360px]:w-[16%] md:table-cell md:w-[6%]"
                >
                  <PlatformResponsiveTableLabel compact="Temp" full="Temp" />
                </PlatformSortableTableHead>
                <Show when={showSwarmColumn()}>
                  <PlatformSortableTableHead
                    kind="text"
                    sort={sort}
                    sortKey="swarm"
                    class="hidden md:table-cell md:w-[10%]"
                  >
                    Swarm role
                  </PlatformSortableTableHead>
                </Show>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={sortedHosts} estimatedRowHeight={32}>
                  {(host) => {
                    const docker = () => dockerHostMeta(host);
                    const name = () => asTrimmedString(host.name) || host.id;
                    const [customUrl, setCustomUrl] = createSignal(asTrimmedString(host.customUrl));
                    const systemBadge = () => getDockerHostSystemBadge(host);
                    const version = () => asTrimmedString(docker()?.runtimeVersion) || '—';
                    const containerCount = () => docker()?.containerCount ?? 0;
                    const swarmRole = () => {
                      if (!hasDockerSwarmEvidence(host)) return '—';
                      const role = asTrimmedString(docker()?.swarm?.nodeRole);
                      return role ? role.charAt(0).toUpperCase() + role.slice(1) : '—';
                    };
                    const indicator = () => getSimpleStatusIndicator(host.status);
                    const canRenderMetrics = () => indicator().variant !== 'danger';
                    const metricsKey = () => buildMetricKeyForUnifiedResource(host);
                    const alertResourceIds = () => hostOverrideIdCandidates(host);
                    const cpuThresholds = () =>
                      alertsActivation.getMetricThresholds('agent', 'cpu', alertResourceIds());
                    const memoryThresholds = () =>
                      alertsActivation.getMetricThresholds('agent', 'memory', alertResourceIds());
                    const diskThresholds = () =>
                      alertsActivation.getMetricThresholds('agent', 'disk', alertResourceIds());
                    const cpuPercent = () => percentFromMetric(host.cpu);
                    const memoryUsed = () => memoryUsedFor(host);
                    const memoryTotal = () => memoryTotalFor(host);
                    const memoryPercentOnly = () => memoryPercentOnlyFor(host);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const aggregateDisk = () => aggregateDiskFor(host);
                    const disks = () => normalizeDiskArray(host.agent?.disks);
                    const hasDiskMetric = () =>
                      aggregateDisk() !== undefined || (disks()?.length ?? 0) > 0;
                    const detailRowId = () => `docker-host-drawer-${host.id}`;
                    const isSelected = () => selectedHostId() === host.id;
                    const toggleDrawer = () =>
                      setSelectedHostId((current) => (current === host.id ? null : host.id));
                    const hostAlertStyles = createMemo(() =>
                      getAlertStyles(host.id, activeAlerts, alertsEnabled(), name()),
                    );
                    const hostAlertBg = () => {
                      const s = hostAlertStyles();
                      if (!s.hasUnacknowledgedAlert) return '';
                      return s.severity === 'critical'
                        ? 'bg-red-50 dark:bg-red-950'
                        : 'bg-yellow-50 dark:bg-yellow-950';
                    };
                    return (
                      <>
                        <TableRow
                          class={`cursor-pointer text-[11px] sm:text-xs ${
                            isSelected() ? 'bg-surface-hover' : hostAlertBg()
                          }`}
                          data-docker-host-row={host.id}
                          onClick={toggleDrawer}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isSelected()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={toggleDrawer}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={host.status || 'unknown'}
                                ariaHidden
                              />
                              <ResourceNameWithWebInterfaceLink
                                name={name()}
                                url={customUrl()}
                                class="min-w-0"
                                nameClass="truncate font-semibold text-base-content"
                              />
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <Show when={systemBadge()} fallback={<span class="text-muted">—</span>}>
                              {(badge) => (
                                <span
                                  class={badge().classes}
                                  title={badge().title ?? badge().label}
                                >
                                  {badge().label}
                                </span>
                              )}
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden font-mono text-[11px] text-base-content md:table-cell`}
                          >
                            {version()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {containerCount()}
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
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
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                            <Show
                              when={canRenderMetrics() && hasMemoryMetric()}
                              fallback={<PlatformTableMetricFallback />}
                            >
                              <StackedMemoryBar
                                used={memoryUsed()}
                                total={memoryTotal()}
                                unavailable={memoryUnavailableFor(host)}
                                percentOnly={memoryPercentOnly()}
                                thresholds={memoryThresholds()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                            <Show
                              when={canRenderMetrics() && hasDiskMetric()}
                              fallback={<PlatformTableMetricFallback />}
                            >
                              <StackedDiskBar
                                mode={(disks()?.length ?? 0) > 1 ? 'vertical-bars' : undefined}
                                disks={disks()}
                                aggregateDisk={aggregateDisk()}
                                thresholds={diskThresholds()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content sm:hidden md:table-cell`}
                          >
                            {formatPlatformTableUptimeValue(host.uptime ?? docker()?.uptimeSeconds)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} platform-table-narrow-hidden hidden text-base-content min-[360px]:table-cell md:table-cell`}
                          >
                            <PlatformTableTemperatureValue
                              value={host.temperature ?? docker()?.temperature}
                            />
                          </TableCell>
                          <Show when={showSwarmColumn()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                            >
                              {swarmRole()}
                            </TableCell>
                          </Show>
                        </TableRow>
                        <Show when={isSelected()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={drawerColspan()}
                            data-inline-docker-host-detail-for={host.id}
                          >
                            <DockerHostDrawer
                              host={host}
                              onClose={() => setSelectedHostId(null)}
                              customUrl={customUrl()}
                              onCustomUrlChange={setCustomUrl}
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

export default DockerHostsTable;
