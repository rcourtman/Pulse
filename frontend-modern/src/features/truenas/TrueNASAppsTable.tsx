import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { getShortImageName } from '@/utils/format';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformTableFilterOption,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource, ResourceTrueNASAppMeta, ResourceTrueNASAppPort } from '@/types/resource';
import { filterTrueNASApps, type TrueNASAppStatusFilter } from './truenasPageModel';

const TRUENAS_APP_STATUS_OPTIONS: PlatformTableFilterOption<TrueNASAppStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running', tone: 'success' },
  { value: 'attention', label: 'Attention', tone: 'warning' },
  { value: 'stopped', label: 'Stopped', tone: 'danger' },
];

const metricFallback = () => (
  <div class="flex justify-center">
    <span class="text-xs text-muted" aria-hidden="true">
      -
    </span>
  </div>
);

const finiteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const titleCase = (value: string | undefined): string => {
  const normalized = asTrimmedString(value);
  if (!normalized) return 'Unknown';
  return normalized.charAt(0).toUpperCase() + normalized.slice(1).toLowerCase();
};

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
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title="Apps" />
            <Table class="min-w-full table-fixed text-xs md:min-w-[960px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[17%]`}>
                    App
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                  >
                    Version
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[9%]`}>
                    CPU
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[12%]`}>
                    Memory
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden sm:table-cell md:w-[12%]`}
                  >
                    Containers
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[18%]`}
                  >
                    Ports
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[12%]`}
                  >
                    Images
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[10%]`}>
                    Updates
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(resource) => {
                    const app = () => appMeta(resource);
                    const name = () =>
                      asTrimmedString(app()?.name) ||
                      asTrimmedString(resource.displayName) ||
                      asTrimmedString(resource.name) ||
                      resource.id;
                    const indicator = () => getSimpleStatusIndicator(resource.status);
                    const metricsKey = () => buildMetricKeyForUnifiedResource(resource);
                    const cpuPercent = () => finiteMetric(resource.cpu?.current);
                    const memoryTotal = () => finiteMetric(resource.memory?.total) ?? 0;
                    const memoryUsed = () => finiteMetric(resource.memory?.used) ?? 0;
                    const memoryPercentOnly = () =>
                      memoryTotal() > 0 ? undefined : finiteMetric(resource.memory?.current);
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
                                  {titleCase(app()?.state)} on {resource.parentName || 'TrueNAS'}
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
                            <Show when={cpuPercent() !== undefined} fallback={metricFallback()}>
                              <ResponsiveMetricCell
                                value={cpuPercent()!}
                                type="cpu"
                                resourceId={metricsKey()}
                                isRunning={canRenderMetrics()}
                                fallback={metricFallback()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                            <Show when={hasMemoryMetric()} fallback={metricFallback()}>
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
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

export default TrueNASAppsTable;
