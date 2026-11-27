import { Show, createMemo, type Component } from 'solid-js';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import { TableCell, TableRow } from '@/components/shared/Table';
import {
  PlatformWindowedRows,
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  getPlatformTableCellClassForKind,
  type PlatformTableFilterOption,
  PlatformTableShell,
  type PlatformTableSortValue,
  withPlatformStatusCounts,
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
  DockerStaleDataCue,
  DockerStaleReadingsNotice,
  dockerByteValue,
  dockerHostName,
  dockerResourceName,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import { useDockerHostIdentities } from './dockerHostIdentity';
import {
  applyDockerSourceFreshness,
  filterDockerResources,
  getDockerStaleNotice,
} from './dockerPageModel';

type DockerImageFilter = 'all' | 'updates' | 'in-use' | 'unused';

const DOCKER_IMAGE_FILTER_OPTIONS: PlatformTableFilterOption<DockerImageFilter>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'updates',
    label: 'Updates',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  },
  {
    value: 'in-use',
    label: 'In use',
    tone: 'success',
    leading: filterChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'unused',
    label: 'Unused',
    leading: filterChipStatusDot('bg-slate-400'),
  },
];
import {
  getDockerImageOperationalPresentation,
  type DockerImageUpdateTone,
} from './dockerImagePresentation';
import type { Resource } from '@/types/resource';
import { getSimpleStatusIndicator } from '@/utils/status';

const updateToneClass: Record<DockerImageUpdateTone, string> = {
  danger: 'bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-300',
  warning: 'bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300',
  success: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300',
  muted: 'bg-surface-hover text-muted',
};

const DOCKER_IMAGE_SORT_KEYS = ['image', 'host', 'usedBy', 'size', 'update'] as const;

type DockerImageSortKey = (typeof DOCKER_IMAGE_SORT_KEYS)[number];

export const DockerImagesTable: Component<
  DockerNativeTableProps & { relatedContainers?: Resource[] }
