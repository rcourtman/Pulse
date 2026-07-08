import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { useSearchParams } from '@solidjs/router';
import type { FilterDef } from '@/components/shared/FilterBar';
import { UpdateButton } from '@/components/shared/ContainerUpdateBadge';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { getWorkloadTableLayoutMode } from '@/components/Workloads/guestRowModel';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import { DOCKER_QUERY_PARAMS } from '@/routing/resourceLinks';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableMetricFallback,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableFiniteMetric,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  DockerResourceNameCell,
  dockerHostName,
  dockerJoinValues,
  dockerResourceName,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import {
  compareDockerContainers,
  dockerContainerPortsSummary,
  filterDockerResources,
  mapDockerContainerStatus,
  type DockerResourceStatusFilter,
} from './dockerPageModel';
import {
  getDockerContainerColumnWidthStyle,
  getDockerContainerTableMinWidthClass,
  getDockerContainerVisibleColumnsForLayout,
  type DockerContainerTableColumn,
} from './dockerContainerTableModel';
import type { DockerContainerUpdateStatus } from '@/types/api';
import type { Resource } from '@/types/resource';
import { DockerContainerLifecycleControls } from './DockerContainerLifecycleControls';

type DockerNetwork = NonNullable<NonNullable<Resource['docker']>['networks']>[number];
type DockerMount = NonNullable<NonNullable<Resource['docker']>['mounts']>[number];

type DockerContainersTableProps = DockerNativeTableProps & {
  onLifecycleActionSettled?: () => void | Promise<void>;
};

const networkLabel = (network: DockerNetwork): string => {
  const name = asTrimmedString(network.name);
  const address = asTrimmedString(network.ipv4) || asTrimmedString(network.ipv6);
  if (name && address) return `${name} ${address}`;
  return name || address || '';
};

const networksSummary = (resource: Resource): string =>
  dockerJoinValues(resource.docker?.networks?.map((network) => networkLabel(network)));

const mountLabel = (mount: DockerMount): string => {
  const type = asTrimmedString(mount.type);
  const destination = asTrimmedString(mount.destination);
  const source = asTrimmedString(mount.source);
  const mode = asTrimmedString(mount.mode) || (mount.rw === false ? 'ro' : '');
  const endpoint = destination || source;
  if (!endpoint) return type || '';
  const prefix = type ? `${type}:` : '';
  const suffix = mode ? ` (${mode})` : '';
  return `${prefix}${endpoint}${suffix}`;
};

const mountsSummary = (resource: Resource): string =>
  dockerJoinValues(resource.docker?.mounts?.map((mount) => mountLabel(mount)));

const runtimeSummary = (resource: Resource): string => {
  const runtime = asTrimmedString(resource.docker?.runtime);
  const version = asTrimmedString(
    resource.docker?.runtimeVersion || resource.docker?.dockerVersion,
  );
  if (runtime && version) return `${runtime} ${version}`;
  return runtime || version || '—';
};

const containerState = (resource: Resource): string =>
  dockerTextValue(resource.docker?.containerState || resource.status);

const isContainerRunning = (resource: Resource): boolean =>
  (asTrimmedString(resource.docker?.containerState || resource.status) ?? '').toLowerCase() ===
  'running';

// v5 flagged crash-loopers in the restarts column; a container that restarted
// more than this many times needs an operator's eye even while "running".
const RESTART_ATTENTION_THRESHOLD = 5;

const updateStatusLabel = (resource: Resource): string => {
  const update = resource.docker?.updateStatus;
  if (!update) return '—';
  if (asTrimmedString(update.error)) return 'Error';
  return update.updateAvailable ? 'Available' : 'Current';
};

const updateStatusTitle = (resource: Resource): string => {
  const update = resource.docker?.updateStatus;
  if (!update) return 'No image update check reported';

  return dockerJoinValues(
    [
      updateStatusLabel(resource),
      update.error,
      update.currentDigest ? `current ${update.currentDigest}` : undefined,
      update.latestDigest ? `latest ${update.latestDigest}` : undefined,
      update.lastChecked ? `checked ${update.lastChecked}` : undefined,
    ],
    updateStatusLabel(resource),
  );
};

const updateLastCheckedMillis = (lastChecked: string | undefined): number => {
  if (!lastChecked) return 0;
  const parsed = Date.parse(lastChecked);
  return Number.isFinite(parsed) ? parsed : 0;
};

