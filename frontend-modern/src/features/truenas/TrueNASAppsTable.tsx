import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getShortImageName } from '@/utils/format';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  PlatformSortableTableHead,
  PlatformTableMetricFallback,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTitleCaseValue,
  getPlatformTableFiniteMetric,
  getPlatformTableCellClassForKind,
  type PlatformTableFilterOption,
  type PlatformTableSortValue,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource, ResourceTrueNASAppMeta, ResourceTrueNASAppPort } from '@/types/resource';
import {
  filterTrueNASApps,
  getTrueNASResourceDisplayStatus,
  type TrueNASAppStatusFilter,
} from './truenasPageModel';

const TRUENAS_APP_STATUS_OPTIONS: PlatformTableFilterOption<TrueNASAppStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running', tone: 'success' },
  { value: 'attention', label: 'Attention', tone: 'warning' },
  { value: 'stopped', label: 'Stopped', tone: 'danger' },
];

const appMeta = (resource: Resource): ResourceTrueNASAppMeta | undefined => resource.truenas?.app;

const appVersionLabel = (app: ResourceTrueNASAppMeta | undefined): string => {
  const human = asTrimmedString(app?.humanVersion);
  const version = asTrimmedString(app?.version);
  if (human && version && human !== version) return `${human} / ${version}`;
  return human || version || '-';
};

const appContainerCount = (app: ResourceTrueNASAppMeta | undefined): number => {
  const explicit = app?.containerCount;
  if (typeof explicit === 'number' && Number.isFinite(explicit)) return explicit;
  return app?.containers?.length ?? 0;
};

const formatHostPort = (port: ResourceTrueNASAppPort): string => {
  const protocol = (asTrimmedString(port.protocol) ?? '').toLowerCase() || 'tcp';
  const target = typeof port.containerPort === 'number' ? String(port.containerPort) : '';
  const targetLabel = target ? `${target}/${protocol}` : protocol;
  const hostPorts = port.hostPorts ?? [];
  if (hostPorts.length === 0) return targetLabel;
  return hostPorts
    .map((hostPort) => {
      const published = typeof hostPort.hostPort === 'number' ? String(hostPort.hostPort) : '';
      const hostIp = asTrimmedString(hostPort.hostIp);
      const source = [hostIp, published].filter(Boolean).join(':');
      return source ? `${source}->${targetLabel}` : targetLabel;
    })
    .join(', ');
};

const formatPorts = (app: ResourceTrueNASAppMeta | undefined): { label: string; title: string } => {
  const ports = app?.usedPorts ?? [];
  const labels = ports.map(formatHostPort).filter(Boolean);
  if (labels.length === 0) return { label: '-', title: '' };
  const visible = labels.slice(0, 2);
  const suffix = labels.length > visible.length ? ` +${labels.length - visible.length}` : '';
  return { label: `${visible.join(', ')}${suffix}`, title: labels.join(', ') };
};

const shortImage = (image: string): string => getShortImageName(image) || image;

const formatImages = (
  app: ResourceTrueNASAppMeta | undefined,
): { label: string; title: string } => {
  const images = app?.images?.filter((image) => asTrimmedString(image)) ?? [];
  if (images.length === 0) return { label: '-', title: '' };
  const labels = images.map(shortImage);
  const visible = labels.slice(0, 2);
  const suffix = labels.length > visible.length ? ` +${labels.length - visible.length}` : '';
  return { label: `${visible.join(', ')}${suffix}`, title: images.join(', ') };
};

const UpdatePills: Component<{ app: ResourceTrueNASAppMeta | undefined }> = (props) => {
  const hasAppUpdate = () => props.app?.upgradeAvailable === true;
  const hasImageUpdate = () => props.app?.imageUpdatesAvailable === true;
  return (
    <div class="flex justify-center gap-1">
      <Show
        when={hasAppUpdate() || hasImageUpdate()}
        fallback={
          <span class="rounded border border-border px-1.5 py-0.5 text-[10px] font-medium text-muted">
            Current
          </span>
        }
      >
        <Show when={hasAppUpdate()}>
          <span class="rounded border border-amber-300/50 bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-300">
            App
          </span>
        </Show>
        <Show when={hasImageUpdate()}>
          <span class="rounded border border-blue-300/50 bg-blue-500/10 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:text-blue-300">
            Image
          </span>
        </Show>
      </Show>
    </div>
  );
};

// Columns a user can sort by. Ports and Images summarize several values at
// once, so they carry no single scalar to order on. Updates orders on how
// many update kinds (app / image) are pending so outdated apps surface first.
const TRUENAS_APP_SORT_KEYS = [
  'app',
  'version',
  'cpu',
  'memory',
  'containers',
  'updates',
] as const;

type TrueNASAppSortKey = (typeof TRUENAS_APP_SORT_KEYS)[number];

