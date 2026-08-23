import { A } from '@solidjs/router';
import { Show, createMemo, type Component, type JSX } from 'solid-js';
import PlusIcon from 'lucide-solid/icons/plus';
import SettingsIcon from 'lucide-solid/icons/settings';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
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
import {
  getAvailabilityProbeEndpointLabel,
  getAvailabilityProbePresentation,
} from '@/utils/availabilityProbePresentation';
import { getProbeSourceChipLabel, type ProbeAgentOption } from '@/utils/availabilityProbeAgents';
import {
  buildAvailabilitySettingsPath,
  buildAvailabilityTargetAddPath,
} from '@/components/Settings/availabilitySettingsModel';
import {
  getStandaloneResourceStatusIndicator,
  sortStandaloneResourcesByAttention,
} from './standalonePageModel';

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

const formatFailures = (availability: ResourceAvailabilityMeta | undefined): string => {
  const failures = availability?.consecutiveFailures;
  if (typeof failures !== 'number' || !Number.isFinite(failures) || failures <= 0) return '—';
  const threshold = availability?.failureThreshold;
  if (typeof threshold === 'number' && Number.isFinite(threshold) && threshold > 0) {
    return `${failures}/${threshold}`;
  }
  return String(failures);
};

export const AvailabilityChecksTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  /** Connected agent hosts, used to name the source of probe-reported results. */
  probeAgentOptions?: readonly ProbeAgentOption[];
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: (resources, search, status) =>
      filterPlatformResources(resources, search, status, (resource) => {
        const variant = getStandaloneResourceStatusIndicator(resource).variant;
        if (variant === 'success') return 'online';
        if (variant === 'danger') return 'offline';
        return 'degraded';
      }),
  });
  const orderedChecks = createMemo(() => sortStandaloneResourcesByAttention(tableState.filtered()));
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
          <PlatformTableShell
            title="Availability checks"
            actions={
              <div class="flex flex-wrap items-center justify-end gap-2">
                <A href={buildAvailabilityTargetAddPath('service')} class={settingsLinkClass}>
                  <PlusIcon class="h-3.5 w-3.5" />
                  Add service/device check
                </A>
                <A href={buildAvailabilitySettingsPath()} class={settingsLinkClass}>
                  <SettingsIcon class="h-3.5 w-3.5" />
                  Manage
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
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden lg:table-cell lg:w-[8%]`}
                >
                  Interval
                </TableHead>
              </>
            }
            body={
              <PlatformWindowedRows items={orderedChecks} estimatedRowHeight={32}>
                {(check) => {
                  const availability = () => availabilityFor(check);
                  const probe = () => getAvailabilityProbePresentation(check);
                  const indicator = () => getStandaloneResourceStatusIndicator(check);
                  const method = () => probe()?.methodLabel ?? availability()?.protocol ?? 'Probe';
                  const result = () => probe()?.resultLabel ?? indicator().label;
                  const target = () => formatTarget(check);
                  const probeSource = () =>
                    getProbeSourceChipLabel(
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
                        aria-controls={isExpanded() ? detailRowId() : undefined}
                        aria-expanded={isExpanded() ? 'true' : 'false'}
                        onClick={() => drawer.toggle(check)}
                        onKeyDown={drawer.handleActivationKey(check)}
                        tabIndex={0}
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
                          {formatFailures(availability())}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content lg:table-cell`}
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
                        colSpan={8}
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
      </div>
    </Show>
  );
};

export default AvailabilityChecksTable;