const containerUpdateStatus = (resource: Resource): DockerContainerUpdateStatus | undefined => {
  const update = resource.docker?.updateStatus;
  if (!update) return undefined;

  const error = asTrimmedString(update.error);
  if (typeof update.updateAvailable !== 'boolean' && !error) return undefined;

  return {
    updateAvailable: update.updateAvailable === true,
    currentDigest: asTrimmedString(update.currentDigest) || undefined,
    latestDigest: asTrimmedString(update.latestDigest) || undefined,
    lastChecked: updateLastCheckedMillis(update.lastChecked),
    error: error || undefined,
  };
};

const containerUpdateAction = (
  resource: Resource,
):
  | {
      agentId: string;
      containerId: string;
      containerName: string;
      updateStatus: DockerContainerUpdateStatus;
    }
  | undefined => {
  const updateStatus = containerUpdateStatus(resource);
  const agentId = asTrimmedString(resource.docker?.hostSourceId);
  const containerId = asTrimmedString(resource.docker?.containerId);
  if (!updateStatus || !agentId || !containerId) return undefined;

  return {
    agentId,
    containerId,
    containerName:
      asTrimmedString(resource.name) ||
      asTrimmedString(resource.displayName) ||
      asTrimmedString(resource.docker?.displayName) ||
      containerId,
    updateStatus,
  };
};

const DOCKER_CONTAINER_SEARCH_TIPS = {
  popoverId: 'docker-containers-search-help',
  intro: 'Filter containers by name, image, compose stack, or label.',
  tips: [
    { code: 'nginx', description: 'Match container names, images, and labels' },
    { code: '-watchtower', description: 'Hide containers matching a term' },
  ],
  footerHighlight: 'Tip',
  footerText:
    'Combine exclusions and save them as a default view to keep noisy containers hidden.',
};

