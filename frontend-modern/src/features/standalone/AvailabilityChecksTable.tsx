import { A } from '@solidjs/router';
import {
  Show,
  createMemo,
  createResource,
  onCleanup,
  onMount,
  type Component,
  type JSX,
} from 'solid-js';
import PlusIcon from 'lucide-solid/icons/plus';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { FilterSegmentedControl } from '@/components/shared/FilterToolbar';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformResponsiveTableLabel,
  PlatformTableDurationValue,
  PlatformTableEmptyState,
  PlatformTableRelativeTimeValue,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  filterPlatformResources,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformResourceStatusFilter,
  PlatformTableShell,
  PlatformWindowedRows,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailTableRow,
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import { AvailabilityHistoryAPI } from '@/api/availabilityHistory';
import {
  getAvailabilityProbeEndpointLabel,
  getAvailabilityProbePresentation,
} from '@/utils/availabilityProbePresentation';
import { getProbeSourceChipLabel, type ProbeAgentOption } from '@/utils/availabilityProbeAgents';
import { buildAvailabilityTargetAddPath } from '@/components/Settings/availabilitySettingsModel';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import {
  getStandaloneResourceStatusIndicator,
  sortStandaloneResourcesByAttention,
} from './standalonePageModel';
import { AvailabilityFleetView, getAvailabilityUptimeCell } from './AvailabilityFleetView';

export type AvailabilityChecksView = 'table' | 'fleet';

// The fleet presentation exists to keep an estate-sized availability list
// scannable. Small inventories still benefit from the table's extra columns,
// while 20+ checks match the first scale at which operators have reported the
// table becoming difficult to scan. An explicit route value always wins so a
// chosen presentation remains shareable and stable across refreshes.
export const AVAILABILITY_FLEET_DEFAULT_MIN_CHECKS = 20;

export const resolveAvailabilityChecksView = (
  routeValue: string | string[] | undefined,
  checkCount: number,
): AvailabilityChecksView => {
  const selectedValue = Array.isArray(routeValue) ? routeValue[0] : routeValue;
  if (selectedValue === 'table' || selectedValue === 'fleet') return selectedValue;
  return checkCount >= AVAILABILITY_FLEET_DEFAULT_MIN_CHECKS ? 'fleet' : 'table';
};

const settingsLinkClass =
  'inline-flex min-h-8 items-center justify-center gap-1.5 rounded-md border border-border bg-surface px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover';

const availabilityFor = (resource: Resource): ResourceAvailabilityMeta | undefined =>
  resource.availability ??
  (resource.platformData?.availability as ResourceAvailabilityMeta | undefined);

const formatTarget = (resource: Resource): string => {
  const availability = availabilityFor(resource);
  if (!availability) return resource.name;
  return getAvailabilityProbeEndpointLabel(availability) || resource.name;
};

// "3/2" read as a fraction. Say what it counts and, on hover, when it turns
// into offline.
const formatFailures = (
  availability: ResourceAvailabilityMeta | undefined,
): { label: string; title?: string } => {
  const failures = availability?.consecutiveFailures;
  if (typeof failures !== 'number' || !Number.isFinite(failures) || failures <= 0) {
    return { label: '—' };
  }
  const threshold = availability?.failureThreshold;
  const title =
    typeof threshold === 'number' && Number.isFinite(threshold) && threshold > 0
      ? `${failures} failed ${failures === 1 ? 'check' : 'checks'} in a row. It counts as offline after ${threshold}.`
      : undefined;
  return { label: `${failures} in a row`, title };
};

const AVAILABILITY_HISTORY_REFRESH_MS = 5 * 60 * 1000;

const UPTIME_TONE_CLASS = {
  default: 'text-base-content',
  warning: 'text-amber-700 dark:text-amber-300',
  danger: 'text-red-600 dark:text-red-300',
  muted: 'text-muted',
} as const;