> = (props) => {
  const hostIdentities = useDockerHostIdentities();
  const filterImages = (
    resources: Resource[],
    search: string,
    filter: DockerImageFilter,
  ): Resource[] => {
    const matchingSearch = filterDockerResources(resources, search, 'all');
    if (filter === 'all') return matchingSearch;
    const containers = props.relatedContainers ?? [];
    return matchingSearch.filter((resource) => {
      const presentation = getDockerImageOperationalPresentation(resource, containers);
      if (filter === 'updates') {
        return presentation.updateTone === 'warning' || presentation.updateTone === 'danger';
      }
      if (filter === 'in-use') return presentation.consumerCount > 0;
      return presentation.consumerCount === 0;
    });
  };
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerImageFilter,
    filter: filterImages,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-image-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);
  const sort = createPlatformTableSortState({
    storageKey: 'dockerImages',
    sortKeys: DOCKER_IMAGE_SORT_KEYS,
    descendingFirst: ['size'],
  });
  // Closure rather than a module-level accessor: the operational columns
  // (Used by / Update check) derive from props.relatedContainers.
  const getImageSortValue = (
    resource: Resource,
    key: DockerImageSortKey,
  ): PlatformTableSortValue => {
    switch (key) {
      case 'image':
        return dockerResourceName(resource);
      case 'host': {
        const host = dockerHostName(resource);
        return host === '—' ? null : host;
      }
      case 'usedBy': {
        const summary = getDockerImageOperationalPresentation(
          resource,
          props.relatedContainers ?? [],
        ).consumerSummary;
        return summary && summary !== '—' ? summary : null;
      }
      case 'size':
        return typeof resource.docker?.sizeBytes === 'number' && resource.docker.sizeBytes > 0
          ? resource.docker.sizeBytes
          : null;
      case 'update':
        return getDockerImageOperationalPresentation(resource, props.relatedContainers ?? [])
          .updateLabel;
      default:
        key satisfies never;
        return null;
    }
  };
  const sortedRows = createMemo(() => sort.sortRows(tableState.filtered(), getImageSortValue));

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
            searchPlaceholder="Search image, host, or update state"
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              DOCKER_IMAGE_FILTER_OPTIONS,
              tableState.countForStatus,
            )}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="images"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No images match current filters"
              description="Adjust the search or status filter to see more Docker images."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Images'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[880px]"
            header={
              <>
                {/*
                    The Update check badge needs about 122px with its cell
                    padding ("Update available", "Managed by Pulse"). An
                    equal tablet-band share gave it 100px at 600px, and 16%
                    at md gave it 111px at 768px, so the label was cut off.
                    Size values are short, so the band takes the extra room
                    from Size, and md takes a point each from Host, Used by
                    and Size, which had room to spare there.
                  */}
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="image"
                  class="platform-table-mobile-w-30 md:w-[30%]"
                  bandWidth={30}
                >
                  Image
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="host"
                  class="platform-table-phone-hidden md:w-[17%]"
                  bandWidth={17.5}
                >
                  Host
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="usedBy"
                  class="platform-table-phone-hidden md:w-[23%]"
                  bandWidth={17.5}
                >
                  Used by
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="size"
                  class="platform-table-mobile-w-20 md:w-[11%]"
                  bandWidth={12}
                >
                  Size
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="update"
                  class="platform-table-mobile-w-25 md:w-[19%]"
                  bandWidth={23}
                >
                  <PlatformResponsiveTableLabel compact="Update" full="Update check" />
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
                  {(resource) => {
                    const operational = () =>
                      getDockerImageOperationalPresentation(
                        resource,
                        props.relatedContainers ?? [],
                      );
                    const staleNotice = () => getDockerStaleNotice(resource);
                    const indicator = () =>
                      applyDockerSourceFreshness(
                        resource,
                        getSimpleStatusIndicator(resource.status),
                      );
                    const updateBadgeTone = (): DockerImageUpdateTone => {
                      const tone = operational().updateTone;
                      return staleNotice() && tone === 'success' ? 'muted' : tone;
                    };
                    const updateBadgeTitle = () => {
                      const stale = staleNotice();
                      return stale
                        ? `${operational().updateDetail} ${stale.detail}`
                        : operational().updateDetail;
                    };
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-docker-image-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                        >
                          <DockerResourceNameCell
                            resource={resource}
                            indicator={indicator()}
                            imageReference
                            detail={<DockerStaleDataCue notice={staleNotice()} scope="image" />}
                            detailToggle={
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={dockerResourceName(resource)}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(resource)}
                              />
                            }
                          />
                          {/* Host and consumers fit the row expansion on a phone; the
                              row keeps the image, its size and the update verdict. */}
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content platform-table-phone-hidden`}
                          >
                            {hostIdentities().labelFor(resource)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content platform-table-phone-hidden`}
                          >
                            <span class="block truncate" title={operational().consumerSummary}>
                              {operational().consumerSummary}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {dockerByteValue(resource.docker?.sizeBytes)}
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <Show
                              when={operational().updateTone === 'danger'}
                              fallback={
                                <span
                                  class={`inline-flex rounded-full px-2 py-0.5 text-[10px] font-medium ${updateToneClass[updateBadgeTone()]}`}
                                  title={updateBadgeTitle()}
                                  data-docker-image-update-tone={updateBadgeTone()}
                                >
                                  {operational().updateLabel}
                                </span>
                              }
                            >
                              <button
                                type="button"
                                class={`inline-flex min-h-9 max-w-full items-center rounded-full px-2 py-0.5 text-[10px] font-medium underline decoration-dotted underline-offset-2 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-sky-400/70 sm:min-h-0 ${updateToneClass[operational().updateTone]}`}
                                aria-label={`Check failed for ${dockerResourceName(resource)}. Details`}
                                aria-expanded={isExpanded()}
                                aria-controls={detailRowId()}
                                title={updateBadgeTitle()}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  drawer.toggle(resource);
                                }}
                              >
                                {operational().updateLabel}
                              </button>
                            </Show>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          statusIndicator={indicator()}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={5}
                          resolveResourceLabel={resolveResourceLabel}
                          context={
                            <DockerStaleReadingsNotice notice={staleNotice()} class="mb-3" />
                          }
                          onClose={() => drawer.close(resource)}
                        />
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

export default DockerImagesTable;