export const DockerContainersTable: Component<DockerContainersTableProps> = (props) => {
  const breakpoint = useBreakpoint();
  const layoutMode = createMemo(() => getWorkloadTableLayoutMode(breakpoint.width()));
  // Search and status live in the URL, like the host scope below, so the
  // whole filter state is shareable and captured by saved views. That is what
  // makes exclusions persistent: search `-name`, save it as the default view,
  // and the containers stay hidden on every visit. URL writes replace the
  // history entry so typing does not pile up back-button states.
  const [searchParams, setSearchParams] = useSearchParams();
  const searchFilter = () => {
    const value = searchParams[DOCKER_QUERY_PARAMS.query];
    return typeof value === 'string' ? value : '';
  };
  const setSearchFilter = (value: string) =>
    setSearchParams({ [DOCKER_QUERY_PARAMS.query]: value || null }, { replace: true });
  const statusFilter = (): DockerResourceStatusFilter => {
    const value = searchParams[DOCKER_QUERY_PARAMS.status];
    return value === 'online' || value === 'degraded' || value === 'offline' ? value : 'all';
  };
  const setStatusFilter = (value: DockerResourceStatusFilter) =>
    setSearchParams(
      { [DOCKER_QUERY_PARAMS.status]: value === 'all' ? null : value },
      { replace: true },
    );
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
    externalSearch: searchFilter,
    onExternalSearchChange: setSearchFilter,
    externalStatus: statusFilter,
    onExternalStatusChange: setStatusFilter,
  });

  // Host scope filter, URL-backed so it is shareable and captured by saved
  // views. Distinct hosts are derived from the current resource set; the facet
  // only appears once more than one host is present (a single-host fleet has
  // nothing to scope by).
  const hostFilter = () => {
    const host = searchParams[DOCKER_QUERY_PARAMS.host];
    return typeof host === 'string' ? host : '';
  };
  const setHostFilter = (value: string) =>
    setSearchParams({ [DOCKER_QUERY_PARAMS.host]: value || null });
  const hostOptions = createMemo(() => {
    const seen = new Set<string>();
    for (const resource of props.resources) {
      const host = dockerHostName(resource);
      if (host && host !== '—') seen.add(host);
    }
    return [...seen].sort((a, b) => a.localeCompare(b));
  });
  const scopeFilters = createMemo<FilterDef[]>(() => {
    if (hostOptions().length <= 1) return [];
    return [
      {
        id: 'host',
        label: 'Host',
        group: 'scope',
        options: () => [
          { value: '', label: 'All hosts' },
          ...hostOptions().map((host) => ({ value: host, label: host })),
        ],
        value: hostFilter,
        setValue: setHostFilter,
        defaultValue: '',
      },
    ];
  });
  const scopedRows = createMemo(() => {
    const host = hostFilter();
    const base = tableState.filtered();
    return host ? base.filter((resource) => dockerHostName(resource) === host) : base;
  });
  const hasActiveFilters = () => tableState.hasActiveFilters() || hostFilter() !== '';
  const resetFilters = () => {
    tableState.resetFilters();
    setHostFilter('');
  };
  const sortedRows = createMemo(() => [...scopedRows()].sort(compareDockerContainers));
  // Runtime is host-level data repeated on every container row; it only
  // informs when the fleet actually mixes runtimes (docker vs podman).
  const showRuntimeColumn = createMemo(() => {
    const kinds = new Set<string>();
    for (const resource of props.resources) {
      const kind = (asTrimmedString(resource.docker?.runtime) ?? '').toLowerCase();
      if (kind) kinds.add(kind);
    }
    return kinds.size > 1;
  });
  const showRestartColumn = createMemo(() =>
    scopedRows().some(
      (resource) =>
        typeof resource.docker?.restartCount === 'number' && resource.docker.restartCount > 0,
    ),
  );
  const showStateColumn = createMemo(() =>
    scopedRows().some((resource) => {
      const state = asTrimmedString(resource.docker?.containerState || resource.status);
      return !!state && state.toLowerCase() !== 'running';
    }),
  );
  const visibleColumns = createMemo(() =>
    getDockerContainerVisibleColumnsForLayout(
      layoutMode(),
      showRuntimeColumn(),
      showRestartColumn(),
      showStateColumn(),
    ),
  );
  const visibleColumnIds = createMemo(() => visibleColumns().map((column) => column.id));
  const drawerColSpan = createMemo(() => visibleColumns().length);
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-container-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search containers"
            searchTips={DOCKER_CONTAINER_SEARCH_TIPS}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            filters={scopeFilters()}
            savedViewsKey="docker-containers"
            visible={scopedRows().length}
            total={tableState.total()}
            rowNoun="containers"
            hasActiveFilters={hasActiveFilters()}
            onResetFilters={resetFilters}
          />
        </Show>

        <Show
          when={scopedRows().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No containers match current filters"
              description="Adjust the search, status, or host filter to see more Docker containers."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Containers'}
            tableClass={`${getDockerContainerTableMinWidthClass()} table-fixed text-xs`}
            colgroup={
              <colgroup>
                <For each={visibleColumns()}>
                  {(column) => (
                    <col
                      style={getDockerContainerColumnWidthStyle(
                        column.id,
                        layoutMode(),
                        visibleColumnIds(),
                      )}
                    />
                  )}
                </For>
              </colgroup>
            }
            header={
              <>
                <For each={visibleColumns()}>
                  {(column) => (
                    <TableHead class={getPlatformTableHeadClassForKind(column.kind)}>
                      <Show when={column.id === 'memory'} fallback={<>{column.label}</>}>
                        <span class="md:hidden">Mem</span>
                        <span class="hidden md:inline">Memory</span>
                      </Show>
                    </TableHead>
                  )}
                </For>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(resource) => {
                    const indicator = mapDockerContainerStatus(resource);
                    const image = () => dockerTextValue(resource.docker?.image);
                    const state = () => containerState(resource);
                    const runtime = () => runtimeSummary(resource);
                    const host = () => dockerHostName(resource);
                    const running = () => isContainerRunning(resource);
                    const metricsKey = () => buildMetricKeyForUnifiedResource(resource);
                    const cpuPercent = () => getPlatformTableFiniteMetric(resource.cpu?.current);
                    const memoryUsed = () =>
                      getPlatformTableFiniteMetric(resource.memory?.used) ?? 0;
                    const memoryTotal = () =>
                      getPlatformTableFiniteMetric(resource.memory?.total) ?? 0;
                    const memoryPercentOnly = () =>
                      memoryTotal() > 0
                        ? undefined
                        : getPlatformTableFiniteMetric(resource.memory?.current);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const restartCount = () => resource.docker?.restartCount ?? 0;
                    const ports = () => dockerContainerPortsSummary(resource);
                    const networks = () => networksSummary(resource);
                    const mounts = () => mountsSummary(resource);
                    const updates = () => updateStatusLabel(resource);
                    const updateTitle = () => updateStatusTitle(resource);
                    const action = containerUpdateAction(resource);
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);
                    const renderColumnCell = (column: DockerContainerTableColumn): JSX.Element => {
                      switch (column.id) {
                        case 'container':
                          return (
                            <DockerResourceNameCell
                              resource={resource}
                              indicator={indicator}
                              detailToggle={
                                <PlatformResourceDetailToggleButton
                                  expanded={isExpanded()}
                                  resourceLabel={dockerResourceName(resource)}
                                  controlsId={detailRowId()}
                                  onToggle={() => drawer.toggle(resource)}
                                />
                              }
                            />
                          );
                        case 'host':
                          return (
                            <TableCell
                              class={`${getPlatformTableCellClassForKind(column.kind)} text-base-content`}
                            >
                              <span class="block max-w-full truncate" title={host()}>
                                {host()}
                              </span>
                            </TableCell>
                          );
                        case 'runtime':
                          return (
                            <TableCell
                              class={`${getPlatformTableCellClassForKind(column.kind)} text-base-content`}
                            >
                              <span class="block max-w-full truncate" title={runtime()}>
                                {runtime()}
                              </span>
                            </TableCell>
                          );
                        case 'image':
                          return (
                            <TableCell
                              class={`${getPlatformTableCellClassForKind(column.kind)} text-base-content`}
                            >
                              <span class="block max-w-full truncate" title={image()}>
                                {image()}
                              </span>
                            </TableCell>
                          );
                        case 'state':
                          return (
                            <TableCell
                              class={`${getPlatformTableCellClassForKind(column.kind)} text-base-content`}
                            >
                              <span class="block max-w-full truncate" title={state()}>
                                {state()}
                              </span>
                            </TableCell>
                          );
                        case 'cpu':
                          return (
                            <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                              <ResponsiveMetricCell
                                class="w-full"
                                value={cpuPercent() ?? 0}
                                type="cpu"
                                resourceId={metricsKey()}
                                isRunning={running() && cpuPercent() !== undefined}
                                showMobile={false}
                              />
                            </TableCell>
                          );
                        case 'memory':
                          return (
                            <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                              <Show
                                when={running() && hasMemoryMetric()}
                                fallback={<PlatformTableMetricFallback />}
                              >
                                <StackedMemoryBar
                                  used={memoryUsed()}
                                  total={memoryTotal()}
                                  percentOnly={memoryPercentOnly()}
                                />
                              </Show>
                            </TableCell>
                          );
                        case 'restarts':
                          return (
                            <TableCell
                              class={`${getPlatformTableCellClassForKind(column.kind)} text-base-content`}
                            >
                              <Show
                                when={typeof resource.docker?.restartCount === 'number'}
                                fallback={<span>—</span>}
                              >
                                <span
                                  class={`tabular-nums ${
                                    restartCount() > RESTART_ATTENTION_THRESHOLD
                                      ? 'font-medium text-red-600 dark:text-red-400'
                                      : ''
                                  }`}
                                >
                                  {resource.docker?.restartCount}
                                </span>
                              </Show>
                            </TableCell>
                          );
                        case 'ports':
                          return (
                            <TableCell
                              class={`${getPlatformTableCellClassForKind(column.kind)} text-base-content`}
                            >
                              <span
                                class="block max-w-full truncate font-mono text-[11px]"
                                title={ports()}
                              >
                                {ports()}
                              </span>
                            </TableCell>
                          );
                        case 'networks':
                          return (
                            <TableCell
                              class={`${getPlatformTableCellClassForKind(column.kind)} text-base-content`}
                            >
                              <span class="block max-w-full truncate" title={networks()}>
                                {networks()}
                              </span>
                            </TableCell>
                          );
                        case 'mounts':
                          return (
                            <TableCell
                              class={`${getPlatformTableCellClassForKind(column.kind)} text-base-content`}
                            >
                              <span class="block max-w-full truncate" title={mounts()}>
                                {mounts()}
                              </span>
                            </TableCell>
                          );
                        case 'updates':
                          return (
                            <TableCell
                              class={`${getPlatformTableCellClassForKind(column.kind)} text-base-content`}
                            >
                              <Show
                                when={action}
                                fallback={
                                  <span class="block max-w-full truncate" title={updateTitle()}>
                                    {updates()}
                                  </span>
                                }
                              >
                                {(updateAction) => (
                                  <UpdateButton
                                    agentId={updateAction().agentId}
                                    containerId={updateAction().containerId}
                                    containerName={updateAction().containerName}
                                    updateStatus={updateAction().updateStatus}
                                  />
                                )}
                              </Show>
                            </TableCell>
                          );
                        case 'actions':
                          return (
                            <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                              <DockerContainerLifecycleControls
                                resource={resource}
                                onActionSettled={props.onLifecycleActionSettled}
                              />
                            </TableCell>
                          );
                        default:
                          column.id satisfies never;
                          return <></>;
                      }
                    };

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-docker-container-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                          onKeyDown={drawer.handleActivationKey(resource)}
                          tabIndex={0}
                        >
                          <For each={visibleColumns()}>{(column) => renderColumnCell(column)}</For>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={drawerColSpan()}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(resource)}
                          onResourceActionSettled={props.onLifecycleActionSettled}
                        />
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

export default DockerContainersTable;
