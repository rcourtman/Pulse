import { Show, createMemo, type Component, type JSX } from 'solid-js';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getShortImageName } from '@/utils/format';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { unifiedPlatformOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
import {
  PlatformWindowedRows,
  PlatformSortableTableHead,
  PlatformResponsiveTableLabel,
  PlatformTableMetricFallback,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTitleCaseValue,
  getPlatformTableFiniteMetric,
  getPlatformTableCellClassForKind,
  summarizePlatformTableValues,
  type PlatformTableFilterOption,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import {
  TrueNASResourceDetailRow,
  TrueNASRowLead,
  TrueNASTableFrame,
  createTrueNASDetailState,
  getTrueNASMemoryDisplay,
  getTrueNASMemoryPercentSortValue,
} from './TrueNASTableFrame';
import {
  getPlatformInlineStateClass,
  getPlatformStatusTextToneClass,
  isPlatformAttentionVariant,
} from '@/features/platformPage/platformStatusTone';
import {
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource, ResourceTrueNASAppMeta, ResourceTrueNASAppPort } from '@/types/resource';
import {
  filterTrueNASApps,
  getTrueNASResourceDisplayStatus,
  getTrueNASStaleNotice,
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

const formatPorts = (app: ResourceTrueNASAppMeta | undefined): { label: string; title: string } =>
  summarizePlatformTableValues((app?.usedPorts ?? []).map(formatHostPort), {
    emptyText: '—',
    maxVisible: 2,
  });

const formatImages = (
  app: ResourceTrueNASAppMeta | undefined,
): { label: string; title: string } => {
  const images = app?.images?.filter((image) => asTrimmedString(image)) ?? [];
  const summary = summarizePlatformTableValues(
    images.map((image) => getShortImageName(image) || image),
    { emptyText: '—', maxVisible: 2 },
  );
  return { label: summary.label, title: images.join(', ') };
};

const UpdatePills: Component<{ app: ResourceTrueNASAppMeta | undefined }> = (props) => {
  const hasAppUpdate = () => props.app?.upgradeAvailable === true;
  const hasImageUpdate = () => props.app?.imageUpdatesAvailable === true;
  return (
    <div class="flex justify-center gap-1">
      <Show
        when={hasAppUpdate() || hasImageUpdate()}
        fallback={
          <span class="rounded-sm border border-border px-1.5 py-0.5 text-[10px] font-medium text-muted">
            Current
          </span>
        }
      >
        <Show when={hasAppUpdate()}>
          <span
            class="rounded-sm border border-amber-300/50 bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-300"
            title="App update available"
          >
            <PlatformResponsiveTableLabel compact="A" full="App" />
          </span>
        </Show>
        <Show when={hasImageUpdate()}>
          <span
            class="rounded-sm border border-blue-300/50 bg-blue-500/10 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:text-blue-300"
            title="Image update available"
          >
            <PlatformResponsiveTableLabel compact="I" full="Image" />
          </span>
        </Show>
      </Show>
    </div>
  );
};

const APP_ATTENTION_STATES = new Set(['crashed', 'deploying', 'stopping', 'error', 'failed']);
const APP_DANGER_STATES = new Set(['crashed', 'error', 'failed']);

const appStatePresentation = (
  resource: Resource,
  app: ResourceTrueNASAppMeta | undefined,
  fallbackVariant: ReturnType<typeof getSimpleStatusIndicator>['variant'],
  fallbackLabel: string,
): {
  label: string;
  title?: string;
  variant: ReturnType<typeof getSimpleStatusIndicator>['variant'];
} => {
  const rawState = formatPlatformTableTitleCaseValue(app?.state, '');
  const normalized = (asTrimmedString(app?.state) ?? '').toLowerCase();
  const stale = getTrueNASStaleNotice(resource);
  if (stale) {
    if (APP_ATTENTION_STATES.has(normalized)) {
      return {
        label: `${rawState} · ${stale.badge}`,
        title: `Last reported ${rawState} · ${stale.detail}`,
        variant: APP_DANGER_STATES.has(normalized) ? 'danger' : 'warning',
      };
    }
    return {
      label: stale.badge,
      title: rawState ? `Last reported ${rawState} · ${stale.detail}` : stale.detail,
      variant: fallbackVariant === 'danger' ? 'danger' : 'warning',
    };
  }
  return {
    label: rawState || fallbackLabel,
    variant: APP_DANGER_STATES.has(normalized) ? 'danger' : fallbackVariant,
  };
};

// Columns a user can sort by. Ports and Images summarize several values at
// once, so they carry no single scalar to order on. Updates orders on how
// many update kinds (app / image) are pending so outdated apps surface first.
const TRUENAS_APP_SORT_KEYS = ['app', 'version', 'cpu', 'memory', 'containers', 'updates'] as const;

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
    case 'memory':
      return getTrueNASMemoryPercentSortValue(resource);
    case 'containers':
      return appContainerCount(app);
    case 'updates':
      return (
        (app?.upgradeAvailable === true ? 1 : 0) + (app?.imageUpdatesAvailable === true ? 1 : 0)
      );
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
  /** The page's URL search, when it owns one; the table then reads and writes it. */
  search?: () => string;
  onSearchChange?: (value: string) => void;
}> = (props) => {
  const alertsActivation = useAlertsActivation();
  const tableState = createPlatformTableFilterState({
    resources: () => props.apps,
    initialStatus: 'all' as TrueNASAppStatusFilter,
    filter: filterTrueNASApps,
    externalSearch: props.search,
    onExternalSearchChange: props.onSearchChange,
  });
  const drawer = createTrueNASDetailState('truenas-app-drawer');
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);
  const sort = createPlatformTableSortState({
    storageKey: 'truenasApps',
    sortKeys: TRUENAS_APP_SORT_KEYS,
    descendingFirst: ['cpu', 'memory', 'containers', 'updates'],
  });
  const sortedRows = createMemo(() => sort.sortRows(tableState.filtered(), getTrueNASAppSortValue));

  return (
    <TrueNASTableFrame
      hasItems={props.apps.length > 0}
      tableState={tableState}
      common={props}
      searchPlaceholder="Search TrueNAS apps"
      statusOptions={TRUENAS_APP_STATUS_OPTIONS}
      rowNoun="apps"
      title="Apps"
      tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
      header={
        <>
          <PlatformSortableTableHead
            kind="name"
            sort={sort}
            sortKey="app"
            class="platform-table-mobile-w-30 truenas-app-name-column md:w-[17%]"
          >
            App
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="text"
            sort={sort}
            sortKey="version"
            class="truenas-app-version-column hidden md:table-cell md:w-[10%]"
          >
            Version
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="metric-bar"
            sort={sort}
            sortKey="cpu"
            class="platform-table-mobile-w-15 truenas-app-cpu-column md:w-[9%]"
          >
            CPU
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="metric-bar"
            sort={sort}
            sortKey="memory"
            class="platform-table-mobile-w-20 truenas-app-memory-column md:w-[12%]"
          >
            <PlatformResponsiveTableLabel compact="Mem" full="Memory" />
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="numeric-value"
            sort={sort}
            sortKey="containers"
            class="platform-table-mobile-w-15 truenas-app-containers-column md:w-[12%]"
          >
            <PlatformResponsiveTableLabel compact="Ctrs" full="Containers" />
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="text"
            sort={sort}
            class="truenas-app-ports-column hidden md:table-cell md:w-[18%]"
          >
            Ports
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="text"
            sort={sort}
            class="truenas-app-images-column hidden lg:table-cell md:w-[12%]"
          >
            Images
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="badge"
            sort={sort}
            sortKey="updates"
            class="platform-table-mobile-w-20 truenas-app-updates-column md:w-[10%]"
          >
            <PlatformResponsiveTableLabel compact="Upd." full="Updates" />
          </PlatformSortableTableHead>
        </>
      }
      body={
        <>
          <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
            {(resource) => {
              const app = () => appMeta(resource);
              const name = () =>
                asTrimmedString(app()?.name) ||
                asTrimmedString(resource.displayName) ||
                asTrimmedString(resource.name) ||
                resource.id;
              const displayStatus = () => getTrueNASResourceDisplayStatus(resource);
              const indicator = () => getSimpleStatusIndicator(displayStatus());
              const appState = () =>
                appStatePresentation(resource, app(), indicator().variant, indicator().label);
              const showAppState = () => isPlatformAttentionVariant(appState().variant);
              const metricsKey = () => buildMetricKeyForUnifiedResource(resource);
              const alertResourceIds = () => unifiedPlatformOverrideIdCandidates(resource);
              const cpuThresholds = () =>
                alertsActivation.getMetricThresholds('truenas', 'cpu', alertResourceIds());
              const memoryThresholds = () =>
                alertsActivation.getMetricThresholds('truenas', 'memory', alertResourceIds());
              const cpuPercent = () => getPlatformTableFiniteMetric(resource.cpu?.current);
              const memory = () => getTrueNASMemoryDisplay(resource);
              const canRenderMetrics = () => indicator().variant !== 'muted';
              const ports = createMemo(() => formatPorts(app()));
              const images = createMemo(() => formatImages(app()));
              const isExpanded = () => drawer.isExpanded(resource);
              return (
                <>
                  <TableRow
                    class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                    data-truenas-app-row={resource.id}
                    onClick={() => drawer.toggle(resource)}
                  >
                    <TableCell class={getPlatformTableCellClassForKind('name')}>
                      <TrueNASRowLead
                        item={resource}
                        drawer={drawer}
                        label={name()}
                        variant={appState().variant}
                        dotTitle={appState().title ?? indicator().label}
                      >
                        <div
                          class={
                            showAppState()
                              ? 'min-w-0 flex-1 sm:flex sm:items-center sm:gap-1.5'
                              : 'flex min-w-0 flex-1 items-center gap-1.5'
                          }
                        >
                          <div
                            class="truenas-app-name-value min-w-0 truncate font-medium text-base-content"
                            title={[
                              name(),
                              `${formatPlatformTableTitleCaseValue(app()?.state)} on ${
                                resource.parentName || 'TrueNAS'
                              }${app()?.customApp ? ' · Custom' : ''}`,
                              appState().title,
                            ]
                              .filter(Boolean)
                              .join(' · ')}
                          >
                            {name()}
                          </div>
                          {/* The app has no State column: a state that needs attention joins
                                    the name in its dot's colour, stacking on narrow phones so
                                    neither name nor stale/crashed label clips. */}
                          <div
                            class={`text-[10px] leading-tight sm:text-[11px] ${getPlatformInlineStateClass(showAppState())} ${getPlatformStatusTextToneClass(appState().variant)}`}
                            title={appState().title}
                            data-truenas-app-state={appState().variant}
                          >
                            {appState().label}
                          </div>
                        </div>
                      </TrueNASRowLead>
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
                          thresholds={cpuThresholds()}
                        />
                      </Show>
                    </TableCell>
                    <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                      <Show when={memory().hasMetric} fallback={<PlatformTableMetricFallback />}>
                        <StackedMemoryBar
                          used={memory().used}
                          total={memory().total}
                          percentOnly={memory().percentOnly}
                          thresholds={memoryThresholds()}
                        />
                      </Show>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('numeric-value')} tabular-nums`}
                    >
                      {appContainerCount(app()) || '-'}
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} hidden md:table-cell`}
                    >
                      <span class="block truncate font-mono text-[11px]" title={ports().title}>
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
                  <TrueNASResourceDetailRow
                    resource={resource}
                    indicator={indicator()}
                    drawer={drawer}
                    colSpan={8}
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

export default TrueNASAppsTable;
