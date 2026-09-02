import { Show, createMemo, type Component } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableRelativeTimeValue,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  getPlatformTableCellClassForKind,
  getPlatformTableDateTimeSortValue,
  PlatformTableShell,
  type PlatformTableSortValue,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  DockerNativeDetailPanel,
  DockerResourceNameCell,
  dockerHostName,
  dockerLabelsSummary,
  dockerResourceName,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import { filterDockerResources, type DockerResourceStatusFilter } from './dockerPageModel';
import type { Resource } from '@/types/resource';

const DOCKER_CONFIG_SORT_KEYS = ['config', 'template', 'created', 'host'] as const;

type DockerConfigSortKey = (typeof DOCKER_CONFIG_SORT_KEYS)[number];

const getDockerConfigSortValue = (
  resource: Resource,
  key: DockerConfigSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'config':
      return dockerResourceName(resource);
    case 'template':
      return asTrimmedString(resource.docker?.templatingDriver) || null;
    case 'created':
      return getPlatformTableDateTimeSortValue(resource.docker?.objectCreatedAt);
    case 'host': {
      const host = dockerHostName(resource);
      return host === '—' ? null : host;
    }
    default:
      key satisfies never;
      return null;
  }
};

export const DockerConfigsTable: Component<DockerNativeTableProps> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
  });
  const sort = createPlatformTableSortState({
    storageKey: 'dockerConfigs',
    sortKeys: DOCKER_CONFIG_SORT_KEYS,
    descendingFirst: ['created'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), getDockerConfigSortValue),
  );
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-config-detail' });

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
            searchPlaceholder="Search Swarm configs"
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              PLATFORM_HEALTH_FILTER_OPTIONS,
              tableState.countForStatus,
            )}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="configs"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No Swarm configs match current filters"
              description="Adjust the search or status filter to see more Docker Swarm configs."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Swarm Configs'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1040px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="config"
                  class="platform-table-mobile-w-30 md:w-[28%]"
                >
                  Config
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="template"
                  class="platform-table-phone-hidden md:w-[16%]"
                >
                  Template
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="created"
                  class="platform-table-mobile-w-15 md:w-[16%]"
                >
                  Created
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  class="platform-table-phone-hidden md:w-[24%]"
                >
                  Labels
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="host"
                  class="platform-table-mobile-w-15 md:w-[16%]"
                >
                  Host
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
                  {(resource) => {
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-docker-config-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                        >
                          <DockerResourceNameCell
                            resource={resource}
                            detailToggle={
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={dockerResourceName(resource)}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(resource)}
                              />
                            }
                          />
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden text-base-content`}
                          >
                            {dockerTextValue(resource.docker?.templatingDriver)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span
                              class="inline-block max-w-[14rem] truncate"
                              title={dockerTextValue(resource.docker?.objectCreatedAt)}
                            >
                              <PlatformTableRelativeTimeValue
                                value={resource.docker?.objectCreatedAt}
                              />
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden text-base-content`}
                          >
                            <span
                              class="inline-block max-w-[22rem] truncate"
                              title={dockerLabelsSummary(resource.docker?.labels)}
                            >
                              {dockerLabelsSummary(resource.docker?.labels)}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {dockerHostName(resource)}
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={5}
                            data-inline-docker-config-detail-for={resource.id}
                          >
                            <DockerNativeDetailPanel
                              title={dockerResourceName(resource)}
                              fields={[
                                ['Template', dockerTextValue(resource.docker?.templatingDriver)],
                                ['Created', dockerTextValue(resource.docker?.objectCreatedAt)],
                                ['Labels', dockerLabelsSummary(resource.docker?.labels)],
                                ['Host', dockerHostName(resource)],
                              ]}
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

export default DockerConfigsTable;