const getTrueNASAppSortValue = (
  resource: Resource,
  key: TrueNASAppSortKey,
): PlatformTableSortValue => {
  const app = appMeta(resource);
  switch (key) {
    case 'app':
      return (
        asTrimmedString(app?.name) ||
        asTrimmedString(resource.displayName) ||
        asTrimmedString(resource.name) ||
        resource.id
      );
    case 'version': {
      const version = appVersionLabel(app);
      return version === '-' ? null : version;
    }
    case 'cpu':
      return getPlatformTableFiniteMetric(resource.cpu?.current) ?? null;
    case 'memory': {
      const total = getPlatformTableFiniteMetric(resource.memory?.total) ?? 0;
      if (total > 0) {
        return ((getPlatformTableFiniteMetric(resource.memory?.used) ?? 0) / total) * 100;
      }
      return getPlatformTableFiniteMetric(resource.memory?.current) ?? null;
    }
    case 'containers':
      return appContainerCount(app);
    case 'updates':
      return (app?.upgradeAvailable === true ? 1 : 0) + (app?.imageUpdatesAvailable === true ? 1 : 0);
    default:
      key satisfies never;
      return null;
  }
};

export const TrueNASAppsTable: Component<{
  apps: Resource[];
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.apps,
    initialStatus: 'all' as TrueNASAppStatusFilter,
    filter: filterTrueNASApps,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'truenas-app-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);
  const sort = createPlatformTableSortState({
    storageKey: 'truenasApps',
    sortKeys: TRUENAS_APP_SORT_KEYS,
    descendingFirst: ['cpu', 'memory', 'containers', 'updates'],
  });
  const sortedRows = createMemo(() => sort.sortRows(tableState.filtered(), getTrueNASAppSortValue));

  return (
    <Show
      when={props.apps.length > 0}
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
            searchPlaceholder="Search TrueNAS apps"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={TRUENAS_APP_STATUS_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="apps"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No apps match current filters"
              description="Adjust the search or status filter to see more TrueNAS apps."
            />
          }
        >
          <PlatformTableShell
            title="Apps"
            tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
            header={
              <>
                <PlatformSortableTableHead kind="name" sort={sort} sortKey="app" class="md:w-[17%]">
                  App
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="version"
                  class="hidden md:table-cell md:w-[10%]"
                >
                  Version
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="cpu"
                  class="md:w-[9%]"
                >
                  CPU
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="memory"
                  class="md:w-[12%]"
                >
                  Memory
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="containers"
                  class="hidden sm:table-cell md:w-[12%]"
                >
                  Containers
                </PlatformSortableTableHead>
                <PlatformSortableTableHead kind="text" sort={sort} class="hidden md:table-cell md:w-[18%]">
                  Ports
                </PlatformSortableTableHead>
                <PlatformSortableTableHead kind="text" sort={sort} class="hidden lg:table-cell md:w-[12%]">
                  Images
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="updates"
                  class="md:w-[10%]"
                >
                  Updates
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(resource) => {
                    const app = () => appMeta(resource);
                    const name = () =>
                      asTrimmedString(app()?.name) ||
                      asTrimmedString(resource.displayName) ||
                      asTrimmedString(resource.name) ||
                      resource.id;
                    const displayStatus = () => getTrueNASResourceDisplayStatus(resource);
                    const indicator = () => getSimpleStatusIndicator(displayStatus());
                    const metricsKey = () => buildMetricKeyForUnifiedResource(resource);
                    const cpuPercent = () => getPlatformTableFiniteMetric(resource.cpu?.current);
                    const memoryTotal = () =>
                      getPlatformTableFiniteMetric(resource.memory?.total) ?? 0;
                    const memoryUsed = () =>
                      getPlatformTableFiniteMetric(resource.memory?.used) ?? 0;
                    const memoryPercentOnly = () =>
                      memoryTotal() > 0
                        ? undefined
                        : getPlatformTableFiniteMetric(resource.memory?.current);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const canRenderMetrics = () => indicator().variant !== 'muted';
                    const ports = createMemo(() => formatPorts(app()));
                    const images = createMemo(() => formatImages(app()));
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-truenas-app-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                          onKeyDown={drawer.handleActivationKey(resource)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(resource)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={indicator().label}
                              />
                              <div class="min-w-0">
                                <div class="truncate font-medium text-base-content" title={name()}>
                                  {name()}
                                </div>
                                <div class="truncate text-[10px] text-muted">
                                  {formatPlatformTableTitleCaseValue(app()?.state)} on{' '}
                                  {resource.parentName || 'TrueNAS'}
                                  <Show when={app()?.customApp}> - Custom</Show>
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden md:table-cell`}
                          >
                            <span class="block truncate" title={appVersionLabel(app())}>
                              {appVersionLabel(app())}
                            </span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                            <Show
                              when={cpuPercent() !== undefined}
                              fallback={<PlatformTableMetricFallback />}
                            >
                              <ResponsiveMetricCell
                                value={cpuPercent()!}
                                type="cpu"
                                resourceId={metricsKey()}
                                isRunning={canRenderMetrics()}
                                fallback={<PlatformTableMetricFallback />}
                              />
                            </Show>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                            <Show
                              when={hasMemoryMetric()}
                              fallback={<PlatformTableMetricFallback />}
                            >
                              <StackedMemoryBar
                                used={memoryUsed()}
                                total={memoryTotal()}
                                percentOnly={memoryPercentOnly()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden sm:table-cell tabular-nums`}
                          >
                            {appContainerCount(app()) || '-'}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden md:table-cell`}
                          >
                            <span
                              class="block truncate font-mono text-[11px]"
                              title={ports().title}
                            >
                              {ports().label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden lg:table-cell`}
                          >
                            <span class="block truncate" title={images().title}>
                              {images().label}
                            </span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <UpdatePills app={app()} />
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={8}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(resource)}
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

export default TrueNASAppsTable;