export const AvailabilityChecksTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  /** Connected agent hosts, used to name the source of probe-reported results. */
  probeAgentOptions?: readonly ProbeAgentOption[];
  view?: AvailabilityChecksView;
  onViewChange?: (view: AvailabilityChecksView) => void;
  externalSearch?: () => string;
  onExternalSearchChange?: (value: string) => void;
  externalStatus?: () => PlatformResourceStatusFilter;
  onExternalStatusChange?: (status: PlatformResourceStatusFilter) => void;
  onResetFilters?: () => void;
}> = (props) => {
  // Rows stay mounted while a stalled probe's lastChecked stops changing, so
  // the stale status, its filter bucket and the probe detail read the shared
  // clock.
  const now = useRelativeTimeNow();
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: (resources, search, status) =>
      filterPlatformResources(resources, search, status, (resource) => {
        const variant = getStandaloneResourceStatusIndicator(resource, now()).variant;
        if (variant === 'success') return 'online';
        if (variant === 'danger') return 'offline';
        return 'degraded';
      }),
    externalSearch: props.externalSearch,
    onExternalSearchChange: props.onExternalSearchChange,
    externalStatus: props.externalStatus,
    onExternalStatusChange: props.onExternalStatusChange,
  });
  const resetFilters = () => {
    if (props.onResetFilters) {
      props.onResetFilters();
      return;
    }
    tableState.resetFilters();
  };
  const orderedChecks = createMemo(() =>
    sortStandaloneResourcesByAttention(tableState.filtered(), now()),
  );
  const historyTargetIDs = createMemo(() =>
    props.resources
      .map((resource) => availabilityFor(resource)?.targetId)
      .filter((targetID): targetID is string => Boolean(targetID)),
  );
  // The table's Uptime 24h column and the fleet view read the same history.
  const historySource = createMemo(() =>
    historyTargetIDs().length > 0 ? historyTargetIDs().join('\u0000') : undefined,
  );
  const [history, historyActions] = createResource(historySource, async () => {
    try {
      return {
        response: await AvailabilityHistoryAPI.batch(historyTargetIDs(), '24h'),
        error: undefined,
      };
    } catch (error) {
      return {
        response: undefined,
        error: error instanceof Error ? error.message : 'Availability history is unavailable',
      };
    }
  });
  // Uptime moves as probes run, and a failed read should not stick: refresh
  // the shared history on a slow cadence while the table is mounted.
  onMount(() => {
    const timer = window.setInterval(() => {
      if (historySource()) void historyActions.refetch();
    }, AVAILABILITY_HISTORY_REFRESH_MS);
    onCleanup(() => window.clearInterval(timer));
  });
  const historyByTarget = createMemo(
    () => new Map((history()?.response?.targets ?? []).map((target) => [target.targetId, target])),
  );
  const drawer = createPlatformResourceDetailState({ idPrefix: 'availability-check-detail' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
          actions={
            <A href={buildAvailabilityTargetAddPath('service')} class={settingsLinkClass}>
              <PlusIcon class="h-3.5 w-3.5" />
              Add service/device check
            </A>
          }
        />
      }
    >
      <div class="space-y-3">
        <PlatformTableToolbar
          search={tableState.search}
          onSearchChange={tableState.setSearch}
          searchPlaceholder="Search availability checks"
          searchSuggestions={tableState.searchSuggestions}
          status={tableState.status()}
          onStatusChange={tableState.setStatus}
          statusOptions={withPlatformStatusCounts(
            PLATFORM_HEALTH_FILTER_OPTIONS,
            tableState.countForStatus,
          )}
          visible={tableState.visible()}
          total={tableState.total()}
          rowNoun="checks"
          hasActiveFilters={tableState.hasActiveFilters()}
          onResetFilters={resetFilters}
          viewOptions={
            <div>
              <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
                Availability presentation
              </div>
              <FilterSegmentedControl
                aria-label="Availability presentation"
                value={props.view ?? 'table'}
                onChange={(value) => props.onViewChange?.(value as AvailabilityChecksView)}
                options={[
                  { value: 'table', label: 'Table' },
                  { value: 'fleet', label: 'Fleet' },
                ]}
              />
            </div>
          }
        />

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No checks match current filters"
              description="Adjust the search or health filter to see more availability checks."
            />
          }
        >
          <Show
            when={(props.view ?? 'table') === 'table'}
            fallback={
              <AvailabilityFleetView
                resources={orderedChecks()}
                historyByTarget={historyByTarget()}
                historyLoading={history.loading}
                historyError={history()?.error}
                probeAgentOptions={props.probeAgentOptions}
                onRetryHistory={() => void historyActions.refetch()}
              />
            }
          >
            <PlatformTableShell
              title="Availability checks"
              actions={
                <div class="flex flex-wrap items-center justify-end gap-2">
                  <A href={buildAvailabilityTargetAddPath('service')} class={settingsLinkClass}>
                    <PlusIcon class="h-3.5 w-3.5" />
                    Add service/device check
                  </A>
                </div>
              }
              tableClass="min-w-full table-fixed text-xs md:min-w-[900px]"
              header={
                <>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 md:w-[20%]`}
                  >
                    Check
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15 md:w-[12%]`}
                  >
                    Method
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-25 md:w-[22%]`}
                  >
                    Target
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[12%]`}
                  >
                    Result
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden lg:table-cell lg:w-[9%]`}
                    title="Share of the last 24 hours the check was reachable"
                  >
                    Uptime 24h
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[10%]`}
                  >
                    <PlatformResponsiveTableLabel compact="Seen" full="Checked" />
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden lg:table-cell lg:w-[10%]`}
                  >
                    Last healthy
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden lg:table-cell lg:w-[8%]`}
                  >
                    Failures
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden 2xl:table-cell 2xl:w-[7%]`}
                  >
                    Interval
                  </TableHead>
                </>
              }
              body={
                <PlatformWindowedRows items={orderedChecks} estimatedRowHeight={32}>
                  {(check) => {
                    const availability = () => availabilityFor(check);
                    const probe = () => getAvailabilityProbePresentation(check, new Date(now()));
                    const indicator = () => getStandaloneResourceStatusIndicator(check, now());
                    const method = () =>
                      probe()?.methodLabel ?? availability()?.protocol ?? 'Probe';
                    const result = () => probe()?.resultLabel ?? indicator().label;
                    const target = () => formatTarget(check);
                    const probeSource = () =>
                      (availability()?.locations?.length ?? 0) > 1
                        ? `${availability()?.locations?.length} locations · ${availability()?.reportingLocations ?? 0}/${availability()?.expectedLocations ?? availability()?.locations?.length ?? 0} reporting`
                        : getProbeSourceChipLabel(
                            props.probeAgentOptions ?? [],
                            availability()?.probeAgentId,
                          );
                    const detailRowId = () => drawer.detailRowId(check);
                    const isExpanded = () => drawer.isExpanded(check);

                    return (
                      <>
                        <TableRow
                          data-availability-check-row={check.id}
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          onClick={() => drawer.toggle(check)}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={check.name}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(check)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={indicator().label}
                                ariaHidden
                              />
                              <span
                                class="truncate font-semibold text-base-content"
                                title={check.name}
                              >
                                {check.name}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {method()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span class="block truncate" title={target()}>
                              {target()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <span class={probe()?.toneClassName ?? ''} title={probe()?.detailLabel}>
                              {result()}
                            </span>
                            <Show when={probeSource()}>
                              {(sourceLabel) => (
                                <MetadataBadge
                                  tone="muted"
                                  size="xs"
                                  appearance="outline"
                                  class="mt-0.5 flex"
                                  data-availability-probe-source={availability()?.probeAgentId}
                                >
                                  {sourceLabel()}
                                </MetadataBadge>
                              )}
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden tabular-nums lg:table-cell`}
                          >
                            {(() => {
                              const uptime = getAvailabilityUptimeCell(
                                historyByTarget().get(availability()?.targetId ?? ''),
                              );
                              return (
                                <span
                                  class={UPTIME_TONE_CLASS[uptime.tone]}
                                  title={uptime.title}
                                  data-availability-uptime
                                >
                                  {uptime.label}
                                </span>
                              );
                            })()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableRelativeTimeValue
                              value={availability()?.lastChecked}
                              emptyText="Not checked"
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content lg:table-cell`}
                          >
                            <PlatformTableRelativeTimeValue
                              value={availability()?.lastSuccess}
                              emptyText="Never"
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content lg:table-cell`}
                          >
                            <span title={formatFailures(availability()).title}>
                              {formatFailures(availability()).label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content 2xl:table-cell`}
                          >
                            <PlatformTableDurationValue
                              seconds={availability()?.pollIntervalSeconds}
                            />
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={check}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={9}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(check)}
                        />
                      </>
                    );
                  }}
                </PlatformWindowedRows>
              }
            />
          </Show>
        </Show>
      </div>
    </Show>
  );
};

export default AvailabilityChecksTable;
